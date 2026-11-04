package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenFreshBypassesRemoteCache(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cached")
	if e := os.WriteFile(p, []byte("stale"), 0600); e != nil {
		t.Fatal(e)
	}
	m := newMem()
	m.m["original"] = []byte("fresh")
	Bind(p, m, "original")
	r, e := OpenFresh(context.Background(), p)
	if e != nil {
		t.Fatal(e)
	}
	data, e := io.ReadAll(r)
	r.Close()
	if e != nil || string(data) != "fresh" {
		t.Fatal(string(data), e)
	}
	delete(m.m, "original")
	if r, e = OpenFresh(context.Background(), p); e == nil {
		r.Close()
		t.Fatal("missing remote masked by cache")
	}
}
func TestSameObjectOverlappingBindings(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	x, e := NewDAV(map[string]string{"url": "https://example.test/photos/"})
	if e != nil {
		t.Fatal(e)
	}
	y, e := NewDAV(map[string]string{"url": "https://example.test/"})
	if e != nil {
		t.Fatal(e)
	}
	Bind(a, x, "image.png")
	Bind(b, y, "photos/image.png")
	if !SameObject(a, b) {
		t.Fatal("overlapping DAV alias missed")
	}
	Bind(b, y, "elsewhere/image.png")
	if SameObject(a, b) {
		t.Fatal("distinct object treated as alias")
	}
	sx, _ := NewSMB(map[string]string{"server": "nas", "share": "photos", "subdirectory": "summer"})
	sy, _ := NewSMB(map[string]string{"server": "NAS", "share": "PHOTOS"})
	Bind(a, sx, "image.png")
	Bind(b, sy, "SUMMER/IMAGE.PNG")
	if !SameObject(a, b) {
		t.Fatal("SMB alias missed")
	}
}
