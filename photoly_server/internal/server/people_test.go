package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPeopleAccountSessionBoundary(t *testing.T) {
	f := newLibraryTest(t)
	var saved []byte
	if err := f.a.Pool.QueryRow(f.ctx, `SELECT desired FROM ai_control_state WHERE name='global'`).Scan(&saved); err != nil {
		t.Fatal(err)
	}
	defer f.a.Pool.Exec(f.ctx, `UPDATE ai_control_state SET desired=$1 WHERE name='global'`, saved)
	if _, err := f.a.Pool.Exec(f.ctx, `UPDATE ai_control_state SET desired=desired||'{"enabled":true,"people_enabled":true}' WHERE name='global'`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.a.Pool.Exec(f.ctx, `UPDATE "user" SET ai_search_enabled=true WHERE user_id=$1`, f.uid); err != nil {
		t.Fatal(err)
	}
	r := f.request("PATCH", "/ai/people/settings", "application/json", strings.NewReader(`{"enabled":true}`))
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	for _, path := range []string{"/ai/people/settings", "/ai/people/persons", "/ai/people/photos/1", "/ai/people/faces/1/thumbnail"} {
		for _, header := range []bool{false, true} {
			req := httptest.NewRequest("GET", path+"?token=public-photo-share-token", nil)
			if header {
				req.Header.Set("HRD-Token", "public-photo-share-token")
			}
			w := httptest.NewRecorder()
			f.h.ServeHTTP(w, req)
			if w.Code != 401 {
				t.Fatalf("share token authorized %s: %d %s", path, w.Code, w.Body.String())
			}
		}
	}
	// An owned account must not edit or resolve identities in a different account.
	var foreign int64
	if err := f.a.Pool.QueryRow(f.ctx, `INSERT INTO people_person(user_id,name) SELECT user_id,'Private name' FROM "user" WHERE user_id<>$1 LIMIT 1 RETURNING person_id`, f.uid).Scan(&foreign); err != nil {
		t.Fatal(err)
	}
	defer f.a.Pool.Exec(f.ctx, `DELETE FROM people_person WHERE person_id=$1`, foreign)
	for _, method := range []string{"GET", "PATCH"} {
		r = f.request(method, fmt.Sprintf("/ai/people/persons/%d", foreign), "application/json", strings.NewReader(`{"name":"Changed"}`))
		if r.Code != 404 || strings.Contains(r.Body.String(), "Private name") {
			t.Fatal("cross-user person access", r.Code, r.Body.String())
		}
	}
	r = f.request("DELETE", "/ai/people/data", "application/json", strings.NewReader(`{}`))
	if r.Code != 400 {
		t.Fatal("delete without confirmation", r.Body.String())
	}
	r = f.request("GET", "/ai/people/settings", "", nil)
	var result struct {
		T struct {
			Preference struct {
				Enabled bool `json:"enabled"`
			} `json:"preference"`
		} `json:"t"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &result); err != nil || !result.T.Preference.Enabled {
		t.Fatal("failed delete changed preference")
	}
}

func TestCachedFaceStillChecksVisibilityAndVersion(t *testing.T) {
	f := newLibraryTest(t)
	var saved []byte
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT desired FROM ai_control_state WHERE name='global'`).Scan(&saved); e != nil {
		t.Fatal(e)
	}
	defer f.a.Pool.Exec(f.ctx, `UPDATE ai_control_state SET desired=$1 WHERE name='global'`, saved)
	f.a.Pool.Exec(f.ctx, `UPDATE ai_control_state SET desired=desired||'{"enabled":true,"people_enabled":true}' WHERE name='global'`)
	f.a.Pool.Exec(f.ctx, `UPDATE "user" SET ai_search_enabled=true WHERE user_id=$1`, f.uid)
	f.ok("PATCH", "/ai/people/settings", map[string]any{"enabled": true})
	pid := f.upload(f.root, "face.png")
	var person, face int64
	if e := f.a.Pool.QueryRow(f.ctx, `INSERT INTO people_person(user_id) VALUES($1) RETURNING person_id`, f.uid).Scan(&person); e != nil {
		t.Fatal(e)
	}
	if _, e := f.a.Pool.Exec(f.ctx, `INSERT INTO people_media(photo_id,user_id,content_version,fingerprint,generation,status) VALUES($1,$2,1,'test',1,'ready') ON CONFLICT(photo_id) DO NOTHING`, pid, f.uid); e != nil {
		t.Fatal(e)
	}
	if e := f.a.Pool.QueryRow(f.ctx, `INSERT INTO people_face(user_id,photo_id,content_version,detector_version,box,landmarks,score,person_id) VALUES($1,$2,1,'test',ARRAY[0.0,0.0,1.0,1.0],ARRAY[0.0,0.0,0.0,0.0,0.0,0.0,0.0,0.0,0.0,0.0],1,$3) RETURNING face_id`, f.uid, pid, person).Scan(&face); e != nil {
		t.Fatal(e)
	}
	path := fmt.Sprintf("/ai/people/faces/%d/thumbnail", face)
	for range 2 {
		r := f.request("GET", path, "", nil)
		if r.Code != 200 || r.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal(r.Code, r.Body.String())
		}
	}
	f.a.Pool.Exec(f.ctx, `UPDATE photo SET content_version=2 WHERE photo_id=$1`, pid)
	if f.request("GET", path, "", nil).Code == 200 {
		t.Fatal("old version crop exposed")
	}
	f.a.Pool.Exec(f.ctx, `UPDATE photo SET content_version=1 WHERE photo_id=$1`, pid)
	f.a.Pool.Exec(f.ctx, `UPDATE people_face SET dismissed=true WHERE face_id=$1`, face)
	if f.request("GET", path, "", nil).Code == 200 {
		t.Fatal("dismissed crop exposed")
	}
	f.a.Pool.Exec(f.ctx, `UPDATE people_face SET dismissed=false WHERE face_id=$1`, face)
	f.ok("PATCH", "/ai/people/settings", map[string]any{"enabled": false})
	if f.request("GET", path, "", nil).Code == 200 {
		t.Fatal("disabled People crop exposed")
	}
}
