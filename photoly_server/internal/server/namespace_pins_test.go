package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/session"
)

func pinState(f *libraryTest) ([]any, any) {
	state := f.ok("GET", "/namespace/pins", nil).(map[string]any)
	return state["items"].([]any), state["defaultNsId"]
}

func TestNamespacePinsLifecycle(t *testing.T) {
	f := newLibraryTest(t)
	a, b := f.folder(f.root, "Trips"), f.folder(f.root, "Pets")
	post := func(action string, id any) { f.ok("POST", "/namespace/"+action, map[string]any{"nsId": id}) }
	items, def := pinState(f)
	if len(items) != 0 || def != nil {
		t.Fatal(items, def)
	}
	post("pin", a)
	post("pin", b)
	post("pin", a)
	items, _ = pinState(f)
	if len(items) != 2 || items[0].(map[string]any)["nsId"] != float64(a) || items[1].(map[string]any)["nsId"] != float64(b) {
		t.Fatal(items)
	}
	post("defaultLocation", a)
	f.ok("POST", "/namespace/updateName", map[string]any{"nsId": a, "nsName": "Renamed"})
	f.ok("POST", "/namespace/updateParent", map[string]any{"nsId": a, "nsParentId": b})
	items, def = pinState(f)
	if def != float64(a) || items[0].(map[string]any)["nsName"] != "Renamed" || !strings.Contains(items[0].(map[string]any)["path"].(string), "Pets / Renamed") {
		t.Fatal(items, def)
	}

	// Another device/session for the same account reads the persisted setting.
	token, err := (session.Store{Pool: f.a.Pool}).Create(f.ctx, f.uid, 1)
	if err != nil {
		t.Fatal(err)
	}
	f.token = token
	_, def = pinState(f)
	if def != float64(a) {
		t.Fatal(def)
	}
	post("defaultLocation", b)
	post("defaultLocation", nil)
	_, def = pinState(f)
	if def != nil {
		t.Fatal(def)
	}
	post("defaultLocation", a)
	post("unpin", a)
	post("unpin", a)
	items, def = pinState(f)
	if len(items) != 1 || def != nil {
		t.Fatal(items, def)
	}

	// Failed changes roll back the old default, even when the ID is owned but not pinned.
	post("defaultLocation", b)
	if v := f.json("POST", "/namespace/defaultLocation", map[string]any{"nsId": a}); v["msgCode"] != float64(409) {
		t.Fatal(v)
	}
	_, def = pinState(f)
	if def != float64(b) {
		t.Fatal(def)
	}
	post("pin", a)
	post("defaultLocation", a)
	f.ok("POST", fmt.Sprintf("/namespace/delete?nsId=%d", b), nil)
	items, def = pinState(f)
	if len(items) != 0 || def != nil {
		t.Fatal(items, def)
	}
}

func TestNamespacePinsIsolationAndValidation(t *testing.T) {
	f, other := newLibraryTest(t), newLibraryTest(t)
	foreign := other.folder(other.root, "Foreign")
	other.ok("POST", "/namespace/pin", map[string]any{"nsId": foreign})
	for _, action := range []string{"pin", "defaultLocation"} {
		if v := f.json("POST", "/namespace/"+action, map[string]any{"nsId": foreign}); v["msgCode"] == float64(200) {
			t.Fatal(v)
		}
	}
	f.ok("POST", "/namespace/unpin", map[string]any{"nsId": foreign})
	if items, _ := pinState(f); len(items) != 0 {
		t.Fatal(items)
	}
	if items, _ := pinState(other); len(items) != 1 {
		t.Fatal(items)
	}
	for _, action := range []string{"pin", "unpin", "defaultLocation"} {
		for _, input := range []any{map[string]any{}, map[string]any{"nsId": -1}, map[string]any{"nsId": 1.2}, map[string]any{"nsId": "1"}, map[string]any{"nsId": 2147483648}} {
			if v := f.json("POST", "/namespace/"+action, input); v["msgCode"] == float64(200) {
				t.Fatal(action, input, v)
			}
		}
	}
	if _, err := other.a.Pool.Exec(other.ctx, `DELETE FROM "user" WHERE user_id=$1`, other.uid); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM namespace_pin WHERE user_id=$1`, other.uid).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func TestNamespacePinsConcurrentDefaults(t *testing.T) {
	f := newLibraryTest(t)
	ids := []int64{f.folder(f.root, "One"), f.folder(f.root, "Two")}
	for _, id := range ids {
		f.ok("POST", "/namespace/pin", map[string]any{"nsId": id})
	}
	var wg sync.WaitGroup
	results := make(chan string, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body, _ := json.Marshal(map[string]any{"nsId": ids[i%2]})
			w := f.request("POST", "/namespace/defaultLocation", "application/json", bytes.NewReader(body))
			var reply map[string]any
			if json.Unmarshal(w.Body.Bytes(), &reply) != nil || reply["msgCode"] != float64(200) {
				results <- w.Body.String()
			}
		}(i)
	}
	wg.Wait()
	close(results)
	for result := range results {
		t.Error(result)
	}
	var count int
	if err := f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM namespace_pin WHERE user_id=$1 AND is_default`, f.uid).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}

func TestNamespacePinsMountedFolders(t *testing.T) {
	for _, kind := range []string{"local", "webdav"} {
		t.Run(kind, func(t *testing.T) {
			f := newLibraryTest(t)
			_, endpoint, input := externalBinding(t, f, kind)
			created := f.ok("POST", endpoint, input).(map[string]any)
			sid := int64(created["sourceId"].(float64))
			var mount int64
			if err := f.a.Pool.QueryRow(f.ctx, `SELECT ns_id FROM namespace WHERE source_id=$1 AND is_mount`, sid).Scan(&mount); err != nil {
				t.Fatal(err)
			}
			child := f.folder(mount, "Trips")
			for _, id := range []int64{mount, child} {
				f.ok("POST", "/namespace/pin", map[string]any{"nsId": id})
			}
			f.ok("POST", "/namespace/defaultLocation", map[string]any{"nsId": child})
			if _, err := f.a.Pool.Exec(f.ctx, `UPDATE storage_source SET enabled=false WHERE id=$1`, sid); err != nil {
				t.Fatal(err)
			}
			items, def := pinState(f)
			if len(items) != 2 || def != float64(child) || items[0].(map[string]any)["sourceType"] != kind {
				t.Fatal(items, def)
			}
		})
	}
}
