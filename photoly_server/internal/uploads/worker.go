package uploads

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/photo"
	"github.com/jackc/pgx/v5"
)

const (
	sessionLock = 74831004
	tickEvery   = 2 * time.Second
	cleanEvery  = 300 * time.Second
)

var (
	covers     = map[string]bool{"jpg": true, "jpeg": true, "heic": true, "heif": true}
	candidates = map[string]bool{"jpg": true, "jpeg": true, "heic": true, "heif": true, "mov": true}
)

// checked is CheckedStream over an io.Reader.
type checked struct {
	s        *Service
	r        io.Reader
	expected int64
	count    int64
}

func (c *checked) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.count += int64(n)
	if c.count > c.expected {
		return 0, photo.Err("file_size_mismatch", 400)
	}
	if n > 0 && photo.DiskFree(c.s.Root) < uint64(n)+uint64(c.s.Cfg.UploadDiskReserveBytes) {
		return 0, photo.Err("insufficient_disk_space", 507)
	}
	if err == io.EOF && c.count != c.expected {
		return n, photo.Err("file_size_mismatch", 400)
	}
	return n, err
}

// open returns the content of an item: its own upload or a ZIP entry.
func (s *Service) open(it *itemRow) (io.ReadCloser, error) {
	if it.SourceID != nil {
		p, err := s.source(*it.SourceID)
		if err != nil {
			return nil, err
		}
		zr, err := zip.OpenReader(p)
		if err != nil {
			return nil, err
		}
		if it.EntryIndex == nil || int(*it.EntryIndex) >= len(zr.File) {
			zr.Close()
			return nil, zip.ErrFormat
		}
		rc, err := zr.File[*it.EntryIndex].Open()
		if err != nil {
			zr.Close()
			return nil, err
		}
		return &zipEntry{rc, zr}, nil
	}
	p, err := s.source(it.ID)
	if err != nil {
		return nil, err
	}
	return os.Open(p)
}

type zipEntry struct {
	io.ReadCloser
	z *zip.ReadCloser
}

func (z *zipEntry) Close() error { z.ReadCloser.Close(); return z.z.Close() }

func (s *Service) materialize(it *itemRow, dest string) error {
	rc, err := s.open(it)
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, &checked{s: s, r: rc, expected: it.Size})
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}

// folder is UploadService.folder: create the directory chain under the batch namespace.
func (s *Service) folder(ctx context.Context, tx pgx.Tx, b *batchRow, p string) (int64, error) {
	var base int64 = -1
	if b.NsID != nil {
		base = *b.NsID
	}
	if err := s.Owned(ctx, tx, "namespace", base, b.UserID, "destination_deleted", 404); err != nil {
		return 0, err
	}
	parent := base
	parts := strings.Split(p, "/")
	parts = parts[:len(parts)-1]
	dirty := false
	for i, name := range parts {
		key := strings.Join(parts[:i+1], "/")
		if id, ok := b.Folders[key]; ok {
			if err := s.Owned(ctx, tx, "namespace", id, b.UserID, "destination_deleted", 404); err != nil {
				return 0, err
			}
			parent = id
			continue
		}
		cand, n := name, 1
		for {
			var one int64
			err := tx.QueryRow(ctx, `SELECT ns_id FROM namespace WHERE user_id=$1 AND ns_parent_id=$2 AND ns_name=$3 LIMIT 1`, b.UserID, parent, cand).Scan(&one)
			if err == pgx.ErrNoRows {
				break
			}
			if err != nil {
				return 0, err
			}
			tail := fmt.Sprintf(" (%d)", n)
			r := []rune(name)
			if len(r) > 255-len(tail) {
				r = r[:255-len(tail)]
			}
			cand, n = string(r)+tail, n+1
		}
		var id int64
		if err := tx.QueryRow(ctx, `INSERT INTO namespace (ns_name, ns_parent_id, user_id) VALUES ($1,$2,$3) RETURNING ns_id`, cand, parent, b.UserID).Scan(&id); err != nil {
			return 0, err
		}
		parent, b.Folders[key], dirty = id, id, true
	}
	if dirty {
		raw, _ := json.Marshal(b.Folders)
		if _, err := tx.Exec(ctx, `UPDATE upload_batch SET folders=$2 WHERE id=$1`, b.ID, raw); err != nil {
			return 0, err
		}
	}
	return parent, nil
}

func (s *Service) install(ctx context.Context, tx pgx.Tx, f *photo.Files, u photo.UserRow, it *itemRow, ns int64, r io.Reader, prep *photo.Prepared) (*photo.Photo, error) {
	p, err := s.InstallOne(ctx, tx, f, u, path.Base(it.Path), &checked{s: s, r: r, expected: it.Size}, ns, int64(0), prep)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE upload_item SET photo_id=$2, ns_id=$3, state='done', error=NULL, updated_at=$4 WHERE id=$1`, it.ID, p.ID, ns, now())
	it.PhotoID, it.State = &p.ID, "done"
	if err == nil {
		// Own connection, and it takes the user-row lock this tx holds: run it after the tx commits.
		go s.applyHeld(ctx, it.ID, u.ID, p.ID)
	}
	return p, err
}

// applyHeld stores a browser embedding that arrived before the photo existed.
func (s *Service) applyHeld(ctx context.Context, item string, uid, photoID int64) {
	if h, ok := takeHeld(item); ok {
		if _, err := StoreClientEmbedding(ctx, uid, photoID, h.fingerprint, h.vector); err != nil {
			log.Printf("uploads: client embedding for %s: %v", item, err)
		}
	}
}

func (s *Service) expand(ctx context.Context, tx pgx.Tx, it *itemRow, b *batchRow) error {
	src, err := s.source(it.ID)
	if err != nil {
		return err
	}
	zr, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()
	var total int64
	for _, e := range zr.File {
		if _, err := relativePath(strings.TrimRight(e.Name, "/")); err != nil {
			return err
		}
		m := e.Mode()
		if e.Flags&1 != 0 || m&os.ModeSymlink != 0 || (m.Type() != 0 && !m.IsDir()) {
			return photo.Err("unsafe_or_encrypted_zip", 400)
		}
		total += int64(e.UncompressedSize64)
		if total > s.Cfg.UploadZipMaxBytes {
			return photo.Err("zip_expansion_limit", 400)
		}
	}
	need := total
	if s.Cfg.MaxContentLength < need {
		need = s.Cfg.MaxContentLength
	}
	if photo.DiskFree(s.Root) < uint64(need)+uint64(s.Cfg.UploadDiskReserveBytes) {
		return photo.Err("insufficient_disk_space", 507)
	}
	prefix := it.Path
	if i := strings.LastIndex(prefix, "."); i > strings.LastIndex(prefix, "/")+1 {
		prefix = prefix[:i]
	}
	_, ns, _ := photo.ParseUUID(it.ID)
	for idx, e := range zr.File {
		if e.FileInfo().IsDir() {
			continue
		}
		key := photo.UUID5(ns, fmt.Sprint(idx))
		p, err := relativePath(prefix + "/" + e.Name)
		if err != nil {
			return err
		}
		state, errc := "ready", any(nil)
		if !media[suffix(p)] {
			state, errc = "skipped", "unsupported_file_type"
		} else if int64(e.UncompressedSize64) > s.Cfg.MaxContentLength {
			state, errc = "failed", "file_too_large"
		}
		old, err := s.item(ctx, tx, key)
		if err != nil {
			return err
		}
		if old == nil {
			_, err = tx.Exec(ctx, `INSERT INTO upload_item (id, batch_id, path, size, state, error, source_id, entry_index, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
				key, b.ID, p, int64(e.UncompressedSize64), state, errc, it.ID, idx, now())
		} else if old.State == "pending" {
			_, err = tx.Exec(ctx, `UPDATE upload_item SET state=$2, error=$3, updated_at=$4 WHERE id=$1`, key, state, errc, now())
		}
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE upload_item SET state='expanded', error=NULL WHERE id=$1`, it.ID)
	return err
}

// errCode is the item.error text for a failed worker run.
func errCode(err error) (string, bool) {
	var b *photo.Biz
	if errors.As(err, &b) {
		return fmt.Sprint(b.Value), true
	}
	if errors.Is(err, zip.ErrFormat) || errors.Is(err, zip.ErrAlgorithm) || errors.Is(err, zip.ErrChecksum) {
		return "invalid_zip", true
	}
	return "processing_failed", false
}

// process is UploadService.process.
func (s *Service) process(ctx context.Context, id string) {
	it, err := s.item(ctx, s.Pool, id)
	if err != nil || it == nil || !(it.State == "ready" || it.State == "processing" || it.State == "waiting") {
		return
	}
	var uid int64
	if err := s.Pool.QueryRow(ctx, `SELECT user_id FROM upload_batch WHERE id=$1`, it.BatchID).Scan(&uid); err != nil {
		return
	}
	skip := false
	err = s.Atomic(ctx, uid, func(tx pgx.Tx, u photo.UserRow, f *photo.Files) error {
		cur, err := s.item(ctx, tx, id)
		if err != nil || cur == nil {
			return err
		}
		b, err := s.batch(ctx, tx, uid, cur.BatchID)
		if err != nil {
			return err
		}
		if b.Cancelled || !(cur.State == "ready" || cur.State == "processing" || cur.State == "waiting") {
			skip = true
			return nil
		}
		it = cur
		_, err = tx.Exec(ctx, `UPDATE upload_item SET state='processing', updated_at=$2 WHERE id=$1`, id, now())
		return err
	})
	if err != nil || skip {
		return
	}
	if err = s.run(ctx, uid, it); err != nil {
		code, known := errCode(err)
		if !known {
			log.Printf("uploads: import failed %s: %v", id, err)
		}
		_ = s.Atomic(ctx, uid, func(tx pgx.Tx, u photo.UserRow, f *photo.Files) error {
			cur, err := s.item(ctx, tx, id)
			if err != nil || cur == nil || cur.State != "processing" {
				return err
			}
			t := now()
			if _, err = tx.Exec(ctx, `UPDATE upload_item SET state='failed', error=$2, updated_at=$3 WHERE id=$1`, id, code, t); err != nil {
				return err
			}
			if cur.PairID != nil {
				_, err = tx.Exec(ctx, `UPDATE upload_item SET state='failed', error=$2, updated_at=$3 WHERE id=$1 AND state='processing'`, *cur.PairID, code, t)
			}
			return err
		})
	}
}

func (s *Service) run(ctx context.Context, uid int64, it *itemRow) error {
	if it.Kind != "upload" {
		return s.ProcessMedia(ctx, it.ID)
	}
	if suffix(it.Path) != "zip" {
		return s.processImport(ctx, uid, it)
	}
	return s.Atomic(ctx, uid, func(tx pgx.Tx, u photo.UserRow, f *photo.Files) error {
		b, err := s.batch(ctx, tx, uid, it.BatchID)
		if err != nil {
			return err
		}
		cur, err := s.item(ctx, tx, it.ID)
		if err != nil || cur == nil || b.Cancelled || cur.State != "processing" {
			return err
		}
		if cur.SourceID == nil {
			if err = s.expand(ctx, tx, cur, b); err != nil {
				return err
			}
		} else {
			// A nested archive entry is not expanded again: import it as a file (unsupported type).
			return photo.Err("unsupported_file_type", 400)
		}
		_, err = tx.Exec(ctx, `UPDATE upload_item SET updated_at=$2 WHERE id=$1`, it.ID, now())
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE upload_batch SET updated_at=$2 WHERE id=$1`, b.ID, now())
		}
		return err
	})
}

func (s *Service) siblings(ctx context.Context, batchID string) ([]*itemRow, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+itemCols+` FROM upload_item WHERE batch_id=$1 AND kind='upload'`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*itemRow
	for rows.Next() {
		i, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func identifierOf(raw []byte) string {
	var m struct {
		Identifier *string `json:"identifier"`
	}
	_ = json.Unmarshal(raw, &m)
	if m.Identifier == nil {
		return ""
	}
	return *m.Identifier
}

func (s *Service) wait(ctx context.Context, uid int64, id string) error {
	return s.Atomic(ctx, uid, func(tx pgx.Tx, u photo.UserRow, f *photo.Files) error {
		_, err := tx.Exec(ctx, `UPDATE upload_item SET state='waiting', updated_at=$2 WHERE id=$1 AND state='processing'`, id, now())
		return err
	})
}

// processImport is live_import.process_import for non-archive uploads.
func (s *Service) processImport(ctx context.Context, uid int64, it *itemRow) error {
	staging := filepath.Join(s.Root, ".staging")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return err
	}
	work, err := os.MkdirTemp(staging, "import-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	sfx := suffix(it.Path)
	original := filepath.Join(work, "source."+sfx)
	if err := s.materialize(it, original); err != nil {
		return err
	}
	ident := identifierOf(it.MediaMeta)
	if it.MediaMeta == nil && candidates[sfx] {
		meta := s.ReadMetadata(ctx, original)
		id, _ := meta["ContentIdentifier"].(string)
		ft := strings.ToUpper(meta.String("FileType"))
		valid := ft == "JPEG" || ft == "HEIC" || ft == "HEIF"
		if sfx == "mov" {
			valid = ft == "MOV"
		}
		var pairing any = map[string]any{"identifier": nil}
		ident = ""
		if valid && id != "" && len(id) <= 255 {
			pairing, ident = map[string]any{"identifier": id}, id
		}
		raw, _ := json.Marshal(pairing)
		stop := false
		if err := s.Atomic(ctx, uid, func(tx pgx.Tx, u photo.UserRow, f *photo.Files) error {
			cur, err := s.item(ctx, tx, it.ID)
			if err != nil || cur == nil || cur.State != "processing" {
				stop = true
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE upload_item SET media_metadata=$2 WHERE id=$1`, it.ID, raw)
			return err
		}); err != nil || stop {
			return err
		}
		it.MediaMeta = raw
	}
	if ident != "" {
		done, err := s.pair(ctx, uid, it, ident, work, original)
		if err != nil || done {
			return err
		}
	}
	// No reliable unique counterpart: import exactly as a normal file.
	prep, err := s.Prepare(ctx, original, work)
	if err != nil {
		return err
	}
	return s.Atomic(ctx, uid, func(tx pgx.Tx, u photo.UserRow, f *photo.Files) error {
		cur, err := s.item(ctx, tx, it.ID)
		if err != nil || cur == nil {
			return err
		}
		b, err := s.batch(ctx, tx, uid, cur.BatchID)
		if err != nil || b.Cancelled || cur.State != "processing" {
			return err
		}
		ns, err := s.folder(ctx, tx, b, cur.Path)
		if err != nil {
			return err
		}
		in, err := os.Open(original)
		if err != nil {
			return err
		}
		defer in.Close()
		if _, err = s.install(ctx, tx, f, u, cur, ns, in, prep); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE upload_batch SET updated_at=$2 WHERE id=$1`, b.ID, now())
		return err
	})
}

// pair handles an item carrying a ContentIdentifier. It reports whether the item was fully handled.
func (s *Service) pair(ctx context.Context, uid int64, it *itemRow, ident, work, original string) (bool, error) {
	var sealed bool
	if err := s.Pool.QueryRow(ctx, `SELECT sealed FROM upload_batch WHERE id=$1`, it.BatchID).Scan(&sealed); err != nil {
		return true, err
	}
	sibs, err := s.siblings(ctx, it.BatchID)
	if err != nil {
		return true, err
	}
	parent := path.Dir(it.Path)
	var cands []*itemRow
	unsettled, incomplete := false, !sealed
	for _, i := range sibs {
		if suffix(i.Path) == "zip" && !isTerminal(i.State) {
			unsettled = true
		}
		if path.Dir(i.Path) == parent && candidates[suffix(i.Path)] {
			cands = append(cands, i)
			if !isTerminal(i.State) && i.MediaMeta == nil {
				incomplete = true
			}
		}
	}
	if incomplete || unsettled {
		return true, s.wait(ctx, uid, it.ID)
	}
	var cover, movie *itemRow
	nc, nm := 0, 0
	for _, i := range cands {
		if identifierOf(i.MediaMeta) != ident {
			continue
		}
		if i.State != "ready" && i.State != "waiting" && i.State != "processing" {
			return false, nil
		}
		if covers[suffix(i.Path)] {
			cover, nc = i, nc+1
		} else if suffix(i.Path) == "mov" {
			movie, nm = i, nm+1
		}
	}
	if nc != 1 || nm != 1 {
		return false, nil
	}
	if it.ID != cover.ID {
		original = filepath.Join(work, "cover."+suffix(cover.Path))
		if err := s.materialize(cover, original); err != nil {
			return true, err
		}
	}
	moviePath := filepath.Join(work, "live.mov")
	if err := s.materialize(movie, moviePath); err != nil {
		return true, err
	}
	ok := false
	if err := s.Atomic(ctx, uid, func(tx pgx.Tx, u photo.UserRow, f *photo.Files) error {
		b, err := s.batch(ctx, tx, uid, it.BatchID)
		if err != nil || b.Cancelled {
			return err
		}
		for _, i := range []*itemRow{cover, movie} {
			cur, err := s.item(ctx, tx, i.ID)
			if err != nil || cur == nil || !(cur.State == "ready" || cur.State == "waiting" || cur.State == "processing") {
				return err
			}
		}
		t := now()
		for _, p := range [][2]string{{cover.ID, movie.ID}, {movie.ID, cover.ID}} {
			if _, err = tx.Exec(ctx, `UPDATE upload_item SET pair_id=$2, state='processing', updated_at=$3 WHERE id=$1`, p[0], p[1], t); err != nil {
				return err
			}
		}
		ok = true
		return nil
	}); err != nil || !ok {
		return true, err
	}
	preview := filepath.Join(work, "live.mp4")
	failure := ""
	if err := s.EncodeLive(ctx, moviePath, preview, false); err != nil {
		failure = "processing_failed"
		var b *photo.Biz
		if errors.As(err, &b) {
			failure = fmt.Sprint(b.Value)
		}
	} else if fi, err := os.Stat(preview); err != nil || fi.Size() > s.Cfg.MaxContentLength {
		failure = "file_too_large"
	}
	prep, err := s.Prepare(ctx, original, work)
	if err != nil {
		return true, err
	}
	return true, s.Atomic(ctx, uid, func(tx pgx.Tx, u photo.UserRow, f *photo.Files) error {
		b, err := s.batch(ctx, tx, uid, it.BatchID)
		if err != nil || b.Cancelled {
			return err
		}
		for _, i := range []*itemRow{cover, movie} {
			cur, err := s.item(ctx, tx, i.ID)
			if err != nil || cur == nil || !(cur.State == "ready" || cur.State == "waiting" || cur.State == "processing") {
				return err
			}
		}
		ns, err := s.folder(ctx, tx, b, cover.Path)
		if err != nil {
			return err
		}
		in, err := os.Open(original)
		if err != nil {
			return err
		}
		defer in.Close()
		p, err := s.install(ctx, tx, f, u, cover, ns, in, prep)
		if err != nil {
			return err
		}
		put := func(src, role string) error {
			dest, err := s.AssetPath(u.UUID, p.UUID, role)
			if err != nil {
				return err
			}
			r, err := os.Open(src)
			if err != nil {
				return err
			}
			defer r.Close()
			_, err = f.Install(r, dest)
			return err
		}
		if err = put(moviePath, "live_original"); err != nil {
			return err
		}
		status, errv := "ready", any(nil)
		if failure != "" {
			status, errv = "failed", failure
		} else if err = put(preview, "live_preview"); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE photo_media SET live=true, live_status=$2, live_error=$3 WHERE photo_id=$1`, p.ID, status, errv); err != nil {
			return err
		}
		t := now()
		if _, err = tx.Exec(ctx, `UPDATE upload_item SET pair_id=$2 WHERE id=$1`, cover.ID, movie.ID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE upload_item SET pair_id=$2, photo_id=$3, ns_id=$4, state='done', error=NULL, updated_at=$5 WHERE id=$1`, movie.ID, cover.ID, p.ID, ns, t); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE upload_batch SET updated_at=$2 WHERE id=$1`, b.ID, t)
		return err
	})
}

// cleanup is UploadService.cleanup.
func (s *Service) cleanup(ctx context.Context) error {
	cutoff := now().Add(-time.Duration(s.Cfg.UploadTempHours) * time.Hour)
	history := now().Add(-time.Duration(s.Cfg.UploadHistoryDays) * 24 * time.Hour)
	rows, err := s.Pool.Query(ctx, `SELECT id, user_id FROM upload_batch WHERE updated_at < $1`, cutoff)
	if err != nil {
		return err
	}
	type pair struct {
		id  string
		uid int64
	}
	var ids []pair
	for rows.Next() {
		var p pair
		if err := rows.Scan(&p.id, &p.uid); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, p)
	}
	rows.Close()
	for _, p := range ids {
		err := s.Atomic(ctx, p.uid, func(tx pgx.Tx, u photo.UserRow, f *photo.Files) error {
			var updated time.Time
			if err := tx.QueryRow(ctx, `SELECT updated_at FROM upload_batch WHERE id=$1`, p.id).Scan(&updated); err != nil {
				if err == pgx.ErrNoRows {
					return nil
				}
				return err
			}
			if !updated.Before(cutoff) {
				return nil
			}
			var active int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM upload_item WHERE batch_id=$1 AND state IN ('ready','processing')`, p.id).Scan(&active); err != nil || active > 0 {
				return err
			}
			t := now()
			if _, err := tx.Exec(ctx, `UPDATE upload_item SET state='failed', error='transmission_expired', updated_at=$2 WHERE batch_id=$1 AND state='pending'`, p.id, t); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE upload_batch SET sealed=true WHERE id=$1`, p.id); err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, `UPDATE upload_item SET state='ready', updated_at=$2 WHERE batch_id=$1 AND state='waiting'`, p.id, t)
			if err != nil {
				return err
			}
			if tag.RowsAffected() > 0 {
				_, err = tx.Exec(ctx, `UPDATE upload_batch SET updated_at=$2 WHERE id=$1`, p.id, t)
				return err
			}
			keys, err := tx.Query(ctx, `SELECT id FROM upload_item WHERE batch_id=$1 AND source_id IS NULL`, p.id)
			if err != nil {
				return err
			}
			var del []string
			for keys.Next() {
				var k string
				if err := keys.Scan(&k); err != nil {
					keys.Close()
					return err
				}
				del = append(del, k)
			}
			keys.Close()
			for _, k := range del {
				if sp, err := s.source(k); err == nil {
					if _, err := f.Remove(sp); err != nil {
						return err
					}
				}
			}
			if updated.Before(history) {
				if _, err := tx.Exec(ctx, `DELETE FROM upload_item WHERE batch_id=$1`, p.id); err != nil {
					return err
				}
				_, err = tx.Exec(ctx, `DELETE FROM upload_batch WHERE id=$1`, p.id)
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	dir := filepath.Join(s.Root, ".uploads")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		fi, err := e.Info()
		if err != nil || !fi.Mode().IsRegular() || fi.ModTime().After(cutoff) {
			continue
		}
		if strings.HasSuffix(e.Name(), ".part") {
			os.Remove(filepath.Join(dir, e.Name()))
			continue
		}
		var one int
		if err := s.Pool.QueryRow(ctx, `SELECT 1 FROM upload_item WHERE id=$1`, e.Name()).Scan(&one); err == pgx.ErrNoRows {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	return nil
}

// tick processes up to 100 items; it reports whether any work was found.
func (s *Service) tick(ctx context.Context) bool {
	rows, err := s.Pool.Query(ctx, `SELECT i.id, i.batch_id FROM upload_item i WHERE i.state IN ('ready','processing','waiting')
		ORDER BY (i.state='waiting'), i.updated_at, i.id LIMIT 100`)
	if err != nil {
		log.Printf("uploads: worker query: %v", err)
		return false
	}
	type pair struct{ id, batch string }
	var work []pair
	for rows.Next() {
		var p pair
		if rows.Scan(&p.id, &p.batch) == nil {
			work = append(work, p)
		}
	}
	rows.Close()
	for _, w := range work {
		if ctx.Err() != nil {
			return true
		}
		s.locked(ctx, w.batch, func() { s.process(ctx, w.id) })
	}
	return len(work) > 0
}

// locked runs fn under the per-batch session advisory lock, skipping when another worker holds it.
func (s *Service) locked(ctx context.Context, batch string, fn func()) {
	c, err := s.Pool.Acquire(ctx)
	if err != nil {
		return
	}
	defer c.Release()
	var got bool
	if err := c.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1, $2))`, batch, sessionLock).Scan(&got); err != nil || !got {
		return
	}
	defer c.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1, $2))`, batch, sessionLock)
	fn()
}

// Worker is the upload worker goroutine (server.Background).
func Worker(ctx context.Context, a *app.App) {
	s := &Service{photo.For(a)}
	hb := filepath.Join(s.Root, ".upload-worker.json")
	lastClean := time.Time{}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		busy := s.tick(ctx)
		if time.Since(lastClean) >= cleanEvery {
			if err := s.cleanup(ctx); err != nil && ctx.Err() == nil {
				log.Printf("uploads: cleanup: %v", err)
			}
			lastClean = time.Now()
		}
		raw, _ := json.Marshal(map[string]any{"time": time.Now().Unix()})
		_ = os.WriteFile(hb, raw, 0o644)
		if busy {
			timer.Reset(100 * time.Millisecond)
		} else {
			timer.Reset(tickEvery)
		}
	}
}
