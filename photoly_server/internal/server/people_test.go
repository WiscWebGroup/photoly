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
