package ai

// Admin AI API (services/ai_admin.py): settings, profiles, operations, status, jobs and the
// per-model load/unload used by the inference card.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var commands = map[string]bool{"ai.enable": true, "ai.pause": true, "model.load": true, "model.unload": true, "model.restart": true,
	"scheduler.start": true, "scheduler.pause": true, "scheduler.resume": true, "scheduler.stop": true, "maintenance.enter": true,
	"maintenance.exit": true, "backfill": true, "retry_failed": true, "rebuild": true, "activate": true, "purge": true}

type Admin struct {
	pool  *pgxpool.Pool
	rt    *Runtime
	smart *Smart
	root  string
}

func (a *Admin) tx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (a *Admin) GetSettings(ctx context.Context) (map[string]any, error) {
	row, err := control(ctx, a.pool, "global", false)
	if err != nil {
		return nil, err
	}
	cfg, err := loadSettings(ctx, a.pool)
	if err != nil {
		return nil, err
	}
	return merged(row.Generation, cfg), nil
}

func merged(generation int64, cfg Settings) map[string]any {
	out := map[string]any{"generation": generation}
	for k, v := range cfg {
		out[k] = v
	}
	return out
}

func (a *Admin) PatchSettings(ctx context.Context, generation any, raw any) (res map[string]any, err error) {
	changes, _ := raw.(map[string]any)
	if err := validateSettings(changes); err != nil {
		return nil, err
	}
	err = a.tx(ctx, func(tx pgx.Tx) error {
		row, err := control(ctx, tx, "global", true)
		if err != nil {
			return err
		}
		if g, ok := toInt(generation); !ok || g != row.Generation {
			return fail("generation_conflict", 409)
		}
		current, err := loadSettings(ctx, tx)
		if err != nil {
			return err
		}
		proposed := current.clone()
		for k, v := range changes {
			proposed[k] = v
		}
		if _, ok := changes["ram_max_percent"]; ok {
			proposed["ram_policy"] = "percent"
		} else if _, a := changes["memory_pause_free_mib"]; a {
			proposed["ram_policy"] = "free"
		} else if _, b := changes["memory_resume_free_mib"]; b {
			proposed["ram_policy"] = "free"
		}
		for _, r := range []string{"cpu", "gpu"} {
			pause, resume := r+"_pause_percent", r+"_resume_percent"
			if v, ok := changes[pause]; ok {
				if _, both := changes[resume]; !both {
					n, _ := toInt(v)
					if n < 11 {
						return fail("invalid_resource_limits")
					}
					proposed[resume] = n - 10
				}
			}
		}
		if err := validateLimitPairs(proposed); err != nil {
			return err
		}
		if current.Str("runtime") == "loaded" && current.Int("cpu_threads") != proposed.Int("cpu_threads") {
			return fail("unload_before_thread_change", 409)
		}
		if devicePrecisions[proposed.Str("device")] != proposed.Str("precision") {
			return fail("unsupported_device_precision")
		}
		_, dev := changes["device"]
		_, prec := changes["precision"]
		if proposed.Str("runtime") == "loaded" && (dev || prec) {
			return fail("unload_before_device_change", 409)
		}
		if en, ok := changes["enabled"]; ok && current.Bool("managed_lifecycle") && en != current["enabled"] {
			proposed["lifecycle_id"] = row.Generation + 1
			if proposed.Bool("enabled") {
				proposed["runtime"] = "loaded"
				id := current.PID("building_profile_id")
				if id == nil {
					id = current.PID("search_profile_id")
				}
				if id == nil {
					p, err := createProfile(ctx, tx, "siglip2")
					if err != nil {
						return err
					}
					id = &p.ID
					proposed["building_profile_id"] = *id
				}
				proposed["loaded_profile_id"] = *id
			} else {
				proposed["runtime"], proposed["loaded_profile_id"] = "unloaded", nil
			}
		}
		row.Generation++
		row.Desired = proposed
		if err := saveDesired(ctx, tx, row); err != nil {
			return err
		}
		res = merged(row.Generation, proposed)
		return nil
	})
	return
}

func unit(key string) string {
	switch {
	case strings.HasSuffix(key, "_mib"):
		return "MiB"
	case strings.HasSuffix(key, "_percent"):
		return "%"
	case strings.HasSuffix(key, "_seconds"):
		return "seconds"
	}
	return "count"
}

func (a *Admin) Capabilities() map[string]any {
	fields := map[string]any{}
	for key, def := range limitDefaults {
		var pair any
		var rel any
		switch {
		case strings.Contains(key, "pause"):
			pair = strings.Replace(key, "pause", "resume", 1)
		case strings.Contains(key, "resume"):
			pair = strings.Replace(key, "resume", "pause", 1)
		}
		if pair != nil {
			resume := strings.Contains(key, "resume")
			if strings.HasSuffix(key, "_percent") {
				rel = map[bool]string{true: "less", false: "greater"}[resume]
			} else {
				rel = map[bool]string{true: "greater", false: "less"}[resume]
			}
		}
		apply := "next_cycle"
		if key == "cpu_threads" {
			apply = "model_reload"
		}
		b := limitBounds[key]
		fields[key] = map[string]any{"default": def, "minimum": b[0], "maximum": b[1], "unit": unit(key), "apply": apply, "pairedWith": pair, "relation": rel}
	}
	fields["quota_bytes"] = map[string]any{"default": int64(10) << 30, "minimum": 1 << 20, "maximum": int64(100) << 40, "unit": "bytes", "apply": "next_cycle", "pairedWith": nil, "relation": nil}
	fields["similarity_threshold"] = map[string]any{"default": nil, "minimum": -1, "maximum": 1, "unit": "cosine similarity", "apply": "next_query", "pairedWith": nil, "relation": nil}
	st := a.rt.Status()
	return map[string]any{"fields": fields, "devices": st["devices"], "verified": st["verified"], "maxConcurrency": 1, "batchSize": 1}
}

func (a *Admin) Profiles(ctx context.Context) (map[string]any, error) {
	cfg, err := loadSettings(ctx, a.pool)
	if err != nil {
		return nil, err
	}
	rs := a.rt.Status()
	rows, err := a.pool.Query(ctx, `SELECT `+profileCols+` FROM embedding_profile ORDER BY profile_id`)
	if err != nil {
		return nil, err
	}
	var ps []*profileRow
	for rows.Next() {
		p := &profileRow{}
		if err := p.scan(rows); err != nil {
			rows.Close()
			return nil, err
		}
		ps = append(ps, p)
	}
	rows.Close()
	out := []map[string]any{}
	for _, p := range ps {
		counts, err := coverage(ctx, a.pool, &p.ID, nil)
		if err != nil {
			return nil, err
		}
		activation := []string{}
		if p.IndexStatus != "ready" {
			activation = append(activation, "index_not_ready")
		}
		if rs["state"] != "ready" || rs["fingerprint"] != p.Fingerprint {
			activation = append(activation, "model_not_ready")
		}
		if counts["pending"].(int64) > 0 {
			activation = append(activation, "pending_images")
		}
		if counts["failed"].(int64) > 0 {
			activation = append(activation, "failed_images")
		}
		purge := []string{}
		if cfg.isPID("search_profile_id", p.ID) || cfg.isPID("building_profile_id", p.ID) || cfg.isPID("loaded_profile_id", p.ID) {
			purge = append(purge, "profile_in_use")
		}
		var busy bool
		if err := a.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM ai_job WHERE profile_id=$1 AND kind='search' AND status='running')`, p.ID).Scan(&busy); err != nil {
			return nil, err
		}
		if busy {
			purge = append(purge, "requests_in_flight")
		}
		d := p.dto()
		d["coverage"], d["activationBlockers"], d["purgeBlockers"] = counts, activation, purge
		out = append(out, d)
	}
	return map[string]any{"registry": []any{ONNXProfile().Spec}, "profiles": out}, nil
}

// createProfile registers the provider's spec (one profile per fingerprint) and queues its index.
func createProfile(ctx context.Context, q Q, provider any) (*profileRow, error) {
	spec, err := profileSpec(provider)
	if err != nil {
		return nil, err
	}
	fp := spec["fingerprint"].(string)
	p := &profileRow{}
	err = p.scan(q.QueryRow(ctx, `SELECT `+profileCols+` FROM embedding_profile WHERE fingerprint=$1`, fp))
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if _, err = q.Exec(ctx, `INSERT INTO embedding_profile (provider, dimension, fingerprint, spec) VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
		provider, embedDim, fp, spec); err != nil {
		return nil, err
	}
	if err = p.scan(q.QueryRow(ctx, `SELECT `+profileCols+` FROM embedding_profile WHERE fingerprint=$1`, fp)); err != nil {
		return nil, err
	}
	_, err = q.Exec(ctx, `INSERT INTO ai_job (kind, profile_id, dedup_key) VALUES ('index',$1,$2) ON CONFLICT DO NOTHING`, p.ID, fmt.Sprintf("index:%d", p.ID))
	return p, err
}

func (a *Admin) CreateProfile(ctx context.Context, provider any) (map[string]any, error) {
	var out map[string]any
	err := a.tx(ctx, func(tx pgx.Tx) error {
		p, err := createProfile(ctx, tx, provider)
		if err == nil {
			out = p.dto()
		}
		return err
	})
	return out, err
}

func (a *Admin) SetUser(ctx context.Context, userID int64, enabled any) (map[string]any, error) {
	on, ok := enabled.(bool)
	if !ok {
		return nil, fail("invalid_enabled")
	}
	err := a.tx(ctx, func(tx pgx.Tx) error {
		var id int64
		if err := tx.QueryRow(ctx, `SELECT user_id FROM "user" WHERE user_id=$1 FOR UPDATE`, userID).Scan(&id); errors.Is(err, pgx.ErrNoRows) {
			return fail("user_not_found", 404)
		} else if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE "user" SET ai_search_enabled=$2 WHERE user_id=$1`, userID, on); err != nil {
			return err
		}
		if !on {
			_, err := tx.Exec(ctx, `UPDATE ai_job SET status='cancelled', reserved_bytes=0, finished_at=now()
				WHERE photo_id IN (SELECT photo_id FROM photo WHERE user_id=$1) AND status IN ('queued','running')`, userID)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Reconciliation pages eligible users, so enabling never enqueues an unbounded library here.
	return map[string]any{"userId": userID, "enabled": on}, nil
}

func (a *Admin) UserStates(ctx context.Context, ids []int64) ([]map[string]any, error) {
	if len(ids) > 100 {
		return nil, fail("invalid_user_ids")
	}
	for _, id := range ids {
		if id <= 0 {
			return nil, fail("invalid_user_ids")
		}
	}
	rows, err := a.pool.Query(ctx, `SELECT user_id, ai_search_enabled FROM "user" WHERE user_id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var on bool
		if err := rows.Scan(&id, &on); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"userId": id, "enabled": on})
	}
	return out, rows.Err()
}

func canonical(v any) string {
	raw, _ := json.Marshal(v) // maps marshal with sorted keys
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (a *Admin) Operation(ctx context.Context, userID int64, data map[string]any, key string) (map[string]any, error) {
	if l := len(key); l < 1 || l > 80 {
		return nil, fail("idempotency_key_required")
	}
	command, _ := data["command"].(string)
	var pid *int64
	if v, ok := data["profileId"]; ok && v != nil {
		n, ok := toInt(v)
		if !ok || n <= 0 {
			return nil, fail("invalid_operation")
		}
		pid = &n
	}
	generation, genOK := toInt(data["generation"])
	if !commands[command] || !genOK {
		return nil, fail("invalid_operation")
	}
	hash := canonical(data)
	dedup := fmt.Sprintf("operation:%d:%s", userID, key)
	var out map[string]any
	err := a.tx(ctx, func(tx pgx.Tx) error {
		row, err := control(ctx, tx, "global", true)
		if err != nil {
			return err
		}
		old := &jobRow{}
		var oldPayload map[string]any
		err = tx.QueryRow(ctx, `SELECT `+jobCols+` FROM ai_job WHERE kind='operation' AND dedup_key=$1`, dedup).
			Scan(&old.ID, &old.Kind, &old.PhotoID, &old.ProfileID, &old.Payload, &old.Result, &old.Status, &old.Attempts, &old.CreatedAt, &old.FinishedAt)
		if err == nil {
			oldPayload = old.Payload
			if oldPayload["request_hash"] != hash {
				return fail("idempotency_conflict", 409)
			}
			out = old.dto()
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if generation != row.Generation {
			return fail("generation_conflict", 409)
		}
		cfg, err := loadSettings(ctx, tx)
		if err != nil {
			return err
		}
		if pid != nil {
			var one int
			if err := tx.QueryRow(ctx, `SELECT 1 FROM embedding_profile WHERE profile_id=$1`, *pid).Scan(&one); errors.Is(err, pgx.ErrNoRows) {
				return fail("profile_not_found", 404)
			} else if err != nil {
				return err
			}
		}
		switch command {
		case "model.load", "backfill", "retry_failed", "rebuild", "activate", "purge":
			if pid == nil {
				return fail("profile_required")
			}
		}
		if command == "model.restart" && pid == nil {
			pid = cfg.PID("loaded_profile_id")
		}
		switch command {
		case "model.load", "model.restart", "backfill", "retry_failed", "rebuild":
			if sp := cfg.PID("search_profile_id"); sp != nil && (pid == nil || *sp != *pid) && !cfg.Bool("maintenance") {
				return fail("maintenance_required", 409)
			}
		}
		if command == "rebuild" && !cfg.Bool("maintenance") {
			return fail("maintenance_required", 409)
		}
		if strings.HasPrefix(command, "model.") || command == "activate" || command == "maintenance.enter" {
			cfg["managed_lifecycle"] = false
		}
		next := row.Generation + 1
		switch {
		case command == "ai.enable":
			if provider, ok := data["provider"]; ok && provider != nil {
				p, err := createProfile(ctx, tx, provider)
				if err != nil {
					return err
				}
				pid = &p.ID
			} else {
				if pid == nil {
					for _, k := range []string{"loaded_profile_id", "building_profile_id", "search_profile_id"} {
						if pid = cfg.PID(k); pid != nil {
							break
						}
					}
				}
			}
			// Only this binary's own (ONNX) profile can be loaded: an earlier PyTorch profile
			// keeps serving search while a new one is built beside it.
			if pid != nil {
				p, err := getProfile(ctx, tx, *pid)
				if err != nil {
					return err
				}
				if p.Fingerprint != ONNXProfile().Fingerprint {
					pid = nil
				}
			}
			if pid == nil {
				p, err := createProfile(ctx, tx, "siglip2")
				if err != nil {
					return err
				}
				pid = &p.ID
			}
			if _, err := tx.Exec(ctx, `UPDATE ai_job SET status='queued', attempts=0, finished_at=NULL, next_run_at=now(), result='{}'
				WHERE kind='index' AND profile_id=$1 AND status IN ('failed','cancelled')`, *pid); err != nil {
				return err
			}
			if !cfg.Bool("ai_initialized") {
				cfg["scheduler"] = "running"
			}
			sp := cfg.PID("search_profile_id")
			differs := sp == nil || *sp != *pid
			maint := differs
			if sp != nil && differs {
				if p, err := getProfile(ctx, tx, *sp); err == nil && onnxCompatible(p.Spec) {
					maint = false // the old index stays searchable through the ONNX encoder
				}
			}
			cfg["enabled"], cfg["ai_initialized"], cfg["managed_lifecycle"] = true, true, true
			cfg["lifecycle_id"], cfg["runtime"], cfg["loaded_profile_id"] = next, "loaded", *pid
			cfg["building_profile_id"] = nil
			if differs {
				cfg["building_profile_id"] = *pid
			}
			cfg["maintenance"] = maint
		case command == "ai.pause":
			cfg["enabled"], cfg["managed_lifecycle"], cfg["lifecycle_id"] = false, true, next
			cfg["runtime"], cfg["loaded_profile_id"] = "unloaded", nil
		case strings.HasPrefix(command, "scheduler."):
			cfg["ai_initialized"] = true
			cfg["scheduler"] = map[string]string{"start": "running", "resume": "running", "pause": "paused", "stop": "stopped"}[strings.TrimPrefix(command, "scheduler.")]
		case command == "maintenance.enter":
			cfg["maintenance"] = true
		case command == "maintenance.exit":
			if b := cfg.PID("building_profile_id"); b != nil && !cfg.isPID("search_profile_id", *b) {
				return fail("building_profile_not_activated", 409)
			}
			cfg["maintenance"] = false
		case command == "model.load" || command == "model.restart":
			if pid == nil {
				pid = cfg.PID("loaded_profile_id")
			}
			if pid == nil {
				return fail("profile_required")
			}
			cfg["runtime"], cfg["loaded_profile_id"] = "loaded", *pid
			if !cfg.isPID("search_profile_id", *pid) {
				cfg["building_profile_id"] = *pid
			}
		case command == "model.unload":
			cfg["runtime"], cfg["loaded_profile_id"] = "unloaded", nil
		case command == "backfill" || command == "retry_failed" || command == "rebuild":
			cfg["building_profile_id"] = *pid
		}
		if command == "purge" && (cfg.isPID("search_profile_id", *pid) || cfg.isPID("building_profile_id", *pid) || cfg.isPID("loaded_profile_id", *pid)) {
			return fail("profile_in_use", 409)
		}
		row.Generation = next
		row.Desired = cfg
		if err := saveDesired(ctx, tx, row); err != nil {
			return err
		}
		if strings.HasPrefix(command, "ai.") {
			if _, err := tx.Exec(ctx, `UPDATE ai_job SET status='cancelled', finished_at=now(), result='{"reason":"superseded"}'
				WHERE kind='operation' AND status IN ('queued','running') AND payload->>'command' IN ('ai.enable','ai.pause')`); err != nil {
				return err
			}
		}
		priority := 0
		if command == "ai.pause" {
			priority = 100
		}
		var pidArg any
		if pid != nil {
			pidArg = *pid
		}
		job := &jobRow{}
		err = job.scan(tx.QueryRow(ctx, `INSERT INTO ai_job (kind, priority, profile_id, dedup_key, payload) VALUES ('operation',$1,$2,$3,$4) RETURNING `+jobCols,
			priority, pidArg, dedup, map[string]any{"command": command, "generation": row.Generation, "actor": userID, "request_hash": hash, "cursor": 0}))
		if err != nil {
			return err
		}
		out = job.dto()
		return nil
	})
	return out, err
}

func (a *Admin) GetOperation(ctx context.Context, id int64) (map[string]any, error) {
	j := &jobRow{}
	err := j.scan(a.pool.QueryRow(ctx, `SELECT `+jobCols+` FROM ai_job WHERE job_id=$1 AND kind='operation'`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fail("operation_not_found", 404)
	}
	if err != nil {
		return nil, err
	}
	return j.dto(), nil
}

func (a *Admin) Jobs(ctx context.Context, q map[string]string) (map[string]any, error) {
	num := func(key, def string) (int64, error) {
		v, ok := q[key]
		if !ok {
			v = def
		}
		n, ok := toInt(json.Number(strings.TrimSpace(v)))
		if !ok {
			return 0, fail("invalid_pagination")
		}
		return n, nil
	}
	limit, err := num("limit", "50")
	if err != nil {
		return nil, err
	}
	after, err := num("after", "0")
	if err != nil {
		return nil, err
	}
	before, err := num("before", "0")
	if err != nil {
		return nil, err
	}
	order := q["order"]
	if order == "" {
		order = "asc"
	}
	if limit < 1 || limit > 100 || after < 0 || before < 0 || (order != "asc" && order != "desc") || (order == "desc" && after != 0) || (order == "asc" && before != 0) {
		return nil, fail("invalid_pagination")
	}
	sql, args := `SELECT `+jobCols+` FROM ai_job WHERE true`, []any{}
	add := func(cond string, v any) { args = append(args, v); sql += " AND " + fmt.Sprintf(cond, len(args)) }
	if order == "asc" {
		add("job_id>$%d", after)
	} else if before != 0 {
		add("job_id<$%d", before)
	}
	for _, key := range []string{"status", "kind"} {
		if v, ok := q[key]; ok {
			add(key+"=$%d", v)
		}
	}
	dir := "ASC"
	if order == "desc" {
		dir = "DESC"
	}
	args = append(args, limit)
	rows, err := a.pool.Query(ctx, sql+" ORDER BY job_id "+dir+fmt.Sprintf(" LIMIT $%d", len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	var last int64
	for rows.Next() {
		j := &jobRow{}
		if err := j.scan(rows); err != nil {
			return nil, err
		}
		items = append(items, j.dto())
		last = j.ID
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var nextAfter, nextBefore any
	if int64(len(items)) == limit {
		if order == "asc" {
			nextAfter = last
		} else {
			nextBefore = last
		}
	}
	return map[string]any{"items": items, "nextAfter": nextAfter, "nextBefore": nextBefore}, nil
}

func (a *Admin) service(ctx context.Context, name string, now time.Time) (map[string]any, error) {
	row, err := control(ctx, a.pool, name, false)
	if err != nil {
		return nil, err
	}
	fresh := row.Heartbeat != nil && now.Sub(*row.Heartbeat) < 60*time.Second
	observed := map[string]any{"state": "offline"}
	if fresh {
		observed = row.Observed
	}
	state, _ := observed["state"].(string)
	return map[string]any{"online": fresh && state != "offline" && state != "unknown", "heartbeatAt": isoPtr(row.Heartbeat), "observed": observed}, nil
}

// indexSources counts finished vectors by where they were computed, and what waits for a browser.
func (a *Admin) indexSources(ctx context.Context, cfg Settings) (map[string]any, error) {
	out := map[string]any{"server": 0, "browser": 0, "waiting": 0, "spotCheck": nil}
	pid := cfg.PID("building_profile_id")
	if pid == nil {
		pid = cfg.PID("search_profile_id")
	}
	if pid == nil {
		return out, nil
	}
	var server, browser, waiting int64
	if err := a.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE s.status='ready' AND s.source='server'), count(*) FILTER (WHERE s.status='ready' AND s.source='client'),
		count(*) FILTER (WHERE s.status='pending' AND ($2='browser' OR ($2='user' AND COALESCE(up.image,'server')='browser')))
		FROM photo_embedding_state s JOIN photo p ON p.photo_id=s.photo_id JOIN "user" u ON u.user_id=p.user_id AND u.ai_search_enabled
		LEFT JOIN user_ai_preference up ON up.user_id=u.user_id WHERE s.profile_id=$1 AND s.content_version=p.content_version`, *pid, cfg.Str("placement_image")).
		Scan(&server, &browser, &waiting); err != nil {
		return nil, err
	}
	out["server"], out["browser"], out["waiting"] = server, browser, waiting
	row, err := control(ctx, a.pool, "spot_check", false)
	if err != nil {
		return nil, err
	}
	if c, ok := toInt(row.Observed["checked"]); ok && c > 0 {
		m, _ := toInt(row.Observed["matched"])
		out["spotCheck"] = map[string]any{"checked": c, "matched": m, "at": row.Observed["at"]}
	}
	return out, nil
}

func (a *Admin) Status(ctx context.Context) (map[string]any, error) {
	now := time.Now()
	services := map[string]any{}
	for _, name := range []string{"scheduler", "maintenance"} {
		s, err := a.service(ctx, name, now)
		if err != nil {
			return nil, err
		}
		services[name] = s
	}
	rt := a.rt.Status()
	services["runtime"] = map[string]any{"online": rt["state"] != "failed" || rt["reason"] != "runtime_unavailable", "heartbeatAt": iso(now), "observed": rt}
	cfgs, err := a.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	cfg, err := loadSettings(ctx, a.pool)
	if err != nil {
		return nil, err
	}
	generation, _ := rt["generation"].(int64)
	sameLoad := generation == cfg.Int("lifecycle_id")
	stage := startupStage(cfg, rt, sameLoad)
	var progress map[string]any
	if sameLoad || !cfg.Bool("managed_lifecycle") {
		progress, _ = rt["progress"].(map[string]any)
	}
	var indexReason any
	if b := cfg.PID("building_profile_id"); b != nil {
		var status string
		var result map[string]any
		err := a.pool.QueryRow(ctx, `SELECT status, result FROM ai_job WHERE kind='index' AND profile_id=$1 ORDER BY job_id LIMIT 1`, *b).Scan(&status, &result)
		if err == nil {
			indexReason = result["reason"]
			if cfg.Bool("enabled") && status == "failed" {
				stage = "failed"
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	pv := func(k string) any {
		if progress == nil {
			return nil
		}
		return progress[k]
	}
	reason := indexReason
	if reason == nil && (sameLoad || !cfg.Bool("managed_lifecycle")) {
		reason = rt["reason"]
	}
	startup := map[string]any{"stage": stage, "lifecycleId": cfg.Int("lifecycle_id"), "completed": pv("completed"), "total": pv("total"),
		"unit": pv("unit"), "updatedAt": pv("updatedAt"), "reason": reason, "online": services["runtime"].(map[string]any)["online"]}
	u, err := usage(ctx, a.pool)
	if err != nil {
		return nil, err
	}
	bg, err := backgroundStatus(ctx, a.pool, cfg)
	if err != nil {
		return nil, err
	}
	jobs := map[string]int64{}
	rows, err := a.pool.Query(ctx, `SELECT status, count(*) FROM ai_job GROUP BY status`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var s string
		var n int64
		if err := rows.Scan(&s, &n); err != nil {
			rows.Close()
			return nil, err
		}
		jobs[s] = n
	}
	rows.Close()
	cov, err := coverage(ctx, a.pool, cfg.PID("search_profile_id"), nil)
	if err != nil {
		return nil, err
	}
	var bcov any
	if b := cfg.PID("building_profile_id"); b != nil {
		if bcov, err = coverage(ctx, a.pool, b, nil); err != nil {
			return nil, err
		}
	}
	sources, err := a.indexSources(ctx, cfg)
	if err != nil {
		return nil, err
	}
	smart, err := a.smart.AdminStatus(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"startup": startup, "settings": cfgs, "services": services, "quota": u.toMap(), "backgroundLimits": bg, "jobs": jobs,
		"coverage": cov, "buildingCoverage": bcov, "smartTag": smart, "worker": a.rt.WorkerStatus(), "indexSources": sources}, nil
}

// ---- per-model load / unload ----

func (a *Admin) LoadModel(kind string, force bool) (map[string]any, error) {
	if kind != "image" && kind != "text" {
		return nil, fail("invalid_model", 404)
	}
	info, err := a.rt.LoadModel(kind, force)
	if err != nil {
		return nil, err
	}
	return map[string]any{"kind": kind, "loaded": info.Loaded, "variant": info.Variant, "loadMs": info.LoadMs, "worker": a.rt.WorkerStatus()}, nil
}

func (a *Admin) UnloadModel(kind string) (map[string]any, error) {
	if kind != "image" && kind != "text" {
		return nil, fail("invalid_model", 404)
	}
	before, after, err := a.rt.UnloadModel(kind)
	if err != nil {
		return nil, err
	}
	return map[string]any{"kind": kind, "beforeMB": before, "afterMB": after, "worker": a.rt.WorkerStatus()}, nil
}

// Enabled is the desired setting, not proof that model loading completed.
func startupStage(cfg Settings, rt map[string]any, sameLoad bool) string {
	state, _ := rt["state"].(string)
	if cfg.Bool("managed_lifecycle") && !sameLoad {
		if cfg.Bool("enabled") {
			return "preparing"
		}
		return "pausing"
	}
	if !cfg.Bool("enabled") {
		if state == "unloaded" || !cfg.Bool("managed_lifecycle") {
			return "paused"
		}
		if state == "failed" {
			return "failed"
		}
		return "pausing"
	}
	if state == "unloaded" || state == "paused" {
		if cfg.Str("runtime") == "loaded" {
			return "preparing"
		}
		return "idle"
	}
	if state == "ready" && cfg.PID("building_profile_id") != nil && (cfg.Bool("maintenance") || cfg.PID("search_profile_id") == nil) {
		return "preparing_index"
	}
	return state
}
