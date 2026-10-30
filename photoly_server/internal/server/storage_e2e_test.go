package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
)

func s3Config(t *testing.T) map[string]any {
	t.Helper()
	ep := os.Getenv("PHOTOLY_TEST_S3_ENDPOINT")
	if ep == "" {
		t.Skip("PHOTOLY_TEST_S3_ENDPOINT required")
	}
	return map[string]any{"enabled": true, "values": map[string]any{"endpoint": ep, "bucket": os.Getenv("PHOTOLY_TEST_S3_BUCKET"), "prefix": "source-lifecycle", "access_key": os.Getenv("PHOTOLY_TEST_S3_ACCESS_KEY"), "secret_key": os.Getenv("PHOTOLY_TEST_S3_SECRET_KEY"), "use_ssl": false, "path_style": true}}
}
func TestPhotoLifecycleOnS3(t *testing.T) {
	cfg := s3Config(t)
	t.Setenv("STORAGE_SECRET_KEY", "test-key")
	f := newLibraryTest(t)
	f.ok("PUT", "/user/storage/s3", cfg)
	var ns, sid int64
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT ns_id,source_id FROM namespace WHERE user_id=$1 AND is_mount`, f.uid).Scan(&ns, &sid); e != nil {
		t.Fatal(e)
	}
	local := f.upload(f.root, "local.png")
	pid := f.upload(ns, "remote.png")
	var uuid string
	f.a.Pool.QueryRow(f.ctx, `SELECT photo_uuid FROM photo WHERE photo_id=$1`, pid).Scan(&uuid)
	source, e := storage.GetSource(f.ctx, f.a.Pool, sid)
	if e != nil {
		t.Fatal(e)
	}
	st, e := storage.SourceStore(source)
	if e != nil {
		t.Fatal(e)
	}
	key := fmt.Sprintf("%s/%d/%s/original.png", f.uuid, sid, uuid)
	if _, e = st.Stat(f.ctx, key); e != nil {
		t.Fatal("remote original missing", e)
	}
	defer st.DeletePrefix(f.ctx, f.uuid+"/")
	if usage := storage.Usage(f.a.Cfg.UploadFolder, f.uuid); usage <= 0 {
		t.Fatal("local usage missing")
	}
	var used int64
	f.a.Pool.QueryRow(f.ctx, `SELECT used_bytes FROM "user" WHERE user_id=$1`, f.uid).Scan(&used)
	if used != storage.Usage(f.a.Cfg.UploadFolder, f.uuid) {
		t.Fatal("S3 counted in local quota")
	}
	// Force a real SDK GET, not a cache hit.
	p, e := storage.Resolve(f.ctx, f.a.Pool, f.a.Cfg.UploadFolder, f.uuid, uuid, "original")
	if e != nil {
		t.Fatal(e)
	}
	os.Remove(p)
	w := f.request("GET", fmt.Sprintf("/photo/render/%s?photoId=%d", f.token, pid), "", nil)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), pngData()) {
		t.Fatalf("S3 render %d %s", w.Code, w.Body.String())
	}
	data := f.json("GET", "/user/storage/s3", nil)
	encoded, _ := json.Marshal(data)
	if strings.Contains(string(encoded), os.Getenv("PHOTOLY_TEST_S3_SECRET_KEY")) {
		t.Fatal("secret leaked")
	}
	// Two sources remain visible; cross-source operations cannot relocate objects.
	if v := f.json("POST", fmt.Sprintf("/photo/changeNamespace?photoId=%d&nsId=%d", pid, f.root), nil); v["msgCode"] == float64(200) {
		t.Fatal("cross-source move accepted")
	}
	f.ok("PUT", "/user/storage/s3", map[string]any{"enabled": false})
	var visible bool
	f.a.Pool.QueryRow(f.ctx, `SELECT photo_visible(storage_id,user_id) FROM photo WHERE photo_id=$1`, pid).Scan(&visible)
	if visible {
		t.Fatal("disabled remote visible")
	}
	if w = f.request("GET", fmt.Sprintf("/photo/render/%s?photoId=%d", f.token, local), "", nil); w.Code != 200 {
		t.Fatal("local hidden by S3 disable")
	}
	f.ok("PUT", "/user/storage/s3", map[string]any{"enabled": true})
	cfg["values"].(map[string]any)["prefix"] = "different"
	if v := f.json("PUT", "/user/storage/s3", cfg); v["msgCode"] != float64(409) {
		t.Fatal("occupied source retargeted", v)
	}
	// Renaming a remote folder and photo does not alter object keys.
	folder := f.folder(ns, "remote folder")
	f.ok("POST", "/namespace/updateName", map[string]any{"nsId": folder, "nsName": "renamed"})
	f.ok("POST", fmt.Sprintf("/photo/changeNamespace?photoId=%d&nsId=%d", pid, folder), nil)
	f.ok("POST", "/photo/update", map[string]any{"photoId": pid, "photoName": "new name"})
	if _, e = st.Stat(f.ctx, key); e != nil {
		t.Fatal("remote rename changed key", e)
	}
	w = f.request("GET", fmt.Sprintf("/namespace/downloadNs/%s?nsId=%d", f.token, f.root), "", nil)
	zr, e := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if e != nil {
		t.Fatalf("combined ZIP: %s %v", w.Body.String(), e)
	}
	names := map[string]bool{}
	for _, entry := range zr.File {
		names[entry.Name] = true
	}
	if !names["root/local.png"] || !names["root/S3/renamed/new name.png"] {
		t.Fatalf("combined source ZIP: %v", names)
	}
	f.ok("POST", fmt.Sprintf("/photo/delete?photoId=%d", pid), nil)
	if _, e = st.Stat(f.ctx, key); e != storage.ErrNotFound {
		t.Fatal("object remained after delete", e)
	}
	if _, e = os.Stat(filepath.Join(f.dir, "local.png")); e != nil {
		t.Fatal(e)
	}
}

func TestPersonalS3UserIsolation(t *testing.T) {
	t.Setenv("STORAGE_SECRET_KEY", "test-key")
	cfg := s3Config(t)
	first, second := newLibraryTest(t), newLibraryTest(t)
	first.ok("PUT", "/user/storage/s3", cfg)
	cfg["values"].(map[string]any)["prefix"] = "second-account"
	second.ok("PUT", "/user/storage/s3", cfg)
	for _, f := range []*libraryTest{first, second} {
		var ns, sid int64
		if e := f.a.Pool.QueryRow(f.ctx, `SELECT ns_id,source_id FROM namespace WHERE user_id=$1 AND is_mount`, f.uid).Scan(&ns, &sid); e != nil {
			t.Fatal(e)
		}
		id := f.upload(ns, "same.png")
		if v := first.json("POST", fmt.Sprintf("/photo/changeNamespace?photoId=%d&nsId=%d", id, first.root), nil); f == second && v["msgCode"] == float64(200) {
			t.Fatal("cross-user move accepted")
		}
		source, e := storage.GetSource(f.ctx, f.a.Pool, sid)
		if e != nil {
			t.Fatal(e)
		}
		st, e := storage.SourceStore(source)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { _ = st.DeletePrefix(f.ctx, f.uuid+"/") })
		if e := os.Mkdir(filepath.Join(f.dir, "S3"), 0755); e != nil {
			t.Fatal(e)
		}
		f.scan()
		var name string
		if e := f.a.Pool.QueryRow(f.ctx, `SELECT ns_name FROM namespace WHERE ns_id=$1`, ns).Scan(&name); e != nil || name != "S3 (1)" {
			t.Fatalf("external mount name collision: %q %v", name, e)
		}
	}
	first.ok("PUT", "/user/storage/s3", map[string]any{"enabled": false})
	var n int
	if e := second.a.Pool.QueryRow(second.ctx, `SELECT count(*) FROM photo WHERE user_id=$1 AND photo_visible(storage_id,user_id)`, second.uid).Scan(&n); e != nil || n != 1 {
		t.Fatalf("one user's settings affected another: %d %v", n, e)
	}
}

func itoa(n int64) string { return fmt.Sprint(n) }
