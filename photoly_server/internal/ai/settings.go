package ai

// Admin settings (ai_control_state.desired) and their validation: app/ai/state.py + limits.py.

import (
	"context"
	"encoding/json"
	"maps"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Q is what both *pgxpool.Pool and pgx.Tx offer.
type Q interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Settings map[string]any

var limitDefaults = map[string]int64{
	"background_interval_seconds": 2, "background_max_per_minute": 0, "cpu_threads": 4,
	"cpu_pause_percent": 85, "cpu_resume_percent": 75, "gpu_pause_percent": 90, "gpu_resume_percent": 80,
	"ram_max_percent": 85, "memory_pause_free_mib": 1024, "memory_resume_free_mib": 2048,
	"vram_pause_free_mib": 1024, "vram_resume_free_mib": 2048, "disk_pause_free_mib": 2048, "disk_resume_free_mib": 3072,
}

var limitBounds = func() map[string][2]int64 {
	b := map[string][2]int64{"background_interval_seconds": {0, 3600}, "background_max_per_minute": {0, 600}, "cpu_threads": {1, 64}}
	for k := range limitDefaults {
		switch {
		case len(k) > 8 && k[len(k)-8:] == "_percent":
			b[k] = [2]int64{1, 100}
		case len(k) > 4 && k[len(k)-4:] == "_mib":
			b[k] = [2]int64{1, 16 * 1024 * 1024}
		}
	}
	b["ram_max_percent"] = [2]int64{6, 100}
	return b
}()

// Devices: CPU everywhere; CUDA (Linux/Windows) and CoreML (macOS) when ONNX Runtime has the
// provider. A device that can't be set up falls back to CPU (see LoadInfo.DeviceNote).
var devicePrecisions = map[string]string{"cpu": "float32", "cuda": "float32", "coreml": "float32"}
var deviceOrder = []string{"cpu", "cuda", "coreml"}

func defaults() Settings {
	s := Settings{
		"enabled": false, "search_enabled": true, "ai_initialized": false, "managed_lifecycle": false, "lifecycle_id": 0,
		"ram_policy": "percent", "new_user_default": false, "smart_tag_enabled": false, "maintenance": false,
		"similarity_threshold": nil, "search_profile_id": nil, "building_profile_id": nil, "quota_bytes": int64(10) << 30,
		"scheduler": "stopped", "runtime": "unloaded", "loaded_profile_id": nil, "device": "cpu", "precision": "float32",
		"placement_image": "user", "placement_text": "user", "browser_text_model": "small",
	}
	for k, v := range limitDefaults {
		s[k] = v
	}
	return s
}

func (s Settings) Int(k string) int64  { n, _ := toInt(s[k]); return n }
func (s Settings) Bool(k string) bool  { b, _ := s[k].(bool); return b }
func (s Settings) Str(k string) string { v, _ := s[k].(string); return v }
func (s Settings) PID(k string) *int64 {
	if n, ok := toInt(s[k]); ok {
		return &n
	}
	return nil
}
func (s Settings) clone() Settings               { return maps.Clone(s) }
func (s Settings) isPID(k string, id int64) bool { p := s.PID(k); return p != nil && *p == id }

// ControlRow is one ai_control_state row.
type ControlRow struct {
	Name       string
	Generation int64
	Desired    Settings
	Observed   map[string]any
	LeaseOwner *string
	LeaseToken *string
	LeaseUntil *time.Time
	Heartbeat  *time.Time
}

// control returns (creating if needed) a control row; lock takes FOR UPDATE.
func control(ctx context.Context, q Q, name string, lock bool) (*ControlRow, error) {
	init := "{}"
	if name == "global" {
		b, _ := json.Marshal(defaults())
		init = string(b)
	}
	if _, err := q.Exec(ctx, `INSERT INTO ai_control_state (name, desired) VALUES ($1, $2::jsonb) ON CONFLICT DO NOTHING`, name, init); err != nil {
		return nil, err
	}
	sql := `SELECT name, generation, desired, observed, lease_owner, lease_token, lease_expires_at, heartbeat_at FROM ai_control_state WHERE name=$1`
	if lock {
		sql += ` FOR UPDATE`
	}
	r := &ControlRow{}
	err := q.QueryRow(ctx, sql, name).Scan(&r.Name, &r.Generation, &r.Desired, &r.Observed, &r.LeaseOwner, &r.LeaseToken, &r.LeaseUntil, &r.Heartbeat)
	return r, err
}

func saveDesired(ctx context.Context, q Q, r *ControlRow) error {
	_, err := q.Exec(ctx, `UPDATE ai_control_state SET desired=$2, generation=$3 WHERE name=$1`, r.Name, map[string]any(r.Desired), r.Generation)
	return err
}

func saveObserved(ctx context.Context, q Q, name string, observed any) error {
	_, err := q.Exec(ctx, `UPDATE ai_control_state SET observed=$2, heartbeat_at=now() WHERE name=$1`, name, observed)
	return err
}

// loadSettings merges defaults, the legacy repairs and the saved values (state.py settings()).
func loadSettings(ctx context.Context, q Q) (Settings, error) {
	var saved map[string]any
	err := q.QueryRow(ctx, `SELECT desired FROM ai_control_state WHERE name='global'`).Scan(&saved)
	out := defaults()
	if err == pgx.ErrNoRows {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	if _, ok := saved["ram_policy"]; !ok {
		out["ram_policy"], out["cpu_pause_percent"], out["cpu_resume_percent"] = "free", int64(90), int64(70)
		out["gpu_pause_percent"], out["gpu_resume_percent"] = int64(95), int64(70)
	}
	if _, ok := saved["ai_initialized"]; !ok {
		sc, _ := saved["scheduler"].(string)
		out["ai_initialized"] = Settings(saved).Bool("enabled") || saved["loaded_profile_id"] != nil || saved["search_profile_id"] != nil || (sc != "" && sc != "stopped")
	}
	maps.Copy(out, saved)
	normalizeLegacyDevice(out, runtime.GOOS)
	return out, nil
}

func isBool(v any) bool { _, ok := v.(bool); return ok }

var settingKeys = func() map[string]bool {
	m := map[string]bool{"enabled": true, "search_enabled": true, "new_user_default": true, "smart_tag_enabled": true, "quota_bytes": true,
		"device": true, "precision": true, "similarity_threshold": true, "placement_image": true, "placement_text": true, "browser_text_model": true}
	for k := range limitDefaults {
		m[k] = true
	}
	return m
}()

func validateSettings(changes map[string]any) error {
	if changes == nil {
		return fail("invalid_settings")
	}
	for k := range changes {
		if !settingKeys[k] {
			return fail("invalid_settings")
		}
	}
	for _, k := range []string{"enabled", "search_enabled", "new_user_default", "smart_tag_enabled"} {
		if v, ok := changes[k]; ok && !isBool(v) {
			return fail("invalid_settings")
		}
	}
	if v, ok := changes["quota_bytes"]; ok {
		if n, ok := toInt(v); !ok || n < 1<<20 || n > int64(100)<<40 {
			return fail("invalid_quota")
		}
	}
	if v, ok := changes["device"]; ok {
		if d, _ := v.(string); devicePrecisions[d] == "" {
			return fail("unsupported_device")
		}
	}
	if v, ok := changes["precision"]; ok {
		if p, _ := v.(string); p != "float32" {
			return fail("unsupported_precision")
		}
	}
	if v := changes["similarity_threshold"]; v != nil {
		if f, ok := toFloat(v); !ok || isBool(v) || f < -1 || f > 1 {
			return fail("invalid_similarity_threshold")
		}
	}
	for _, k := range []string{"placement_image", "placement_text"} {
		if v, ok := changes[k]; ok {
			if s, _ := v.(string); s != "server" && s != "browser" && s != "user" {
				return fail("invalid_placement")
			}
		}
	}
	if v, ok := changes["browser_text_model"]; ok {
		if s, _ := v.(string); s != "small" && s != "full" {
			return fail("invalid_browser_text_model")
		}
	}
	for k, b := range limitBounds {
		if v, ok := changes[k]; ok {
			if n, ok := toInt(v); !ok || n < b[0] || n > b[1] {
				return fail("invalid_resource_limits")
			}
		}
	}
	return nil
}

func validateLimitPairs(c Settings) error {
	for _, r := range []string{"cpu", "gpu"} {
		if c.Int(r+"_resume_percent") >= c.Int(r+"_pause_percent") {
			return fail("invalid_resource_hysteresis")
		}
	}
	for _, r := range []string{"memory", "vram", "disk"} {
		if c.Int(r+"_resume_free_mib") <= c.Int(r+"_pause_free_mib") {
			return fail("invalid_resource_hysteresis")
		}
	}
	return nil
}

// effectivePlacement resolves "user" with the user's own choice (default: browser).
func effectivePlacement(admin, userChoice string) string {
	if admin == "user" {
		if userChoice == "server" {
			return "server"
		}
		return "browser"
	}
	return admin
}

// Python deployments used MPS and reduced precision, which are not ONNX device
// names. Normalize saved settings only; new API input still uses strict validation.
func normalizeLegacyDevice(s Settings, goos string) {
	if s.Str("device") == "mps" {
		s["device"] = "cpu"
		if goos == "darwin" {
			s["device"] = "coreml"
		}
	}
	if s.Str("precision") == "bfloat16" || s.Str("precision") == "float16" {
		s["precision"] = "float32"
	}
}
