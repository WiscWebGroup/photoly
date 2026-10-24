package ai

// The inference runtime (app/ai/runtime.py) as an in-process manager over the ai-worker
// child process: lifecycle states, generations, and two bounded queues (online 8,
// background 1) served one request at a time. There is no HTTP hop and no token: the
// worker is a child on pipes.

import (
	"errors"
	"runtime"
	"sync"
	"time"
)

const (
	onlineQueueMax     = 8
	backgroundQueueMax = 1
)

type encItem struct {
	kind        string // image | text
	pixels      []float32
	text        string
	fingerprint string
	deadline    time.Time
	done        chan struct{}
	vec         []float32
	err         error
}

type Runtime struct {
	profile Profile
	root    string // upload folder, for the disk figure in status

	wmu     sync.Mutex
	w       *Worker
	offWhy  string
	retryAt time.Time

	changeMu sync.Mutex // one lifecycle change at a time
	ioMu     sync.Mutex // one worker request at a time

	mu           sync.Mutex
	probe        map[string]string // device -> why unusable ("" when usable), from the worker
	devUsed      string
	devNote      string
	cond         *sync.Cond
	state        string // unloaded draining loading ready failed
	spec         map[string]any
	config       map[string]any
	generation   int64
	requested    int64
	cancel       chan struct{}
	lastOp       string
	verified     []map[string]any
	progress     map[string]any
	failure      string
	online       []*encItem
	background   []*encItem
	onlineStreak int
}

func NewRuntime(root string) *Runtime {
	r := &Runtime{profile: ONNXProfile(), root: root, state: "unloaded", generation: -1, requested: -1,
		config: map[string]any{"device": "cpu", "precision": "float32", "cpu_threads": 4}, cancel: make(chan struct{})}
	r.cond = sync.NewCond(&r.mu)
	go r.dispatch()
	return r
}

// worker returns the running AI worker, starting it if needed. It fails (and says why)
// where ONNX Runtime is unavailable; the next attempt waits 30 s.
func (r *Runtime) worker() (*Worker, error) {
	r.wmu.Lock()
	defer r.wmu.Unlock()
	if r.w != nil {
		return r.w, nil
	}
	if time.Now().Before(r.retryAt) {
		return nil, fail("runtime_unavailable", 503)
	}
	w, err := startWorker()
	if err != nil {
		r.offWhy, r.retryAt = err.Error(), time.Now().Add(30*time.Second)
		return nil, fail("runtime_unavailable", 503)
	}
	r.w, r.offWhy = w, ""
	return w, nil
}

// Close stops the worker.
func (r *Runtime) Close() {
	r.wmu.Lock()
	w := r.w
	r.w = nil
	r.wmu.Unlock()
	if w != nil {
		w.Close()
	}
}

// WhyOff says why server inference is unavailable ("" when it is, or was never tried).
func (r *Runtime) WhyOff() string { r.wmu.Lock(); defer r.wmu.Unlock(); return r.offWhy }

func (r *Runtime) reportProgress(stage string, completed, total *int, unit string) {
	p := map[string]any{"stage": stage, "completed": nil, "total": nil, "unit": nil,
		"updatedAt": time.Now().UTC().Format(time.RFC3339Nano), "lifecycleId": r.generation}
	if completed != nil {
		p["completed"], p["total"], p["unit"] = *completed, *total, unit
	}
	r.progress = p
}

func (r *Runtime) flush(err error) {
	for _, q := range []*[]*encItem{&r.online, &r.background} {
		for _, it := range *q {
			it.err = err
			close(it.done)
		}
		*q = nil
	}
}

// Submit accepts a lifecycle request without blocking (the scheduler and Pause stay
// responsive); poll Status. A newer generation cancels an older load that has not begun.
func (r *Runtime) Submit(command string, kinds []string, config map[string]any, generation int64, opID string) (map[string]any, error) {
	if err := checkCommand(command, config); err != nil {
		return nil, err
	}
	r.mu.Lock()
	if generation < max(r.requested, r.generation) {
		r.mu.Unlock()
		return nil, fail("stale_generation", 409)
	}
	if generation == r.requested {
		r.mu.Unlock()
		return r.Status(), nil
	}
	close(r.cancel)
	cancel := make(chan struct{})
	r.cancel, r.requested = cancel, generation
	r.mu.Unlock()
	go func() { _, _ = r.change(command, kinds, config, generation, opID, cancel) }()
	return r.Status(), nil
}

// Change runs a lifecycle request and waits for it.
func (r *Runtime) Change(command string, kinds []string, config map[string]any, generation int64, opID string) (map[string]any, error) {
	if err := checkCommand(command, config); err != nil {
		return nil, err
	}
	r.mu.Lock()
	cancel := r.cancel
	r.mu.Unlock()
	return r.change(command, kinds, config, generation, opID, cancel)
}

func checkCommand(command string, config map[string]any) error {
	if command != "load" && command != "unload" && command != "restart" {
		return fail("invalid_control")
	}
	if command != "unload" {
		dev, _ := config["device"].(string)
		if devicePrecisions[dev] == "" || devicePrecisions[dev] != config["precision"] {
			return fail("unsupported_device_precision")
		}
	}
	return nil
}

func (r *Runtime) change(command string, kinds []string, config map[string]any, generation int64, opID string, cancel chan struct{}) (map[string]any, error) {
	r.changeMu.Lock()
	defer r.changeMu.Unlock()
	cancelled := func() bool {
		select {
		case <-cancel:
			return true
		default:
			return false
		}
	}
	if cancelled() {
		return nil, fail("superseded", 409)
	}
	r.mu.Lock()
	if generation < max(r.generation, r.requested) && generation != r.requested {
		r.mu.Unlock()
		return nil, fail("stale_generation", 409)
	}
	if opID != "" && opID == r.lastOp && (r.state == "ready" || r.state == "unloaded") {
		r.mu.Unlock()
		return r.Status(), nil
	}
	r.state, r.generation, r.failure = "draining", generation, ""
	r.reportProgress("preparing", nil, nil, "")
	r.flush(fail("model_restarting", 503))
	r.mu.Unlock()

	r.ioMu.Lock()
	defer r.ioMu.Unlock()
	err := r.apply(command, kinds, config, cancelled)
	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		code := asErr(err).Code
		if errors.As(err, new(*LowMemoryError)) {
			code = "low_memory"
		}
		r.state, r.failure = "failed", code
		r.reportProgress("failed", nil, nil, "")
		r.spec = nil
		return nil, err
	}
	r.lastOp = opID
	return r.statusLocked(), nil
}

func (r *Runtime) apply(command string, kinds []string, config map[string]any, cancelled func() bool) error {
	r.mu.Lock()
	r.spec = nil
	r.mu.Unlock()
	if command == "unload" {
		r.unloadAll()
		r.mu.Lock()
		r.state = "unloaded"
		r.reportProgress("paused", nil, nil, "")
		r.mu.Unlock()
		return nil
	}
	dev, _ := config["device"].(string)
	threads, _ := toInt(config["cpu_threads"])
	r.mu.Lock()
	r.state, r.config = "loading", config
	r.mu.Unlock()
	if command == "restart" {
		r.unloadAll()
		r.Close()
	}
	if len(kinds) > 0 {
		w, err := r.worker()
		if err != nil {
			return err
		}
		w.Threads, w.Device = int(threads), dev
		if probe, err := w.Devices(); err == nil {
			r.mu.Lock()
			r.probe = probe
			r.mu.Unlock()
		}
		total := len(kinds)
		for i, kind := range kinds {
			if cancelled() {
				return fail("superseded", 409)
			}
			r.mu.Lock()
			r.reportProgress("loading", &i, &total, "models")
			r.mu.Unlock()
			info, err := w.Ensure(kind, false)
			if err != nil {
				return err
			}
			r.mu.Lock()
			r.devUsed, r.devNote = info.Device, info.DeviceNote
			r.mu.Unlock()
		}
	}
	if cancelled() {
		return fail("superseded", 409)
	}
	r.mu.Lock()
	r.spec, r.state = r.profile.Spec, "ready"
	n := len(kinds)
	r.reportProgress("ready", &n, &n, "models")
	combo := map[string]any{"fingerprint": r.profile.Fingerprint, "device": config["device"], "precision": config["precision"], "cpu_threads": config["cpu_threads"]}
	if !containsMap(r.verified, combo) {
		r.verified = append(r.verified, combo)
	}
	r.mu.Unlock()
	return nil
}

func containsMap(list []map[string]any, m map[string]any) bool {
	for _, x := range list {
		same := len(x) == len(m)
		for k, v := range m {
			if a, _ := toInt(x[k]); a != 0 || x[k] == nil {
				if b, _ := toInt(v); a != b {
					same = false
				}
			} else if x[k] != v {
				same = false
			}
		}
		if same {
			return true
		}
	}
	return false
}

func (r *Runtime) unloadAll() {
	r.wmu.Lock()
	w := r.w
	r.wmu.Unlock()
	if w != nil {
		for _, kind := range []string{"image", "text"} {
			_, _, _ = w.Unload(kind)
		}
	}
}

// Status is one consistent snapshot (state, fingerprint and generation from one transition).
func (r *Runtime) Status() map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.statusLocked()
}

func (r *Runtime) statusLocked() map[string]any {
	r.wmu.Lock()
	w, off := r.w, r.offWhy
	r.wmu.Unlock()
	if r.state == "ready" && w != nil && w.dead() {
		// Models are gone with the process (most often killed for memory); they reload on demand.
		r.state, r.failure = "failed", "runtime_offline"
		r.reportProgress("failed", nil, nil, "")
	}
	var fp any
	if r.spec != nil {
		fp = r.spec["fingerprint"]
	}
	var reason any
	if r.failure != "" {
		reason = r.failure
	} else if off != "" && r.state != "ready" {
		reason = "runtime_unavailable"
	}
	devices := []map[string]any{}
	for _, d := range deviceOrder {
		why := any(nil)
		if d != "cpu" {
			// Until a worker has probed, trust the platform; a load that can't use the provider falls back to CPU.
			if p, ok := r.probe[d]; ok && p != "" {
				why = "device_unavailable: " + p
			} else if !ok && (d == "coreml") != (runtime.GOOS == "darwin") {
				why = "device_unavailable"
			}
		}
		devices = append(devices, map[string]any{"device": d, "precision": devicePrecisions[d], "available": why == nil, "reason": why})
	}
	return map[string]any{"state": r.state, "progress": r.progress, "reason": reason, "fingerprint": fp,
		"generation": r.generation, "config": r.config, "verified": r.verified, "devices": devices, "device_used": r.devUsed, "device_note": r.devNote,
		"queued": len(r.online) + len(r.background), "resources": SystemUsage(r.root).toMap()}
}

// Ready: loaded and able to encode with this binary's profile.
func (r *Runtime) Ready() bool {
	s := r.Status()
	return s["state"] == "ready"
}

// Encode queues one embedding and waits for it. Queries are "online" (8 deep), indexing is
// "background" (1 deep); after 8 online requests in a row one background request goes next.
func (r *Runtime) Encode(it *encItem, background bool, timeout time.Duration) ([]float32, error) {
	it.deadline, it.done = time.Now().Add(timeout), make(chan struct{})
	r.mu.Lock()
	if r.state != "ready" || r.spec == nil || it.fingerprint != r.spec["fingerprint"] {
		r.mu.Unlock()
		return nil, fail("model_not_ready", 503)
	}
	q, limit := &r.online, onlineQueueMax
	if background {
		q, limit = &r.background, backgroundQueueMax
	}
	if len(*q) >= limit {
		r.mu.Unlock()
		return nil, fail("runtime_busy", 429)
	}
	*q = append(*q, it)
	r.cond.Signal()
	r.mu.Unlock()
	select {
	case <-it.done:
	case <-time.After(timeout):
		return nil, fail("query_timeout", 504)
	}
	return it.vec, it.err
}

func (r *Runtime) dispatch() {
	for {
		r.mu.Lock()
		for len(r.online) == 0 && len(r.background) == 0 {
			r.cond.Wait()
		}
		var it *encItem
		if len(r.background) > 0 && (r.onlineStreak >= 8 || len(r.online) == 0) {
			it, r.background = r.background[0], r.background[1:]
			r.onlineStreak = 0
		} else {
			it, r.online = r.online[0], r.online[1:]
			r.onlineStreak++
		}
		r.mu.Unlock()
		it.vec, it.err = r.run(it)
		close(it.done)
	}
}

func (r *Runtime) run(it *encItem) ([]float32, error) {
	r.ioMu.Lock()
	defer r.ioMu.Unlock()
	remaining := time.Until(it.deadline)
	if remaining <= 0 {
		return nil, fail("query_timeout", 504)
	}
	r.mu.Lock()
	ok := r.state == "ready" && r.spec != nil && r.spec["fingerprint"] == it.fingerprint
	r.mu.Unlock()
	if !ok {
		return nil, fail("model_not_ready", 503)
	}
	w, err := r.worker()
	if err != nil {
		return nil, err
	}
	type result struct {
		vec []float32
		err error
	}
	done := make(chan result, 1)
	go func() {
		var res result
		if it.kind == "image" {
			res.vec, res.err = w.EmbedImage(it.pixels)
		} else {
			res.vec, _, _, res.err = w.EmbedText(it.text)
		}
		done <- res
	}()
	select {
	case res := <-done:
		return res.vec, mapWorkerErr(res.err)
	case <-time.After(remaining):
		// A stuck inference is cut off by stopping the worker; models reload on demand.
		w.Close()
		r.mu.Lock()
		r.state, r.failure = "failed", "inference_timeout"
		r.mu.Unlock()
		return nil, fail("inference_timeout", 504)
	}
}

func mapWorkerErr(err error) error {
	var low *LowMemoryError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &low):
		return fail("low_memory", 503)
	case errors.Is(err, errWorkerDied):
		return fail("runtime_offline", 503)
	case errors.Is(err, errQuery):
		return fail("invalid_text")
	case errors.Is(err, errBadImage):
		return mapImageErr(err)
	}
	return fail("inference_failed", 502)
}

// ---- admin model load / unload (phase 5) ----

// LoadModel loads one server model ("image" or "text"). force loads even when memory looks
// short. Errors are *LowMemoryError (409 low-memory) or wrap errWorkerDied (503 worker-died).
func (r *Runtime) LoadModel(kind string, force bool) (LoadInfo, error) {
	w, err := r.worker()
	if err != nil {
		return LoadInfo{}, err
	}
	r.ioMu.Lock()
	defer r.ioMu.Unlock()
	return w.Ensure(kind, force)
}

func (r *Runtime) UnloadModel(kind string) (before, after int, err error) {
	r.wmu.Lock()
	w := r.w
	r.wmu.Unlock()
	if w == nil {
		return 0, 0, nil
	}
	r.ioMu.Lock()
	defer r.ioMu.Unlock()
	return w.Unload(kind)
}

// Models lists what the worker currently holds (nothing when it is stopped).
func (r *Runtime) Models() map[string]LoadInfo {
	r.wmu.Lock()
	w := r.w
	r.wmu.Unlock()
	if w == nil {
		return nil
	}
	return w.Status()
}

// dead reports that the worker process is gone.
func (w *Worker) dead() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.client != nil && isClosed(w.exited)
}

// WorkerStatus is the admin view of the AI worker process and its models.
func (r *Runtime) WorkerStatus() map[string]any {
	r.wmu.Lock()
	w, why := r.w, r.offWhy
	r.wmu.Unlock()
	out := map[string]any{"running": false, "appMemoryMB": procMB("VmRSS"), "restartedAt": nil, "restartReason": nil}
	models := map[string]any{}
	for _, kind := range []string{"image", "text"} {
		models[kind] = map[string]any{"loaded": false, "needMB": modelNeedMB[kind]["fp32"]}
	}
	out["models"] = models
	if w == nil {
		if why != "" {
			out["restartReason"] = why
		}
		return out
	}
	info := w.Info()
	out["running"], out["memoryMB"] = info.Running, info.MemoryMB
	if info.Restarted {
		out["restartedAt"] = info.StartedAt.UTC().Format(time.RFC3339)
		if info.LastEnded != "" {
			out["restartReason"] = info.LastEnded
		}
	}
	for kind, m := range w.Status() {
		d := map[string]any{"loaded": m.Loaded, "needMB": modelNeedMB[kind][m.Variant]}
		if m.Loaded {
			d["variant"], d["loadedAt"] = m.Variant, nil
		}
		models[kind] = d
	}
	return out
}
