// Package catalog ports controllers/{namespace,tag,gallery,photo_extra}.py.
package catalog

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	"github.com/jackc/pgx/v5"
)

type svc struct{ a *app.App }

// kind describes tag or gallery rows (services/catalog.py is generic over both).
type kind struct {
	table, pk, cols, idArg string
	scan                   func(pgx.Row) (map[string]any, error)
	smartCol               string
}

var tagKind = kind{"tag", "tag_id", "tag_id, user_id, tag_name", "tagId", func(r pgx.Row) (map[string]any, error) {
	var id, uid int64
	var name *string
	if err := r.Scan(&id, &uid, &name); err != nil {
		return nil, err
	}
	return map[string]any{"tagId": id, "userId": uid, "tagName": name}, nil
}, "tag_id"}

var galleryKind = kind{"gallery", "ga_id", "ga_id, ga_name, user_id, create_date, cover_id, cover_color", "gaId", func(r pgx.Row) (map[string]any, error) {
	var id int64
	var name, color *string
	var uid, cover *int64
	var created *time.Time
	if err := r.Scan(&id, &name, &uid, &created, &cover, &color); err != nil {
		return nil, err
	}
	return map[string]any{"gaId": id, "gaName": name, "userId": uid, "createDate": legacy.ISO(created), "coverId": cover, "coverColor": color}, nil
}, "ga_id"}

func Register(a *app.App) {
	_ = legacy.UseDefaultSessions(context.Background(), a.Pool)
	s := &svc{a}
	m := a.Mux
	std := legacy.Opt{Auth: "header"}
	m1 := legacy.Opt{Auth: "header", Missing: -1}
	page := legacy.Opt{Auth: "header", Shape: "page", PageMissing: -1}
	bin := legacy.Opt{Auth: "path", Shape: "binary"}

	m.HandleFunc("GET /tag/getAll", std.Wrap(a, s.all(tagKind)))
	m.HandleFunc("GET /tag/getPage", page.Wrap(a, s.paged(tagKind)))
	m.HandleFunc("POST /tag/insert", m1.Wrap(a, s.insert(tagKind)))
	m.HandleFunc("POST /tag/update", std.Wrap(a, s.update(tagKind)))
	m.HandleFunc("POST /tag/delete", std.Wrap(a, s.delete(tagKind)))

	m.HandleFunc("POST /gallery/insert", legacy.Opt{Auth: "header", Missing: false, Failure: false, HasFailure: true}.Wrap(a, s.insert(galleryKind)))
	m.HandleFunc("GET /gallery/getAll", std.Wrap(a, s.all(galleryKind)))
	m.HandleFunc("GET /gallery/getPage", page.Wrap(a, s.paged(galleryKind)))
	m.HandleFunc("POST /gallery/delete", m1.Wrap(a, s.delete(galleryKind)))
	m.HandleFunc("POST /gallery/update", m1.Wrap(a, s.update(galleryKind)))
	m.HandleFunc("GET /gallery/downloadGa/{token}", bin.Wrap(a, s.downloadGallery))

	m.HandleFunc("GET /namespace/getRoot", std.Wrap(a, s.nsRoot))
	m.HandleFunc("GET /namespace/getChildren", std.Wrap(a, s.nsChildren))
	m.HandleFunc("GET /namespace/getDetails", std.Wrap(a, s.nsDetails))
	m.HandleFunc("GET /namespace/getNameById", std.Wrap(a, s.nsName))
	m.HandleFunc("POST /namespace/updateName", std.Wrap(a, s.nsRename))
	m.HandleFunc("POST /namespace/updateParent", std.Wrap(a, s.nsMove))
	m.HandleFunc("POST /namespace/insert", m1.Wrap(a, s.nsInsert))
	m.HandleFunc("POST /namespace/delete", std.Wrap(a, s.nsDelete))
	m.HandleFunc("GET /namespace/trace", std.Wrap(a, s.nsTrace))
	m.HandleFunc("GET /namespace/downloadNs/{token}", bin.Wrap(a, s.nsDownload))

	m.HandleFunc("GET /photo-extra/getGlobalMapPhoto", std.Wrap(a, s.mapPhotos(false)))
	m.HandleFunc("GET /photo-extra/getMapPlaces", std.Wrap(a, s.mapPlaces))
	m.HandleFunc("GET /photo-extra/getGlobalMapPhotoByGallery", std.Wrap(a, s.mapPhotos(true)))
}

func includeSmart(r *http.Request) bool { return r.URL.Query().Get("includeSmart") == "true" }

func (s *svc) list(ctx context.Context, k kind, uid int64, limit, offset int64) ([]map[string]any, error) {
	return legacy.Rows(ctx, s.a.Pool, k.scan, `SELECT `+k.cols+` FROM `+k.table+` WHERE user_id=$1 ORDER BY `+k.pk+` LIMIT $2 OFFSET $3`, uid, limit, offset)
}

// decorate adds the opt-in "smart" provenance object (services.smart.decorate).
func (s *svc) decorate(ctx context.Context, k kind, rows []map[string]any) ([]map[string]any, error) {
	if len(rows) == 0 {
		return []map[string]any{}, nil
	}
	idKey := k.idArg
	keys := make([]int64, len(rows))
	for i, r := range rows {
		keys[i] = r[idKey].(int64)
	}
	q, err := s.a.Pool.Query(ctx, `SELECT `+k.smartCol+`, item_id, source, name, custom_name, suppressed, basis FROM smart_item WHERE `+k.smartCol+` = ANY($1)`, keys)
	if err != nil {
		return nil, err
	}
	defer q.Close()
	items := map[int64]map[string]any{}
	for q.Next() {
		var key, id int64
		var source, name string
		var custom *string
		var sup bool
		var basis []byte
		if err := q.Scan(&key, &id, &source, &name, &custom, &sup, &basis); err != nil {
			return nil, err
		}
		n := name
		if custom != nil && *custom != "" {
			n = *custom
		}
		var b any
		_ = json.Unmarshal(basis, &b)
		items[key] = map[string]any{"id": id, "source": source, "name": n, "suppressed": sup, "basis": b}
	}
	for i, r := range rows {
		if it, ok := items[keys[i]]; ok {
			r["smart"] = it
		} else {
			r["smart"] = nil
		}
		rows[i] = r
	}
	return rows, q.Err()
}

func (s *svc) all(k kind) legacy.Handler {
	return func(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
		rows, err := s.list(r.Context(), k, uid, math.MaxInt64, 0)
		if err == nil && includeSmart(r) {
			rows, err = s.decorate(r.Context(), k, rows)
		}
		return legacy.Res{V: rows, Code: 200}, err
	}
}

func (s *svc) paged(k kind) legacy.Handler {
	return func(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
		page, _, err := legacy.ArgInt(r, "page", true)
		if err != nil {
			return nil, err
		}
		rows, _, err := legacy.ArgInt(r, "rowsPerPage", true)
		if err != nil || page < 1 || rows < 1 {
			return nil, legacy.Bad()
		}
		ctx := r.Context()
		var total int64
		if err := s.a.Pool.QueryRow(ctx, `SELECT count(*) FROM `+k.table+` WHERE user_id=$1`, uid).Scan(&total); err != nil {
			return nil, err
		}
		var out []map[string]any
		if page-1 > math.MaxInt64/rows {
			out = []map[string]any{}
		} else if out, err = s.list(ctx, k, uid, rows, (page-1)*rows); err != nil {
			return nil, err
		}
		if includeSmart(r) {
			if out, err = s.decorate(ctx, k, out); err != nil {
				return nil, err
			}
		}
		return legacy.PageRes{V: out, Pages: int((total + rows - 1) / rows), Code: 200}, nil
	}
}

func optInt(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	if n, ok := legacy.Int(v); ok && legacy.InInt32(n) {
		return n, nil
	}
	return nil, legacy.Bad()
}

func optStr(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	if s, ok := v.(string); ok && len([]rune(s)) <= 255 {
		return s, nil
	}
	return nil, legacy.Bad()
}

func (s *svc) insert(k kind) legacy.Handler {
	return func(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
		data, err := legacy.Body(r)
		if err != nil {
			return nil, err
		}
		ctx := r.Context()
		err = legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uid, false, func(tx pgx.Tx, _ *legacy.Files, _ *legacy.LockedUser) error {
			if k.table == "gallery" {
				name, err := legacy.ValidName(data["gaName"], false)
				if err != nil {
					return err
				}
				cover, err := optInt(data["coverId"])
				if err != nil {
					return err
				}
				color, err := optStr(data["coverColor"])
				if err != nil {
					return err
				}
				_, err = tx.Exec(ctx, `INSERT INTO gallery(user_id,ga_name,create_date,cover_id,cover_color) VALUES ($1,$2,$3,$4,$5)`, uid, name, legacy.Now(), cover, color)
				return err
			}
			name, err := legacy.ValidName(data["tagName"], false)
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `INSERT INTO tag(user_id,tag_name) VALUES ($1,$2)`, uid, name)
			return err
		})
		if err != nil {
			return nil, err
		}
		if k.table == "gallery" {
			return legacy.Res{V: true, Code: 200}, nil
		}
		return legacy.Changed(1, 1), nil
	}
}

// smartItem finds the smart_item bound to a tag or gallery.
func smartItem(ctx context.Context, tx pgx.Tx, k kind, key int64) (id, owner int64, found bool, err error) {
	err = tx.QueryRow(ctx, `SELECT item_id, user_id FROM smart_item WHERE `+k.smartCol+`=$1`, key).Scan(&id, &owner)
	if err == pgx.ErrNoRows {
		return 0, 0, false, nil
	}
	return id, owner, err == nil, err
}

// smartRename is services.smart.rename with the object's current (already updated) values.
func smartRename(ctx context.Context, tx pgx.Tx, k kind, item int64, name string, cover, color any) error {
	if k.table == "gallery" {
		cj, _ := json.Marshal(map[string]any{"cover_id": cover, "cover_color": color})
		_, err := tx.Exec(ctx, `UPDATE smart_item SET custom_name=$2, cover=$3::jsonb WHERE item_id=$1`, item, name, string(cj))
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE smart_item SET custom_name=$2 WHERE item_id=$1`, item, name)
	return err
}

func (s *svc) update(k kind) legacy.Handler {
	return func(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
		data, err := legacy.Body(r)
		if err != nil {
			return nil, err
		}
		ctx := r.Context()
		err = legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uid, false, func(tx pgx.Tx, _ *legacy.Files, _ *legacy.LockedUser) error {
			id, ok := legacy.Int(data[k.idArg])
			if err := legacy.Owned(ctx, tx, k.table, k.pk, id, ok, uid, -1, 400); err != nil {
				return err
			}
			var name string
			var cover, color any
			var err error
			if k.table == "gallery" {
				if name, err = legacy.ValidName(data["gaName"], false); err != nil {
					return err
				}
				color, cover = data["coverColor"], data["coverId"]
				if color, err = optStr(color); err != nil {
					return err
				}
				if cover, err = optInt(cover); err != nil {
					return err
				}
				if _, err = tx.Exec(ctx, `UPDATE gallery SET ga_name=$2, cover_color=$3, cover_id=$4 WHERE ga_id=$1`, id, name, color, cover); err != nil {
					return err
				}
			} else {
				if name, err = legacy.ValidName(data["tagName"], false); err != nil {
					return err
				}
				if _, err = tx.Exec(ctx, `UPDATE tag SET tag_name=$2 WHERE tag_id=$1`, id, name); err != nil {
					return err
				}
			}
			item, _, found, err := smartItem(ctx, tx, k, id)
			if err != nil || !found {
				return err
			}
			return smartRename(ctx, tx, k, item, name, cover, color)
		})
		if err != nil {
			return nil, err
		}
		return legacy.Changed(1, 1), nil
	}
}

func (s *svc) delete(k kind) legacy.Handler {
	return func(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
		id, ok, err := legacy.ArgInt(r, k.idArg, false)
		if err != nil {
			return nil, err
		}
		ctx := r.Context()
		err = legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uid, false, func(tx pgx.Tx, _ *legacy.Files, _ *legacy.LockedUser) error {
			if err := legacy.Owned(ctx, tx, k.table, k.pk, id, ok, uid, -1, 400); err != nil {
				return err
			}
			item, owner, found, err := smartItem(ctx, tx, k, id)
			if err != nil {
				return err
			}
			if found { // services.smart.dismiss: keep a suppressed, renamed record
				var name string
				var cover, color any
				if k.table == "gallery" {
					var cv *int64
					var cl *string
					if err := tx.QueryRow(ctx, `SELECT ga_name, cover_id, cover_color FROM gallery WHERE ga_id=$1`, id).Scan(&name, &cv, &cl); err != nil {
						return err
					}
					cover, color = cv, cl
				} else if err := tx.QueryRow(ctx, `SELECT tag_name FROM tag WHERE tag_id=$1`, id).Scan(&name); err != nil {
					return err
				}
				if err := smartRename(ctx, tx, k, item, name, cover, color); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `UPDATE smart_item SET suppressed=true, ga_id=NULL, tag_id=NULL WHERE item_id=$1`, item); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `UPDATE smart_preference SET revision=revision+1 WHERE user_id=$1`, owner); err != nil {
					return err
				}
			}
			_, err = tx.Exec(ctx, `DELETE FROM `+k.table+` WHERE `+k.pk+`=$1`, id)
			return err
		})
		if err != nil {
			return nil, err
		}
		return legacy.Changed(1, 1), nil
	}
}

func (s *svc) downloadGallery(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	ctx := r.Context()
	id, ok, err := legacy.ArgInt(r, "gaId", false)
	if err != nil {
		return nil, err
	}
	if err := legacy.Owned(ctx, s.a.Pool, "gallery", "ga_id", id, ok, uid, -1, 400); err != nil {
		return nil, err
	}
	var gname, uuid string
	if err := s.a.Pool.QueryRow(ctx, `SELECT g.ga_name, u.uuid FROM gallery g, "user" u WHERE g.ga_id=$1 AND u.user_id=$2`, id, uid).Scan(&gname, &uuid); err != nil {
		return nil, err
	}
	rows, err := s.a.Pool.Query(ctx, `SELECT p.photo_id, p.photo_uuid, p.photo_name, p.format FROM photo p
		JOIN gallery_photo gp ON gp.photo_id=p.photo_id WHERE gp.ga_id=$1 AND p.user_id=$2 ORDER BY p.photo_id`, id, uid)
	if err != nil {
		return nil, err
	}
	type ph struct {
		id                 int64
		uuid, name, format string
	}
	var photos []ph
	for rows.Next() {
		var p ph
		if err := rows.Scan(&p.id, &p.uuid, &p.name, &p.format); err != nil {
			rows.Close()
			return nil, err
		}
		photos = append(photos, p)
	}
	rows.Close()
	name, err := legacy.ValidName(gname, false)
	if err != nil {
		return nil, err
	}
	var entries []legacy.Entry
	for _, p := range photos {
		pn, err := legacy.ValidName(p.name, false)
		if err != nil {
			return nil, err
		}
		path, err := legacy.MediaPath(s.a.Cfg.UploadFolder, uuid, p.uuid+"."+p.format)
		if err != nil {
			return nil, err
		}
		entries = append(entries, legacy.Entry{Path: path, Name: name + "/" + pn + "." + p.format, ID: p.id})
	}
	legacy.ServeArchive(w, r, entries)
	return legacy.Done{}, nil
}
