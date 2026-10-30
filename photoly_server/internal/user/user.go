// Package user ports controllers/{user,cred}.py: accounts, sessions, avatars and API credentials.
package user

import (
	"bytes"
	"context"
	_ "embed"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	"github.com/jackc/pgx/v5"
)

//go:embed defaultAvatar.jpg
var defaultAvatar []byte

var emailRe = regexp.MustCompile(`\A.+@.+\..+\z`)

var (
	resultOpt = legacy.Opt{Auth: "header"}
	noAuthRes = legacy.Opt{}
	tokenOpt  = legacy.Opt{Shape: "token"}
)

func Register(a *app.App) {
	_ = legacy.UseDefaultSessions(context.Background(), a.Pool)
	s := &svc{a: a}
	m := a.Mux
	m.HandleFunc("POST /user/signUp", tokenOpt.Wrap(a, s.signUp))
	m.HandleFunc("POST /user/signIn", tokenOpt.Wrap(a, s.signIn))
	m.HandleFunc("POST /user/signOut", noAuthRes.Wrap(a, s.signOut))
	m.HandleFunc("GET /user/getInfo", resultOpt.Wrap(a, s.info))
	m.HandleFunc("GET /user/getStorage", resultOpt.Wrap(a, s.storage))
	s.registerSources()
	m.HandleFunc("POST /user/updateUsername", resultOpt.Wrap(a, s.updateField("user_name", "username")))
	m.HandleFunc("POST /user/updateEmail", resultOpt.Wrap(a, s.updateField("email", "email")))
	m.HandleFunc("POST /user/updatePassword", resultOpt.Wrap(a, s.updateField("password", "newPass")))
	m.HandleFunc("POST /user/updateAvatar", resultOpt.Wrap(a, s.updateAvatar))
	m.HandleFunc("GET /user/getAvatar/{token}", legacy.Opt{Auth: "path", Shape: "binary"}.Wrap(a, s.avatar))
	m.HandleFunc("GET /user/deleteUser", resultOpt.Wrap(a, s.delete))
	m.HandleFunc("POST /user/pingToken", resultOpt.Wrap(a, func(http.ResponseWriter, *http.Request, int64) (any, error) {
		return legacy.Res{V: 1, Code: 200}, nil
	}))
	registerCred(a, s)
}

type svc struct{ a *app.App }

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s *svc) setting(ctx context.Context, name, def string) string {
	v, err := dbgen.New(s.a.Pool).GetSetting(ctx, name)
	if err != nil || v == nil {
		return def
	}
	return *v
}

func (s *svc) emailTaken(ctx context.Context, q legacy.DB, email string) (bool, error) {
	n, err := dbgen.New(q).CountUsersByEmail(ctx, strings.ToLower(email))
	return n > 0, err
}

func (s *svc) login(ctx context.Context, uid int64) (any, error) {
	days, err := strconv.Atoi(strings.TrimSpace(s.setting(ctx, "TokenDuration", "30")))
	if err != nil {
		days = 30
	}
	tok, err := legacy.Sessions.Create(ctx, uid, days)
	if err != nil {
		return nil, err
	}
	return legacy.TokenRes{V: "Succeed", Token: &tok, Code: 200}, nil
}

func tokErr(msg string, code int) legacy.TokenRes { return legacy.TokenRes{V: msg, Code: code} }

func (s *svc) signUp(w http.ResponseWriter, r *http.Request, _ int64) (any, error) {
	data, err := legacy.Body(r)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	if s.setting(ctx, "SignUp", "0") != "1" {
		return tokErr("Register is closed by Admin", 400), nil
	}
	if data["email"] == nil {
		return tokErr("Email is null", 400), nil
	}
	email, ok := data["email"].(string)
	if !ok || !emailRe.MatchString(email) {
		return tokErr("Email is incorrect", 400), nil
	}
	if taken, err := s.emailTaken(ctx, s.a.Pool, email); err != nil {
		return nil, err
	} else if taken {
		return tokErr("Repeated Email Address", 401), nil
	}
	password, ok := data["password"].(string)
	if !ok || password == "" {
		return nil, legacy.Bad()
	}
	var name *string
	if v, present := data["userName"]; present && v != nil {
		n, ok := v.(string)
		if !ok {
			return nil, legacy.Bad()
		}
		name = &n
	}
	var uid int64
	err = legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, 0, false, func(tx pgx.Tx, f *legacy.Files, _ *legacy.LockedUser) error {
		if taken, err := s.emailTaken(ctx, tx, email); err != nil {
			return err
		} else if taken {
			return legacy.Bad()
		}
		q := dbgen.New(tx)
		aiDefault, _ := q.NewUserAIDefault(ctx)
		now := legacy.Now()
		var err error
		uid, err = q.InsertUser(ctx, dbgen.InsertUserParams{UserName: name, Role: "user", Email: email, Password: legacy.HashPassword(password),
			Uuid: legacy.NewUUID(), CreateDate: now, AiSearchEnabled: aiDefault})
		if err != nil {
			return err
		}
		if err = q.InsertRootNamespace(ctx, uid); err != nil {
			return err
		}
		var accountUUID string
		if err = tx.QueryRow(ctx, `SELECT uuid FROM "user" WHERE user_id=$1`, uid).Scan(&accountUUID); err != nil {
			return err
		}
		accountDir, e := storage.SafePath(s.a.Cfg.UploadFolder, accountUUID)
		if e != nil {
			return e
		}
		if e = f.Mkdir(accountDir); e != nil {
			return e
		}
		if e = storage.RecordRootIdentity(ctx, tx, uid, accountDir); e != nil {
			return e
		}
		return q.InsertFavoriteGallery(ctx, dbgen.InsertFavoriteGalleryParams{UserID: uid, CreateDate: now})
	})
	if err != nil {
		return nil, err
	}
	return s.login(ctx, uid)
}

func (s *svc) signIn(w http.ResponseWriter, r *http.Request, _ int64) (any, error) {
	data, err := legacy.Body(r)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	// by_email compares against str(email).lower(), so a missing email looks up "none".
	em := "none"
	switch v := data["email"].(type) {
	case string:
		em = v
	case nil:
	default:
		return tokErr("Invalid Email or Password", 400), nil
	}
	u, err := dbgen.New(s.a.Pool).GetUserLogin(ctx, strings.ToLower(em))
	hash, uid := u.Password, u.UserID
	pw, ok := data["password"].(string)
	if err != nil || !ok || hash == nil || *hash == "" || !legacy.CheckPassword(*hash, pw) {
		if err != nil && err != pgx.ErrNoRows {
			return nil, err
		}
		return tokErr("Invalid Email or Password", 400), nil
	}
	return s.login(ctx, uid)
}

func (s *svc) signOut(w http.ResponseWriter, r *http.Request, _ int64) (any, error) {
	legacy.Sessions.Delete(r.Context(), r.Header.Get("HRD-Token"))
	return legacy.Res{V: 1, Code: 200}, nil
}

func (s *svc) info(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	u, err := dbgen.New(s.a.Pool).GetUserInfo(r.Context(), uid)
	if err != nil {
		return nil, err
	}
	return legacy.Res{V: map[string]any{"userId": u.UserID, "userName": u.UserName, "email": u.Email, "createDate": legacy.ISO(u.CreateDate),
		"role": u.Role, "password": nil, "uuid": u.Uuid}, Code: 200}, nil
}

func (s *svc) storage(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	ctx := r.Context()
	if err := legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uid, false, func(pgx.Tx, *legacy.Files, *legacy.LockedUser) error { return nil }); err != nil {
		return nil, err
	}
	st, err := dbgen.New(s.a.Pool).GetUserStorage(ctx, uid)
	if err != nil {
		return nil, err
	}
	limit, source := st.GroupMaxStorageBytes, "group"
	if st.StorageOverride {
		limit, source = st.MaxStorageBytes, "user"
	}
	var u int64
	if st.UsedBytes != nil {
		u = *st.UsedBytes
	}
	return legacy.Res{V: map[string]any{"usedBytes": u, "capacityBytes": limit, "groupId": st.GroupID, "groupName": st.GroupName,
		"storageOverride": st.StorageOverride, "maxStorageBytes": st.MaxStorageBytes, "quotaSource": source, "backend": "Local disk",
		"overQuota": limit != nil && u > *limit}, Code: 200}, nil
}

func optStr(v string, ok bool) any {
	if !ok {
		return nil
	}
	return v
}

// updateField is UserService.update for user_name, email and password (arg names differ).
func (s *svc) updateField(field, argName string) legacy.Handler {
	return func(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
		val, present := legacy.Arg(r, argName)
		old, oldPresent := legacy.Arg(r, "oldPass")
		ctx := r.Context()
		err := legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uid, false, func(tx pgx.Tx, _ *legacy.Files, _ *legacy.LockedUser) error {
			switch field {
			case "email":
				if !present {
					return legacy.Bad()
				}
				if taken, err := s.emailTaken(ctx, tx, val); err != nil {
					return err
				} else if !emailRe.MatchString(val) || taken {
					return legacy.Err(-2, 400)
				}
			case "password":
				hash, err := dbgen.New(tx).GetUserPassword(ctx, uid)
				if err != nil {
					return err
				}
				if !oldPresent || old == "" || hash == nil || !legacy.CheckPassword(*hash, old) {
					return legacy.Err(-2, 400)
				}
				if !present || val == "" {
					return legacy.Bad()
				}
				val = legacy.HashPassword(val)
			default:
				if !present || val == "" {
					return legacy.Bad()
				}
			}
			q := dbgen.New(tx)
			switch field {
			case "email":
				return q.UpdateUserEmail(ctx, dbgen.UpdateUserEmailParams{UserID: uid, Value: &val})
			case "password":
				return q.UpdateUserPassword(ctx, dbgen.UpdateUserPasswordParams{UserID: uid, Value: &val})
			default:
				return q.UpdateUserName(ctx, dbgen.UpdateUserNameParams{UserID: uid, Value: &val})
			}
		})
		if err != nil {
			return nil, err
		}
		return legacy.Changed(1, 1), nil
	}
}

func (s *svc) updateAvatar(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	_ = r.ParseMultipartForm(32 << 20)
	file, _, err := r.FormFile("file")
	if err != nil {
		return nil, legacy.Bad()
	}
	defer file.Close()
	img, err := avatarJPEG(file)
	if err != nil {
		return nil, err
	}
	err = legacy.Atomic(r.Context(), s.a.Pool, s.a.Cfg.UploadFolder, uid, false, func(tx pgx.Tx, f *legacy.Files, u *legacy.LockedUser) error {
		p, err := legacy.MediaPath(r.Context(), s.a.Pool, s.a.Cfg.UploadFolder, u.UUID, u.UUID+".jpg")
		if err != nil {
			return err
		}
		return f.Install(bytes.NewReader(img), p)
	})
	if err != nil {
		return nil, err
	}
	return legacy.Changed(1, 1), nil
}

func (s *svc) avatar(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	uuidp, err := dbgen.New(s.a.Pool).GetUserUUID(r.Context(), uid)
	if err != nil {
		return nil, err
	}
	uuid := deref(uuidp)
	p, err := legacy.MediaPath(r.Context(), s.a.Pool, s.a.Cfg.UploadFolder, uuid, uuid+".jpg")
	if err != nil {
		return nil, err
	}
	if isFile(p) {
		legacy.ServeFile(w, r, p, "image/jpeg", "")
		return legacy.Done{}, nil
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(defaultAvatar))
	return legacy.Done{}, nil
}

func (s *svc) delete(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	ctx := r.Context()
	root := s.a.Cfg.UploadFolder
	count := 0
	err := legacy.Atomic(storage.ExclusiveUser(ctx), s.a.Pool, root, uid, false, func(tx pgx.Tx, f *legacy.Files, u *legacy.LockedUser) error {
		q := dbgen.New(tx)
		ids, err := q.ListUserPhotoIDs(ctx, uid)
		if err != nil {
			return err
		}
		n, err := legacy.DeletePhotos(storage.PreserveExternal(ctx), tx, f, root, u, ids)
		if err != nil {
			return err
		}
		count += n
		for _, del := range []func(context.Context, int64) (int64, error){q.DeleteUserNamespaces, q.DeleteUserTags, q.DeleteUserGalleries} {
			n, err := del(ctx, uid)
			if err != nil {
				return err
			}
			count += int(n)
		}
		if _, err = q.DeleteUserCreds(ctx, uid); err != nil {
			return err
		}
		p, err := legacy.MediaPath(r.Context(), s.a.Pool, root, u.UUID, u.UUID+".jpg")
		if err != nil {
			return err
		}
		if _, err = f.Remove(p); err != nil {
			return err
		}
		if err = legacy.RemoveUserFiles(f, root, u.UUID); err != nil {
			return err
		}
		if _, err = q.DeleteUser(ctx, uid); err != nil {
			return err
		}
		u.Gone = true
		count++
		return nil
	})
	if err != nil {
		return nil, err
	}
	legacy.Sessions.RevokeUser(ctx, uid)
	return legacy.Changed(count, 1), nil
}

func isFile(p string) bool {
	storage.Fetch(p)
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}
