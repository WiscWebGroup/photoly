package migrations

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

// This optional database must be empty and disposable; unlike the normal test
// database it is deliberately upgraded through a populated version-8 schema.
func TestExternalSourcesUpgrade(t *testing.T) {
	dsn := os.Getenv("PHOTOLY_TEST_UPGRADE_DATABASE_URL")
	if dsn == "" {
		t.Skip("PHOTOLY_TEST_UPGRADE_DATABASE_URL required")
	}
	ctx := context.Background()
	db, e := sql.Open("pgx", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	p, e := provider(db)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.UpTo(ctx, 8); e != nil {
		t.Fatal(e)
	}
	if _, e = db.ExecContext(ctx, `INSERT INTO user_group(id,name,is_default) VALUES(991,'upgrade',false); INSERT INTO "user"(user_id,uuid,user_name,email,role,group_id) VALUES(991,'upgrade-uuid','upgrade','upgrade@example.test','user',991); INSERT INTO namespace(ns_id,ns_name,ns_parent_id,user_id) VALUES(991,'/',-1,991); INSERT INTO photo(photo_id,photo_uuid,photo_name,format,ns_id,user_id,token) VALUES(991,'stable-photo','kept','png',991,991,'stable-share'); INSERT INTO media_asset(photo_id,role,filename) VALUES(991,'original','kept.png')`); e != nil {
		t.Fatal(e)
	}
	if _, e = p.UpTo(ctx, 11); e != nil {
		t.Fatal(e)
	}
	var uuid, token, filename, root string
	if e = db.QueryRowContext(ctx, `SELECT p.photo_uuid,p.token,a.filename,s.root_path FROM photo p JOIN media_asset a USING(photo_id) JOIN storage_source s ON s.id=p.source_id WHERE p.photo_id=991`).Scan(&uuid, &token, &filename, &root); e != nil {
		t.Fatal(e)
	}
	if uuid != "stable-photo" || token != "stable-share" || filename != "kept.png" || root != "upgrade-uuid" {
		t.Fatalf("upgrade changed data: %s %s %s %s", uuid, token, filename, root)
	}
}
