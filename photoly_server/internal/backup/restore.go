package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Journal struct {
	ID                  string    `json:"id"`
	State               string    `json:"state"`
	Database            string    `json:"database"`
	StagedDatabase      string    `json:"stagedDatabase"`
	OldDatabase         string    `json:"oldDatabase"`
	OldNames            []string  `json:"oldNames"`
	NewNames            []string  `json:"newNames"`
	Bytes               int64     `json:"bytes"`
	DatabaseBytes       int64     `json:"databaseBytes"`
	Created             time.Time `json:"createdAt"`
	IdentitiesFinalized bool      `json:"identitiesFinalized,omitempty"`
}

func (e *Engine) journalPath() string {
	return filepath.Join(e.Cfg.BackupDir, ".control", "restore.json")
}
func (e *Engine) saveJournal(j *Journal) error { return atomicJSON(e.journalPath(), j) }
func (e *Engine) staging(j *Journal) string {
	return filepath.Join(e.Cfg.UploadFolder, ".recovery", j.ID)
}
func (e *Engine) Recover(ctx context.Context) error {
	var j Journal
	if err := readJSON(e.journalPath(), &j); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if j.State == "committed" && !j.IdentitiesFinalized {
		// Older restores recorded identities only in the temporary tree. Repair
		// once, before starting business workers, and only for verified content.
		if err := e.finalizeIdentities(ctx); err != nil {
			return err
		}
		j.IdentitiesFinalized = true
		return e.saveJournal(&j)
	}
	if j.State == "committed" || j.State == "rolled_back" {
		return nil
	}
	return e.rollback(ctx, &j)
}
func (e *Engine) Rollback(ctx context.Context) error {
	var j Journal
	if err := readJSON(e.journalPath(), &j); err != nil {
		return err
	}
	if j.State == "rolled_back" {
		return nil
	}
	return e.rollback(ctx, &j)
}
func dbExists(ctx context.Context, c *pgx.Conn, name string) (bool, error) {
	var b bool
	err := c.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)`, name).Scan(&b)
	return b, err
}
func renameDB(ctx context.Context, c *pgx.Conn, from, to string) error {
	if _, err := c.Exec(ctx, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=$1 AND pid<>pg_backend_pid()`, from); err != nil {
		return err
	}
	_, err := c.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{from}.Sanitize()+` RENAME TO `+pgx.Identifier{to}.Sanitize())
	return err
}
func (e *Engine) rollback(ctx context.Context, j *Journal) error {
	id, err := validatedID(j.ID)
	if err != nil {
		return err
	}
	j.ID = id
	j.State = "rolling_back"
	if err := e.saveJournal(j); err != nil {
		return err
	}
	base := e.staging(j)
	stage := filepath.Join(base, "new")
	old := filepath.Join(base, "old")
	// Existence tests make every move idempotent, including a crash after rename but before journal update.
	for _, name := range j.NewNames {
		dst, err := safe(stage, name)
		if err != nil {
			return err
		}
		live, err := safe(e.Cfg.UploadFolder, name)
		if err != nil {
			return err
		}
		if !exists(dst) && exists(live) {
			if err = os.Rename(live, dst); err != nil {
				return err
			}
		}
	}
	for _, name := range j.OldNames {
		src, err := safe(old, name)
		if err != nil {
			return err
		}
		dst, err := safe(e.Cfg.UploadFolder, name)
		if err != nil {
			return err
		}
		if exists(src) {
			if exists(dst) {
				return fmt.Errorf("rollback destination occupied: %s", name)
			}
			if err = os.Rename(src, dst); err != nil {
				return err
			}
		}
	}
	if err := syncDir(e.Cfg.UploadFolder); err != nil {
		return err
	}
	if err := syncDir(stage); err != nil {
		return err
	}
	if err := syncDir(old); err != nil {
		return err
	}
	c, err := e.database(ctx, "postgres")
	if err != nil {
		return err
	}
	defer c.Close(ctx)
	oldExists, err := dbExists(ctx, c, j.OldDatabase)
	if err != nil {
		return err
	}
	if oldExists {
		live, err := dbExists(ctx, c, j.Database)
		if err != nil {
			return err
		}
		if live {
			if err = renameDB(ctx, c, j.Database, j.StagedDatabase); err != nil {
				return err
			}
		}
		if err = renameDB(ctx, c, j.OldDatabase, j.Database); err != nil {
			return err
		}
	}
	j.State = "rolled_back"
	if err = e.saveJournal(j); err != nil {
		return err
	}
	retained := filepath.Join(e.Cfg.BackupDir, ".control", "rollback-"+j.ID+".json")
	if exists(retained) {
		return atomicJSON(retained, j)
	}
	return nil
}
func (e *Engine) Restore(ctx context.Context, id, password string) (err error) {
	e.progress("Verifying backup", 0, 0)
	m, dir, err := e.manifest(ctx, id, password)
	if err != nil {
		return err
	}
	name, err := e.dbName()
	if err != nil {
		return err
	}
	live, err := e.database(ctx, name)
	if err != nil {
		return err
	}
	var current Manifest
	err = dbInfo(ctx, live, &current)
	live.Close(ctx)
	if err != nil {
		return err
	}
	if current.Schema != m.Schema || current.PostgreSQL != m.PostgreSQL || current.Vector != m.Vector {
		return errors.New("backup schema, PostgreSQL major and pgvector versions must match this instance")
	}
	j := Journal{ID: uuid.NewString(), State: "prepared", Database: name, Created: time.Now().UTC()}
	suffix := strings.ReplaceAll(j.ID, "-", "")
	j.StagedDatabase = "photoly_restore_" + suffix
	j.OldDatabase = "photoly_previous_" + suffix
	for _, a := range m.Archives {
		if !storage.ValidComponent(a.Root) || (strings.HasPrefix(a.Root, ".") && a.Root != ".assets") || a.Bytes < 0 {
			return errors.New("invalid archive root or size")
		}
		j.NewNames = append(j.NewNames, a.Root)
		j.Bytes += a.Bytes
	}
	if m.DatabaseBytes < 0 {
		return errors.New("invalid database size")
	}
	if err = free(e.Cfg.UploadFolder, j.Bytes+m.DatabaseBytes); err != nil {
		return err
	}
	base := e.staging(&j)
	if _, err = safe(e.Cfg.UploadFolder, ".recovery/"+j.ID); err != nil {
		return err
	}
	switching := false
	defer func() {
		if !switching {
			_ = os.RemoveAll(base)
		}
	}()
	stage := filepath.Join(base, "new")
	old := filepath.Join(base, "old")
	if err = os.MkdirAll(stage, 0700); err != nil {
		return err
	}
	if _, err = storage.NoLinks(base); err != nil {
		return err
	}
	if err = os.Mkdir(old, 0700); err != nil {
		return err
	}
	for _, a := range m.Archives {
		if err = unpack(ctx, dir, stage, password, a, func(f, b int64) { e.progress("Verifying restored media", f, b) }); err != nil {
			return err
		}
	}
	r, close, err := decryptParts(ctx, dir, password, m.Database)
	if err != nil {
		return err
	}
	defer close()
	// Read the database fully into private staging before executing anything from it.
	dump := filepath.Join(base, "database.dump")
	df, err := os.OpenFile(dump, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(df, r)
	if err == nil {
		err = df.Sync()
	}
	ce := df.Close()
	if err != nil {
		return err
	}
	if ce != nil {
		return ce
	}
	defer os.Remove(dump)
	admin, err := e.database(ctx, "postgres")
	if err != nil {
		return err
	}
	defer admin.Close(context.Background())
	// PostgreSQL checks actual tablespace space during restore; old data is untouched on failure.
	if _, err = admin.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{j.StagedDatabase}.Sanitize()+` TEMPLATE template0`); err != nil {
		return err
	}
	applied := false
	defer func() {
		if !applied {
			_, _ = admin.Exec(context.Background(), `DROP DATABASE IF EXISTS `+pgx.Identifier{j.StagedDatabase}.Sanitize()+` WITH (FORCE)`)
		}
	}()
	staged, err := e.database(ctx, j.StagedDatabase)
	if err != nil {
		return err
	}
	if _, err = staged.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
		staged.Close(ctx)
		return err
	}
	if _, err = staged.Exec(ctx, `GRANT ALL ON SCHEMA public TO `+pgx.Identifier{e.migrationRole()}.Sanitize()); err != nil {
		staged.Close(ctx)
		return err
	}
	staged.Close(ctx)
	e.progress("Restoring database", 0, 0)
	cmd, err := e.command(ctx, "pg_restore", j.StagedDatabase, "--exit-on-error", "--no-owner", "--no-acl", "--role="+e.migrationRole(), dump)
	if err != nil {
		return err
	}
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("database restore failed: %w", err)
	}
	staged, err = e.database(ctx, j.StagedDatabase)
	if err != nil {
		return err
	}
	if err = e.normalize(ctx, staged, stage, m.StorageKey); err != nil {
		staged.Close(ctx)
		return err
	}
	staged.Close(ctx)
	stagedURL := databaseURL(e.Cfg.BackupDatabaseURL, j.StagedDatabase)
	if err = validateOriginals(ctx, stagedURL, stage, true); err != nil {
		return err
	}
	staged, err = e.database(ctx, j.StagedDatabase)
	if err != nil {
		return err
	}
	err = rebuildDirectories(ctx, staged, stage)
	staged.Close(ctx)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(e.Cfg.UploadFolder)
	if err != nil {
		return err
	}
	for _, d := range entries {
		if d.Name() == ".recovery" {
			continue
		}
		if _, err = safe(e.Cfg.UploadFolder, d.Name()); err != nil {
			return err
		}
		j.OldNames = append(j.OldNames, d.Name())
	}
	j.Bytes = 0
	for _, name := range j.OldNames {
		if err = filepath.WalkDir(filepath.Join(e.Cfg.UploadFolder, name), func(_ string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			st, er := d.Info()
			if er != nil {
				return er
			}
			if st.Mode().IsRegular() {
				j.Bytes += st.Size()
			}
			return nil
		}); err != nil {
			return err
		}
	}
	if err = admin.QueryRow(ctx, `SELECT pg_database_size($1)`, j.Database).Scan(&j.DatabaseBytes); err != nil {
		return err
	}
	if err = e.saveJournal(&j); err != nil {
		return err
	}
	applied = true
	switching = true
	defer func() {
		if err != nil {
			recovery, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if re := e.rollback(recovery, &j); re != nil {
				err = fmt.Errorf("%v; rollback requires recovery: %v", err, re)
			}
		}
	}()
	e.progress("Switching instance", 0, 0)
	if err = renameDB(ctx, admin, j.Database, j.OldDatabase); err != nil {
		return err
	}
	if err = e.fault("old_database_renamed"); err != nil {
		return err
	}
	if err = renameDB(ctx, admin, j.StagedDatabase, j.Database); err != nil {
		return err
	}
	if err = e.fault("database_switched"); err != nil {
		return err
	}
	for _, n := range j.OldNames {
		if err = os.Rename(filepath.Join(e.Cfg.UploadFolder, n), filepath.Join(old, n)); err != nil {
			return err
		}
		if err = syncDir(old); err != nil {
			return err
		}
		if err = syncDir(e.Cfg.UploadFolder); err != nil {
			return err
		}
		if err = e.fault("old_media_moved"); err != nil {
			return err
		}
	}
	for _, n := range j.NewNames {
		if err = os.Rename(filepath.Join(stage, n), filepath.Join(e.Cfg.UploadFolder, n)); err != nil {
			return err
		}
		if err = syncDir(stage); err != nil {
			return err
		}
		if err = syncDir(e.Cfg.UploadFolder); err != nil {
			return err
		}
		if err = e.fault("new_media_moved"); err != nil {
			return err
		}
	}
	// Do not persist staging identities as the final identity of live media.
	// Shared/virtual filesystems can assign new identities at the destination.
	e.progress("Verifying installed media", 0, 0)
	if err = e.finalizeIdentities(ctx); err != nil {
		return err
	}
	if err = e.fault("identities_finalized"); err != nil {
		return err
	}
	j.IdentitiesFinalized = true
	j.State = "committed"
	if err = e.saveJournal(&j); err != nil {
		return err
	}
	if err = atomicJSON(filepath.Join(e.Cfg.BackupDir, ".control", "rollback-"+j.ID+".json"), j); err != nil {
		return err
	}
	return nil
}

func (e *Engine) migrationRole() string {
	raw := e.Cfg.MigrationDatabaseURL
	if raw == "" {
		raw = e.Cfg.DatabaseURL
	}
	c, err := pgx.ParseConfig(raw)
	if err != nil {
		return "photoly_migrator"
	}
	return c.User
}
func (e *Engine) normalize(ctx context.Context, c *pgx.Conn, root, oldKey string) error {
	rows, err := c.Query(ctx, `SELECT id,secret FROM storage_source WHERE secret IS NOT NULL`)
	if err != nil {
		return err
	}
	type secret struct {
		id int64
		b  []byte
	}
	var secrets []secret
	for rows.Next() {
		var s secret
		if err = rows.Scan(&s.id, &s.b); err != nil {
			rows.Close()
			return err
		}
		secrets = append(secrets, s)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, s := range secrets {
		sealed, err := storage.ReencryptSecret(s.b, oldKey, os.Getenv("STORAGE_SECRET_KEY"))
		if err != nil {
			return fmt.Errorf("configure the target STORAGE_SECRET_KEY before restoring credentials: %w", err)
		}
		if _, err = c.Exec(ctx, `UPDATE storage_source SET secret=$2 WHERE id=$1`, s.id, sealed); err != nil {
			return err
		}
	}
	_, err = c.Exec(ctx, `
 TRUNCATE photoly_session;
 UPDATE storage_source SET enabled=false WHERE NOT(is_default AND kind='local');
 UPDATE storage_source SET root_identity='',scan_requested=false,scan_phase='idle',scan_status='pending',scan_started_at=NULL;
 UPDATE namespace SET fs_identity=''; UPDATE photo SET fs_identity=''; UPDATE media_asset SET fs_identity='';
 TRUNCATE storage_observation;
 UPDATE storage_scan_job SET state='error',error='Interrupted by restore',finished_at=now() WHERE state IN ('queued','running');
 UPDATE upload_item SET state='failed',error='Interrupted by restore; upload again',updated_at=now() WHERE state NOT IN ('done','failed','cancelled');
 UPDATE upload_batch SET cancelled=true,sealed=true,updated_at=now() WHERE NOT cancelled;
 UPDATE ai_job SET status='queued',lease_owner=NULL,lease_token=NULL,lease_expires_at=NULL,heartbeat_at=NULL WHERE status='running';
 UPDATE ai_control_state SET lease_owner=NULL,lease_token=NULL,lease_expires_at=NULL,heartbeat_at=NULL;
 UPDATE video_thumbnail_job SET state='pending',lease_token=NULL WHERE state='processing';
 UPDATE media_classification SET state='pending',lease_token=NULL WHERE state='processing';
 `)
	if err != nil {
		return err
	}
	cfg, err := pgx.ParseConfig(e.Cfg.DatabaseURL)
	if err != nil {
		return err
	}
	role := pgx.Identifier{cfg.User}.Sanitize()
	migration := pgx.Identifier{e.migrationRole()}.Sanitize()
	for _, sql := range []string{`GRANT USAGE ON SCHEMA public TO ` + role, `GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO ` + role, `GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA public TO ` + role, `ALTER DEFAULT PRIVILEGES FOR ROLE ` + migration + ` IN SCHEMA public GRANT SELECT,INSERT,UPDATE,DELETE ON TABLES TO ` + role, `ALTER DEFAULT PRIVILEGES FOR ROLE ` + migration + ` IN SCHEMA public GRANT USAGE,SELECT ON SEQUENCES TO ` + role} {
		if _, err = c.Exec(ctx, sql); err != nil {
			return err
		}
	}
	if _, err = c.Exec(ctx, `REVOKE INSERT,UPDATE,DELETE ON goose_db_version FROM `+role); err != nil {
		return err
	}
	var database string
	if err = c.QueryRow(ctx, `SELECT current_database()`).Scan(&database); err != nil {
		return err
	}
	if _, err = c.Exec(ctx, `REVOKE ALL ON DATABASE `+pgx.Identifier{database}.Sanitize()+` FROM PUBLIC`); err != nil {
		return err
	}
	if _, err = c.Exec(ctx, `GRANT CONNECT ON DATABASE `+pgx.Identifier{database}.Sanitize()+` TO `+role+`,`+migration); err != nil {
		return err
	}
	return nil
}
func (e *Engine) finalizeIdentities(ctx context.Context) error {
	name, err := e.dbName()
	if err != nil {
		return err
	}
	c, err := e.database(ctx, name)
	if err != nil {
		return err
	}
	defer c.Close(ctx)
	return finalizeLocalIdentities(ctx, c, e.Cfg.UploadFolder)
}

func finalizeLocalIdentities(ctx context.Context, c *pgx.Conn, root string) error {
	tx, err := c.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// These guards are cleared only inside this private transaction. Missing,
	// changed or unverified originals roll back every identity update.
	if _, err = tx.Exec(ctx, `
 UPDATE namespace SET fs_identity='' WHERE source_id IN (SELECT id FROM storage_source WHERE is_default AND kind='local');
 UPDATE storage_source SET root_identity='' WHERE is_default AND kind='local';
 `); err != nil {
		return err
	}
	if err = checkOriginals(ctx, tx, root, true, true); err != nil {
		return err
	}
	if err = rebuildDirectories(ctx, tx, root); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE storage_source SET scan_status='pending',scan_error=NULL WHERE is_default AND kind='local'`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func rebuildDirectories(ctx context.Context, c dbgen.DBTX, root string) error {
	// Resolve against blank identities first; record each new inode without scanning/importing files.
	var after int64
	for {
		rows, err := c.Query(ctx, `SELECT n.ns_id FROM namespace n JOIN storage_source s ON s.id=n.source_id WHERE s.is_default AND s.kind='local' AND n.ns_id>$1 ORDER BY n.ns_id LIMIT 256`, after)
		if err != nil {
			return err
		}
		var ids []int64
		for rows.Next() {
			var id int64
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		for _, id := range ids {
			after = id
			p, s, err := storage.Folder(ctx, c, root, id)
			if err != nil {
				return err
			}
			st, err := os.Stat(p)
			if err != nil {
				return err
			}
			if !st.IsDir() {
				return errors.New("namespace is not a directory")
			}
			if _, err = c.Exec(ctx, `UPDATE namespace SET fs_identity=$2 WHERE ns_id=$1`, id, storage.Identity(st)); err != nil {
				return err
			}
			if p == filepath.Join(root, s.Root) {
				if _, err = c.Exec(ctx, `UPDATE storage_source SET root_identity=$2 WHERE id=$1`, s.ID, storage.Identity(st)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
