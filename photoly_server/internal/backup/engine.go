package backup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Manifest struct {
	Format        int       `json:"format"`
	ID            string    `json:"id"`
	Created       time.Time `json:"createdAt"`
	Build         string    `json:"build"`
	Schema        int64     `json:"schema"`
	PostgreSQL    int       `json:"postgresMajor"`
	Vector        string    `json:"pgvector"`
	StorageKey    string    `json:"storageKey,omitempty"`
	External      []string  `json:"externalSources"`
	Database      []Part    `json:"database"`
	Archives      []Archive `json:"archives"`
	DatabaseBytes int64     `json:"databaseBytes"`
}
type Summary struct {
	ID       string    `json:"id"`
	Created  time.Time `json:"createdAt"`
	Size     int64     `json:"size"`
	Complete bool      `json:"complete"`
}
type Engine struct {
	Cfg      app.Config
	PartSize int64
	Progress func(string, int64, int64)
	Fault    func(string) error
}

func (e *Engine) progress(stage string, files, bytes int64) {
	if e.Progress != nil {
		e.Progress(stage, files, bytes)
	}
}
func (e *Engine) fault(stage string) error {
	if e.Fault != nil {
		return e.Fault(stage)
	}
	return nil
}
func (e *Engine) limit() int64 {
	if e.PartSize > 0 {
		return e.PartSize
	}
	return partSize
}
func (e *Engine) database(ctx context.Context, name string) (*pgx.Conn, error) {
	if e.Cfg.BackupDatabaseURL == "" {
		return nil, errors.New("BACKUP_DATABASE_URL is required")
	}
	business, be := pgx.ParseConfig(e.Cfg.DatabaseURL)
	management, me := pgx.ParseConfig(e.Cfg.BackupDatabaseURL)
	if be != nil || me != nil || business.Host != management.Host || business.Port != management.Port || business.Database != management.Database {
		return nil, errors.New("backup and application URLs must address the same host, port and database")
	}

	c, err := pgx.ParseConfig(e.Cfg.BackupDatabaseURL)
	if err != nil {
		return nil, errors.New("invalid backup database configuration")
	}
	if name != "" {
		c.Database = name
	}
	return pgx.ConnectConfig(ctx, c)
}
func (e *Engine) dbName() (string, error) {
	c, err := pgx.ParseConfig(e.Cfg.DatabaseURL)
	if err != nil {
		return "", err
	}
	return c.Database, nil
}
func dbInfo(ctx context.Context, c *pgx.Conn, m *Manifest) error {
	return c.QueryRow(ctx, `SELECT current_setting('server_version_num')::int / 10000,(SELECT max(version_id) FROM goose_db_version WHERE is_applied),(SELECT extversion FROM pg_extension WHERE extname='vector')`).Scan(&m.PostgreSQL, &m.Schema, &m.Vector)
}
func databaseURL(raw, name string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u.Path = "/" + name
	return u.String()
}
func (e *Engine) command(ctx context.Context, tool, name string, args ...string) (*exec.Cmd, error) {
	u, err := url.Parse(e.Cfg.BackupDatabaseURL)
	if err != nil || u.User == nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return nil, errors.New("BACKUP_DATABASE_URL must be a PostgreSQL URL")
	}
	password, _ := u.User.Password()
	u.User = url.User(u.User.Username())
	if name != "" {
		u.Path = "/" + name
	}
	if u.Query().Get("password") != "" {
		return nil, errors.New("use URL userinfo for the database password")
	}
	args = append([]string{"--dbname=" + u.String()}, args...)
	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+password, "PGDATABASE="+u.String())
	return cmd, nil
}
func (e *Engine) Capabilities(ctx context.Context) []string {
	var problems []string
	for _, t := range []string{"pg_dump", "pg_restore"} {
		if _, err := exec.LookPath(t); err != nil {
			problems = append(problems, t+" is not installed")
		}
	}
	c, err := e.database(ctx, "")
	if err != nil {
		problems = append(problems, "Backup database management connection unavailable")
	} else {
		defer c.Close(ctx)
		var allowed bool
		if err = c.QueryRow(ctx, `SELECT rolsuper OR rolcreatedb FROM pg_roles WHERE rolname=current_user`).Scan(&allowed); err != nil || !allowed {
			problems = append(problems, "Backup database role needs database creation and ownership privileges")
		}
	}
	return problems
}
func checkTransactions(root string) error {
	for _, p := range []string{".staging", ".photoly-system"} {
		entries, err := os.ReadDir(filepath.Join(root, p))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if len(entries) > 0 {
			return fmt.Errorf("unresolved work remains in %s; resolve it before backup", p)
		}
	}
	return nil
}
func ownedRoots(ctx context.Context, c *pgx.Conn) ([]string, []string, error) {
	rows, err := c.Query(ctx, `SELECT root_path FROM storage_source WHERE is_default AND kind='local' ORDER BY root_path`)
	if err != nil {
		return nil, nil, err
	}
	var roots, external []string
	for rows.Next() {
		var r string
		if err = rows.Scan(&r); err != nil {
			break
		}
		if !storage.ValidComponent(r) || strings.HasPrefix(r, ".") {
			err = errors.New("invalid managed root")
			break
		}
		roots = append(roots, r)
	}
	rows.Close()
	if err != nil {
		return nil, nil, err
	}
	if err = rows.Err(); err != nil {
		return nil, nil, err
	}
	rows, err = c.Query(ctx, `SELECT kind || ': ' || display_name FROM storage_source WHERE NOT(is_default AND kind='local') ORDER BY id`)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var r string
		if err = rows.Scan(&r); err != nil {
			break
		}
		external = append(external, r)
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	return roots, external, err
}
func validateOriginals(ctx context.Context, url, root string, recordIdentities bool) error {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()
	// A cursor-sized batch avoids retaining the whole library or issuing queries on a busy connection.
	var after int64
	for {
		rows, err := pool.Query(ctx, `SELECT p.photo_id,u.uuid,p.photo_uuid FROM photo p JOIN "user" u USING(user_id) JOIN storage_source s ON s.id=p.source_id WHERE s.is_default AND s.kind='local' AND p.photo_id>$1 ORDER BY p.photo_id LIMIT 256`, after)
		if err != nil {
			return err
		}
		type item struct {
			id          int64
			owner, uuid string
		}
		var batch []item
		for rows.Next() {
			var x item
			if err = rows.Scan(&x.id, &x.owner, &x.uuid); err != nil {
				rows.Close()
				return err
			}
			batch = append(batch, x)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}
		for _, x := range batch {
			after = x.id
			var original bool
			rs, err := pool.Query(ctx, `SELECT role FROM media_asset WHERE photo_id=$1 AND (role='original' OR role='live_original' AND EXISTS(SELECT 1 FROM photo_media m WHERE m.photo_id=$1 AND m.live)) ORDER BY role`, x.id)
			if err != nil {
				return err
			}
			var roles []string
			for rs.Next() {
				var r string
				if err = rs.Scan(&r); err != nil {
					rs.Close()
					return err
				}
				roles = append(roles, r)
			}
			rs.Close()
			if err = rs.Err(); err != nil {
				return err
			}
			for _, role := range roles {
				if role == "original" {
					original = true
				}
				p, err := storage.Resolve(ctx, pool, root, x.owner, x.uuid, role)
				if err != nil {
					return fmt.Errorf("media %d %s: %w", x.id, role, err)
				}
				st, err := os.Lstat(p)
				if err != nil {
					return err
				}
				if !st.Mode().IsRegular() {
					return errors.New("original is not a regular file")
				}
				if recordIdentities {
					if _, err = pool.Exec(ctx, `UPDATE media_asset SET fs_identity=$3,fs_size=$4,fs_mtime=$5 WHERE photo_id=$1 AND role=$2`, x.id, role, storage.Identity(st), st.Size(), st.ModTime().UnixNano()); err != nil {
						return err
					}
					if role == "original" {
						if _, err = pool.Exec(ctx, `UPDATE photo SET fs_identity=$2,fs_size=$3,fs_mtime=$4 WHERE photo_id=$1`, x.id, storage.Identity(st), st.Size(), st.ModTime().UnixNano()); err != nil {
							return err
						}
					}
				}
			}
			if !original {
				return fmt.Errorf("media %d has no original asset", x.id)
			}
		}
	}
	return nil
}
func (e *Engine) Backup(ctx context.Context, id, password string) (Summary, error) {
	var summary Summary
	if err := checkID(id); err != nil {
		return summary, err
	}
	if len(password) < 12 {
		return summary, errors.New("use a backup password of at least 12 characters")
	}
	e.progress("Checking backup", 0, 0)
	if err := checkTransactions(e.Cfg.UploadFolder); err != nil {
		return summary, err
	}
	if p := e.Capabilities(ctx); len(p) > 0 {
		return summary, errors.New(strings.Join(p, "; "))
	}
	name, err := e.dbName()
	if err != nil {
		return summary, err
	}
	c, err := e.database(ctx, name)
	if err != nil {
		return summary, err
	}
	defer c.Close(ctx)
	m := Manifest{Format: 1, ID: id, Created: time.Now().UTC(), StorageKey: os.Getenv("STORAGE_SECRET_KEY")}
	// Container builds may omit VCS metadata; retain an exact binary identity instead.
	if executable, er := os.Executable(); er == nil {
		if hash, er := digest(executable); er == nil {
			m.Build = "sha256:" + hash
		}
	}
	if b, ok := debug.ReadBuildInfo(); ok {
		for _, s := range b.Settings {
			if s.Key == "vcs.revision" {
				m.Build = s.Value
			}
		}
	}
	if err = dbInfo(ctx, c, &m); err != nil {
		return summary, err
	}
	roots, external, err := ownedRoots(ctx, c)
	if err != nil {
		return summary, err
	}
	m.External = external
	if err = validateOriginals(ctx, e.Cfg.DatabaseURL, e.Cfg.UploadFolder, false); err != nil {
		return summary, err
	}
	if exists(filepath.Join(e.Cfg.UploadFolder, ".assets")) {
		roots = append(roots, ".assets")
	}
	var bytes int64
	for _, r := range roots {
		p, er := safe(e.Cfg.UploadFolder, r)
		if er != nil {
			return summary, er
		}
		err = filepath.WalkDir(p, func(p string, d os.DirEntry, er error) error {
			if er != nil {
				return er
			}
			if er = ctx.Err(); er != nil {
				return er
			}
			st, er := d.Info()
			if er != nil {
				return er
			}
			if !st.IsDir() && !st.Mode().IsRegular() {
				return errors.New("unsupported file type")
			}
			if st.Mode().IsRegular() {
				bytes += st.Size()
			}
			return nil
		})
		if err != nil {
			return summary, err
		}
	}
	if err = c.QueryRow(ctx, `SELECT pg_database_size(current_database())`).Scan(&m.DatabaseBytes); err != nil {
		return summary, err
	}
	if err = free(e.Cfg.BackupDir, bytes+bytes/20+m.DatabaseBytes*2); err != nil {
		return summary, err
	}
	dir := filepath.Join(e.Cfg.BackupDir, ".partial-"+id)
	if err = os.Mkdir(dir, 0700); err != nil {
		return summary, err
	}
	e.progress("Exporting database", 0, 0)
	sw, enc, err := encryptWriter(dir, "database", password, e.limit())
	if err != nil {
		return summary, err
	}
	cmd, err := e.command(ctx, "pg_dump", name, "-Fc", "--no-owner", "--no-acl", "--no-comments", "--exclude-table-data=public.photoly_session")
	if err != nil {
		return summary, err
	}
	cmd.Stdout = enc
	if err = cmd.Run(); err != nil {
		enc.Close()
		if sw.f != nil {
			sw.f.Close()
		}
		return summary, fmt.Errorf("database export failed: %w", err)
	}
	if err = finishEncrypted(enc, sw); err != nil {
		return summary, err
	}
	m.Database = sw.parts
	for _, r := range roots {
		e.progress("Archiving "+r, 0, 0)
		a, er := pack(ctx, e.Cfg.UploadFolder, r, dir, password, e.limit(), func(f, b int64) { e.progress("Archiving media", f, b) })
		if er != nil {
			return summary, er
		}
		m.Archives = append(m.Archives, a)
	}
	sw, enc, err = encryptWriter(dir, "manifest", password, e.limit())
	if err != nil {
		return summary, err
	}
	if err = json.NewEncoder(enc).Encode(m); err != nil {
		return summary, err
	}
	if err = finishEncrypted(enc, sw); err != nil {
		return summary, err
	}
	// The index is a locator, never trusted as proof of completeness or authenticity.
	if err = atomicJSON(filepath.Join(dir, "manifest-parts.json"), sw.parts); err != nil {
		return summary, err
	}
	summary = Summary{ID: id, Created: m.Created, Complete: true}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return summary, err
	}
	for _, v := range entries {
		st, er := v.Info()
		if er != nil {
			return summary, er
		}
		summary.Size += st.Size()
	}
	if err = atomicJSON(filepath.Join(dir, "index.json"), summary); err != nil {
		return summary, err
	}
	if err = atomicJSON(filepath.Join(dir, "COMPLETE"), map[string]int{"format": 1}); err != nil {
		return summary, err
	}
	if err = os.Rename(dir, filepath.Join(e.Cfg.BackupDir, id)); err != nil {
		return summary, err
	}
	return summary, syncDir(e.Cfg.BackupDir)
}
func (e *Engine) List() ([]Summary, error) {
	list := []Summary{}
	entries, err := os.ReadDir(e.Cfg.BackupDir)
	if err != nil {
		return nil, err
	}
	for _, d := range entries {
		if !d.IsDir() || checkID(d.Name()) != nil {
			continue
		}
		dir, er := safe(e.Cfg.BackupDir, d.Name())
		if er != nil {
			continue
		}
		var s Summary
		if !exists(filepath.Join(dir, "COMPLETE")) || readJSON(filepath.Join(dir, "index.json"), &s) != nil || s.ID != d.Name() {
			continue
		}
		list = append(list, s)
	}
	return list, nil
}
func (e *Engine) manifest(ctx context.Context, id, password string) (Manifest, string, error) {
	var m Manifest
	if err := checkID(id); err != nil {
		return m, "", err
	}
	dir, err := safe(e.Cfg.BackupDir, id)
	if err != nil {
		return m, "", err
	}
	if !exists(filepath.Join(dir, "COMPLETE")) {
		return m, "", errors.New("backup is incomplete")
	}
	var parts []Part
	if err = readJSON(filepath.Join(dir, "manifest-parts.json"), &parts); err != nil {
		return m, "", err
	}
	r, close, err := decryptParts(ctx, dir, password, parts)
	if err != nil {
		return m, "", err
	}
	defer close()
	b, err := io.ReadAll(io.LimitReader(r, 16<<20))
	if err != nil {
		return m, "", err
	}
	if len(b) >= 16<<20 {
		return m, "", errors.New("manifest too large")
	}
	if err = json.Unmarshal(b, &m); err != nil {
		return m, "", err
	}
	if m.ID != id || m.Format != 1 {
		return m, "", errors.New("unsupported backup format")
	}
	return m, dir, nil
}
