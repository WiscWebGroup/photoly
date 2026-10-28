package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/config"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/session"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
)

// Needs a migrated database: TEST_DATABASE_URL=postgres://...
func TestStorageAdmin(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL")
	}
	t.Setenv("STORAGE_SECRET_KEY", "test-key")
	cfg := config.Load()
	cfg.DatabaseURL, cfg.UploadFolder = url, t.TempDir()
	ctx := context.Background()
	a, h, err := New(ctx, cfg, []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		a.Pool.Exec(ctx, `UPDATE storage_config SET driver='local', backend_id='local', config='{}', secret=NULL WHERE id=1`)
		storage.Set("local", storage.Local{Dir: cfg.UploadFolder})
	})
	srv := httptest.NewServer(h)
	defer srv.Close()

	var uid int64
	if err := a.Pool.QueryRow(ctx, `SELECT user_id FROM "user" WHERE role='admin' LIMIT 1`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	tok, _ := session.Store{Pool: a.Pool}.Create(ctx, uid, 1)
	do := func(method, path, body string) map[string]any {
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		req.Header.Set("HRD-Token", tok)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var m map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
			t.Fatal(err)
		}
		return m
	}

	m := do("GET", "/admin/storage", "")
	t3 := m["t"].(map[string]any)
	if m["msgCode"] != 200.0 || t3["driver"] != "local" || t3["active"] != "local" || t3["canSave"] != true {
		t.Fatalf("GET = %v", m)
	}
	if len(t3["drivers"].([]any)) < 2 {
		t.Fatalf("drivers = %v", t3["drivers"])
	}

	if m := do("PUT", "/admin/storage", `{"driver":"nope","values":{}}`); m["t"] != "unknown_driver" {
		t.Fatalf("unknown driver = %v", m)
	}
	if m := do("PUT", "/admin/storage", `{"driver":"s3","values":{"bucket":"b"}}`); m["msgCode"] != 400.0 || !strings.Contains(m["t"].(string), "Endpoint") {
		t.Fatalf("missing field = %v", m)
	}
	// Unreachable endpoint: rejected, and the saved config stays local.
	bad := `{"driver":"s3","values":{"endpoint":"127.0.0.1:1","bucket":"b","access_key":"a","secret_key":"TOPSECRET","use_ssl":false}}`
	if m := do("PUT", "/admin/storage", bad); m["msgCode"] != 400.0 {
		t.Fatalf("unreachable s3 saved: %v", m)
	}
	if m := do("POST", "/admin/storage/test", bad); m["msgCode"] != 400.0 {
		t.Fatalf("test of unreachable s3 = %v", m)
	}
	if storage.ActiveDriver() != "local" {
		t.Fatal("failed save must not switch the backend")
	}

	// Without a secret key, credentials cannot be stored.
	t.Setenv("STORAGE_SECRET_KEY", "")
	if m := do("PUT", "/admin/storage", bad); m["t"] != "storage_secret_key_missing" {
		t.Fatalf("no key = %v", m)
	}

	if m := do("PUT", "/admin/storage", `{"driver":"local","values":{}}`); m["msgCode"] != 200.0 {
		t.Fatalf("PUT local = %v", m)
	}

	// Real S3 round trip when an endpoint is provided (see TestS3Contract for setup).
	if ep := os.Getenv("PHOTOLY_TEST_S3_ENDPOINT"); ep != "" {
		t.Setenv("STORAGE_SECRET_KEY", "test-key")
		good := map[string]any{"driver": "s3", "values": map[string]any{
			"endpoint": ep, "bucket": os.Getenv("PHOTOLY_TEST_S3_BUCKET"), "access_key": os.Getenv("PHOTOLY_TEST_S3_ACCESS_KEY"),
			"secret_key": os.Getenv("PHOTOLY_TEST_S3_SECRET_KEY"), "use_ssl": false, "path_style": true}}
		b, _ := json.Marshal(good)
		// Changing backend hides media on the old one, so it must be confirmed.
		if m := do("PUT", "/admin/storage", string(b)); m["msgCode"] != 409.0 || m["t"] != "storage_switch_confirm" || storage.ActiveDriver() != "local" {
			t.Fatalf("unconfirmed switch = %v", m)
		}
		good["confirm"] = true
		b, _ = json.Marshal(good)
		if m := do("PUT", "/admin/storage", string(b)); m["msgCode"] != 200.0 {
			t.Fatalf("PUT s3 = %v", m)
		}
		if storage.ActiveDriver() != "s3" {
			t.Fatal("s3 should be active")
		}
		// Re-save with the secret left blank keeps the stored one.
		good["values"].(map[string]any)["secret_key"] = ""
		b, _ = json.Marshal(good)
		if m := do("PUT", "/admin/storage", string(b)); m["msgCode"] != 200.0 {
			t.Fatalf("PUT s3 blank secret = %v", m)
		}
		m := do("GET", "/admin/storage", "")
		raw, _ := json.Marshal(m)
		if strings.Contains(string(raw), os.Getenv("PHOTOLY_TEST_S3_SECRET_KEY")) || m["t"].(map[string]any)["hasSecret"] != true {
			t.Fatalf("GET leaks or loses the secret: %s", raw)
		}
		// A restart picks the saved backend up.
		storage.Set("local", storage.Local{Dir: cfg.UploadFolder})
		if err := storage.Startup(ctx, a.Pool); err != nil || storage.ActiveDriver() != "s3" {
			t.Fatalf("Startup = %v, active %s", err, storage.ActiveDriver())
		}
	}
}
