//go:build cgo

package ai

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestPeopleRealInference(t *testing.T) {
	if os.Getenv("PHOTOLY_TEST_FACES") == "" {
		t.Skip("PHOTOLY_TEST_FACES=1 enables pinned real-model inference")
	}
	t.Setenv("AI_TEST_RUNTIME", "")
	e, err := newEmbedder(newModelCache(t.TempDir()), ONNXProfile())
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if _, err = e.Load(context.Background(), "people", "fp32"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"astronaut.png", "chelsea.png", "camera.png"} {
		folder := "ai"
		if name == "camera.png" {
			folder = "people"
		}
		data, err := os.ReadFile(filepath.Join("testdata", folder, name))
		if err != nil {
			t.Fatal(err)
		}
		im, err := canonicalFaceImage(data)
		if err != nil {
			t.Fatal(err)
		}
		started := time.Now()
		faces, err := e.DetectFaces(im)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %d faces, %s, RSS %d MB peak %d MB", name, len(faces), time.Since(started), procMB("VmRSS"), procMB("VmHWM"))
		if name == "astronaut.png" && len(faces) != 1 {
			t.Fatalf("expected astronaut face, got %d", len(faces))
		}
		if name == "chelsea.png" && len(faces) != 0 {
			t.Fatal("cat detected as human")
		}
		if dir := os.Getenv("PHOTOLY_FACE_REPORT"); dir != "" {
			raw, _ := json.Marshal(map[string]any{"faces": faces, "width": im.Width, "height": im.Height})
			os.WriteFile(filepath.Join(dir, name+".json"), raw, 0644)
			os.WriteFile(filepath.Join(dir, name+".rgb"), im.Bytes(), 0644)
		}
	}
}

func TestPeopleRealWorkerRecovery(t *testing.T) {
	if os.Getenv("PHOTOLY_TEST_FACES") == "" {
		t.Skip("set PHOTOLY_TEST_FACES=1")
	}
	t.Setenv("AI_TEST_RUNTIME", "")
	w, err := startWorker()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	w.Threads = 2
	raw, err := os.ReadFile("testdata/ai/astronaut.png")
	if err != nil {
		t.Fatal(err)
	}
	im, err := canonicalFaceImage(raw)
	if err != nil {
		t.Fatal(err)
	}
	faces, err := w.DetectFaces(context.Background(), im)
	if err != nil || len(faces) != 1 {
		t.Fatal("worker face inference", len(faces), err)
	}
	t.Logf("worker CPU model and inference RSS %d MB", w.Info().MemoryMB)
	w.mu.Lock()
	proc := w.proc
	w.mu.Unlock()
	if err = proc.Signal(syscall.SIGSTOP); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err = w.DetectFaces(ctx, im); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("worker deadline", err)
	}
	faces, err = w.DetectFaces(context.Background(), im)
	if err != nil || len(faces) != 1 {
		t.Fatal("worker restart", len(faces), err)
	}
	if _, after, err := w.Unload("people"); err != nil || after != 0 {
		t.Fatal("people unload", after, err)
	}
	if w.Status()["people"].Loaded {
		t.Fatal("people still loaded")
	}
}
