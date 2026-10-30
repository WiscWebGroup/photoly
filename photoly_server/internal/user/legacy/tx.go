package legacy

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB is satisfied by *pgxpool.Pool and pgx.Tx.
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Files struct{ *storage.Files }

func (f *Files) Install(r io.Reader, p string) error { _, e := f.Files.Install(r, p); return e }

// LockedUser is the row locked by Atomic.
type LockedUser struct {
	ID    int64
	UUID  string
	Used  int64
	Gone  bool // set by fn when it deleted the user, skipping the quota update
	Limit *int64
}

func (u *LockedUser) Dir(root string) string { return filepath.Join(root, u.UUID) }

// Usage is LocalStorage.usage: bytes in a user's media folder.
func Usage(root, uuid string) int64 { return storage.Usage(root, uuid) }

// Atomic is BaseService.atomic. uid==0 takes no user lock. Quota is enforced on the
// file delta for the locked user.
func Atomic(ctx context.Context, pool *pgxpool.Pool, root string, uid int64, policyWrite bool,
	fn func(tx pgx.Tx, f *Files, u *LockedUser) error) (err error) {
	base, err := storage.NewFiles(root)
	if err != nil {
		return err
	}
	f := &Files{base}
	defer f.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			f.Rollback()
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()
	lock := "pg_advisory_xact_lock_shared"
	if policyWrite {
		lock = "pg_advisory_xact_lock"
	}
	if _, err = tx.Exec(ctx, "SELECT "+lock+"(74831003)"); err != nil {
		return err
	}
	var u *LockedUser
	if uid != 0 {
		u = &LockedUser{ID: uid}
		var used *int64
		var override bool
		var max *int64
		var group int64
		e := tx.QueryRow(ctx, `SELECT uuid, used_bytes, storage_override, max_storage_bytes, group_id
			FROM "user" WHERE user_id=$1 FOR UPDATE`, uid).Scan(&u.UUID, &used, &override, &max, &group)
		if e == pgx.ErrNoRows {
			err = Bad()
			return err
		} else if e != nil {
			err = e
			return err
		}
		if used == nil {
			u.Used = Usage(root, u.UUID)
		} else {
			u.Used = *used
		}
		if override {
			u.Limit = max
		} else if err = tx.QueryRow(ctx, `SELECT max_storage_bytes FROM user_group WHERE id=$1`, group).Scan(&u.Limit); err != nil {
			return err
		}
		if used == nil {
			if _, err = tx.Exec(ctx, `UPDATE "user" SET used_bytes=$2 WHERE user_id=$1`, uid, u.Used); err != nil {
				return err
			}
		}
	}
	var before int64
	if u != nil {
		before = Usage(root, u.UUID)
	}
	if err = fn(tx, f, u); err != nil {
		return err
	}
	if u != nil && !u.Gone {
		after := Usage(root, u.UUID)
		delta := after - before
		if delta > 0 && u.Limit != nil && after > *u.Limit {
			err = Err("storage_quota_exceeded", 413)
			return err
		}
		n := after
		if n < 0 {
			n = 0
		}
		if _, err = tx.Exec(ctx, `UPDATE "user" SET used_bytes=$2 WHERE user_id=$1`, uid, n); err != nil {
			return err
		}
	}
	if err = f.Flush(ctx); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	f.Commit()
	return nil
}

// Now is BaseService.now: wall clock in the configured timezone, naive, whole seconds.
func Now() time.Time {
	name := os.Getenv("TIMEZONE")
	if name == "" {
		name = "America/New_York"
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		loc = time.UTC
	}
	t := time.Now().In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
}

// MediaPath resolves legacy public asset names through the source model.
func MediaPath(ctx context.Context, q DB, root, uuid, name string) (string, error) {
	return storage.ResolveLegacyName(ctx, q, root, uuid, name)
}
