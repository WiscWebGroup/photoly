package migrations

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

func TestGalleryDescriptionUpgrade(t *testing.T) {
	dsn := os.Getenv("PHOTOLY_TEST_GALLERY_UPGRADE_DATABASE_URL")
	if dsn == "" {
		t.Skip("PHOTOLY_TEST_GALLERY_UPGRADE_DATABASE_URL required (isolated empty database)")
	}
	ctx := context.Background()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
		t.Fatal(err)
	}
	p, err := provider(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.UpTo(ctx, 17); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO gallery(ga_name,cover_id,cover_color) VALUES('Existing',3,'#ffffff')`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = Run(ctx, dsn); err != nil {
			t.Fatal(err)
		}
	}
	var name, description, color string
	var cover int
	if err = db.QueryRowContext(ctx, `SELECT ga_name,description,cover_id,cover_color FROM gallery`).Scan(&name, &description, &cover, &color); err != nil {
		t.Fatal(err)
	}
	if name != "Existing" || description != "" || cover != 3 || color != "#ffffff" {
		t.Fatalf("existing gallery changed: %q %q %d %q", name, description, cover, color)
	}
	if _, err = db.ExecContext(ctx, `UPDATE gallery SET description='旅行📷'`); err != nil {
		t.Fatal(err)
	}
	if err = Run(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT description FROM gallery`).Scan(&description); err != nil || description != "旅行📷" {
		t.Fatalf("restart lost description: %q %v", description, err)
	}
}
