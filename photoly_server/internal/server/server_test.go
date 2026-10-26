package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/config"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/session"
)

// Needs a migrated database (photoly migrate): TEST_DATABASE_URL=postgres://...
func TestAuthAndAdmin(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL")
	}
	cfg := config.Load()
	cfg.DatabaseURL = url
	Registrars = append(Registrars, func(a *app.App) {
		a.Mux.HandleFunc("GET /t/user", a.Auth(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, a.UserFrom(r.Context()).Email) }))
		a.Mux.HandleFunc("GET /t/admin", a.Admin(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") }))
	})
	ctx := context.Background()
	a, h, err := New(ctx, cfg, []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	get := func(path, token string) string {
		req, _ := http.NewRequest("GET", srv.URL+path, nil)
		if token != "" {
			req.Header.Set("HRD-Token", token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	if got := get("/t/user", ""); got != "{\"msgCode\":403,\"t\":null}\n" {
		t.Fatalf("no token: %q", got)
	}
	store := session.Store{Pool: a.Pool}
	var uid int64
	if err := a.Pool.QueryRow(ctx, `SELECT user_id FROM "user" WHERE role='admin' LIMIT 1`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	tok, _ := store.Create(ctx, uid, 1)
	if get("/t/user", tok) == "" || get("/t/admin", tok) != "ok" {
		t.Fatal("valid admin token rejected")
	}
	expired, _ := store.Create(ctx, uid, 0)
	if get("/t/user", expired) != "{\"msgCode\":403,\"t\":null}\n" {
		t.Fatal("zero-day session must be expired")
	}
	store.Delete(ctx, tok)
	if get("/t/user", tok) != "{\"msgCode\":403,\"t\":null}\n" {
		t.Fatal("deleted token accepted")
	}
	if got := get("/not/a/route", ""); got != "404 page not found\n" {
		t.Fatalf("unknown route: %q", got)
	}
	if got := get("/healthz", ""); got != "{\"status\":\"ok\"}\n" {
		t.Fatalf("healthz %q", got)
	}
	// End to end through the real user package: sign in with the seeded admin (ADMIN_PASSWORD).
	if pw := os.Getenv("ADMIN_PASSWORD"); pw != "" {
		resp, err := http.Post(srv.URL+"/user/signIn", "application/json", strings.NewReader(`{"email":"`+cfg.AdminEmail+`","password":"`+pw+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		var out struct {
			Token string `json:"token"`
			Code  int    `json:"msgCode"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		resp.Body.Close()
		if out.Code != 200 || out.Token == "" {
			t.Fatalf("signIn: %+v", out)
		}
		if got := get("/user/getInfo", out.Token); !strings.Contains(got, `"msgCode":200`) {
			t.Fatalf("getInfo: %s", got)
		}
	}
}
