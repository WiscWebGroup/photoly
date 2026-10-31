package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
)

func TestDemoGuard(t *testing.T) {
	users := map[string]app.User{
		"demo":  {Email: "Demo@A.com", Role: "admin"},
		"admin": {Email: "admin@a.com", Role: "admin"},
	}
	lookup := func(_ context.Context, tok string) (app.User, bool) { u, ok := users[tok]; return u, ok }
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	h := demoGuard("demo@a.com", lookup, ok)
	for _, c := range []struct {
		method, path, token string
		blocked             bool
	}{
		{"PUT", "/admin/users/3/storage", "demo", true},
		{"POST", "/admin/user-groups", "demo", true},
		{"PUT", "/admin/user-groups/2", "demo", true},
		{"DELETE", "/admin/user-groups/2", "demo", true},
		{"POST", "/admin/addUser", "demo", true},
		{"POST", "/admin/resetUserRole", "demo", true},
		{"POST", "/admin/resetUserPassword", "demo", true},
		{"POST", "/admin/setGeneralSetting", "demo", true},
		{"PATCH", "/admin/ai/settings", "demo", true},
		{"PUT", "/admin//users/3/storage", "demo", true},
		{"PUT", "/x/../admin/users/3/storage", "demo", true},
		{"POST", "/user/updateEmail", "demo", true},
		{"POST", "/user/updatePassword", "demo", true},
		{"GET", "/user/deleteUser", "demo", true},
		{"GET", "/admin/getUsers", "demo", false},
		{"GET", "/admin/user-groups", "demo", false},
		{"POST", "/admin/getGeneralSetting", "demo", false},
		{"POST", "/user/updateUsername", "demo", false},
		{"PUT", "/admin/users/3/storage", "admin", false},
		{"POST", "/admin/addUser", "admin", false},
		{"POST", "/admin/addUser", "", false}, // unauthenticated: the route's own 403 applies
	} {
		req := httptest.NewRequest(c.method, c.path, nil)
		req.Header.Set("HRD-Token", c.token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if got := w.Body.String() != "ok"; got != c.blocked {
			t.Errorf("%s %s as %q: blocked=%v, want %v", c.method, c.path, c.token, got, c.blocked)
		}
	}
	// Sign-up is closed for everyone in demo mode, and untouched otherwise.
	req := httptest.NewRequest("POST", "/user/signUp", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Body.String() == "ok" {
		t.Error("sign-up must be closed in demo mode")
	}
	w = httptest.NewRecorder()
	demoGuard("", lookup, ok).ServeHTTP(w, req)
	if w.Body.String() != "ok" {
		t.Error("empty DEMO_EMAIL must pass everything through")
	}
}
