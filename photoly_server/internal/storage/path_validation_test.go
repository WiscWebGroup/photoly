package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathConstructionRejectsUntrustedComponents(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"", ".", "..", "../outside", "/etc/passwd", "a/../b", "a/b", `a\b`, "a\x00b"} {
		t.Run(name, func(t *testing.T) {
			if p, err := SafePath(root, name); err == nil {
				t.Fatalf("accepted %q as %q", name, p)
			}
		})
	}
	for _, name := range []string{"照片 (1).jpg", "report..jpg", ".hidden.jpg"} {
		p, err := SafePath(root, name)
		if err != nil || p != filepath.Join(root, name) {
			t.Fatalf("valid filename %q: %q %v", name, p, err)
		}
	}
}

func TestLocalStoreRejectsLinksAndRootKeys(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "private.jpg"), []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	st := Local{Dir: root}
	ctx := context.Background()
	for _, key := range []string{".", "..", "../outside", "/etc/passwd", "a/../../outside", `a\b`, "link/private.jpg"} {
		if _, err := st.Get(ctx, key); err == nil {
			t.Fatalf("Get accepted %q", key)
		}
		if _, err := st.Stat(ctx, key); err == nil {
			t.Fatalf("Stat accepted %q", key)
		}
		if err := st.Put(ctx, key, strings.NewReader("changed"), 7); err == nil {
			t.Fatalf("Put accepted %q", key)
		}
		if err := st.Delete(ctx, key); err == nil {
			t.Fatalf("Delete accepted %q", key)
		}
		if err := st.DeletePrefix(ctx, key); err == nil {
			t.Fatalf("DeletePrefix accepted %q", key)
		}
	}
	if b, err := os.ReadFile(filepath.Join(outside, "private.jpg")); err != nil || string(b) != "private" {
		t.Fatalf("outside file changed: %q %v", b, err)
	}
}
