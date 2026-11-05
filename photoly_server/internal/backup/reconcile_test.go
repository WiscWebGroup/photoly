package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestReconcileRemountedLocalLibrary(t *testing.T) {
	e, url := testEngine(t)
	ctx := context.Background()
	// Establish checksums and identities, then simulate a remount that gives
	// unchanged files and directories new inode numbers.
	if err := validateOriginals(ctx, url, e.Cfg.UploadFolder, true); err != nil {
		t.Fatal(err)
	}
	if err := e.finalizeIdentities(ctx); err != nil {
		t.Fatal(err)
	}
	c, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(ctx)
	if _, err := c.Exec(ctx, `UPDATE storage_source SET root_identity='external-unchanged' WHERE NOT is_default`); err != nil {
		t.Fatal(err)
	}
	var before, after string
	const relationships = `SELECT json_build_array((SELECT json_agg(row(photo_id,photo_uuid,content_version)) FROM photo),(SELECT json_agg(g) FROM gallery_photo g),(SELECT json_agg(n) FROM media_note n))::text`
	if err := c.QueryRow(ctx, relationships).Scan(&before); err != nil {
		t.Fatal(err)
	}
	copy := filepath.Join(t.TempDir(), "remounted")
	if err := os.CopyFS(copy, os.DirFS(e.Cfg.UploadFolder)); err != nil {
		t.Fatal(err)
	}
	e.Cfg.UploadFolder = copy
	if err := validateOriginals(ctx, url, copy, false); err == nil {
		t.Fatal("fixture did not reproduce stale directory identities")
	}
	if repaired, err := ReconcileLocalIdentities(ctx, e.Cfg); err != nil || !repaired {
		t.Fatal("remount not repaired", repaired, err)
	}
	assertInstalledIdentities(t, e, url)
	if err := c.QueryRow(ctx, relationships).Scan(&after); err != nil || before != after {
		t.Fatal("media identity or relationships changed", err)
	}
	var externalOK bool
	if err := c.QueryRow(ctx, `SELECT bool_and(root_identity='external-unchanged') FROM storage_source WHERE NOT is_default`).Scan(&externalOK); err != nil || !externalOK {
		t.Fatal("external source changed", err)
	}
	// An ordinary second start must not run a full content check.
	if _, err := c.Exec(ctx, `UPDATE photo SET content_sha256=''`); err != nil {
		t.Fatal(err)
	}
	if repaired, err := ReconcileLocalIdentities(ctx, e.Cfg); err != nil || repaired {
		t.Fatal("unnecessary repair", repaired, err)
	}
}

func TestReconcileRejectsUnverifiedRemount(t *testing.T) {
	for _, kind := range []string{"missing", "changed", "missing-checksum", "live-changed", "symlink", "empty-root"} {
		t.Run(kind, func(t *testing.T) {
			e, url := testEngine(t)
			ctx := context.Background()
			if err := validateOriginals(ctx, url, e.Cfg.UploadFolder, true); err != nil {
				t.Fatal(err)
			}
			c, err := pgx.Connect(ctx, url)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close(ctx)
			var owner string
			var pid int64
			if err := c.QueryRow(ctx, `SELECT u.uuid,p.photo_id FROM photo p JOIN "user" u USING(user_id) ORDER BY p.photo_id DESC LIMIT 1`).Scan(&owner, &pid); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(e.Cfg.UploadFolder, owner, "中文", "image.jpg")
			switch kind {
			case "missing":
				err = os.Remove(path)
			case "changed":
				err = os.WriteFile(path, []byte("replacement"), 0600)
			case "missing-checksum":
				_, err = c.Exec(ctx, `UPDATE photo SET content_sha256='' WHERE photo_id=$1`, pid)
			case "live-changed":
				err = os.WriteFile(filepath.Join(filepath.Dir(path), "live.mov"), []byte("replacement"), 0600)
			case "symlink":
				target := filepath.Join(t.TempDir(), "original")
				if err = os.Rename(path, target); err == nil {
					err = os.Symlink(target, path)
				}
			case "empty-root":
				// Do not adopt unverified content in a previously empty library.
				var adminOwner string
				if err = c.QueryRow(ctx, `SELECT uuid FROM "user" WHERE email='admin@test.example'`).Scan(&adminOwner); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(e.Cfg.UploadFolder, adminOwner, "unverified.jpg"), []byte("unverified"), 0600); err != nil {
					t.Fatal(err)
				}
				_, err = c.Exec(ctx, `UPDATE storage_source SET root_identity='stale' WHERE is_default AND user_id=(SELECT user_id FROM "user" WHERE email='admin@test.example')`)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = c.Exec(ctx, `UPDATE storage_source SET root_identity='stale' WHERE id IN (SELECT source_id FROM photo); UPDATE namespace SET fs_identity='stale'; UPDATE photo SET fs_identity='stale'; UPDATE media_asset SET fs_identity='stale'`); err != nil {
				t.Fatal(err)
			}
			if repaired, err := ReconcileLocalIdentities(ctx, e.Cfg); err == nil || repaired {
				t.Fatal("accepted unverified remount", repaired, err)
			}
			var unchanged bool
			if err := c.QueryRow(ctx, `SELECT (SELECT bool_and(root_identity='stale') FROM storage_source WHERE id IN (SELECT source_id FROM photo)) AND (SELECT bool_and(fs_identity='stale') FROM namespace) AND (SELECT bool_and(fs_identity='stale') FROM photo) AND (SELECT bool_and(fs_identity='stale') FROM media_asset)`).Scan(&unchanged); err != nil || !unchanged {
				t.Fatal("failed repair partially updated identities", err)
			}
		})
	}
}
