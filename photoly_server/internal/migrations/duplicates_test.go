package migrations

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

func TestDuplicatesUpgrade(t *testing.T) {
	dsn := os.Getenv("PHOTOLY_TEST_DUPLICATES_UPGRADE_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated database required")
	}
	ctx := context.Background()
	db, e := sql.Open("pgx", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if _, e = db.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS vector`); e != nil {
		t.Fatal(e)
	}
	p, e := provider(db)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.UpTo(ctx, 18); e != nil {
		t.Fatal(e)
	}
	if _, e = db.ExecContext(ctx, `INSERT INTO gallery(ga_name,description) VALUES('Existing','Keep this description')`); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		if e = Run(ctx, dsn); e != nil {
			t.Fatal(e)
		}
	}
	var description string
	if e = db.QueryRowContext(ctx, `SELECT description FROM gallery WHERE ga_name='Existing'`).Scan(&description); e != nil || description != "Keep this description" {
		t.Fatal(description, e)
	}
	for _, table := range []string{"duplicate_keep", "duplicate_ignore", "duplicate_candidate", "duplicate_group"} {
		var n int
		if e = db.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&n); e != nil || n != 0 {
			t.Fatal(table, n, e)
		}
	}
}
