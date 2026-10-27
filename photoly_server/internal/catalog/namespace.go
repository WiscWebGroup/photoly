package catalog

import (
	"context"
	"net/http"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	"github.com/jackc/pgx/v5"
)

func (s *svc) nsRoot(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	m, err := legacy.NsMap(s.a.Pool.QueryRow(r.Context(), `SELECT `+legacy.NsCols+` FROM namespace WHERE user_id=$1 AND ns_parent_id=-1 LIMIT 1`, uid))
	if err == pgx.ErrNoRows {
		return legacy.Res{V: nil, Code: 200}, nil
	}
	return legacy.Res{V: m, Code: 200}, err
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
	rows, err := legacy.Rows(r.Context(), s.a.Pool, legacy.NsMap, `SELECT `+legacy.NsCols+` FROM namespace
		WHERE user_id=$1 AND ns_parent_id IS NOT DISTINCT FROM $2::int8 ORDER BY ns_id`, uid, arg)
	return legacy.Res{V: rows, Code: 200}, err
}

func (s *svc) owned(ctx context.Context, db legacy.DB, id int64, ok bool, uid int64, failure any) error {
	return legacy.Owned(ctx, db, "namespace", "ns_id", id, ok, uid, failure, 400)
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
	var name *string
	err = s.a.Pool.QueryRow(ctx, `SELECT ns_name FROM namespace WHERE ns_id=$1`, id).Scan(&name)
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

func fileSize(p string) (int64, error) {
	st, err := os.Stat(p)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

func isFile(p string) (int64, bool) {
	st, err := os.Stat(p)
	if err != nil || !st.Mode().IsRegular() {
		return 0, false
	}
	return st.Size(), true
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
	var nsName *string
	var uuid string
	var folders int64
	const cte = `WITH RECURSIVE d(ns_id) AS (SELECT ns_id FROM namespace WHERE ns_id=$1 AND user_id=$2
		UNION SELECT n.ns_id FROM namespace n JOIN d ON n.ns_parent_id=d.ns_id WHERE n.user_id=$2) `
	if err := s.a.Pool.QueryRow(ctx, cte+`SELECT (SELECT ns_name FROM namespace WHERE ns_id=$1), (SELECT uuid FROM "user" WHERE user_id=$2), (SELECT count(*) FROM d)`, id, uid).Scan(&nsName, &uuid, &folders); err != nil {
		return nil, err
	}
	rows, err := s.a.Pool.Query(ctx, cte+`SELECT p.photo_uuid, p.format, m.photo_id IS NOT NULL, e.f_photo_id IS NOT NULL,
		e.extra_photo_camera, e.extra_photo_lens, e.extra_photo_gps_lat, e.extra_photo_gps_lon
		FROM photo p LEFT JOIN photo_extra e ON e.f_photo_id=p.photo_id LEFT JOIN photo_media m ON m.photo_id=p.photo_id
		WHERE p.user_id=$2 AND p.ns_id IN (SELECT ns_id FROM d) ORDER BY p.photo_id`, id, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var photos, videos, orig, thumb, missing int64
	cams, lenses := newTally(), newTally()
	type pk struct{ place, country string }
	placeCount := map[pk]int{}
	root := s.a.Cfg.UploadFolder
	for rows.Next() {
		var puuid, format string
		var hasMedia, hasExtra bool
		var cam, lens *string
		var lat, lon *float64
		if err := rows.Scan(&puuid, &format, &hasMedia, &hasExtra, &cam, &lens, &lat, &lon); err != nil {
			return nil, err
		}
		suffix := strings.ToLower(format)
		if imageFormats[suffix] {
			photos++
		}
		if videoFormats[suffix] {
			videos++
		}
		path := func(n string) (string, error) { return legacy.MediaPath(root, uuid, n) }
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
		if hasMedia {
			for _, sfx := range []string{"_preview.jpg", "_live.mov", "_live.mp4"} {
				if p, err := path(puuid + sfx); err == nil {
					if n, ok := isFile(p); ok {
						orig += n
					}
				}
			}
		}
		if hasExtra && imageFormats[suffix] {
			cams.add(cam)
			lenses.add(lens)
			if pl := places().nearest(lat, lon); pl != nil {
				placeCount[pk{pl.Place, pl.Country}]++
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
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
	err = legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uid, false, func(tx pgx.Tx, _ *legacy.Files, _ *legacy.LockedUser) error {
		name, err := legacy.ValidName(data["nsName"], false)
		if err != nil {
			return err
		}
		pid, ok := legacy.Int(data["nsParentId"])
		if err := s.owned(ctx, tx, pid, ok, uid, -1); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO namespace(ns_name,ns_parent_id,user_id) VALUES ($1,$2,$3)`, name, pid, uid)
		return err
	})
	if err != nil {
		return nil, err
	}
	return legacy.Changed(1, 1), nil
}

func parentOf(ctx context.Context, db legacy.DB, id int64) (parent int64, name string, err error) {
	var p *int64
	var n *string
	err = db.QueryRow(ctx, `SELECT ns_parent_id, ns_name FROM namespace WHERE ns_id=$1`, id).Scan(&p, &n)
	if p != nil {
		parent = *p
	}
	if n != nil {
		name = *n
	}
	return
}

func (s *svc) nsRename(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	data, err := legacy.Body(r)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	err = legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uid, false, func(tx pgx.Tx, _ *legacy.Files, _ *legacy.LockedUser) error {
		id, ok := legacy.Int(data["nsId"])
		if err := s.owned(ctx, tx, id, ok, uid, -1); err != nil {
			return err
		}
		if p, _, err := parentOf(ctx, tx, id); err != nil {
			return err
		} else if p == -1 {
			return legacy.Bad()
		}
		name, err := legacy.ValidName(data["nsName"], false)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE namespace SET ns_name=$2 WHERE ns_id=$1`, id, name)
		return err
	})
	if err != nil {
		return nil, err
	}
	return legacy.Changed(1, 1), nil
}

func (s *svc) nsMove(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	data, err := legacy.Body(r)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	err = legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uid, false, func(tx pgx.Tx, _ *legacy.Files, _ *legacy.LockedUser) error {
		id, ok := legacy.Int(data["nsId"])
		if err := s.owned(ctx, tx, id, ok, uid, -1); err != nil {
			return err
		}
		pid, ok := legacy.Int(data["nsParentId"])
		if err := s.owned(ctx, tx, pid, ok, uid, -1); err != nil {
			return err
		}
		if p, _, err := parentOf(ctx, tx, id); err != nil {
			return err
		} else if p == -1 {
			return legacy.Bad()
		}
		seen := map[int64]bool{id: true}
		for cur := pid; ; {
			if seen[cur] {
				return legacy.Bad()
			}
			seen[cur] = true
			p, _, err := parentOf(ctx, tx, cur)
			if err != nil {
				return err
			}
			if p == -1 {
				break
			}
			if err := s.owned(ctx, tx, p, true, uid, -1); err != nil {
				return err
			}
			cur = p
		}
		_, err = tx.Exec(ctx, `UPDATE namespace SET ns_parent_id=$2 WHERE ns_id=$1`, id, pid)
		return err
	})
	if err != nil {
		return nil, err
	}
	return legacy.Changed(1, 1), nil
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
		out = append(out, map[string]any{"nsId": id, "nsName": name})
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
		rows, err := db.Query(ctx, `SELECT ns_id, ns_name FROM namespace WHERE user_id=$1 AND ns_parent_id=$2 ORDER BY ns_id`, uid, n.id)
		if err != nil {
			return nil, err
		}
		names := map[string]bool{}
		for rows.Next() {
			var cid int64
			var cname *string
			if err := rows.Scan(&cid, &cname); err != nil {
				rows.Close()
				return nil, err
			}
			var v any
			if cname != nil {
				v = *cname
			}
			nm, err := legacy.ValidName(v, false)
			if err != nil {
				rows.Close()
				return nil, err
			}
			if names[nm] {
				nm += " (" + itoa(cid) + ")"
			}
			names[nm] = true
			stack = append(stack, node{cid, n.path + "/" + nm})
		}
		rows.Close()
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
		tree, err := s.tree(ctx, tx, uid, id)
		if err != nil {
			return err
		}
		for i := len(tree) - 1; i >= 0; i-- {
			rows, err := tx.Query(ctx, `SELECT photo_id FROM photo WHERE user_id=$1 AND ns_id=$2 ORDER BY photo_id`, uid, tree[i].id)
			if err != nil {
				return err
			}
			var ids []int64
			for rows.Next() {
				var pid int64
				if err := rows.Scan(&pid); err != nil {
					rows.Close()
					return err
				}
				ids = append(ids, pid)
			}
			rows.Close()
			n, err := legacy.DeletePhotos(ctx, tx, f, root, u, ids)
			if err != nil {
				return err
			}
			count += n
			if _, err := tx.Exec(ctx, `DELETE FROM namespace WHERE ns_id=$1`, tree[i].id); err != nil {
				return err
			}
			count++
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
	if err := s.owned(ctx, s.a.Pool, id, ok, uid, -1); err != nil {
		return nil, err
	}
	var uuid string
	if err := s.a.Pool.QueryRow(ctx, `SELECT uuid FROM "user" WHERE user_id=$1`, uid).Scan(&uuid); err != nil {
		return nil, err
	}
	tree, err := s.tree(ctx, s.a.Pool, uid, id)
	if err != nil {
		return nil, err
	}
	var entries []legacy.Entry
	for _, n := range tree {
		rows, err := s.a.Pool.Query(ctx, `SELECT p.photo_id, p.photo_uuid, p.photo_name, p.format, coalesce(m.live,false)
			FROM photo p LEFT JOIN photo_media m ON m.photo_id=p.photo_id WHERE p.user_id=$1 AND p.ns_id=$2 ORDER BY p.photo_id`, uid, n.id)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var pid int64
			var puuid, format string
			var pname *string
			var live bool
			if err := rows.Scan(&pid, &puuid, &pname, &format, &live); err != nil {
				rows.Close()
				return nil, err
			}
			var v any
			if pname != nil {
				v = *pname
			}
			nm, err := legacy.ValidName(v, false)
			if err != nil {
				rows.Close()
				return nil, err
			}
			p, err := legacy.MediaPath(s.a.Cfg.UploadFolder, uuid, puuid+"."+format)
			if err != nil {
				rows.Close()
				return nil, err
			}
			entries = append(entries, legacy.Entry{Path: p, Name: n.path + "/" + nm + "." + format, ID: pid})
			if live {
				lp, err := legacy.MediaPath(s.a.Cfg.UploadFolder, uuid, puuid+"_live.mov")
				if err != nil {
					rows.Close()
					return nil, err
				}
				entries = append(entries, legacy.Entry{Path: lp, Name: n.path + "/" + nm + ".mov", ID: pid})
			}
		}
		rows.Close()
	}
	legacy.ServeArchive(w, r, entries)
	return legacy.Done{}, nil
}
