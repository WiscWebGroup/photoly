package photo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
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
	v, err := dbgen.New(q).GetSetting(ctx, name)
	if err != nil || v == nil {
		return def
	}
	return *v
}

// ---- storage ----

// MediaPath preserves public UUID URLs but resolves physical assets from the database.
func (s *Service) MediaPath(ctx context.Context, q DB, userUUID, photoUUID, format string, thumbnail bool) (string, error) {
	role := "original"
	if thumbnail {
		role = "thumbnail"
	}
	return storage.Resolve(ctx, q, s.Root, userUUID, photoUUID, role)
}

var assetRoles = []string{"preview", "live_original", "live_preview"}

func (s *Service) AssetPath(ctx context.Context, q DB, userUUID, photoUUID, role string) (string, error) {
	return storage.Resolve(ctx, q, s.Root, userUUID, photoUUID, role)
}
func (s *Service) Usage(userUUID string) int64 { return storage.Usage(s.Root, userUUID) }

// DiskFree returns free bytes of the filesystem holding path.
func DiskFree(path string) uint64 {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return ^uint64(0)
	}
	return uint64(st.Bavail) * uint64(st.Bsize)
}

type Files = storage.Files

func (s *Service) newFiles() (*Files, error) { return storage.NewFiles(s.Root) }
func fileSize(p string) int64 {
	if st, e := os.Stat(p); e == nil && st.Mode().IsRegular() {
		return st.Size()
	}
	return 0
}

// UserRow is the locked account inside Atomic.
type UserRow struct {
	ID   int64
	UUID string
}

// Atomic is BaseService.atomic: file compensation + shared policy lock + user row lock +
// quota accounting + commit. fn runs inside the transaction.
func (s *Service) Atomic(ctx context.Context, uid int64, fn func(tx pgx.Tx, u UserRow, f *Files) error) error {
	return s.atomic(ctx, uid, false, fn)
}
func (s *Service) atomic(ctx context.Context, uid int64, scan bool, fn func(tx pgx.Tx, u UserRow, f *Files) error) error {
	files, err := s.newFiles()
	if err != nil {
		return err
	}
	ok := false
	defer files.Close()
	files.SetContext(ctx)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	tx = storage.WriteTransaction(tx)
	defer func() {
		if !ok {
			files.Rollback()
		}
		_ = tx.Rollback(context.WithoutCancel(ctx))
		if !ok && (files.RecoveryRequired() || storage.NeedsRescan(err)) {
			_, _ = s.Pool.Exec(context.WithoutCancel(ctx), `UPDATE storage_source SET scan_requested=true,scan_status='error',scan_error='File operation failed; scan requested. Retained workspaces may need inspection.' WHERE user_id=$1 AND kind<>'s3'`, uid)
		}
	}()
	q := dbgen.New(tx)
	if err = q.LockPolicyShared(ctx); err != nil {
		return err
	}
	var u UserRow
	var override, group *int64
	var isOverride bool
	if uid != 0 {
		u.ID = uid
		if err = tx.QueryRow(ctx, `SELECT uuid FROM "user" WHERE user_id=$1 FOR KEY SHARE`, uid).Scan(&u.UUID); err != nil {
			return err
		}
	}
	before := s.Usage(u.UUID)
	if err = fn(tx, u, files); err != nil {
		return err
	}
	if err = files.Flush(ctx); err != nil {
		return err
	}
	if err = files.RecordVersions(ctx, tx); err != nil {
		return err
	}
	if uid != 0 {
		row, e := q.GetUserStorageForUpdate(ctx, uid)
		if e != nil {
			return e
		}
		isOverride, override, group = row.StorageOverride, row.OverrideBytes, row.GroupBytes
		after := s.Usage(u.UUID)
		delta := after - before
		limit := group
		if isOverride {
			limit = override
		}
		if !scan && delta > 0 && limit != nil && after > *limit {
			return Err("storage_quota_exceeded", 413)
		}
		n := after
		if n < 0 {
			n = 0
		}
		if err = q.SetUserUsedBytes(ctx, dbgen.SetUserUsedBytesParams{UsedBytes: n, UserID: uid}); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	ok = true
	files.Commit()
	return nil
}

// ErrNotFound reports pgx.ErrNoRows.
func isNoRows(err error) bool { return err == pgx.ErrNoRows }
