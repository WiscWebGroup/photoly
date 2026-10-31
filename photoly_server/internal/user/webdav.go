package user

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	"github.com/jackc/pgx/v5"
)

func (s *svc) registerWebDAV() {
	opt := legacy.Opt{Auth: "header"}
	for _, kind := range []string{"webdav", "smb"} {
		for _, pattern := range []string{"GET /user/storage/", "POST /user/storage/", "PATCH /user/storage/", "DELETE /user/storage/"} {
			suffix := ""
			if strings.HasPrefix(pattern, "PATCH") || strings.HasPrefix(pattern, "DELETE") {
				suffix = "/{sourceId}"
			}
			s.a.Mux.HandleFunc(pattern+kind+suffix, opt.Wrap(s.a, s.webdav))
		}
		s.a.Mux.HandleFunc("POST /user/storage/"+kind+"/test", opt.Wrap(s.a, s.webdav))
	}
	s.a.Mux.HandleFunc("POST /user/storage/sources/{sourceId}/scan", opt.Wrap(s.a, s.requestScan))
}
func (s *svc) webdav(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	ctx := r.Context()
	kind := "webdav"
	if strings.HasPrefix(r.URL.Path, "/user/storage/smb") {
		kind = "smb"
	}
	id, _ := strconv.ParseInt(r.PathValue("sourceId"), 10, 64)
	if r.Method == "GET" {
		return s.bindingList(r, uid, kind)
	}
	var in storage.BindingInput
	if r.Method != "DELETE" {
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&in); e != nil {
			return nil, legacy.Bad()
		}
	}
	if strings.HasSuffix(r.URL.Path, "/test") {
		var e error
		if v := r.URL.Query().Get("sourceId"); v != "" {
			id, e = strconv.ParseInt(v, 10, 64)
			if e != nil {
				return nil, legacy.Bad()
			}
		}
		e = storage.TestTreeBinding(ctx, s.a.Pool, uid, id, kind, in)
		if e != nil {
			return nil, legacy.Err(e.Error(), 400)
		}
		return legacy.Changed(1, 1), nil
	}
	if r.Method == "PATCH" || r.Method == "DELETE" {
		if id <= 0 {
			return nil, legacy.Bad()
		}
	}
	if id != 0 && in.Enabled == nil {
		source, e := storage.ReadSource(ctx, s.a.Pool, id)
		if e != nil || source.UserID != uid || source.Kind != kind {
			return nil, legacy.Bad()
		}
		enabled := source.Enabled
		in.Enabled = &enabled
	}
	if r.Method != "DELETE" && (in.Enabled == nil || *in.Enabled) {
		if e := storage.TestTreeBinding(ctx, s.a.Pool, uid, id, kind, in); e != nil {
			return nil, legacy.Err(e.Error(), 400)
		}
	}
	err := legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uid, false, func(tx pgx.Tx, f *legacy.Files, u *legacy.LockedUser) error {
		if r.Method == "DELETE" {
			return storage.DetachBinding(ctx, tx, f.Files, s.a.Cfg.UploadFolder, uid, id, kind)
		}
		var e error
		id, e = storage.SaveBinding(ctx, tx, s.a.Cfg.UploadFolder, uid, id, kind, in)
		return e
	})
	if err != nil {
		return nil, legacy.Err(err.Error(), 400)
	}
	return legacy.Res{V: map[string]any{"sourceId": id}, Code: 200}, nil
}
func (s *svc) bindingList(r *http.Request, uid int64, kind string) (any, error) {
	rows, e := s.a.Pool.Query(r.Context(), `SELECT id,display_name,config,enabled,scan_interval FROM storage_source WHERE user_id=$1 AND kind=$2 ORDER BY id`, uid, kind)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var name string
		var raw []byte
		var enabled bool
		var interval int
		if e = rows.Scan(&id, &name, &raw, &enabled, &interval); e != nil {
			return nil, e
		}
		v := map[string]string{}
		if e = json.Unmarshal(raw, &v); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"sourceId": id, "name": name, "url": v["url"], "enabled": enabled, "scanInterval": interval, "hasCredentials": true, "sourceType": kind, "server": v["server"], "share": v["share"], "subdirectory": v["subdirectory"]})
	}
	return legacy.Res{V: out, Code: 200}, rows.Err()
}
func (s *svc) requestScan(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	id, e := strconv.ParseInt(r.PathValue("sourceId"), 10, 64)
	if e != nil {
		return nil, legacy.Bad()
	}
	var in struct {
		Force       bool   `json:"force"`
		NamespaceID *int64 `json:"nsId"`
	}
	if r.ContentLength != 0 {
		if e = json.NewDecoder(r.Body).Decode(&in); e != nil {
			return nil, legacy.Bad()
		}
	}
	job, e := storage.QueueScan(r.Context(), s.a.Pool, uid, id, in.NamespaceID, in.Force)
	if e != nil {
		return nil, legacy.Err(e.Error(), 400)
	}
	return legacy.Res{V: map[string]any{"jobId": job.ID, "state": "queued"}, Code: 200}, nil
}
