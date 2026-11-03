package migrations

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
)

// Use a separate empty database: this exercises a real version-16 -> 17 upgrade.
func TestNamespacePinsUpgrade(t *testing.T) {
	dsn := os.Getenv("PHOTOLY_TEST_PINS_UPGRADE_DATABASE_URL")
	if dsn == "" {
		t.Skip("PHOTOLY_TEST_PINS_UPGRADE_DATABASE_URL required")
	}
	ctx := context.Background()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	p, err := provider(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.UpTo(ctx, 16); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO user_group(id,name,is_default) VALUES(991,'pins upgrade',false); INSERT INTO "user"(user_id,uuid,user_name,email,role,group_id) VALUES(991,'pins-upgrade','pins','pins@example.test','user',991); INSERT INTO namespace(ns_id,ns_name,ns_parent_id,user_id) VALUES(991,'/',-1,991),(992,'Trips',991,991)`); err != nil {
		t.Fatal(err)
	}
	if _, err = p.UpTo(ctx, 17); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM namespace_pin`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	var name string
	if err = db.QueryRowContext(ctx, `SELECT ns_name FROM namespace WHERE ns_id=992`).Scan(&name); err != nil || name != "Trips" {
		t.Fatal(name, err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO namespace_pin(user_id,ns_id,is_default) VALUES(991,992,true)`); err != nil {
		t.Fatal(err)
	}
	// Rolling back the feature drops preferences only, without touching folders.
	migration, err := files.ReadFile("sql/00017_namespace_pins.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, strings.Split(string(migration), "-- +goose Down")[1]); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRowContext(ctx, `SELECT ns_name FROM namespace WHERE ns_id=992`).Scan(&name); err != nil || name != "Trips" {
		t.Fatal(name, err)
	}
}
