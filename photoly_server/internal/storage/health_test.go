package storage

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/testutil/davserver"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAccessCode(t *testing.T) {
	for _, c := range []struct {
		err  error
		want string
	}{{ErrNotFound, "file_missing"}, {os.ErrPermission, "access_denied"}, {context.DeadlineExceeded, "unreachable"}, {errors.New("webdav_request_failed_401"), "access_denied"}, {errors.New("smb_authentication_or_negotiation_failed"), "access_denied"}, {errors.New("webdav_redirect_refused_use_canonical_url"), "configuration_error"}, {errors.New("smb_dns_failed"), "unreachable"}, {nil, "available"}} {
		if got := AccessCode(c.err); got != c.want {
			t.Fatalf("%v: %s", c.err, got)
		}
	}
}
func TestHealthReadOnlyCoalescedAndRecovery(t *testing.T) {
	var reads, writes atomic.Int64
	var denied atomic.Bool
	handler := davserver.Handler(t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			writes.Add(1)
			w.WriteHeader(403)
			return
		}
		reads.Add(1)
		time.Sleep(30 * time.Millisecond)
		if denied.Load() {
			w.WriteHeader(401)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	defer srv.Close()
	t.Setenv("STORAGE_WEBDAV_ALLOWED_HOSTS", srv.URL)
	cfg, _ := json.Marshal(map[string]string{"url": srv.URL + "/"})
	source := Source{ID: 902, UserID: 17, Kind: "webdav", Enabled: true, Config: cfg}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h, e := ProbeHealth(context.Background(), nil, source, true)
			if e != nil || h.State != "available" {
				t.Errorf("%+v %v", h, e)
			}
		}()
	}
	wg.Wait()
	if reads.Load() != 1 || writes.Load() != 0 {
		t.Fatalf("reads=%d writes=%d", reads.Load(), writes.Load())
	}
	denied.Store(true)
	h, _ := ProbeHealth(context.Background(), nil, source, false)
	if h.State != "available" {
		t.Fatal("cache not used")
	}
	h, _ = ProbeHealth(context.Background(), nil, source, true)
	if h.State != "access_denied" {
		t.Fatal(h)
	}
	denied.Store(false)
	h, _ = ProbeHealth(context.Background(), nil, source, true)
	if h.State != "available" {
		t.Fatal(h)
	}
	observeAccess(source, ErrNotFound)
	h, _ = ProbeHealth(context.Background(), nil, source, false)
	if h.State != "available" {
		t.Fatal("file error poisoned source", h)
	}
	source.Enabled = false
	h, _ = ProbeHealth(context.Background(), nil, source, false)
	if h.State != "disabled" {
		t.Fatal(h)
	}
}
