package main

import (
	"compress/gzip"
	"context"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func unit(n int) []float32 {
	v := make([]float32, n)
	for i := range v {
		v[i] = float32(1 / math.Sqrt(float64(n)))
	}
	return v
}

func TestValidateEmbedding(t *testing.T) {
	if err := validateEmbedding(unit(dimension)); err != nil {
		t.Fatalf("unit vector rejected: %v", err)
	}
	notUnit := unit(dimension)
	notUnit[0] *= 2
	nan := unit(dimension)
	nan[5] = float32(math.NaN())
	inf := unit(dimension)
	inf[5] = float32(math.Inf(1))
	for name, v := range map[string][]float32{
		"short": unit(dimension - 1), "long": unit(dimension + 1), "not unit": notUnit, "NaN": nan, "Inf": inf,
	} {
		if err := validateEmbedding(v); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestProfile(t *testing.T) {
	a, err := newProfile("fp32")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := newProfile("fp16")
	if a.Fingerprint == b.Fingerprint || len(a.Fingerprint) != 64 {
		t.Fatalf("fingerprints must differ by dtype: %s %s", a.Fingerprint, b.Fingerprint)
	}
	again, _ := newProfile("fp32")
	if again.Fingerprint != a.Fingerprint {
		t.Fatal("fingerprint is not stable")
	}
	for _, name := range []string{a.VisionFile, a.TextFile, b.VisionFile, b.TextFile} {
		if _, ok := modelFiles[name]; !ok {
			t.Errorf("%s missing from the manifest", name)
		}
	}
	if _, err := newProfile("int8"); err == nil {
		t.Fatal("unknown dtype accepted")
	}
}

func TestModelRoutesOnlyServePinnedFiles(t *testing.T) {
	mux := http.NewServeMux()
	cache := newModelCache(t.TempDir())
	cache.baseURL = "http://127.0.0.1:1/" // must never be reached in this test
	mux.Handle("GET /models/{rev}/{model}/{file...}", cache)
	for _, path := range []string{
		"/models/" + modelRevision[:12] + "/" + modelName + "/../../etc/passwd",
		"/models/" + modelRevision[:12] + "/" + modelName + "/secret.json",
		"/models/000000000000/" + modelName + "/config.json",
		"/models/" + modelRevision[:12] + "/other-model/config.json",
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusMovedPermanently {
			t.Errorf("%s: status %d", path, rec.Code)
		}
	}
}

func TestModelDownloadIsVerified(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, strings.Repeat("x", 435)) // right size, wrong content
	}))
	defer upstream.Close()
	cache := newModelCache(t.TempDir())
	cache.baseURL = upstream.URL + "/"
	if _, err := cache.Ensure(context.Background(), "config.json"); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("tampered download accepted: %v", err)
	}
	if _, err := os.Stat(cache.path("config.json")); err == nil {
		t.Fatal("tampered file was kept")
	}
}

func TestStaticGzip(t *testing.T) {
	if _, err := webFiles.Open("web/vendor/transformers.min.js.gz"); err != nil {
		t.Skip("run scripts/vendor.sh first")
	}
	h := staticHandler()
	req := httptest.NewRequest("GET", "/vendor/transformers.min.js", nil)
	req.Header.Set("Accept-Encoding", "gzip, br")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Header().Get("Content-Encoding") != "gzip" || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/javascript") {
		t.Fatalf("gzip: %d %v", rec.Code, rec.Header())
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	gz, _ := io.ReadAll(zr)

	plain := httptest.NewRecorder()
	h.ServeHTTP(plain, httptest.NewRequest("GET", "/vendor/transformers.min.js", nil))
	if plain.Header().Get("Content-Encoding") != "" || plain.Body.String() != string(gz) {
		t.Fatal("uncompressed fallback differs")
	}

	wasm := httptest.NewRecorder()
	h.ServeHTTP(wasm, httptest.NewRequest("GET", "/vendor/ort-wasm-simd-threaded.asyncify.wasm", nil))
	if wasm.Header().Get("Content-Type") != "application/wasm" {
		t.Fatalf("wasm content type %q", wasm.Header().Get("Content-Type"))
	}
}
