package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// The model is the ONNX export of google/siglip2-base-patch16-224 at revision 75de2d55,
// the same revision Photoly's PyTorch runtime pins. Every file is pinned by SHA-256.
const (
	modelRepo     = "onnx-community/siglip2-base-patch16-224-ONNX"
	modelRevision = "ba1f3b0843f24bc5417d38e19c37b287d719b2f4"
	modelName     = "siglip2-base-patch16-224"
	baseModel     = "google/siglip2-base-patch16-224@75de2d55ec2d0b4efc50b3e9ad70dba96a7b2fa2"
	dimension     = 768
	// Bump when the browser's image preparation changes; it is part of the fingerprint.
	preprocessing = "exif-white-rgb-224-bilinear-v1"
)

type modelFile struct {
	Size   int64
	SHA256 string
}

var modelFiles = map[string]modelFile{
	"config.json":                 {435, "e43a9f7692d3819886a82cb2097048258d444f123c67d37ec825f9345b019cf2"},
	"preprocessor_config.json":    {394, "9b36b57ebaf20f09bf4c22100ccc21877ea6bfe5aead0c00c59f8af8ccefacfc"},
	"tokenizer.json":              {34363039, "cb9140fae3ac5122c972d37adf83e1248471a38147ad76f8215c8872c6fd8322"},
	"tokenizer_config.json":       {47240, "7c3a247599e741bceba1a3fe0285aea88d1044dc1fad2caa1e48cdd9fd25f630"},
	"special_tokens_map.json":     {636, "baec30ea10906f16adb8c18af7a34023002c1746542612b8b41c9f09e1351351"},
	"onnx/vision_model.onnx":      {371807752, "c0573e3f4140c3a7c4e9cc5912bd6b26a033b46a6a8e8af26cbea262b163bcad"},
	"onnx/vision_model_fp16.onnx": {186039516, "a1959f7bd3993a607e48839f6d01e25b876fe76afda301b028b78eef68aabd95"},
	"onnx/text_model.onnx":        {1129469657, "baf12d941beabafafb14f7b4adb38dc15be18681b964a84410ec53d9d65e6293"},
	"onnx/text_model_fp16.onnx":   {564862230, "711da56ada0a4aa11c7dd3320df741081a3cae4f0ae1b5e5c6d5b294738d0eb0"},
}

// Profile describes exactly which model produces the stored vectors. Clients must send the
// fingerprint with every vector; a mismatch is rejected.
type Profile struct {
	Model         string `json:"model"`
	BaseModel     string `json:"baseModel"`
	Revision      string `json:"revision"`
	Dtype         string `json:"dtype"`
	VisionFile    string `json:"visionFile"`
	VisionSHA256  string `json:"visionSha256"`
	TextFile      string `json:"textFile"`
	TextSHA256    string `json:"textSha256"`
	Dimension     int    `json:"dimension"`
	Preprocessing string `json:"preprocessing"`
	Normalization string `json:"normalization"`
	Fingerprint   string `json:"fingerprint"`
	// Where the browser loads the model from (versioned, so browser caches never go stale).
	ModelBaseURL string `json:"modelBaseUrl"`
	ModelName    string `json:"modelName"`
}

func newProfile(dtype string) (Profile, error) {
	suffix := map[string]string{"fp32": "", "fp16": "_fp16"}
	s, ok := suffix[dtype]
	if !ok {
		return Profile{}, fmt.Errorf("unsupported dtype %q (use fp32 or fp16)", dtype)
	}
	vision, text := "onnx/vision_model"+s+".onnx", "onnx/text_model"+s+".onnx"
	p := Profile{
		Model: modelRepo, BaseModel: baseModel, Revision: modelRevision, Dtype: dtype,
		VisionFile: vision, VisionSHA256: modelFiles[vision].SHA256,
		TextFile: text, TextSHA256: modelFiles[text].SHA256,
		Dimension: dimension, Preprocessing: preprocessing, Normalization: "l2-fp32",
	}
	raw, _ := json.Marshal(p) // Fingerprint and URL fields are still empty here.
	sum := sha256.Sum256(raw)
	p.Fingerprint = hex.EncodeToString(sum[:])
	p.ModelBaseURL = "/models/" + modelRevision[:12] + "/"
	p.ModelName = modelName
	return p, nil
}

// ModelCache downloads pinned files from Hugging Face on first use, verifies them, and
// serves them from disk afterwards.
type ModelCache struct {
	dir     string
	baseURL string
	client  *http.Client
	mu      sync.Mutex
	pending map[string]*sync.Mutex
}

func newModelCache(dir string) *ModelCache {
	return &ModelCache{
		dir:     dir,
		baseURL: "https://huggingface.co/" + modelRepo + "/resolve/" + modelRevision + "/",
		client:  &http.Client{Timeout: 30 * time.Minute},
		pending: map[string]*sync.Mutex{},
	}
}

func (c *ModelCache) path(name string) string {
	return filepath.Join(c.dir, modelRevision, filepath.FromSlash(name))
}

// Ensure returns the local path of a verified copy of name, downloading it if needed.
// Concurrent callers for the same file wait for one download.
func (c *ModelCache) Ensure(ctx context.Context, name string) (string, error) {
	want, ok := modelFiles[name]
	if !ok {
		return "", errNotInManifest
	}
	c.mu.Lock()
	lock := c.pending[name]
	if lock == nil {
		lock = &sync.Mutex{}
		c.pending[name] = lock
	}
	c.mu.Unlock()
	lock.Lock()
	defer lock.Unlock()

	dst := c.path(name)
	if st, err := os.Stat(dst); err == nil && st.Size() == want.Size {
		return dst, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	log.Printf("model: downloading %s (%.1f MB)", name, float64(want.Size)/1e6)
	req, err := http.NewRequestWithContext(context.WithoutCancel(ctx), http.MethodGet, c.baseURL+name, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: HTTP %d", name, resp.StatusCode)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".download-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, hash), resp.Body)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	if got := hex.EncodeToString(hash.Sum(nil)); n != want.Size || got != want.SHA256 {
		return "", fmt.Errorf("download %s: got %d bytes sha256 %s, want %d bytes %s", name, n, got, want.Size, want.SHA256)
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return "", err
	}
	log.Printf("model: verified %s", name)
	return dst, nil
}

var errNotInManifest = errors.New("file is not part of the pinned model")

// ServeHTTP serves /models/{rev12}/siglip2-base-patch16-224/{file}.
func (c *ModelCache) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("rev") != modelRevision[:12] || r.PathValue("model") != modelName {
		http.NotFound(w, r)
		return
	}
	name := r.PathValue("file")
	path, err := c.Ensure(r.Context(), name)
	if errors.Is(err, errNotInManifest) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("model: %v", err)
		http.Error(w, "model download failed", http.StatusBadGateway)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()
	st, _ := f.Stat()
	// The URL contains the revision, so the content never changes.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+modelFiles[name].SHA256+`"`)
	if filepath.Ext(name) == ".json" {
		w.Header().Set("Content-Type", "application/json")
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	http.ServeContent(w, r, "", st.ModTime(), f)
}
