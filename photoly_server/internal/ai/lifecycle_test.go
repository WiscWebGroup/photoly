package ai

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestLegacyDeviceSettings(t *testing.T) {
	for _, tc := range []struct{ device, precision, goos, want string }{
		{"mps", "float32", "linux", "cpu"},
		{"mps", "float16", "darwin", "coreml"},
		{"cuda", "bfloat16", "linux", "cuda"},
		{"cpu", "float32", "linux", "cpu"},
	} {
		s := Settings{"device": tc.device, "precision": tc.precision, "enabled": true}
		normalizeLegacyDevice(s, tc.goos)
		if s.Str("device") != tc.want || s.Str("precision") != "float32" || !s.Bool("enabled") {
			t.Fatalf("%+v: %+v", tc, s)
		}
		if err := checkCommand("load", runtimeConfig(s)); err != nil {
			t.Fatal(err)
		}
	}
	if err := validateSettings(map[string]any{"device": "mps"}); err == nil {
		t.Fatal("new requests must still reject unsupported devices")
	}
}

func TestStartupStates(t *testing.T) {
	for _, tc := range []struct {
		name, state, want      string
		enabled, managed, same bool
	}{
		{"legacy enabled but unloaded", "unloaded", "preparing", true, false, false},
		{"enabled after pause", "unloaded", "preparing", true, true, false},
		{"loading", "loading", "loading", true, true, true},
		{"failed", "failed", "failed", true, true, true},
		{"ready", "ready", "ready", true, true, true},
		{"pause in progress", "ready", "pausing", false, true, false},
		{"paused", "unloaded", "paused", false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Settings{"enabled": tc.enabled, "managed_lifecycle": tc.managed, "runtime": "loaded"}
			got := startupStage(cfg, map[string]any{"state": tc.state}, tc.same)
			if got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestInvalidRuntimeConfigurationReportsFailure(t *testing.T) {
	r := NewRuntime(t.TempDir())
	defer r.Close()
	if _, err := r.Submit("load", nil, map[string]any{"device": "invalid", "precision": "float32"}, 7, "invalid"); err == nil {
		t.Fatal("accepted invalid device")
	}
	st := r.Status()
	if st["state"] != "failed" || st["reason"] != "unsupported_device_precision" || st["generation"] != int64(7) {
		t.Fatalf("invalid configuration stays visibly failed: %+v", st)
	}
	if _, err := r.Submit("load", nil, map[string]any{"device": "cpu", "precision": "float32"}, 8, "retry"); err != nil {
		t.Fatal(err)
	}
	awaitRuntime(t, r, "ready")
	if _, err := r.Submit("unload", nil, nil, 9, "pause"); err != nil {
		t.Fatal(err)
	}
	awaitRuntime(t, r, "unloaded")
	if _, err := r.Submit("load", nil, map[string]any{"device": "cpu", "precision": "float32"}, 10, "enable-again"); err != nil {
		t.Fatal(err)
	}
	awaitRuntime(t, r, "ready")
}

func awaitRuntime(t *testing.T, r *Runtime, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if r.Status()["state"] == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("runtime did not reach %s: %+v", want, r.Status())
}

func TestRuntimeStartsWithoutSchedulerLease(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "ai_lifecycle_" + itoa(int(time.Now().UnixNano()))
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`)
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `CREATE TABLE ai_control_state(name text PRIMARY KEY,generation bigint NOT NULL DEFAULT 0,desired jsonb NOT NULL DEFAULT '{}',observed jsonb NOT NULL DEFAULT '{}',lease_owner text,lease_token text,lease_expires_at timestamptz,heartbeat_at timestamptz)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO ai_control_state(name,desired) VALUES ('global','{"enabled":true,"managed_lifecycle":true,"lifecycle_id":2,"runtime":"loaded","loaded_profile_id":1,"device":"mps","placement_image":"browser","placement_text":"browser"}');
 INSERT INTO ai_control_state(name,lease_owner,lease_token,lease_expires_at) VALUES ('scheduler','old-process','old-token',now()+interval '120 seconds')`)
	if err != nil {
		t.Fatal(err)
	}
	r := NewRuntime(t.TempDir())
	defer r.Close()
	s := &Scheduler{pool: pool, rt: r}
	if err = s.runtimeTick(ctx); err != nil {
		t.Fatal(err)
	}
	awaitRuntime(t, r, "ready")
	var owner string
	if err = pool.QueryRow(ctx, `SELECT lease_owner FROM ai_control_state WHERE name='scheduler'`).Scan(&owner); err != nil || owner != "old-process" {
		t.Fatalf("runtime must not steal background leadership: %s %v", owner, err)
	}
}

func TestBackgroundIndexDoesNotDelayAIReadiness(t *testing.T) {
	cfg := Settings{"enabled": true, "managed_lifecycle": true, "search_profile_id": 1, "building_profile_id": 2, "maintenance": false}
	rt := map[string]any{"state": "ready"}
	if got := startupStage(cfg, rt, true); got != "ready" {
		t.Fatalf("existing searchable index should be ready, got %s", got)
	}
	cfg["maintenance"] = true
	if got := startupStage(cfg, rt, true); got != "preparing_index" {
		t.Fatalf("unavailable index must still prepare, got %s", got)
	}
	cfg["maintenance"] = false
	cfg["search_profile_id"] = nil
	if got := startupStage(cfg, rt, true); got != "preparing_index" {
		t.Fatalf("first index must still prepare, got %s", got)
	}
}
