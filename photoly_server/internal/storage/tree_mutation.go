package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (f *Files) stageFor(p string) (string, error) {
	if Within(f.Root, p) {
		return filepath.Join(f.work, uuid.NewString()), nil
	}
	work, e := externalWorkspace(p)
	if e != nil {
		return "", e
	}
	f.extraWork = append(f.extraWork, work)
	return filepath.Join(work, uuid.NewString()), nil
}
func (f *Files) TreeMkdir(ctx context.Context, d TreeStore, key string) error {
	if e := d.Mkdir(ctx, key); e != nil {
		return e
	}
	f.remoteUndo = append(f.remoteUndo, func() error { return cleanupTree(func(ctx context.Context) error { return d.Delete(ctx, key) }) })
	return nil
}
func cleanupTree(fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return fn(ctx)
}
func (f *Files) TreeMove(ctx context.Context, d TreeStore, src, dst, version string) error {
	if src == dst {
		return nil
	}
	before, e := d.Property(ctx, src)
	if e != nil {
		return e
	}
	if version != "" && version != before.Version {
		return errors.New("storage_tree_conflict_rescan_required")
	}
	if _, e = d.Property(ctx, dst); e == nil {
		return errors.New("storage_tree_conflict_rescan_required")
	} else if !errors.Is(e, ErrNotFound) {
		return e
	}
	// Register the inverse before dispatch: a lost response may still have moved
	// the resource. Never move an unrelated writer's destination during rollback.
	f.remoteUndo = append(f.remoteUndo, func() error {
		return cleanupTree(func(ctx context.Context) error {
			target, e := d.Property(ctx, dst)
			if errors.Is(e, ErrNotFound) {
				return nil
			}
			if e != nil {
				return e
			}
			if _, e = d.Property(ctx, src); !errors.Is(e, ErrNotFound) {
				return errors.New("storage_tree_partial_move_retained_for_recovery")
			}
			if before.Identity != "" && target.Identity != before.Identity || before.Identity == "" && before.Version != "" && target.Version != before.Version {
				return errors.New("storage_tree_rollback_conflict")
			}
			return d.Move(ctx, dst, src, target.Version)
		})
	})
	return d.Move(ctx, src, dst, before.Version)
}
func (f *Files) TreeRemove(ctx context.Context, d TreeStore, key, expected string) error {
	info, e := d.Property(ctx, key)
	if errors.Is(e, ErrNotFound) {
		return nil
	}
	if e != nil {
		return e
	}
	if expected != "" && info.Version != expected {
		return errors.New("storage_tree_conflict_rescan_required")
	}
	if _, e = d.Property(ctx, SystemDirectory); errors.Is(e, ErrNotFound) {
		if e = d.Mkdir(ctx, SystemDirectory); e != nil {
			return e
		}
	} else if e != nil {
		return e
	}
	backup := SystemDirectory + "/" + uuid.NewString()
	if e = f.TreeMove(ctx, d, key, backup, info.Version); e != nil {
		return e
	}
	f.remoteCleanup = append(f.remoteCleanup, func() error { return cleanupTree(func(ctx context.Context) error { return d.Delete(ctx, backup) }) })
	return nil
}
func (f *Files) flushTree(ctx context.Context, p string, loc location, d TreeStore) error {
	in, e := OpenFile(p)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	defer in.Close()
	if e = d.PutVersion(ctx, loc.key, in, ""); e != nil {
		return e
	}
	stat, e := d.Property(ctx, loc.key)
	if e != nil {
		return e
	}
	f.remoteUndo = append(f.remoteUndo, func() error {
		return cleanupTree(func(ctx context.Context) error {
			current, e := d.Property(ctx, loc.key)
			if e != nil {
				return e
			}
			if current.Version != stat.Version {
				return errors.New("storage_tree_rollback_conflict")
			}
			return d.Delete(ctx, loc.key)
		})
	})
	return nil
}
func (f *Files) SetContext(ctx context.Context) { f.ctx = ctx }
func (f *Files) context() context.Context {
	if f.ctx != nil {
		return f.ctx
	}
	return context.Background()
}
func TreeSource(s Source) (TreeStore, error) {
	v, e := SourceValues(s)
	if e != nil {
		return nil, e
	}
	return NewTree(s.Kind, v)
}

// CopyTo reads a remote version into a private processing snapshot, checking the
// version again afterwards. No partially downloaded content becomes an asset.
func (d *DAV) CopyTo(ctx context.Context, key string, out io.Writer, expected TreeEntry) error {
	return copyTreeTo(ctx, d, key, out, expected)
}
func copyTreeTo(ctx context.Context, d TreeStore, key string, out io.Writer, expected TreeEntry) error {
	r, e := d.Get(ctx, key)
	if e != nil {
		return e
	}
	_, e = io.Copy(out, r)
	r.Close()
	if e != nil {
		return e
	}
	after, e := d.Property(ctx, key)
	if e != nil {
		return e
	}
	if !SameEntry(expected, after) {
		return errors.New("storage_tree_file_changed_during_read")
	}
	return nil
}
func SameEntry(a, b TreeEntry) bool {
	return a.Identity == b.Identity && a.Size == b.Size && a.Modified == b.Modified && a.Version == b.Version && a.Directory == b.Directory
}

// RecordVersions runs after durable writes and before the database commit. Local
// cache inode metadata must never be mistaken for a remote resource identity.
func (f *Files) RecordVersions(ctx context.Context, q dbgen.DBTX) error {
	for p := range f.before {
		loc, ok := remoteLocation(p)
		if !ok || loc.photoID == 0 {
			continue
		}
		d, ok := loc.store.(TreeStore)
		if !ok {
			continue
		}
		if _, e := os.Lstat(p); os.IsNotExist(e) {
			continue
		}
		info, e := d.Property(ctx, loc.key)
		if e != nil {
			return e
		}
		if _, e = q.Exec(ctx, `UPDATE media_asset SET version=$3,fs_identity=$4,fs_size=$5,fs_mtime=$6,content_version=content_version+1 WHERE photo_id=$1 AND role=$2`, loc.photoID, loc.role, info.Version, info.Identity, info.Size, info.Modified); e != nil {
			return e
		}
		if loc.role == "original" {
			if _, e = q.Exec(ctx, `UPDATE photo SET fs_identity=$2,fs_size=$3,fs_mtime=$4,remote_version=$5 WHERE photo_id=$1`, loc.photoID, info.Identity, info.Size, info.Modified, info.Version); e != nil {
				return e
			}
		}
	}
	return nil
}
func LockPhoto(ctx context.Context, q dbgen.DBTX, pid int64) error {
	var sid int64
	if e := q.QueryRow(ctx, `SELECT source_id FROM photo WHERE photo_id=$1`, pid).Scan(&sid); e != nil {
		return e
	}
	s, e := ReadSource(ctx, q, sid)
	if e != nil {
		return e
	}
	return LockSource(ctx, q, s)
}

type exclusiveUserKey struct{}

func ExclusiveUser(ctx context.Context) context.Context {
	return context.WithValue(ctx, exclusiveUserKey{}, true)
}
func UserLockMode(ctx context.Context) string {
	if ctx.Value(exclusiveUserKey{}) == true {
		return "FOR UPDATE"
	}
	return "FOR KEY SHARE"
}

func (f *Files) RecoveryRequired() bool { return f.rollbackFailed }
func NeedsRescan(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "storage_tree_") || strings.Contains(err.Error(), "webdav_") || strings.Contains(err.Error(), "smb_") || strings.Contains(err.Error(), "identity_changed"))
}

func (f *Files) ensureParent(p string) error {
	if Within(f.Root, p) {
		return mkdirAll(filepath.Dir(p))
	}
	parent, e := NoLinks(filepath.Dir(p))
	if e != nil {
		return e
	}
	st, e := os.Lstat(parent)
	if e != nil {
		return e
	}
	if !st.IsDir() {
		return errors.New("storage_folder_unavailable")
	}
	return nil
}

// Read transactions (AI snapshots, shares, metadata) must not take a physical
// write lock. Mark only transactions that own file compensation.
type mutationTx struct{ pgx.Tx }

func (mutationTx) StorageMutation()     {}
func WriteTransaction(tx pgx.Tx) pgx.Tx { return mutationTx{tx} }
