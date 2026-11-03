package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Model a filesystem which does not preserve staging inodes at the installed
// location. Ordinary ext4 rename tests cannot expose this restore failure.
func TestRestoreRebindsInstalledIdentities(t *testing.T) {
	e, url := testEngine(t)
	ctx := context.Background()
	id := uuid.NewString()
	if _, err := e.Backup(ctx, id, "identity test password"); err != nil {
		t.Fatal(err)
	}
	e.Fault = func(point string) error {
		if point != "new_media_moved" {
			return nil
		}
		var j Journal
		if err := readJSON(e.journalPath(), &j); err != nil {
			return err
		}
		for _, name := range j.NewNames {
			live := filepath.Join(e.Cfg.UploadFolder, name)
			if !exists(live) {
				continue
			}
			copy := filepath.Join(t.TempDir(), name)
			if err := os.CopyFS(copy, os.DirFS(live)); err != nil {
				return err
			}
			// Keep the old inode allocated so its number cannot be reused.
			if err := os.Rename(live, filepath.Join(t.TempDir(), name)); err != nil {
				return err
			}
			if err := os.Rename(copy, live); err != nil {
				return err
			}
		}
		return nil
	}
	if err := e.Restore(ctx, id, "identity test password"); err != nil {
		t.Fatal(err)
	}
	assertInstalledIdentities(t, e, url)
	if err := e.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	assertInstalledIdentities(t, e, url)
}

func assertInstalledIdentities(t *testing.T, e *Engine, url string) {
	t.Helper()
	ctx := context.Background()
	if err := validateOriginals(ctx, url, e.Cfg.UploadFolder, false); err != nil {
		t.Fatal("restored originals cannot be resolved", err)
	}
	c, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(ctx)
	rows, err := c.Query(ctx, `SELECT n.ns_id,n.fs_identity FROM namespace n JOIN storage_source s ON s.id=n.source_id WHERE s.is_default AND s.kind='local'`)
	if err != nil {
		t.Fatal(err)
	}
	type folder struct {
		id       int64
		identity string
	}
	var folders []folder
	for rows.Next() {
		var f folder
		if err := rows.Scan(&f.id, &f.identity); err != nil {
			t.Fatal(err)
		}
		folders = append(folders, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, f := range folders {
		p, _, err := storage.Folder(ctx, c, e.Cfg.UploadFolder, f.id)
		if err != nil {
			t.Fatal(err)
		}
		st, err := os.Stat(p)
		if err != nil || f.identity == "" || f.identity != storage.Identity(st) {
			t.Fatalf("directory identity not rebuilt: %d: %s: %v", f.id, f.identity, err)
		}
	}
	rows, err = c.Query(ctx, `SELECT u.uuid,p.photo_uuid,a.role,a.fs_identity FROM media_asset a JOIN photo p USING(photo_id) JOIN "user" u USING(user_id) JOIN storage_source s ON s.id=p.source_id WHERE s.is_default AND a.role IN ('original','live_original')`)
	if err != nil {
		t.Fatal(err)
	}
	type asset struct{ owner, uuid, role, identity string }
	var assets []asset
	for rows.Next() {
		var a asset
		if err := rows.Scan(&a.owner, &a.uuid, &a.role, &a.identity); err != nil {
			t.Fatal(err)
		}
		assets = append(assets, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, a := range assets {
		p, err := storage.Resolve(ctx, c, e.Cfg.UploadFolder, a.owner, a.uuid, a.role)
		if err != nil {
			t.Fatal(err)
		}
		st, err := os.Stat(p)
		if err != nil || a.identity == "" || a.identity != storage.Identity(st) {
			t.Fatalf("asset identity not rebuilt: %s: %v", a.role, err)
		}
	}
}

func TestLegacyRestoreIdentityRepair(t *testing.T) {
	e, url := testEngine(t)
	ctx := context.Background()
	id := uuid.NewString()
	if _, err := e.Backup(ctx, id, "identity test password"); err != nil {
		t.Fatal(err)
	}
	if err := e.Restore(ctx, id, "identity test password"); err != nil {
		t.Fatal(err)
	}
	var j Journal
	if err := readJSON(e.journalPath(), &j); err != nil {
		t.Fatal(err)
	}
	j.IdentitiesFinalized = false
	if err := e.saveJournal(&j); err != nil {
		t.Fatal(err)
	}
	c, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(ctx)
	if _, err := c.Exec(ctx, `UPDATE namespace SET fs_identity='stale'; UPDATE photo SET fs_identity='stale'; UPDATE media_asset SET fs_identity='stale'; UPDATE storage_source SET root_identity='stale';`); err != nil {
		t.Fatal(err)
	}
	var owner, photoUUID, hash string
	var pid int64
	if err := c.QueryRow(ctx, `SELECT p.photo_id,u.uuid,p.photo_uuid,p.content_sha256 FROM photo p JOIN "user" u USING(user_id) LIMIT 1`).Scan(&pid, &owner, &photoUUID, &hash); err != nil {
		t.Fatal(err)
	}
	// Missing checksums must not silently disable the identity protection.
	if _, err := c.Exec(ctx, `UPDATE photo SET content_sha256='' WHERE photo_id=$1`, pid); err != nil {
		t.Fatal(err)
	}
	if err := e.Recover(ctx); err == nil {
		t.Fatal("accepted unverified original")
	}
	if _, err := c.Exec(ctx, `UPDATE photo SET content_sha256=$2 WHERE photo_id=$1`, pid, hash); err != nil {
		t.Fatal(err)
	}
	// Locate fixture contents without using the intentionally stale DB guards.
	p := filepath.Join(e.Cfg.UploadFolder, owner, "中文", "image.jpg")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("different contents"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := e.Recover(ctx); err == nil {
		t.Fatal("accepted changed original")
	}
	var identity string
	if err := c.QueryRow(ctx, `SELECT root_identity FROM storage_source WHERE is_default LIMIT 1`).Scan(&identity); err != nil || identity != "stale" {
		t.Fatal("failed repair changed identities", identity, err)
	}
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	if err := e.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	assertInstalledIdentities(t, e, url)
	if err := readJSON(e.journalPath(), &j); err != nil || !j.IdentitiesFinalized {
		t.Fatal("repair was not recorded", err)
	}
	if err := c.QueryRow(ctx, `SELECT root_identity FROM storage_source WHERE kind='webdav' LIMIT 1`).Scan(&identity); err != nil || identity != "stale" {
		t.Fatal("repair touched external storage", identity, err)
	}
	// A finalized journal must not authorize rebinding arbitrary future changes.
	if _, err := c.Exec(ctx, `UPDATE storage_source SET root_identity='changed-later' WHERE is_default`); err != nil {
		t.Fatal(err)
	}
	if err := e.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.QueryRow(ctx, `SELECT root_identity FROM storage_source WHERE is_default LIMIT 1`).Scan(&identity); err != nil || identity != "changed-later" {
		t.Fatal("repeated automatic repair", identity, err)
	}
}
