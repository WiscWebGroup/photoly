package catalog

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy/testutil"
)

func id(v any) string { return strconv.FormatInt(int64(v.(float64)), 10) }

func setup(t *testing.T) (*testutil.Client, *testutil.Client, func(string, ...any) int64, string) {
	a := testutil.New(t)
	Register(a)
	c := &testutil.Client{T: t, Mux: a.Mux}
	// two users created straight in SQL; sessions via the default store
	mk := func(email string) (*testutil.Client, int64, string) {
		var uid int64
		uuid := email + "-uuid"
		if err := a.Pool.QueryRow(t.Context(), `INSERT INTO "user"(email,uuid,role,used_bytes) VALUES ($1,$2,'user',0) RETURNING user_id`, email, uuid).Scan(&uid); err != nil {
			t.Fatal(err)
		}
		if _, err := a.Pool.Exec(t.Context(), `INSERT INTO namespace(ns_name,ns_parent_id,user_id) VALUES ('/',-1,$1)`, uid); err != nil {
			t.Fatal(err)
		}
		tok := "tok-" + email
		if _, err := a.Pool.Exec(t.Context(), `INSERT INTO photoly_session VALUES ($1,$2,now()+interval '1 day')`, tok, uid); err != nil {
			t.Fatal(err)
		}
		return &testutil.Client{T: t, Mux: a.Mux, Token: tok}, uid, uuid
	}
	u1, uid1, uuid1 := mk("one")
	u2, _, _ := mk("two")
	_ = c
	exec := func(q string, args ...any) int64 {
		var n int64
		if err := a.Pool.QueryRow(t.Context(), q, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	exec2 := func(q string, args ...any) int64 { // returns uid1 for convenience
		return exec(q, args...)
	}
	_ = exec2
	// stash helpers in closures: uid1 as first return of exec via "SELECT uid"
	pre := func(q string, args ...any) int64 {
		if q == "uid" {
			return uid1
		}
		if q == "root" {
			return exec(`SELECT ns_id FROM namespace WHERE user_id=$1`, uid1)
		}
		if q == "dir" {
			return 0
		}
		return exec(q, args...)
	}
	_ = os.MkdirAll(filepath.Join(a.Cfg.UploadFolder, uuid1), 0o755)
	return u1, u2, pre, a.Cfg.UploadFolder + "|" + uuid1
}

func TestTagGalleryNamespaceFlow(t *testing.T) {
	u1, u2, q, dirs := setup(t)
	root, uuid := dirs[:len(dirs)-len("one-uuid")-1], "one-uuid"
	rootNs := q("root")

	// tags
	u1.Want(u1.JSON("POST", "/tag/insert", map[string]any{"tagName": "beach"}), 200)
	u1.Want(u1.JSON("POST", "/tag/insert", map[string]any{"tagName": "city"}), 200)
	if m := u1.JSON("POST", "/tag/insert", map[string]any{"tagName": "a/b"}); m["t"] != -1.0 || m["msgCode"] != 400.0 {
		t.Fatalf("bad tag = %v", m)
	}
	tags := u1.Want(u1.JSON("GET", "/tag/getAll", nil), 200).([]any)
	if len(tags) != 2 || tags[0].(map[string]any)["tagName"] != "beach" || tags[0].(map[string]any)["userId"] != float64(q("uid")) {
		t.Fatalf("tags = %v", tags)
	}
	pg := u1.JSON("GET", "/tag/getPage?page=2&rowsPerPage=1", nil)
	if pg["pageNum"] != 2.0 || len(pg["t"].([]any)) != 1 || pg["t"].([]any)[0].(map[string]any)["tagName"] != "city" {
		t.Fatalf("page = %v", pg)
	}
	if pg := u1.JSON("GET", "/tag/getPage?page=0&rowsPerPage=1", nil); pg["pageNum"] != 0.0 || pg["t"] != nil || pg["msgCode"] != 400.0 {
		t.Fatalf("bad page = %v", pg)
	}
	anon := &testutil.Client{T: t, Mux: u1.Mux}
	if pg := anon.JSON("GET", "/tag/getPage?page=1&rowsPerPage=1", nil); pg["pageNum"] != -1.0 || pg["msgCode"] != 403.0 {
		t.Fatalf("anon page = %v", pg)
	}
	if m := anon.JSON("POST", "/tag/insert", map[string]any{"tagName": "x"}); m["t"] != -1.0 || m["msgCode"] != 403.0 {
		t.Fatalf("anon insert = %v", m)
	}
	if m := anon.JSON("POST", "/gallery/insert", map[string]any{"gaName": "x"}); m["t"] != false || m["msgCode"] != 403.0 {
		t.Fatalf("anon gallery insert = %v", m)
	}
	tag := tags[0].(map[string]any)
	if m := u2.JSON("POST", "/tag/update", map[string]any{"tagId": tag["tagId"], "tagName": "hijack"}); m["t"] != -1.0 || m["msgCode"] != 400.0 {
		t.Fatalf("foreign update = %v", m)
	}
	u1.Want(u1.JSON("POST", "/tag/update", map[string]any{"tagId": tag["tagId"], "tagName": "sea"}), 200)

	// smart tag: update renames, delete dismisses (keeps suppressed item)
	smart := q(`INSERT INTO smart_item(user_id,source,tag_id,name,basis) VALUES ($1,'camera',$2,'Canon','{"k":1}') RETURNING item_id`, q("uid"), int64(tag["tagId"].(float64)))
	q(`INSERT INTO smart_preference(user_id) VALUES ($1) RETURNING revision`, q("uid"))
	u1.Want(u1.JSON("POST", "/tag/update", map[string]any{"tagId": tag["tagId"], "tagName": "sea"}), 200)
	withSmart := u1.Want(u1.JSON("GET", "/tag/getAll?includeSmart=true", nil), 200).([]any)
	sm := withSmart[0].(map[string]any)["smart"].(map[string]any)
	if sm["name"] != "sea" || sm["source"] != "camera" || sm["suppressed"] != false || withSmart[1].(map[string]any)["smart"] != nil {
		t.Fatalf("smart = %v", withSmart)
	}
	u1.Want(u1.JSON("POST", "/tag/delete?tagId="+id(tag["tagId"]), nil), 200)
	if q(`SELECT count(*) FROM smart_item WHERE item_id=$1 AND suppressed AND tag_id IS NULL AND custom_name='sea'`, smart) != 1 ||
		q(`SELECT revision FROM smart_preference WHERE user_id=$1`, q("uid")) != 2 {
		t.Fatal("smart item not dismissed")
	}

	// galleries
	if m := u1.JSON("POST", "/gallery/insert", map[string]any{"gaName": "Trip", "coverId": 3, "coverColor": "#fff"}); m["t"] != true || m["msgCode"] != 200.0 {
		t.Fatalf("gallery insert = %v", m)
	}
	if m := u1.JSON("POST", "/gallery/insert", map[string]any{"gaName": ""}); m["t"] != false || m["msgCode"] != 400.0 {
		t.Fatalf("gallery bad = %v", m)
	}
	gals := u1.Want(u1.JSON("GET", "/gallery/getAll", nil), 200).([]any)
	g := gals[0].(map[string]any)
	if g["gaName"] != "Trip" || g["coverId"] != 3.0 || g["coverColor"] != "#fff" || len(g["createDate"].(string)) != 19 {
		t.Fatalf("gallery = %v", g)
	}
	u1.Want(u1.JSON("POST", "/gallery/update", map[string]any{"gaId": g["gaId"], "gaName": "Trip2", "coverId": 4, "coverColor": "#000"}), 200)
	if m := u1.JSON("POST", "/gallery/delete?gaId=99999", nil); m["t"] != -1.0 {
		t.Fatalf("gallery delete missing = %v", m)
	}

	// namespaces
	u1.Want(u1.JSON("POST", "/namespace/insert", map[string]any{"nsName": "Europe", "nsParentId": rootNs}), 200)
	kids := u1.Want(u1.JSON("GET", "/namespace/getChildren?parentId="+strconv.FormatInt(rootNs, 10), nil), 200).([]any)
	europe := kids[0].(map[string]any)
	u1.Want(u1.JSON("POST", "/namespace/insert", map[string]any{"nsName": "France", "nsParentId": europe["nsId"]}), 200)
	u1.Want(u1.JSON("POST", "/namespace/insert", map[string]any{"nsName": "Asia", "nsParentId": rootNs}), 200)
	if m := u1.JSON("POST", "/namespace/insert", map[string]any{"nsName": "x", "nsParentId": 424242}); m["t"] != -1.0 || m["msgCode"] != 400.0 {
		t.Fatalf("insert bad parent = %v", m)
	}
	france := u1.Want(u1.JSON("GET", "/namespace/getChildren?parentId="+id(europe["nsId"]), nil), 200).([]any)[0].(map[string]any)
	tr := u1.Want(u1.JSON("GET", "/namespace/trace?nsId="+id(france["nsId"]), nil), 200).([]any)
	if len(tr) != 3 || tr[0].(map[string]any)["nsName"] != "/" || tr[2].(map[string]any)["nsName"] != "France" {
		t.Fatalf("trace = %v", tr)
	}
	if m := u2.JSON("GET", "/namespace/trace?nsId="+id(france["nsId"]), nil); m["t"] != nil || m["msgCode"] != 400.0 {
		t.Fatalf("foreign trace = %v", m)
	}
	if got := u1.Want(u1.JSON("GET", "/namespace/getNameById?nsId="+id(france["nsId"]), nil), 200); got != "France" {
		t.Fatalf("name = %v", got)
	}
	// cycle: move Europe under France
	if m := u1.JSON("POST", "/namespace/updateParent", map[string]any{"nsId": europe["nsId"], "nsParentId": france["nsId"]}); m["t"] != -1.0 {
		t.Fatalf("cycle = %v", m)
	}
	if m := u1.JSON("POST", "/namespace/updateName", map[string]any{"nsId": rootNs, "nsName": "r"}); m["t"] != -1.0 {
		t.Fatalf("rename root = %v", m)
	}
	u1.Want(u1.JSON("POST", "/namespace/updateName", map[string]any{"nsId": france["nsId"], "nsName": "Francia"}), 200)

	// photos with files in France, details + download + delete
	uid := q("uid")
	for i, n := range []string{"p1", "p2"} {
		pid := q(`INSERT INTO photo(photo_name,upload_date,format,ns_id,visibility,token,photo_uuid,user_id) VALUES ('same','2024-05-06 07:08:09','jpg',$1,0,$2,$3,$4) RETURNING photo_id`,
			int64(france["nsId"].(float64)), "tk"+n, n, uid)
		q(`INSERT INTO photo_extra(f_photo_id,extra_photo_gps_lat,extra_photo_gps_lon,extra_photo_camera,extra_photo_taken_date) VALUES ($1,48.8566,2.3522,$2,'2024-05-06 07:08:09') RETURNING 1`, pid, []string{"Canon  R5", "canon r5"}[i])
		if err := os.WriteFile(filepath.Join(root, uuid, n+".jpg"), []byte("0123456789"), 0o644); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			os.WriteFile(filepath.Join(root, uuid, n+"_thumbnail.jpg"), []byte("abc"), 0o644)
		}
		if i == 0 {
			q(`INSERT INTO gallery_photo(ga_id,photo_id) VALUES ($1,$2) RETURNING 1`, int64(g["gaId"].(float64)), pid)
		}
	}
	d := u1.Want(u1.JSON("GET", "/namespace/getDetails?nsId="+id(europe["nsId"]), nil), 200).(map[string]any)
	cams := d["cameras"].([]any)
	if d["photoCount"] != 2.0 || d["folderCount"] != 1.0 || d["originalBytes"] != 20.0 || d["thumbnailBytes"] != 3.0 || d["totalBytes"] != 23.0 ||
		d["missingFileCount"] != 1.0 || d["complete"] != false || len(cams) != 1 || cams[0].(map[string]any)["count"] != 2.0 || cams[0].(map[string]any)["name"] != "Canon R5" ||
		d["places"].([]any)[0].(map[string]any)["name"] != "Paris, France" {
		t.Fatalf("details = %v", d)
	}
	if m := u1.JSON("GET", "/namespace/getDetails", nil); m["t"] != -1.0 || m["msgCode"] != 400.0 {
		t.Fatalf("details missing arg = %v", m)
	}

	// map routes
	mp := u1.Want(u1.JSON("GET", "/photo-extra/getGlobalMapPhoto", nil), 200).([]any)
	r0 := mp[0].(map[string]any)
	if len(mp) != 2 || r0["upload_date"] != "2024-05-06" || r0["extra_photo_taken_date"] != "2024-05-06" || r0["photo_uuid"] != "p1" || r0["extra_photo_gps_lat"] != 48.8566 {
		t.Fatalf("map photos = %v", mp)
	}
	if bg := u1.Want(u1.JSON("GET", "/photo-extra/getGlobalMapPhotoByGallery?gaId="+id(g["gaId"]), nil), 200).([]any); len(bg) != 1 {
		t.Fatalf("by gallery = %v", bg)
	}
	if m := u1.JSON("GET", "/photo-extra/getGlobalMapPhotoByGallery", nil); m["t"] != -1.0 {
		t.Fatalf("by gallery missing = %v", m)
	}
	if m := u2.JSON("GET", "/photo-extra/getGlobalMapPhotoByGallery?gaId="+id(g["gaId"]), nil); m["t"] != nil || m["msgCode"] != 400.0 {
		t.Fatalf("by gallery foreign = %v", m)
	}
	pl := u1.Want(u1.JSON("GET", "/photo-extra/getMapPlaces", nil), 200).([]any)
	if len(pl) != 2 || pl[0].(map[string]any)["place"] != "Paris" || pl[0].(map[string]any)["country"] != "France" {
		t.Fatalf("places = %v", pl)
	}

	// downloads: duplicate names are disambiguated, token comes from the path
	code, body := anon.Do("GET", "/namespace/downloadNs/"+u1.Token+"?nsId="+id(europe["nsId"]), "", nil)
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if code != 200 || err != nil {
		t.Fatalf("zip: %d %v", code, err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	if len(names) != 2 || names[0] != "Europe/Francia/same.jpg" || names[1] == names[0] {
		t.Fatalf("zip names = %v", names)
	}
	if code, _ := anon.Do("GET", "/namespace/downloadNs/bad?nsId=1", "", nil); code != 403 {
		t.Fatalf("zip anon = %d", code)
	}
	if code, _ := anon.Do("GET", "/namespace/downloadNs/"+u2.Token+"?nsId="+id(europe["nsId"]), "", nil); code != 400 {
		t.Fatalf("zip foreign = %d", code)
	}
	code, body = anon.Do("GET", "/gallery/downloadGa/"+u1.Token+"?gaId="+id(g["gaId"]), "", nil)
	if zr, err = zip.NewReader(bytes.NewReader(body), int64(len(body))); code != 200 || err != nil || len(zr.File) != 1 || zr.File[0].Name != "Trip2/same.jpg" {
		t.Fatalf("gallery zip: %d %v", code, err)
	}
	os.Remove(filepath.Join(root, uuid, "p2.jpg"))
	if code, _ := anon.Do("GET", "/namespace/downloadNs/"+u1.Token+"?nsId="+id(europe["nsId"]), "", nil); code != 404 {
		t.Fatalf("zip with missing file = %d", code)
	}
	os.WriteFile(filepath.Join(root, uuid, "p2.jpg"), []byte("0123456789"), 0o644)

	// delete subtree: root refused; Europe removes 2 namespaces + 3 files
	if m := u1.JSON("POST", "/namespace/delete?nsId="+strconv.FormatInt(rootNs, 10), nil); m["t"] != -1.0 || m["msgCode"] != 400.0 {
		t.Fatalf("delete root = %v", m)
	}
	m := u1.JSON("POST", "/namespace/delete?nsId="+id(europe["nsId"]), nil)
	if m["t"] != 5.0 || m["msgCode"] != 200.0 {
		t.Fatalf("delete ns = %v", m)
	}
	if _, err := os.Stat(filepath.Join(root, uuid, "p1.jpg")); !os.IsNotExist(err) {
		t.Fatal("photo file not removed")
	}
	if q(`SELECT used_bytes FROM "user" WHERE user_id=$1`, uid) != 0 {
		t.Fatalf("used_bytes = %d", q(`SELECT used_bytes FROM "user" WHERE user_id=$1`, uid))
	}
	if q(`SELECT count(*) FROM gallery_photo`) != 0 || q(`SELECT count(*) FROM photo`) != 0 {
		t.Fatal("photo rows left")
	}
}
