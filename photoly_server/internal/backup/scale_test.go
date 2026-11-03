package backup

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
)

// This explicitly measures archive file-count and byte-throughput costs, not database/API capacity.
func TestBackupArchiveScale(t *testing.T) {
	count, _ := strconv.Atoi(os.Getenv("BACKUP_SCALE_FILES"))
	if count == 0 {
		t.Skip("set BACKUP_SCALE_FILES=200000 or 500000 for archive scale")
	}
	size, _ := strconv.Atoi(os.Getenv("BACKUP_SCALE_BYTES"))
	if size <= 0 {
		size = 4096
	}
	root, repo, target := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "user"), 0700); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, min(size, 1<<20))
	started := time.Now()
	for i := 0; i < count; i++ {
		dir := filepath.Join(root, "user", fmt.Sprintf("%04d", i/1000))
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("media-%08d.bin", i)))
		if err != nil {
			t.Fatal(err)
		}
		for left := size; left > 0; {
			n := min(left, len(buf))
			if _, err = rand.Read(buf[:n]); err != nil {
				t.Fatal(err)
			}
			if _, err = f.Write(buf[:n]); err != nil {
				t.Fatal(err)
			}
			left -= n
		}
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("seed files=%d bytesPerFile=%d elapsed=%s", count, size, time.Since(started))
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	started = time.Now()
	a, err := pack(context.Background(), root, "user", repo, "scale test password", 4<<20, func(int64, int64) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("backup elapsed=%s parts=%d bytes=%d", time.Since(started), len(a.Parts), a.Bytes)
	started = time.Now()
	if err = unpack(context.Background(), repo, target, "scale test password", a, func(int64, int64) {}); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&after)
	t.Logf("restore elapsed=%s allocatedBytes=%d heapBytes=%d", time.Since(started), after.TotalAlloc-before.TotalAlloc, after.HeapAlloc)
}
