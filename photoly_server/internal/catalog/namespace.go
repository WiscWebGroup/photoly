package catalog

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	"github.com/jackc/pgx/v5"
)

func (s *svc) nsRoot(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	ns, err := dbgen.New(s.a.Pool).GetRootNamespace(r.Context(), uid)
	if err == pgx.ErrNoRows {
		return legacy.Res{V: nil, Code: 200}, nil
	}
	return legacy.Res{V: legacy.NsJSON(ns), Code: 200}, err
}

func (s *svc) nsChildren(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	pid, ok, err := legacy.ArgInt(r, "parentId", false)
	if err != nil {
		return nil, err
	}
	var arg *int64
	if ok {
		arg = &pid
	}
	rows, err := dbgen.New(s.a.Pool).ListChildNamespaces(r.Context(), dbgen.ListChildNamespacesParams{UserID: uid, ParentID: arg})
	return legacy.Res{V: legacy.NsListJSON(rows), Code: 200}, err
}

func (s *svc) owned(ctx context.Context, db legacy.DB, id int64, ok bool, uid int64, failure any) error {
	return legacy.Owned(ctx, db, "namespace", id, ok, uid, failure, 400)
}

func (s *svc) nsName(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	id, ok, err := legacy.ArgInt(r, "nsId", false)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	if err := s.owned(ctx, s.a.Pool, id, ok, uid, -1); err != nil {
		return nil, err
	}
	name, err := dbgen.New(s.a.Pool).GetNamespaceName(ctx, id)
	return legacy.Res{V: name, Code: 200}, err
}

var (
	imageFormats = set("jpg", "jpeg", "gif", "png", "webp", "ico", "bmp", "tif", "tiff", "svg", "psd", "raw", "arw", "pcd", "jfif", "heic", "heif")
	videoFormats = set("mp4", "avi", "wmv", "mpeg", "m4v", "mov", "flv", "asf", "f4v", "rmvb", "vob", "rm")
	thumbFormats = set("jpg", "jpeg", "png", "gif", "jfif", "ico", "bmp", "tif", "tiff", "webp")
)

func set(v ...string) map[string]bool {
	m := map[string]bool{}
	for _, s := range v {
		m[s] = true
	}
	return m
}

type tally struct {
	labels map[string]string
	counts map[string]int
}

func (t *tally) add(v *string) {
	if v == nil {
		return
	}
	name := strings.Join(strings.Fields(*v), " ")
	if name == "" {
		return
	}
	key := strings.ToLower(name)
	if _, ok := t.labels[key]; !ok {
		t.labels[key] = name
	}
	t.counts[key]++
}

func (t *tally) list() []map[string]any {
	keys := make([]string, 0, len(t.counts))
	for k := range t.counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if t.counts[keys[i]] != t.counts[keys[j]] {
			return t.counts[keys[i]] > t.counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	out := []map[string]any{}
	for _, k := range keys {
		out = append(out, map[string]any{"name": t.labels[k], "count": t.counts[k]})
	}
	return out
}

func newTally() *tally { return &tally{map[string]string{}, map[string]int{}} }

func fileSize(p string) (int64, error) { return storage.Size(p) }

func isFile(p string) (int64, bool) {
	n, err := storage.Size(p)
	return n, err == nil
}

func (s *svc) nsDetails(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	ctx := r.Context()
	id, _, err := legacy.ArgInt(r, "nsId", true)
	if err != nil {
		return nil, err
	}
	if err := s.owned(ctx, s.a.Pool, id, true, uid, -1); err != nil {
		return nil, err
	}
	q := dbgen.New(s.a.Pool)
	sum, err := q.GetNamespaceSummary(ctx, dbgen.GetNamespaceSummaryParams{NsID: id, UserID: uid})
	if err != nil {
		return nil, err
	}
	nsName, uuid, folders := sum.NsName, sum.Uuid, sum.Folders
	rows, err := q.ListNamespaceTreePhotos(ctx, dbgen.ListNamespaceTreePhotosParams{NsID: id, UserID: uid})
	if err != nil {
		return nil, err
	}
	var photos, videos, orig, thumb, missing int64
	cams, lenses := newTally(), newTally()
	type pk struct{ place, country string }
	placeCount := map[pk]int{}
	root := s.a.Cfg.UploadFolder
	for _, row := range rows {
		puuid, format := row.PhotoUuid, row.Format
		suffix := strings.ToLower(format)
		if imageFormats[suffix] {
			photos++
		}
		if videoFormats[suffix] {
			videos++
		}
		path := func(n string) (string, error) { return legacy.MediaPath(r.Context(), s.a.Pool, root, uuid, n) }
		p, err := path(puuid + "." + format)
		if err != nil {
			return nil, err
		}
		if n, err := fileSize(p); err != nil {
			missing++
		} else {
			orig += n
		}
		if p, err = path(puuid + "_thumbnail." + format); err != nil {
			return nil, err
		}
		if n, err := fileSize(p); err != nil {
			if !os.IsNotExist(err) || thumbFormats[suffix] {
				missing++
			}
		} else {
			thumb += n
		}
		if row.HasMedia {
			for _, sfx := range []string{"_preview.jpg", "_live.mov", "_live.mp4"} {
				if p, err := path(puuid + sfx); err == nil {
					if n, ok := isFile(p); ok {
						orig += n
					}
				}
			}
		}
		if row.HasExtra && imageFormats[suffix] {
			cams.add(row.ExtraPhotoCamera)
			lenses.add(row.ExtraPhotoLens)
			if pl := places().nearest(row.ExtraPhotoGpsLat, row.ExtraPhotoGpsLon); pl != nil {
				placeCount[pk{pl.Place, pl.Country}]++
			}
		}
	}
	keys := make([]pk, 0, len(placeCount))
	for k := range placeCount {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b pk) int {
		if placeCount[a] != placeCount[b] {
			return placeCount[b] - placeCount[a]
		}
		if c := strings.Compare(a.place, b.place); c != 0 {
			return c
		}
		return strings.Compare(a.country, b.country)
	})
	pls := []map[string]any{}
	for _, k := range keys {
		pls = append(pls, map[string]any{"name": k.place + ", " + k.country, "count": placeCount[k]})
	}
	return legacy.Res{V: map[string]any{"nsId": id, "nsName": nsName, "photoCount": photos, "videoCount": videos,
		"folderCount": folders - 1, "originalBytes": orig, "thumbnailBytes": thumb, "totalBytes": orig + thumb,
		"missingFileCount": missing, "complete": missing == 0, "cameras": cams.list(), "lenses": lenses.list(), "places": pls}, Code: 200}, nil
}

func (s *svc) nsInsert(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	data, err := legacy.Body(r)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	var name string
	var id int64
	err = legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uid, false, func(tx pgx.Tx, f *legacy.Files, u *legacy.LockedUser) error {
		var e error
		name, e = legacy.ValidName(data["nsName"], false)
		if e != nil {
			return e
		}
		pid, ok := legacy.Int(data["nsParentId"])
		if e = s.owned(ctx, tx, pid, ok, uid, -1); e != nil {
			return e
		}
		id, name, e = storage.CreateFolder(ctx, tx, f.Files, s.a.Cfg.UploadFolder, uid, pid, name)
		return e
	})
	return legacy.Res{V: map[string]any{"nsId": id, "nsName": name}, Code: 200}, err
}

func parentOf(ctx context.Context, db legacy.DB, id int64) (parent int64, name string, err error) {
	ns, err := dbgen.New(db).GetNamespace(ctx, id)
	if ns.NsParentID != nil {
		parent = *ns.NsParentID
	}
	if ns.NsName != nil {
		name = *ns.NsName
	}
	return parent, name, err
}

func (s *svc) changeFolder(r *http.Request, uid int64, rename bool) (any, error) {
	data, err := legacy.Body(r)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	id, ok := legacy.Int(data["nsId"])
	var name string
	err = legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uid, false, func(tx pgx.Tx, f *legacy.Files, u *legacy.LockedUser) error {
		if e := s.owned(ctx, tx, id, ok, uid, -1); e != nil {
			return e
		}
		var parent int64
		var requested string
		if rename {
			var e error
			requested, e = legacy.ValidName(data["nsName"], false)
			if e != nil {
				return e
			}
		} else {
			pid, present := legacy.Int(data["nsParentId"])
			if e := s.owned(ctx, tx, pid, present, uid, -1); e != nil {
				return e
			}
			parent = pid
		}
		var e error
		name, e = storage.RehomeFolder(ctx, tx, f.Files, s.a.Cfg.UploadFolder, uid, id, parent, requested)
		return e
	})
	return legacy.Res{V: map[string]any{"nsId": id, "nsName": name}, Code: 200}, err
}
func (s *svc) nsRename(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	return s.changeFolder(r, uid, true)
}
func (s *svc) nsMove(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	return s.changeFolder(r, uid, false)
}

func (s *svc) nsTrace(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	id, ok, err := legacy.ArgInt(r, "nsId", false)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	out := []map[string]any{}
	seen := map[int64]bool{}
	for {
		if err := s.owned(ctx, s.a.Pool, id, ok, uid, nil); err != nil {
			return nil, err
		}
		if seen[id] {
			return nil, legacy.Err(nil, 400)
		}
		seen[id] = true
		p, name, err := parentOf(ctx, s.a.Pool, id)
		if err != nil {
			return nil, err
		}
		row, err := dbgen.New(s.a.Pool).GetNamespace(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, legacy.NsJSON(row))
		_ = name
		if p == -1 {
			break
		}
		id, ok = p, true
	}
	slices.Reverse(out)
	return legacy.Res{V: out, Code: 200}, nil
}

type node struct {
	id   int64
	path string
}

// tree is NamespaceService.tree: depth-first (stack) order with sibling-name de-duplication.
func (s *svc) tree(ctx context.Context, db legacy.DB, uid, id int64) ([]node, error) {
	if err := s.owned(ctx, db, id, true, uid, -1); err != nil {
		return nil, err
	}
	_, name, err := parentOf(ctx, db, id)
	if err != nil {
		return nil, err
	}
	first := "root"
	if name != "/" {
		if first, err = legacy.ValidName(name, false); err != nil {
			return nil, err
		}
	}
	stack, seen, out := []node{{id, first}}, map[int64]bool{}, []node{}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[n.id] {
			return nil, legacy.Bad()
		}
		seen[n.id] = true
		out = append(out, n)
		children, err := dbgen.New(db).ListChildNamespaceNames(ctx, dbgen.ListChildNamespaceNamesParams{UserID: uid, ParentID: n.id})
		if err != nil {
			return nil, err
		}
		names := map[string]bool{}
		for _, c := range children {
			var enabled bool
			if err := db.QueryRow(ctx, `SELECT s.enabled FROM namespace n JOIN storage_source s ON s.id=n.source_id WHERE n.ns_id=$1`, c.NsID).Scan(&enabled); err != nil {
				return nil, err
			}
			if !enabled {
				continue
			}
			var v any
			if c.NsName != nil {
				v = *c.NsName
			}
			nm, err := legacy.ValidName(v, false)
			if err != nil {
				return nil, err
			}
			if names[nm] {
				nm += " (" + itoa(c.NsID) + ")"
			}
			names[nm] = true
			stack = append(stack, node{c.NsID, n.path + "/" + nm})
		}
	}
	return out, nil
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func (s *svc) nsDelete(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	id, ok, err := legacy.ArgInt(r, "nsId", false)
	if err != nil {
		return nil, err
	}
	ctx, root := r.Context(), s.a.Cfg.UploadFolder
	count := 0
	err = legacy.Atomic(ctx, s.a.Pool, root, uid, false, func(tx pgx.Tx, f *legacy.Files, u *legacy.LockedUser) error {
		if err := s.owned(ctx, tx, id, ok, uid, -1); err != nil {
			return err
		}
		if p, _, err := parentOf(ctx, tx, id); err != nil {
			return err
		} else if p == -1 {
			return legacy.Bad()
		}
		var mount bool
		if err := tx.QueryRow(ctx, `SELECT is_mount FROM namespace WHERE ns_id=$1`, id).Scan(&mount); err != nil {
			return err
		}
		if mount {
			return legacy.Err("protected_folder", 400)
		}
		dir, source, err := storage.Folder(ctx, tx, root, id)
		if err != nil {
			return err
		}
		tree, err := s.tree(ctx, tx, uid, id)
		if err != nil {
			return err
		}
		for i := len(tree) - 1; i >= 0; i-- {
			q := dbgen.New(tx)
			ids, err := q.ListPhotoIDsInNamespace(ctx, dbgen.ListPhotoIDsInNamespaceParams{UserID: uid, NsID: tree[i].id})
			if err != nil {
				return err
			}
			n, err := legacy.DeletePhotos(ctx, tx, f, root, u, ids)
			if err != nil {
				return err
			}
			count += n
			if err := q.DeleteNamespace(ctx, tree[i].id); err != nil {
				return err
			}
			count++
		}
		if source.Kind == "local" {
			if _, err = f.Remove(dir); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return legacy.Changed(count, 0), nil
}

func (s *svc) nsDownload(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	ctx := r.Context()
	id, ok, err := legacy.ArgInt(r, "nsId", false)
	if err != nil {
		return nil, err
	}
	if err = s.owned(ctx, s.a.Pool, id, ok, uid, -1); err != nil {
		return nil, err
	}
	uuidp, err := dbgen.New(s.a.Pool).GetUserUUID(ctx, uid)
	if err != nil {
		return nil, err
	}
	uuid := deref(uuidp)
	nodes, err := s.tree(ctx, s.a.Pool, uid, id)
	if err != nil {
		return nil, err
	}
	var entries []legacy.Entry
	localCovered := map[int64]bool{}
	for _, n := range nodes {
		dir, source, err := storage.Folder(ctx, s.a.Pool, s.a.Cfg.UploadFolder, n.id)
		if err != nil {
			return nil, err
		}
		if source.Kind == "local" {
			expected, e := dbgen.New(s.a.Pool).ListNamespaceDownloadPhotos(ctx, dbgen.ListNamespaceDownloadPhotosParams{UserID: uid, NsID: n.id})
			if e != nil {
				return nil, e
			}
			for _, item := range expected {
				roles := []string{"original"}
				if item.Live {
					roles = append(roles, "live_original")
				}
				for _, role := range roles {
					p, e := storage.Resolve(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uuid, item.PhotoUuid, role)
					if e != nil {
						return nil, e
					}
					if _, e = os.Lstat(p); e != nil {
						return nil, legacy.Err("archive_source_missing", 404)
					}
				}
			}
			if localCovered[source.ID] {
				continue
			}
			localCovered[source.ID] = true
			err = filepath.WalkDir(dir, func(p string, d os.DirEntry, e error) error {
				if e != nil {
					return e
				}
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if d.Type()&os.ModeSymlink != 0 {
					return nil
				}
				if !d.IsDir() && !d.Type().IsRegular() {
					return nil
				}
				rel, e := filepath.Rel(dir, p)
				if e != nil {
					return e
				}
				name := n.path
				if rel != "." {
					name += "/" + filepath.ToSlash(rel)
				}
				entries = append(entries, legacy.Entry{Path: p, Name: name, Dir: d.IsDir()})
				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			entries = append(entries, legacy.Entry{Name: n.path, Dir: true})
			rows, err := dbgen.New(s.a.Pool).ListNamespaceDownloadPhotos(ctx, dbgen.ListNamespaceDownloadPhotosParams{UserID: uid, NsID: n.id})
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				name := deref(row.PhotoName) + "." + row.Format
				p, err := storage.Resolve(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uuid, row.PhotoUuid, "original")
				if err != nil {
					return nil, err
				}
				entries = append(entries, legacy.Entry{Path: p, Name: n.path + "/" + name, ID: row.PhotoID})
				if row.Live {
					p, err = storage.Resolve(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uuid, row.PhotoUuid, "live_original")
					if err != nil {
						return nil, err
					}
					var liveName string
					if err = s.a.Pool.QueryRow(ctx, `SELECT coalesce(nullif(display_name,''),$2) FROM media_asset WHERE photo_id=$1 AND role='live_original'`, row.PhotoID, deref(row.PhotoName)+"_live.mov").Scan(&liveName); err != nil {
						return nil, err
					}
					entries = append(entries, legacy.Entry{Path: p, Name: n.path + "/" + liveName, ID: row.PhotoID})
				}
			}
		}
	}
	name := "root.zip"
	if len(nodes) > 0 {
		name = filepath.Base(nodes[0].path) + ".zip"
	}
	legacy.ServeArchiveNamed(w, r, entries, name)
	return legacy.Done{}, nil
}
