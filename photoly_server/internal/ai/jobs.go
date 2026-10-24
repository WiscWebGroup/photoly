package ai

// The ai_job table as a queue (scheduler/job_repository.py, background_limits.py). Leases,
// dedup keys and statuses are unchanged so Flask and Go can share a database.

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
)

const leaseSeconds = 120

// Task is a claimed job.
type Task struct {
	ID        int64
	Token     string
	PhotoID   *int64
	ProfileID *int64
	Version   *int32
	Kind      string
	Payload   map[string]any
}

func vectorCharge(dimension int) int64 { return int64(2*(2*dimension+8) + 1024) }

type UsageInfo struct{ Estimated, Reserved, Physical int64 }

func (u UsageInfo) toMap() map[string]any {
	return map[string]any{"estimated_bytes": u.Estimated, "reserved_bytes": u.Reserved, "physical_bytes": u.Physical}
}

func usage(ctx context.Context, q Q) (u UsageInfo, err error) {
	err = q.QueryRow(ctx, `SELECT COALESCE(SUM((2*(2*p.dimension+8)+1024) * CASE WHEN s.sampling_version IS NOT NULL THEN s.frame_count ELSE 1 END),0)::bigint
		FROM photo_embedding_state s JOIN embedding_profile p ON p.profile_id=s.profile_id WHERE s.status='ready'`).Scan(&u.Estimated)
	if err != nil {
		return
	}
	if err = q.QueryRow(ctx, `SELECT COALESCE(SUM(reserved_bytes),0)::bigint FROM ai_job WHERE status='running'`).Scan(&u.Reserved); err != nil {
		return
	}
	err = q.QueryRow(ctx, `SELECT (pg_total_relation_size('photo_embedding_state') + pg_total_relation_size('embedding_profile') + pg_total_relation_size('video_frame_embedding'))::bigint`).Scan(&u.Physical)
	return
}

func recoverJobs(ctx context.Context, q Q) error {
	if _, err := q.Exec(ctx, `UPDATE ai_job SET status='failed', finished_at=now(), result='{"reason":"query_timeout"}'
		WHERE kind='search' AND status='running' AND lease_expires_at < now()`); err != nil {
		return err
	}
	_, err := q.Exec(ctx, `UPDATE ai_job SET status='queued', reserved_bytes=0, lease_token=NULL, lease_owner=NULL, result='{"reason":"lease_expired"}'
		WHERE kind<>'search' AND status='running' AND lease_expires_at < now()`)
	return err
}

// ClaimOpts narrows a claim.
type ClaimOpts struct {
	Kinds     []string
	ProfileID *int64
	LastUser  *int64
	// Embedding placement: skip users whose images are meant for the browser (admin
	// "browser" placement never queues on the server; "user" waits a grace period).
	AdminImage string
	Grace      time.Duration
}

func claim(ctx context.Context, tx pgx.Tx, owner string, o ClaimOpts) (*Task, error) {
	row, err := control(ctx, tx, "global", true)
	if err != nil {
		return nil, err
	}
	if err = recoverJobs(ctx, tx); err != nil {
		return nil, err
	}
	embedding := len(o.Kinds) == 1 && o.Kinds[0] == "embedding"
	smart := len(o.Kinds) == 1 && o.Kinds[0] == "smart"
	sql := `SELECT j.job_id, j.photo_id, j.profile_id, j.content_version, j.kind, j.payload FROM ai_job j `
	var args = []any{o.Kinds}
	where := ` WHERE j.kind = ANY($1) AND j.status='queued' AND j.next_run_at <= now()`
	order := ``
	if embedding {
		sql += `JOIN photo ph ON ph.photo_id=j.photo_id JOIN "user" u ON u.user_id=ph.user_id LEFT JOIN user_ai_preference up ON up.user_id=u.user_id `
		where += ` AND u.ai_search_enabled AND ph.content_version=j.content_version`
		args = append(args, o.AdminImage, int(o.Grace.Seconds()))
		where += ` AND $2::text <> 'browser' AND ($2::text <> 'user' OR COALESCE(up.image,'server') <> 'browser' OR j.created_at < now() - make_interval(secs => $3::float8))`
		if o.LastUser != nil {
			args = append(args, *o.LastUser)
			order = `(ph.user_id = $` + itoa(len(args)) + `) ASC, `
		}
	}
	if o.ProfileID != nil {
		args = append(args, *o.ProfileID)
		where += ` AND j.profile_id = $` + itoa(len(args))
	}
	if smart && o.LastUser != nil {
		args = append(args, *o.LastUser)
		order = `((j.payload->>'user_id')::int <= $` + itoa(len(args)) + `) ASC, (j.payload->>'user_id')::int, `
	}
	sql += where + ` ORDER BY ` + order + `j.priority DESC, j.job_id FOR UPDATE OF j SKIP LOCKED LIMIT 1`
	var t Task
	var payload []byte
	err = tx.QueryRow(ctx, sql, args...).Scan(&t.ID, &t.PhotoID, &t.ProfileID, &t.Version, &t.Kind, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(payload, &t.Payload)
	var reserve int64
	if t.Kind == "embedding" {
		cfg := row.Desired
		full, err := loadSettings(ctx, tx)
		if err != nil {
			return nil, err
		}
		for k, v := range cfg {
			full[k] = v
		}
		if !full.Bool("enabled") || full.Str("scheduler") != "running" {
			return nil, nil
		}
		var dim int
		var format *string
		if err = tx.QueryRow(ctx, `SELECT p.dimension, ph.format FROM embedding_profile p, photo ph WHERE p.profile_id=$1 AND ph.photo_id=$2`, t.ProfileID, t.PhotoID).Scan(&dim, &format); err != nil {
			return nil, err
		}
		reserve = vectorCharge(dim)
		if format != nil && isVideo(*format) {
			reserve *= maxFrames
		}
		cur, err := usage(ctx, tx)
		if err != nil {
			return nil, err
		}
		if max(cur.Estimated+cur.Reserved+reserve, cur.Physical) > full.Int("quota_bytes") {
			_, err = tx.Exec(ctx, `UPDATE ai_job SET result='{"reason":"quota_exceeded"}', next_run_at=now()+interval '10 seconds' WHERE job_id=$1`, t.ID)
			return nil, err
		}
		if ok, err := admit(ctx, tx, full); err != nil || !ok {
			return nil, err
		}
	}
	t.Token = newUUID()
	_, err = tx.Exec(ctx, `UPDATE ai_job SET status='running', lease_owner=$2, lease_token=$3, lease_expires_at=now()+make_interval(secs=>$4::float8),
		heartbeat_at=now(), reserved_bytes=$5 WHERE job_id=$1`, t.ID, owner, t.Token, leaseSeconds, reserve)
	return &t, err
}

func itoa(n int) string { return strconv.Itoa(n) }

func renew(ctx context.Context, q Q, t *Task) (bool, error) {
	tag, err := q.Exec(ctx, `UPDATE ai_job SET lease_expires_at=now()+make_interval(secs=>$3::float8), heartbeat_at=now()
		WHERE job_id=$1 AND status='running' AND lease_token=$2 AND lease_expires_at > now()`, t.ID, t.Token, leaseSeconds)
	return tag.RowsAffected() == 1, err
}

// currentJob locks the job if the task still holds its lease.
func currentJob(ctx context.Context, q Q, t *Task) (*jobRow, error) {
	j := &jobRow{}
	err := j.scan(q.QueryRow(ctx, `SELECT `+jobCols+` FROM ai_job WHERE job_id=$1 AND status='running' AND lease_token=$2 AND lease_expires_at > now() FOR UPDATE`, t.ID, t.Token))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return j, err
}

type jobRow struct {
	ID, Attempts       int64
	Kind, Status       string
	PhotoID, ProfileID *int64
	Payload, Result    map[string]any
	CreatedAt          time.Time
	FinishedAt         *time.Time
}

const jobCols = `job_id, kind, photo_id, profile_id, payload, result, status, attempts, created_at, finished_at`

func (j *jobRow) scan(r pgx.Row) error {
	return r.Scan(&j.ID, &j.Kind, &j.PhotoID, &j.ProfileID, &j.Payload, &j.Result, &j.Status, &j.Attempts, &j.CreatedAt, &j.FinishedAt)
}

func (j *jobRow) dto() map[string]any {
	op := j.Kind == "operation"
	var command, generation any
	if op {
		command, generation = j.Payload["command"], j.Payload["generation"]
	}
	stage := j.Status
	if s, ok := j.Result["step"]; ok {
		stage = s.(string)
	}
	return map[string]any{"operationId": j.ID, "kind": j.Kind, "photoId": j.PhotoID, "profileId": j.ProfileID, "command": command,
		"generation": generation, "stage": stage, "status": j.Status, "attempts": j.Attempts, "result": j.Result,
		"createdAt": iso(j.CreatedAt), "finishedAt": isoPtr(j.FinishedAt)}
}

func finish(ctx context.Context, q Q, t *Task, status string, result map[string]any) (bool, error) {
	if status == "" {
		status = "succeeded"
	}
	if result == nil {
		result = map[string]any{}
	}
	tag, err := q.Exec(ctx, `UPDATE ai_job SET status=$3, result=$4, reserved_bytes=0, finished_at=now()
		WHERE job_id=$1 AND status='running' AND lease_token=$2 AND lease_expires_at > now()`, t.ID, t.Token, status, result)
	return tag.RowsAffected() == 1, err
}

func failJob(ctx context.Context, q Q, t *Task, code string, transient bool) error {
	j, err := currentJob(ctx, q, t)
	if err != nil || j == nil {
		return err
	}
	if !transient {
		j.Attempts++
	}
	status := "queued"
	if j.Attempts >= 5 {
		status = "failed"
	}
	backoff := min(900, 10<<j.Attempts)
	if _, err = q.Exec(ctx, `UPDATE ai_job SET attempts=$2, status=$3, result=$4, reserved_bytes=0, next_run_at=now()+make_interval(secs=>$5::float8),
		finished_at=CASE WHEN $3='failed' THEN now() ELSE NULL END WHERE job_id=$1`, t.ID, j.Attempts, status, map[string]any{"reason": code}, backoff); err != nil {
		return err
	}
	if status == "failed" {
		_, err = q.Exec(ctx, `WITH s AS (UPDATE photo_embedding_state SET status='failed', embedding=NULL, error_code=$4, frame_count=0
			WHERE photo_id=$1 AND profile_id=$2 AND content_version=$3 RETURNING state_id)
			DELETE FROM video_frame_embedding WHERE state_id IN (SELECT state_id FROM s)`, j.PhotoID, j.ProfileID, t.Version, code)
	}
	return err
}

// ---- background throttle (durable rolling window; caller holds the global lock) ----

func throttleWindow(observed map[string]any, cfg Settings, now time.Time) (starts []time.Time, status map[string]any) {
	raw, _ := observed["starts"].([]any)
	for _, v := range raw {
		if s, ok := v.(string); ok {
			if t, err := time.Parse(time.RFC3339Nano, s); err == nil && t.After(now.Add(-time.Minute)) {
				starts = append(starts, t)
			}
		}
	}
	var last *time.Time
	if s, ok := observed["last_started_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			last = &t
		}
	}
	var deadline time.Time
	reason := ""
	if last != nil {
		if d := last.Add(time.Duration(cfg.Int("background_interval_seconds")) * time.Second); d.After(now) {
			deadline, reason = d, "interval_wait"
		}
	}
	if m := int(cfg.Int("background_max_per_minute")); m > 0 && len(starts) >= m {
		if d := starts[len(starts)-m].Add(time.Minute); d.After(now) && !d.Before(deadline) {
			deadline, reason = d, "rate_limited"
		}
	}
	status = map[string]any{"lastStartedAt": nil, "startsLastMinute": len(starts), "nextAllowedAt": nil, "reason": nil, "maxConcurrency": 1, "batchSize": 1}
	if last != nil {
		status["lastStartedAt"] = iso(*last)
	}
	if reason != "" {
		status["nextAllowedAt"], status["reason"] = iso(deadline), reason
	}
	return
}

func backgroundStatus(ctx context.Context, q Q, cfg Settings) (map[string]any, error) {
	var observed map[string]any
	err := q.QueryRow(ctx, `SELECT observed FROM ai_control_state WHERE name='background_throttle'`).Scan(&observed)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	_, st := throttleWindow(observed, cfg, time.Now().UTC())
	return st, err
}

func admit(ctx context.Context, q Q, cfg Settings) (bool, error) {
	row, err := control(ctx, q, "background_throttle", false)
	if err != nil {
		return false, err
	}
	now := time.Now().UTC()
	starts, st := throttleWindow(row.Observed, cfg, now)
	if st["reason"] != nil {
		return false, nil
	}
	starts = append(starts, now)
	if len(starts) > 600 {
		starts = starts[len(starts)-600:]
	}
	out := make([]string, len(starts))
	for i, s := range starts {
		out[i] = s.Format(time.RFC3339Nano)
	}
	_, err = q.Exec(ctx, `UPDATE ai_control_state SET observed=$1 WHERE name='background_throttle'`,
		map[string]any{"starts": out, "last_started_at": now.Format(time.RFC3339Nano)})
	return true, err
}

// ---- storing embeddings ----

// Snapshot is what an embedding was computed from.
type Snapshot struct {
	UserID  int64
	Version int32
	Hash    string
	Data    []byte
	Frames  []Frame // videos
}

type Frame struct {
	TimestampMS int64
	Data        []byte
}

func halfvecs(vs [][]float32) []pgvector.HalfVector {
	out := make([]pgvector.HalfVector, len(vs))
	for i, v := range vs {
		out[i] = pgvector.NewHalfVector(v)
	}
	return out
}

// upsertState writes a ready embedding (one vector, or frames for a video) for photo+profile.
func upsertState(ctx context.Context, tx pgx.Tx, photoID int64, profileID int64, version int32, hash string, vectors [][]float32, frames []Frame, durationMS int, source string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM video_frame_embedding WHERE state_id IN (SELECT state_id FROM photo_embedding_state WHERE photo_id=$1 AND profile_id=$2)`, photoID, profileID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE photo SET content_sha256=COALESCE(NULLIF($2,''), content_sha256) WHERE photo_id=$1`, photoID, hash); err != nil {
		return err
	}
	video := frames != nil
	var value any
	count := 0
	var sampling any
	if video {
		count, sampling = len(vectors), samplingVersion
	} else {
		value = pgvector.NewHalfVector(vectors[0])
	}
	spot := source == "client" && randFloat() < spotCheckRate
	var stateID int64
	err := tx.QueryRow(ctx, `INSERT INTO photo_embedding_state (photo_id, profile_id, content_version, content_sha256, status, embedding, frame_count, sampling_version, computed_at, duration_ms, source, spot_check)
		VALUES ($1,$2,$3,$4,'ready',$5,$6,$7,now(),$8,$9,$10)
		ON CONFLICT ON CONSTRAINT uq_embedding_profile_photo DO UPDATE SET content_version=EXCLUDED.content_version, content_sha256=EXCLUDED.content_sha256,
		status='ready', embedding=EXCLUDED.embedding, frame_count=EXCLUDED.frame_count, sampling_version=EXCLUDED.sampling_version, computed_at=now(),
		duration_ms=EXCLUDED.duration_ms, error_code=NULL, source=EXCLUDED.source, spot_check=EXCLUDED.spot_check, spot_checked_at=NULL
		RETURNING state_id`, photoID, profileID, version, hash, value, count, sampling, durationMS, source, spot).Scan(&stateID)
	if err != nil {
		return err
	}
	for i, f := range frames {
		if _, err := tx.Exec(ctx, `INSERT INTO video_frame_embedding (state_id, profile_id, frame_index, timestamp_ms, embedding) VALUES ($1,$2,$3,$4,$5)`,
			stateID, profileID, i, f.TimestampMS, pgvector.NewHalfVector(vectors[i])); err != nil {
			return err
		}
	}
	return nil
}

// commitEmbedding stores a server embedding. Lock order: user -> control -> job.
func commitEmbedding(ctx context.Context, tx pgx.Tx, t *Task, snap *Snapshot, vectors [][]float32, durationMS int) (bool, error) {
	var enabled bool
	err := tx.QueryRow(ctx, `SELECT ai_search_enabled FROM "user" WHERE user_id=$1 FOR UPDATE`, snap.UserID).Scan(&enabled)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if _, err = control(ctx, tx, "global", true); err != nil {
		return false, err
	}
	job, err := currentJob(ctx, tx, t)
	if err != nil || job == nil {
		return false, err
	}
	cfg, err := loadSettings(ctx, tx)
	if err != nil {
		return false, err
	}
	var version int32
	var sha *string
	err = tx.QueryRow(ctx, `SELECT content_version, content_sha256 FROM photo WHERE photo_id=$1`, t.PhotoID).Scan(&version, &sha)
	missing := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !missing {
		return false, err
	}
	pid := *t.ProfileID
	if !enabled || !cfg.Bool("enabled") || missing || version != *t.Version || (sha != nil && *sha != snap.Hash) ||
		!(cfg.isPID("search_profile_id", pid) || cfg.isPID("building_profile_id", pid)) {
		return finish(ctx, tx, t, "cancelled", map[string]any{"reason": "stale_content_or_eligibility"})
	}
	profile, err := getProfile(ctx, tx, pid)
	if err != nil {
		return false, err
	}
	var frames []Frame
	if snap.Frames != nil {
		frames = snap.Frames
		if len(vectors) != len(frames) || len(frames) < 1 || len(frames) > maxFrames {
			return false, fail("invalid_video_frames")
		}
	}
	for _, v := range vectors {
		if len(v) != profile.Dimension {
			return false, fail("invalid_embedding", 502)
		}
	}
	if err = upsertState(ctx, tx, *t.PhotoID, pid, version, snap.Hash, vectors, frames, durationMS, "server"); err != nil {
		return false, err
	}
	return finish(ctx, tx, t, "", nil)
}
