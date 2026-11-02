package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

type People struct {
	pool   *pgxpool.Pool
	root   string
	rt     *Runtime
	policy Policy
}
type PeoplePreference struct {
	Enabled    bool   `json:"enabled"`
	Paused     bool   `json:"paused"`
	Placement  string `json:"placement"`
	Generation int64  `json:"generation"`
	Available  bool   `json:"available"`
	Visible    bool   `json:"visible"`
	Choosable  bool   `json:"choosable"`
	Reason     string `json:"reason"`
}

func (p *People) preference(ctx context.Context, q Q, uid int64, lock bool) (PeoplePreference, Settings, error) {
	var pref PeoplePreference
	if lock {
		if _, err := q.Exec(ctx, `SELECT 1 FROM ai_control_state WHERE name='global' FOR SHARE`); err != nil {
			return pref, nil, err
		}
		if _, err := q.Exec(ctx, `SELECT 1 FROM "user" WHERE user_id=$1 FOR SHARE`, uid); err != nil {
			return pref, nil, err
		}
	}
	cfg, err := loadSettings(ctx, q)
	if err != nil {
		return pref, nil, err
	}
	if _, err = q.Exec(ctx, `INSERT INTO people_preference(user_id) VALUES($1) ON CONFLICT DO NOTHING`, uid); err != nil {
		return pref, nil, err
	}
	sql := `SELECT enabled,paused,placement,generation FROM people_preference WHERE user_id=$1`
	if lock {
		sql += ` FOR UPDATE`
	}
	if err = q.QueryRow(ctx, sql, uid).Scan(&pref.Enabled, &pref.Paused, &pref.Placement, &pref.Generation); err != nil {
		return pref, nil, err
	}
	var permitted bool
	if err = q.QueryRow(ctx, `SELECT ai_search_enabled FROM "user" WHERE user_id=$1`, uid).Scan(&permitted); err != nil {
		return pref, nil, err
	}
	pref.Available = cfg.Bool("enabled") && cfg.Bool("people_enabled") && permitted
	pref.Visible = pref.Available && pref.Enabled
	pref.Choosable = cfg.Str("placement_people") == "user"
	pref.Placement = effectivePlacement(cfg.Str("placement_people"), pref.Placement)
	switch {
	case !cfg.Bool("enabled"):
		pref.Reason = "ai_disabled"
	case !cfg.Bool("people_enabled"):
		pref.Reason = "people_disabled"
	case !permitted:
		pref.Reason = "ai_permission_required"
	case !pref.Enabled:
		pref.Reason = "user_disabled"
	case pref.Paused:
		pref.Reason = "paused"
	case cfg.Bool("maintenance"):
		pref.Reason = "maintenance"
	case cfg.Str("scheduler") != "running":
		pref.Reason = "scheduler_" + cfg.Str("scheduler")
	}
	return pref, cfg, nil
}
func (p *People) settings(ctx context.Context, uid int64) (any, error) {
	pref, _, err := p.preference(ctx, p.pool, uid, false)
	if err != nil {
		return nil, err
	}
	return map[string]any{"preference": pref, "model": faceManifest()}, nil
}
func (p *People) changeSettings(ctx context.Context, uid int64, d map[string]any) (any, error) {
	for k, v := range d {
		switch k {
		case "enabled", "paused":
			if !isBool(v) {
				return nil, fail("invalid_settings")
			}
		case "placement":
			if v != "server" && v != "browser" {
				return nil, fail("invalid_placement")
			}
		default:
			return nil, fail("invalid_settings")
		}
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	pref, _, err := p.preference(ctx, tx, uid, true)
	if err != nil {
		return nil, err
	}
	if d["enabled"] == true && !pref.Available {
		return nil, fail(pref.Reason, 403)
	}
	if _, ok := d["placement"]; ok && !pref.Choosable {
		return nil, fail("placement_managed", 403)
	}
	for _, key := range []string{"enabled", "paused", "placement"} {
		if v, ok := d[key]; ok {
			if _, err = tx.Exec(ctx, `UPDATE people_preference SET `+key+`=$2, updated_at=now() WHERE user_id=$1`, uid, v); err != nil {
				return nil, err
			}
		}
	}
	// Expire active leases immediately; a changed preference must not accept late work.
	if _, err = tx.Exec(ctx, `UPDATE ai_job SET status='queued',attempts=GREATEST(attempts-1,0),lease_token=NULL,lease_expires_at=NULL,next_run_at=now(),reserved_bytes=0 WHERE kind='people' AND payload->>'user_id'=$1 AND status='running'`, fmt.Sprint(uid)); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE people_media SET status='pending' WHERE user_id=$1 AND status='processing'`, uid); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return p.settings(ctx, uid)
}
func (p *People) clear(ctx context.Context, uid int64) (any, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, _, err = p.preference(ctx, tx, uid, true); err != nil {
		return nil, err
	}
	for _, sql := range []string{
		`UPDATE people_preference SET enabled=false,paused=false,generation=generation+1,discovery_after=0,updated_at=now() WHERE user_id=$1`,
		`DELETE FROM ai_job WHERE kind='people' AND (payload->>'user_id')::bigint=$1`,
		`DELETE FROM people_media WHERE user_id=$1`,
		`DELETE FROM people_person WHERE user_id=$1`,
	} {
		if _, err = tx.Exec(ctx, sql, uid); err != nil {
			return nil, err
		}
	}
	return map[string]bool{"cleared": true}, tx.Commit(ctx)
}
func (p *People) status(ctx context.Context, uid int64) (any, error) {
	pref, cfg, err := p.preference(ctx, p.pool, uid, false)
	if err != nil {
		return nil, err
	}
	waiting := pref.Reason
	if waiting == "" {
		decision := p.policy.Decide(SystemUsage(p.root).toMap(), cfg)
		if !decision.Allowed {
			waiting = strings.Join(decision.Reasons, ", ")
		}
		if waiting == "" {
			if used, e := usage(ctx, p.pool); e == nil && max(used.Estimated+used.Reserved+int64(faceMaxCount)*faceStorageCharge, used.Physical) > cfg.Int("quota_bytes") {
				waiting = "quota_exceeded"
			}
		}
		if waiting == "" && pref.Placement == "server" {
			if throttle, e := backgroundStatus(ctx, p.pool, cfg); e == nil {
				waiting, _ = throttle["reason"].(string)
			}
		}
	}
	progress, err := dbgen.New(p.pool).PeopleProgress(ctx, dbgen.PeopleProgressParams{Fingerprint: faceFingerprint, UserID: uid, Formats: formats(false)})
	if err != nil {
		return nil, err
	}
	return map[string]any{"preference": pref, "waitReason": waiting, "total": progress.Total, "ready": progress.Ready, "failed": progress.Failed, "processing": progress.Processing, "pending": max(int64(0), progress.Total-progress.Ready-progress.Failed-progress.Processing)}, nil
}
func (p *People) retry(ctx context.Context, uid int64) (any, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	pref, _, err := p.preference(ctx, tx, uid, true)
	if err != nil {
		return nil, err
	}
	if !pref.Visible {
		return nil, fail(pref.Reason, 403)
	}
	_, err = tx.Exec(ctx, `UPDATE ai_job SET status='queued',attempts=0,next_run_at=now(),result='{}',finished_at=NULL WHERE kind='people' AND payload->>'user_id'=$1 AND status='failed'`, fmt.Sprint(uid))
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE people_media SET status='pending',error=NULL WHERE user_id=$1 AND status='failed'`, uid)
	if err != nil {
		return nil, err
	}
	return map[string]bool{"queued": true}, tx.Commit(ctx)
}
func (p *People) readable(ctx context.Context, q Q, uid int64) error {
	pref, _, err := p.preference(ctx, q, uid, false)
	if err != nil {
		return err
	}
	if !pref.Visible {
		return fail(pref.Reason, 403)
	}
	return nil
}
func (p *People) mutable(ctx context.Context, tx pgx.Tx, uid int64) error {
	pref, _, err := p.preference(ctx, tx, uid, true)
	if err != nil {
		return err
	}
	if !pref.Visible {
		return fail(pref.Reason, 403)
	}
	return nil
}

// All AI results cross the same validated, version-fenced commit path.
func (p *People) store(ctx context.Context, uid int64, job PeopleJob, faces []FaceDetection) (any, error) {
	if len(faces) > faceMaxCount {
		return nil, fail("too_many_faces")
	}
	for _, f := range faces {
		if !validFace(f) {
			return nil, fail("invalid_face_result")
		}
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	pref, _, err := p.preference(ctx, tx, uid, true)
	if err != nil {
		return nil, err
	}
	if pref.Reason != "" {
		return nil, fail(pref.Reason, 409)
	}
	current, err := p.checkJob(ctx, tx, uid, job, true)
	if err != nil {
		return nil, err
	}
	if current.Status == "succeeded" {
		return map[string]bool{"stored": true}, nil
	}
	// A ready version is immutable to automatic retries: manual edits remain intact.
	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM people_media WHERE photo_id=$1 FOR UPDATE`, current.PhotoID).Scan(&status)
	if err != nil {
		return nil, err
	}
	if status != "ready" {
		for _, f := range faces {
			quality := f.Score >= .9 && f.Box[2] >= .025 && f.Box[3] >= .025
			personID := int64(0)
			if quality {
				personID, err = p.matchPerson(ctx, tx, uid, f.Vector)
				if err != nil {
					return nil, err
				}
			}
			if personID == 0 {
				if err = tx.QueryRow(ctx, `INSERT INTO people_person(user_id) VALUES($1) RETURNING person_id`, uid).Scan(&personID); err != nil {
					return nil, err
				}
			}
			var id int64
			err = tx.QueryRow(ctx, `INSERT INTO people_face(user_id,photo_id,content_version,detector_version,box,landmarks,score,quality,person_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING face_id`, uid, current.PhotoID, current.Version, faceDetectorVersion, f.Box, f.Landmarks, f.Score, quality, personID).Scan(&id)
			if err != nil {
				return nil, err
			}
			_, err = tx.Exec(ctx, `INSERT INTO people_embedding(face_id,user_id,fingerprint,embedding,source) VALUES($1,$2,$3,$4,$5)`, id, uid, faceFingerprint, pgvector.NewVector(f.Vector), current.Placement)
			if err != nil {
				return nil, err
			}
		}
	}
	_, err = tx.Exec(ctx, `UPDATE people_media SET status='ready',face_count=$2,error=NULL,updated_at=now() WHERE photo_id=$1`, current.PhotoID, len(faces))
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE ai_job SET status='succeeded',finished_at=now(),reserved_bytes=0 WHERE job_id=$1`, current.ID)
	if err != nil {
		return nil, err
	}
	return map[string]bool{"stored": true}, tx.Commit(ctx)
}
func (p *People) matchPerson(ctx context.Context, q Q, uid int64, vector []float32) (int64, error) {
	if _, err := q.Exec(ctx, `SET LOCAL hnsw.iterative_scan = 'strict_order'`); err != nil {
		return 0, err
	}
	// Query the nearest different identity separately: a large album for the
	// leading identity must not crowd the runner-up out of a fixed candidate set.
	nearest := func(exclude int64) (int64, float64, error) {
		var id int64
		var distance float64
		err := q.QueryRow(ctx, `SELECT f.person_id,e.embedding::vector(128) <=> $2::vector(128) AS distance
 FROM people_embedding e JOIN people_face f ON f.face_id=e.face_id JOIN photo ph ON ph.photo_id=f.photo_id JOIN people_person p ON p.person_id=f.person_id
 WHERE e.user_id=$1 AND e.fingerprint='yunet2023-sface2021-rgb-v1' AND f.quality AND NOT f.dismissed AND p.merged_into IS NULL AND f.person_id<>$3 AND photo_visible(ph.storage_id,ph.user_id)
 ORDER BY e.embedding::vector(128) <=> $2::vector(128) LIMIT 1`, uid, pgvector.NewVector(vector), exclude).Scan(&id, &distance)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, nil
		}
		return id, 1 - distance, err
	}
	id, score, err := nearest(0)
	if err != nil || id == 0 || score < faceMatchThreshold {
		return 0, err
	}
	other, runnerUp, err := nearest(id)
	if err != nil {
		return 0, err
	}
	if other != 0 && score-runnerUp < faceMatchMargin {
		return 0, nil
	}
	return id, nil
}

func decodeFaceBody(r *http.Request) (PeopleJob, []FaceDetection, error) {
	var d struct {
		Job   PeopleJob       `json:"job"`
		Faces []FaceDetection `json:"faces"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 2<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return PeopleJob{}, nil, fail("invalid_face_result")
	}
	return d.Job, d.Faces, nil
}
func cleanPersonName(v any) (string, error) {
	s, ok := v.(string)
	s = strings.TrimSpace(s)
	if !ok || len([]rune(s)) > 120 {
		return "", fail("invalid_person_name")
	}
	return s, nil
}
func peopleNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fail("not_found", 404)
	}
	return err
}
