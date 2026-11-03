package server

import (
	"bytes"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/photo"
)

func TestShareLinksFollowBrowserAndFetchRemoteOriginal(t *testing.T) {
	for _, kind := range []string{"local", "webdav"} {
		t.Run(kind, func(t *testing.T) {
			f := newLibraryTest(t)
			ns := f.root
			if kind == "webdav" {
				_, endpoint, input := externalBinding(t, f, kind)
				created := f.ok("POST", endpoint, input).(map[string]any)
				sid := int64(created["sourceId"].(float64))
				if err := f.a.Pool.QueryRow(f.ctx, `SELECT ns_id FROM namespace WHERE source_id=$1 AND is_mount`, sid).Scan(&ns); err != nil {
					t.Fatal(err)
				}
			}
			pid := f.upload(ns, "shared.png")
			f.ok("POST", "/photo/update", map[string]any{"photoId": pid, "visibility": 1})
			base := "https://photos.example.test/photoly/api"
			link := f.ok("GET", fmt.Sprintf("/photo/getPath?photoId=%d&shareBase=%s", pid, url.QueryEscape(base)), nil).(string)
			if !strings.HasPrefix(link, base+"/photo/renderToken?") || strings.Contains(link, f.token) {
				t.Fatal("incorrect share address")
			}
			parsed, _ := url.Parse(link)
			route := "/photo/renderToken?" + parsed.RawQuery
			// Remove only the disposable fixture's downloaded copies: first anonymous access must fetch.
			if kind == "webdav" {
				if err := os.RemoveAll(filepath.Join(f.a.Cfg.UploadFolder, ".remote")); err != nil {
					t.Fatal(err)
				}
			}
			get := func(path string) *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				f.h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
				return w
			}
			w := get(route)
			if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "image/") {
				t.Fatalf("anonymous %s: %d %s", kind, w.Code, w.Body.String())
			}
			qr := f.request("GET", fmt.Sprintf("/photo/getQR2?photoId=%d&token=%s&shareBase=%s", pid, url.QueryEscape(f.token), url.QueryEscape(base)), "", nil)
			expected, e := photo.QRPNG(link)
			if e != nil || !bytes.Equal(expected, qr.Body.Bytes()) {
				t.Fatal("QR and link differ")
			}
			bad := parsed.Query()
			bad.Set("token", "wrong")
			if get("/photo/renderToken?"+bad.Encode()).Code == 200 {
				t.Fatal("invalid token accepted")
			}
			f.ok("POST", "/photo/update", map[string]any{"photoId": pid, "visibility": 0})
			if get(route).Code == 200 {
				t.Fatal("disabled sharing accepted")
			}
			if _, err := photo.For(f.a).ShareURL(f.ctx, f.uid, pid, "javascript:alert(1)"); err == nil {
				t.Fatal("unsafe base accepted")
			}
		})
	}
}
