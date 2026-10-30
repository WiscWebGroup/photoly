// Package catalog ports controllers/{namespace,tag,gallery,photo_extra}.py.
package catalog

import (
	"context"
	"encoding/json"
	"math"
	"net/http"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	"github.com/jackc/pgx/v5"
)

type svc struct{ a *app.App }

// kind selects tag or gallery rows (services/catalog.py is generic over both).
type kind struct{ table, idArg string }

var (
	tagKind     = kind{"tag", "tagId"}
	galleryKind = kind{"gallery", "gaId"}
)

func (k kind) isGallery() bool { return k.table == "gallery" }

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
	q := dbgen.New(s.a.Pool)
	out := []map[string]any{}
	if k.isGallery() {
		rows, err := q.ListGalleries(ctx, dbgen.ListGalleriesParams{UserID: uid, RowLimit: limit, RowOffset: offset})
		for _, g := range rows {
			out = append(out, map[string]any{"gaId": g.GaID, "gaName": g.GaName, "userId": g.UserID, "createDate": legacy.ISO(g.CreateDate),
				"coverId": g.CoverID, "coverColor": g.CoverColor})
		}
		return out, err
	}
	rows, err := q.ListTags(ctx, dbgen.ListTagsParams{UserID: uid, RowLimit: limit, RowOffset: offset})
	for _, t := range rows {
		out = append(out, map[string]any{"tagId": t.TagID, "userId": t.UserID, "tagName": t.TagName})
	}
	return out, err
}

// decorate adds the opt-in "smart" provenance object (services.smart.decorate).
func (s *svc) decorate(ctx context.Context, k kind, rows []map[string]any) ([]map[string]any, error) {
	if len(rows) == 0 {
		return []map[string]any{}, nil
	}
	keys := make([]int64, len(rows))
	for i, r := range rows {
		keys[i] = r[k.idArg].(int64)
	}
	params := dbgen.ListSmartItemsForParams{TagIds: []int64{}, GaIds: []int64{}}
	if k.isGallery() {
		params.GaIds = keys
	} else {
		params.TagIds = keys
	}
	smart, err := dbgen.New(s.a.Pool).ListSmartItemsFor(ctx, params)
	if err != nil {
		return nil, err
	}
	items := map[int64]map[string]any{}
	for _, it := range smart {
		key := it.TagID
		if k.isGallery() {
			key = it.GaID
		}
		if key == nil {
			continue
		}
		n := it.Name
		if it.CustomName != nil && *it.CustomName != "" {
			n = *it.CustomName
		}
		var b any
		_ = json.Unmarshal(it.Basis, &b)
		items[*key] = map[string]any{"id": it.ItemID, "source": it.Source, "name": n, "suppressed": it.Suppressed, "basis": b}
	}
	for i, r := range rows {
		if it, ok := items[keys[i]]; ok {
			r["smart"] = it
		} else {
			r["smart"] = nil
		}
		rows[i] = r
	}
	return rows, nil
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
		count := dbgen.New(s.a.Pool).CountTags
		if k.isGallery() {
			count = dbgen.New(s.a.Pool).CountGalleries
		}
		total, err := count(ctx, uid)
		if err != nil {
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

func optInt(v any) (*int64, error) {
	if v == nil {
		return nil, nil
	}
	if n, ok := legacy.Int(v); ok && legacy.InInt32(n) {
		return &n, nil
	}
	return nil, legacy.Bad()
}

func optStr(v any) (*string, error) {
	if v == nil {
		return nil, nil
	}
	if s, ok := v.(string); ok && len([]rune(s)) <= 255 {
		return &s, nil
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
			q := dbgen.New(tx)
			if k.isGallery() {
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
				return q.InsertGallery(ctx, dbgen.InsertGalleryParams{UserID: uid, GaName: name, CreateDate: legacy.Now(), CoverID: cover, CoverColor: color})
			}
			name, err := legacy.ValidName(data["tagName"], false)
			if err != nil {
				return err
			}
			return q.InsertTag(ctx, dbgen.InsertTagParams{UserID: uid, TagName: name})
		})
		if err != nil {
			return nil, err
		}
		if k.isGallery() {
			return legacy.Res{V: true, Code: 200}, nil
		}
		return legacy.Changed(1, 1), nil
	}
}

// smartItem finds the smart_item bound to a tag or gallery.
func smartItem(ctx context.Context, tx pgx.Tx, k kind, key int64) (id, owner int64, found bool, err error) {
	p := dbgen.GetSmartItemForParams{}
	if k.isGallery() {
		p.GaID = &key
	} else {
		p.TagID = &key
	}
	row, err := dbgen.New(tx).GetSmartItemFor(ctx, p)
	if err == pgx.ErrNoRows {
		return 0, 0, false, nil
	}
	return row.ItemID, row.UserID, err == nil, err
}

// smartRename is services.smart.rename with the object's current (already updated) values.
func smartRename(ctx context.Context, tx pgx.Tx, k kind, item int64, name string, cover *int64, color *string) error {
	q := dbgen.New(tx)
	if k.isGallery() {
		cj, _ := json.Marshal(map[string]any{"cover_id": cover, "cover_color": color})
		return q.RenameSmartItemWithCover(ctx, dbgen.RenameSmartItemWithCoverParams{ItemID: item, CustomName: name, Cover: cj})
	}
	return q.RenameSmartItem(ctx, dbgen.RenameSmartItemParams{ItemID: item, CustomName: name})
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
			if err := legacy.Owned(ctx, tx, k.table, id, ok, uid, -1, 400); err != nil {
				return err
			}
			q := dbgen.New(tx)
			var name string
			var cover *int64
			var color *string
			var err error
			if k.isGallery() {
				if name, err = legacy.ValidName(data["gaName"], false); err != nil {
					return err
				}
				if color, err = optStr(data["coverColor"]); err != nil {
					return err
				}
				if cover, err = optInt(data["coverId"]); err != nil {
					return err
				}
				if err = q.UpdateGallery(ctx, dbgen.UpdateGalleryParams{GaID: id, GaName: name, CoverColor: color, CoverID: cover}); err != nil {
					return err
				}
			} else {
				if name, err = legacy.ValidName(data["tagName"], false); err != nil {
					return err
				}
				if err = q.UpdateTag(ctx, dbgen.UpdateTagParams{TagID: id, TagName: name}); err != nil {
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
			if err := legacy.Owned(ctx, tx, k.table, id, ok, uid, -1, 400); err != nil {
				return err
			}
			q := dbgen.New(tx)
			item, owner, found, err := smartItem(ctx, tx, k, id)
			if err != nil {
				return err
			}
			if found { // services.smart.dismiss: keep a suppressed, renamed record
				var name string
				var cover *int64
				var color *string
				if k.isGallery() {
					g, err := q.GetGallery(ctx, id)
					if err != nil {
						return err
					}
					name, cover, color = deref(g.GaName), g.CoverID, g.CoverColor
				} else if name, err = q.GetTagName(ctx, id); err != nil {
					return err
				}
				if err := smartRename(ctx, tx, k, item, name, cover, color); err != nil {
					return err
				}
				if err := q.SuppressSmartItem(ctx, item); err != nil {
					return err
				}
				if err := q.BumpSmartRevision(ctx, owner); err != nil {
					return err
				}
			}
			if k.isGallery() {
				return q.DeleteGallery(ctx, id)
			}
			return q.DeleteTag(ctx, id)
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
	if err := legacy.Owned(ctx, s.a.Pool, "gallery", id, ok, uid, -1, 400); err != nil {
		return nil, err
	}
	q := dbgen.New(s.a.Pool)
	owner, err := q.GetGalleryDownloadOwner(ctx, dbgen.GetGalleryDownloadOwnerParams{GaID: id, UserID: uid})
	if err != nil {
		return nil, err
	}
	photos, err := q.ListGalleryDownloadPhotos(ctx, dbgen.ListGalleryDownloadPhotosParams{GaID: id, UserID: uid})
	if err != nil {
		return nil, err
	}
	gname, uuid := owner.GaName, owner.Uuid
	name, err := legacy.ValidName(gname, false)
	if err != nil {
		return nil, err
	}
	var entries []legacy.Entry
	for _, p := range photos {
		pn, err := legacy.ValidName(p.PhotoName, false)
		if err != nil {
			return nil, err
		}
		path, err := legacy.MediaPath(r.Context(), s.a.Pool, s.a.Cfg.UploadFolder, uuid, p.PhotoUuid+"."+p.Format)
		if err != nil {
			return nil, err
		}
		entries = append(entries, legacy.Entry{Path: path, Name: name + "/" + pn + "." + p.Format, ID: p.PhotoID})
	}
	legacy.ServeArchive(w, r, entries)
	return legacy.Done{}, nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
