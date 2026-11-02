package photo

import (
	"context"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/jackc/pgx/v5"
)

var categoryNames = []string{"video", "panorama", "live", "animated", "screenshot", "portrait"}
var screenshotName = regexp.MustCompile(`(?i)^(screenshot|screen[ _-]?shot|screen[ _-]?capture|截屏|屏幕截图|螢幕截圖)([ _-]|\d|$)`)

func classifyMedia(name, format string, m Meta) []string {
	out := []string{}
	if VideoFormats[strings.ToLower(format)] || !ImageFormats[strings.ToLower(format)] {
		return out
	}
	screenshot := screenshotName.MatchString(name) || strings.EqualFold(m.String("UserComment"), "Screenshot") || strings.EqualFold(m.String("SpecialTypeID"), "screenshot")
	if screenshot {
		out = append(out, "screenshot")
	}
	w, _ := num(m, "ImageWidth")
	h, _ := num(m, "ImageHeight")
	capture, _ := num(m, "ImageCaptureType")
	if capture == 2 || strings.EqualFold(m.String("SpecialTypeID"), "portrait") {
		out = append(out, "portrait")
	}
	if !screenshot && (m.String("ProjectionType") != "" || m.String("UsePanoramaViewer") == "True" || (w > 0 && h > 0 && (w/h >= 3 || h/w >= 3))) {
		out = append(out, "panorama")
	}
	return out
}
func semanticCategory(v string) bool { return v == "panorama" || v == "screenshot" || v == "portrait" }
func validCategory(v string) bool {
	for _, c := range categoryNames {
		if c == v {
			return true
		}
	}
	return false
}
func (s *Service) registerCategories() {
	s.app.Mux.HandleFunc("GET /photo/categories", s.JSONHandler(func(r *http.Request, uid int64) (any, error) {
		rows, e := dbgen.New(s.Pool).ListCategoryCounts(r.Context(), uid)
		if e != nil {
			return nil, e
		}
		counts := map[string]int64{}
		for _, row := range rows {
			counts[row.Category] = row.Count
		}
		items := []map[string]any{}
		for _, k := range categoryNames {
			items = append(items, map[string]any{"category": k, "count": counts[k]})
		}
		var pending int64
		e = s.Pool.QueryRow(r.Context(), `SELECT count(*) FROM media_classification c JOIN photo p USING(photo_id) JOIN storage_source s ON s.id=p.source_id WHERE p.user_id=$1 AND s.enabled AND c.state<>'ready'`, uid).Scan(&pending)
		return map[string]any{"items": items, "pending": pending}, e
	}))
	s.app.Mux.HandleFunc("GET /photo/categories/{category}/photos", s.JSONHandler(func(r *http.Request, uid int64) (any, error) {
		category := r.PathValue("category")
		if !validCategory(category) {
			return nil, Bad()
		}
		after := int64(0)
		var e error
		if r.URL.Query().Has("after") {
			after, e = qint(r, "after")
			if e != nil || after < 0 {
				return nil, Bad()
			}
		}
		rows, e := s.Pool.Query(r.Context(), `SELECT `+photoCols+` FROM photo p JOIN effective_media_category c USING(photo_id) JOIN storage_source s ON s.id=p.source_id WHERE p.user_id=$1 AND c.category=$2 AND s.enabled AND ($3::bigint=0 OR p.photo_id<$3) ORDER BY p.photo_id DESC LIMIT 101`, uid, category, after)
		if e != nil {
			return nil, e
		}
		items, e := collect(rows)
		if e != nil {
			return nil, e
		}
		var next any
		if len(items) > 100 {
			items = items[:100]
			next = items[99]["photoId"]
		}
		return map[string]any{"items": items, "nextAfter": next}, nil
	}))
	s.app.Mux.HandleFunc("GET /photo/category", s.JSONHandler(func(r *http.Request, uid int64) (any, error) {
		id, e := qint(r, "photoId")
		if e != nil {
			return nil, e
		}
		if _, e = s.OwnedPhoto(r.Context(), s.Pool, id, uid, nil, 404); e != nil {
			return nil, e
		}
		rows, e := s.Pool.Query(r.Context(), `SELECT k.category,coalesce(k.category=ANY(a.categories),false),o.included FROM (VALUES ('panorama'),('screenshot'),('portrait')) k(category) LEFT JOIN media_classification a ON a.photo_id=$1 LEFT JOIN media_category_override o ON o.photo_id=$1 AND o.category=k.category`, id)
		if e != nil {
			return nil, e
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var k string
			var auto bool
			var override *bool
			if e = rows.Scan(&k, &auto, &override); e != nil {
				return nil, e
			}
			items = append(items, map[string]any{"category": k, "automatic": auto, "override": override})
		}
		return items, rows.Err()
	}))
	s.app.Mux.HandleFunc("PUT /photo/category", s.JSONHandler(func(r *http.Request, uid int64) (any, error) {
		d, e := body(r)
		if e != nil {
			return nil, e
		}
		id, ok := intOf(d["photoId"])
		if !ok {
			return nil, Bad()
		}
		k, _ := d["category"].(string)
		if !semanticCategory(k) {
			return nil, Bad()
		}
		if _, ok = d["included"]; !ok {
			return nil, Bad()
		}
		tx, e := s.Pool.Begin(r.Context())
		if e != nil {
			return nil, e
		}
		defer tx.Rollback(r.Context())
		var owner int64
		e = tx.QueryRow(r.Context(), `SELECT user_id FROM photo WHERE photo_id=$1 FOR UPDATE`, id).Scan(&owner)
		if e == pgx.ErrNoRows || e == nil && owner != uid {
			return nil, Err(nil, 404)
		}
		if e != nil {
			return nil, e
		}
		if d["included"] == nil {
			_, e = tx.Exec(r.Context(), `DELETE FROM media_category_override WHERE photo_id=$1 AND category=$2`, id, k)
		} else {
			v, ok := d["included"].(bool)
			if !ok {
				return nil, Bad()
			}
			_, e = tx.Exec(r.Context(), `INSERT INTO media_category_override(photo_id,category,included) VALUES($1,$2,$3) ON CONFLICT(photo_id,category) DO UPDATE SET included=excluded.included`, id, k, v)
		}
		if e != nil {
			return nil, e
		}
		return nil, tx.Commit(r.Context())
	}))
}

// A bounded, persistent queue independent of the AI scheduler. Leases survive restarts.
func CategoryWorker(ctx context.Context, a *app.App) {
	s := For(a)
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		for i := 0; i < 10; i++ {
			if !s.classifyNext(ctx) {
				break
			}
		}
	}
}
func (s *Service) classifyNext(ctx context.Context) bool {
	token := NewUUID()
	var id, version, uid int64
	var name, format string
	e := s.Pool.QueryRow(ctx, `WITH candidate AS (SELECT c.photo_id FROM media_classification c JOIN photo p USING(photo_id) JOIN storage_source s ON s.id=p.source_id WHERE c.state<>'ready' AND c.next_attempt<=now() AND s.enabled ORDER BY c.next_attempt,c.photo_id FOR UPDATE OF c SKIP LOCKED LIMIT 1) UPDATE media_classification c SET state='processing',lease_token=$1,next_attempt=now()+interval '2 minutes' FROM candidate k,photo p WHERE c.photo_id=k.photo_id AND p.photo_id=c.photo_id RETURNING c.photo_id,c.content_version,p.user_id,coalesce(p.photo_name,''),coalesce(p.format,'')`, token).Scan(&id, &version, &uid, &name, &format)
	if e == pgx.ErrNoRows {
		return false
	}
	if e != nil {
		log.Printf("category queue: %v", e)
		return false
	}
	work, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	m := Meta{}
	animated := false
	if ImageFormats[strings.ToLower(format)] {
		var path string
		path, e = s.PhotoPath(work, uid, id, false)
		if e == nil {
			m = s.ReadMetadata(work, path)
			animated = frameCount(path) > 1
		}
	}
	if e != nil {
		_, _ = s.Pool.Exec(ctx, `UPDATE media_classification SET state='pending',attempts=attempts+1,error='media_unavailable',next_attempt=now()+make_interval(secs=>LEAST(3600,30*power(2,LEAST(attempts,7)))::int),lease_token=NULL WHERE photo_id=$1 AND lease_token=$2`, id, token)
		return true
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		log.Printf("category result: %v", e)
		return true
	}
	defer tx.Rollback(ctx)
	var currentVersion int64
	e = tx.QueryRow(ctx, `SELECT content_version FROM photo WHERE photo_id=$1 FOR UPDATE`, id).Scan(&currentVersion)
	if e != nil || currentVersion != version {
		return true
	}
	tag, e := tx.Exec(ctx, `UPDATE media_classification SET categories=$3,state='ready',rule_version=1,error=NULL,lease_token=NULL WHERE photo_id=$1 AND lease_token=$2 AND content_version=$4`, id, token, classifyMedia(name, format, m), version)
	if e == nil && tag.RowsAffected() > 0 {
		_, e = tx.Exec(ctx, `INSERT INTO photo_media(photo_id,animated) VALUES($1,$2) ON CONFLICT(photo_id) DO UPDATE SET animated=excluded.animated`, id, animated)
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		log.Printf("category result: %v", e)
	}
	return true
}
