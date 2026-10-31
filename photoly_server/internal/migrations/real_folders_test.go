package migrations

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestRealFoldersRejectsLegacyLibrary(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	c, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(ctx)
	// Temporary tables shadow the real tables only on this connection. No public
	// schema or existing user data is touched by this migration guard test.
	if _, err = c.Exec(ctx, `CREATE TEMP TABLE photo(id int); CREATE TEMP TABLE namespace(ns_parent_id int); INSERT INTO photo VALUES(42)`); err != nil {
		t.Fatal(err)
	}
	sql, err := files.ReadFile("sql/00008_real_folders.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := c.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, strings.Split(string(sql), "-- +goose Down")[0])
	if err == nil || !strings.Contains(err.Error(), "require a fresh library") {
		t.Fatalf("legacy library accepted: %v", err)
	}
	_ = tx.Rollback(ctx)
	var id int
	if err = c.QueryRow(ctx, `SELECT id FROM photo`).Scan(&id); err != nil || id != 42 {
		t.Fatalf("guard changed existing data: %d %v", id, err)
	}
}
