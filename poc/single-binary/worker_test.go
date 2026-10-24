package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The test binary doubles as the AI worker, as the real binary does.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "ai-worker" {
		if err := runWorker(); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestWorker(t *testing.T) {
	// The worker reads models from $PHOTOLY_DATA/models/<revision>.
	data := t.TempDir()
	if dir := os.Getenv("PHOTOLY_TEST_MODELS"); dir != "" {
		if err := os.Symlink(dir, filepath.Join(data, "models")); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PHOTOLY_DATA", data)
	// Model sizes are set per step below, so the test doesn't depend on free memory.
	saved := modelNeedMB
	defer func() { modelNeedMB = saved }()
	// Text uses int8 throughout: fp32 text (2 GB peak) next to the image model may not fit.
	modelNeedMB = map[string]map[string]int{"image": {"fp32": 1}, "text": {"fp32": 1 << 30, "int8": 1}}
	w, err := startWorker()
	if err != nil && strings.Contains(err.Error(), "without cgo") {
		t.Skipf("no server inference in this build: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	modelDir(t)
	ref := loadReference(t)

	// Round trip: a fixture embedded in the worker matches the reference.
	pixels, err := prepareImage(readFixture(t, "coffee.png"))
	if err != nil {
		t.Fatal(err)
	}
	vec, err := w.EmbedImage(pixels)
	if err != nil {
		t.Fatal(err)
	}
	if c := cosine(vec, ref.Images["coffee.png"].Vector); c < 0.9999 {
		t.Fatalf("image through the worker: cosine %.6f", c)
	}
	if _, _, _, err := w.EmbedText(strings.Repeat("dog cat ", 40)); !errors.Is(err, errQuery) {
		t.Fatalf("over-long query: %v, want errQuery", err)
	}

	// Kill the worker as the kernel would when out of memory, in the middle of a request
	// (frozen first, so the request is surely in flight): that request fails with
	// errWorkerDied, and the next one starts a fresh worker.
	w.mu.Lock()
	proc := w.proc
	w.mu.Unlock()
	proc.Signal(syscall.SIGSTOP)
	go func() {
		time.Sleep(300 * time.Millisecond)
		proc.Signal(syscall.SIGKILL)
	}()
	if _, err := w.EmbedImage(pixels); !errors.Is(err, errWorkerDied) || !strings.Contains(err.Error(), "out of memory") {
		t.Fatalf("killed mid-request: %v, want errWorkerDied", err)
	}
	if s := w.Status(); s["image"].Loaded {
		t.Fatal("a dead worker's models still reported as loaded")
	}
	if _, err := w.EmbedImage(pixels); err != nil {
		t.Fatalf("after a restart: %v", err)
	}

	// Unloading gives memory back: one model shrinks the worker, the last stops it.
	if _, _, _, err := w.EmbedText("a cup of coffee"); err != nil {
		t.Fatal(err)
	}
	before, after, err := w.Unload("text")
	if err != nil || (before > 0 && after >= before) || w.Status()["text"].Loaded || !w.Status()["image"].Loaded {
		t.Fatalf("unload text: %d -> %d MB, %v, status %+v", before, after, err, w.Status())
	}
	t.Logf("unload text: worker %d -> %d MB", before, after)
	before, after, err = w.Unload("image")
	if err != nil || after != 0 || w.Status() != nil {
		t.Fatalf("unload image: %d -> %d MB, %v, status %+v", before, after, err, w.Status())
	}
	t.Logf("unload image: worker %d MB -> stopped", before)
	if _, err := w.EmbedImage(pixels); err != nil {
		t.Fatalf("after the worker stopped: %v", err)
	}

	if _, ok := availableMB(); !ok {
		t.Skip("memory checks need Linux")
	}
	// Full text model doesn't fit, smaller one does: the smaller one loads by itself.
	modelNeedMB["text"] = map[string]int{"fp32": 1 << 30, "int8": 1}
	if info, err := w.Ensure("text", false); err != nil || info.Variant != "int8" {
		t.Fatalf("fallback: %+v %v, want int8", info, err)
	}
	w.Close()

	// Nothing fits: refused with the numbers, unless forced.
	modelNeedMB["text"] = map[string]int{"fp32": 1 << 30, "int8": 1 << 30}
	var low *LowMemoryError
	if _, err := w.Ensure("text", false); !errors.As(err, &low) || low.Variant != "int8" || low.AvailableMB <= 0 {
		t.Fatalf("low memory: %v", err)
	}
	if info, err := w.Ensure("text", true); err != nil || info.Variant != "int8" {
		t.Fatalf("forced: %+v %v", info, err)
	}
}
