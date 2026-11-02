package server

import (
	"context"
	"fmt"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/photo"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNotesPrivacyVersionsAndLifecycle(t *testing.T) {
	f := newLibraryTest(t)
	pid := f.upload(f.root, "note.png")
	path := fmt.Sprintf("/photo/notes?photoId=%d", pid)
	created := f.ok("POST", "/photo/notes", map[string]any{"photoId": pid, "body": "A memory\n第二段", "x": 0.25, "y": 0.75, "contentVersion": 1}).(map[string]any)
	nid := int64(created["id"].(float64))
	np := fmt.Sprintf("/photo/notes/%d", nid)
	f.ok("POST", "/photo/notes", map[string]any{"photoId": pid, "body": "General"})
	data := f.ok("GET", path, nil).(map[string]any)
	items := data["items"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["needsReposition"] != false {
		t.Fatal(data)
	}
	change := map[string]any{"photoId": pid, "body": "Edited", "x": 0.25, "y": 0.75, "contentVersion": 1, "revision": 1}
	f.ok("PATCH", np, change)
	if got := f.json("PATCH", np, change)["msgCode"]; got != float64(409) {
		t.Fatal("stale edit", got)
	}
	if _, e := f.a.Pool.Exec(f.ctx, `UPDATE photo SET content_version=content_version+1 WHERE photo_id=$1`, pid); e != nil {
		t.Fatal(e)
	}
	data = f.ok("GET", path, nil).(map[string]any)
	items = data["items"].([]any)
	if items[0].(map[string]any)["needsReposition"] != true || items[1].(map[string]any)["needsReposition"] != false {
		t.Fatal(data)
	}
	change["revision"] = 2
	f.ok("PATCH", np, change) // text-only save preserves stale anchor
	change["revision"] = 3
	change["contentVersion"] = 2
	change["x"] = 0.5
	f.ok("PATCH", np, change)
	other := newLibraryTest(t)
	if got := other.json("GET", path, nil)["msgCode"]; got != float64(404) {
		t.Fatal("cross-user read", got)
	}
	if got := other.json("PATCH", np, change)["msgCode"]; got != float64(404) {
		t.Fatal("cross-user write", got)
	}
	req := httptest.NewRequest("GET", path+"&token=public-share", nil)
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, req)
	if strings.Contains(w.Body.String(), "Edited") {
		t.Fatal("share leaked note")
	}
	if got := f.json("POST", "/photo/notes", map[string]any{"photoId": pid, "body": "bad", "x": 2, "y": 0, "contentVersion": 2})["msgCode"]; got == float64(200) {
		t.Fatal("accepted invalid point")
	}
	f.ok("POST", fmt.Sprintf("/photo/delete?photoId=%d", pid), nil)
	var count int
	f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM media_note WHERE photo_id=$1`, pid).Scan(&count)
	if count != 0 {
		t.Fatal("orphan notes")
	}
}
func TestCategoryOverridesAndPagination(t *testing.T) {
	f := newLibraryTest(t)
	pid := f.upload(f.root, "wide.png")
	_, e := f.a.Pool.Exec(f.ctx, `UPDATE media_classification SET categories=ARRAY['panorama'],state='ready' WHERE photo_id=$1`, pid)
	if e != nil {
		t.Fatal(e)
	}
	page := f.ok("GET", "/photo/categories/panorama/photos", nil).(map[string]any)
	if len(page["items"].([]any)) != 1 {
		t.Fatal(page)
	}
	f.ok("PUT", "/photo/category", map[string]any{"photoId": pid, "category": "panorama", "included": false})
	page = f.ok("GET", "/photo/categories/panorama/photos", nil).(map[string]any)
	if len(page["items"].([]any)) != 0 {
		t.Fatal(page)
	}
	f.ok("PUT", "/photo/category", map[string]any{"photoId": pid, "category": "portrait", "included": true})
	_, e = f.a.Pool.Exec(f.ctx, `UPDATE photo SET content_version=content_version+1 WHERE photo_id=$1`, pid)
	if e != nil {
		t.Fatal(e)
	}
	page = f.ok("GET", "/photo/categories/portrait/photos", nil).(map[string]any)
	if len(page["items"].([]any)) != 1 {
		t.Fatal("override lost", page)
	}
	f.ok("PUT", "/photo/category", map[string]any{"photoId": pid, "category": "portrait", "included": nil})
	if got := f.json("PUT", "/photo/category", map[string]any{"photoId": pid, "category": "video", "included": true})["msgCode"]; got == float64(200) {
		t.Fatal("structural override allowed")
	}
	other := newLibraryTest(t)
	if got := other.json("PUT", "/photo/category", map[string]any{"photoId": pid, "category": "portrait", "included": true})["msgCode"]; got != float64(404) {
		t.Fatal("cross-user category mutation", got)
	}
	var sid int64
	f.a.Pool.QueryRow(f.ctx, `SELECT source_id FROM photo WHERE photo_id=$1`, pid).Scan(&sid)
	if got := other.json("GET", fmt.Sprintf("/user/storage/health?ids=%d", sid), nil)["msgCode"]; got != float64(404) {
		t.Fatal("cross-user health", got)
	}
}

func TestCategoryLargeLibraryPagination(t *testing.T) {
	if os.Getenv("PHOTOLY_SCALE_TEST") == "" {
		t.Skip("set PHOTOLY_SCALE_TEST for the 100,000-row database test")
	}
	f := newLibraryTest(t)
	started := time.Now()
	for start := 1; start <= 100001; start += 1000 {
		end := start + 999
		if end > 100001 {
			end = 100001
		}
		_, e := f.a.Pool.Exec(f.ctx, `INSERT INTO photo(photo_name,format,ns_id,user_id,photo_uuid,token,upload_date) SELECT 'scale-'||i,'mp4',$1,$2,md5(($2::int)::text||':'||i),md5(i::text),now() FROM generate_series($3::int,$4::int) i`, f.root, f.uid, start, end)
		if e != nil {
			t.Fatal(e)
		}
	}
	t.Logf("100001 indexed rows inserted in %s (no media decoding)", time.Since(started))
	started = time.Now()
	first := f.ok("GET", "/photo/categories/video/photos", nil).(map[string]any)
	if len(first["items"].([]any)) != 100 || first["nextAfter"] == nil {
		t.Fatal(first)
	}
	next := f.ok("GET", fmt.Sprintf("/photo/categories/video/photos?after=%.0f", first["nextAfter"]), nil).(map[string]any)
	if len(next["items"].([]any)) != 100 {
		t.Fatal(next)
	}
	if first["items"].([]any)[99].(map[string]any)["photoId"].(float64) <= next["items"].([]any)[0].(map[string]any)["photoId"].(float64) {
		t.Fatal("pages overlap")
	}
	summary := f.ok("GET", "/photo/categories", nil).(map[string]any)
	for _, v := range summary["items"].([]any) {
		row := v.(map[string]any)
		if row["category"] == "video" && row["count"] != float64(100001) {
			t.Fatal(row)
		}
	}
	t.Logf("Two pages and all counts queried in %s", time.Since(started))
}

func TestCategoryQueueRecovery(t *testing.T) {
	f := newLibraryTest(t)
	good := f.upload(f.root, "Screenshot_2026.png")
	bad := f.upload(f.root, "missing.png")
	if e := os.Remove(filepath.Join(f.dir, "missing.png")); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(f.ctx)
	done := make(chan struct{})
	go func() { defer close(done); photo.CategoryWorker(ctx, f.a) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		var ready bool
		var attempts int
		e := f.a.Pool.QueryRow(f.ctx, `SELECT state='ready' AND 'screenshot'=ANY(categories) FROM media_classification WHERE photo_id=$1`, good).Scan(&ready)
		if e != nil {
			t.Fatal(e)
		}
		f.a.Pool.QueryRow(f.ctx, `SELECT attempts FROM media_classification WHERE photo_id=$1`, bad).Scan(&attempts)
		if ready && attempts > 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("classification or retry did not persist")
}
