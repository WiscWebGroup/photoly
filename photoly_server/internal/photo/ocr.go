package photo

import (
	"context"
	"errors"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// app/ocr/__init__.py
const (
	ocrLanguages   = "chi_sim+chi_tra+eng+jpn"
	ocrMaxBytes    = 20 * 1024 * 1024
	ocrMaxPixels   = 40_000_000
	ocrMaxText     = 100_000
	ocrTotal       = 45 * time.Second
	ocrEngine      = 30 * time.Second
	ocrAdvisoryKey = 74831004
)

var ocrFormats = setOf("JPEG", "PNG", "WEBP", "GIF", "TIFF", "BMP", "ICO", "PPM")

func ocrFail(reason string, code int) error { return Err(map[string]any{"error": reason}, code) }

func (s *Service) ocrPhoto(ctx context.Context, q DB, uid int64, photoID any, lock bool, deadline *time.Time) (*Photo, error) {
	id, ok := intOf(photoID)
	if !ok || id < 1 {
		return nil, ocrFail("invalid_request", 400)
	}
	if lock {
		remaining := 5 * time.Second
		if deadline != nil {
			remaining = time.Until(*deadline)
		}
		if remaining <= 0 {
			return nil, ocrFail("ocr_timeout", 504)
		}
		if _, err := q.Exec(ctx, `SELECT set_config('lock_timeout', $1, true)`, itoaMS(remaining)); err != nil {
			return nil, err
		}
		var one int64
		if err := q.QueryRow(ctx, `SELECT user_id FROM "user" WHERE user_id=$1 FOR UPDATE`, uid).Scan(&one); err != nil && err != pgx.ErrNoRows {
			var pg *pgconn.PgError
			if errors.As(err, &pg) && pg.Code == "55P03" {
				if deadline != nil {
					return nil, ocrFail("ocr_timeout", 504)
				}
				return nil, ocrFail("ocr_busy", 429)
			}
			return nil, err
		}
	}
	p, err := scanPhoto(q.QueryRow(ctx, `SELECT `+photoCols+` FROM photo p WHERE p.photo_id=$1 AND p.user_id=$2`, id, uid))
	if err == pgx.ErrNoRows {
		return nil, ocrFail("photo_not_found", 404)
	} else if err != nil {
		return nil, err
	}
	if !ImageFormats[strings.ToLower(p.Format)] {
		return nil, ocrFail("unsupported_format", 400)
	}
	return p, nil
}

func itoaMS(d time.Duration) string {
	ms := d.Milliseconds()
	if ms < 1 {
		ms = 1
	}
	return strconv.FormatInt(ms, 10) + "ms"
}

type ocrRow struct {
	Version  int32
	Revision int32
	Text     string
	Engine   *string
	Updated  time.Time
}

func (s *Service) ocrRowOf(ctx context.Context, q DB, photoID int64) (*ocrRow, error) {
	var r ocrRow
	err := q.QueryRow(ctx, `SELECT content_version, revision, text, engine_version, updated_at FROM photo_ocr WHERE photo_id=$1`, photoID).
		Scan(&r.Version, &r.Revision, &r.Text, &r.Engine, &r.Updated)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &r, err
}

func ocrResponse(p *Photo, r *ocrRow) map[string]any {
	out := map[string]any{"photoId": p.ID, "contentVersion": p.ContentVersion, "revision": 0, "text": nil,
		"languages": ocrLanguages, "engineVersion": nil, "updatedAt": nil}
	if r != nil {
		out["revision"], out["text"], out["engineVersion"], out["updatedAt"] = r.Revision, r.Text, nilStr(r.Engine), IsoAware(r.Updated)
	}
	return out
}

// OCRGet is OCRService.get.
func (s *Service) OCRGet(ctx context.Context, uid, photoID int64) (any, error) {
	p, err := s.ocrPhoto(ctx, s.Pool, uid, photoID, false, nil)
	if err != nil {
		return nil, err
	}
	r, err := s.ocrRowOf(ctx, s.Pool, photoID)
	if err != nil {
		return nil, err
	}
	if r != nil && r.Version != p.ContentVersion {
		r = nil
	}
	return ocrResponse(p, r), nil
}

func ocrValidate(p *Photo, r *ocrRow, data map[string]any) error {
	v, ok1 := intOf(data["contentVersion"])
	rev, ok2 := intOf(data["revision"])
	if !ok1 || !ok2 {
		return ocrFail("invalid_request", 400)
	}
	cur := int64(0)
	if r != nil {
		cur = int64(r.Revision)
	}
	if int64(p.ContentVersion) != v || cur != rev {
		return ocrFail("ocr_conflict", 409)
	}
	return nil
}

type recognized struct{ Text, Engine string }

// OCRSave is OCRService.save (a manual edit when recognized is nil).
func (s *Service) OCRSave(ctx context.Context, uid int64, data map[string]any, rec *recognized, deadline *time.Time) (any, error) {
	var value string
	if rec != nil {
		value = rec.Text
	} else {
		v, ok := data["text"].(string)
		if !ok {
			return nil, ocrFail("invalid_text", 400)
		}
		value = v
	}
	if utf8.RuneCountInString(value) > ocrMaxText || strings.ContainsRune(value, 0) {
		return nil, ocrFail("invalid_text", 400)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	p, err := s.ocrPhoto(ctx, tx, uid, data["photoId"], true, deadline)
	if err != nil {
		return nil, err
	}
	r, err := s.ocrRowOf(ctx, tx, p.ID)
	if err != nil {
		return nil, err
	}
	if err = ocrValidate(p, r, data); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	var engine any
	if r != nil {
		engine = nilStr(r.Engine)
	}
	if rec != nil {
		engine = rec.Engine
	}
	rev := int32(1)
	if r != nil {
		rev = r.Revision + 1
	}
	if _, err = tx.Exec(ctx, `INSERT INTO photo_ocr (photo_id, content_version, text, languages, engine_version, revision, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (photo_id) DO UPDATE SET content_version=EXCLUDED.content_version, text=EXCLUDED.text,
		languages=EXCLUDED.languages, engine_version=EXCLUDED.engine_version, revision=EXCLUDED.revision, updated_at=EXCLUDED.updated_at`,
		p.ID, p.ContentVersion, value, ocrLanguages, engine, rev, now); err != nil {
		return nil, err
	}
	var eng *string
	if e, ok := engine.(string); ok {
		eng = &e
	}
	out := ocrResponse(p, &ocrRow{Revision: rev, Text: value, Engine: eng, Updated: now})
	return out, tx.Commit(ctx)
}

// OCRRecognize is OCRService.recognize: tesseract behind a database-wide single slot.
func (s *Service) OCRRecognize(ctx context.Context, uid int64, data map[string]any) (any, error) {
	deadline := time.Now().Add(ocrTotal)
	if _, err := s.ocrPhoto(ctx, s.Pool, uid, data["photoId"], false, nil); err != nil {
		return nil, err
	}
	conn, err := s.Pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	var got bool
	if err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, int64(ocrAdvisoryKey)).Scan(&got); err != nil || !got {
		conn.Release()
		if err != nil {
			return nil, err
		}
		return nil, ocrFail("ocr_busy", 429)
	}
	defer func() {
		var ok bool
		c := context.WithoutCancel(ctx)
		if conn.QueryRow(c, `SELECT pg_advisory_unlock($1)`, int64(ocrAdvisoryKey)).Scan(&ok) != nil || !ok {
			_ = conn.Hijack().Close(c) // never return a connection that may still hold the slot
			return
		}
		conn.Release()
	}()
	work, err := os.MkdirTemp("", "photoly-ocr-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)
	source := filepath.Join(work, "source")

	stream, err := func() (*os.File, error) {
		tx, err := s.Pool.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
		p, err := s.ocrPhoto(ctx, tx, uid, data["photoId"], true, &deadline)
		if err != nil {
			return nil, err
		}
		r, err := s.ocrRowOf(ctx, tx, p.ID)
		if err != nil {
			return nil, err
		}
		if err = ocrValidate(p, r, data); err != nil {
			return nil, err
		}
		uu, err := s.userUUID(ctx, tx, uid)
		if err != nil {
			return nil, err
		}
		path, err := s.MediaPath(uu, p.UUID, p.Format, false)
		if err != nil {
			return nil, err
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, ocrFail("image_missing", 404)
		}
		return f, nil
	}()
	if err != nil {
		return nil, err
	}
	out, err := os.Create(source)
	if err != nil {
		stream.Close()
		return nil, err
	}
	var count int64
	buf := make([]byte, 1<<20)
	for {
		n, rerr := stream.Read(buf)
		if n > 0 {
			count += int64(n)
			if count > ocrMaxBytes {
				err = ocrFail("image_too_large", 413)
				break
			}
			if time.Now().After(deadline) {
				err = ocrFail("ocr_timeout", 504)
				break
			}
			if _, err = out.Write(buf[:n]); err != nil {
				break
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			err = rerr
			break
		}
	}
	stream.Close()
	out.Close()
	if err != nil {
		return nil, err
	}
	rec, err := s.runTesseract(ctx, source, work, deadline)
	if err != nil {
		return nil, err
	}
	if time.Now().After(deadline) {
		return nil, ocrFail("ocr_timeout", 504)
	}
	return s.OCRSave(ctx, uid, data, rec, &deadline)
}

// runTesseract is app.ocr.worker + run_worker: prepare the image, run tesseract, bound the text.
func (s *Service) runTesseract(ctx context.Context, source, dir string, deadline time.Time) (*recognized, error) {
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return nil, ocrFail("ocr_timeout", 504)
	}
	ctx, cancel := context.WithTimeout(ctx, remaining)
	defer cancel()
	env := append(os.Environ(), "OMP_THREAD_LIMIT=1")
	run := func(timeout time.Duration, args ...string) ([]byte, error) {
		c, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		cmd := exec.CommandContext(c, "tesseract", args...)
		cmd.Env = env
		out, err := cmd.Output()
		if c.Err() != nil {
			return nil, context.DeadlineExceeded
		}
		return out, err
	}
	tmo := func(err error) error {
		if errors.Is(err, context.DeadlineExceeded) {
			return ocrFail("ocr_timeout", 504)
		}
		if errors.Is(err, exec.ErrNotFound) {
			return ocrFail("engine_unavailable", 503)
		}
		return nil
	}
	ver, err := run(3*time.Second, "--version")
	if err != nil {
		if e := tmo(err); e != nil {
			return nil, e
		}
		return nil, ocrFail("recognition_failed", 400)
	}
	langs, err := run(3*time.Second, "--list-langs")
	if err != nil {
		if e := tmo(err); e != nil {
			return nil, e
		}
		return nil, ocrFail("recognition_failed", 400)
	}
	have := map[string]bool{}
	for _, l := range strings.Split(string(langs), "\n") {
		have[strings.TrimSpace(l)] = true
	}
	for _, l := range strings.Split(ocrLanguages, "+") {
		if !have[l] {
			return nil, ocrFail("languages_unavailable", 503)
		}
	}
	prepared := filepath.Join(dir, "prepared.png")
	if err := s.ocrPrepare(ctx, source, prepared); err != nil {
		return nil, err
	}
	outBase := filepath.Join(dir, "recognized")
	if _, err = run(ocrEngine, prepared, outBase, "-l", ocrLanguages, "--oem", "1", "--psm", "3"); err != nil {
		if e := tmo(err); e != nil {
			return nil, e
		}
		return nil, ocrFail("recognition_failed", 400)
	}
	raw, err := os.ReadFile(outBase + ".txt")
	if err != nil {
		return nil, ocrFail("invalid_image", 400)
	}
	text := strings.ReplaceAll(strings.ToValidUTF8(string(raw), "�"), "\r\n", "\n")
	if utf8.RuneCountInString(text) > ocrMaxText+1 {
		text = string([]rune(text)[:ocrMaxText+1])
	}
	text = strings.TrimRight(text, "\n\f")
	if utf8.RuneCountInString(text) > ocrMaxText {
		return nil, ocrFail("text_too_long", 400)
	}
	if strings.ContainsRune(text, 0) {
		return nil, ocrFail("recognition_failed", 400)
	}
	version := strings.TrimSpace(strings.SplitN(string(ver), "\n", 2)[0])
	return &recognized{Text: text, Engine: version}, nil
}

// ocrPrepare is app.ocr.worker.prepare: orient, fit within 4096px, flatten on white, write PNG.
func (s *Service) ocrPrepare(ctx context.Context, source, dest string) error {
	format := sniff(source)
	if !ocrFormats[format] {
		return ocrFail("unsupported_format", 400)
	}
	if w, h, ok := imageDims(source); ok && w*h > ocrMaxPixels {
		return ocrFail("image_too_large", 400)
	}
	d, err := decodeImage(ctx, source, ocrEngine)
	if err != nil {
		return ocrFail("invalid_image", 400)
	}
	img := d.img
	if !d.oriented {
		img = transpose(img, orientationOf(source))
	}
	img = flattenWhite(resizeFit(img, 4096))
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return ocrFail("invalid_image", 400)
	}
	return nil
}
