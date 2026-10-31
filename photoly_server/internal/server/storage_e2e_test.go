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
		var alias string
		if e := f.a.Pool.QueryRow(f.ctx, `SELECT display_name FROM storage_source WHERE id=$1`, sid).Scan(&alias); e != nil || alias != name {
			t.Fatal("mount alias diverged", alias, name, e)
		}
	}
	first.ok("PUT", "/user/storage/s3", map[string]any{"enabled": false})
	var n int
	if e := second.a.Pool.QueryRow(second.ctx, `SELECT count(*) FROM photo WHERE user_id=$1 AND photo_visible(storage_id,user_id)`, second.uid).Scan(&n); e != nil || n != 1 {
		t.Fatalf("one user's settings affected another: %d %v", n, e)
	}
}

func itoa(n int64) string { return fmt.Sprint(n) }

func TestMultipleS3Connections(t *testing.T) {
	cfg := s3Config(t)
	t.Setenv("STORAGE_SECRET_KEY", "test-key")
	f := newLibraryTest(t)
	f.folder(f.root, "S3")
	first := f.ok("POST", "/user/storage/s3", cfg).(map[string]any)
	second := f.ok("POST", "/user/storage/s3", cfg).(map[string]any)
	id1, id2 := int64(first["sourceId"].(float64)), int64(second["sourceId"].(float64))
	if first["name"] != "S3 (1)" || second["name"] != "S3 (2)" || id1 == id2 {
		t.Fatal(first, second)
	}
	endpoint := func(id int64) string { return fmt.Sprintf("/user/storage/s3/%d", id) }
	list := f.ok("GET", "/user/storage/s3", nil).(map[string]any)["connections"].([]any)
	if len(list) != 2 {
		t.Fatal(list)
	}
	for _, v := range list {
		values := v.(map[string]any)["values"].(map[string]any)
		if values["access_key"] != nil || values["secret_key"] != nil {
			t.Fatal("credentials returned")
		}
	}
	if v := f.json("PUT", "/user/storage/s3", map[string]any{"enabled": false}); v["msgCode"] != float64(409) {
		t.Fatal("ambiguous legacy update", v)
	}
	var ns1, ns2 int64
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT ns_id FROM namespace WHERE source_id=$1 AND is_mount`, id1).Scan(&ns1); e != nil {
		t.Fatal(e)
	}
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT ns_id FROM namespace WHERE source_id=$1 AND is_mount`, id2).Scan(&ns2); e != nil {
		t.Fatal(e)
	}
	// Even identical endpoints/buckets/prefixes have independent object keys.
	p1, p2 := f.upload(ns1, "same.png"), f.upload(ns2, "same.png")
	for _, pair := range [][2]int64{{id1, p1}, {id2, p2}} {
		source, e := storage.ReadSource(f.ctx, f.a.Pool, pair[0])
		if e != nil {
			t.Fatal(e)
		}
		st, e := storage.SourceStore(source)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { _ = st.DeletePrefix(f.ctx, f.uuid+"/") })
		var uuid string
		if e = f.a.Pool.QueryRow(f.ctx, `SELECT photo_uuid FROM photo WHERE photo_id=$1`, pair[1]).Scan(&uuid); e != nil {
			t.Fatal(e)
		}
		if _, e = st.Stat(f.ctx, fmt.Sprintf("%s/%d/%s/original.png", f.uuid, pair[0], uuid)); e != nil {
			t.Fatal(e)
		}
	}
	if v := f.json("DELETE", endpoint(id1), nil); v["msgCode"] != float64(409) {
		t.Fatal("removed occupied source", v)
	}
	other := newLibraryTest(t)
	for _, method := range []string{"PATCH", "DELETE"} {
		if v := other.json(method, endpoint(id1), map[string]any{"name": "stolen"}); v["msgCode"] != float64(404) {
			t.Fatal(v)
		}
	}
	if v := other.json("POST", "/user/storage/s3/test", map[string]any{"sourceId": id1}); v["msgCode"] != float64(404) {
		t.Fatal(v)
	}
	f.ok("POST", "/user/storage/s3/test", map[string]any{"sourceId": id1, "values": map[string]any{"access_key": "", "secret_key": ""}})
	f.ok("PATCH", endpoint(id1), map[string]any{"enabled": false})
	var active bool
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT enabled FROM storage_source WHERE id=$1`, id2).Scan(&active); e != nil || !active {
		t.Fatal("disabled sibling", e)
	}
	f.ok("PATCH", endpoint(id1), map[string]any{"enabled": true})
	f.ok("POST", "/namespace/order", map[string]any{"parentId": f.root, "nsId": ns2, "beforeNsId": ns1})
	before := childIDs(f, f.root)
	f.ok("PATCH", endpoint(id1), map[string]any{"enabled": false})
	f.ok("PATCH", endpoint(id1), map[string]any{"enabled": true})
	if fmt.Sprint(before) != fmt.Sprint(childIDs(f, f.root)) {
		t.Fatal("disable changed mount order")
	}
	// Alias updates do not decrypt or contact the server.
	t.Setenv("STORAGE_SECRET_KEY", "")
	renamed := f.ok("PATCH", endpoint(id1), map[string]any{"name": "Trips"}).(map[string]any)
	if renamed["name"] != "Trips" {
		t.Fatal(renamed)
	}
	var sourceName, folderName string
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT s.display_name,n.ns_name FROM storage_source s JOIN namespace n ON n.source_id=s.id AND n.is_mount WHERE s.id=$1`, id1).Scan(&sourceName, &folderName); e != nil || sourceName != "Trips" || folderName != sourceName {
		t.Fatal(sourceName, folderName, e)
	}
	t.Setenv("STORAGE_SECRET_KEY", "test-key")
	empty := f.ok("POST", "/user/storage/s3", cfg).(map[string]any)
	id3 := int64(empty["sourceId"].(float64))
	source, e := storage.ReadSource(f.ctx, f.a.Pool, id3)
	if e != nil {
		t.Fatal(e)
	}
	st, e := storage.SourceStore(source)
	if e != nil {
		t.Fatal(e)
	}
	key := fmt.Sprintf("%s/%d/unmanaged.txt", f.uuid, id3)
	if e = st.Put(f.ctx, key, strings.NewReader("keep"), 4); e != nil {
		t.Fatal(e)
	}
	defer st.DeletePrefix(f.ctx, f.uuid+"/")
	// An unsealed upload reserves the connection even before any photo exists.
	var emptyNS int64
	if e = f.a.Pool.QueryRow(f.ctx, `SELECT ns_id FROM namespace WHERE source_id=$1 AND is_mount`, id3).Scan(&emptyNS); e != nil {
		t.Fatal(e)
	}
	batchID := fmt.Sprintf("test-%d", id3)
	if _, e = f.a.Pool.Exec(f.ctx, `INSERT INTO upload_batch(id,user_id,ns_id,name,created_at,updated_at,folders) VALUES($1,$2,$3,'pending',now(),now(),'{}')`, batchID, f.uid, emptyNS); e != nil {
		t.Fatal(e)
	}
	if v := f.json("DELETE", endpoint(id3), nil); v["msgCode"] != float64(409) {
		t.Fatal("removed uploading source", v)
	}
	if _, e = f.a.Pool.Exec(f.ctx, `UPDATE upload_batch SET sealed=true WHERE id=$1`, batchID); e != nil {
		t.Fatal(e)
	}
	f.ok("DELETE", endpoint(id3), nil)
	if _, e = st.Stat(f.ctx, key); e != nil {
		t.Fatal("removal deleted remote objects", e)
	}
}
