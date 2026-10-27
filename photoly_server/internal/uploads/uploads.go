// Package uploads ports controllers/uploads.py, services/uploads.py and the upload worker.
package uploads

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/photo"
	"github.com/jackc/pgx/v5"
)

var terminal = map[string]bool{"done": true, "failed": true, "skipped": true, "cancelled": true, "expanded": true}

func isTerminal(s string) bool { return terminal[s] }

func now() time.Time { return time.Now().UTC() }

// suffix is PurePosixPath(path).suffix[1:].lower().
func suffix(p string) string {
	name := path.Base(p)
	i := strings.LastIndex(name, ".")
	if i <= 0 || i == len(name)-1 {
		return ""
	}
	return strings.ToLower(name[i+1:])
}

var media = func() map[string]bool {
	m := map[string]bool{}
	for k := range photo.ImageFormats {
		m[k] = true
	}
	for k := range photo.VideoFormats {
		m[k] = true
	}
	return m
}()

// relativePath is uploads.relative_path.
func relativePath(v any) (string, error) {
	s, ok := v.(string)
	if !ok || len(s) > 4096 || strings.Contains(s, `\`) || strings.HasPrefix(s, "/") {
		return "", photo.Err("unsafe_path", 400)
	}
	for _, part := range strings.Split(s, "/") {
		if _, err := photo.ValidName(part, false); err != nil {
			return "", err
		}
		if strings.Contains(part, ":") {
			return "", photo.Err("unsafe_path", 400)
		}
	}
	return s, nil
}

type batchRow struct {
	ID         string
	UserID     int64
	NsID       *int64
	Name       string
	Sealed     bool
	Cancelled  bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Folders    map[string]int64
	Dismissed  bool
	AckedAt    *time.Time
	foldersRaw []byte
}

const batchCols = `id, user_id, ns_id, name, sealed, cancelled, created_at, updated_at, folders, dismissed, failures_acknowledged_at`

func scanBatch(row pgx.Row) (*batchRow, error) {
	var b batchRow
	if err := row.Scan(&b.ID, &b.UserID, &b.NsID, &b.Name, &b.Sealed, &b.Cancelled, &b.CreatedAt, &b.UpdatedAt, &b.foldersRaw, &b.Dismissed, &b.AckedAt); err != nil {
		return nil, err
	}
	b.Folders = map[string]int64{}
	_ = json.Unmarshal(b.foldersRaw, &b.Folders)
	return &b, nil
}

type itemRow struct {
	ID, BatchID, Path, State, Kind string
	Size                           int64
	Error, SHA, SourceID, PairID   *string
	PhotoID, NsID                  *int64
	EntryIndex                     *int32
	Options, MediaMeta             []byte
}

const itemCols = `id, batch_id, path, size, state, error, content_sha256, photo_id, ns_id, source_id, entry_index, kind, options, media_metadata, pair_id`

func scanItem(row pgx.Row) (*itemRow, error) {
	var i itemRow
	if err := row.Scan(&i.ID, &i.BatchID, &i.Path, &i.Size, &i.State, &i.Error, &i.SHA, &i.PhotoID, &i.NsID, &i.SourceID, &i.EntryIndex, &i.Kind, &i.Options, &i.MediaMeta, &i.PairID); err != nil {
		return nil, err
	}
	return &i, nil
}

func sp(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func ip(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func (i *itemRow) JSON() map[string]any {
	return map[string]any{"id": i.ID, "batchId": i.BatchID, "path": i.Path, "size": i.Size, "state": i.State, "error": sp(i.Error),
		"photoId": ip(i.PhotoID), "nsId": ip(i.NsID), "sourceId": sp(i.SourceID)}
}

// Service wraps the shared photo service.
type Service struct {
	*photo.Service
}

func (s *Service) source(id string) (string, error) {
	norm, err := photo.Identifier(id)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.Root, ".uploads", norm), nil
}

func (s *Service) batch(ctx context.Context, q photo.DB, uid int64, id string) (*batchRow, error) {
	norm, err := photo.Identifier(id)
	if err != nil {
		return nil, err
	}
	b, err := scanBatch(q.QueryRow(ctx, `SELECT `+batchCols+` FROM upload_batch WHERE id=$1`, norm))
	if err == pgx.ErrNoRows || (err == nil && b.UserID != uid) {
		return nil, photo.Err("upload_not_found", 404)
	}
	return b, err
}

func (s *Service) item(ctx context.Context, q photo.DB, id string) (*itemRow, error) {
	i, err := scanItem(q.QueryRow(ctx, `SELECT `+itemCols+` FROM upload_item WHERE id=$1`, id))
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return i, err
}

func (s *Service) create(ctx context.Context, uid int64, data map[string]any) (any, error) {
	id, err := photo.Identifier(data["id"])
	if err != nil {
		return nil, err
	}
	err = s.Atomic(ctx, uid, func(tx pgx.Tx, u photo.UserRow, f *photo.Files) error {
		var one int
		switch err := tx.QueryRow(ctx, `SELECT 1 FROM upload_batch WHERE id=$1`, id).Scan(&one); err {
		case nil:
			_, err := s.batch(ctx, tx, uid, id)
			return err
		case pgx.ErrNoRows:
		default:
			return err
		}
		ns, ok := photo.IntOf(data["nsId"])
		if !ok {
			ns = 0
		}
		if err := s.Owned(ctx, tx, "namespace", ns, uid, -1, 400); err != nil {
			return err
		}
		name, err := photo.ValidName(data["name"], false)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO upload_batch (id, user_id, ns_id, name, created_at, updated_at, folders) VALUES ($1,$2,$3,$4,$5,$5,'{}')`, id, uid, ns, name, now())
		return err
	})
	if err != nil {
		return nil, err
	}
	return id, nil
}

func (s *Service) register(ctx context.Context, uid int64, batchID string, raw any) (any, error) {
	entries, ok := raw.([]any)
	if !ok || len(entries) < 1 || len(entries) > 500 {
		return nil, photo.Err("invalid_manifest_page", 400)
	}
	err := s.Atomic(ctx, uid, func(tx pgx.Tx, u photo.UserRow, f *photo.Files) error {
		b, err := s.batch(ctx, tx, uid, batchID)
		if err != nil {
			return err
		}
		if b.Cancelled || b.Sealed {
			return photo.Err("upload_closed", 409)
		}
		for _, e := range entries {
			d, ok := e.(map[string]any)
			if !ok {
				return photo.Err("invalid_entry", 400)
			}
			key, err := photo.Identifier(d["id"])
			if err != nil {
				return err
			}
			p, err := relativePath(d["path"])
			if err != nil {
				return err
			}
			size, ok := photo.IntOf(d["size"])
			if !ok || size < 0 || size > 9007199254740991 {
				return photo.Err("invalid_size", 400)
			}
			old, err := s.item(ctx, tx, key)
			if err != nil {
				return err
			}
			if old != nil {
				if old.BatchID != b.ID || old.Path != p || old.Size != size {
					return photo.Err("upload_id_conflict", 409)
				}
				continue
			}
			state, errc := "pending", any(nil)
			if sfx := suffix(p); !media[sfx] && sfx != "zip" {
				state, errc = "skipped", "unsupported_file_type"
			} else if size > s.Cfg.MaxContentLength {
				state, errc = "failed", "file_too_large"
			}
			if _, err = tx.Exec(ctx, `INSERT INTO upload_item (id, batch_id, path, size, state, error, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
				key, b.ID, p, size, state, errc, now()); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `UPDATE upload_batch SET updated_at=$2 WHERE id=$1`, b.ID, now())
		return err
	})
	if err != nil {
		return nil, err
	}
	return 1, nil
}

// checkedCopy is CheckedStream: exact size, free-space reserve.
func (s *Service) checkedCopy(dst io.Writer, src io.Reader, expected int64, h io.Writer) error {
	buf := make([]byte, 1<<20)
	var count int64
	for {
		n, err := src.Read(buf)
		if n > 0 {
			count += int64(n)
			if count > expected {
				return photo.Err("file_size_mismatch", 400)
			}
			if photo.DiskFree(s.Root) < uint64(n)+uint64(s.Cfg.UploadDiskReserveBytes) {
				return photo.Err("insufficient_disk_space", 507)
			}
			if h != nil {
				h.Write(buf[:n])
			}
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			if strings.Contains(err.Error(), "too large") {
				return photo.Err("file_too_large", 413)
			}
			return err
		}
	}
	if count != expected {
		return photo.Err("file_size_mismatch", 400)
	}
	return nil
}

func (s *Service) receive(ctx context.Context, uid int64, batchID, itemID string, body io.Reader) (any, error) {
	b, err := s.batch(ctx, s.Pool, uid, batchID)
	if err != nil {
		return nil, err
	}
	iid, err := photo.Identifier(itemID)
	if err != nil {
		return nil, err
	}
	it, err := s.item(ctx, s.Pool, iid)
	if err != nil {
		return nil, err
	}
	if it == nil || it.BatchID != b.ID || it.SourceID != nil {
		return nil, photo.Err("upload_not_found", 404)
	}
	if it.State != "pending" {
		return it.JSON(), nil
	}
	if b.Cancelled {
		return nil, photo.Err("upload_cancelled", 409)
	}
	dir := filepath.Join(s.Root, ".uploads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(dir, "*.part")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	h := sha256.New()
	err = s.checkedCopy(tmp, body, it.Size, h)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, err
	}
	sum := hex.EncodeToString(h.Sum(nil))
	var out *itemRow
	err = s.Atomic(ctx, uid, func(tx pgx.Tx, u photo.UserRow, f *photo.Files) error {
		b, err := s.batch(ctx, tx, uid, batchID)
		if err != nil {
			return err
		}
		cur, err := s.item(ctx, tx, iid)
		if err != nil {
			return err
		}
		if b.Cancelled || cur == nil {
			return photo.Err("upload_cancelled", 409)
		}
		if cur.State == "pending" {
			if cur.SHA != nil && *cur.SHA != sum {
				return photo.Err("source_changed", 409)
			}
			dest, _ := s.source(iid)
			if _, err = f.Remove(dest); err != nil {
				return err
			}
			if err = os.Rename(tmp.Name(), dest); err != nil {
				return err
			}
			f.Created(dest)
			t := now()
			if _, err = tx.Exec(ctx, `UPDATE upload_item SET content_sha256=$2, state='ready', error=NULL, updated_at=$3 WHERE id=$1`, iid, sum, t); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE upload_batch SET updated_at=$2 WHERE id=$1`, b.ID, t); err != nil {
				return err
			}
		}
		out, err = s.item(ctx, tx, iid)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out.JSON(), nil
}

func (s *Service) finish(ctx context.Context, uid int64, batchID string) (any, error) {
	err := s.Atomic(ctx, uid, func(tx pgx.Tx, u photo.UserRow, f *photo.Files) error {
		b, err := s.batch(ctx, tx, uid, batchID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE upload_batch SET sealed=true, updated_at=$2 WHERE id=$1`, b.ID, now())
		return err
	})
	return 1, err
}

func (s *Service) listing(ctx context.Context, uid int64, page int64, batchID string) (any, error) {
	if page < 1 {
		return nil, photo.Err("invalid_page", 400)
	}
	if batchID != "" {
		b, err := s.batch(ctx, s.Pool, uid, batchID)
		if err != nil {
			return nil, err
		}
		var total int64
		if err = s.Pool.QueryRow(ctx, `SELECT count(*) FROM upload_item WHERE batch_id=$1`, b.ID).Scan(&total); err != nil {
			return nil, err
		}
		rows, err := s.Pool.Query(ctx, `SELECT `+itemCols+` FROM upload_item WHERE batch_id=$1 ORDER BY path, id OFFSET $2 LIMIT 50`, b.ID, (page-1)*50)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			i, err := scanItem(rows)
			if err != nil {
				return nil, err
			}
			items = append(items, i.JSON())
		}
		return map[string]any{"items": items, "total": total}, rows.Err()
	}
	var total int64
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM upload_batch WHERE user_id=$1 AND NOT dismissed`, uid).Scan(&total); err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT `+batchCols+` FROM upload_batch WHERE user_id=$1 AND NOT dismissed ORDER BY created_at DESC, id OFFSET $2 LIMIT 20`, uid, (page-1)*20)
	if err != nil {
		return nil, err
	}
	var batches []*batchRow
	for rows.Next() {
		b, err := scanBatch(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		batches = append(batches, b)
	}
	rows.Close()
	counts := map[string]map[string]any{}
	if len(batches) > 0 {
		ids := make([]string, len(batches))
		for i, b := range batches {
			ids[i] = b.ID
		}
		cr, err := s.Pool.Query(ctx, `SELECT batch_id, state, count(*), sum(size)::text FROM upload_item WHERE batch_id=ANY($1) GROUP BY batch_id, state`, ids)
		if err != nil {
			return nil, err
		}
		for cr.Next() {
			var bid, state, bytes string
			var n int64
			if err := cr.Scan(&bid, &state, &n, &bytes); err != nil {
				cr.Close()
				return nil, err
			}
			if counts[bid] == nil {
				counts[bid] = map[string]any{}
			}
			// sum(bigint) is numeric: Flask emits Decimal as a JSON string.
			counts[bid][state] = map[string]any{"count": n, "bytes": bytes}
		}
		cr.Close()
	}
	var active, failed int64
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM upload_item i JOIN upload_batch b ON b.id=i.batch_id
		WHERE b.user_id=$1 AND i.state IN ('pending','ready','processing','waiting')`, uid).Scan(&active); err != nil {
		return nil, err
	}
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM upload_item i JOIN upload_batch b ON b.id=i.batch_id
		WHERE b.user_id=$1 AND NOT b.dismissed AND i.state='failed' AND (b.failures_acknowledged_at IS NULL OR i.updated_at > b.failures_acknowledged_at)`, uid).Scan(&failed); err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, b := range batches {
		c := counts[b.ID]
		if c == nil {
			c = map[string]any{}
		}
		out = append(out, map[string]any{"id": b.ID, "name": b.Name, "nsId": ip(b.NsID), "sealed": b.Sealed, "cancelled": b.Cancelled,
			"createdAt": photo.IsoAware(b.CreatedAt), "counts": c})
	}
	return map[string]any{"activeCount": active, "failedCount": failed, "batches": out, "total": total, "maxFileBytes": s.Cfg.MaxContentLength}, nil
}

func (s *Service) clearActivity(ctx context.Context, uid int64, batchID string) (any, error) {
	ids := []string{}
	err := s.Atomic(ctx, uid, func(tx pgx.Tx, u photo.UserRow, f *photo.Files) error {
		var only *batchRow
		var err error
		if batchID != "" {
			if only, err = s.batch(ctx, tx, uid, batchID); err != nil {
				return err
			}
		}
		q := `SELECT id FROM upload_batch b WHERE user_id=$1 AND NOT dismissed AND sealed AND NOT EXISTS (
			SELECT 1 FROM upload_item i WHERE i.batch_id=b.id AND i.state NOT IN ('done','failed','skipped','cancelled','expanded'))`
		args := []any{uid}
		if only != nil {
			q += ` AND id=$2`
			args = append(args, only.ID)
		}
		rows, err := tx.Query(ctx, q, args...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
		if only != nil && len(ids) == 0 && !only.Dismissed {
			return photo.Err("upload_still_active", 409)
		}
		if len(ids) > 0 {
			_, err = tx.Exec(ctx, `UPDATE upload_batch SET dismissed=true WHERE id=ANY($1)`, ids)
		}
		return err
	})
	return ids, err
}

func (s *Service) acknowledge(ctx context.Context, uid int64) (any, error) {
	err := s.Atomic(ctx, uid, func(tx pgx.Tx, u photo.UserRow, f *photo.Files) error {
		_, err := tx.Exec(ctx, `UPDATE upload_batch SET failures_acknowledged_at=$2 WHERE user_id=$1 AND NOT dismissed`, uid, now())
		return err
	})
	return 1, err
}

func (s *Service) cancel(ctx context.Context, uid int64, batchID string) (any, error) {
	err := s.Atomic(ctx, uid, func(tx pgx.Tx, u photo.UserRow, f *photo.Files) error {
		b, err := s.batch(ctx, tx, uid, batchID)
		if err != nil {
			return err
		}
		t := now()
		if _, err = tx.Exec(ctx, `UPDATE upload_batch SET cancelled=true, sealed=true, updated_at=$2 WHERE id=$1`, b.ID, t); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE upload_item SET state='cancelled', updated_at=$2 WHERE batch_id=$1 AND state IN ('pending','ready','processing','waiting')`, b.ID, t)
		return err
	})
	return 1, err
}

func (s *Service) retry(ctx context.Context, uid int64, batchID string) (any, error) {
	err := s.Atomic(ctx, uid, func(tx pgx.Tx, u photo.UserRow, f *photo.Files) error {
		b, err := s.batch(ctx, tx, uid, batchID)
		if err != nil {
			return err
		}
		t := now()
		if _, err = tx.Exec(ctx, `UPDATE upload_batch SET cancelled=false, dismissed=false, updated_at=$2 WHERE id=$1`, b.ID, t); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT `+itemCols+` FROM upload_item WHERE batch_id=$1 AND state IN ('failed','cancelled')`, b.ID)
		if err != nil {
			return err
		}
		var items []*itemRow
		for rows.Next() {
			i, err := scanItem(rows)
			if err != nil {
				rows.Close()
				return err
			}
			items = append(items, i)
		}
		rows.Close()
		for _, i := range items {
			if i.Error != nil && *i.Error == "file_too_large" {
				continue
			}
			available := i.Kind != "upload"
			if !available {
				sid := i.ID
				if i.SourceID != nil {
					sid = *i.SourceID
				}
				p, _ := s.source(sid)
				if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
					available = true
				}
			}
			state, errc := "pending", any("source_expired")
			if available {
				state, errc = "ready", nil
			}
			if _, err = tx.Exec(ctx, `UPDATE upload_item SET state=$2, error=$3, updated_at=$4 WHERE id=$1`, i.ID, state, errc, t); err != nil {
				return err
			}
			if !available && i.SourceID != nil {
				if _, err = tx.Exec(ctx, `UPDATE upload_item SET state='pending', error='source_expired', updated_at=$2 WHERE id=$1`, *i.SourceID, t); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return 1, err
}

// embedding is PUT /uploads/{batch}/items/{id}/embedding.
func (s *Service) embedding(ctx context.Context, uid int64, batchID, itemID string, data map[string]any) (any, error) {
	b, err := s.batch(ctx, s.Pool, uid, batchID)
	if err != nil {
		return nil, err
	}
	iid, err := photo.Identifier(itemID)
	if err != nil {
		return nil, err
	}
	it, err := s.item(ctx, s.Pool, iid)
	if err != nil {
		return nil, err
	}
	if it == nil || it.BatchID != b.ID || it.SourceID != nil || it.Kind != "upload" {
		return nil, photo.Err("upload_not_found", 404)
	}
	fp, _ := data["fingerprint"].(string)
	raw, _ := data["vector"].([]any)
	vec := make([]float32, 0, len(raw))
	for _, x := range raw {
		n, ok := x.(json.Number)
		if !ok {
			return nil, photo.Err("invalid_embedding", 400)
		}
		f, err := n.Float64()
		if err != nil {
			return nil, photo.Err("invalid_embedding", 400)
		}
		vec = append(vec, float32(f))
	}
	if fp == "" || !ValidVector(vec) {
		return nil, photo.Err("invalid_embedding", 400)
	}
	switch it.State {
	case "ready", "processing", "waiting":
		hold(iid, fp, vec)
		return map[string]any{"state": Held}, nil
	case "done":
		if it.PhotoID == nil {
			return nil, photo.Err("upload_not_found", 404)
		}
		st, err := StoreClientEmbedding(ctx, uid, *it.PhotoID, fp, vec)
		if err != nil {
			return nil, err
		}
		return map[string]any{"state": st}, nil
	}
	return nil, photo.Err("upload_not_ready", 409)
}

// Register mounts the /uploads routes.
func Register(a *app.App) {
	s := &Service{photo.For(a)}
	j := s.JSONHandler
	m := a.Mux
	page := func(r *http.Request) (int64, error) {
		n, ok, err := photo.ArgInt(r, "page", false)
		if err != nil {
			return 0, err
		}
		if !ok {
			return 1, nil
		}
		return n, nil
	}
	m.HandleFunc("POST /uploads", j(func(r *http.Request, uid int64) (any, error) {
		d, err := photo.Body(r)
		if err != nil {
			return nil, err
		}
		return s.create(r.Context(), uid, d)
	}))
	m.HandleFunc("GET /uploads", j(func(r *http.Request, uid int64) (any, error) {
		p, err := page(r)
		if err != nil {
			return nil, err
		}
		return s.listing(r.Context(), uid, p, "")
	}))
	m.HandleFunc("POST /uploads/{batch}/items", j(func(r *http.Request, uid int64) (any, error) {
		d, err := photo.Body(r)
		if err != nil {
			return nil, err
		}
		return s.register(r.Context(), uid, r.PathValue("batch"), d["items"])
	}))
	m.HandleFunc("GET /uploads/{batch}/items", j(func(r *http.Request, uid int64) (any, error) {
		p, err := page(r)
		if err != nil {
			return nil, err
		}
		return s.listing(r.Context(), uid, p, r.PathValue("batch"))
	}))
	m.HandleFunc("PUT /uploads/{batch}/items/{item}/content", j(func(r *http.Request, uid int64) (any, error) {
		return s.receive(r.Context(), uid, r.PathValue("batch"), r.PathValue("item"), r.Body)
	}))
	m.HandleFunc("PUT /uploads/{batch}/items/{item}/embedding", j(func(r *http.Request, uid int64) (any, error) {
		d, err := photo.Body(r)
		if err != nil {
			return nil, err
		}
		return s.embedding(r.Context(), uid, r.PathValue("batch"), r.PathValue("item"), d)
	}))
	m.HandleFunc("POST /uploads/{batch}/finish", j(func(r *http.Request, uid int64) (any, error) {
		return s.finish(r.Context(), uid, r.PathValue("batch"))
	}))
	m.HandleFunc("POST /uploads/{batch}/cancel", j(func(r *http.Request, uid int64) (any, error) {
		return s.cancel(r.Context(), uid, r.PathValue("batch"))
	}))
	m.HandleFunc("POST /uploads/{batch}/retry", j(func(r *http.Request, uid int64) (any, error) {
		return s.retry(r.Context(), uid, r.PathValue("batch"))
	}))
	m.HandleFunc("DELETE /uploads", j(func(r *http.Request, uid int64) (any, error) { return s.clearActivity(r.Context(), uid, "") }))
	m.HandleFunc("DELETE /uploads/{batch}", j(func(r *http.Request, uid int64) (any, error) {
		return s.clearActivity(r.Context(), uid, r.PathValue("batch"))
	}))
	m.HandleFunc("POST /uploads/acknowledge-failures", j(func(r *http.Request, uid int64) (any, error) { return s.acknowledge(r.Context(), uid) }))
}
