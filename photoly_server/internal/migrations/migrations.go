// Package migrations applies the embedded goose SQL. 00001 is Alembic 0001-0008 squashed
// (pg_dump of the schema Alembic builds); existing Alembic 0008 databases are baselined.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed sql/*.sql
var files embed.FS

const (
	alembicHead = "0008_upload_activity"
	lockID      = 74831001 // same advisory lock Alembic's env.py takes
)

func provider(db *sql.DB) (*goose.Provider, error) {
	sub, err := fs.Sub(files, "sql")
	if err != nil {
		return nil, err
	}
	return goose.NewProvider(goose.DialectPostgres, db, sub)
}

func exists(ctx context.Context, c *sql.Conn, table string) bool {
	var ok bool
	_ = c.QueryRowContext(ctx, `SELECT to_regclass('public.'||$1) IS NOT NULL`, table).Scan(&ok)
	return ok
}

// Run migrates dsn to head, baselining Alembic 0008 databases and refusing older ones.
func Run(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	conn, err := db.Conn(ctx) // dedicated connection holds the advisory lock
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, lockID); err != nil {
		return err
	}
	defer conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, lockID)

	var ext string
	if err := conn.QueryRowContext(ctx, `SELECT coalesce((SELECT extversion FROM pg_extension WHERE extname='vector'),'')`).Scan(&ext); err != nil {
		return err
	}
	if ext == "" {
		return errors.New("pgvector is not installed; the database bootstrap must run CREATE EXTENSION vector")
	}
	if !exists(ctx, conn, "goose_db_version") && exists(ctx, conn, "alembic_version") {
		var rev string
		if err := conn.QueryRowContext(ctx, `SELECT version_num FROM alembic_version`).Scan(&rev); err != nil {
			return err
		}
		if rev != alembicHead {
			return fmt.Errorf("database is at Alembic %q; upgrade with the Flask migrate service to %s first", rev, alembicHead)
		}
		p, err := provider(db)
		if err != nil {
			return err
		}
		if _, err := p.GetDBVersion(ctx); err != nil { // creates goose_db_version
			return err
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO goose_db_version (version_id,is_applied) VALUES (1,true)`); err != nil {
			return err
		}
	}
	p, err := provider(db)
	if err != nil {
		return err
	}
	if _, err := p.Up(ctx); err != nil {
		return err
	}
	_, _ = conn.ExecContext(ctx, `DO $$ BEGIN IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname='photoly') THEN
		REVOKE INSERT, UPDATE, DELETE ON goose_db_version FROM photoly; END IF; END $$`)
	return nil
}

// Check fails unless every embedded migration is applied (serve without migrate rights).
func Check(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	p, err := provider(db)
	if err != nil {
		return err
	}
	pending, err := p.HasPending(ctx)
	if err != nil {
		return err
	}
	if pending {
		return errors.New("database schema is not at the application revision; run `photoly migrate`")
	}
	return nil
}
