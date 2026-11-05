package ai

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModelNoticesWithoutWeights(t *testing.T) {
	cache := newModelCache(t.TempDir())
	cache.baseURL = "http://127.0.0.1:1/"
	for _, name := range []string{"LICENSE", "NOTICE"} {
		r := httptest.NewRequest("GET", "/models/notice", nil)
		r.SetPathValue("rev", modelRevision[:12])
		r.SetPathValue("model", modelName)
		r.SetPathValue("file", name)
		w := httptest.NewRecorder()
		cache.ServeHTTP(w, r)
		if w.Code != 200 || w.Body.Len() < 500 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/plain") {
			t.Fatalf("%s: %d", name, w.Code)
		}
	}
	entries, err := os.ReadDir(cache.dir)
	if err != nil || len(entries) != 0 {
		t.Fatal("notice request touched model cache", err)
	}
}
func TestCachedModelGetsNotices(t *testing.T) {
	cache := newModelCache(t.TempDir())
	cache.baseURL = "http://127.0.0.1:1/"
	name := "config.json"
	if err := os.MkdirAll(filepath.Dir(cache.path(name)), 0755); err != nil {
		t.Fatal(err)
	}
	// Existing-cache acceptance is by size; this test does not change that policy.
	if err := os.WriteFile(cache.path(name), make([]byte, modelFiles[name].Size), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Ensure(context.Background(), name); err != nil {
		t.Fatal(err)
	}
	for _, notice := range []string{"LICENSE", "NOTICE"} {
		b, err := os.ReadFile(cache.path(notice))
		if err != nil || len(b) < 500 {
			t.Fatalf("missing %s: %v", notice, err)
		}
	}
}
