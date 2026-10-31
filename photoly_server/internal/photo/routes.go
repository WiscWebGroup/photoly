package photo

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	"github.com/jackc/pgx/v5"
)

var (
	shared   = map[*app.App]*Service{}
	sharedMu sync.Mutex
)

// For returns the process-wide Service of an App (photo and uploads share one).
func For(a *app.App) *Service {
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if s, ok := shared[a]; ok {
		return s
	}
	s := New(a)
	shared[a] = s
	return s
}

// body is controllers.common.body: a JSON object or BusinessError(-1).
func body(r *http.Request) (map[string]any, error) {
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" && !strings.HasSuffix(mt, "+json") {
		return nil, Bad()
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, Bad()
	}
	return decodeObject(raw)
}

func decodeObject(raw []byte) (map[string]any, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var v any
	if d.Decode(&v) != nil || d.More() {
		return nil, Bad()
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, Bad()
	}
	return m, nil
}

// intOf is `type(x) is int` for decoded JSON.
func intOf(v any) (int64, bool) {
	switch n := v.(type) {
	case json.Number:
		if strings.ContainsAny(string(n), ".eE") {
			return 0, false
		}
		i, err := n.Int64()
		return i, err == nil
	case float64:
		return toInt(n)
	case int64:
		return n, true
	case int:
		return int64(n), true
	}
	return 0, false
}

// multipartPhotos is controllers.common.multipart_photos.
func multipartPhotos(r *http.Request, multiple bool) ([]*multipart.FileHeader, []map[string]any, error) {
	name := "photo"
	if multiple {
		name = "photosStr"
	}
	str, ok := argStr(r, name)
	if !ok {
		return nil, nil, Bad()
	}
	d := json.NewDecoder(strings.NewReader(str))
	d.UseNumber()
	var v any
	if d.Decode(&v) != nil || d.More() {
		return nil, nil, Bad()
	}
	if multiple {
		list, ok := v.([]any)
		if !ok {
			return nil, nil, Bad()
		}
		out := make([]map[string]any, len(list))
		for i, x := range list {
			if out[i], ok = x.(map[string]any); !ok {
				return nil, nil, Bad() // a non-object description fails the same way later
			}
		}
		if r.MultipartForm == nil {
			return nil, out, nil
		}
		return r.MultipartForm.File["files"], out, nil
	}
	one, ok := v.(map[string]any)
	if !ok || r.MultipartForm == nil || len(r.MultipartForm.File["file"]) == 0 {
		return nil, nil, Bad()
	}
	return r.MultipartForm.File["file"][:1], []map[string]any{one}, nil
}

// Upload is PhotoService.upload; it returns the new photo UUIDs.
func (s *Service) Upload(ctx context.Context, uid int64, files []*multipart.FileHeader, descs []map[string]any) ([]string, error) {
	if len(files) != len(descs) || len(files) == 0 {
		return nil, Bad()
	}
	var out []string
	err := s.Atomic(ctx, uid, func(tx pgx.Tx, u UserRow, f *Files) error {
		out = out[:0]
		ns := make([]int64, len(descs))
		for i, d := range descs {
			id, ok := intOf(d["nsId"])
			if !ok {
				return Err(-2, 400)
			}
			if err := s.Owned(ctx, tx, "namespace", id, uid, -2, 400); err != nil {
				return err
			}
			ns[i] = id
		}
		for i, fh := range files {
			var vis any
			if n, ok := intOf(descs[i]["visibility"]); ok {
				vis = n
			}
			file, err := fh.Open()
			if err != nil {
				return err
			}
			p, err := s.InstallOne(ctx, tx, f, u, fh.Filename, file, ns[i], vis, nil)
			file.Close()
			if err != nil {
				return err
			}
			out = append(out, p.UUID)
		}
		return nil
	})
	return out, err
}

func firstFile(r *http.Request, name string) (*multipart.FileHeader, bool) {
	if r.MultipartForm != nil {
		if fs := r.MultipartForm.File[name]; len(fs) > 0 {
			return fs[0], true
		}
	}
	return nil, false
}

func qint(r *http.Request, name string) (int64, error) {
	n, _, err := argInt(r, name, false)
	return n, err
}

// Register mounts the /photo routes and wires the cred upload hook.
func Register(a *app.App) {
	user.DecodeFallback = DecodeBytes
	s := For(a)
	user.UploadPhotos = func(ctx context.Context, uid int64, files []*multipart.FileHeader, descs []map[string]any) ([]string, error) {
		out, err := s.Upload(ctx, uid, files, descs)
		if b, ok := BizOf(err); ok {
			return nil, legacy.Err(b.Value, b.Code)
		}
		return out, err
	}
	m := a.Mux
	j := s.JSONHandler
	handle := func(pattern string, h http.HandlerFunc) { m.HandleFunc(pattern, h) }

	changedJ := func(fn func(r *http.Request, uid int64) (int, error)) http.HandlerFunc {
		return j(func(r *http.Request, uid int64) (any, error) {
			n, err := fn(r, uid)
			if err != nil {
				return nil, err
			}
			return changed(n), nil
		})
	}
	insert := func(multiple bool) http.HandlerFunc {
		return j(func(r *http.Request, uid int64) (any, error) {
			files, descs, err := multipartPhotos(r, multiple)
			if err != nil {
				return nil, err
			}
			out, err := s.Upload(r.Context(), uid, files, descs)
			if err != nil {
				return nil, err
			}
			return changed(len(out)), nil
		})
	}
	handle("POST /photo/insert", insert(false))
	handle("POST /photo/inserts", insert(true))
	handle("POST /photo/delete", changedJ(func(r *http.Request, uid int64) (int, error) {
		id, err := qint(r, "photoId")
		if err != nil {
			return 0, err
		}
		return s.Delete(r.Context(), uid, id)
	}))
	handle("POST /photo/replace", j(func(r *http.Request, uid int64) (any, error) {
		_ = r.ParseMultipartForm(32 << 20)
		fh, ok := firstFile(r, "file")
		if !ok {
			return nil, Bad()
		}
		id, _, err := argInt(r, "photoId", true)
		if err != nil {
			return nil, err
		}
		f, err := fh.Open()
		if err != nil {
			return nil, err
		}
		defer f.Close()
		n, err := s.Replace(r.Context(), uid, id, fh.Filename, f)
		if err != nil {
			return nil, err
		}
		return changed(n), nil
	}))
	handle("POST /photo/update", j(func(r *http.Request, uid int64) (any, error) {
		d, err := body(r)
		if err != nil {
			return nil, err
		}
		if _, err = s.Update(r.Context(), uid, d); err != nil {
			return nil, err
		}
		id, _ := toInt(d["photoId"])
		p, err := s.OwnedPhoto(r.Context(), s.Pool, id, uid, nil, 400)
		if err != nil {
			return nil, err
		}
		return p.JSON(), nil
	}))
	handle("GET /photo/getDateSummary", j(func(r *http.Request, uid int64) (any, error) { return s.DateSummary(r.Context(), uid) }))
	handle("GET /photo/getByDate", j(func(r *http.Request, uid int64) (any, error) {
		q := r.URL.Query()
		order := "latest"
		if q.Has("order") {
			order = q.Get("order")
		}
		limit := int64(100)
		if q.Has("limit") {
			var err error
			if limit, err = qint(r, "limit"); err != nil {
				return nil, err
			}
		}
		var day *string
		if q.Has("day") {
			d := q.Get("day")
			day = &d
		}
		return s.ByDate(r.Context(), uid, order, limit, q.Get("cursor"), day)
	}))
	handle("GET /photo/getByNamespace", j(func(r *http.Request, uid int64) (any, error) {
		n, ok, err := argInt(r, "nsId", false)
		if err != nil {
			return nil, err
		}
		var ns *int64
		if ok {
			ns = &n
		}
		return s.ByNamespace(r.Context(), uid, ns)
	}))
	handle("GET /photo/getByTags", j(func(r *http.Request, uid int64) (any, error) {
		ids, err := argIDs(r, "tagIds")
		if err != nil {
			return nil, err
		}
		return s.ByTags(r.Context(), uid, ids)
	}))
	handle("GET /photo/getByGallery", j(func(r *http.Request, uid int64) (any, error) {
		id, err := qint(r, "gaId")
		if err != nil {
			return nil, err
		}
		return s.ByGallery(r.Context(), uid, id)
	}))
	handle("GET /photo/getCovers", j(func(r *http.Request, uid int64) (any, error) {
		src, ok := argStr(r, "source")
		ids, err := argIDs(r, "ids")
		if err != nil {
			return nil, err
		}
		return s.Covers(r.Context(), uid, src, ok, ids)
	}))
	handle("POST /photo/changeNamespace", j(func(r *http.Request, uid int64) (any, error) {
		id, err := qint(r, "photoId")
		if err != nil {
			return nil, err
		}
		ns, err := qint(r, "nsId")
		if err != nil {
			return nil, err
		}
		if _, err = s.Move(r.Context(), uid, id, ns); err != nil {
			return nil, err
		}
		p, err := s.OwnedPhoto(r.Context(), s.Pool, id, uid, nil, 400)
		if err != nil {
			return nil, err
		}
		return p.JSON(), nil
	}))
	handle("POST /photo/addTags", changedJ(func(r *http.Request, uid int64) (int, error) {
		p, err := qint(r, "photoId")
		if err != nil {
			return 0, err
		}
		ids, err := argIDs(r, "tagIds")
		if err != nil {
			return 0, err
		}
		return s.AddTags(r.Context(), uid, p, ids)
	}))
	handle("POST /photo/deleteTag", changedJ(func(r *http.Request, uid int64) (int, error) {
		p, err := qint(r, "photoId")
		if err != nil {
			return 0, err
		}
		t, err := qint(r, "tagId")
		if err != nil {
			return 0, err
		}
		return s.DeleteTags(r.Context(), uid, p, []int64{t})
	}))
	handle("POST /photo/deleteTags", changedJ(func(r *http.Request, uid int64) (int, error) {
		p, err := qint(r, "photoId")
		if err != nil {
			return 0, err
		}
		ids, err := argIDs(r, "tagIds")
		if err != nil {
			return 0, err
		}
		return s.DeleteTags(r.Context(), uid, p, ids)
	}))
	gallery := func(remove bool) http.HandlerFunc {
		return changedJ(func(r *http.Request, uid int64) (int, error) {
			p, err := qint(r, "photoId")
			if err != nil {
				return 0, err
			}
			g, err := qint(r, "gaId")
			if err != nil {
				return 0, err
			}
			return s.GalleryLink(r.Context(), uid, p, g, remove)
		})
	}
	handle("POST /photo/addToGallery", gallery(false))
	handle("POST /photo/deleteFromGallery", gallery(true))
	handle("GET /photo/getTagByPhoto", j(func(r *http.Request, uid int64) (any, error) {
		p, err := qint(r, "photoId")
		if err != nil {
			return nil, err
		}
		return s.Tags(r.Context(), uid, p, r.URL.Query().Get("includeSmart") == "true")
	}))
	handle("GET /photo/getGalleryByPhoto", j(func(r *http.Request, uid int64) (any, error) {
		p, err := qint(r, "photoId")
		if err != nil {
			return nil, err
		}
		return s.Galleries(r.Context(), uid, p, r.URL.Query().Get("includeSmart") == "true")
	}))

	// ---- binary routes ----
	bin := s.BinaryHandler
	view := func(thumb, video bool) func(w http.ResponseWriter, r *http.Request, uid int64) error {
		return func(w http.ResponseWriter, r *http.Request, uid int64) error {
			id, err := qint(r, "photoId")
			if err != nil {
				return err
			}
			var path string
			if video {
				path, err = s.PhotoPath(r.Context(), uid, id, false)
			} else {
				path, err = s.PreviewPath(r.Context(), uid, id, thumb)
			}
			if err != nil {
				return err
			}
			return serveFile(w, r, path, MIME(path), "")
		}
	}
	handle("GET /photo/render/{token}", bin("path", view(false, false)))
	handle("GET /photo/renderThumbnail/{token}", bin("path", view(true, false)))
	handle("GET /photo/renderV/{token}", bin("path", view(false, true)))
	sharedView := func(thumb bool) http.HandlerFunc {
		return bin("none", func(w http.ResponseWriter, r *http.Request, _ int64) error {
			path, hasPath := argStr(r, "path")
			token, hasToken := argStr(r, "token")
			p, err := s.SharedPath(r.Context(), path, token, hasPath, hasToken, thumb)
			if err != nil {
				return err
			}
			return serveFile(w, r, p, MIME(p), "")
		})
	}
	handle("GET /photo/renderToken", sharedView(false))
	handle("GET /photo/renderToken/Thumbnail", sharedView(true))
	handle("GET /photo/renderVToken", sharedView(false))
	handle("GET /photo/download/{token}", bin("path", func(w http.ResponseWriter, r *http.Request, uid int64) error {
		id, err := qint(r, "photoId")
		if err != nil {
			return err
		}
		p, err := s.PhotoPath(r.Context(), uid, id, false)
		if err != nil {
			return err
		}
		return serveFile(w, r, p, "application/octet-stream", filepath.Base(p))
	}))
	handle("GET /photo/getPath", j(func(r *http.Request, uid int64) (any, error) {
		id, err := qint(r, "photoId")
		if err != nil {
			return nil, err
		}
		return s.ShareURL(r.Context(), uid, id)
	}))
	qr := func(w http.ResponseWriter, r *http.Request, uid int64) error {
		id, err := qint(r, "photoId")
		if err != nil {
			return err
		}
		u, err := s.ShareURL(r.Context(), uid, id)
		if err != nil {
			return err
		}
		png, err := QRPNG(u)
		if err != nil {
			return err
		}
		// Original QRGen sends PNG bytes under image/jpeg; keep that legacy header.
		serveReader(w, bytes.NewReader(png), "image/jpeg")
		return nil
	}
	handle("GET /photo/getQR", bin("header", qr))
	handle("GET /photo/getQR2", bin("query", qr))
	handle("GET /photo/searchSmart", j(func(r *http.Request, uid int64) (any, error) {
		q, ok := argStr(r, "query")
		return s.SearchSmart(r.Context(), uid, q, ok)
	}))
	handle("GET /photo/searchPhoto", s.app.Auth(func(w http.ResponseWriter, r *http.Request) {
		opt, _ := argStr(r, "option")
		q, _ := argStr(r, "query")
		v, err := s.SearchPhoto(r.Context(), s.app.UserFrom(r.Context()).ID, opt, q)
		if err == nil {
			if list, ok := v.([]map[string]any); ok && len(list) == 0 {
				v = rc{list, 404}
			}
		}
		emit(w, v, err)
	}))
	handle("GET /photo/getPhotoExif", j(func(r *http.Request, uid int64) (any, error) {
		id, err := qint(r, "photoId")
		if err != nil {
			return nil, err
		}
		return s.ExifOf(r.Context(), uid, id)
	}))

	// ---- OCR ----
	noStore := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "private, no-store")
			h(w, r)
		}
	}
	handle("GET /photo/ocr", noStore(j(func(r *http.Request, uid int64) (any, error) {
		id, _, err := argInt(r, "photoId", true)
		if err != nil {
			return nil, err
		}
		return s.OCRGet(r.Context(), uid, id)
	})))
	handle("POST /photo/ocr/recognize", noStore(j(func(r *http.Request, uid int64) (any, error) {
		d, err := body(r)
		if err != nil {
			return nil, err
		}
		return s.OCRRecognize(r.Context(), uid, d)
	})))
	handle("PUT /photo/ocr", noStore(j(func(r *http.Request, uid int64) (any, error) {
		d, err := body(r)
		if err != nil {
			return nil, err
		}
		return s.OCRSave(r.Context(), uid, d, nil, nil)
	})))

	// ---- media tools ----
	handle("GET /photo/mediaCapabilities", j(func(r *http.Request, uid int64) (any, error) { return s.Capabilities(r.Context()), nil }))
	handle("GET /photo/mediaInfo", j(func(r *http.Request, uid int64) (any, error) {
		id, _, err := argInt(r, "photoId", true)
		if err != nil {
			return nil, err
		}
		return s.MediaInfo(r.Context(), uid, id)
	}))
	submit := func(kind string) http.HandlerFunc {
		return j(func(r *http.Request, uid int64) (any, error) {
			d, err := body(r)
			if err != nil {
				return nil, err
			}
			return s.Submit(r.Context(), uid, d, kind)
		})
	}
	handle("POST /photo/convert", submit("convert"))
	handle("POST /photo/onlineDownload", submit("online"))
	handle("POST /photo/retryLivePreview", submit("live_preview"))
	handle("GET /photo/renderLive/{token}", bin("path", func(w http.ResponseWriter, r *http.Request, uid int64) error {
		id, err := qint(r, "photoId")
		if err != nil {
			return err
		}
		p, err := s.LivePath(r.Context(), uid, id)
		if err != nil {
			return err
		}
		return serveFile(w, r, p, "video/mp4", "")
	}))
	handle("GET /photo/downloadLive/{token}", bin("path", func(w http.ResponseWriter, r *http.Request, uid int64) error {
		id, err := qint(r, "photoId")
		if err != nil {
			return err
		}
		entries, err := s.LiveEntries(r.Context(), uid, id)
		if err != nil {
			return err
		}
		legacy.ServeArchiveNamed(w, r, entries, "Live Photo.zip")
		return nil
	}))
	handle("GET /photo/getMonthPreview", j(func(r *http.Request, uid int64) (any, error) {
		month, ok := argStr(r, "month")
		order := "latest"
		if r.URL.Query().Has("order") {
			order = r.URL.Query().Get("order")
		}
		return s.MonthPreview(r.Context(), uid, month, ok, order)
	}))
}
