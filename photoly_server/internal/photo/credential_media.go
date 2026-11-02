package photo

import (
	"context"
	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/httpx"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	"net/http"
	"strconv"
	"strings"
)

func credError(w http.ResponseWriter, e error) {
	code, msg := 500, "internal_error"
	if b, ok := BizOf(e); ok {
		code = b.Code
		msg = "media_unavailable"
		if code == 404 {
			msg = "not_found"
		}
		if code == 400 {
			msg = "invalid_request"
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	httpx.JSON(w, msg, code)
}
func credDTO(p dbgen.ListCredentialMediaRow) map[string]any {
	return map[string]any{"photoId": p.PhotoID, "uuid": p.Uuid, "name": p.Name, "format": p.Format, "nsId": p.NsID, "contentVersion": p.ContentVersion, "animated": p.Animated, "live": p.Live, "categories": p.Categories}
}
func (s *Service) registerCredentialMedia() {
	a := s.app
	a.CredRender = func(w http.ResponseWriter, r *http.Request, uid, id int64) error {
		path, e := s.PhotoPath(r.Context(), uid, id, false)
		if e == nil {
			e = serveFile(w, r, path, MIME(path), "")
		}
		if b, ok := BizOf(e); ok {
			return legacy.Err(b.Value, b.Code)
		}
		return e
	}
	a.CredDelete = func(ctx context.Context, uid, id int64) error {
		_, e := s.Delete(ctx, uid, id)
		if b, ok := BizOf(e); ok {
			return legacy.Err(b.Value, b.Code)
		}
		return e
	}
	list := func(w http.ResponseWriter, r *http.Request, uid int64) {
		args := dbgen.ListCredentialMediaParams{UserID: uid, PageLimit: 101, Category: r.PathValue("category")}
		if args.Category != "" && !validCategory(args.Category) {
			credError(w, Err(nil, 404))
			return
		}
		for _, p := range []struct {
			key    string
			target *int64
		}{{"after", &args.AfterID}, {"nsId", &args.NsID}, {"limit", &args.PageLimit}} {
			if r.URL.Query().Has(p.key) {
				n, e := strconv.ParseInt(r.URL.Query().Get(p.key), 10, 32)
				if e != nil || n < 0 || (p.key == "limit" && (n < 1 || n > 200)) {
					credError(w, Bad())
					return
				}
				*p.target = n
				if p.key == "limit" {
					args.PageLimit++
				}
			}
		}
		if args.NsID > 0 {
			var owned bool
			e := s.Pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM namespace n JOIN storage_source s ON s.id=n.source_id WHERE n.ns_id=$1 AND n.user_id=$2 AND s.enabled)`, args.NsID, uid).Scan(&owned)
			if e != nil {
				credError(w, e)
				return
			}
			if !owned {
				credError(w, Err(nil, 404))
				return
			}
		}
		rows, e := dbgen.New(s.Pool).ListCredentialMedia(r.Context(), args)
		if e != nil {
			credError(w, e)
			return
		}
		var next any
		if int64(len(rows)) == args.PageLimit {
			rows = rows[:len(rows)-1]
			next = rows[len(rows)-1].PhotoID
		}
		items := []map[string]any{}
		for _, p := range rows {
			items = append(items, credDTO(p))
		}
		httpx.JSON(w, map[string]any{"items": items, "nextAfter": next}, 200)
	}
	a.Mux.HandleFunc("GET /cred/media", user.CredentialHandler(a, "R", list))
	a.Mux.HandleFunc("GET /cred/categories/{category}/media", user.CredentialHandler(a, "R", list))
	for _, thumb := range []bool{false, true} {
		path := "GET /cred/media/{uuid}"
		if thumb {
			path += "/thumbnail"
		}
		a.Mux.HandleFunc(path, user.CredentialHandler(a, "R", func(w http.ResponseWriter, r *http.Request, uid int64) {
			rows, e := dbgen.New(s.Pool).ListCredentialMedia(r.Context(), dbgen.ListCredentialMediaParams{UserID: uid, Uuid: r.PathValue("uuid"), PageLimit: 1})
			if e != nil {
				credError(w, e)
				return
			}
			if len(rows) == 0 {
				credError(w, Err(nil, 404))
				return
			}
			if !thumb {
				httpx.JSON(w, credDTO(rows[0]), 200)
				return
			}
			owner, e := s.userUUID(r.Context(), s.Pool, uid)
			if e != nil {
				credError(w, e)
				return
			}
			expected, e := s.MediaPath(r.Context(), s.Pool, owner, rows[0].Uuid, rows[0].Format, true)
			if e != nil {
				credError(w, mediaAccessError(e))
				return
			}
			if !isFile(expected) {
				credError(w, Err("thumbnail_unavailable", 404))
				return
			}
			p, e := s.PhotoPath(r.Context(), uid, rows[0].PhotoID, true)
			if e == nil {
				if !strings.HasPrefix(MIME(p), "image/") {
					e = Err("thumbnail_unavailable", 404)
				} else {
					e = serveFile(w, r, p, MIME(p), "")
				}
			}
			if e != nil {
				credError(w, e)
			}
		}))
	}
	a.Mux.HandleFunc("GET /cred/categories", user.CredentialHandler(a, "R", func(w http.ResponseWriter, r *http.Request, uid int64) {
		q := dbgen.New(s.Pool)
		counts, e := q.ListCategoryCounts(r.Context(), uid)
		if e != nil {
			credError(w, e)
			return
		}
		previews, e := q.ListCategoryPreviews(r.Context(), uid)
		if e != nil {
			credError(w, e)
			return
		}
		items := []map[string]any{}
		for _, k := range categoryNames {
			count := int64(0)
			for _, c := range counts {
				if c.Category == k {
					count = c.Count
				}
			}
			covers := []map[string]any{}
			for _, p := range previews {
				if p.Category == k {
					covers = append(covers, map[string]any{"photoId": p.PhotoID, "uuid": p.Uuid, "name": p.PhotoName, "format": p.Format, "contentVersion": p.ContentVersion})
				}
			}
			items = append(items, map[string]any{"category": k, "count": count, "previews": covers})
		}
		httpx.JSON(w, map[string]any{"items": items}, 200)
	}))
}
