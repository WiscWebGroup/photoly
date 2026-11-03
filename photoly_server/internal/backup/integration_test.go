package backup

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/migrations"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/session"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testEngine(t *testing.T) (*Engine, string) {
	t.Helper()
	raw := os.Getenv("BACKUP_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("BACKUP_TEST_DATABASE_URL required for isolated backup integration tests")
	}
	for _, tool := range []string{"pg_dump", "pg_restore"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	name := "backup_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	url := databaseURL(raw, name)
	admin, err := pgx.Connect(ctx, databaseURL(raw, "postgres"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	var fixtureAdminUUID string
	t.Cleanup(func() {
		rows, _ := admin.Query(ctx, `SELECT datname FROM pg_database WHERE datname LIKE 'photoly_restore_%' OR datname LIKE 'photoly_previous_%'`)
		var names []string
		if rows != nil {
			for rows.Next() {
				var n string
				_ = rows.Scan(&n)
				names = append(names, n)
			}
			rows.Close()
		}
		names = append(names, name)
		for _, n := range names {
			if n != name {
				// Never remove another instance's retained rollback database.
				candidate, err := pgx.Connect(ctx, databaseURL(raw, n))
				if err != nil {
					continue
				}
				var ours bool
				err = candidate.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM "user" WHERE uuid=$1)`, fixtureAdminUUID).Scan(&ours)
				candidate.Close(ctx)
				if err != nil || !ours || fixtureAdminUUID == "" {
					continue
				}
			}
			_, _ = admin.Exec(ctx, `DROP DATABASE IF EXISTS `+pgx.Identifier{n}.Sanitize()+` WITH (FORCE)`)
		}
		admin.Close(ctx)
	})
	c, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Exec(ctx, `CREATE EXTENSION vector`); err != nil {
		t.Fatal(err)
	}
	c.Close(ctx)
	if err = migrations.Run(ctx, url); err != nil {
		t.Fatal(err)
	}
	c, err = pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(ctx)
	cfg := app.Config{DatabaseURL: url, MigrationDatabaseURL: url, BackupDatabaseURL: url, UploadFolder: t.TempDir(), BackupDir: t.TempDir(), AdminEmail: "admin@test.example", AdminPassword: "administrator test password"}
	if err = migrations.Seed(ctx, c, cfg); err != nil {
		t.Fatal(err)
	}
	if err = c.QueryRow(ctx, `SELECT uuid FROM "user" WHERE email=$1`, cfg.AdminEmail).Scan(&fixtureAdminUUID); err != nil {
		t.Fatal(err)
	}
	lock, err := prepare(cfg)
	if err != nil {
		t.Fatal(err)
	}
	lock.Close()
	// Two users, nested directories, primary and Live Photo media, assets, and an external source.
	t.Setenv("STORAGE_SECRET_KEY", "source-storage-test-key")
	for i := 0; i < 2; i++ {
		owner := uuid.NewString()
		var uid, ns, pid int64
		if err = c.QueryRow(ctx, `INSERT INTO "user"(uuid,email,user_name,role,password,group_id) VALUES($1,$2,'Fixture','user',$3,(SELECT id FROM user_group WHERE is_default)) RETURNING user_id`, owner, owner+"@test.example", legacy.HashPassword("fixture password")).Scan(&uid); err != nil {
			t.Fatal(err)
		}
		if err = c.QueryRow(ctx, `INSERT INTO namespace(ns_name,ns_parent_id,user_id) VALUES('/',-1,$1) RETURNING ns_id`, uid).Scan(&ns); err != nil {
			t.Fatal(err)
		}
		var child int64
		if err = c.QueryRow(ctx, `INSERT INTO namespace(ns_name,ns_parent_id,user_id) VALUES('中文',$1,$2) RETURNING ns_id`, ns, uid).Scan(&child); err != nil {
			t.Fatal(err)
		}
		if err = os.MkdirAll(filepath.Join(cfg.UploadFolder, owner, "中文"), 0700); err != nil {
			t.Fatal(err)
		}
		puid := uuid.NewString()
		if err = c.QueryRow(ctx, `INSERT INTO photo(photo_uuid,photo_name,format,user_id,ns_id,file_name,source_id) SELECT $1,'image','jpg',$2,$3,'image.jpg',source_id FROM namespace WHERE ns_id=$3 RETURNING photo_id`, puid, uid, child).Scan(&pid); err != nil {
			t.Fatal(err)
		}
		for _, asset := range []struct{ role, name string }{{"original", "image.jpg"}, {"live_original", "live.mov"}, {"thumbnail", "thumbnail.jpg"}} {
			if _, err = c.Exec(ctx, `INSERT INTO media_asset(photo_id,role,filename) VALUES($1,$2,$3)`, pid, asset.role, asset.name); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(cfg.UploadFolder, owner, "中文", asset.name)
			if asset.role == "thumbnail" {
				path = filepath.Join(cfg.UploadFolder, ".assets", owner, puid, asset.name)
				_ = os.MkdirAll(filepath.Dir(path), 0700)
			}
			if err = os.WriteFile(path, []byte(owner+asset.role+strings.Repeat("fixture", 4096)), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if _, err = c.Exec(ctx, `INSERT INTO photo_media(photo_id,live) VALUES($1,true)`, pid); err != nil {
			t.Fatal(err)
		}
		var gallery int64
		if err = c.QueryRow(ctx, `INSERT INTO gallery(ga_name,user_id) VALUES('Fixture album',$1) RETURNING ga_id`, uid).Scan(&gallery); err != nil {
			t.Fatal(err)
		}
		if _, err = c.Exec(ctx, `INSERT INTO gallery_photo(ga_id,photo_id) VALUES($1,$2)`, gallery, pid); err != nil {
			t.Fatal(err)
		}
		if _, err = c.Exec(ctx, `INSERT INTO media_note(photo_id,user_id,body) VALUES($1,$2,'Preserved note')`, pid, uid); err != nil {
			t.Fatal(err)
		}
		plain, secret, err := storage.SealValues(map[string]string{"url": "https://nas.example/photos", "password": "remote password"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = c.Exec(ctx, `INSERT INTO storage_source(user_id,kind,root_path,config,secret,display_name) VALUES($1,'webdav','',$2,$3,'External NAS')`, uid, plain, secret); err != nil {
			t.Fatal(err)
		}
	}
	if err = storage.EnsureRoots(ctx, c, cfg.UploadFolder); err != nil {
		t.Fatal(err)
	}
	return &Engine{Cfg: cfg, PartSize: 1024}, url
}
func TestDatabaseBackupRestoreAndRollback(t *testing.T) {
	e, url := testEngine(t)
	ctx := context.Background()
	id := uuid.NewString()
	password := "test backup password 123"
	var originalPath string
	_ = filepath.WalkDir(e.Cfg.UploadFolder, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, "image.jpg") {
			originalPath = p
		}
		return err
	})
	originalHash, err := digest(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := e.Backup(ctx, id, password)
	if err != nil {
		t.Fatal(err)
	}
	if !summary.Complete {
		t.Fatal("incomplete")
	}
	if err = os.WriteFile(originalPath, []byte("changed after backup"), 0600); err != nil {
		t.Fatal(err)
	}
	list, err := e.List()
	if err != nil || len(list) != 1 {
		t.Fatal(list, err)
	}
	c, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	var before int
	_ = c.QueryRow(ctx, `SELECT count(*) FROM photo`).Scan(&before)
	_, err = c.Exec(ctx, `UPDATE media_note SET body='Changed after backup'`)
	if err != nil {
		t.Fatal(err)
	}
	c.Close(ctx)
	if err = e.Restore(ctx, id, "wrong password"); err == nil {
		t.Fatal("wrong password accepted")
	}
	// Every switching fault must leave the pre-restore database usable with its newer data.
	for _, point := range []string{"old_database_renamed", "database_switched", "old_media_moved", "new_media_moved"} {
		t.Run(point, func(t *testing.T) {
			hit := false
			e.Fault = func(p string) error {
				if p == point {
					hit = true
					return errors.New("injected failure")
				}
				return nil
			}
			if err = e.Restore(ctx, id, password); err == nil {
				t.Fatal("expected fault")
			}
			if !hit {
				t.Fatal("did not reach switching fault", point)
			}
			c, err := pgx.Connect(ctx, url)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close(ctx)
			var body string
			if err = c.QueryRow(ctx, `SELECT body FROM media_note LIMIT 1`).Scan(&body); err != nil || body != "Changed after backup" {
				t.Fatal("rollback lost pre-restore database", body, err)
			}
			if err = validateOriginals(ctx, url, e.Cfg.UploadFolder, false); err != nil {
				t.Fatal(err)
			}
		})
	}
	e.Fault = nil
	t.Setenv("STORAGE_SECRET_KEY", "target-storage-test-key")
	if err = e.Restore(ctx, id, password); err != nil {
		t.Fatal(err)
	}
	restoredHash, err := digest(originalPath)
	if err != nil || restoredHash != originalHash {
		t.Fatal("original contents not restored", err)
	}
	c, err = pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(ctx)
	var count int
	if err = c.QueryRow(ctx, `SELECT count(*) FROM photo`).Scan(&count); err != nil || count != before {
		t.Fatal(count, err)
	}
	var body string
	if err = c.QueryRow(ctx, `SELECT body FROM media_note LIMIT 1`).Scan(&body); err != nil || body != "Preserved note" {
		t.Fatal(body, err)
	}
	var enabled bool
	var sid int64
	if err = c.QueryRow(ctx, `SELECT id,enabled FROM storage_source WHERE kind='webdav' LIMIT 1`).Scan(&sid, &enabled); err != nil || enabled {
		t.Fatal("external source should be disabled", err)
	}
	var s storage.Source
	if err = c.QueryRow(ctx, `SELECT config,secret FROM storage_source WHERE id=$1`, sid).Scan(&s.Config, &s.Secret); err != nil {
		t.Fatal(err)
	}
	v, err := storage.SourceValues(s)
	if err != nil || v["password"] != "remote password" {
		t.Fatal("credential rekey failed", err)
	}
	if err = validateOriginals(ctx, url, e.Cfg.UploadFolder, false); err != nil {
		t.Fatal(err)
	}
	c.Close(ctx)
	if err = e.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err = e.Rollback(ctx); err != nil {
		t.Fatal("rollback must be idempotent", err)
	}
	b, err := os.ReadFile(originalPath)
	if err != nil || string(b) != "changed after backup" {
		t.Fatal("retained media rollback failed", err)
	}
	c, err = pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(ctx)
	if err = c.QueryRow(ctx, `SELECT body FROM media_note LIMIT 1`).Scan(&body); err != nil || body != "Changed after backup" {
		t.Fatal("retained database rollback failed", body, err)
	}
	var journal Journal
	if err = readJSON(e.journalPath(), &journal); err != nil {
		t.Fatal(err)
	}
	var retained Journal
	if err = readJSON(filepath.Join(e.Cfg.BackupDir, ".control", "rollback-"+journal.ID+".json"), &retained); err != nil || retained.State != "rolled_back" {
		t.Fatal("retained rollback metadata is stale", retained.State, err)
	}
}
func TestMaintenanceGateAndExclusiveJob(t *testing.T) {
	e, url := testEngine(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var starts, stops atomic.Int32
	release := make(chan struct{})
	entered := make(chan struct{})
	start := func(context.Context) (Runtime, error) {
		starts.Add(1)
		return Runtime{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/slow" {
				close(entered)
				<-release
			}
			w.WriteHeader(204)
		}), Stop: func() { stops.Add(1) }}, nil
	}
	c, err := Open(ctx, e.Cfg, start)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var uid int64
	_ = pool.QueryRow(ctx, `SELECT user_id FROM "user" WHERE role='admin' LIMIT 1`).Scan(&uid)
	token, err := (session.Store{Pool: pool}).Create(ctx, uid, 1)
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("HRD-Token", token)
		w := httptest.NewRecorder()
		c.ServeHTTP(w, r)
		return w
	}
	go request("GET", "/slow", "")
	<-entered
	if w := request("POST", "/admin/maintenance/enter", ""); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	c.mu.Lock()
	draining := c.state.Mode
	c.mu.Unlock()
	if draining != "draining" {
		t.Fatal("did not wait for request")
	}
	if w := request("POST", "/admin/backups", `{"password":"a long test password"}`); w.Code != 409 {
		t.Fatal("accepted backup during drain")
	}
	close(release)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		on := c.state.Mode == "on"
		c.mu.Unlock()
		if on {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, path := range []string{"/photo/list", "/cred/media/test", "/share/test", "/admin/setSignUpPermission"} {
		if w := request("GET", path, ""); w.Code != 503 {
			t.Fatal(path, w.Code)
		}
	}
	if w := request("GET", "/healthz", ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	for _, id := range []string{strings.Repeat("g", 36), "../outside", uuid.NewString() + "/../outside"} {
		if w := request("GET", "/admin/backup-jobs/"+id, ""); w.Code != 400 {
			t.Fatal("invalid job ID accepted", id, w.Code)
		}
		if w := request("POST", "/admin/backups/"+id+"/restore", `{"password":"long password for task","confirm":true}`); w.Code != 400 {
			t.Fatal("invalid restore ID accepted", id, w.Code)
		}
	}
	payload, _ := json.Marshal(map[string]string{"password": "long password for task"})
	w := request("POST", "/admin/backups", string(payload))
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request("POST", "/admin/backups", string(payload)); w.Code != 409 {
		t.Fatal("duplicate job", w.Code)
	}
	if w = request("POST", "/admin/maintenance/exit", ""); w.Code != 409 {
		t.Fatal("exit during job", w.Code)
	}
	c.workers.Wait()
	if w = request("POST", "/admin/maintenance/exit", ""); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	if starts.Load() != 2 || stops.Load() != 1 {
		t.Fatal("runtime lifecycle", starts.Load(), stops.Load())
	}
}

func TestRestoreCrashHelper(t *testing.T) {
	if os.Getenv("BACKUP_CRASH_HELPER") != "1" {
		return
	}
	var cfg app.Config
	if err := json.Unmarshal([]byte(os.Getenv("BACKUP_CRASH_CONFIG")), &cfg); err != nil {
		t.Fatal(err)
	}
	e := &Engine{Cfg: cfg, Fault: func(point string) error {
		if point == os.Getenv("BACKUP_CRASH_POINT") {
			os.Exit(81)
		}
		return nil
	}}
	if err := e.Restore(context.Background(), os.Getenv("BACKUP_CRASH_ID"), "test backup password 123"); err != nil {
		t.Fatal(err)
	}
	t.Fatal("crash point was not reached")
}
func TestRestoreProcessCrashRecovery(t *testing.T) {
	e, url := testEngine(t)
	ctx := context.Background()
	id := uuid.NewString()
	if _, err := e.Backup(ctx, id, "test backup password 123"); err != nil {
		t.Fatal(err)
	}
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, `UPDATE media_note SET body='Keep newer state'`)
	conn.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}
	config, _ := json.Marshal(e.Cfg)
	for _, point := range []string{"old_database_renamed", "database_switched", "old_media_moved", "new_media_moved"} {
		t.Run(point, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestRestoreCrashHelper$")
			cmd.Env = append(os.Environ(), "BACKUP_CRASH_HELPER=1", "BACKUP_CRASH_CONFIG="+string(config), "BACKUP_CRASH_POINT="+point, "BACKUP_CRASH_ID="+id)
			out, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 81 {
				t.Fatalf("helper: %v %s", err, out)
			}
			if err = e.Recover(ctx); err != nil {
				t.Fatal(err)
			}
			if err = e.Recover(ctx); err != nil {
				t.Fatal("recovery not idempotent", err)
			}
			c, err := pgx.Connect(ctx, url)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close(ctx)
			var body string
			if err = c.QueryRow(ctx, `SELECT body FROM media_note LIMIT 1`).Scan(&body); err != nil || body != "Keep newer state" {
				t.Fatal(body, err)
			}
			if err = validateOriginals(ctx, url, e.Cfg.UploadFolder, false); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMaintenanceRestartAndLock(t *testing.T) {
	e, _ := testEngine(t)
	ctx := context.Background()
	var starts atomic.Int32
	start := func(context.Context) (Runtime, error) {
		starts.Add(1)
		return Runtime{Handler: http.NotFoundHandler(), Stop: func() {}}, nil
	}
	c, err := Open(ctx, e.Cfg, start)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Open(ctx, e.Cfg, start); err == nil {
		t.Fatal("second instance accepted")
	}
	if err = c.enter(); err != nil {
		t.Fatal(err)
	}
	c.workers.Wait()
	c.Close()
	restored, err := Open(ctx, e.Cfg, start)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if starts.Load() != 1 || restored.state.Mode != "on" {
		t.Fatal("maintenance did not survive restart")
	}
	if err = restored.exit(); err != nil {
		t.Fatal(err)
	}
	if starts.Load() != 2 {
		t.Fatal("runtime not resumed exactly once")
	}
}

func TestMigrationToAnotherDatabaseAndRoot(t *testing.T) {
	source, _ := testEngine(t)
	ctx := context.Background()
	id := uuid.NewString()
	password := "migration test password"
	if _, err := source.Backup(ctx, id, password); err != nil {
		t.Fatal(err)
	}
	target, url := testEngine(t)
	src := filepath.Join(source.Cfg.BackupDir, id)
	dst := filepath.Join(target.Cfg.BackupDir, id)
	if err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		to := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(to, 0700)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(to, b, 0600)
	}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STORAGE_SECRET_KEY", "different-target-key")
	if err := target.Restore(ctx, id, password); err != nil {
		t.Fatal(err)
	}
	if err := validateOriginals(ctx, url, target.Cfg.UploadFolder, false); err != nil {
		t.Fatal(err)
	}
	if err := validateOriginals(ctx, source.Cfg.DatabaseURL, source.Cfg.UploadFolder, false); err != nil {
		t.Fatal("migration changed source", err)
	}
	c, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(ctx)
	var sessions, albums int
	if err = c.QueryRow(ctx, `SELECT count(*) FROM photoly_session`).Scan(&sessions); err != nil || sessions != 0 {
		t.Fatal("sessions restored", sessions, err)
	}
	if err = c.QueryRow(ctx, `SELECT count(*) FROM gallery_photo`).Scan(&albums); err != nil || albums != 2 {
		t.Fatal("album relationships lost", albums, err)
	}
}

func TestBackupExportFailureAndMissingOriginal(t *testing.T) {
	e, _ := testEngine(t)
	ctx := context.Background()
	t.Run("failed pg_dump cannot publish", func(t *testing.T) {
		bin := t.TempDir()
		if err := os.WriteFile(filepath.Join(bin, "pg_dump"), []byte("#!/bin/sh\nexit 17\n"), 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		if _, err := e.Backup(ctx, uuid.NewString(), "export failure password"); err == nil || !strings.Contains(err.Error(), "database export failed") {
			t.Fatal("failed pg_dump was accepted", err)
		}
		list, err := e.List()
		if err != nil || len(list) != 0 {
			t.Fatal("partial dump published", list, err)
		}
	})
	var original string
	if err := filepath.WalkDir(e.Cfg.UploadFolder, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == "image.jpg" {
			original = path
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if original == "" {
		t.Fatal("missing fixture")
	}
	if err := os.Remove(original); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Backup(ctx, uuid.NewString(), "missing original password"); err == nil {
		t.Fatal("backup accepted a missing original")
	}
	list, err := e.List()
	if err != nil || len(list) != 0 {
		t.Fatal("incomplete library published", list, err)
	}
}
