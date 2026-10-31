package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
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
