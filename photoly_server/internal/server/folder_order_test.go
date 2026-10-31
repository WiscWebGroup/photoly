package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func childIDs(f *libraryTest, parent int64) []int64 {
	rows := f.ok("GET", fmt.Sprintf("/namespace/getChildren?parentId=%d", parent), nil).([]any)
	ids := []int64{}
	for _, v := range rows {
		ids = append(ids, int64(v.(map[string]any)["nsId"].(float64)))
	}
	return ids
}
func TestFolderDisplayOrder(t *testing.T) {
	f := newLibraryTest(t)
	a, b, c := f.folder(f.root, "A"), f.folder(f.root, "B"), f.folder(f.root, "C")
	order := func(id int64, before any) {
		f.ok("POST", "/namespace/order", map[string]any{"parentId": f.root, "nsId": id, "beforeNsId": before})
	}
	expect := func(ids ...int64) {
		t.Helper()
		if got := childIDs(f, f.root); !reflect.DeepEqual(got, ids) {
			t.Fatalf("order %v want %v", got, ids)
		}
	}
	order(c, a)
	expect(c, a, b)
	f.ok("POST", "/namespace/updateName", map[string]any{"nsId": a, "nsName": "renamed"})
	expect(c, a, b)
	// Reordering has no filesystem effects; child moves append, renames keep order.
	for _, name := range []string{"renamed", "B", "C"} {
		if _, e := os.Stat(filepath.Join(f.dir, name)); e != nil {
			t.Fatal(e)
		}
	}
	d := f.folder(f.root, "D")
	expect(c, a, b, d)
	nested := f.folder(b, "nested")
	f.ok("POST", "/namespace/updateParent", map[string]any{"nsId": nested, "nsParentId": f.root})
	expect(c, a, b, d, nested)
	if e := os.Mkdir(filepath.Join(f.dir, "external"), 0755); e != nil {
		t.Fatal(e)
	}
	f.scan()
	f.scan()
	got := childIDs(f, f.root)
	if !reflect.DeepEqual(got[:5], []int64{c, a, b, d, nested}) || len(got) != 6 {
		t.Fatal(got)
	}
	order(c, nil)
	expect(a, b, d, nested, got[5], c)
	other := newLibraryTest(t)
	foreign := other.folder(other.root, "foreign")
	for _, before := range []int64{foreign, 999999999} {
		v := f.json("POST", "/namespace/order", map[string]any{"parentId": f.root, "nsId": a, "beforeNsId": before})
		if v["msgCode"] != float64(409) {
			t.Fatal(v)
		}
	}
	// A stale target moved into another parent is a conflict, with no partial order change.
	f.ok("POST", "/namespace/updateParent", map[string]any{"nsId": d, "nsParentId": b})
	v := f.json("POST", "/namespace/order", map[string]any{"parentId": f.root, "nsId": c, "beforeNsId": d})
	if v["msgCode"] != float64(409) {
		t.Fatal(v)
	}
	expect(a, b, nested, got[5], c)
}

func TestConcurrentFolderOrder(t *testing.T) {
	f := newLibraryTest(t)
	ids := []int64{f.folder(f.root, "One"), f.folder(f.root, "Two"), f.folder(f.root, "Three")}
	var wg sync.WaitGroup
	results := make(chan int, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body, _ := json.Marshal(map[string]any{"parentId": f.root, "nsId": ids[i%3], "beforeNsId": nil})
			w := f.request("POST", "/namespace/order", "application/json", bytes.NewReader(body))
			var reply map[string]any
			if json.Unmarshal(w.Body.Bytes(), &reply) != nil {
				results <- w.Code
				return
			}
			results <- int(reply["msgCode"].(float64))
		}(i)
	}
	wg.Wait()
	close(results)
	for code := range results {
		if code != 200 {
			t.Fatalf("concurrent order returned %d", code)
		}
	}
	var total, distinct int
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT count(*),count(DISTINCT display_order) FROM namespace WHERE ns_parent_id=$1`, f.root).Scan(&total, &distinct); e != nil || total != 3 || distinct != 3 {
		t.Fatal(total, distinct, e)
	}
}
