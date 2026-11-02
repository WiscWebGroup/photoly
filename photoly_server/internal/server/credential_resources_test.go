package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/testutil/davserver"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/photo"
)

func credExec(f *libraryTest, query string, args ...any) {
	f.t.Helper()
	if _, err := f.a.Pool.Exec(f.ctx, query, args...); err != nil {
		f.t.Fatal(err)
	}
}
func credRequest(f *libraryTest, method, path, token, ctype string, body io.Reader) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, body)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if ctype != "" {
		r.Header.Set("Content-Type", ctype)
	}
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, r)
	return w
}
func TestCredentialMediaAndLegacyAliases(t *testing.T) {
	f := newLibraryTest(t)
	q := dbgen.New(f.a.Pool)
	token := photo.NewUUID()
	if e := q.InsertCred(f.ctx, dbgen.InsertCredParams{UserID: f.uid, Token: token, Auth: "CRD"}); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		multiple bool
		path     string
	}{{false, "/cred/uploadPhoto"}, {true, "/cred/uploadPhotos"}} {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		field, key := "file", "photo"
		desc := fmt.Sprintf(`{"nsId":%d,"visibility":0}`, f.root)
		if tc.multiple {
			field, key = "files", "photosStr"
			desc = "[" + desc + "]"
		}
		part, _ := mw.CreateFormFile(field, "sample.png")
		part.Write(pngData())
		mw.WriteField(key, desc)
		mw.Close()
		w := credRequest(f, "POST", tc.path, token, mw.FormDataContentType(), &buf)
		if w.Code != 200 || w.Body.Len() == 0 {
			t.Fatalf("upload %d %s", w.Code, w.Body.String())
		}
	}
	pid := f.upload(f.root, "private.png")
	var uuid string
	f.a.Pool.QueryRow(f.ctx, `SELECT photo_uuid FROM photo WHERE photo_id=$1`, pid).Scan(&uuid)
	f.ok("POST", "/photo/notes", map[string]any{"photoId": pid, "body": "private note must not leak"})
	f.ok("PUT", "/photo/category", map[string]any{"photoId": pid, "category": "panorama", "included": true})
	for _, path := range []string{"/cred/queryRootNamespace", fmt.Sprintf("/cred/queryNamespaces?parentId=%d", f.root), fmt.Sprintf("/cred/queryPhotoList?nsId=%d", f.root), "/cred/media?limit=1", "/cred/media/" + uuid, "/cred/categories", "/cred/categories/panorama/media"} {
		w := credRequest(f, "GET", path, token, "", nil)
		if w.Code != 200 || !json.Valid(w.Body.Bytes()) {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		if strings.Contains(path, "/cred/media") || strings.Contains(path, "/cred/categories") {
			for _, secret := range []string{"private note", "photoToken", "photo_token", "source_id", "file_name"} {
				if strings.Contains(w.Body.String(), secret) {
					t.Fatal("private field", secret)
				}
			}
		}
	}
	first := credRequest(f, "GET", "/cred/media?limit=1", token, "", nil)
	var page struct {
		T struct {
			Items     []map[string]any
			NextAfter int64
		}
	}
	json.Unmarshal(first.Body.Bytes(), &page)
	if len(page.T.Items) != 1 || page.T.NextAfter == 0 {
		t.Fatal(first.Body.String())
	}
	next := credRequest(f, "GET", fmt.Sprintf("/cred/media?limit=1&after=%d", page.T.NextAfter), token, "", nil)
	if next.Body.String() == first.Body.String() {
		t.Fatal("cursor did not advance")
	}
	for _, path := range []string{"/cred/render?uuid=" + uuid, "/cred/render/" + token + "?uuid=" + uuid, "/cred/media/" + uuid + "/thumbnail"} {
		w := credRequest(f, "GET", path, token, "", nil)
		if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "image/") {
			t.Fatalf("binary %s: %d %s", path, w.Code, w.Body.String())
		}
	}
	r := httptest.NewRequest("GET", "/cred/render?uuid="+uuid, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Range", "bytes=0-7")
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, r)
	if w.Code != 206 || w.Body.Len() != 8 || w.Header().Get("Content-Type") != "image/png" {
		t.Fatal(w.Code, w.Header())
	}
	other := newLibraryTest(t)
	otherID := other.upload(other.root, "other.png")
	var otherUUID string
	other.a.Pool.QueryRow(other.ctx, `SELECT photo_uuid FROM photo WHERE photo_id=$1`, otherID).Scan(&otherUUID)
	for _, path := range []string{"/cred/media/" + otherUUID, fmt.Sprintf("/cred/media?nsId=%d", other.root)} {
		if w := credRequest(f, "GET", path, token, "", nil); w.Code != 404 {
			t.Fatal("ownership", w.Code, w.Body.String())
		}
	}
	for _, bad := range []string{"", "invalid", f.token} {
		if w := credRequest(f, "GET", "/cred/media", bad, "", nil); w.Code != 401 {
			t.Fatal("accepted non API token", w.Code)
		}
	}
	for _, path := range []string{"/cred/media?limit=201", "/cred/media?after=-1", "/cred/media?limit=0"} {
		if w := credRequest(f, "GET", path, token, "", nil); w.Code != 400 {
			t.Fatal(w.Code, path)
		}
	}
	for _, auth := range []string{"C", "D", "CD"} {
		credExec(f, `UPDATE cred SET "authorization"=$1 WHERE token=$2`, auth, token)
		if w := credRequest(f, "GET", "/cred/media", token, "", nil); w.Code != 403 {
			t.Fatal(auth, w.Code)
		}
	}
	credExec(f, `UPDATE cred SET "authorization"='R' WHERE token=$1`, token)
	if w := credRequest(f, "POST", "/cred/deletePhoto?uuid="+uuid, token, "", nil); w.Code != 403 {
		t.Fatal("read token deleted", w.Code)
	}
	credExec(f, `UPDATE cred SET "authorization"='D' WHERE token=$1`, token)
	w = credRequest(f, "POST", "/cred/deletePhoto", token, "application/x-www-form-urlencoded", strings.NewReader(url.Values{"uuid": {uuid}}.Encode()))
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != "1" {
		t.Fatal(w.Code, w.Body.String())
	}
	var n int
	f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM media_note WHERE photo_id=$1`, pid).Scan(&n)
	if n != 0 {
		t.Fatal("orphan notes")
	}
	credExec(f, `DELETE FROM cred WHERE token=$1`, token)
	if w := credRequest(f, "GET", "/cred/media", token, "", nil); w.Code != 401 {
		t.Fatal("revoked token accepted")
	}
}
func TestAdminResourceHistoryAndRuntime(t *testing.T) {
	f := newLibraryTest(t)
	if got := f.json("GET", "/admin/runtime", nil)["msgCode"]; got == float64(200) {
		t.Fatal("ordinary user can read runtime")
	}
	credExec(f, `UPDATE "user" SET role='admin' WHERE user_id=$1`, f.uid)
	runtime := f.ok("GET", "/admin/runtime", nil).(map[string]any)
	if runtime["bootId"] == "" || runtime["uptimeSeconds"].(float64) < 0 {
		t.Fatal(runtime)
	}
	ctx, cancel := context.WithCancel(f.ctx)
	done := make(chan struct{})
	go func() { defer close(done); f.a.ResourceRun(ctx) }()
	for start := time.Now(); ; {
		v := f.ok("GET", "/admin/resources/current", nil).(map[string]any)
		if v["backendBytes"] != nil {
			break
		}
		if time.Since(start) > time.Second {
			t.Fatal(v)
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done
	h := f.ok("GET", "/admin/resources/history?hours=24", nil).(map[string]any)
	if len(h["items"].([]any)) == 0 {
		t.Fatal(h)
	}
	q := dbgen.New(f.a.Pool)
	old := time.Now().Add(-49 * time.Hour)
	q.SaveResourceHistory(f.ctx, dbgen.SaveResourceHistoryParams{BootID: "expired-test", Bucket: old, Payload: []byte(`{}`)})
	q.PruneResourceHistory(f.ctx, time.Now().Add(-48*time.Hour))
	var n int
	f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM resource_history WHERE boot_id='expired-test'`).Scan(&n)
	if n != 0 {
		t.Fatal("retention")
	}
	// A fresh App reads the previous boot's persisted sample, without starting a worker.
	other := newLibraryTest(t)
	credExec(other, `UPDATE "user" SET role='admin' WHERE user_id=$1`, other.uid)
	h = other.ok("GET", "/admin/resources/history?hours=12", nil).(map[string]any)
	found := false
	for _, v := range h["items"].([]any) {
		if v.(map[string]any)["bootId"] == f.a.BootID {
			found = true
		}
	}
	if !found {
		t.Fatal("lost history across restart")
	}
	if w := f.request("GET", "/admin/resources/history?hours=48", "", nil); w.Code != 400 {
		t.Fatal(w.Code)
	}
	credExec(f, `DELETE FROM resource_history WHERE boot_id=$1`, f.a.BootID)
}

func TestCredentialRemoteMediaFailureAndRecovery(t *testing.T) {
	f := newLibraryTest(t)
	dir := t.TempDir()
	var offline atomic.Bool
	dav := davserver.Handler(dir)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if offline.Load() {
			w.WriteHeader(503)
			return
		}
		dav.ServeHTTP(w, r)
	}))
	defer remote.Close()
	t.Setenv("STORAGE_WEBDAV_ALLOWED_HOSTS", remote.URL)
	t.Setenv("STORAGE_SECRET_KEY", strings.Repeat("a", 64))
	source := f.ok("POST", "/user/storage/webdav", map[string]any{"name": "API remote", "enabled": true, "url": remote.URL + "/", "username": "test", "password": "test", "scanInterval": 60}).(map[string]any)
	sid := int64(source["sourceId"].(float64))
	var mount int64
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT ns_id FROM namespace WHERE source_id=$1 AND is_mount`, sid).Scan(&mount); e != nil {
		t.Fatal(e)
	}
	pid := f.upload(mount, "remote.png")
	var uuid string
	f.a.Pool.QueryRow(f.ctx, `SELECT photo_uuid FROM photo WHERE photo_id=$1`, pid).Scan(&uuid)
	token := photo.NewUUID()
	if e := dbgen.New(f.a.Pool).InsertCred(f.ctx, dbgen.InsertCredParams{UserID: f.uid, Token: token, Auth: "RD"}); e != nil {
		t.Fatal(e)
	}
	if w := credRequest(f, "GET", "/cred/render?uuid="+uuid, token, "", nil); w.Code != 200 || w.Header().Get("Content-Type") != "image/png" {
		t.Fatal(w.Code, w.Body.String())
	}
	offline.Store(true)
	if w := credRequest(f, "POST", "/cred/deletePhoto?uuid="+uuid, token, "", nil); w.Code < 400 {
		t.Fatal("failed remote delete looked successful", w.Code, w.Body.String())
	}
	var exists bool
	f.a.Pool.QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM photo WHERE photo_id=$1)`, pid).Scan(&exists)
	if !exists {
		t.Fatal("failed delete removed index")
	}
	if _, e := os.Stat(filepath.Join(dir, "remote.png")); e != nil {
		t.Fatal(e)
	}
	if w := credRequest(f, "GET", "/cred/media/"+uuid, token, "", nil); w.Code != 200 {
		t.Fatal("offline source lost indexed metadata", w.Code)
	}
	offline.Store(false)
	if w := credRequest(f, "GET", "/cred/render?uuid="+uuid, token, "", nil); w.Code != 200 {
		t.Fatal("recovery", w.Code)
	}
	credExec(f, `UPDATE storage_source SET enabled=false WHERE id=$1`, sid)
	for _, path := range []string{"/cred/media/" + uuid, "/cred/media/" + uuid + "/thumbnail"} {
		if w := credRequest(f, "GET", path, token, "", nil); w.Code != 404 {
			t.Fatal("disabled source exposed", path, w.Code)
		}
	}
}
