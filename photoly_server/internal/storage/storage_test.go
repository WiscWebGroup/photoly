package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// mem is an in-memory remote Store.
type mem struct {
	mu sync.Mutex
	m  map[string][]byte
}

func newMem() *mem { return &mem{m: map[string][]byte{}} }

func (s *mem) Put(_ context.Context, k string, r io.Reader, _ int64) error {
	b, err := io.ReadAll(r)
	s.mu.Lock()
	s.m[k] = b
	s.mu.Unlock()
	return err
}
func (s *mem) Get(_ context.Context, k string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.m[k]
	if !ok {
		return nil, ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (s *mem) Stat(_ context.Context, k string) (Info, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.m[k]
	if !ok {
		return Info{}, ErrNotFound
	}
	return Info{Size: int64(len(b))}, nil
}
func (s *mem) Delete(_ context.Context, k string) error {
	s.mu.Lock()
	delete(s.m, k)
	s.mu.Unlock()
	return nil
}
func (s *mem) DeletePrefix(_ context.Context, p string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.m {
		if strings.HasPrefix(k, p) {
			delete(s.m, k)
		}
	}
	return nil
}
func (s *mem) Check(context.Context) error { return nil }

// contract is the behaviour every Store must share; the S3 test below reuses it.
func contract(t *testing.T, st Store) {
	t.Helper()
	ctx := context.Background()
	if _, err := st.Get(ctx, "u1/missing"); err != ErrNotFound {
		t.Fatalf("Get missing = %v, want ErrNotFound", err)
	}
	if _, err := st.Stat(ctx, "u1/missing"); err != ErrNotFound {
		t.Fatalf("Stat missing = %v, want ErrNotFound", err)
	}
	if err := st.Put(ctx, "u1/a.jpg", strings.NewReader("hello"), 5); err != nil {
		t.Fatal(err)
	}
	if err := st.Put(ctx, "u1/b.jpg", strings.NewReader("x"), 1); err != nil {
		t.Fatal(err)
	}
	if info, err := st.Stat(ctx, "u1/a.jpg"); err != nil || info.Size != 5 {
		t.Fatalf("Stat = %v, %v", info, err)
	}
	rc, err := st.Get(ctx, "u1/a.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := io.ReadAll(rc); string(b) != "hello" {
		t.Fatalf("Get = %q", b)
	}
	rc.Close()
	if err = st.Delete(ctx, "u1/a.jpg"); err != nil {
		t.Fatal(err)
	}
	if err = st.Delete(ctx, "u1/a.jpg"); err != nil {
		t.Fatalf("Delete of missing key must not fail: %v", err)
	}
	if err = st.DeletePrefix(ctx, "u1/"); err != nil {
		t.Fatal(err)
	}
	if _, err = st.Stat(ctx, "u1/b.jpg"); err != ErrNotFound {
		t.Fatalf("DeletePrefix left b.jpg: %v", err)
	}
	if err = st.Check(ctx); err != nil {
		t.Fatalf("Check: %v", err)
	}
}

func TestLocalContract(t *testing.T) { contract(t, Local{Dir: t.TempDir()}) }

func TestLocalRejectsTraversal(t *testing.T) {
	if err := (Local{Dir: t.TempDir()}).Put(context.Background(), "../x", strings.NewReader("x"), 1); err == nil {
		t.Fatal("expected invalid key error")
	}
}

// TestS3Contract runs against a real endpoint, e.g.
//
//	docker run -p 9000:9000 minio/minio server /data   (bucket must exist)
//	PHOTOLY_TEST_S3_ENDPOINT=localhost:9000 PHOTOLY_TEST_S3_BUCKET=photoly \
//	PHOTOLY_TEST_S3_ACCESS_KEY=minioadmin PHOTOLY_TEST_S3_SECRET_KEY=minioadmin go test ./internal/storage
func TestS3Contract(t *testing.T) {
	ep := os.Getenv("PHOTOLY_TEST_S3_ENDPOINT")
	if ep == "" {
		t.Skip("PHOTOLY_TEST_S3_ENDPOINT not set")
	}
	st, err := Build("s3", map[string]string{
		"endpoint": ep, "bucket": os.Getenv("PHOTOLY_TEST_S3_BUCKET"), "prefix": "contract-test",
		"access_key": os.Getenv("PHOTOLY_TEST_S3_ACCESS_KEY"), "secret_key": os.Getenv("PHOTOLY_TEST_S3_SECRET_KEY"),
		"use_ssl": "false", "path_style": "true",
	})
	if err != nil {
		t.Fatal(err)
	}
	contract(t, st)
}

func TestBuildValidation(t *testing.T) {
	if _, err := Build("s3", map[string]string{"bucket": "b"}); err == nil || !strings.Contains(err.Error(), "Endpoint") {
		t.Fatalf("want required-field error, got %v", err)
	}
	if _, err := Build("nope", nil); err == nil {
		t.Fatal("want unknown driver error")
	}
}

func TestSealRoundTrip(t *testing.T) {
	t.Setenv("STORAGE_SECRET_KEY", "k1")
	sealed, err := seal([]byte("s3cret"))
	if err != nil || bytes.Contains(sealed, []byte("s3cret")) {
		t.Fatalf("seal = %q, %v", sealed, err)
	}
	if b, err := open(sealed); err != nil || string(b) != "s3cret" {
		t.Fatalf("open = %q, %v", b, err)
	}
	t.Setenv("STORAGE_SECRET_KEY", "other")
	if _, err := open(sealed); err == nil {
		t.Fatal("wrong key must fail")
	}
	t.Setenv("STORAGE_SECRET_KEY", "")
	if _, err := seal([]byte("x")); err != ErrNoSecretKey {
		t.Fatalf("seal without key = %v", err)
	}
}

func TestPublishFetch(t *testing.T) {
	root := t.TempDir()
	Init(root)
	remote := newMem()
	Set("s3", remote)
	t.Cleanup(func() { Set("local", Local{Dir: root}) })

	write := func(rel, body string) string {
		p := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	// Publish uploads and drops the local copy.
	p := write("u1/a.jpg", "v1")
	Publish(nil, []string{p})
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("local copy should be offloaded")
	}
	if string(remote.m["u1/a.jpg"]) != "v1" {
		t.Fatalf("remote = %q", remote.m["u1/a.jpg"])
	}

	// Size answers from the remote without downloading; Fetch brings it back.
	if n, err := Size(p); err != nil || n != 2 {
		t.Fatalf("Size = %d, %v", n, err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("Size must not download")
	}
	if !Fetch(p) {
		t.Fatal("Fetch failed")
	}
	if b, _ := os.ReadFile(p); string(b) != "v1" {
		t.Fatalf("fetched %q", b)
	}

	// A replaced path appears in both lists: the new bytes must survive the delete.
	write("u1/a.jpg", "v2")
	Publish([]string{p}, []string{p})
	if string(remote.m["u1/a.jpg"]) != "v2" {
		t.Fatalf("replace lost the new object: %q", remote.m["u1/a.jpg"])
	}

	// Removal deletes remotely; scratch dirs and nested paths are never published.
	Publish([]string{p}, nil)
	if _, ok := remote.m["u1/a.jpg"]; ok {
		t.Fatal("remote object should be deleted")
	}
	staged := write(".staging/tmp/x", "x")
	nested := write("u1/sub/y.jpg", "y")
	Publish(nil, []string{staged, nested})
	if len(remote.m) != 0 {
		t.Fatalf("unexpected uploads: %v", remote.m)
	}

	// Missing everywhere.
	if Fetch(filepath.Join(root, "u1", "nope.jpg")) {
		t.Fatal("Fetch of unknown file should be false")
	}
	if _, err := Size(filepath.Join(root, "u1", "nope.jpg")); !os.IsNotExist(err) {
		t.Fatalf("Size missing = %v", err)
	}

	// Deleting a user removes their prefix.
	remote.m["u2/a.jpg"], remote.m["u3/a.jpg"] = []byte("1"), []byte("1")
	DeleteUser("u2")
	if _, ok := remote.m["u2/a.jpg"]; ok || len(remote.m) != 1 {
		t.Fatalf("DeleteUser: %v", remote.m)
	}
}

func TestLocalActiveIsNoop(t *testing.T) {
	root := t.TempDir()
	Init(root)
	Set("local", Local{Dir: root})
	p := filepath.Join(root, "u1", "a.jpg")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("x"), 0o644)
	Publish(nil, []string{p})
	if !Fetch(p) {
		t.Fatal("local file must stay in place when no remote is active")
	}
}

func TestEvict(t *testing.T) {
	root := t.TempDir()
	Init(root)
	remote := newMem()
	Set("s3", remote)
	t.Cleanup(func() { Set("local", Local{Dir: root}) })
	ctx := context.Background()

	write := func(rel, body string, age time.Duration) string {
		p := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
		old := time.Now().Add(-age)
		os.Chtimes(p, old, old)
		return p
	}
	remote.m["u1/idle.jpg"] = []byte("abc")
	remote.m["u1/fresh.jpg"] = []byte("abc")
	remote.m["u1/mismatch.jpg"] = []byte("different")
	idle := write("u1/idle.jpg", "abc", 2*time.Hour)
	fresh := write("u1/fresh.jpg", "abc", time.Minute)
	mismatch := write("u1/mismatch.jpg", "abc", 2*time.Hour)
	unpublished := write("u1/unpublished.jpg", "abc", 2*time.Hour)
	used := write("u1/used.jpg", "abc", 2*time.Hour)
	remote.m["u1/used.jpg"] = []byte("abc")
	touch(used)

	if n := evict(ctx, time.Hour); n != 1 {
		t.Fatalf("evicted %d, want 1", n)
	}
	for p, want := range map[string]bool{idle: false, fresh: true, mismatch: true, unpublished: true, used: true} {
		if got := exists(p); got != want {
			t.Errorf("%s exists = %v, want %v", filepath.Base(p), got, want)
		}
	}
	if !Fetch(idle) { // an evicted file comes back on demand
		t.Fatal("evicted file should refetch")
	}
}
