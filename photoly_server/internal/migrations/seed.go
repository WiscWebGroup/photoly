package migrations

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	"github.com/jackc/pgx/v5"
)

var defaultSettings = [][3]string{
	{"OnlineDownload", "0", "Allow public image URL imports"},
	{"ping", "pong", "use for testing connection"},
	{"SignUp", "1", "use for determine if new user could sign up"},
	{"SSafeUUID", "1", "use for determine if SSafe is enabled"},
	{"TokenDuration", "30", "use for determine the duration of token"},
}

// Seed is `flask seed-db` plus `reconcile-storage`: default settings, the administrator
// and used_bytes for accounts that have none. Idempotent.
func Seed(ctx context.Context, conn *pgx.Conn, cfg app.Config) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(74831002)`); err != nil {
		return err
	}
	for _, s := range defaultSettings {
		if _, err := tx.Exec(ctx, `INSERT INTO setting (setting_name,setting_value,note) VALUES ($1,$2,$3) ON CONFLICT (setting_name) DO NOTHING`, s[0], s[1], s[2]); err != nil {
			return err
		}
	}
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		loc = time.UTC
	}
	n := time.Now().In(loc) // naive local wall clock, like Flask
	now := time.Date(n.Year(), n.Month(), n.Day(), n.Hour(), n.Minute(), n.Second(), n.Nanosecond(), time.UTC)
	var admin int64
	err = tx.QueryRow(ctx, `SELECT user_id FROM "user" WHERE lower(email)=lower($1)`, cfg.AdminEmail).Scan(&admin)
	if errors.Is(err, pgx.ErrNoRows) {
		if cfg.AdminPassword == "" {
			return errors.New("set ADMIN_PASSWORD before initializing a fresh deployment")
		}
		err = tx.QueryRow(ctx, `INSERT INTO "user" (email,password,user_name,role,uuid,create_date,used_bytes,group_id)
			VALUES ($1,$2,'admin','admin',$3,$4,0,(SELECT id FROM user_group WHERE is_default LIMIT 1)) RETURNING user_id`,
			cfg.AdminEmail, legacy.HashPassword(cfg.AdminPassword), legacy.NewUUID(), now).Scan(&admin)
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO namespace (ns_name,ns_parent_id,user_id)
		SELECT '/',-1,$1 WHERE NOT EXISTS (SELECT 1 FROM namespace WHERE user_id=$1 AND ns_parent_id=-1)`, admin); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO gallery (ga_name,user_id,create_date,cover_id,cover_color)
		SELECT 'My Favorite',$1,$2,2,'#FFFFFF' WHERE NOT EXISTS (SELECT 1 FROM gallery WHERE user_id=$1 AND ga_name='My Favorite')`, admin, now); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT user_id, uuid FROM "user" WHERE used_bytes IS NULL`)
	if err != nil {
		return err
	}
	type u struct {
		id   int64
		uuid string
	}
	var pending []u
	for rows.Next() {
		var x u
		if err := rows.Scan(&x.id, &x.uuid); err != nil {
			return err
		}
		pending = append(pending, x)
	}
	rows.Close()
	for _, x := range pending {
		if _, err := tx.Exec(ctx, `UPDATE "user" SET used_bytes=$2 WHERE user_id=$1`, x.id, dirSize(filepath.Join(cfg.UploadFolder, x.uuid))); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func dirSize(root string) (n int64) {
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, e := d.Info(); e == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return
}
