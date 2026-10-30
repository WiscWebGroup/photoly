// Package admin ports controllers/admin.py and services/{admin,quota}.py: user management,
// settings and storage groups. Responses keep the legacy {t, msgCode} envelope.
package admin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/httpx"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/photo"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// biz is a BusinessError: it unwinds a handler into {t:value, msgCode:code}.
type biz struct {
	value any
	code  int
}

func throw(value any, code int) { panic(biz{value, code}) }

type svc struct{ a *app.App }

// legacy wraps fn like controllers/common.py legacy(): auth failure answers {t:missing, msgCode:403}.
func (s *svc) legacy(missing any, page bool, fn func(w http.ResponseWriter, r *http.Request, u app.User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		called := false
		s.a.Auth(func(_ http.ResponseWriter, r *http.Request) {
			called = true
			defer func() {
				v := recover()
				if v == nil {
					return
				}
				var b biz
				switch e := v.(type) {
				case biz:
					b = e
				case error:
					var pe *pgconn.PgError
					if !errors.As(e, &pe) || pe.Code[:2] != "23" {
						panic(v)
					}
					b = biz{-1, 400}
				default:
					panic(v)
				}
				if page {
					httpx.Page(w, nil, 0, b.code)
				} else {
					httpx.JSON(w, b.value, b.code)
				}
			}()
			fn(w, r, s.a.UserFrom(r.Context()))
		})(&sink{http.Header{}}, r)
		if !called {
			if page {
				httpx.Page(w, missing, 0, 403)
			} else {
				httpx.JSON(w, missing, 403)
			}
		}
	}
}

type sink struct{ h http.Header }

func (d *sink) Header() http.Header         { return d.h }
func (d *sink) Write(b []byte) (int, error) { return len(b), nil }
func (d *sink) WriteHeader(int)             {}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func changed(w http.ResponseWriter, v int64) {
	code := 200
	if v < 1 {
		code = 400
	}
	httpx.JSON(w, v, code)
}

func requireAdmin(u app.User) {
	if u.Role != "admin" {
		throw(-2, 400)
	}
}

// arg reads a query or form value.
func arg(r *http.Request, name string) (string, bool) {
	_ = r.ParseForm()
	v, ok := r.Form[name]
	if !ok || len(v) == 0 {
		return "", false
	}
	return v[0], true
}

func intArg(r *http.Request, name string) (int64, bool) {
	v, ok := arg(r, name)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		throw(-1, 400)
	}
	return n, true
}

func jsonBody(r *http.Request) map[string]any {
	var m map[string]any
	if json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(&m) != nil || m == nil {
		throw(-1, 400)
	}
	return m
}

// integer is Python's `type(v) is int`.
func integer(v any) (int64, bool) {
	f, ok := v.(float64)
	if !ok || f != math.Trunc(f) || math.Abs(f) > 1<<53 {
		return 0, false
	}
	return int64(f), true
}

type txFn func(tx pgx.Tx)

// atomic runs fn in a transaction under the policy lock (shared, or exclusive for quota/group edits),
// optionally holding one user's row lock. A panic rolls it back.
func (s *svc) atomic(ctx context.Context, policyWrite bool, userID *int64, fn txFn) {
	tx := must(s.a.Pool.Begin(ctx))
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	if policyWrite {
		check(q.LockPolicyExclusive(ctx))
	} else {
		check(q.LockPolicyShared(ctx))
	}
	if userID != nil {
		if _, err := q.LockUser(ctx, *userID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				throw(-1, 400)
			}
			panic(err)
		}
	}
	fn(tx)
	check(tx.Commit(ctx))
}

func (s *svc) hostPort(def string) string {
	u, err := url.Parse(s.a.Cfg.DatabaseURL)
	if err != nil || u.Hostname() == "" {
		return ""
	}
	p := u.Port()
	if p == "" {
		p = def
	}
	return u.Hostname() + ":" + p
}

type userRow struct {
	ID                      int64
	Name, Email, Role, UUID *string
	Created                 *time.Time
	GroupID                 int64
	Override                bool
	MaxBytes, Used          *int64
}

func userFromList(r dbgen.ListUsersRow) userRow {
	return userRow{r.UserID, r.UserName, r.Email, r.Role, r.Uuid, r.CreateDate, r.GroupID, r.StorageOverride, r.MaxStorageBytes, r.UsedBytes}
}

func userFromGet(r dbgen.GetAdminUserRow) userRow {
	return userRow{r.UserID, r.UserName, r.Email, r.Role, r.Uuid, r.CreateDate, r.GroupID, r.StorageOverride, r.MaxStorageBytes, r.UsedBytes}
}

func (u *userRow) legacy() map[string]any {
	var created any
	if u.Created != nil {
		layout := "2006-01-02T15:04:05"
		if u.Created.Nanosecond() != 0 {
			layout = "2006-01-02T15:04:05.000000"
		}
		created = u.Created.Format(layout)
	}
	return map[string]any{"userId": u.ID, "userName": u.Name, "email": u.Email, "createDate": created, "role": u.Role, "password": nil, "uuid": u.UUID}
}

// userFilter narrows the user list; nil fields match everything.
type userFilter struct {
	search  *string // ILIKE pattern, wildcards included
	role    *string
	groupID *int64
}

func (s *svc) users(ctx context.Context, f userFilter, page, rows int64) ([]userRow, int64) {
	q := dbgen.New(s.a.Pool)
	total := must(q.CountUsers(ctx, dbgen.CountUsersParams{Search: f.search, Role: f.role, GroupID: f.groupID}))
	list := must(q.ListUsers(ctx, dbgen.ListUsersParams{Search: f.search, Role: f.role, GroupID: f.groupID, RowOffset: (page - 1) * rows, RowLimit: rows}))
	out := make([]userRow, len(list))
	for i, r := range list {
		out[i] = userFromList(r)
	}
	return out, total
}

func (s *svc) projection(ctx context.Context, u userRow) map[string]any {
	group := must(dbgen.New(s.a.Pool).GetUserGroup(ctx, u.GroupID))
	name, groupMax := group.Name, group.MaxStorageBytes
	limit, source := groupMax, "group"
	if u.Override {
		limit, source = u.MaxBytes, "user"
	}
	used := int64(0)
	if u.Used != nil {
		used = *u.Used
	}
	var cap any
	if limit != nil {
		cap = *limit
	}
	var mx any
	if u.MaxBytes != nil {
		mx = *u.MaxBytes
	}
	return map[string]any{"usedBytes": used, "capacityBytes": cap, "groupId": u.GroupID, "groupName": name, "storageOverride": u.Override,
		"maxStorageBytes": mx, "quotaSource": source, "overQuota": limit != nil && used > *limit}
}

func (s *svc) setSetting(ctx context.Context, u app.User, name string, value int64) int64 {
	requireAdmin(u)
	if name == "TokenDuration" && (value < 0 || value > 365) {
		throw(-1, 403)
	}
	if name != "TokenDuration" && value != 0 && value != 1 {
		throw(-1, 400)
	}
	var n int64
	s.atomic(ctx, name == "OnlineDownload", nil, func(tx pgx.Tx) {
		n = must(dbgen.New(tx).UpdateSetting(ctx, dbgen.UpdateSettingParams{Name: name, Value: strconv.FormatInt(value, 10)}))
	})
	if n == 0 {
		return 0
	}
	return 1
}

var emailRe = regexp.MustCompile(`^.+@.+\..+$`)

func (s *svc) createUser(ctx context.Context, tx pgx.Tx, data map[string]any, role string) {
	email, ok1 := data["email"].(string)
	password, ok2 := data["password"].(string)
	if !ok1 || !emailRe.MatchString(email) || !ok2 || password == "" {
		throw(-1, 400)
	}
	q := dbgen.New(tx)
	if must(q.EmailExists(ctx, email)) {
		throw(-1, 400)
	}
	aiDefault, _ := q.NewUserAIDefault(ctx)
	loc, err := time.LoadLocation(s.a.Cfg.Timezone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	naive := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second(), 0, time.UTC)
	var name *string
	if v, ok := data["userName"].(string); ok {
		name = &v
	} else if data["userName"] != nil {
		throw(-1, 400)
	}
	uid := must(q.InsertUser(ctx, dbgen.InsertUserParams{UserName: name, Email: email, Password: legacy.HashPassword(password), Role: role,
		Uuid: photo.NewUUID(), CreateDate: naive, AiSearchEnabled: aiDefault}))
	check(q.InsertRootNamespace(ctx, uid))
	accountUUID := must(q.GetUserUUID(ctx, uid))
	dir := must(storage.SafePath(s.a.Cfg.UploadFolder, deref(accountUUID)))
	check(os.MkdirAll(dir, 0755))
	check(storage.RecordRootIdentity(ctx, tx, uid, dir))
	check(q.InsertFavoriteGallery(ctx, dbgen.InsertFavoriteGalleryParams{UserID: uid, CreateDate: naive}))
}

func validName(v any) string {
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" || len([]rune(s)) > 255 || s == "." || s == ".." || strings.ContainsAny(s, "/\\\x00\r\n") {
		throw(-1, 400)
	}
	return s
}

func storageLimit(v any) *int64 {
	if v == nil {
		return nil
	}
	n, ok := integer(v)
	if !ok || n < 0 || n > 9007199254740991 {
		throw("invalid_storage_limit", 400)
	}
	return &n
}

// Register mounts the /admin routes.
func Register(a *app.App) {
	s := &svc{a}
	s.registerLocalStorage()
	m := a.Mux

	get := func(p string, missing any, page bool, fn func(w http.ResponseWriter, r *http.Request, u app.User)) {
		m.HandleFunc("GET "+p, s.legacy(missing, page, fn))
	}
	post := func(p string, missing any, fn func(w http.ResponseWriter, r *http.Request, u app.User)) {
		m.HandleFunc("POST "+p, s.legacy(missing, false, fn))
	}
	ping := func(w http.ResponseWriter, r *http.Request, u app.User) {
		requireAdmin(u)
		changed(w, must(dbgen.New(a.Pool).Ping(r.Context())))
	}
	get("/admin/isAdmin", false, false, func(w http.ResponseWriter, r *http.Request, u app.User) {
		httpx.JSON(w, u.Role == "admin", 200)
	})
	get("/admin/pingMySQL", -2, false, ping)
	get("/admin/pingDatabase", -2, false, ping)
	get("/admin/pingRedis", -2, false, ping) // sessions live in PostgreSQL now: the database ping is the answer
	get("/admin/databaseInfo", nil, false, func(w http.ResponseWriter, r *http.Request, u app.User) {
		requireAdmin(u)
		ctx := r.Context()
		version := must(dbgen.New(a.Pool).ServerVersion(ctx))
		// pg_extension and goose_db_version live outside the sqlc schema, so these two stay raw SQL.
		var vec, rev *string
		_ = a.Pool.QueryRow(ctx, `SELECT extversion FROM pg_extension WHERE extname='vector'`).Scan(&vec)
		if a.Pool.QueryRow(ctx, `SELECT max(version_id)::text FROM goose_db_version`).Scan(&rev) != nil {
			rev = nil
		}
		httpx.JSON(w, map[string]any{"type": "postgresql", "version": version, "address": s.hostPort("5432"), "pgvectorVersion": vec, "schemaRevision": rev}, 200)
	})
	get("/admin/getAddress", nil, false, func(w http.ResponseWriter, r *http.Request, u app.User) {
		if u.Role != "admin" {
			httpx.JSON(w, nil, 400)
			return
		}
		httpx.JSON(w, map[string]any{"mysql": s.hostPort("5432"), "redis": "in-process", "upload": a.Cfg.UploadFolder, "config": "PostgreSQL: <setting> table"}, 200)
	})
	get("/admin/getSettings", nil, false, func(w http.ResponseWriter, r *http.Request, u app.User) {
		if u.Role != "admin" {
			httpx.JSON(w, nil, 400)
			return
		}
		out := map[string]any{}
		for _, row := range must(dbgen.New(a.Pool).ListSettings(r.Context())) {
			out[deref(row.SettingName)] = row.SettingValue
		}
		httpx.JSON(w, out, 200)
	})
	post("/admin/stop", -2, func(w http.ResponseWriter, r *http.Request, u app.User) {
		if u.Role != "admin" {
			httpx.JSON(w, -2, 591023)
			return
		}
		httpx.JSON(w, 1, 591023)
		go func() {
			time.Sleep(300 * time.Millisecond)
			_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
		}()
	})
	for _, c := range []struct{ path, name, key string }{{"/admin/setSignUpPermission", "SignUp", "permission"}, {"/admin/setSSafeUUIDMode", "SSafeUUID", "mode"}, {"/admin/setTokenDuration", "TokenDuration", "days"}} {
		c := c
		post(c.path, -2, func(w http.ResponseWriter, r *http.Request, u app.User) {
			n, ok := intArg(r, c.key)
			if !ok && u.Role == "admin" {
				// Python's int(None) raises inside set_setting: BusinessError(-1).
				throw(-1, 400)
			}
			changed(w, s.setSetting(r.Context(), u, c.name, n))
		})
	}
	paged := func(w http.ResponseWriter, r *http.Request, u app.User, search *string) {
		requireAdmin(u)
		page, ok1 := intArg(r, "page")
		rows, ok2 := intArg(r, "rowsPerPage")
		if !ok1 || !ok2 || page < 1 || rows < 1 {
			throw(-1, 400)
		}
		var f userFilter
		if search != nil {
			pattern := "%" + *search + "%"
			f.search = &pattern
		}
		us, total := s.users(r.Context(), f, page, rows)
		out := []map[string]any{}
		for i := range us {
			out = append(out, us[i].legacy())
		}
		httpx.Page(w, out, int((total+rows-1)/rows), 200)
	}
	get("/admin/getUserPage", nil, true, func(w http.ResponseWriter, r *http.Request, u app.User) { paged(w, r, u, nil) })
	get("/admin/searchUserPage", nil, true, func(w http.ResponseWriter, r *http.Request, u app.User) {
		q, ok := arg(r, "search")
		var p *string
		if ok {
			p = &q
		}
		paged(w, r, u, p)
	})
	get("/admin/getUsers", -2, false, func(w http.ResponseWriter, r *http.Request, u app.User) {
		requireAdmin(u)
		page, ok1 := intArg(r, "page")
		rows, ok2 := intArg(r, "rowsPerPage")
		role, hasRole := arg(r, "role")
		if !ok1 || !ok2 || page < 1 || rows < 1 || rows > 200 || (hasRole && role != "admin" && role != "user") {
			throw(-1, 400)
		}
		var f userFilter
		if q, _ := arg(r, "search"); q != "" {
			q = strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(q)
			pattern := "%" + q + "%"
			f.search = &pattern
		}
		if hasRole {
			f.role = &role
		}
		if g, ok := intArg(r, "groupId"); ok {
			f.groupID = &g
		}
		us, total := s.users(r.Context(), f, page, rows)
		out := []map[string]any{}
		for _, row := range us {
			item := row.legacy()
			for k, v := range s.projection(r.Context(), row) {
				item[k] = v
			}
			item["mediaCount"] = must(dbgen.New(a.Pool).CountUserPhotos(r.Context(), row.ID))
			out = append(out, item)
		}
		httpx.JSON(w, map[string]any{"users": out, "total": total}, 200)
	})
	post("/admin/addUser", -2, func(w http.ResponseWriter, r *http.Request, u app.User) {
		requireAdmin(u)
		d := jsonBody(r)
		if d["userName"] == nil {
			throw(-1, 400)
		}
		role := "user"
		if v, _ := d["role"].(string); v == "admin" {
			role = "admin"
		}
		s.atomic(r.Context(), false, nil, func(tx pgx.Tx) { s.createUser(r.Context(), tx, d, role) })
		changed(w, 1)
	})
	post("/admin/deleteUser", -2, func(w http.ResponseWriter, r *http.Request, u app.User) {
		requireAdmin(u)
		id, ok := intArg(r, "userId")
		if !ok {
			changed(w, -2)
			return
		}
		ctx := r.Context()
		uuid, err := dbgen.New(a.Pool).GetUserUUID(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			changed(w, -5)
			return
		}
		check(err)
		_ = uuid
		var count int64
		check(legacy.Atomic(storage.ExclusiveUser(ctx), a.Pool, a.Cfg.UploadFolder, id, false, func(tx pgx.Tx, f *legacy.Files, u *legacy.LockedUser) error {
			q := dbgen.New(tx)
			ids, err := q.ListUserPhotoIDs(ctx, id)
			if err != nil {
				return err
			}
			if _, err = legacy.DeletePhotos(storage.PreserveExternal(ctx), tx, f, a.Cfg.UploadFolder, u, ids); err != nil {
				return err
			}
			if err = legacy.RemoveUserFiles(f, a.Cfg.UploadFolder, u.UUID); err != nil {
				return err
			}
			for _, del := range []func(context.Context, int64) (int64, error){q.DeleteUserPhotos, q.DeleteUserNamespaces, q.DeleteUserTags, q.DeleteUserGalleries, q.DeleteUserCreds, q.DeleteUser} {
				n, e := del(ctx, id)
				if e != nil {
					return e
				}
				count += n
			}
			u.Gone = true
			return nil
		}))
		_ = dbgen.New(a.Pool).DeleteUserSessions(ctx, id)
		changed(w, count)
	})
	for _, c := range []struct{ path, key, col string }{{"/admin/resetUserPassword", "password", "password"}, {"/admin/resetUserEmail", "email", "email"},
		{"/admin/resetUserRole", "role", "role"}, {"/admin/resetUsername", "userName", "user_name"}} {
		c := c
		post(c.path, -2, func(w http.ResponseWriter, r *http.Request, u app.User) {
			requireAdmin(u)
			d := jsonBody(r)
			var res int64
			ctx := r.Context()
			id, hasID := integer(d["userId"])
			s.atomic(ctx, false, nil, func(tx pgx.Tx) {
				q := dbgen.New(tx)
				if !hasID {
					return
				}
				if _, err := q.LockUser(ctx, id); err != nil {
					return
				}
				v, ok := d[c.key].(string)
				if !ok || v == "" {
					throw(-1, 400)
				}
				switch c.col {
				case "password":
					v = legacy.HashPassword(v)
				case "role":
					if v != "user" && v != "admin" {
						throw(-1, 400)
					}
				case "email":
					if other, err := q.GetUserIDByEmail(ctx, v); err == nil && other != id {
						throw(-1, 400)
					}
				}
				switch c.col {
				case "password":
					check(q.UpdateUserPassword(ctx, dbgen.UpdateUserPasswordParams{UserID: id, Value: &v}))
				case "email":
					check(q.UpdateUserEmail(ctx, dbgen.UpdateUserEmailParams{UserID: id, Value: &v}))
				case "role":
					check(q.UpdateUserRole(ctx, dbgen.UpdateUserRoleParams{UserID: id, Value: &v}))
				default:
					check(q.UpdateUserName(ctx, dbgen.UpdateUserNameParams{UserID: id, Value: &v}))
				}
				res = 1
			})
			changed(w, res)
		})
	}
	post("/admin/getGeneralSetting", nil, func(w http.ResponseWriter, r *http.Request, u app.User) {
		if u.Role != "admin" {
			httpx.JSON(w, nil, 400)
			return
		}
		name, _ := arg(r, "name")
		v, err := dbgen.New(a.Pool).GetSetting(r.Context(), name)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			panic(err)
		}
		if v == nil {
			httpx.JSON(w, nil, 400)
			return
		}
		httpx.JSON(w, *v, 200)
	})
	post("/admin/setGeneralSetting", -2, func(w http.ResponseWriter, r *http.Request, u app.User) {
		if u.Role != "admin" {
			changed(w, -1)
			return
		}
		name, _ := arg(r, "name")
		val, ok := arg(r, "val")
		if !ok {
			throw(-1, 400)
		}
		switch name {
		case "TokenDuration", "SignUp", "SSafeUUID", "OnlineDownload":
		default:
			var n int64
			s.atomic(r.Context(), false, nil, func(tx pgx.Tx) {
				n = must(dbgen.New(tx).UpdateSetting(r.Context(), dbgen.UpdateSettingParams{Name: name, Value: val}))
			})
			changed(w, min(n, 1))
			return
		}
		n, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		if err != nil {
			throw(-1, 400)
		}
		changed(w, s.setSetting(r.Context(), u, name, n))
	})

	// ---- storage groups (services/quota.py) ----
	m.HandleFunc("GET /admin/user-groups", s.legacy(nil, false, func(w http.ResponseWriter, r *http.Request, u app.User) {
		requireAdmin(u)
		out := []map[string]any{}
		for _, g := range must(dbgen.New(a.Pool).ListUserGroups(r.Context())) {
			out = append(out, map[string]any{"id": g.ID, "name": g.Name, "maxStorageBytes": g.MaxStorageBytes, "isDefault": g.IsDefault, "memberCount": g.MemberCount})
		}
		httpx.JSON(w, out, 200)
	}))
	save := func(w http.ResponseWriter, r *http.Request, u app.User, id *int64) {
		requireAdmin(u)
		d := jsonBody(r)
		ctx := r.Context()
		var out int64
		s.atomic(ctx, true, nil, func(tx pgx.Tx) {
			q := dbgen.New(tx)
			if id != nil {
				if _, err := q.LockUserGroup(ctx, *id); err != nil {
					throw("group_not_found", 404)
				}
			}
			name, limit := validName(d["name"]), storageLimit(d["maxStorageBytes"])
			if id != nil {
				check(q.UpdateUserGroup(ctx, dbgen.UpdateUserGroupParams{ID: *id, Name: name, MaxStorageBytes: limit}))
				out = *id
			} else {
				out = must(q.InsertUserGroup(ctx, dbgen.InsertUserGroupParams{Name: name, MaxStorageBytes: limit}))
			}
		})
		httpx.JSON(w, out, 200)
	}
	m.HandleFunc("POST /admin/user-groups", s.legacy(nil, false, func(w http.ResponseWriter, r *http.Request, u app.User) { save(w, r, u, nil) }))
	m.HandleFunc("PUT /admin/user-groups/{id}", s.legacy(nil, false, func(w http.ResponseWriter, r *http.Request, u app.User) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		check(err)
		save(w, r, u, &id)
	}))
	m.HandleFunc("DELETE /admin/user-groups/{id}", s.legacy(nil, false, func(w http.ResponseWriter, r *http.Request, u app.User) {
		requireAdmin(u)
		id := must(strconv.ParseInt(r.PathValue("id"), 10, 64))
		d := jsonBody(r)
		target, ok := integer(d["reassignTo"])
		ctx := r.Context()
		s.atomic(ctx, true, nil, func(tx pgx.Tx) {
			q := dbgen.New(tx)
			if !ok || target == id {
				throw("invalid_group_reassignment", 400)
			}
			if group, err := q.GetUserGroup(ctx, id); err != nil || group.IsDefault {
				throw("invalid_group_reassignment", 400)
			}
			if _, err := q.GetUserGroup(ctx, target); err != nil {
				throw("invalid_group_reassignment", 400)
			}
			check(q.ReassignUserGroup(ctx, dbgen.ReassignUserGroupParams{FromGroup: id, ToGroup: target}))
			check(q.DeleteUserGroup(ctx, id))
		})
		httpx.JSON(w, 1, 200)
	}))
	m.HandleFunc("PUT /admin/users/{id}/storage", s.legacy(nil, false, func(w http.ResponseWriter, r *http.Request, u app.User) {
		requireAdmin(u)
		target := must(strconv.ParseInt(r.PathValue("id"), 10, 64))
		d := jsonBody(r)
		ctx := r.Context()
		var row userRow
		s.atomic(ctx, true, &target, func(tx pgx.Tx) {
			g, gok := integer(d["groupId"])
			override, bok := d["storageOverride"].(bool)
			q := dbgen.New(tx)
			if !gok || !bok {
				throw("invalid_storage_settings", 400)
			}
			if _, err := q.GetUserGroup(ctx, g); err != nil {
				throw("invalid_storage_settings", 400)
			}
			var limit *int64
			if override {
				limit = storageLimit(d["maxStorageBytes"])
			}
			check(q.UpdateUserStorage(ctx, dbgen.UpdateUserStorageParams{UserID: target, GroupID: g, StorageOverride: override, MaxStorageBytes: limit}))
			if updated, err := q.GetAdminUser(ctx, target); err == nil {
				row = userFromGet(updated)
			}
		})
		httpx.JSON(w, s.projection(ctx, row), 200)
	}))
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
