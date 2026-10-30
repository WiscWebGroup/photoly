package storage

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSMBPaths(t *testing.T) {
	for _, key := range []string{"../a", "a/../b", "a\\b", "a:b", "%2e%2e/x", "x/child.", ".photoly-system"} {
		if _, e := smbParts(key, false); e == nil {
			t.Errorf("accepted %q", key)
		}
	}
	for _, key := range []string{"中文/照片.jpg", "file (1).jpg", "a#b&c.jpg"} {
		if _, e := smbParts(key, false); e != nil {
			t.Fatal(key, e)
		}
	}
}

// TestSMBIntegration writes only beneath a unique directory that this test owns.
// Credentials come from a local file, never command arguments or test output.
func TestSMBIntegration(t *testing.T) {
	file := os.Getenv("SMB_TEST_ENV_FILE")
	if file == "" {
		t.Skip("SMB_TEST_ENV_FILE required")
	}
	b, e := os.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	v := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		p := strings.SplitN(line, "=", 2)
		if len(p) == 2 {
			v[strings.TrimSpace(p[0])] = strings.Trim(strings.TrimSpace(p[1]), "\"'")
		}
	}
	host := os.Getenv("SMB_TEST_HOST")
	if host == "" {
		host = "10.0.0.249"
	}
	share := os.Getenv("SMB_TEST_SHARE")
	if share == "" {
		share = "TestFolder"
	}
	t.Setenv("STORAGE_SMB_ALLOWED_HOSTS", host)
	d, e := NewSMB(map[string]string{"server": host, "share": share, "username": v["user"], "password": v["password"]})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root := "photoly-integration-" + uuid.NewString()
	if e = d.Mkdir(ctx, root); e != nil {
		t.Fatal(e)
	}
	base := *d
	defer func() {
		if e := base.Delete(context.Background(), root); e != nil {
			t.Error("test directory cleanup failed", e)
		}
	}()
	d.directory = root
	if e = d.Check(ctx); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{"中文", "空目录"} {
		if e = d.Mkdir(ctx, p); e != nil {
			t.Fatal(e)
		}
	}
	if e = d.Put(ctx, "中文/照片 #1.jpg", strings.NewReader("first"), 5); e != nil {
		t.Fatal(e)
	}
	before, e := d.Property(ctx, "中文/照片 #1.jpg")
	if e != nil {
		t.Fatal(e)
	}
	if e = d.Put(ctx, "中文/照片 #1.jpg", strings.NewReader("overwrite"), 9); e == nil {
		t.Fatal("create overwrote existing")
	}
	if e = d.Move(ctx, "中文/照片 #1.jpg", "moved.jpg", before.Version); e != nil {
		t.Fatal(e)
	}
	moved, e := d.Property(ctx, "moved.jpg")
	if e != nil {
		t.Fatal(e)
	}
	if moved.Identity == "" || moved.Identity != before.Identity {
		t.Fatal("identity lost on rename")
	}
	if e = d.PutVersion(ctx, "moved.jpg", strings.NewReader("new"), "stale"); e == nil {
		t.Fatal("stale write succeeded")
	}
	if e = d.PutVersion(ctx, "moved.jpg", strings.NewReader("new"), moved.Version); e != nil {
		t.Fatal(e)
	}
	r, e := d.Get(ctx, "moved.jpg")
	if e != nil {
		t.Fatal(e)
	}
	data, e := io.ReadAll(r)
	r.Close()
	if e != nil || string(data) != "new" {
		t.Fatal("read mismatch", e)
	}
	if e = d.Put(ctx, "exists.jpg", strings.NewReader("keep"), 4); e != nil {
		t.Fatal(e)
	}
	if e = d.Move(ctx, "moved.jpg", "exists.jpg", ""); e == nil {
		t.Fatal("rename overwrote destination")
	}
	entries := []TreeEntry{}
	if e = d.Walk(ctx, "", func(v TreeEntry) error { entries = append(entries, v); return nil }); e != nil {
		t.Fatal(e)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}
	if e = d.Delete(ctx, "moved.jpg"); e != nil {
		t.Fatal(e)
	}
	if _, e = d.Property(ctx, "moved.jpg"); !errors.Is(e, ErrNotFound) {
		t.Fatal("delete failed", e)
	}
	d.directory = ""
}

func TestSMBFixtureIsolation(t *testing.T) {
	root := os.Getenv("SMB_FIXTURE_ROOT")
	if root == "" {
		t.Skip("SMB_FIXTURE_ROOT required")
	}
	directory, e := os.MkdirTemp(root, "safety-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(directory)
	t.Setenv("STORAGE_SMB_ALLOWED_HOSTS", "127.0.0.1")
	d, e := NewSMB(map[string]string{"server": "127.0.0.1", "share": "fixture", "subdirectory": filepath.Base(directory), "username": "photolytest", "password": "fixture-password"})
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	// Concurrent creators never replace or merge each other's original.
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, body := range []string{"first", "second"} {
		wg.Add(1)
		go func(v string) {
			defer wg.Done()
			results <- d.Put(ctx, "same.jpg", strings.NewReader(v), int64(len(v)))
		}(body)
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("expected one creator, got %d", success)
	}
	before, e := d.Property(ctx, "same.jpg")
	if e != nil {
		t.Fatal(e)
	}
	reader, e := d.Get(ctx, "same.jpg")
	if e != nil {
		t.Fatal(e)
	}
	if e = d.PutVersion(ctx, "same.jpg", strings.NewReader("conflict"), before.Version); e == nil {
		t.Error("write bypassed reader sharing guard")
	}
	reader.Close()
	outside := t.TempDir()
	if e = os.WriteFile(filepath.Join(outside, "secret"), []byte("outside"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.Symlink(outside, filepath.Join(directory, "alias")); e != nil {
		t.Fatal(e)
	}
	if _, e = d.Get(ctx, "alias/secret"); e == nil {
		t.Fatal("followed directory symlink")
	}
	if _, e = d.Property(ctx, "../outside"); e == nil {
		t.Fatal("accepted traversal")
	}
	// Never permit link-local metadata targets, even through explicit allowlisting.
	t.Setenv("STORAGE_SMB_ALLOWED_HOSTS", "169.254.169.254")
	blocked, _ := NewSMB(map[string]string{"server": "169.254.169.254", "share": "fixture"})
	if _, e = blocked.Property(ctx, ""); e == nil || !strings.Contains(e.Error(), "address_not_allowed") {
		t.Fatal("metadata target not blocked", e)
	}
}
