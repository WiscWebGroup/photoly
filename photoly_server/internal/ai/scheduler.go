package ai

// The embedding scheduler: one leader loop over the ai_job table (scheduler/photo_embedding_scheduler.py),
// the maintenance loop (scheduler/maintenance.py) and the idle spot check of browser-made vectors.

import (
	"context"
	"errors"
	"math"
	"os"
	"sync/atomic"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const browserGrace = 5 * time.Minute

type Scheduler struct {
	pool       *pgxpool.Pool
	rt         *Runtime
	root       string
	owner      string
	token      string
	policy     Policy
	lastUser   *int64
	userCursor int64
	smart      *SmartSched
}

func runtimeConfig(cfg Settings) map[string]any {
	return map[string]any{"device": cfg.Str("device"), "precision": cfg.Str("precision"), "cpu_threads": cfg.Int("cpu_threads")}
}

// wantKinds are the models this server loads: browser-only placements need none.
func wantKinds(cfg Settings) []string {
	var k []string
	if cfg.Str("placement_image") != "browser" {
		k = append(k, "image")
	}
	if cfg.Str("placement_text") != "browser" {
		k = append(k, "text")
	}
	return k
}

func (s *Scheduler) lead(ctx context.Context, tx pgx.Tx) (bool, error) {
	row, err := control(ctx, tx, "scheduler", true)
	if err != nil {
		return false, err
	}
	now := time.Now()
	if row.LeaseUntil != nil && row.LeaseUntil.After(now) && (row.LeaseOwner == nil || *row.LeaseOwner != s.owner) {
		return false, nil
	}
	if row.LeaseOwner == nil || *row.LeaseOwner != s.owner || row.LeaseUntil == nil || !row.LeaseUntil.After(now) {
		s.token = newUUID()
	}
	_, err = tx.Exec(ctx, `UPDATE ai_control_state SET lease_owner=$1, lease_token=$2, lease_expires_at=now()+interval '120 seconds', heartbeat_at=now() WHERE name='scheduler'`, s.owner, s.token)
	return err == nil, err
}

func (s *Scheduler) reconcile(ctx context.Context, tx pgx.Tx) error {
	cfg, err := loadSettings(ctx, tx)
	if err != nil {
		return err
	}
	if err := recoverJobs(ctx, tx); err != nil {
		return err
	}
	if !cfg.Bool("enabled") {
		return nil
	}
	rows, err := tx.Query(ctx, `SELECT user_id FROM "user" u WHERE ai_search_enabled AND user_id>$1 AND EXISTS (SELECT 1 FROM photo p WHERE p.user_id=u.user_id)
		ORDER BY user_id LIMIT 4`, s.userCursor)
	if err != nil {
		return err
	}
	var users []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		users = append(users, id)
	}
	rows.Close()
	s.userCursor = 0
	if len(users) > 0 {
		s.userCursor = users[len(users)-1]
	}
	seen := map[int64]bool{}
	for _, k := range []string{"search_profile_id", "building_profile_id"} {
		pid := cfg.PID(k)
		if pid == nil || seen[*pid] {
			continue
		}
		seen[*pid] = true
		for _, uid := range users {
			prow, err := tx.Query(ctx, `SELECT p.photo_id FROM photo p LEFT JOIN photo_embedding_state s ON s.photo_id=p.photo_id AND s.profile_id=$2
				WHERE p.user_id=$1 AND (s.state_id IS NULL OR s.content_version<>p.content_version
					OR (lower(p.format)=ANY($3) AND (s.sampling_version IS NULL OR s.sampling_version<>$4))
					OR (s.status='pending' AND NOT EXISTS (SELECT 1 FROM ai_job j WHERE j.photo_id=p.photo_id AND j.profile_id=$2
						AND j.content_version=p.content_version AND j.status IN ('queued','running'))))
				ORDER BY p.photo_id LIMIT 25`, uid, *pid, videoList(), samplingVersion)
			if err != nil {
				return err
			}
			var ids []int64
			for prow.Next() {
				var id int64
				if err := prow.Scan(&id); err != nil {
					prow.Close()
					return err
				}
				ids = append(ids, id)
			}
			prow.Close()
			for _, id := range ids {
				if err := EnqueuePhoto(ctx, tx, id, *pid); err != nil {
					return err
				}
			}
		}
	}
	_, err = tx.Exec(ctx, `DELETE FROM ai_job WHERE job_id IN (SELECT job_id FROM ai_job WHERE kind<>'operation' AND status IN ('succeeded','cancelled')
		AND finished_at < now()-interval '7 days' ORDER BY job_id LIMIT 1000)`)
	return err
}

func videoList() []string {
	var l []string
	for k := range videoExt {
		l = append(l, k)
	}
	return l
}

func (s *Scheduler) publish(ctx context.Context, observed map[string]any, reason any) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	row, err := control(ctx, tx, "scheduler", true)
	if err != nil || row.LeaseOwner == nil || *row.LeaseOwner != s.owner || row.LeaseToken == nil || *row.LeaseToken != s.token {
		return
	}
	cfg, err := loadSettings(ctx, tx)
	if err != nil {
		return
	}
	_ = saveObserved(ctx, tx, "scheduler", map[string]any{"state": cfg.Str("scheduler"), "reason": reason})
	_ = saveObserved(ctx, tx, "runtime", observed)
	_ = tx.Commit(ctx)
}

// heartbeat keeps the leadership (and the task lease) alive and publishes runtime progress.
func (s *Scheduler) heartbeat(ctx context.Context, t *Task) (stop func(), lost *atomic.Bool) {
	lost = &atomic.Bool{}
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for {
			select {
			case <-done:
				return
			case <-time.After(2 * time.Second):
			}
			ok := func() bool {
				tx, err := s.pool.Begin(ctx)
				if err != nil {
					return false
				}
				defer tx.Rollback(ctx)
				if ok, err := s.lead(ctx, tx); err != nil || !ok {
					return false
				}
				if t != nil {
					if ok, err := renew(ctx, tx, t); err != nil || !ok {
						return false
					}
				}
				return tx.Commit(ctx) == nil
			}()
			if !ok {
				lost.Store(true)
				return
			}
			observed := s.rt.Status()
			s.publish(ctx, observed, nil)
			if t != nil {
				_, _ = s.pool.Exec(ctx, `UPDATE ai_job SET result=result || jsonb_build_object('step',$2::text) WHERE job_id=$1 AND kind='operation' AND status='running'`, t.ID, observed["state"])
			}
		}
	}()
	return func() { close(done); <-finished }, lost
}

func (s *Scheduler) operate(ctx context.Context, t *Task) error {
	var cfg Settings
	var spec map[string]any
	command, _ := t.Payload["command"].(string)
	gen, _ := toInt(t.Payload["generation"])
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	row, err := control(ctx, tx, "global", true)
	if err == nil {
		cfg, err = loadSettings(ctx, tx)
	}
	if err != nil {
		tx.Rollback(ctx)
		return err
	}
	expected := row.Generation
	if len(command) > 3 && command[:3] == "ai." {
		expected = cfg.Int("lifecycle_id")
	}
	if gen != expected {
		_, err = finish(ctx, tx, t, "cancelled", map[string]any{"reason": "superseded"})
		if err == nil {
			err = tx.Commit(ctx)
		}
		return err
	}
	if t.ProfileID != nil {
		if p, perr := getProfile(ctx, tx, *t.ProfileID); perr == nil {
			spec = p.Spec
		}
	}
	_ = spec
	tx.Commit(ctx)

	switch {
	case len(command) > 6 && command[:6] == "model.":
		stop, lost := s.heartbeat(ctx, t)
		verb := command[6:]
		var kinds []string
		if verb != "unload" {
			kinds = wantKinds(cfg)
		}
		_, err := s.rt.Change(verb, kinds, runtimeConfig(cfg), gen, itoa(int(t.ID)))
		stop()
		if lost.Load() {
			return nil
		}
		if err != nil {
			return err
		}
		s.publish(ctx, s.rt.Status(), nil)
	case command == "backfill" || command == "retry_failed" || command == "rebuild":
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		job, err := currentJob(ctx, tx, t)
		if err != nil || job == nil {
			return err
		}
		cursor, _ := toInt(job.Payload["cursor"])
		sql := `SELECT p.photo_id FROM photo p JOIN "user" u ON u.user_id=p.user_id `
		args := []any{cursor}
		if command == "retry_failed" {
			sql += `JOIN photo_embedding_state s ON s.photo_id=p.photo_id AND s.profile_id=$2 AND s.status='failed' `
			args = append(args, *t.ProfileID)
		}
		rows, err := tx.Query(ctx, sql+`WHERE u.ai_search_enabled AND p.photo_id>$1 ORDER BY p.photo_id LIMIT 100`, args...)
		if err != nil {
			return err
		}
		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
		for _, id := range ids {
			if command == "backfill" {
				var ready bool
				if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM photo_embedding_state s JOIN photo p ON p.photo_id=s.photo_id WHERE s.photo_id=$1 AND s.profile_id=$2
					AND s.content_version=p.content_version AND s.status='ready')`, id, *t.ProfileID).Scan(&ready); err != nil {
					return err
				}
				if ready {
					continue
				}
			}
			if err := EnqueuePhoto(ctx, tx, id, *t.ProfileID); err != nil {
				return err
			}
		}
		if len(ids) > 0 {
			job.Payload["cursor"] = ids[len(ids)-1]
			if _, err := tx.Exec(ctx, `UPDATE ai_job SET payload=$2, status='queued', reserved_bytes=0 WHERE job_id=$1`, t.ID, job.Payload); err != nil {
				return err
			}
			return tx.Commit(ctx)
		}
		if _, err := finish(ctx, tx, t, "", nil); err != nil {
			return err
		}
		return tx.Commit(ctx)
	case command == "activate":
		observed := s.rt.Status()
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		row, err := control(ctx, tx, "global", true)
		if err != nil {
			return err
		}
		p, err := getProfile(ctx, tx, *t.ProfileID)
		if err != nil {
			return err
		}
		counts, err := coverage(ctx, tx, t.ProfileID, nil)
		if err != nil {
			return err
		}
		pending, _ := toInt(counts["pending"])
		failed, _ := toInt(counts["failed"])
		if row.Generation != gen || p.IndexStatus != "ready" || observed["state"] != "ready" || !onnxCompatible(p.Spec) ||
			observed["fingerprint"] != ONNXProfile().Fingerprint || pending > 0 || failed > 0 {
			_, err = finish(ctx, tx, t, "failed", map[string]any{"reason": "activation_not_ready", "coverage": counts})
		} else {
			cur, lerr := loadSettings(ctx, tx)
			if lerr != nil {
				return lerr
			}
			cur["search_profile_id"], cur["building_profile_id"], cur["maintenance"] = p.ID, nil, false
			row.Desired = cur
			if err = saveDesired(ctx, tx, row); err == nil {
				_, err = finish(ctx, tx, t, "", map[string]any{"coverage": counts})
			}
		}
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	case command == "purge":
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		job, err := currentJob(ctx, tx, t)
		if err != nil || job == nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO ai_job (kind, profile_id, dedup_key, payload) VALUES ('purge',$1,$2,$3) ON CONFLICT DO NOTHING`,
			*t.ProfileID, "purge:"+itoa(int(*t.ProfileID)), map[string]any{"operation_id": t.ID, "generation": gen}); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE ai_job SET result='{"step":"waiting_for_maintenance"}', lease_expires_at=now()+interval '15 minutes' WHERE job_id=$1`, t.ID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	_, err = finish(ctx, s.pool, t, "", nil)
	return err
}

var unsupportedCodes = map[string]bool{"unsupported_image": true, "invalid_image": true, "image_too_large": true, "image_too_many_pixels": true,
	"invalid_video": true, "video_stream_missing": true, "video_frame_missing": true, "video_frame_too_large": true, "video_too_large": true}

func (s *Scheduler) embed(ctx context.Context, t *Task) error {
	stop, lost := s.heartbeat(ctx, t)
	defer stop()
	snap, err := snapshotPhoto(ctx, s.pool, s.root, *t.PhotoID, nil, t.Version)
	if err != nil {
		return err
	}
	p, err := getProfile(ctx, s.pool, *t.ProfileID)
	if err != nil {
		return err
	}
	if !onnxCompatible(p.Spec) {
		return fail("profile_mismatch", 409)
	}
	uid := snap.UserID
	s.lastUser = &uid
	start := time.Now()
	datas := [][]byte{snap.Data}
	if snap.Frames != nil {
		datas = nil
		for _, f := range snap.Frames {
			datas = append(datas, f.Data)
		}
	}
	fp := ONNXProfile().Fingerprint
	var vectors [][]float32
	for _, d := range datas {
		if lost.Load() {
			return nil
		}
		if j, err := currentJob(ctx, s.pool, t); err != nil || j == nil {
			return err
		}
		pixels, err := prepareImage(d)
		if err != nil {
			return mapImageErr(err)
		}
		vec, err := s.rt.Encode(&encItem{kind: "image", pixels: pixels, fingerprint: fp}, true, 5*time.Minute)
		if err != nil {
			return err
		}
		vectors = append(vectors, vec)
	}
	if lost.Load() {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	row, err := control(ctx, tx, "scheduler", true)
	if err != nil {
		return err
	}
	if row.LeaseToken == nil || *row.LeaseToken != s.token || row.LeaseUntil == nil || !row.LeaseUntil.After(time.Now()) {
		return nil
	}
	if _, err := commitEmbedding(ctx, tx, t, snap, vectors, int(time.Since(start).Milliseconds())); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Scheduler) managedRuntime(ctx context.Context, cfg Settings) (map[string]any, error) {
	observed := s.rt.Status()
	var pid *int64
	if cfg.Bool("enabled") {
		pid = cfg.PID("loaded_profile_id")
	}
	gen := cfg.Int("lifecycle_id")
	if g, _ := toInt(observed["generation"]); g != gen {
		cmd, kinds := "unload", []string(nil)
		if pid != nil {
			cmd, kinds = "load", wantKinds(cfg)
		}
		var err error
		if observed, err = s.rt.Submit(cmd, kinds, runtimeConfig(cfg), gen, "ai:"+itoa(int(gen))); err != nil {
			return nil, err
		}
	}
	expected := "unloaded"
	if pid != nil {
		expected = "ready"
	}
	if g, _ := toInt(observed["generation"]); g != gen || observed["state"] != expected || pid == nil {
		return observed, nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	row, err := control(ctx, tx, "global", true)
	if err != nil {
		return nil, err
	}
	cur, err := loadSettings(ctx, tx)
	if err != nil {
		return nil, err
	}
	p, err := getProfile(ctx, tx, *pid)
	if err != nil {
		return observed, nil
	}
	sp := cur.PID("search_profile_id")
	switches := sp == nil || *sp != *pid
	if cur.Bool("enabled") && cur.Bool("managed_lifecycle") && cur.Int("lifecycle_id") == gen && cur.isPID("loaded_profile_id", *pid) &&
		onnxCompatible(p.Spec) && observed["fingerprint"] == ONNXProfile().Fingerprint && p.IndexStatus == "ready" &&
		(switches || cur["building_profile_id"] != nil || cur.Bool("maintenance")) {
		ok := true
		if switches && sp != nil && !cur.Bool("maintenance") {
			// An index built beside a serving one replaces it only once it covers the library.
			c, err := coverage(ctx, tx, pid, nil)
			if err != nil {
				return nil, err
			}
			pending, _ := toInt(c["pending"])
			failed, _ := toInt(c["failed"])
			ok = pending == 0 && failed == 0
		}
		if ok {
			cur["search_profile_id"], cur["building_profile_id"], cur["maintenance"] = *pid, nil, false
			row.Desired = cur
			row.Generation++
			if err := saveDesired(ctx, tx, row); err != nil {
				return nil, err
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
		}
	}
	return observed, nil
}

func (s *Scheduler) tick(ctx context.Context) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if ok, err := s.lead(ctx, tx); err != nil || !ok {
		return err
	}
	if err := s.reconcile(ctx, tx); err != nil {
		return err
	}
	cfg, err := loadSettings(ctx, tx)
	if err != nil {
		return err
	}
	task, err := claim(ctx, tx, s.owner, ClaimOpts{Kinds: []string{"operation"}})
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if task != nil {
		if err := s.operate(ctx, task); err != nil {
			code := asErr(err).Code
			status := "failed"
			if code == "superseded" || code == "stale_generation" {
				status = "cancelled"
			}
			_, _ = finish(ctx, s.pool, task, status, map[string]any{"reason": code})
		}
		return nil
	}
	if _, err := s.smart.Tick(ctx); err != nil {
		logf("Smart Tag batch failed: %v", err)
	}
	var observed map[string]any
	if cfg.Bool("managed_lifecycle") {
		observed, err = s.managedRuntime(ctx, cfg)
	} else {
		observed = s.rt.Status()
		pid := cfg.PID("loaded_profile_id")
		want := "unloaded"
		if cfg.Str("runtime") == "loaded" {
			want = "ready"
		}
		if observed["state"] != want || (pid != nil && observed["fingerprint"] != ONNXProfile().Fingerprint) {
			stop, lost := s.heartbeat(ctx, nil)
			gen := cfg.Int("lifecycle_id")
			var row *ControlRow
			if row, err = control(ctx, s.pool, "global", false); err == nil {
				gen = row.Generation
				cmd, kinds := "unload", []string(nil)
				if pid != nil {
					cmd, kinds = "load", wantKinds(cfg)
				}
				observed, err = s.rt.Change(cmd, kinds, runtimeConfig(cfg), gen, "reconcile:"+itoa(int(gen)))
			}
			stop()
			if lost.Load() {
				return nil
			}
		}
	}
	if err != nil {
		s.publish(ctx, map[string]any{"state": "offline"}, "runtime_offline")
		return nil
	}
	res, _ := observed["resources"].(map[string]any)
	d := s.policy.Decide(res, cfg)
	var reason any
	if !d.Allowed {
		reason = d.Reasons
	}
	s.publish(ctx, observed, reason)
	if !cfg.Bool("enabled") || cfg.Str("scheduler") != "running" || observed["state"] != "ready" || !d.Allowed {
		return nil
	}
	tx, err = s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	task, err = claim(ctx, tx, s.owner, ClaimOpts{Kinds: []string{"embedding"}, ProfileID: cfg.PID("loaded_profile_id"), LastUser: s.lastUser,
		AdminImage: cfg.Str("placement_image"), Grace: browserGrace})
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if task == nil {
		return s.spotCheck(ctx, cfg)
	}
	if err := s.embed(ctx, task); err != nil {
		code := asErr(err).Code
		tx, e2 := s.pool.Begin(ctx)
		if e2 != nil {
			return e2
		}
		defer tx.Rollback(ctx)
		switch {
		case unsupportedCodes[code]:
			if j, _ := currentJob(ctx, tx, task); j != nil {
				_, _ = tx.Exec(ctx, `WITH s AS (UPDATE photo_embedding_state SET status='unsupported', embedding=NULL, error_code=$4, frame_count=0
					WHERE photo_id=$1 AND profile_id=$2 AND content_version=$3 RETURNING state_id) DELETE FROM video_frame_embedding WHERE state_id IN (SELECT state_id FROM s)`,
					task.PhotoID, task.ProfileID, task.Version, code)
				_, _ = finish(ctx, tx, task, "", map[string]any{"reason": code})
			}
		case code == "stale_content" || code == "photo_not_found":
			_, _ = finish(ctx, tx, task, "cancelled", map[string]any{"reason": code})
		default:
			st := asErr(err).Status
			err = failJob(ctx, tx, task, code, (st == 429 || st == 503 || st == 504) && code != "video_decode_timeout")
			if err != nil {
				return err
			}
		}
		return tx.Commit(ctx)
	}
	return nil
}

// spotCheck re-embeds one sampled browser-made vector on an idle server. A mismatch (cosine < 0.98)
// drops that user's browser vectors for the profile and queues them for the server.
func (s *Scheduler) spotCheck(ctx context.Context, cfg Settings) error {
	pid := cfg.PID("loaded_profile_id")
	if pid == nil || cfg.Str("placement_image") == "browser" {
		return nil
	}
	var photoID, uid int64
	var version int32
	err := s.pool.QueryRow(ctx, `SELECT s.photo_id, s.content_version, p.user_id FROM photo_embedding_state s JOIN photo p ON p.photo_id=s.photo_id
		WHERE s.profile_id=$1 AND s.spot_check AND s.spot_checked_at IS NULL AND s.status='ready' AND s.source='client' AND s.embedding IS NOT NULL
			AND s.content_version=p.content_version LIMIT 1`, *pid).Scan(&photoID, &version, &uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	mark := func(match bool) error {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, `UPDATE photo_embedding_state SET spot_checked_at=now() WHERE photo_id=$1 AND profile_id=$2`, photoID, *pid); err != nil {
			return err
		}
		row, err := control(ctx, tx, "spot_check", true)
		if err != nil {
			return err
		}
		checked, _ := toInt(row.Observed["checked"])
		matched, _ := toInt(row.Observed["matched"])
		checked++
		if match {
			matched++
		}
		if err := saveObserved(ctx, tx, "spot_check", map[string]any{"checked": checked, "matched": matched, "at": time.Now().UTC().Format(time.RFC3339)}); err != nil {
			return err
		}
		if !match {
			rows, err := tx.Query(ctx, `UPDATE photo_embedding_state s SET status='pending' FROM photo p WHERE p.photo_id=s.photo_id AND p.user_id=$1 AND s.profile_id=$2 AND s.source='client' RETURNING s.photo_id`, uid, *pid)
			if err != nil {
				return err
			}
			var ids []int64
			for rows.Next() {
				var id int64
				if err := rows.Scan(&id); err != nil {
					rows.Close()
					return err
				}
				ids = append(ids, id)
			}
			rows.Close()
			for _, id := range ids {
				if err := EnqueuePhoto(ctx, tx, id, *pid); err != nil {
					return err
				}
			}
		}
		return tx.Commit(ctx)
	}
	stored, err := storedVectors(ctx, s.pool, photoID, *pid, version)
	if err != nil || len(stored) != 1 {
		return err
	}
	snap, err := snapshotPhoto(ctx, s.pool, s.root, photoID, nil, &version)
	if err != nil || snap.Data == nil {
		return mark(true) // not checkable (video or gone): do not retry forever
	}
	pixels, err := prepareImage(snap.Data)
	if err != nil {
		return mark(true)
	}
	vec, err := s.rt.Encode(&encItem{kind: "image", pixels: pixels, fingerprint: ONNXProfile().Fingerprint}, true, 2*time.Minute)
	if err != nil {
		return nil
	}
	var dot, na, nb float64
	for i := range vec {
		a, b := float64(vec[i]), float64(stored[0][i])
		dot, na, nb = dot+a*b, na+a*a, nb+b*b
	}
	return mark(dot/math.Sqrt(na*nb) >= 0.98)
}

// ---- maintenance: HNSW index builds and profile purge ----

func (s *Scheduler) maintenanceTick(ctx context.Context) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := saveObserved(ctx, tx, "maintenance", map[string]any{"state": "running"}); err != nil {
		if _, e := control(ctx, tx, "maintenance", false); e != nil {
			return e
		}
	}
	task, err := claim(ctx, tx, s.owner+"-maint", ClaimOpts{Kinds: []string{"index", "purge"}})
	if err != nil || task == nil {
		if err == nil {
			err = tx.Commit(ctx)
		}
		return err
	}
	p, err := getProfile(ctx, tx, *task.ProfileID)
	if err != nil {
		_, _ = finish(ctx, tx, task, "cancelled", nil)
		return tx.Commit(ctx)
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	pid, dim := p.ID, p.Dimension
	if dim != 768 && dim != 2048 {
		return errors.New("invalid registered dimension")
	}
	name, frame := "ix_embedding_profile_"+itoa(int(pid))+"_hnsw", "ix_video_frame_profile_"+itoa(int(pid))+"_hnsw"
	hbCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-time.After(20 * time.Second):
				if ok, err := renew(hbCtx, s.pool, task); err != nil || !ok {
					return
				}
			}
		}
	}()
	fail := func(code string, transient bool) error {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if err := failJob(ctx, tx, task, code, transient); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	var got bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(74832002)`).Scan(&got); err != nil || !got {
		return fail("maintenance_busy", true)
	}
	defer conn.Exec(context.Background(), `SELECT pg_advisory_unlock(74832002)`)
	defer conn.Exec(context.Background(), `RESET statement_timeout`)
	if _, err := conn.Exec(ctx, `SET statement_timeout='90s'`); err != nil {
		return err
	}
	if task.Kind == "index" {
		c, err := loadSettings(ctx, conn)
		if err != nil {
			return err
		}
		u, err := usage(ctx, conn)
		if err != nil {
			return err
		}
		if max(u.Physical, u.Estimated+u.Reserved) >= c.Int("quota_bytes") {
			return fail("quota_exceeded", true)
		}
		ddl, closeDDL := ddlConn(ctx, conn)
		defer closeDDL()
		for _, ix := range [][3]string{{name, "photo_embedding_state", "profile_id=" + itoa(int(pid)) + " AND status='ready'"}, {frame, "video_frame_embedding", "profile_id=" + itoa(int(pid))}} {
			var valid *bool
			err := conn.QueryRow(ctx, `SELECT indisvalid FROM pg_index JOIN pg_class ON indexrelid=oid WHERE relname=$1`, ix[0]).Scan(&valid)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return fail("index_maintenance_failed", false)
			}
			if valid != nil && !*valid {
				if _, err := ddl.Exec(ctx, `DROP INDEX CONCURRENTLY IF EXISTS "`+ix[0]+`"`); err != nil {
					return fail("index_maintenance_failed", false)
				}
			}
			if _, err := ddl.Exec(ctx, `CREATE INDEX CONCURRENTLY IF NOT EXISTS "`+ix[0]+`" ON `+ix[1]+` USING hnsw ((embedding::halfvec(`+itoa(dim)+`)) halfvec_cosine_ops) WHERE `+ix[2]); err != nil {
				logf("index build %s: %v", ix[0], err)
				return fail("index_maintenance_failed", false)
			}
		}
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, `UPDATE embedding_profile SET index_status='ready' WHERE profile_id=$1`, pid); err != nil {
			return err
		}
		if _, err := finish(ctx, tx, task, "", nil); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	// purge
	tx, err = s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := control(ctx, tx, "global", true); err != nil {
		return err
	}
	c, err := loadSettings(ctx, tx)
	if err != nil {
		return err
	}
	var active bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM ai_job WHERE kind='search' AND profile_id=$1 AND status='running' AND lease_expires_at>now())`, pid).Scan(&active); err != nil {
		return err
	}
	opID, _ := toInt(task.Payload["operation_id"])
	finishOp := func(status string, result map[string]any) error {
		_, err := tx.Exec(ctx, `UPDATE ai_job SET status=$2, result=$3, finished_at=now() WHERE job_id=$1 AND status='running'`, opID, status, result)
		return err
	}
	if c.isPID("search_profile_id", pid) || c.isPID("building_profile_id", pid) || c.isPID("loaded_profile_id", pid) || active {
		if _, err := finish(ctx, tx, task, "failed", map[string]any{"reason": "profile_in_use"}); err != nil {
			return err
		}
		if err := finishOp("failed", map[string]any{"reason": "profile_in_use"}); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	// DROP INDEX CONCURRENTLY cannot run inside the transaction, so the global lock is held while it runs on the other connection.
	for _, n := range []string{name, frame} {
		if _, err := conn.Exec(ctx, `DROP INDEX CONCURRENTLY IF EXISTS "`+n+`"`); err != nil {
			return fail("index_maintenance_failed", false)
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM embedding_profile WHERE profile_id=$1`, pid); err != nil {
		return err
	}
	if _, err := finish(ctx, tx, task, "", nil); err != nil {
		return err
	}
	if err := finishOp("succeeded", map[string]any{}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Start runs the scheduler and maintenance loops until ctx ends.
func (s *Scheduler) Start(ctx context.Context) {
	loop := func(name string, every time.Duration, fn func(context.Context) error) {
		go func() {
			for ctx.Err() == nil {
				if err := fn(ctx); err != nil && ctx.Err() == nil {
					logf("ai %s cycle failed: %v", name, err)
				}
				select {
				case <-ctx.Done():
				case <-time.After(every):
				}
			}
		}()
	}
	loop("scheduler", 2*time.Second, s.tick)
	loop("maintenance", 3*time.Second, s.maintenanceTick)
}

// ddlExecer is the DDL connection: the migrator role owns the tables, the app role cannot index them.
type ddlExecer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// ddlConn connects as the migrator when its credentials are configured (as Flask's ai-maintenance did),
// else falls back to the pooled connection.
func ddlConn(ctx context.Context, fallback *pgxpool.Conn) (ddlExecer, func()) {
	if os.Getenv("MIGRATION_DATABASE_URL") == "" && os.Getenv("POSTGRES_MIGRATION_PASSWORD") == "" {
		return fallback, func() {}
	}
	c, err := pgx.Connect(ctx, config.DatabaseURL(true))
	if err != nil {
		logf("migrator connection failed, indexing as the app role: %v", err)
		return fallback, func() {}
	}
	return c, func() { c.Close(ctx) }
}
