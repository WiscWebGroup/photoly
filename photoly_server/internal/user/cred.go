package user

import (
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	"github.com/jackc/pgx/v5"
)

// UploadPhotos is PhotoService.upload, owned by the photo port. cred upload routes call it
// with the multipart files and per-file descriptions and expect the new photo UUIDs back.
// While nil, those two routes answer 501.
var UploadPhotos func(ctx context.Context, userID int64, files []*multipart.FileHeader, descriptions []map[string]any) ([]string, error)

type cred struct {
	UserID int64
	Auth   string
}

func (s *svc) identify(ctx context.Context, token string) (*cred, error) {
	row, err := dbgen.New(s.a.Pool).GetCredByToken(ctx, token)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &cred{UserID: row.UserID, Auth: row.Authorization}, err
}

func authorization(v any) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", legacy.Bad()
	}
	up, out := strings.ToUpper(s), ""
	for _, c := range "CRD" {
		if strings.ContainsRune(up, c) {
			out += string(c)
		}
	}
	if out == "" {
		return "", legacy.Bad()
	}
	return out, nil
}

func registerCred(a *app.App, s *svc) {
	m := a.Mux
	res := legacy.Opt{Auth: "header"}
	m.HandleFunc("POST /cred/create", legacy.Opt{Auth: "header", Failure: 0, HasFailure: true}.Wrap(a, s.credCreate))
	m.HandleFunc("GET /cred/query", res.Wrap(a, func(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
		rows, err := dbgen.New(a.Pool).ListUserCreds(r.Context(), uid)
		out := make([]map[string]any, len(rows))
		for i, c := range rows {
			out[i] = map[string]any{"credId": c.CredID, "userId": c.UserID, "token": c.Token, "authorization": c.Authorization}
		}
		return legacy.Res{V: out, Code: 200}, err
	}))
	m.HandleFunc("POST /cred/update", res.Wrap(a, s.credUpdate))
	m.HandleFunc("POST /cred/delete", res.Wrap(a, s.credDelete))
	raw := legacy.Opt{Shape: "raw"}
	m.HandleFunc("POST /cred/uploadPhoto/{token}", raw.Wrap(a, s.credUpload(false)))
	m.HandleFunc("POST /cred/uploadPhotos/{token}", raw.Wrap(a, s.credUpload(true)))
	m.HandleFunc("GET /cred/render/{token}", raw.Wrap(a, s.credRender))
	m.HandleFunc("POST /cred/deletePhoto/{token}", legacy.Opt{Shape: "raw", Failure: -1, HasFailure: true}.Wrap(a, s.credDeletePhoto))
	m.HandleFunc("GET /cred/queryRootNamespace/{token}", raw.Wrap(a, s.credRoot))
	m.HandleFunc("GET /cred/queryNamespaces/{token}", raw.Wrap(a, s.credChildren))
	m.HandleFunc("GET /cred/queryPhotoList/{token}", raw.Wrap(a, s.credPhotoList))
}

func (s *svc) credCreate(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	data, err := legacy.Body(r)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	err = legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uid, false, func(tx pgx.Tx, _ *legacy.Files, _ *legacy.LockedUser) error {
		auth, err := authorization(data["authorization"])
		if err != nil {
			return err
		}
		return dbgen.New(tx).InsertCred(ctx, dbgen.InsertCredParams{UserID: uid, Token: legacy.NewUUID(), Auth: auth})
	})
	if err != nil {
		return nil, err
	}
	return legacy.Changed(1, 1), nil
}

func (s *svc) credUpdate(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	data, err := legacy.Body(r)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	err = legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uid, false, func(tx pgx.Tx, _ *legacy.Files, _ *legacy.LockedUser) error {
		id, ok := legacy.Int(data["credId"])
		if err := legacy.Owned(ctx, tx, "cred", id, ok, uid, -1, 400); err != nil {
			return err
		}
		auth, err := authorization(data["authorization"])
		if err != nil {
			return err
		}
		return dbgen.New(tx).UpdateCredAuthorization(ctx, dbgen.UpdateCredAuthorizationParams{CredID: id, Auth: auth})
	})
	if err != nil {
		return nil, err
	}
	return legacy.Changed(1, 1), nil
}

func (s *svc) credDelete(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	id, ok, err := legacy.ArgInt(r, "credId", false)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	err = legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uid, false, func(tx pgx.Tx, _ *legacy.Files, _ *legacy.LockedUser) error {
		if err := legacy.Owned(ctx, tx, "cred", id, ok, uid, -1, 400); err != nil {
			return err
		}
		return dbgen.New(tx).DeleteCred(ctx, id)
	})
	if err != nil {
		return nil, err
	}
	return legacy.Changed(1, 1), nil
}

// withCred resolves the path token and requires letter (empty = any credential).
func (s *svc) withCred(r *http.Request, letter string) (*cred, error) {
	c, err := s.identify(r.Context(), r.PathValue("token"))
	if err != nil || c == nil || (letter != "" && !strings.Contains(c.Auth, letter)) {
		return nil, err
	}
	return c, nil
}

func (s *svc) credUpload(multiple bool) legacy.Handler {
	return func(w http.ResponseWriter, r *http.Request, _ int64) (any, error) {
		c, err := s.identify(r.Context(), r.PathValue("token"))
		if err != nil {
			return nil, err
		}
		if c == nil {
			if multiple {
				return nil, nil
			}
			return legacy.Text("Unidentified Token"), nil
		}
		if !strings.Contains(c.Auth, "C") {
			if multiple {
				return nil, nil
			}
			return legacy.Text("Wrong Auth for current Token"), nil
		}
		_ = r.ParseMultipartForm(32 << 20)
		name := "photo"
		if multiple {
			name = "photosStr"
		}
		str, ok := legacy.Arg(r, name)
		if !ok {
			return nil, legacy.Bad()
		}
		var files []*multipart.FileHeader
		var descs []map[string]any
		if multiple {
			var list []map[string]any
			if json.Unmarshal([]byte(str), &list) != nil {
				return nil, legacy.Bad()
			}
			if r.MultipartForm != nil {
				files = r.MultipartForm.File["files"]
			}
			descs = list
		} else {
			var one map[string]any
			if json.Unmarshal([]byte(str), &one) != nil || one == nil || r.MultipartForm == nil || len(r.MultipartForm.File["file"]) == 0 {
				return nil, legacy.Bad()
			}
			files, descs = r.MultipartForm.File["file"][:1], []map[string]any{one}
		}
		if UploadPhotos == nil {
			http.Error(w, "photo upload not wired", http.StatusNotImplemented)
			return legacy.Done{}, nil
		}
		uuids, err := UploadPhotos(r.Context(), c.UserID, files, descs)
		if err != nil {
			return nil, err
		}
		if multiple {
			return legacy.Raw{V: uuids}, nil
		}
		return legacy.Text(uuids[0]), nil
	}
}

func (s *svc) photoByUUID(ctx context.Context, uid int64, uuid string, ok bool) (id int64, owner string, format string, err error) {
	if ok {
		var row dbgen.GetPhotoForCredRow
		row, err = dbgen.New(s.a.Pool).GetPhotoForCred(ctx, dbgen.GetPhotoForCredParams{UserID: uid, PhotoUuid: uuid})
		if err == nil {
			id, owner, format = row.PhotoID, deref(row.OwnerUuid), deref(row.Format)
		}
	} else {
		err = pgx.ErrNoRows
	}
	if err == pgx.ErrNoRows {
		err = legacy.Bad()
	}
	return
}

func (s *svc) credRender(w http.ResponseWriter, r *http.Request, _ int64) (any, error) {
	c, err := s.withCred(r, "R")
	if c == nil || err != nil {
		return nil, err
	}
	u, ok := legacy.Arg(r, "uuid")
	_, owner, format, err := s.photoByUUID(r.Context(), c.UserID, u, ok)
	if err != nil {
		return nil, err
	}
	p, err := legacy.MediaPath(s.a.Cfg.UploadFolder, owner, u+"."+format)
	if err != nil {
		return nil, err
	}
	if !isFile(p) {
		return nil, legacy.Err(nil, 404)
	}
	legacy.ServeFile(w, r, p, "image/jpeg", "")
	return legacy.Done{}, nil
}

func (s *svc) credDeletePhoto(w http.ResponseWriter, r *http.Request, _ int64) (any, error) {
	c, err := s.identify(r.Context(), r.PathValue("token"))
	if err != nil {
		return nil, err
	}
	if c == nil {
		return legacy.Raw{V: -1}, nil
	}
	if !strings.Contains(c.Auth, "D") {
		return legacy.Raw{V: -2}, nil
	}
	ctx := r.Context()
	u, ok := legacy.Arg(r, "uuid")
	id, _, _, err := s.photoByUUID(ctx, c.UserID, u, ok)
	if err != nil {
		return nil, err
	}
	root := s.a.Cfg.UploadFolder
	err = legacy.Atomic(ctx, s.a.Pool, root, c.UserID, false, func(tx pgx.Tx, f *legacy.Files, lu *legacy.LockedUser) error {
		if err := legacy.Owned(ctx, tx, "photo", id, true, c.UserID, -1, 400); err != nil {
			return err
		}
		_, err := legacy.DeletePhotos(ctx, tx, f, root, lu, []int64{id})
		return err
	})
	if err != nil {
		return nil, err
	}
	return legacy.Raw{V: 1}, nil
}

func (s *svc) credRoot(w http.ResponseWriter, r *http.Request, _ int64) (any, error) {
	c, err := s.withCred(r, "R")
	if c == nil || err != nil {
		return nil, err
	}
	ns, err := dbgen.New(s.a.Pool).GetRootNamespace(r.Context(), c.UserID)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return legacy.Raw{V: legacy.NsJSON(ns)}, err
}

func (s *svc) credChildren(w http.ResponseWriter, r *http.Request, _ int64) (any, error) {
	c, err := s.withCred(r, "R")
	if c == nil || err != nil {
		return nil, err
	}
	pid, ok, err := legacy.ArgInt(r, "parentId", false)
	if err != nil {
		return nil, err
	}
	var arg *int64
	if ok {
		arg = &pid
	}
	rows, err := dbgen.New(s.a.Pool).ListChildNamespaces(r.Context(), dbgen.ListChildNamespacesParams{UserID: c.UserID, ParentID: arg})
	return legacy.Raw{V: legacy.NsListJSON(rows)}, err
}

func (s *svc) credPhotoList(w http.ResponseWriter, r *http.Request, _ int64) (any, error) {
	c, err := s.withCred(r, "R")
	if c == nil || err != nil {
		return nil, err
	}
	ctx := r.Context()
	ns, ok, err := legacy.ArgInt(r, "nsId", false)
	if err != nil {
		return nil, err
	}
	if err := legacy.Owned(ctx, s.a.Pool, "namespace", ns, ok, c.UserID, -1, 400); err != nil {
		return nil, err
	}
	rows, err := dbgen.New(s.a.Pool).ListNamespacePhotos(ctx, dbgen.ListNamespacePhotosParams{UserID: c.UserID, NsID: ns})
	return legacy.Raw{V: legacy.PhotoListJSON(rows)}, err
}
