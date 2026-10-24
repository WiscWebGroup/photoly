//go:build cgo

package ai

// SigLIP2 on ONNX Runtime (CPU). It runs inside `photoly ai-worker` (see worker.go), a
// child process, so a model that runs the machine out of memory only kills the worker.

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	ort "github.com/yalue/onnxruntime_go"
)

// Embedder loads each model on request and keeps it in memory.
type Embedder struct {
	cache *ModelCache
	files map[string]map[string]string // kind -> variant -> ONNX file
	image model
	text  model
	tok   *Tokenizer // set once the text model is loaded
	// threads is ONNX Runtime's intra-op thread count for models loaded from now on (0: its default).
	threads int
	device  string // requested device: cpu (default), cuda or coreml
	// fake serves deterministic hash vectors without ONNX Runtime or model files (AI_TEST_RUNTIME, tests only).
	fake bool
}

// SetThreads applies the admin's cpu_threads setting to the next model load.
func (e *Embedder) SetThreads(n int) { e.threads = n }

// SetDevice picks the execution provider for the next model load.
func (e *Embedder) SetDevice(d string) { e.device = d }

// useDevice appends the execution provider for dev to opts. ONNX Runtime falls back to
// CPU for ops a provider lacks; a provider that can't be set up at all is an error.
func useDevice(opts *ort.SessionOptions, dev string) error {
	switch dev {
	case "cuda":
		if runtime.GOOS == "darwin" {
			return errors.New("CUDA is not available on macOS")
		}
		c, err := ort.NewCUDAProviderOptions()
		if err != nil {
			return err
		}
		defer c.Destroy()
		if err = c.Update(map[string]string{"device_id": "0"}); err != nil {
			return err
		}
		return opts.AppendExecutionProviderCUDA(c)
	case "coreml":
		if runtime.GOOS != "darwin" {
			return errors.New("CoreML is only available on macOS")
		}
		return opts.AppendExecutionProviderCoreML(0)
	}
	return nil
}

// ProbeDevices reports, per non-CPU device, why it can't be used ("" when it can).
func (e *Embedder) ProbeDevices() map[string]string {
	out := map[string]string{}
	for _, d := range []string{"cuda", "coreml"} {
		opts, err := ort.NewSessionOptions()
		if err == nil {
			err = useDevice(opts, d)
			opts.Destroy()
		}
		if err != nil {
			out[d] = err.Error()
		} else {
			out[d] = ""
		}
	}
	return out
}

type model struct {
	// ponytail: one run at a time per model (ONNX Runtime already uses every core for one
	// run). Use a pool of sessions if many users embed at once.
	mu      sync.Mutex
	session *ort.DynamicAdvancedSession
	input   string
	info    LoadInfo
}

func newEmbedder(cache *ModelCache, profile Profile) (*Embedder, error) {
	if profile.Dtype != "fp32" {
		return nil, errors.New("server inference runs the fp32 model only; set PHOTOLY_MODEL_DTYPE=fp32")
	}
	if os.Getenv("AI_TEST_RUNTIME") != "" {
		return &Embedder{fake: true}, nil
	}
	lib := env("ONNXRUNTIME_LIB", map[string]string{"darwin": "libonnxruntime.dylib", "windows": "onnxruntime.dll"}[runtime.GOOS])
	if lib == "" {
		lib = "libonnxruntime.so"
	}
	// No usage data leaves a self-hosted server. It also avoids a crash: on Linux, ONNX
	// Runtime's telemetry runs popen(), which needs /bin/sh, and distroless has none.
	os.Setenv("ORT_DISABLE_TELEMETRY", "1")
	ort.SetSharedLibraryPath(lib)
	if ort.IsInitialized() {
		// another Embedder in this process (tests) already did it
	} else if err := ort.InitializeEnvironment(); err != nil {
		return nil, fmt.Errorf("ONNX Runtime (%s): %w", lib, err)
	}
	return &Embedder{
		cache: cache,
		files: map[string]map[string]string{
			"image": {"fp32": profile.VisionFile},
			// Queries are never stored, so a smaller text model doesn't change the index.
			"text": {"fp32": profile.TextFile, "int8": textInt8File},
		},
		image: model{input: "pixel_values"},
		text:  model{input: "input_ids"},
	}, nil
}

func (e *Embedder) model(kind string) (*model, error) {
	switch kind {
	case "image":
		return &e.image, nil
	case "text":
		return &e.text, nil
	}
	return nil, fmt.Errorf("unknown model %q", kind)
}

// Load loads and warms up one variant of a model ("image" or "text") unless that model
// is already loaded (in whichever variant).
func (e *Embedder) Load(ctx context.Context, kind, variant string) (LoadInfo, error) {
	m, err := e.model(kind)
	if err != nil {
		return LoadInfo{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session != nil {
		return m.info, nil
	}
	if e.fake {
		m.info = LoadInfo{Loaded: true, Variant: variant, Device: "cpu"}
		return m.info, nil
	}
	file, ok := e.files[kind][variant]
	if !ok {
		return LoadInfo{}, fmt.Errorf("no %s variant of the %s model", variant, kind)
	}
	started := time.Now()
	before := procMB("VmRSS")
	path, err := e.cache.Ensure(ctx, file)
	if err != nil {
		return LoadInfo{}, err
	}
	if kind == "text" {
		tokPath, err := e.cache.Ensure(ctx, "tokenizer.json")
		if err != nil {
			return LoadInfo{}, err
		}
		if e.tok, err = loadTokenizer(tokPath); err != nil {
			return LoadInfo{}, err
		}
	}
	var opts *ort.SessionOptions
	used, note := "cpu", ""
	if e.threads > 0 || e.device == "cuda" || e.device == "coreml" {
		if opts, err = ort.NewSessionOptions(); err != nil {
			return LoadInfo{}, err
		}
		defer opts.Destroy()
		if e.threads > 0 {
			if err = opts.SetIntraOpNumThreads(e.threads); err != nil {
				return LoadInfo{}, err
			}
		}
		if e.device == "cuda" || e.device == "coreml" {
			if perr := useDevice(opts, e.device); perr != nil {
				// A failed append can leave opts half-set; start over on CPU.
				note = e.device + " unavailable, using cpu: " + perr.Error()
				opts.Destroy()
				if opts, err = ort.NewSessionOptions(); err != nil {
					return LoadInfo{}, err
				}
				if e.threads > 0 {
					opts.SetIntraOpNumThreads(e.threads)
				}
			} else {
				used = e.device
			}
		}
	}
	session, err := ort.NewDynamicAdvancedSession(path, []string{m.input}, []string{"pooler_output"}, opts)
	if err != nil {
		return LoadInfo{}, fmt.Errorf("load %s: %w", filepath.Base(path), err)
	}
	m.session = session
	loaded := time.Now()
	// The first run sets up kernels; do it now rather than on the first real request.
	if kind == "image" {
		_, err = m.run(ort.NewShape(1, 3, imageSize, imageSize), make([]float32, 3*imageSize*imageSize))
	} else {
		var ids []int64
		if ids, err = e.tok.inputIDs("warm up"); err == nil {
			_, err = m.run(ort.NewShape(1, maxTokens), ids)
		}
	}
	if err != nil {
		session.Destroy()
		m.session = nil
		return LoadInfo{}, fmt.Errorf("warm up %s model: %w", kind, err)
	}
	m.info = LoadInfo{Loaded: true, Variant: variant, LoadMs: ms(loaded.Sub(started)), WarmMs: ms(time.Since(loaded)), Device: used, DeviceNote: note}
	log.Printf("%s model (%s) loaded in %.0f ms, warm-up %.0f ms; worker memory %d -> %d MB (peak %d MB)",
		kind, variant, m.info.LoadMs, m.info.WarmMs, before, procMB("VmRSS"), procMB("VmHWM"))
	return m.info, nil
}

// Close unloads both models.
func (e *Embedder) Close() {
	e.Unload("image")
	e.Unload("text")
}

// Unload frees a model and hands the memory back to the system where it can.
func (e *Embedder) Unload(kind string) error {
	m, err := e.model(kind)
	if err != nil {
		return err
	}
	m.mu.Lock()
	if e.fake {
		m.info = LoadInfo{}
	}
	if m.session != nil {
		m.session.Destroy()
		m.session, m.info = nil, LoadInfo{}
		if kind == "text" {
			e.tok = nil // ~100 MB of Go maps
		}
	}
	m.mu.Unlock()
	debug.FreeOSMemory()
	trimHeap()
	return nil
}

func (e *Embedder) Status() map[string]LoadInfo {
	status := map[string]LoadInfo{}
	for _, kind := range []string{"image", "text"} {
		m, _ := e.model(kind)
		if m.mu.TryLock() {
			status[kind] = m.info
			m.mu.Unlock()
		}
	}
	return status
}

var errNotLoaded = errors.New("model not loaded")

// EmbedImage embeds pixels from prepareImage. The model must be loaded.
func (e *Embedder) EmbedImage(pixels []float32) ([]float32, error) {
	e.image.mu.Lock()
	defer e.image.mu.Unlock()
	if e.fake && e.image.info.Loaded {
		return fakeVector(pixels), nil
	}
	if e.image.session == nil {
		return nil, errNotLoaded
	}
	return e.image.run(ort.NewShape(1, 3, imageSize, imageSize), pixels)
}

// EmbedText embeds a search query. The model must be loaded. Errors wrapping errQuery
// are the caller's fault.
func (e *Embedder) EmbedText(query string) (vec []float32, tokenize, run time.Duration, err error) {
	e.text.mu.Lock()
	defer e.text.mu.Unlock()
	if e.fake && e.text.info.Loaded {
		return fakeVector([]byte(query)), 0, 0, nil
	}
	if e.text.session == nil {
		return nil, 0, 0, errNotLoaded
	}
	started := time.Now()
	ids, err := e.tok.inputIDs(query)
	if err != nil {
		return nil, 0, 0, err
	}
	tokenized := time.Now()
	vec, err = e.text.run(ort.NewShape(1, maxTokens), ids)
	return vec, tokenized.Sub(started), time.Since(tokenized), err
}

// run feeds one input tensor and returns pooler_output as a unit vector.
func (m *model) run(shape ort.Shape, data any) ([]float32, error) {
	var input ort.Value
	var err error
	switch d := data.(type) {
	case []float32:
		input, err = ort.NewTensor(shape, d)
	case []int64:
		input, err = ort.NewTensor(shape, d)
	}
	if err != nil {
		return nil, err
	}
	defer input.Destroy()
	output, err := ort.NewEmptyTensor[float32](ort.NewShape(1, dimension))
	if err != nil {
		return nil, err
	}
	defer output.Destroy()
	if err := m.session.Run([]ort.Value{input}, []ort.Value{output}); err != nil {
		return nil, err
	}
	return unitVector(output.GetData()), nil
}

// fakeVector is a deterministic unit vector keyed by the input (AI_TEST_RUNTIME).
func fakeVector[T float32 | byte](in []T) []float32 {
	h := fnv.New64a()
	for _, v := range in {
		fmt.Fprint(h, v)
	}
	r := rand.New(rand.NewSource(int64(h.Sum64())))
	out := make([]float32, dimension)
	var sum float64
	for i := range out {
		out[i] = float32(r.NormFloat64())
		sum += float64(out[i]) * float64(out[i])
	}
	n := float32(math.Sqrt(sum))
	for i := range out {
		out[i] /= n
	}
	return out
}
