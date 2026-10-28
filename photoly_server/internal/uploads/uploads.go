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
	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
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
	ID        string
	UserID    int64
	NsID      *int64
	Name      string
	Sealed    bool
	Cancelled bool
	CreatedAt time.Time
	UpdatedAt time.Time
	Folders   map[string]int64
	Dismissed bool
	AckedAt   *time.Time
}

func batchFromRow(r dbgen.UploadBatch) *batchRow {
	b := &batchRow{ID: r.ID, UserID: r.UserID, NsID: r.NsID, Name: r.Name, Sealed: r.Sealed, Cancelled: r.Cancelled, CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt, Dismissed: r.Dismissed, AckedAt: r.FailuresAcknowledgedAt, Folders: map[string]int64{}}
	_ = json.Unmarshal(r.Folders, &b.Folders)
	return b
}

type itemRow struct {
	ID, BatchID, Path, State, Kind string
	Size                           int64
	Error, SHA, SourceID, PairID   *string
	PhotoID, NsID                  *int64
	EntryIndex                     *int64
	Options, MediaMeta             []byte
}

func itemFromRow(r dbgen.UploadItem) *itemRow {
	return &itemRow{ID: r.ID, BatchID: r.BatchID, Path: r.Path, State: r.State, Kind: r.Kind, Size: r.Size, Error: r.Error, SHA: r.ContentSha256,
		SourceID: r.SourceID, PairID: r.PairID, PhotoID: r.PhotoID, NsID: r.NsID, EntryIndex: r.EntryIndex, Options: r.Options, MediaMeta: r.MediaMetadata}
}

func ptr[T any](v T) *T { return &v }

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
	r, err := dbgen.New(q).GetUploadBatch(ctx, norm)
	if err == pgx.ErrNoRows || (err == nil && r.UserID != uid) {
		return nil, photo.Err("upload_not_found", 404)
	} else if err != nil {
		return nil, err
	}
	return batchFromRow(r), nil
}

func (s *Service) item(ctx context.Context, q photo.DB, id string) (*itemRow, error) {
	r, err := dbgen.New(q).GetUploadItem(ctx, id)
	if err == pgx.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return itemFromRow(r), nil
}

func (s *Service) create(ctx context.Context, uid int64, data map[string]any) (any, error) {
	id, err := photo.Identifier(data["id"])
	if err != nil {
		return nil, err
	}
	err = s.Atomic(ctx, uid, func(tx pgx.Tx, u photo.UserRow, f *photo.Files) error {
		q := dbgen.New(tx)
		switch _, err := q.GetUploadBatchOwner(ctx, id); err {
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
		return q.InsertUploadBatch(ctx, dbgen.InsertUploadBatchParams{ID: id, UserID: uid, NsID: ns, Name: name, Now: now()})
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
		q := dbgen.New(tx)
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
			state, errc := "pending", (*string)(nil)
			if sfx := suffix(p); !media[sfx] && sfx != "zip" {
				state, errc = "skipped", ptr("unsupported_file_type")
			} else if size > s.Cfg.MaxContentLength {
				state, errc = "failed", ptr("file_too_large")
			}
			if err = q.InsertUploadItem(ctx, dbgen.InsertUploadItemParams{ID: key, BatchID: b.ID, Path: p, Size: size, State: state, Error: errc, UpdatedAt: now()}); err != nil {
				return err
			}
		}
		return q.TouchUploadBatch(ctx, dbgen.TouchUploadBatchParams{ID: b.ID, UpdatedAt: now()})
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
			q := dbgen.New(tx)
			if err = q.MarkUploadItemReceived(ctx, dbgen.MarkUploadItemReceivedParams{ID: iid, Sha: sum, UpdatedAt: t}); err != nil {
				return err
			}
			if err = q.TouchUploadBatch(ctx, dbgen.TouchUploadBatchParams{ID: b.ID, UpdatedAt: t}); err != nil {
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
		return dbgen.New(tx).SealUploadBatch(ctx, dbgen.SealUploadBatchParams{ID: b.ID, UpdatedAt: now()})
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
		q := dbgen.New(s.Pool)
		total, err := q.CountUploadItems(ctx, b.ID)
		if err != nil {
			return nil, err
		}
		rows, err := q.ListUploadItems(ctx, dbgen.ListUploadItemsParams{BatchID: b.ID, Skip: (page - 1) * 50})
		if err != nil {
			return nil, err
		}
		items := []map[string]any{}
		for _, r := range rows {
			items = append(items, itemFromRow(r).JSON())
		}
		return map[string]any{"items": items, "total": total}, nil
	}
	q := dbgen.New(s.Pool)
	total, err := q.CountUploadBatches(ctx, uid)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListUploadBatches(ctx, dbgen.ListUploadBatchesParams{UserID: uid, Skip: (page - 1) * 20})
	if err != nil {
		return nil, err
	}
	batches := make([]*batchRow, len(rows))
	ids := make([]string, len(rows))
	for i, r := range rows {
		batches[i], ids[i] = batchFromRow(r), r.ID
	}
	counts := map[string]map[string]any{}
	if len(batches) > 0 {
		cr, err := q.CountUploadItemsByState(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, c := range cr {
			if counts[c.BatchID] == nil {
				counts[c.BatchID] = map[string]any{}
			}
			counts[c.BatchID][c.State] = map[string]any{"count": c.N, "bytes": c.Bytes}
		}
	}
	active, err := q.CountActiveUploadItems(ctx, uid)
	if err != nil {
		return nil, err
	}
	failed, err := q.CountUnseenFailedUploadItems(ctx, uid)
	if err != nil {
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
		q := dbgen.New(tx)
		arg := dbgen.ListFinishedUploadBatchIDsParams{UserID: uid}
		if only != nil {
			arg.OnlyID = &only.ID
		}
		if ids, err = q.ListFinishedUploadBatchIDs(ctx, arg); err != nil {
			return err
		}
		if only != nil && len(ids) == 0 && !only.Dismissed {
			return photo.Err("upload_still_active", 409)
		}
		if len(ids) > 0 {
			return q.DismissUploadBatches(ctx, ids)
		}
		return nil
	})
	return ids, err
}

func (s *Service) acknowledge(ctx context.Context, uid int64) (any, error) {
	err := s.Atomic(ctx, uid, func(tx pgx.Tx, u photo.UserRow, f *photo.Files) error {
		return dbgen.New(tx).AcknowledgeUploadFailures(ctx, dbgen.AcknowledgeUploadFailuresParams{UserID: uid, At: now()})
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
		q := dbgen.New(tx)
		if err = q.CancelUploadBatch(ctx, dbgen.CancelUploadBatchParams{ID: b.ID, UpdatedAt: t}); err != nil {
			return err
		}
		return q.CancelOpenUploadItems(ctx, dbgen.CancelOpenUploadItemsParams{BatchID: b.ID, UpdatedAt: t})
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
		q := dbgen.New(tx)
		if err = q.ReopenUploadBatch(ctx, dbgen.ReopenUploadBatchParams{ID: b.ID, UpdatedAt: t}); err != nil {
			return err
		}
		rows, err := q.ListRetryableUploadItems(ctx, b.ID)
		if err != nil {
			return err
		}
		items := make([]*itemRow, len(rows))
		for i, r := range rows {
			items[i] = itemFromRow(r)
		}
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
			state, errc := "pending", ptr("source_expired")
			if available {
				state, errc = "ready", nil
			}
			if err = q.SetUploadItemState(ctx, dbgen.SetUploadItemStateParams{ID: i.ID, State: state, Error: errc, UpdatedAt: t}); err != nil {
				return err
			}
			if !available && i.SourceID != nil {
				if err = q.SetUploadItemState(ctx, dbgen.SetUploadItemStateParams{ID: *i.SourceID, State: "pending", Error: errc, UpdatedAt: t}); err != nil {
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
