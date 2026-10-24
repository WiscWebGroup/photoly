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

func (s *svc) setting(ctx context.Context, name, def string) string {
	var v *string
	if err := s.a.Pool.QueryRow(ctx, `SELECT setting_value FROM setting WHERE setting_name=$1`, name).Scan(&v); err != nil || v == nil {
		return def
	}
	return *v
}

func (s *svc) emailTaken(ctx context.Context, q legacy.DB, email string) (bool, error) {
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM "user" WHERE lower(email)=$1`, strings.ToLower(email)).Scan(&n)
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
	err = legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, 0, false, func(tx pgx.Tx, _ *legacy.Files, _ *legacy.LockedUser) error {
		if taken, err := s.emailTaken(ctx, tx, email); err != nil {
			return err
		} else if taken {
			return legacy.Bad()
		}
		var aiDefault bool
		_ = tx.QueryRow(ctx, `SELECT coalesce((desired->>'new_user_default')::boolean, false) FROM ai_control_state WHERE name='global'`).Scan(&aiDefault)
		now := legacy.Now()
		err := tx.QueryRow(ctx, `INSERT INTO "user"(user_name,email,password,role,uuid,create_date,group_id,used_bytes,ai_search_enabled)
			VALUES ($1,$2,$3,'user',$4,$5,(SELECT id FROM user_group WHERE is_default),0,$6) RETURNING user_id`,
			name, email, legacy.HashPassword(password), legacy.NewUUID(), now, aiDefault).Scan(&uid)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO namespace(ns_name,ns_parent_id,user_id) VALUES ('/',-1,$1)`, uid); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO gallery(ga_name,user_id,create_date,cover_id,cover_color) VALUES ('My Favorite',$1,$2,2,'#FFFFFF')`, uid, now)
		return err
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
	var uid int64
	var hash *string
	err = s.a.Pool.QueryRow(ctx, `SELECT user_id, password FROM "user" WHERE lower(email)=$1`, strings.ToLower(em)).Scan(&uid, &hash)
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
	var id int64
	var name, email, role, uuid *string
	var created *time.Time
	err := s.a.Pool.QueryRow(r.Context(), `SELECT user_id,user_name,email,create_date,role,uuid FROM "user" WHERE user_id=$1`, uid).
		Scan(&id, &name, &email, &created, &role, &uuid)
	if err != nil {
		return nil, err
	}
	return legacy.Res{V: map[string]any{"userId": id, "userName": name, "email": email, "createDate": legacy.ISO(created),
		"role": role, "password": nil, "uuid": uuid}, Code: 200}, nil
}

func (s *svc) storage(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	ctx := r.Context()
	if err := legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uid, false, func(pgx.Tx, *legacy.Files, *legacy.LockedUser) error { return nil }); err != nil {
		return nil, err
	}
	var used *int64
	var override bool
	var max, glimit *int64
	var gid int64
	var gname string
	err := s.a.Pool.QueryRow(ctx, `SELECT u.used_bytes,u.storage_override,u.max_storage_bytes,g.id,g.name,g.max_storage_bytes
		FROM "user" u JOIN user_group g ON g.id=u.group_id WHERE u.user_id=$1`, uid).Scan(&used, &override, &max, &gid, &gname, &glimit)
	if err != nil {
		return nil, err
	}
	limit, source := glimit, "group"
	if override {
		limit, source = max, "user"
	}
	var u int64
	if used != nil {
		u = *used
	}
	return legacy.Res{V: map[string]any{"usedBytes": u, "capacityBytes": limit, "groupId": gid, "groupName": gname,
		"storageOverride": override, "maxStorageBytes": max, "quotaSource": source,
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
				var hash *string
				if err := tx.QueryRow(ctx, `SELECT password FROM "user" WHERE user_id=$1`, uid).Scan(&hash); err != nil {
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
			_, err := tx.Exec(ctx, `UPDATE "user" SET `+field+`=$2 WHERE user_id=$1`, uid, val)
			return err
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
		p, err := legacy.MediaPath(s.a.Cfg.UploadFolder, u.UUID, u.UUID+".jpg")
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
	var uuid string
	if err := s.a.Pool.QueryRow(r.Context(), `SELECT uuid FROM "user" WHERE user_id=$1`, uid).Scan(&uuid); err != nil {
		return nil, err
	}
	p, err := legacy.MediaPath(s.a.Cfg.UploadFolder, uuid, uuid+".jpg")
	if err != nil {
		return nil, err
	}
	if st, e := os.Stat(p); e == nil && st.Mode().IsRegular() {
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
	err := legacy.Atomic(ctx, s.a.Pool, root, uid, false, func(tx pgx.Tx, f *legacy.Files, u *legacy.LockedUser) error {
		rows, err := tx.Query(ctx, `SELECT photo_id FROM photo WHERE user_id=$1 ORDER BY photo_id`, uid)
		if err != nil {
			return err
		}
		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
		n, err := legacy.DeletePhotos(ctx, tx, f, root, u, ids)
		if err != nil {
			return err
		}
		count += n
		for _, t := range []string{"namespace", "tag", "gallery"} {
			ct, err := tx.Exec(ctx, `DELETE FROM `+t+` WHERE user_id=$1`, uid)
			if err != nil {
				return err
			}
			count += int(ct.RowsAffected())
		}
		if _, err = tx.Exec(ctx, `DELETE FROM cred WHERE user_id=$1`, uid); err != nil {
			return err
		}
		p, err := legacy.MediaPath(root, u.UUID, u.UUID+".jpg")
		if err != nil {
			return err
		}
		if _, err = f.Remove(p); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM "user" WHERE user_id=$1`, uid); err != nil {
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
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}
