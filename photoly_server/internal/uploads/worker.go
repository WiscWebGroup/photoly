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
	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/photo"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
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
func (s *Service) folder(ctx context.Context, tx pgx.Tx, b *batchRow, p string, f *photo.Files) (int64, error) {
	var base int64 = -1
	if b.NsID != nil {
		base = *b.NsID
	}
	if err := s.Owned(ctx, tx, "namespace", base, b.UserID, "destination_deleted", 404); err != nil {
		return 0, err
	}
	q := dbgen.New(tx)
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
			taken, err := q.NamespaceNameTaken(ctx, dbgen.NamespaceNameTakenParams{UserID: b.UserID, ParentID: parent, NsName: cand})
			if err != nil {
				return 0, err
			}
			if !taken {
				break
			}
			tail := fmt.Sprintf(" (%d)", n)
			r := []rune(name)
			if len(r) > 255-len(tail) {
				r = r[:255-len(tail)]
			}
			cand, n = string(r)+tail, n+1
		}
		id, _, err := storage.CreateFolder(ctx, tx, f, s.Root, b.UserID, parent, cand)
		if err != nil {
			return 0, err
		}
		parent, b.Folders[key], dirty = id, id, true
	}
	if dirty {
		raw, _ := json.Marshal(b.Folders)
		if err := q.SetUploadBatchFolders(ctx, dbgen.SetUploadBatchFoldersParams{ID: b.ID, Folders: raw}); err != nil {
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
	err = dbgen.New(tx).MarkUploadItemDone(ctx, dbgen.MarkUploadItemDoneParams{ID: it.ID, PhotoID: p.ID, NsID: ns, UpdatedAt: now()})
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
	q := dbgen.New(tx)
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
		state, errc := "ready", (*string)(nil)
		if !media[suffix(p)] {
			state, errc = "skipped", ptr("unsupported_file_type")
		} else if int64(e.UncompressedSize64) > s.Cfg.MaxContentLength {
			state, errc = "failed", ptr("file_too_large")
		}
		old, err := s.item(ctx, tx, key)
		if err != nil {
			return err
		}
		if old == nil {
			err = q.InsertExpandedUploadItem(ctx, dbgen.InsertExpandedUploadItemParams{ID: key, BatchID: b.ID, Path: p, Size: int64(e.UncompressedSize64),
				State: state, Error: errc, SourceID: it.ID, EntryIndex: int64(idx), UpdatedAt: now()})
		} else if old.State == "pending" {
			err = q.SetUploadItemState(ctx, dbgen.SetUploadItemStateParams{ID: key, State: state, Error: errc, UpdatedAt: now()})
		}
		if err != nil {
			return err
		}
	}
	return q.MarkUploadItemExpanded(ctx, it.ID)
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
	uid, err := dbgen.New(s.Pool).GetUploadBatchOwner(ctx, it.BatchID)
	if err != nil {
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
		return dbgen.New(tx).StartUploadItem(ctx, dbgen.StartUploadItemParams{ID: id, UpdatedAt: now()})
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
			q := dbgen.New(tx)
			if err = q.SetUploadItemState(ctx, dbgen.SetUploadItemStateParams{ID: id, State: "failed", Error: &code, UpdatedAt: t}); err != nil {
				return err
			}
			if cur.PairID != nil {
				err = q.FailProcessingUploadItem(ctx, dbgen.FailProcessingUploadItemParams{ID: *cur.PairID, Error: code, UpdatedAt: t})
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
		q := dbgen.New(tx)
		if err = q.TouchUploadItem(ctx, dbgen.TouchUploadItemParams{ID: it.ID, UpdatedAt: now()}); err != nil {
			return err
		}
		return q.TouchUploadBatch(ctx, dbgen.TouchUploadBatchParams{ID: b.ID, UpdatedAt: now()})
	})
}

func (s *Service) siblings(ctx context.Context, batchID string) ([]*itemRow, error) {
	rows, err := dbgen.New(s.Pool).ListUploadSiblings(ctx, batchID)
	if err != nil {
		return nil, err
	}
	out := make([]*itemRow, len(rows))
	for i, r := range rows {
		out[i] = itemFromRow(r)
	}
	return out, nil
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
		return dbgen.New(tx).WaitUploadItem(ctx, dbgen.WaitUploadItemParams{ID: id, UpdatedAt: now()})
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
			return dbgen.New(tx).SetUploadItemMediaMetadata(ctx, dbgen.SetUploadItemMediaMetadataParams{ID: it.ID, MediaMetadata: raw})
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
		ns, err := s.folder(ctx, tx, b, cur.Path, f)
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
		return dbgen.New(tx).TouchUploadBatch(ctx, dbgen.TouchUploadBatchParams{ID: b.ID, UpdatedAt: now()})
	})
}

// pair handles an item carrying a ContentIdentifier. It reports whether the item was fully handled.
func (s *Service) pair(ctx context.Context, uid int64, it *itemRow, ident, work, original string) (bool, error) {
	batch, err := dbgen.New(s.Pool).GetUploadBatch(ctx, it.BatchID)
	if err != nil {
		return true, err
	}
	sealed := batch.Sealed
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
			if err = dbgen.New(tx).PairUploadItem(ctx, dbgen.PairUploadItemParams{ID: p[0], PairID: p[1], UpdatedAt: t}); err != nil {
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
		ns, err := s.folder(ctx, tx, b, cover.Path, f)
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
		// Preserve the incoming attachment name independently of its remote object key.
		liveName, err := storage.AvailableName(ctx, tx, s.Root, ns, path.Base(movie.Path), p.ID, 0)
		if err != nil {
			return err
		}
		_, source, err := storage.Folder(ctx, tx, s.Root, ns)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE media_asset SET display_name=$2,filename=CASE WHEN $3 THEN $2 ELSE filename END WHERE photo_id=$1 AND role='live_original'`, p.ID, liveName, source.Kind == "local"); err != nil {
			return err
		}
		put := func(src, role string) error {
			dest, err := s.AssetPath(ctx, tx, u.UUID, p.UUID, role)
			if err != nil {
				return err
			}
			r, err := os.Open(src)
			if err != nil {
				return err
			}
			defer r.Close()
			if role == "live_original" {
				_, err = f.InstallNew(r, dest)
			} else {
				_, err = f.Install(r, dest)
			}
			return err
		}
		if err = put(moviePath, "live_original"); err != nil {
			return err
		}
		status, errv := "ready", (*string)(nil)
		if failure != "" {
			status, errv = "failed", &failure
		} else if err = put(preview, "live_preview"); err != nil {
			return err
		}
		q := dbgen.New(tx)
		if err = q.MarkPhotoLive(ctx, dbgen.MarkPhotoLiveParams{PhotoID: p.ID, LiveStatus: status, LiveError: errv}); err != nil {
			return err
		}
		t := now()
		if err = q.SetUploadItemPair(ctx, dbgen.SetUploadItemPairParams{ID: cover.ID, PairID: movie.ID}); err != nil {
			return err
		}
		if err = q.MarkPairedUploadItemDone(ctx, dbgen.MarkPairedUploadItemDoneParams{ID: movie.ID, PairID: cover.ID, PhotoID: p.ID, NsID: ns, UpdatedAt: t}); err != nil {
			return err
		}
		return q.TouchUploadBatch(ctx, dbgen.TouchUploadBatchParams{ID: b.ID, UpdatedAt: t})
	})
}

// cleanup is UploadService.cleanup.
func (s *Service) cleanup(ctx context.Context) error {
	cutoff := now().Add(-time.Duration(s.Cfg.UploadTempHours) * time.Hour)
	history := now().Add(-time.Duration(s.Cfg.UploadHistoryDays) * 24 * time.Hour)
	ids, err := dbgen.New(s.Pool).ListStaleUploadBatches(ctx, cutoff)
	if err != nil {
		return err
	}
	for _, p := range ids {
		err := s.Atomic(ctx, p.UserID, func(tx pgx.Tx, u photo.UserRow, f *photo.Files) error {
			q := dbgen.New(tx)
			batch, err := q.GetUploadBatch(ctx, p.ID)
			if err == pgx.ErrNoRows {
				return nil
			} else if err != nil {
				return err
			}
			updated := batch.UpdatedAt
			if !updated.Before(cutoff) {
				return nil
			}
			active, err := q.CountInflightUploadItems(ctx, p.ID)
			if err != nil || active > 0 {
				return err
			}
			t := now()
			if err := q.ExpireUntransmittedUploadItems(ctx, dbgen.ExpireUntransmittedUploadItemsParams{BatchID: p.ID, UpdatedAt: t}); err != nil {
				return err
			}
			if err := q.SealUploadBatchQuietly(ctx, p.ID); err != nil {
				return err
			}
			resumed, err := q.ResumeWaitingUploadItems(ctx, dbgen.ResumeWaitingUploadItemsParams{BatchID: p.ID, UpdatedAt: t})
			if err != nil {
				return err
			}
			if resumed > 0 {
				return q.TouchUploadBatch(ctx, dbgen.TouchUploadBatchParams{ID: p.ID, UpdatedAt: t})
			}
			del, err := q.ListUploadSourceIDs(ctx, p.ID)
			if err != nil {
				return err
			}
			for _, k := range del {
				if sp, err := s.source(k); err == nil {
					if _, err := f.Remove(sp); err != nil {
						return err
					}
				}
			}
			if updated.Before(history) {
				if err := q.DeleteBatchUploadItems(ctx, p.ID); err != nil {
					return err
				}
				return q.DeleteUploadBatch(ctx, p.ID)
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
		if exists, err := dbgen.New(s.Pool).UploadItemExists(ctx, e.Name()); err == nil && !exists {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	return nil
}

// tick processes up to 100 items; it reports whether any work was found.
func (s *Service) tick(ctx context.Context) bool {
	work, err := dbgen.New(s.Pool).ListRunnableUploadItems(ctx)
	if err != nil {
		log.Printf("uploads: worker query: %v", err)
		return false
	}
	for _, w := range work {
		if ctx.Err() != nil {
			return true
		}
		s.locked(ctx, w.BatchID, func() { s.process(ctx, w.ID) })
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
	q := dbgen.New(c)
	arg := dbgen.TryBatchLockParams{Batch: batch, SessionLock: sessionLock}
	if got, err := q.TryBatchLock(ctx, arg); err != nil || !got {
		return
	}
	defer q.BatchUnlock(context.Background(), dbgen.BatchUnlockParams(arg))
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
