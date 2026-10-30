package admin

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	"github.com/jackc/pgx/v5"
)

func (s *svc) registerLocalStorage() {
	opt := legacy.Opt{Auth: "header"}
	for _, pattern := range []string{"GET /admin/storage/local-roots", "GET /admin/storage/local-directories", "GET /admin/users/{userId}/storage/local", "POST /admin/users/{userId}/storage/local", "PATCH /admin/users/{userId}/storage/local/{sourceId}", "DELETE /admin/users/{userId}/storage/local/{sourceId}"} {
		s.a.Mux.HandleFunc(pattern, opt.Wrap(s.a, s.localStorage))
	}
}
func (s *svc) localStorage(w http.ResponseWriter, r *http.Request, caller int64) (any, error) {
	var admin bool
	if e := s.a.Pool.QueryRow(r.Context(), `SELECT role='admin' FROM "user" WHERE user_id=$1`, caller).Scan(&admin); e != nil {
		return nil, e
	}
	if !admin {
		return nil, legacy.Err("admin_required", 403)
	}
	ctx := r.Context()
	root := s.a.Cfg.UploadFolder
	if r.URL.Path == "/admin/storage/local-roots" {
		v, e := storage.LocalRoots()
		return legacy.Res{V: v, Code: 200}, e
	}
	if r.URL.Path == "/admin/storage/local-directories" {
		relative := r.URL.Query().Get("relativePath")
		id := r.URL.Query().Get("rootId")
		p, _, e := storage.LocalBinding(root, id, relative)
		if e != nil {
			return nil, legacy.Err(e.Error(), 400)
		}
		dirs, e := os.ReadDir(p)
		if e != nil {
			return nil, e
		}
		out := []map[string]string{}
		for _, d := range dirs {
			if !d.IsDir() || d.Type()&os.ModeSymlink != 0 || d.Name() == storage.SystemDirectory {
				continue
			}
			rel := filepath.ToSlash(filepath.Join(relative, d.Name()))
			if _, _, e = storage.LocalBinding(root, id, rel); e != nil {
				continue
			}
			out = append(out, map[string]string{"name": d.Name(), "relativePath": rel})
		}
		return legacy.Res{V: out, Code: 200}, nil
	}
	uid, e := strconv.ParseInt(r.PathValue("userId"), 10, 64)
	if e != nil || uid < 1 {
		return nil, legacy.Bad()
	}
	id, _ := strconv.ParseInt(r.PathValue("sourceId"), 10, 64)
	if r.Method == "GET" {
		rows, e := s.a.Pool.Query(ctx, `SELECT id,user_id,display_name,root_path,enabled,scan_interval,config,scan_status,coalesce(scan_error,''),scanned_at FROM storage_source WHERE kind='local' AND managed_by='admin' ORDER BY id`)
		if e != nil {
			return nil, e
		}
		defer rows.Close()
		type binding struct {
			ID, UID       int64
			Name, Path    string
			Enabled       bool
			Interval      int
			Config        []byte
			Status, Error string
			At            any
		}
		all := []binding{}
		for rows.Next() {
			var b binding
			if e = rows.Scan(&b.ID, &b.UID, &b.Name, &b.Path, &b.Enabled, &b.Interval, &b.Config, &b.Status, &b.Error, &b.At); e != nil {
				return nil, e
			}
			all = append(all, b)
		}
		if e = rows.Err(); e != nil {
			return nil, e
		}
		out := []map[string]any{}
		for _, b := range all {
			if b.UID != uid {
				continue
			}
			v := map[string]string{}
			if e = json.Unmarshal(b.Config, &v); e != nil {
				return nil, e
			}
			shared := []map[string]any{}
			for _, other := range all {
				if other.ID != b.ID && (storage.Within(b.Path, other.Path) || storage.Within(other.Path, b.Path)) {
					shared = append(shared, map[string]any{"sourceId": other.ID, "userId": other.UID, "name": other.Name})
				}
			}
			out = append(out, map[string]any{"sourceId": b.ID, "name": b.Name, "rootId": v["root_id"], "relativePath": v["relative_path"], "enabled": b.Enabled, "scanInterval": b.Interval, "scanStatus": b.Status, "scanError": b.Error, "scannedAt": b.At, "overlaps": shared})
		}
		return legacy.Res{V: out, Code: 200}, nil
	}
	if (r.Method == "PATCH" || r.Method == "DELETE") && id < 1 {
		return nil, legacy.Bad()
	}
	var in storage.BindingInput
	if r.Method != "DELETE" {
		if e = json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&in); e != nil {
			return nil, legacy.Bad()
		}
	}
	e = legacy.Atomic(ctx, s.a.Pool, root, uid, false, func(tx pgx.Tx, f *legacy.Files, u *legacy.LockedUser) error {
		if r.Method == "DELETE" {
			return storage.DetachBinding(ctx, tx, f.Files, root, uid, id, "local")
		}
		var err error
		id, err = storage.SaveBinding(ctx, tx, root, uid, id, "local", in)
		return err
	})
	if e != nil {
		return nil, legacy.Err(e.Error(), 400)
	}
	return legacy.Res{V: map[string]any{"sourceId": id}, Code: 200}, nil
}
