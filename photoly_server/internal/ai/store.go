package ai

// Retrieval over pgvector (app/ai/vector_store_pgvector.py): a picture is one frame, a video
// up to five; a photo's score is its best frame. Large libraries first take candidates from
// the HNSW index, then rank them exactly; if that comes up short it falls back to a full scan.

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

const embedDim = 768

type Hit struct {
	Photo     map[string]any
	Score     float64
	Timestamp *int64
}

func (h Hit) dto() map[string]any {
	m := map[string]any{"photo": h.Photo, "score": h.Score}
	if h.Timestamp != nil {
		m["matchedTimestampMs"] = *h.Timestamp
	}
	return m
}

type filter struct {
	UserID    int64
	NS, GA    *int64
	Exclude   *int64
	MinScore  *float64
	After     *[2]float64 // distance, photo id
	Candidate []int64
}

// normalize checks and unit-scales a query vector (types.py normalized).
func normalize(v []float32, dim int) ([]float32, error) {
	if len(v) != dim {
		return nil, fail("invalid_embedding", 502)
	}
	var sum float64
	for _, x := range v {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return nil, fail("invalid_embedding", 502)
		}
		sum += float64(x) * float64(x)
	}
	if sum <= 0 {
		return nil, fail("zero_embedding", 502)
	}
	n := float32(math.Sqrt(sum))
	out := make([]float32, dim)
	for i, x := range v {
		out[i] = x / n
	}
	return out, nil
}

// builder assembles the numbered parameters of one query.
type builder struct{ args []any }

func (b *builder) add(v any) string {
	b.args = append(b.args, v)
	return "$" + strconv.Itoa(len(b.args))
}

func (b *builder) vec(v []float32) string {
	return b.add(pgvector.NewHalfVector(v).String()) + "::text::halfvec(768)"
}

func (b *builder) filters(pid int64, f filter) string {
	// The profile id is a trusted literal so PostgreSQL can prove the partial HNSW predicate.
	w := []string{"p.user_id=" + b.add(f.UserID), "photo_visible(p.storage_id, p.user_id)", "s.profile_id=" + strconv.FormatInt(pid, 10), "s.status='ready'",
		"s.content_version=p.content_version", "(s.sampling_version IS NULL OR s.sampling_version=" + strconv.Itoa(samplingVersion) + ")"}
	if f.NS != nil {
		w = append(w, "p.ns_id="+b.add(*f.NS))
	}
	if f.GA != nil {
		w = append(w, "EXISTS (SELECT 1 FROM gallery_photo gp WHERE gp.photo_id=p.photo_id AND gp.ga_id="+b.add(*f.GA)+")")
	}
	if f.Exclude != nil {
		w = append(w, "p.photo_id<>"+b.add(*f.Exclude))
	}
	if f.Candidate != nil {
		w = append(w, "p.photo_id=ANY("+b.add(f.Candidate)+")")
	}
	return strings.Join(w, " AND ")
}

func distanceExpr(col string, params []string) string {
	ds := make([]string, len(params))
	for i, p := range params {
		ds[i] = "(" + col + "::halfvec(768) <=> " + p + ")"
	}
	if len(ds) == 1 {
		return ds[0]
	}
	return "least(" + strings.Join(ds, ", ") + ")"
}

// branch selects (photo_id, distance, timestamp) rows for pictures or for video frames.
func (b *builder) branch(pid int64, params []string, where string, video bool) string {
	if video {
		return `SELECT p.photo_id, ` + distanceExpr("f.embedding", params) + ` AS distance, f.timestamp_ms AS ts FROM photo p
			JOIN photo_embedding_state s ON s.photo_id=p.photo_id JOIN video_frame_embedding f ON f.state_id=s.state_id
			WHERE ` + where + ` AND f.profile_id=` + strconv.FormatInt(pid, 10) + ` AND s.sampling_version=` + strconv.Itoa(samplingVersion)
	}
	return `SELECT p.photo_id, ` + distanceExpr("s.embedding", params) + ` AS distance, NULL::bigint AS ts FROM photo p
		JOIN photo_embedding_state s ON s.photo_id=p.photo_id
		WHERE ` + where + ` AND s.embedding IS NOT NULL AND s.sampling_version IS NULL`
}

const photoCols = `p.photo_id, p.photo_name, p.upload_date, p.format, p.ns_id, p.visibility, p.token, p.photo_uuid, p.user_id`

func naive(t time.Time) string {
	if t.Nanosecond()/1000 == 0 {
		return t.Format("2006-01-02T15:04:05")
	}
	return t.Format("2006-01-02T15:04:05.000000")
}

func scanHits(rows pgx.Rows) ([]Hit, [][2]float64, error) {
	defer rows.Close()
	var hits []Hit
	var raw [][2]float64
	for rows.Next() {
		var id int64
		var name, format, token, uuid *string
		var date *time.Time
		var ns, vis, uid *int32
		var distance float64
		var ts *int64
		if err := rows.Scan(&id, &name, &date, &format, &ns, &vis, &token, &uuid, &uid, &distance, &ts); err != nil {
			return nil, nil, err
		}
		var d any
		if date != nil {
			d = naive(*date)
		}
		hits = append(hits, Hit{Photo: map[string]any{"photoId": id, "photoName": name, "uploadDate": d, "format": format, "nsId": ns,
			"visibility": vis, "token": token, "photoUuid": uuid, "userId": uid}, Score: 1 - distance, Timestamp: ts})
		raw = append(raw, [2]float64{distance, float64(id)})
	}
	return hits, raw, rows.Err()
}

// exact ranks every matching photo by its best frame.
func (b *builder) exact(pid int64, params []string, f filter, limit int) string {
	where := b.filters(pid, f)
	q := `WITH frames AS (` + b.branch(pid, params, where, false) + ` UNION ALL ` + b.branch(pid, params, where, true) + `),
		ranked AS (SELECT photo_id, distance, ts, row_number() OVER (PARTITION BY photo_id ORDER BY distance, ts) AS position FROM frames)
		SELECT ` + photoCols + `, r.distance, r.ts FROM photo p JOIN ranked r ON r.photo_id=p.photo_id WHERE r.position=1`
	if f.MinScore != nil {
		q += ` AND 1.0 - r.distance >= ` + b.add(*f.MinScore)
	}
	if f.After != nil {
		q += ` AND (r.distance > ` + b.add(f.After[0]) + ` OR (r.distance = ` + b.add(f.After[0]) + ` AND p.photo_id > ` + b.add(int64(f.After[1])) + `))`
	}
	return q + ` ORDER BY r.distance + 0.0, p.photo_id LIMIT ` + strconv.Itoa(limit)
}

func timeoutErr(err error) error {
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "57014" {
		return fail("query_timeout", 504)
	}
	return err
}

type Store struct{ pool *pgxpool.Pool }

func prepVectors(vectors [][]float32) ([][]float32, error) {
	if len(vectors) < 1 || len(vectors) > maxFrames {
		return nil, fail("invalid_embedding", 502)
	}
	out := make([][]float32, len(vectors))
	for i, v := range vectors {
		n, err := normalize(v, embedDim)
		if err != nil {
			return nil, err
		}
		out[i] = n
	}
	return out, nil
}

// Search returns the best `limit` photos of a user for the query vectors.
func (s *Store) Search(ctx context.Context, pid int64, vectors [][]float32, userID int64, limit int, f filter, timeout time.Duration) ([]Hit, error) {
	vecs, err := prepVectors(vectors)
	if err != nil {
		return nil, err
	}
	f.UserID = userID
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	deadline := time.Now().Add(timeout)
	remaining := func() error {
		left := time.Until(deadline).Milliseconds()
		if left <= 0 {
			return fail("query_timeout", 504)
		}
		_, err := tx.Exec(ctx, `SELECT set_config('statement_timeout', $1, true)`, strconv.FormatInt(left, 10))
		return err
	}
	run := func(sql string, args []any) ([]Hit, error) {
		rows, err := tx.Query(ctx, sql, args...)
		if err != nil {
			return nil, timeoutErr(err)
		}
		hits, _, err := scanHits(rows)
		return hits, timeoutErr(err)
	}
	params := func(b *builder) []string {
		ps := make([]string, len(vecs))
		for i, v := range vecs {
			ps[i] = b.vec(v)
		}
		return ps
	}
	if err := remaining(); err != nil {
		return nil, err
	}
	cb := &builder{}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM photo p JOIN photo_embedding_state s ON s.photo_id=p.photo_id WHERE `+cb.filters(pid, f), cb.args...).Scan(&count); err != nil {
		return nil, timeoutErr(err)
	}
	if count > 5000 {
		for _, g := range []string{"SET LOCAL hnsw.iterative_scan = strict_order", "SET LOCAL hnsw.ef_search = 1000", "SET LOCAL hnsw.max_scan_tuples = 20000"} {
			if _, err := tx.Exec(ctx, g); err != nil {
				return nil, err
			}
		}
		cands := map[int64]bool{}
		for _, v := range vecs {
			for _, video := range []bool{false, true} {
				if err := remaining(); err != nil {
					return nil, err
				}
				b := &builder{}
				p := b.vec(v)
				q := `SELECT photo_id FROM (` + b.branch(pid, []string{p}, b.filters(pid, f), video) + `) c ORDER BY distance LIMIT ` + strconv.Itoa(limit*maxFrames)
				rows, err := tx.Query(ctx, q, b.args...)
				if err != nil {
					return nil, timeoutErr(err)
				}
				for rows.Next() {
					var id int64
					if err := rows.Scan(&id); err != nil {
						rows.Close()
						return nil, err
					}
					cands[id] = true
				}
				rows.Close()
				if err := rows.Err(); err != nil {
					return nil, timeoutErr(err)
				}
			}
		}
		if err := remaining(); err != nil {
			return nil, err
		}
		ids := make([]int64, 0, len(cands))
		for id := range cands {
			ids = append(ids, id)
		}
		cf := f
		cf.Candidate = ids
		b := &builder{}
		hits, err := run(b.exact(pid, params(b), cf, limit), b.args)
		if err != nil {
			return nil, err
		}
		if len(hits) >= min(count, limit) {
			return hits, nil
		}
	}
	if err := remaining(); err != nil {
		return nil, err
	}
	b := &builder{}
	return run(b.exact(pid, params(b), f, limit), b.args)
}

// Page is a seek-paginated search: `after` is (distance, photo id) of the last item seen.
// boundary is non-nil when more results exist.
func (s *Store) Page(ctx context.Context, pid int64, vectors [][]float32, userID int64, limit int, f filter, timeout time.Duration) (hits []Hit, boundary *[2]float64, err error) {
	vecs, err := prepVectors(vectors)
	if err != nil {
		return nil, nil, err
	}
	f.UserID = userID
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT set_config('statement_timeout', $1, true)`, strconv.FormatInt(max(1, timeout.Milliseconds()), 10)); err != nil {
		return nil, nil, err
	}
	b := &builder{}
	ps := make([]string, len(vecs))
	for i, v := range vecs {
		ps[i] = b.vec(v)
	}
	rows, err := tx.Query(ctx, b.exact(pid, ps, f, limit+1), b.args...)
	if err != nil {
		return nil, nil, timeoutErr(err)
	}
	all, raw, err := scanHits(rows)
	if err != nil {
		return nil, nil, timeoutErr(err)
	}
	if len(all) > limit {
		last := raw[limit-1]
		boundary = &last
		all = all[:limit]
	}
	return all, boundary, nil
}

// storedVectors returns the vectors already computed for a photo's current content: one for
// a picture, one per frame for a video; nil when none is stored.
func storedVectors(ctx context.Context, q Q, photoID, pid int64, version int32) ([][]float32, error) {
	var stateID int64
	var emb []float32
	var sampling *int32
	err := q.QueryRow(ctx, `SELECT state_id, embedding::real[], sampling_version FROM photo_embedding_state
		WHERE photo_id=$1 AND profile_id=$2 AND content_version=$3 AND status='ready'`, photoID, pid, version).Scan(&stateID, &emb, &sampling)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if emb != nil {
		return [][]float32{emb}, nil
	}
	if sampling == nil || int(*sampling) != samplingVersion {
		return nil, nil
	}
	rows, err := q.Query(ctx, `SELECT embedding::real[] FROM video_frame_embedding WHERE state_id=$1 ORDER BY frame_index`, stateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]float32
	for rows.Next() {
		var v []float32
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ---- coverage ----

func coverage(ctx context.Context, q Q, pid *int64, userID *int64) (map[string]any, error) {
	sql := `SELECT s.status, count(*) FROM photo p JOIN "user" u ON u.user_id=p.user_id AND u.ai_search_enabled
		LEFT JOIN photo_embedding_state s ON s.photo_id=p.photo_id AND s.profile_id=$1 AND s.content_version=p.content_version`
	args := []any{pid}
	if userID != nil {
		sql += ` WHERE p.user_id=$2`
		args = append(args, *userID)
	}
	rows, err := q.Query(ctx, sql+` GROUP BY s.status`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int64{}
	var total int64
	for rows.Next() {
		var st *string
		var n int64
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}
		key := ""
		if st != nil {
			key = *st
		}
		counts[key] += n
		total += n
	}
	supported := total - counts["unsupported"]
	ratio := 1.0
	if supported > 0 {
		ratio = float64(counts["ready"]) / float64(supported)
	}
	return map[string]any{"eligible": total, "supported": supported, "ready": counts["ready"], "pending": counts["pending"] + counts[""],
		"failed": counts["failed"], "unsupported": counts["unsupported"], "ratio": ratio}, rows.Err()
}

// ---- hooks (app/ai/hooks.py) ----

// EnqueuePhoto marks a photo pending for a profile and queues its embedding job.
func EnqueuePhoto(ctx context.Context, q Q, photoID, profileID int64) error {
	var format *string
	var version int32
	if err := q.QueryRow(ctx, `SELECT format, content_version FROM photo WHERE photo_id=$1`, photoID).Scan(&format, &version); err != nil {
		return err
	}
	f := ""
	if format != nil {
		f = strings.ToLower(*format)
	}
	video := isVideo(f)
	supported := video || imageExt[f]
	var sampling any
	if video {
		sampling = samplingVersion
	}
	status := "unsupported"
	if supported {
		status = "pending"
	}
	if _, err := q.Exec(ctx, `DELETE FROM video_frame_embedding WHERE state_id IN (SELECT state_id FROM photo_embedding_state WHERE photo_id=$1 AND profile_id=$2)`, photoID, profileID); err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `INSERT INTO photo_embedding_state (photo_id, profile_id, content_version, sampling_version, frame_count, status)
		VALUES ($1,$2,$3,$4,0,$5) ON CONFLICT ON CONSTRAINT uq_embedding_profile_photo DO UPDATE SET content_version=EXCLUDED.content_version,
		embedding=NULL, frame_count=0, sampling_version=EXCLUDED.sampling_version, status=EXCLUDED.status, computed_at=NULL, error_code=NULL,
		source='server', spot_check=false, spot_checked_at=NULL`, photoID, profileID, version, sampling, status); err != nil {
		return err
	}
	if supported {
		_, err := q.Exec(ctx, `INSERT INTO ai_job (kind, photo_id, profile_id, content_version, dedup_key) VALUES ('embedding',$1,$2,$3,$4) ON CONFLICT DO NOTHING`,
			photoID, profileID, version, fmt.Sprintf("embed:%d:%d:%d", profileID, photoID, version))
		return err
	}
	return nil
}

// PhotoChanged drops a photo's vectors and queues it again for the active profiles. Callers
// bump photo.content_version first.
func PhotoChanged(ctx context.Context, q Q, photoID int64) error {
	if _, err := q.Exec(ctx, `UPDATE ai_job SET status='cancelled', reserved_bytes=0 WHERE photo_id=$1 AND status IN ('queued','running')`, photoID); err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `DELETE FROM video_frame_embedding WHERE state_id IN (SELECT state_id FROM photo_embedding_state WHERE photo_id=$1)`, photoID); err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `UPDATE photo_embedding_state SET status='pending', embedding=NULL, frame_count=0, sampling_version=NULL WHERE photo_id=$1`, photoID); err != nil {
		return err
	}
	cfg, err := loadSettings(ctx, q)
	if err != nil {
		return err
	}
	var enabled bool
	if err := q.QueryRow(ctx, `SELECT u.ai_search_enabled FROM photo p JOIN "user" u ON u.user_id=p.user_id WHERE p.photo_id=$1`, photoID).Scan(&enabled); err != nil {
		return err
	}
	if !cfg.Bool("enabled") || !enabled {
		return nil
	}
	seen := map[int64]bool{}
	for _, k := range []string{"search_profile_id", "building_profile_id"} {
		if id := cfg.PID(k); id != nil && !seen[*id] {
			seen[*id] = true
			if err := EnqueuePhoto(ctx, q, photoID, *id); err != nil {
				return err
			}
		}
	}
	return nil
}
