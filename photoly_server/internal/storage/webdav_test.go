package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/testutil/davserver"
)

func testDAV(t *testing.T) (*DAV, string) {
	t.Helper()
	root := t.TempDir()
	handler := davserver.Handler(root)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "user" || p != "password" {
			w.WriteHeader(401)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	t.Setenv("STORAGE_WEBDAV_ALLOWED_HOSTS", server.URL)
	d, e := NewDAV(map[string]string{"url": server.URL + "/", "username": "user", "password": "password"})
	if e != nil {
		t.Fatal(e)
	}
	return d, root
}
func TestDAVTreeProtocol(t *testing.T) {
	d, root := testDAV(t)
	ctx := context.Background()
	if e := d.Check(ctx); e != nil {
		t.Fatal(e)
	}
	if e := d.Mkdir(ctx, "中文 & #"); e != nil {
		t.Fatal(e)
	}
	key := "中文 & #/file + %.txt"
	if e := d.Put(ctx, key, strings.NewReader("first"), 5); e != nil {
		t.Fatal(e)
	}
	if e := d.Put(ctx, key, strings.NewReader("overwrite"), 9); e == nil {
		t.Fatal("overwrote existing file")
	}
	info, e := d.Property(ctx, key)
	if e != nil {
		t.Fatal(e)
	}
	if info.Size != 5 || info.Version == "" {
		t.Fatalf("stat: %+v", info)
	}
	if e = d.PutVersion(ctx, key, strings.NewReader("stale"), `"stale"`); e == nil {
		t.Fatal("accepted stale replacement")
	}
	if e = d.PutVersion(ctx, key, strings.NewReader("second"), info.Version); e != nil {
		t.Fatal(e)
	}
	if e = d.Move(ctx, key, "中文 & #/renamed.txt", ""); e != nil {
		t.Fatal(e)
	}
	entries, e := d.List(ctx, "中文 & #")
	if e != nil || len(entries) != 1 {
		t.Fatalf("list: %+v %v", entries, e)
	}
	if e = d.Mkdir(ctx, SystemDirectory); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, SystemDirectory, "private"), []byte("private"), 0600); e != nil {
		t.Fatal(e)
	}
	count := 0
	if e = d.Walk(ctx, "", func(v TreeEntry) error {
		if strings.Contains(v.Path, SystemDirectory) {
			t.Fatal("workspace exposed")
		}
		count++
		return nil
	}); e != nil || count != 2 {
		t.Fatalf("walk %d: %v", count, e)
	}
	r, e := d.Get(ctx, "中文 & #/renamed.txt")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := io.ReadAll(r)
	r.Close()
	if string(b) != "second" {
		t.Fatal(string(b))
	}
	if e = d.Delete(ctx, "中文 & #"); e != nil {
		t.Fatal(e)
	}
}
func TestDAVRejectsEscapingHrefs(t *testing.T) {
	d, e := NewDAV(map[string]string{"url": "https://example.com/dav/photos/"})
	if e != nil {
		t.Fatal(e)
	}
	for _, href := range []string{"https://evil.test/dav/photos/a", "/dav/other/a", "/dav/photos/%2e%2e/secret", "/dav/photos/%252e%252e/secret", "/dav/photos/a%2fb/../../secret", "/dav/photos/file?token=secret"} {
		if _, e = d.relative(href); e == nil {
			t.Errorf("accepted %q", href)
		}
	}
}
func TestDAVPartialListingAndRedirect(t *testing.T) {
	for _, mode := range []string{"partial", "redirect"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if mode == "redirect" {
					w.Header().Set("Location", "http://169.254.169.254/")
					w.WriteHeader(302)
					return
				}
				w.WriteHeader(207)
				_, _ = io.WriteString(w, `<d:multistatus xmlns:d="DAV:"><d:response><d:href>/</d:href><d:propstat><d:prop><d:resourcetype><d:collection/></d:resourcetype></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response><d:response><d:href>/private/</d:href><d:status>HTTP/1.1 403 Forbidden</d:status></d:response></d:multistatus>`)
			}))
			defer server.Close()
			t.Setenv("STORAGE_WEBDAV_ALLOWED_HOSTS", server.URL)
			d, e := NewDAV(map[string]string{"url": server.URL})
			if e != nil {
				t.Fatal(e)
			}
			if _, e = d.List(context.Background(), ""); e == nil {
				t.Fatal("accepted unsafe response")
			}
		})
	}
}
func TestExternalWorkspaceCompensation(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	t.Setenv("STORAGE_LOCAL_ROOTS", `[{"id":"nas","name":"NAS","path":"`+outside+`"}]`)
	f, e := NewFiles(root)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	p := filepath.Join(outside, "file.txt")
	if e = os.WriteFile(p, []byte("old"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = f.Install(strings.NewReader("new"), p); e != nil {
		t.Fatal(e)
	}
	if len(f.extraWork) == 0 || !Within(outside, f.extraWork[0]) {
		t.Fatal("backup on wrong filesystem")
	}
	f.Rollback()
	b, e := os.ReadFile(p)
	if e != nil || string(b) != "old" {
		t.Fatalf("rollback %q %v", b, e)
	}
}
func TestDAVCompensation(t *testing.T) {
	d, _ := testDAV(t)
	ctx := context.Background()
	if e := d.Put(ctx, "old.txt", strings.NewReader("old"), 3); e != nil {
		t.Fatal(e)
	}
	f, e := NewFiles(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if e = f.TreeRemove(ctx, d, "old.txt", ""); e != nil {
		t.Fatal(e)
	}
	if _, e = d.Property(ctx, "old.txt"); e != ErrNotFound {
		t.Fatalf("still exists: %v", e)
	}
	f.Rollback()
	r, e := d.Get(ctx, "old.txt")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := io.ReadAll(r)
	r.Close()
	if string(b) != "old" {
		t.Fatal(string(b))
	}
}

func TestDAVDeploymentNextcloud(t *testing.T) {
	endpoint := os.Getenv("PHOTOLY_TEST_WEBDAV_URL")
	if endpoint == "" {
		t.Skip("PHOTOLY_TEST_WEBDAV_URL required")
	}
	d, e := NewDAV(map[string]string{"url": endpoint, "username": os.Getenv("PHOTOLY_TEST_WEBDAV_USERNAME"), "password": os.Getenv("PHOTOLY_TEST_WEBDAV_PASSWORD")})
	if e != nil {
		t.Fatal(e)
	}
	if e = d.Check(context.Background()); e != nil {
		t.Fatal(e)
	}
}

func TestResolvedDirectoryRejectsSymlinkReplacement(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	dir := filepath.Join(root, "child")
	if e := os.Mkdir(dir, 0700); e != nil {
		t.Fatal(e)
	}
	if _, e := StrictPath(dir, "file"); e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(dir); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(outside, dir); e != nil {
		t.Fatal(e)
	}
	if _, e := StrictPath(dir, "file"); e == nil {
		t.Fatal("resolved directory followed replacement symlink")
	}
}
func TestDAVPrivateOriginsRequireDeploymentAllowlist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("unapproved private request reached server") }))
	defer server.Close()
	t.Setenv("STORAGE_WEBDAV_ALLOWED_HOSTS", "")
	d, e := NewDAV(map[string]string{"url": server.URL})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = d.Property(context.Background(), ""); e == nil {
		t.Fatal("allowed private destination")
	}
}
