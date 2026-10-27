package uploads

import (
	"context"
	"testing"
)

func TestSuffixAndPath(t *testing.T) {
	for in, want := range map[string]string{"a/b.JPG": "jpg", ".hidden": "", "x.": "", "a.tar.zip": "zip", "noext": ""} {
		if got := suffix(in); got != want {
			t.Errorf("suffix(%q)=%q want %q", in, got, want)
		}
	}
	for _, bad := range []any{"/abs", `a\b`, "a/../b", "a:b", "", 5, "a//b"} {
		if _, err := relativePath(bad); err == nil {
			t.Errorf("relativePath(%v) accepted", bad)
		}
	}
	if _, err := relativePath("trip/day 1/IMG.heic"); err != nil {
		t.Error(err)
	}
}

func TestEmbedding(t *testing.T) {
	v := make([]float32, 768)
	v[0] = 1
	if !ValidVector(v) || ValidVector(v[:767]) {
		t.Fatal("ValidVector")
	}
	v[1] = 0.5
	if ValidVector(v) {
		t.Fatal("non-unit accepted")
	}
	v[1] = 0
	if _, err := StoreClientEmbedding(context.Background(), 1, 1, "", v); err == nil {
		t.Fatal("empty fingerprint accepted")
	}
	if st, err := StoreClientEmbedding(context.Background(), 1, 1, "fp", v); err != nil || st != Queued {
		t.Fatal(st, err)
	}
}
