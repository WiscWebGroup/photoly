package ai

import (
	"context"
	"errors"
	"slices"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Manual indexing: a user who switched between browser and server placement can ask for
// their library to be indexed now instead of waiting for the scheduler or a new upload.

// maxManualIndex bounds how many photos one request re-queues; a second request continues.
const maxManualIndex = 5000

// Indexer serves POST /ai/index and GET /ai/index/pending.
type Indexer struct {
	pool *pgxpool.Pool
	sink sink
}

// indexContext is what a manual request needs: the profile being filled and where this user's
// photos are indexed.
func (x *Indexer) indexContext(ctx context.Context, q Q, uid int64) (pid int64, placement string, err error) {
	var enabled bool
	if err = q.QueryRow(ctx, `SELECT ai_search_enabled FROM "user" WHERE user_id=$1`, uid).Scan(&enabled); err != nil {
		return 0, "", err
	}
	cfg, err := loadSettings(ctx, q)
	if err != nil {
		return 0, "", err
	}
	p := cfg.PID("building_profile_id")
	if p == nil {
		p = cfg.PID("search_profile_id")
	}
	if !enabled || !cfg.Bool("enabled") || p == nil {
		return 0, "", fail("ai_not_enabled", 403)
	}
	var choice string
	_ = q.QueryRow(ctx, `SELECT image FROM user_ai_preference WHERE user_id=$1`, uid).Scan(&choice)
	return *p, effectivePlacement(cfg.Str("placement_image"), choice), nil
}

func formats(videos bool) []string {
	var out []string
	for f := range imageExt {
		out = append(out, f)
	}
	if videos {
		for f := range videoExt {
			out = append(out, f)
		}
	}
	slices.Sort(out)
	return out
}

// notReady matches photos with no current vector (never indexed, edited since, pending or failed).
const notReady = `(s.state_id IS NULL OR s.content_version<>p.content_version OR s.status IN ('pending','failed'))`

// Trigger re-queues the user's unindexed photos for the server and lets them skip the browser
// grace period. With browser placement the server never indexes them; the count of photos the
// browser should index is returned instead.
func (x *Indexer) Trigger(ctx context.Context, uid int64) (map[string]any, error) {
	tx, err := x.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	pid, placement, err := x.indexContext(ctx, tx, uid)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT p.photo_id FROM photo p LEFT JOIN photo_embedding_state s ON s.photo_id=p.photo_id AND s.profile_id=$2
		WHERE p.user_id=$1 AND lower(p.format)=ANY($3) AND `+notReady+` AND NOT EXISTS (SELECT 1 FROM ai_job j WHERE j.photo_id=p.photo_id
			AND j.profile_id=$2 AND j.content_version=p.content_version AND j.kind='embedding' AND j.status IN ('queued','running'))
		ORDER BY p.photo_id LIMIT $4`, uid, pid, formats(true), maxManualIndex)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if err := EnqueuePhoto(ctx, tx, id, pid); err != nil {
			return nil, err
		}
	}
	if placement == "server" {
		// Skip the wait that gives a browser the first chance, and any retry backoff.
		if _, err := tx.Exec(ctx, `UPDATE ai_job j SET next_run_at=now(), created_at=LEAST(j.created_at, now()-make_interval(secs=>$3::float8))
			FROM photo p WHERE p.photo_id=j.photo_id AND p.user_id=$1 AND j.kind='embedding' AND j.status='queued' AND j.profile_id=$2`,
			uid, pid, browserGrace.Seconds()); err != nil {
			return nil, err
		}
	}
	var queued int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM ai_job j JOIN photo p ON p.photo_id=j.photo_id WHERE p.user_id=$1 AND j.profile_id=$2
		AND j.kind='embedding' AND j.status IN ('queued','running')`, uid, pid).Scan(&queued); err != nil {
		return nil, err
	}
	forBrowser, err := x.pendingCount(ctx, tx, uid, pid)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	out := map[string]any{"placement": placement, "requeued": len(ids), "serverQueued": queued, "browserPending": 0, "more": len(ids) == maxManualIndex}
	if placement == "browser" {
		out["browserPending"], out["serverQueued"] = forBrowser, 0
	}
	return out, nil
}

func (x *Indexer) pendingCount(ctx context.Context, q Q, uid, pid int64) (n int, err error) {
	err = q.QueryRow(ctx, `SELECT count(*) FROM photo p LEFT JOIN photo_embedding_state s ON s.photo_id=p.photo_id AND s.profile_id=$2
		WHERE p.user_id=$1 AND lower(p.format)=ANY($3) AND `+notReady, uid, pid, formats(false)).Scan(&n)
	return n, err
}

// Pending lists photos this browser should embed (images only; videos stay on the server).
func (x *Indexer) Pending(ctx context.Context, uid int64, q map[string]string) (map[string]any, error) {
	pid, placement, err := x.indexContext(ctx, x.pool, uid)
	if err != nil {
		return nil, err
	}
	if placement != "browser" {
		return map[string]any{"placement": placement, "total": 0, "items": []any{}}, nil
	}
	limit, after := 20, int64(0)
	if n, err := strconv.Atoi(q["limit"]); err == nil && n > 0 {
		limit = min(n, 100)
	}
	if n, err := strconv.ParseInt(q["after"], 10, 64); err == nil {
		after = n
	}
	total, err := x.pendingCount(ctx, x.pool, uid, pid)
	if err != nil {
		return nil, err
	}
	rows, err := x.pool.Query(ctx, `SELECT p.photo_id, COALESCE(p.photo_name, '') FROM photo p LEFT JOIN photo_embedding_state s ON s.photo_id=p.photo_id AND s.profile_id=$2
		WHERE p.user_id=$1 AND p.photo_id>$4 AND lower(p.format)=ANY($3) AND `+notReady+` ORDER BY p.photo_id LIMIT $5`,
		uid, pid, formats(false), after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"photoId": id, "name": name})
	}
	return map[string]any{"placement": placement, "total": total, "items": items}, rows.Err()
}

// Store accepts one browser-made vector for an existing photo.
func (x *Indexer) Store(ctx context.Context, uid, photoID int64, d map[string]any) (map[string]any, error) {
	vec, ok := vectorFrom(d["vector"])
	fp, _ := d["fingerprint"].(string)
	if !ok {
		return nil, fail("invalid_embedding")
	}
	var one int
	if err := x.pool.QueryRow(ctx, `SELECT 1 FROM photo WHERE photo_id=$1 AND user_id=$2`, photoID, uid).Scan(&one); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fail("not_found", 404)
		}
		return nil, err
	}
	res, err := x.sink.StoreClient(ctx, uid, photoID, fp, vec)
	if err != nil {
		return nil, err
	}
	return map[string]any{"status": res}, nil
}
