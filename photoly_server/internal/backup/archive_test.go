package backup

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestArchiveRoundTripAndCorruption(t *testing.T) {
	ctx := context.Background()
	root, backup, target := t.TempDir(), t.TempDir(), t.TempDir()
	owner := uuid.NewString()
	dir := filepath.Join(root, owner, "中文")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	content := strings.Repeat("photo-123-", 10000)
	if err := os.WriteFile(filepath.Join(dir, "live.mov"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	a, err := pack(ctx, root, owner, backup, "long password for tests", 128, func(int64, int64) {})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Parts) < 2 {
		t.Fatal("expected split archive")
	}
	if err = unpack(ctx, backup, target, "long password for tests", a, func(int64, int64) {}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(target, owner, "中文", "live.mov"))
	if err != nil || string(b) != content {
		t.Fatal("round trip mismatch", err)
	}
	if err = unpack(ctx, backup, t.TempDir(), "incorrect password", a, func(int64, int64) {}); err == nil {
		t.Fatal("wrong password accepted")
	}
	p := filepath.Join(backup, a.Parts[0].Name)
	if err = os.Rename(p, p+".missing"); err != nil {
		t.Fatal(err)
	}
	if err = unpack(ctx, backup, t.TempDir(), "long password for tests", a, func(int64, int64) {}); err == nil {
		t.Fatal("missing part accepted")
	}
	if err = os.Rename(p+".missing", p); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(p, []byte("damaged"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = unpack(ctx, backup, t.TempDir(), "long password for tests", a, func(int64, int64) {}); err == nil {
		t.Fatal("corrupt archive accepted")
	}
}
func TestUnsafePathsAndLinks(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"../outside", "/absolute", "a/../../x", "a\\b", "a/./b", ""} {
		if _, err := safe(root, p); err == nil {
			t.Errorf("accepted %q", p)
		}
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := safe(root, "link/file"); err == nil {
		t.Fatal("accepted symlink")
	}
	if _, err := pack(context.Background(), root, "link", t.TempDir(), "a sufficiently long password", partSize, func(int64, int64) {}); err == nil {
		t.Fatal("archived symlink")
	}
}
func TestArchiveCancellation(t *testing.T) {
	root := t.TempDir()
	_ = os.Mkdir(filepath.Join(root, "user"), 0700)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := pack(ctx, root, "user", t.TempDir(), "a sufficiently long password", partSize, func(int64, int64) {}); err == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestBackupStorageCapacity(t *testing.T) {
	if err := free(t.TempDir(), 1<<62); err == nil {
		t.Fatal("impossible capacity accepted")
	}
	small := os.Getenv("BACKUP_TEST_SMALL_VOLUME")
	if small == "" {
		return
	}
	dst, err := os.MkdirTemp(small, "disk-full-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dst)
	root := t.TempDir()
	_ = os.Mkdir(filepath.Join(root, "user"), 0700)
	data := make([]byte, 4<<20)
	if _, err = rand.Read(data); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "user", "random.bin"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = pack(context.Background(), root, "user", dst, "disk capacity password", partSize, func(int64, int64) {}); err == nil {
		t.Fatal("disk full did not fail")
	}
	if exists(filepath.Join(dst, "COMPLETE")) {
		t.Fatal("published an incomplete backup")
	}
}
