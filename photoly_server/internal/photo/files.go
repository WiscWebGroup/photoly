package photo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service holds the shared state of the photo/media/upload code.
type Service struct {
	app  *app.App
	Pool *pgxpool.Pool
	Cfg  app.Config
	Root string // resolved UPLOAD_FOLDER
	Loc  *time.Location
	Exif ExifConfig
}

// HookSet are integration points owned by other packages (the AI agent sets them at init).
type HookSet struct {
	// PhotoChanged is app.ai.hooks.photo_changed: cancel jobs, reset embedding state and
	// enqueue the photo for the active profiles. It runs inside the writing transaction.
	PhotoChanged func(ctx context.Context, tx pgx.Tx, photoID, userID int64) error
}

var Hooks HookSet

// New builds the service with Flask-compatible defaults for unset configuration.
func New(a *app.App) *Service {
	c := a.Cfg
	if c.PublicBaseURL == "" {
		c.PublicBaseURL = "http://localhost:8384"
	}
	c.PublicBaseURL = strings.TrimRight(c.PublicBaseURL, "/")
	if c.Timezone == "" {
		c.Timezone = "America/New_York"
	}
	if c.MaxContentLength == 0 {
		c.MaxContentLength = 600 * 1024 * 1024
	}
	if c.UploadZipMaxBytes == 0 {
		c.UploadZipMaxBytes = 50_000_000_000
	}
	if c.UploadDiskReserveBytes == 0 {
		c.UploadDiskReserveBytes = 512 * 1024 * 1024
	}
	if c.UploadTempHours == 0 {
		c.UploadTempHours = 24
	}
	if c.UploadHistoryDays == 0 {
		c.UploadHistoryDays = 30
	}
	if c.OnlineDownloadMaxBytes == 0 {
		c.OnlineDownloadMaxBytes = 50 * 1024 * 1024
	}
	if c.OnlineDownloadConnectTimeout == 0 {
		c.OnlineDownloadConnectTimeout = 5
	}
	if c.OnlineDownloadTimeout == 0 {
		c.OnlineDownloadTimeout = 60
	}
	if c.MediaProcessTimeout == 0 {
		c.MediaProcessTimeout = 60
	}
	root := c.UploadFolder
	if root == "" {
		root = "/photoly"
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		loc = time.UTC
	}
	return &Service{app: a, Pool: a.Pool, Cfg: c, Root: root, Loc: loc, Exif: ExifConfigFromEnv()}
}

// Now is BaseService.now(): naive local time truncated to seconds.
func (s *Service) Now() time.Time {
	n := time.Now().In(s.Loc)
	return time.Date(n.Year(), n.Month(), n.Day(), n.Hour(), n.Minute(), n.Second(), 0, time.UTC)
}

// Setting is BaseService.setting.
func (s *Service) Setting(ctx context.Context, q DB, name, def string) string {
	var v *string
	if err := q.QueryRow(ctx, `SELECT setting_value FROM setting WHERE setting_name=$1`, name).Scan(&v); err != nil || v == nil {
		return def
	}
	return *v
}

// ---- storage ----

// UserPath is LocalStorage.path with lexical and symlink checks.
func (s *Service) UserPath(userUUID, filename string) (string, error) {
	for _, part := range []string{userUUID, filename} {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "/\\\x00") {
			return "", Bad()
		}
	}
	p := filepath.Join(s.Root, userUUID, filename)
	if r, err := filepath.EvalSymlinks(filepath.Dir(p)); err == nil {
		if r != s.Root && !strings.HasPrefix(r+string(filepath.Separator), s.Root+string(filepath.Separator)) {
			return "", Bad()
		}
	}
	return p, nil
}

// MediaPath is LocalStorage.media: <root>/<user_uuid>/<photo_uuid>[_thumbnail].<ext>.
func (s *Service) MediaPath(userUUID, photoUUID, format string, thumbnail bool) (string, error) {
	name := photoUUID
	if thumbnail {
		name += "_thumbnail"
	}
	return s.UserPath(userUUID, name+"."+format)
}

// Asset roles for Live/HEIC derivatives.
var assetSuffixes = map[string]string{"preview": "_preview.jpg", "live_original": "_live.mov", "live_preview": "_live.mp4"}
var assetRoles = []string{"preview", "live_original", "live_preview"}

// AssetPath is infrastructure.media.asset_path.
func (s *Service) AssetPath(userUUID, photoUUID, role string) (string, error) {
	return s.UserPath(userUUID, photoUUID+assetSuffixes[role])
}

// Usage sums lstat sizes below a user's folder.
func (s *Service) Usage(userUUID string) int64 {
	var used int64
	_ = filepath.WalkDir(filepath.Join(s.Root, userUUID), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if fi, err := os.Lstat(p); err == nil {
			used += fi.Size()
		}
		return nil
	})
	return used
}

// DiskFree returns free bytes of the filesystem holding path.
func DiskFree(path string) uint64 {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return ^uint64(0)
	}
	return uint64(st.Bavail) * uint64(st.Bsize)
}

// Files is FileMutation: filesystem changes compensated if the surrounding transaction fails.
type Files struct {
	Root    string
	work    string
	created []string
	removed [][2]string
	sizes   map[string]int64
}

// Work is the per-transaction scratch directory (inside <root>/.staging).
func (f *Files) Work() string { return f.work }

func (s *Service) newFiles() (*Files, error) {
	staging := filepath.Join(s.Root, ".staging")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return nil, err
	}
	w, err := os.MkdirTemp(staging, "")
	if err != nil {
		return nil, err
	}
	return &Files{Root: s.Root, work: w, sizes: map[string]int64{}}, nil
}

func fileSize(p string) int64 {
	if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
		return fi.Size()
	}
	return 0
}

// Install streams r into dest atomically and returns its SHA-256.
func (f *Files) Install(r io.Reader, dest string) (string, error) {
	tmp := filepath.Join(f.work, NewUUID())
	out, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(out, h), r)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	if _, err = f.Remove(dest); err != nil {
		return "", err
	}
	if err = os.Rename(tmp, dest); err != nil {
		return "", err
	}
	f.created = append(f.created, dest)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// InstallFile installs an existing file's bytes.
func (f *Files) InstallFile(src, dest string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	return f.Install(in, dest)
}

// Created records a path the caller placed itself so rollback removes it.
func (f *Files) Created(p string) { f.created = append(f.created, p) }

// Remove moves path into the staging area (restorable) and reports whether it existed.
func (f *Files) Remove(p string) (int, error) {
	if _, ok := f.sizes[p]; !ok {
		f.sizes[p] = fileSize(p)
	}
	if _, err := os.Lstat(p); err != nil {
		return 0, nil
	}
	backup := filepath.Join(f.work, NewUUID())
	if err := os.Rename(p, backup); err != nil {
		return 0, err
	}
	f.removed = append(f.removed, [2]string{p, backup})
	return 1, nil
}

// Delta is the net byte change of tracked files directly inside dir.
func (f *Files) Delta(dir string) int64 {
	var d int64
	for p, before := range f.sizes {
		if filepath.Dir(p) == dir {
			d += fileSize(p) - before
		}
	}
	return d
}

func (f *Files) rollback() {
	for i := len(f.created) - 1; i >= 0; i-- {
		_ = os.Remove(f.created[i])
	}
	for i := len(f.removed) - 1; i >= 0; i-- {
		_ = os.MkdirAll(filepath.Dir(f.removed[i][0]), 0o755)
		_ = os.Rename(f.removed[i][1], f.removed[i][0])
	}
}

func (f *Files) close() { _ = os.RemoveAll(f.work) }

// UserRow is the locked account inside Atomic.
type UserRow struct {
	ID   int64
	UUID string
}

// Atomic is BaseService.atomic: file compensation + shared policy lock + user row lock +
// quota accounting + commit. fn runs inside the transaction.
func (s *Service) Atomic(ctx context.Context, uid int64, fn func(tx pgx.Tx, u UserRow, f *Files) error) error {
	files, err := s.newFiles()
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			files.rollback()
		}
		files.close()
	}()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock_shared(74831003)`); err != nil {
		return err
	}
	var u UserRow
	var used, override, group *int64
	var isOverride bool
	if uid != 0 {
		u.ID = uid
		err = tx.QueryRow(ctx, `SELECT u.uuid, u.used_bytes, u.storage_override, u.max_storage_bytes, g.max_storage_bytes
			FROM "user" u LEFT JOIN user_group g ON g.id=u.group_id WHERE u.user_id=$1 FOR UPDATE OF u`, uid).
			Scan(&u.UUID, &used, &isOverride, &override, &group)
		if err == pgx.ErrNoRows {
			return Bad()
		} else if err != nil {
			return err
		}
		if used == nil {
			n := s.Usage(u.UUID)
			used = &n
		}
	}
	if err = fn(tx, u, files); err != nil {
		return err
	}
	if uid != 0 {
		delta := files.Delta(filepath.Join(s.Root, u.UUID))
		limit := group
		if isOverride {
			limit = override
		}
		if delta > 0 && limit != nil && *used+delta > *limit {
			return Err("storage_quota_exceeded", 413)
		}
		n := *used + delta
		if n < 0 {
			n = 0
		}
		if _, err = tx.Exec(ctx, `UPDATE "user" SET used_bytes=$1 WHERE user_id=$2`, n, uid); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	ok = true
	return nil
}

// ErrNotFound reports pgx.ErrNoRows.
func isNoRows(err error) bool { return err == pgx.ErrNoRows }
