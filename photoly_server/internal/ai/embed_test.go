//go:build cgo

package ai

import (
	"context"
	"math"
	"testing"
)

// Server vectors against Photoly's Python path (Pillow + ONNX Runtime CPU), and recall@1
// of the fixture queries. Needs PHOTOLY_TEST_MODELS and the ONNX Runtime library.
func TestServerEmbeddingsMatchReference(t *testing.T) {
	dir := modelDir(t)
	profile, _ := newProfile("fp32")
	cache := newModelCache(dir)
	cache.baseURL = "http://127.0.0.1:1/" // never download in tests
	ai, err := newEmbedder(cache, profile)
	if err != nil {
		t.Skipf("ONNX Runtime unavailable: %v", err)
	}
	ctx := context.Background()
	ref := loadReference(t)
	refImages := map[string][]float32{}
	for name, img := range ref.Images {
		refImages[name] = img.Vector
	}
	top := func(vec []float32) string {
		best, name := math.Inf(-1), ""
		for n, img := range refImages {
			if s := cosine(vec, img); s > best {
				best, name = s, n
			}
		}
		return name
	}
	// One model at a time, biggest first (fp32 text peaks near 2 GB while loading).
	checkText := func(variant string, minCosine float64) {
		if _, err := ai.Load(ctx, "text", variant); err != nil {
			t.Fatal(err)
		}
		defer ai.Close()
		for query, want := range ref.Queries {
			vec, _, _, err := ai.EmbedText(query)
			if err != nil {
				t.Fatal(err)
			}
			c, got := cosine(vec, want.Vector), top(vec)
			t.Logf("text %s %-34q cosine %.6f top %s", variant, query, c, got)
			if c < minCosine || got != want.Expected {
				t.Errorf("%s %q: cosine %.6f, top %s, want %s", variant, query, c, got, want.Expected)
			}
		}
	}
	if avail, ok := availableMB(); ok && avail < modelNeedMB["text"]["fp32"]+headroomMB {
		t.Logf("skipping the fp32 text model: %d MB free", avail)
	} else {
		checkText("fp32", 0.9999)
	}
	checkText("int8", 0.93) // the smaller model the server falls back to when memory is short

	if _, err := ai.Load(ctx, "image", "fp32"); err != nil {
		t.Fatal(err)
	}
	defer ai.Close()
	for name, want := range ref.Images {
		pixels, err := prepareImage(readFixture(t, name))
		if err != nil {
			t.Fatal(err)
		}
		vec, err := ai.EmbedImage(pixels)
		if err != nil {
			t.Fatal(err)
		}
		c := cosine(vec, want.Vector)
		t.Logf("image %-14s cosine %.6f", name, c)
		// PNGs are byte-identical after preprocessing; JPEG decoders differ slightly.
		if min := map[bool]float64{true: 0.9999, false: 0.995}[name != "rocket.jpg"]; c < min {
			t.Errorf("%s: cosine %.6f < %.4f", name, c, min)
		}
	}
	if s := ai.Status(); !s["image"].Loaded || s["text"].Loaded {
		t.Fatalf("status %+v", s)
	}
}
