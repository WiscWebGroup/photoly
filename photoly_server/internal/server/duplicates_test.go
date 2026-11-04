package server

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func duplicateGroups(f *libraryTest, ignored bool) []any {
	return f.ok("GET", fmt.Sprintf("/photo/duplicates?ignored=%t", ignored), nil).(map[string]any)["items"].([]any)
}
func duplicateGroupFor(f *libraryTest) map[string]any {
	f.t.Helper()
	g := duplicateGroups(f, false)
	if len(g) != 1 {
		f.t.Fatalf("groups: %v", g)
	}
	return g[0].(map[string]any)
}
func duplicateBody(g map[string]any, fields map[string]any) map[string]any {
	d := map[string]any{"key": g["key"], "version": g["version"]}
	for k, v := range fields {
		d[k] = v
	}
	return d
}
func duplicateMembers(f *libraryTest, g map[string]any, extra string) map[string]any {
	return f.ok("GET", "/photo/duplicates/members?key="+url.QueryEscape(g["key"].(string))+"&version="+g["version"].(string)+extra, nil).(map[string]any)
}
func TestDuplicatesWorkflow(t *testing.T) {
	f := newLibraryTest(t)
	a := f.upload(f.root, "a.png")
	b := f.upload(f.folder(f.root, "Elsewhere"), "b.png")
	g := duplicateGroupFor(f)
	if g["count"] != float64(2) {
		t.Fatal(g)
	}
	page := duplicateMembers(f, g, "&limit=1&offset=1")
	if len(page["items"].([]any)) != 1 || page["total"] != float64(2) {
		t.Fatal(page)
	}
	f.ok("POST", "/photo/duplicates/keep", duplicateBody(g, map[string]any{"photoId": a, "value": true}))
	members := duplicateMembers(f, g, "")["items"].([]any)
	if !members[0].(map[string]any)["kept"].(bool) {
		t.Fatal(members)
	}
	bad := f.json("POST", "/photo/duplicates/delete", duplicateBody(g, map[string]any{"photoIds": []int64{a}}))
	if bad["msgCode"] != float64(400) {
		t.Fatal(bad)
	}
	bad = f.json("POST", "/photo/duplicates/delete", duplicateBody(g, map[string]any{"photoIds": []int64{a, b}}))
	if bad["msgCode"] != float64(400) {
		t.Fatal(bad)
	}
	f.ok("POST", "/photo/duplicates/ignore", duplicateBody(g, map[string]any{"value": true}))
	if len(duplicateGroups(f, false)) != 0 || len(duplicateGroups(f, true)) != 1 {
		t.Fatal("ignore did not persist")
	}
	f.ok("POST", "/photo/duplicates/ignore", duplicateBody(g, map[string]any{"value": false}))
	f.ok("POST", "/photo/duplicates/ignore", duplicateBody(g, map[string]any{"value": true}))
	c := f.upload(f.root, "c.png")
	fresh := duplicateGroupFor(f)
	if fresh["version"] == g["version"] || len(duplicateGroups(f, true)) != 0 {
		t.Fatal("new member did not invalidate ignore")
	}
	bad = f.json("POST", "/photo/duplicates/delete", duplicateBody(g, map[string]any{"photoIds": []int64{b}}))
	if bad["msgCode"] != float64(409) {
		t.Fatal(bad)
	}
	f.ok("POST", "/photo/duplicates/delete", duplicateBody(fresh, map[string]any{"photoIds": []int64{b, c}}))
	if len(duplicateGroups(f, false)) != 0 {
		t.Fatal("resolved group remained")
	}
	if _, e := os.Stat(filepath.Join(f.dir, "a.png")); e != nil {
		t.Fatal("keeper removed", e)
	}
	if _, e := os.Stat(filepath.Join(f.dir, "c.png")); !os.IsNotExist(e) {
		t.Fatal("copy still exists", e)
	}
}
func TestDuplicatesEligibilityAndVersions(t *testing.T) {
	f := newLibraryTest(t)
	a := f.upload(f.root, "a.png")
	b := f.upload(f.root, "b.png")
	g := duplicateGroupFor(f)
	f.ok("POST", "/photo/duplicates/keep", duplicateBody(g, map[string]any{"photoId": a, "value": true}))
	exec := func(query string, args ...any) {
		t.Helper()
		if _, e := f.a.Pool.Exec(f.ctx, query, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`UPDATE photo SET content_version=content_version+1 WHERE photo_id=$1`, a)
	g = duplicateGroupFor(f)
	if g["keptCount"] != float64(0) {
		t.Fatal("stale keep persisted", g)
	}
	for _, query := range []string{`UPDATE photo SET content_sha256='' WHERE photo_id=$1`, `UPDATE photo SET missing_scans=1 WHERE photo_id=$1`, `UPDATE photo SET format='mp4' WHERE photo_id=$1`} {
		exec(query, b)
		if len(duplicateGroups(f, false)) != 0 {
			t.Fatal("ineligible record included", query)
		}
		exec(`UPDATE photo SET content_sha256=(SELECT content_sha256 FROM photo WHERE photo_id=$2),missing_scans=0,format='png' WHERE photo_id=$1`, b, a)
	}
	exec(`UPDATE photo_media SET live=true WHERE photo_id=$1`, a)
	if len(duplicateGroups(f, false)) != 0 {
		t.Fatal("live without video grouped with still")
	}
	for _, id := range []int64{a, b} {
		exec(`UPDATE photo_media SET live=true WHERE photo_id=$1`, id)
		exec(`INSERT INTO media_asset(photo_id,role,filename,display_name,content_sha256) VALUES($1,'live_original','live.mov','live.mov',$2) ON CONFLICT(photo_id,role) DO UPDATE SET content_sha256=excluded.content_sha256`, id, strings.Repeat("a", 64))
	}
	g = duplicateGroupFor(f)
	exec(`UPDATE media_asset SET content_sha256=$2 WHERE photo_id=$1 AND role='live_original'`, b, strings.Repeat("b", 64))
	if len(duplicateGroups(f, false)) != 0 {
		t.Fatal("different live videos grouped")
	}
	exec(`UPDATE media_asset SET content_sha256=$2 WHERE photo_id=$1 AND role='live_original'`, b, strings.Repeat("a", 64))
	f.ok("POST", "/photo/duplicates/ignore", duplicateBody(g, map[string]any{"value": true}))
	exec(`UPDATE media_asset SET content_version=content_version+1 WHERE photo_id=$1 AND role='live_original'`, b)
	if len(duplicateGroups(f, false)) != 1 {
		t.Fatal("live version did not invalidate ignore")
	}
}
func TestDuplicatesOwnerAndValidation(t *testing.T) {
	f := newLibraryTest(t)
	f.upload(f.root, "a.png")
	f.upload(f.root, "b.png")
	g := duplicateGroupFor(f)
	other := newLibraryTest(t)
	other.upload(other.root, "own.png")
	if len(duplicateGroups(other, false)) != 0 {
		t.Fatal("cross-user grouping")
	}
	for _, action := range []string{"keep", "ignore", "delete"} {
		if r := other.json("POST", "/photo/duplicates/"+action, duplicateBody(g, map[string]any{"photoId": 1, "photoIds": []int{1}, "value": true})); r["msgCode"] != float64(409) {
			t.Fatal(r)
		}
	}
	for _, q := range []string{"?limit=0", "?limit=101", "?offset=-1", "?ignored=maybe"} {
		if r := f.json("GET", "/photo/duplicates"+q, nil); r["msgCode"] != float64(400) {
			t.Fatal(r)
		}
	}
	w := f.request("GET", "/photo/duplicates", "", nil)
	if w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal(w.Header())
	}
}
func TestDuplicatesConcurrentDelete(t *testing.T) {
	f := newLibraryTest(t)
	a := f.upload(f.root, "a.png")
	b := f.upload(f.root, "b.png")
	g := duplicateGroupFor(f)
	var wg sync.WaitGroup
	codes := make(chan float64, 2)
	for _, id := range []int64{a, b} {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			r := f.json("POST", "/photo/duplicates/delete", duplicateBody(g, map[string]any{"photoIds": []int64{id}}))
			codes <- r["msgCode"].(float64)
		}(id)
	}
	wg.Wait()
	close(codes)
	success, conflict := 0, 0
	for c := range codes {
		if c == 200 {
			success++
		}
		if c == 409 {
			conflict++
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d", success, conflict)
	}
	var n int
	_ = f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM photo WHERE user_id=$1`, f.uid).Scan(&n)
	if n != 1 {
		t.Fatal(n)
	}
}
func TestDuplicatesUnavailableOrChangedFile(t *testing.T) {
	for _, target := range []string{"a.png", "b.png"} {
		t.Run(target, func(t *testing.T) {
			f := newLibraryTest(t)
			f.upload(f.root, "a.png")
			b := f.upload(f.root, "b.png")
			g := duplicateGroupFor(f)
			if e := os.WriteFile(filepath.Join(f.dir, target), []byte("changed outside Photoly"), 0600); e != nil {
				t.Fatal(e)
			}
			r := f.json("POST", "/photo/duplicates/delete", duplicateBody(g, map[string]any{"photoIds": []int64{b}}))
			if r["t"] != "duplicate_file_changed" {
				t.Fatal(r)
			}
			if len(duplicateGroups(f, false)) != 1 {
				t.Fatal("records changed on failed verification")
			}
			if _, e := os.Stat(filepath.Join(f.dir, "b.png")); e != nil {
				t.Fatal("copy removed", e)
			}
		})
	}
}
func TestDuplicatesFileFailureRollsBack(t *testing.T) {
	f := newLibraryTest(t)
	f.upload(f.root, "keep.png")
	a := f.upload(f.root, "a.png")
	b := f.upload(f.root, "b.png")
	g := duplicateGroupFor(f)
	// A bad asset path makes the second deletion fail after the first was staged.
	if _, e := f.a.Pool.Exec(f.ctx, `UPDATE media_asset SET filename='../invalid' WHERE photo_id=$1 AND role='thumbnail'`, b); e != nil {
		t.Fatal(e)
	}
	r := f.request("POST", "/photo/duplicates/delete", "application/json", strings.NewReader(func() string {
		x, _ := json.Marshal(duplicateBody(g, map[string]any{"photoIds": []int64{a, b}}))
		return string(x)
	}()))
	if r.Code == 200 && strings.Contains(r.Body.String(), `"msgCode":200`) {
		t.Fatal("unexpected success")
	}
	for _, name := range []string{"keep.png", "a.png", "b.png"} {
		if _, e := os.Stat(filepath.Join(f.dir, name)); e != nil {
			t.Fatal("rollback lost", name, e)
		}
	}
	if duplicateGroupFor(f)["count"] != float64(3) {
		t.Fatal("database rollback failed")
	}
}

func TestDuplicatesAcrossSources(t *testing.T) {
	for _, kind := range []string{"local", "webdav"} {
		t.Run(kind, func(t *testing.T) {
			f := newLibraryTest(t)
			dir, endpoint, input := externalBinding(t, f, kind)
			created := f.ok("POST", endpoint, input).(map[string]any)
			sid := int64(created["sourceId"].(float64))
			var mount int64
			if e := f.a.Pool.QueryRow(f.ctx, `SELECT ns_id FROM namespace WHERE source_id=$1 AND is_mount`, sid).Scan(&mount); e != nil {
				t.Fatal(e)
			}
			a := f.upload(f.root, "local.png")
			f.upload(mount, "remote.png")
			g := duplicateGroupFor(f)
			f.ok("POST", "/photo/duplicates/delete", duplicateBody(g, map[string]any{"photoIds": []int64{a}}))
			if _, e := os.Stat(filepath.Join(dir, "remote.png")); e != nil {
				t.Fatal("remote survivor lost", e)
			}
			a = f.upload(f.root, "again.png")
			g = duplicateGroupFor(f)
			if e := os.Remove(filepath.Join(dir, "remote.png")); e != nil {
				t.Fatal(e)
			}
			r := f.json("POST", "/photo/duplicates/delete", duplicateBody(g, map[string]any{"photoIds": []int64{a}}))
			if r["t"] != "duplicate_file_changed" {
				t.Fatal("remote cache accepted as survivor", r)
			}
			if _, e := os.Stat(filepath.Join(f.dir, "again.png")); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestDuplicatesRejectUnselectedAlias(t *testing.T) {
	f := newLibraryTest(t)
	a := f.upload(f.root, "a.png")
	b := f.upload(f.root, "b.png")
	f.upload(f.root, "independent.png")
	g := duplicateGroupFor(f)
	f.ok("POST", "/photo/duplicates/keep", duplicateBody(g, map[string]any{"photoId": a, "value": true}))
	// Simulate two indexed records resolving to the same physical original.
	if _, e := f.a.Pool.Exec(f.ctx, `UPDATE media_asset SET filename='a.png' WHERE photo_id=$1 AND role='original'`, b); e != nil {
		t.Fatal(e)
	}
	r := f.json("POST", "/photo/duplicates/delete", duplicateBody(g, map[string]any{"photoIds": []int64{b}}))
	if r["t"] != "duplicate_file_changed" {
		t.Fatal("unselected alias was deleted", r)
	}
	if _, e := os.Stat(filepath.Join(f.dir, "a.png")); e != nil {
		t.Fatal("kept alias lost", e)
	}
}
