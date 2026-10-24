package legacy

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

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

// Files is FileMutation: filesystem changes compensated if the transaction fails.
type Files struct {
	Root     string
	work     string
	created  []string
	removed  [][2]string
	original map[string]int64
	n        int
}

func (f *Files) staging() error {
	if f.work != "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Join(f.Root, ".staging"), 0o755); err != nil {
		return err
	}
	w, err := os.MkdirTemp(filepath.Join(f.Root, ".staging"), "")
	f.work = w
	return err
}

func (f *Files) tmp() string { f.n++; return filepath.Join(f.work, NewUUID()) }

func size(p string) int64 {
	if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
		return st.Size()
	}
	return 0
}

// Install moves src into dest (replacing it) and returns nothing; errors abort the transaction.
func (f *Files) Install(src io.Reader, dest string) error {
	if err := f.staging(); err != nil {
		return err
	}
	t := f.tmp()
	out, err := os.Create(t)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, src); err != nil {
		out.Close()
		return err
	}
	out.Close()
	if err = os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if _, err = f.Remove(dest); err != nil {
		return err
	}
	if err = os.Rename(t, dest); err != nil {
		return err
	}
	f.created = append(f.created, dest)
	return nil
}

// Remove moves path aside (restorable) and reports 1 if it existed.
func (f *Files) Remove(path string) (int, error) {
	if f.original == nil {
		f.original = map[string]int64{}
	}
	if _, ok := f.original[path]; !ok {
		f.original[path] = size(path)
	}
	if _, err := os.Lstat(path); err != nil {
		return 0, nil
	}
	if err := f.staging(); err != nil {
		return 0, err
	}
	b := f.tmp()
	if err := os.Rename(path, b); err != nil {
		return 0, err
	}
	f.removed = append(f.removed, [2]string{path, b})
	return 1, nil
}

// Delta is the change in bytes of the files directly inside dir.
func (f *Files) Delta(dir string) int64 {
	var d int64
	for p, before := range f.original {
		if filepath.Dir(p) == dir {
			d += size(p) - before
		}
	}
	return d
}

func (f *Files) rollback() {
	for i := len(f.created) - 1; i >= 0; i-- {
		os.Remove(f.created[i])
	}
	for i := len(f.removed) - 1; i >= 0; i-- {
		_ = os.MkdirAll(filepath.Dir(f.removed[i][0]), 0o755)
		_ = os.Rename(f.removed[i][1], f.removed[i][0])
	}
}

func (f *Files) cleanup() {
	if f.work != "" {
		_ = os.RemoveAll(f.work)
	}
}

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
func Usage(root, uuid string) int64 {
	var used int64
	_ = filepath.WalkDir(filepath.Join(root, uuid), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, e := d.Info(); e == nil {
				used += info.Size()
			}
		}
		return nil
	})
	return used
}

// Atomic is BaseService.atomic. uid==0 takes no user lock. Quota is enforced on the
// file delta for the locked user.
func Atomic(ctx context.Context, pool *pgxpool.Pool, root string, uid int64, policyWrite bool,
	fn func(tx pgx.Tx, f *Files, u *LockedUser) error) (err error) {
	f := &Files{Root: root}
	defer f.cleanup()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
			f.rollback()
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
	if err = fn(tx, f, u); err != nil {
		return err
	}
	if u != nil && !u.Gone {
		delta := f.Delta(u.Dir(root))
		if delta > 0 && u.Limit != nil && u.Used+delta > *u.Limit {
			err = Err("storage_quota_exceeded", 413)
			return err
		}
		n := u.Used + delta
		if n < 0 {
			n = 0
		}
		if _, err = tx.Exec(ctx, `UPDATE "user" SET used_bytes=$2 WHERE user_id=$1`, uid, n); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
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

// MediaPath is LocalStorage.path: both parts must be plain file-name components.
func MediaPath(root, uuid, name string) (string, error) {
	for _, p := range []string{uuid, name} {
		if p == "" || p == "." || p == ".." || strings.ContainsAny(p, "/\\\x00") {
			return "", Bad()
		}
	}
	return filepath.Join(root, uuid, name), nil
}
