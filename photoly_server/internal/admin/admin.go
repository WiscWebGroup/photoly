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
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/httpx"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/photo"
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
	lock := "pg_advisory_xact_lock_shared"
	if policyWrite {
		lock = "pg_advisory_xact_lock"
	}
	must(tx.Exec(ctx, `SELECT `+lock+`(74831003)`))
	if userID != nil {
		var id int64
		if err := tx.QueryRow(ctx, `SELECT user_id FROM "user" WHERE user_id=$1 FOR UPDATE`, *userID).Scan(&id); err != nil {
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

const userCols = `user_id, user_name, email, create_date, role, uuid, group_id, storage_override, max_storage_bytes, used_bytes`

func (u *userRow) scan(r pgx.Rows) error {
	return r.Scan(&u.ID, &u.Name, &u.Email, &u.Created, &u.Role, &u.UUID, &u.GroupID, &u.Override, &u.MaxBytes, &u.Used)
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

func (s *svc) users(ctx context.Context, where string, args []any, page, rows int64) ([]userRow, int64) {
	var total int64
	check(s.a.Pool.QueryRow(ctx, `SELECT count(*) FROM "user" u `+where, args...).Scan(&total))
	n := len(args)
	rs := must(s.a.Pool.Query(ctx, `SELECT `+userCols+` FROM "user" u `+where+` ORDER BY user_id OFFSET $`+strconv.Itoa(n+1)+` LIMIT $`+strconv.Itoa(n+2),
		append(args, (page-1)*rows, rows)...))
	defer rs.Close()
	var out []userRow
	for rs.Next() {
		var u userRow
		check(u.scan(rs))
		out = append(out, u)
	}
	check(rs.Err())
	return out, total
}

func (s *svc) projection(ctx context.Context, u userRow) map[string]any {
	var name string
	var groupMax *int64
	check(s.a.Pool.QueryRow(ctx, `SELECT name, max_storage_bytes FROM user_group WHERE id=$1`, u.GroupID).Scan(&name, &groupMax))
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
		n = must(tx.Exec(ctx, `UPDATE setting SET setting_value=$2 WHERE setting_name=$1`, name, strconv.FormatInt(value, 10))).RowsAffected()
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
	var exists bool
	check(tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM "user" WHERE email=$1)`, email).Scan(&exists))
	if exists {
		throw(-1, 400)
	}
	aiDefault := false
	_ = tx.QueryRow(ctx, `SELECT COALESCE((desired->>'new_user_default')::boolean, false) FROM ai_control_state WHERE name='global'`).Scan(&aiDefault)
	loc, err := time.LoadLocation(s.a.Cfg.Timezone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	naive := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second(), 0, time.UTC)
	var name any
	if v, ok := data["userName"]; ok {
		name = v
	}
	var uid int64
	check(tx.QueryRow(ctx, `INSERT INTO "user" (user_name, email, password, role, uuid, create_date, group_id, used_bytes, ai_search_enabled)
		VALUES ($1,$2,$3,$4,$5,$6,(SELECT id FROM user_group WHERE is_default LIMIT 1),0,$7) RETURNING user_id`,
		name, email, legacy.HashPassword(password), role, photo.NewUUID(), naive, aiDefault).Scan(&uid))
	must(tx.Exec(ctx, `INSERT INTO namespace (ns_name, ns_parent_id, user_id) VALUES ('/', -1, $1)`, uid))
	must(tx.Exec(ctx, `INSERT INTO gallery (ga_name, user_id, create_date, cover_id, cover_color) VALUES ('My Favorite', $1, $2, 2, '#FFFFFF')`, uid, naive))
}

func validName(v any) string {
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" || len([]rune(s)) > 255 || s == "." || s == ".." || strings.ContainsAny(s, "/\\\x00\r\n") {
		throw(-1, 400)
	}
	return s
}

func storageLimit(v any) any {
	if v == nil {
		return nil
	}
	n, ok := integer(v)
	if !ok || n < 0 || n > 9007199254740991 {
		throw("invalid_storage_limit", 400)
	}
	return n
}

// Register mounts the /admin routes.
func Register(a *app.App) {
	s := &svc{a}
	m := a.Mux
	get := func(p string, missing any, page bool, fn func(w http.ResponseWriter, r *http.Request, u app.User)) {
		m.HandleFunc("GET "+p, s.legacy(missing, page, fn))
	}
	post := func(p string, missing any, fn func(w http.ResponseWriter, r *http.Request, u app.User)) {
		m.HandleFunc("POST "+p, s.legacy(missing, false, fn))
	}
	ping := func(w http.ResponseWriter, r *http.Request, u app.User) {
		requireAdmin(u)
		var one int64
		check(a.Pool.QueryRow(r.Context(), `SELECT 1`).Scan(&one))
		changed(w, one)
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
		var version string
		check(a.Pool.QueryRow(ctx, `SHOW server_version`).Scan(&version))
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
		rs := must(a.Pool.Query(r.Context(), `SELECT setting_name, setting_value FROM setting ORDER BY setting_id`))
		defer rs.Close()
		out := map[string]any{}
		for rs.Next() {
			var k string
			var v *string
			check(rs.Scan(&k, &v))
			out[k] = v
		}
		check(rs.Err())
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
		where, args := "", []any(nil)
		if search != nil {
			where, args = `WHERE user_name ILIKE $1 OR email ILIKE $1`, []any{"%" + *search + "%"}
		}
		us, total := s.users(r.Context(), where, args, page, rows)
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
		var conds []string
		var args []any
		if q, _ := arg(r, "search"); q != "" {
			q = strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(q)
			args = append(args, "%"+q+"%")
			conds = append(conds, `(user_name ILIKE $`+strconv.Itoa(len(args))+` OR email ILIKE $`+strconv.Itoa(len(args))+`)`)
		}
		if hasRole {
			args = append(args, role)
			conds = append(conds, `role=$`+strconv.Itoa(len(args)))
		}
		if g, ok := intArg(r, "groupId"); ok {
			args = append(args, g)
			conds = append(conds, `group_id=$`+strconv.Itoa(len(args)))
		}
		where := ""
		if len(conds) > 0 {
			where = "WHERE " + strings.Join(conds, " AND ")
		}
		us, total := s.users(r.Context(), where, args, page, rows)
		out := []map[string]any{}
		for _, row := range us {
			item := row.legacy()
			for k, v := range s.projection(r.Context(), row) {
				item[k] = v
			}
			var n int64
			check(a.Pool.QueryRow(r.Context(), `SELECT count(*) FROM photo WHERE user_id=$1`, row.ID).Scan(&n))
			item["mediaCount"] = n
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
		var uid string
		if err := a.Pool.QueryRow(ctx, `SELECT uuid FROM "user" WHERE user_id=$1`, id).Scan(&uid); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				changed(w, -5)
				return
			}
			panic(err)
		}
		var count int64
		s.atomic(ctx, false, &id, func(tx pgx.Tx) {
			for _, q := range []string{`DELETE FROM photo WHERE user_id=$1`, `DELETE FROM namespace WHERE user_id=$1`, `DELETE FROM tag WHERE user_id=$1`,
				`DELETE FROM gallery WHERE user_id=$1`, `DELETE FROM cred WHERE user_id=$1`, `DELETE FROM "user" WHERE user_id=$1`} {
				count += must(tx.Exec(ctx, q, id)).RowsAffected()
			}
		})
		if uid != "" && !strings.ContainsAny(uid, "/\\") && uid != "." && uid != ".." {
			_ = os.RemoveAll(filepath.Join(a.Cfg.UploadFolder, uid))
		}
		_, _ = a.Pool.Exec(ctx, `DELETE FROM photoly_session WHERE user_id=$1`, id)
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
				var cur int64
				if !hasID || tx.QueryRow(ctx, `SELECT user_id FROM "user" WHERE user_id=$1 FOR UPDATE`, id).Scan(&cur) != nil {
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
					var other int64
					if err := tx.QueryRow(ctx, `SELECT user_id FROM "user" WHERE email=$1`, v).Scan(&other); err == nil && other != id {
						throw(-1, 400)
					}
				}
				must(tx.Exec(ctx, `UPDATE "user" SET `+c.col+`=$2 WHERE user_id=$1`, id, v))
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
		var v *string
		err := a.Pool.QueryRow(r.Context(), `SELECT setting_value FROM setting WHERE setting_name=$1`, name).Scan(&v)
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
				n = must(tx.Exec(r.Context(), `UPDATE setting SET setting_value=$2 WHERE setting_name=$1`, name, val)).RowsAffected()
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
		rs := must(a.Pool.Query(r.Context(), `SELECT g.id, g.name, g.max_storage_bytes, g.is_default, (SELECT count(*) FROM "user" WHERE group_id=g.id) FROM user_group g ORDER BY g.id`))
		defer rs.Close()
		out := []map[string]any{}
		for rs.Next() {
			var id, members int64
			var name string
			var max *int64
			var def bool
			check(rs.Scan(&id, &name, &max, &def, &members))
			out = append(out, map[string]any{"id": id, "name": name, "maxStorageBytes": max, "isDefault": def, "memberCount": members})
		}
		check(rs.Err())
		httpx.JSON(w, out, 200)
	}))
	save := func(w http.ResponseWriter, r *http.Request, u app.User, id *int64) {
		requireAdmin(u)
		d := jsonBody(r)
		ctx := r.Context()
		var out int64
		s.atomic(ctx, true, nil, func(tx pgx.Tx) {
			if id != nil {
				var x int64
				if err := tx.QueryRow(ctx, `SELECT id FROM user_group WHERE id=$1 FOR UPDATE`, *id).Scan(&x); err != nil {
					throw("group_not_found", 404)
				}
			}
			name, limit := validName(d["name"]), storageLimit(d["maxStorageBytes"])
			if id != nil {
				must(tx.Exec(ctx, `UPDATE user_group SET name=$2, max_storage_bytes=$3 WHERE id=$1`, *id, name, limit))
				out = *id
			} else {
				check(tx.QueryRow(ctx, `INSERT INTO user_group (name, max_storage_bytes) VALUES ($1,$2) RETURNING id`, name, limit).Scan(&out))
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
			var isDefault bool
			var x int64
			if !ok || target == id || tx.QueryRow(ctx, `SELECT is_default FROM user_group WHERE id=$1`, id).Scan(&isDefault) != nil || isDefault ||
				tx.QueryRow(ctx, `SELECT id FROM user_group WHERE id=$1`, target).Scan(&x) != nil {
				throw("invalid_group_reassignment", 400)
			}
			must(tx.Exec(ctx, `UPDATE "user" SET group_id=$2 WHERE group_id=$1`, id, target))
			must(tx.Exec(ctx, `DELETE FROM user_group WHERE id=$1`, id))
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
			var x int64
			if !gok || !bok || tx.QueryRow(ctx, `SELECT id FROM user_group WHERE id=$1`, g).Scan(&x) != nil {
				throw("invalid_storage_settings", 400)
			}
			var limit any
			if override {
				limit = storageLimit(d["maxStorageBytes"])
			}
			must(tx.Exec(ctx, `UPDATE "user" SET group_id=$2, storage_override=$3, max_storage_bytes=$4 WHERE user_id=$1`, target, g, override, limit))
			rs := must(tx.Query(ctx, `SELECT `+userCols+` FROM "user" u WHERE user_id=$1`, target))
			defer rs.Close()
			if rs.Next() {
				check(row.scan(rs))
			}
		})
		httpx.JSON(w, s.projection(ctx, row), 200)
	}))
}
