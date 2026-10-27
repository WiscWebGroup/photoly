package ai

// Server-side inference runs in a child process, `photoly ai-worker`, started by `photoly
// serve`. A goroutine would share the server's memory: if a model doesn't fit, the kernel
// kills the whole process, API included. A separate process is what gets killed instead
// (it also asks to be first in line), and the server reports it and starts a fresh worker
// on the next request.
//
// The two talk net/rpc over a pair of pipes (fds 3 and 4), not stdin/stdout, so nothing
// ONNX Runtime prints can corrupt the stream.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/rpc"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ---------- child side ----------

type LoadArgs struct {
	Kind, Variant string
	Threads       int
	Device        string
}
type ImageArgs struct{ Pixels []float32 }
type TextArgs struct{ Query string }
type VecReply struct {
	Vec           []float32
	Tokenize, Run time.Duration
}
type StatusReply struct{ Models map[string]LoadInfo }

// AIWorker is the RPC service inside the child.
type AIWorker struct{ e *Embedder }

func (w *AIWorker) Load(a LoadArgs, r *LoadInfo) (err error) {
	w.e.SetThreads(a.Threads)
	w.e.SetDevice(a.Device)
	*r, err = w.e.Load(context.Background(), a.Kind, a.Variant)
	return err
}
func (w *AIWorker) Devices(_ struct{}, r *map[string]string) error {
	*r = w.e.ProbeDevices()
	return nil
}
func (w *AIWorker) Status(_ struct{}, r *StatusReply) error {
	r.Models = w.e.Status()
	return nil
}
func (w *AIWorker) Unload(kind string, r *StatusReply) error {
	if err := w.e.Unload(kind); err != nil {
		return err
	}
	r.Models = w.e.Status()
	return nil
}
func (w *AIWorker) EmbedImage(a ImageArgs, r *VecReply) (err error) {
	r.Vec, err = w.e.EmbedImage(a.Pixels)
	return err
}
func (w *AIWorker) EmbedText(a TextArgs, r *VecReply) (err error) {
	r.Vec, r.Tokenize, r.Run, err = w.e.EmbedText(a.Query)
	return err
}

// pipeConn joins a read pipe and a write pipe into one connection.
type pipeConn struct {
	io.ReadCloser
	w io.WriteCloser
}

func (p pipeConn) Write(b []byte) (int, error) { return p.w.Write(b) }
func (p pipeConn) Close() error {
	p.w.Close()
	return p.ReadCloser.Close()
}

// runWorker is `photoly ai-worker`. It serves until the server closes the pipes.
func runWorker() error {
	in, out := os.NewFile(3, "rpc-in"), os.NewFile(4, "rpc-out")
	if in == nil || out == nil {
		return errors.New("ai-worker is started by `photoly serve`, not by hand")
	}
	// Be the process the kernel kills first when memory runs out (Linux; best effort).
	os.WriteFile("/proc/self/oom_score_adj", []byte("1000"), 0)
	log.SetPrefix("ai-worker: ")
	profile, err := newProfile(env("PHOTOLY_MODEL_DTYPE", "fp32"))
	if err != nil {
		return err
	}
	srv := rpc.NewServer()
	e, initErr := newEmbedder(newModelCache(filepath.Join(dataDir(), "models")), profile)
	if initErr == nil {
		srv.Register(&AIWorker{e})
	} else {
		srv.RegisterName("AIWorker", &brokenWorker{initErr.Error()})
	}
	srv.ServeConn(pipeConn{in, out})
	return nil
}

// brokenWorker answers every call with the reason ONNX Runtime didn't start.
type brokenWorker struct{ reason string }

func (b *brokenWorker) Load(LoadArgs, *LoadInfo) error        { return errors.New(b.reason) }
func (b *brokenWorker) Devices(struct{}, *map[string]string) error {
	return errors.New(b.reason)
}
func (b *brokenWorker) Status(struct{}, *StatusReply) error   { return errors.New(b.reason) }
func (b *brokenWorker) EmbedImage(ImageArgs, *VecReply) error { return errors.New(b.reason) }
func (b *brokenWorker) Unload(string, *StatusReply) error     { return errors.New(b.reason) }
func (b *brokenWorker) EmbedText(TextArgs, *VecReply) error   { return errors.New(b.reason) }

// ---------- server side ----------

// Worker starts and talks to the AI worker, and gates model loading on free memory.
type Worker struct {
	mu        sync.Mutex
	client    *rpc.Client
	exited    chan struct{} // closed when the current process is gone
	proc      *os.Process
	why       string // how the last process ended, if it ended badly
	started   time.Time
	restarted bool // a process started after an earlier one ended
	// Threads is ONNX Runtime's intra-op thread count for models loaded from now on.
	Threads int
	// Device is the execution provider for models loaded from now on (cpu, cuda, coreml).
	Device string
}

// Devices probes which non-CPU providers the worker's ONNX Runtime can use.
func (w *Worker) Devices() (map[string]string, error) {
	var r map[string]string
	err := w.call("Devices", struct{}{}, &r)
	return r, err
}

// errWorkerDied means the worker process ended mid-request (most likely out of memory).
var errWorkerDied = errors.New("the AI worker stopped")

// LowMemoryError means a model probably doesn't fit; the client can retry with force.
type LowMemoryError struct {
	Kind        string `json:"kind"`
	Variant     string `json:"variant"`
	NeedMB      int    `json:"needMB"`
	AvailableMB int    `json:"availableMB"`
}

func (e *LowMemoryError) Error() string {
	return fmt.Sprintf("the %s model needs about %d MB, but the server has about %d MB free", e.Kind, e.NeedMB, e.AvailableMB)
}

// startWorker starts the worker and checks that ONNX Runtime works in it. The error says
// why server inference is unavailable.
func startWorker() (*Worker, error) {
	w := &Worker{}
	var status StatusReply
	if err := w.call("Status", struct{}{}, &status); err != nil {
		w.Close()
		return nil, err
	}
	return w, nil
}

// conn returns a client for a running worker, starting a new process if needed.
func (w *Worker) conn() (*rpc.Client, chan struct{}, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.client != nil {
		select {
		case <-w.exited:
		default:
			return w.client, w.exited, nil
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, nil, err
	}
	childIn, toChild, err := os.Pipe() // the child reads childIn as fd 3
	if err != nil {
		return nil, nil, err
	}
	fromChild, childOut, err := os.Pipe() // and writes childOut as fd 4
	if err != nil {
		return nil, nil, err
	}
	cmd := exec.Command(exe, "ai-worker")
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	cmd.ExtraFiles = []*os.File{childIn, childOut}
	err = cmd.Start()
	childIn.Close()
	childOut.Close()
	if err != nil {
		toChild.Close()
		fromChild.Close()
		return nil, nil, err
	}
	exited := make(chan struct{})
	go func() {
		err := cmd.Wait()
		w.mu.Lock()
		w.why = exitReason(cmd.ProcessState, err)
		w.mu.Unlock()
		log.Printf("ai: worker %s", w.why)
		toChild.Close()
		fromChild.Close()
		close(exited)
	}()
	w.restarted = w.restarted || w.proc != nil
	w.started = time.Now()
	w.client = rpc.NewClient(pipeConn{fromChild, toChild})
	w.proc = cmd.Process
	w.exited = exited
	return w.client, exited, nil
}

func exitReason(ps *os.ProcessState, err error) string {
	if ws, ok := ps.Sys().(syscall.WaitStatus); ok && ws.Signaled() && ws.Signal() == syscall.SIGKILL {
		return "was killed by the system, most likely because the server ran out of memory"
	}
	if err != nil {
		return "stopped: " + err.Error()
	}
	return "exited"
}

func (w *Worker) call(method string, args, reply any) error {
	client, exited, err := w.conn()
	if err != nil {
		return fmt.Errorf("start AI worker: %w", err)
	}
	err = client.Call("AIWorker."+method, args, reply)
	var remote rpc.ServerError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &remote):
		// Errors from the worker arrive as text; restore the one callers check for.
		if rest, ok := strings.CutPrefix(string(remote), errQuery.Error()); ok {
			return fmt.Errorf("%w%s", errQuery, rest)
		}
		return errors.New(string(remote))
	}
	// The connection broke: the worker is gone. Wait for its exit status to say why.
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
	}
	w.mu.Lock()
	why := w.why
	w.mu.Unlock()
	return fmt.Errorf("%w: it %s", errWorkerDied, why)
}

// Close stops the worker and waits (briefly) until it is gone.
func (w *Worker) Close() {
	w.mu.Lock()
	exited := w.exited
	if w.client != nil {
		w.client.Close() // closes the pipes; the worker exits
		w.client = nil
	}
	w.mu.Unlock()
	if exited != nil {
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
		}
	}
}

// Unload frees one model. When the worker holds no model any more it is stopped, which
// is the only sure way to give all of its memory back (the next request starts a new
// one). It returns the worker's memory before and after, in MB (0 where unknown).
// ponytail: assumes one user per server, as in this POC. With several, unload when the
// last client stops using a model, or after an idle timeout.
func (w *Worker) Unload(kind string) (beforeMB, afterMB int, err error) {
	w.mu.Lock()
	running := w.client != nil && !isClosed(w.exited)
	pid := ""
	if running {
		pid = strconv.Itoa(w.proc.Pid)
	}
	w.mu.Unlock()
	if !running {
		return 0, 0, nil
	}
	beforeMB = workerMB(pid)
	var r StatusReply
	if err := w.call("Unload", kind, &r); err != nil {
		return beforeMB, 0, err
	}
	for _, info := range r.Models {
		if info.Loaded {
			return beforeMB, workerMB(pid), nil
		}
	}
	w.Close()
	return beforeMB, 0, nil
}

// Status lists loaded models. A stopped worker has none, and is not restarted for this.
func (w *Worker) Status() map[string]LoadInfo {
	models, _ := w.status()
	return models
}

// status fails only if the worker dies during the call. One that died earlier (say,
// killed while idle) just has no models, and the next load starts a fresh one.
func (w *Worker) status() (map[string]LoadInfo, error) {
	w.mu.Lock()
	running := w.client != nil && !isClosed(w.exited)
	w.mu.Unlock()
	var r StatusReply
	if !running {
		return nil, nil
	}
	err := w.call("Status", struct{}{}, &r)
	return r.Models, err
}

func isClosed(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// Ensure loads a model if it isn't loaded. It picks the variant that fits in free memory
// (for text, the smaller int8 model if fp32 doesn't fit). If none fits it returns
// *LowMemoryError, unless force is set, when it loads the smallest variant anyway.
func (w *Worker) Ensure(kind string, force bool) (LoadInfo, error) {
	models, err := w.status()
	if err != nil {
		return LoadInfo{}, err
	}
	if info := models[kind]; info.Loaded {
		return info, nil
	}
	variants := map[string][]string{"image": {"fp32"}, "text": {"fp32", "int8"}}[kind]
	if variants == nil {
		return LoadInfo{}, fmt.Errorf("unknown model %q", kind)
	}
	variant := variants[len(variants)-1]
	if avail, ok := availableMB(); ok {
		chosen := ""
		for _, v := range variants {
			if modelNeedMB[kind][v]+headroomMB <= avail {
				chosen = v
				break
			}
		}
		if chosen == "" && !force {
			return LoadInfo{}, &LowMemoryError{kind, variant, modelNeedMB[kind][variant], avail}
		}
		if chosen != "" {
			variant = chosen
		}
	} else {
		variant = variants[0] // can't measure memory here: load the full model
	}
	var info LoadInfo
	err = w.call("Load", LoadArgs{kind, variant, w.Threads, w.Device}, &info)
	return info, err
}

// EmbedImage and EmbedText load the model first if it fits (see Ensure).
func (w *Worker) EmbedImage(pixels []float32) ([]float32, error) {
	if _, err := w.Ensure("image", false); err != nil {
		return nil, err
	}
	var r VecReply
	err := w.call("EmbedImage", ImageArgs{pixels}, &r)
	return r.Vec, err
}

func (w *Worker) EmbedText(query string) ([]float32, time.Duration, time.Duration, error) {
	if _, err := w.Ensure("text", false); err != nil {
		return nil, 0, 0, err
	}
	var r VecReply
	err := w.call("EmbedText", TextArgs{query}, &r)
	return r.Vec, r.Tokenize, r.Run, err
}

// WorkerInfo is what the admin page shows about the worker process.
type WorkerInfo struct {
	Running   bool
	MemoryMB  int
	StartedAt time.Time
	Restarted bool
	LastEnded string
}

func (w *Worker) Info() WorkerInfo {
	w.mu.Lock()
	defer w.mu.Unlock()
	i := WorkerInfo{Restarted: w.restarted, StartedAt: w.started, LastEnded: w.why}
	if w.client != nil && !isClosed(w.exited) {
		i.Running = true
		i.MemoryMB = workerMB(strconv.Itoa(w.proc.Pid))
	}
	return i
}
