package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Store struct{ db *pgxpool.Pool }

func openStore(ctx context.Context, url string) (*Store, error) {
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db}, nil
}

func (s *Store) Close() { s.db.Close() }

// Migrate applies embedded migrations in name order, once each, under an advisory lock so
// several instances can start at the same time.
func (s *Store) Migrate(ctx context.Context) error {
	conn, err := s.db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(74831099)`); err != nil {
		return err
	}
	defer conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(74831099)`)
	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migration (
		name text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	names, _ := fs.Glob(migrationFiles, "migrations/*.sql")
	sort.Strings(names)
	for _, name := range names {
		var done bool
		if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migration WHERE name=$1)`, name).Scan(&done); err != nil {
			return err
		}
		if done {
			continue
		}
		sql, _ := migrationFiles.ReadFile(name)
		err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(sql)); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO schema_migration (name) VALUES ($1)`, name)
			return err
		})
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		log.Printf("migrate: applied %s", name)
	}
	return nil
}

type Photo struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Mime      string    `json:"mime"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
	Indexed   bool      `json:"indexed"`
}

type Hit struct {
	Photo
	Score float64 `json:"score"`
}

// AddPhoto stores the record and its vector (computed by source: "client" or "server") in one transaction; the file
// is written by the caller and removed again if this fails.
func (s *Store) AddPhoto(ctx context.Context, userID int, p Photo, sha []byte, profile, source string, vec []float32) (int64, error) {
	var id int64
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO photo (user_id, name, mime, size_bytes, sha256)
			VALUES ($1,$2,$3,$4,$5) RETURNING id`, userID, p.Name, p.Mime, p.Size, sha).Scan(&id); err != nil {
			return err
		}
		if vec == nil {
			return nil
		}
		_, err := tx.Exec(ctx, `INSERT INTO photo_embedding (photo_id, profile, source, embedding)
			VALUES ($1,$2,$3,$4)`, id, profile, source, pgvector.NewHalfVector(vec))
		return err
	})
	return id, err
}

func (s *Store) Photos(ctx context.Context, userID int, profile string) ([]Photo, error) {
	rows, err := s.db.Query(ctx, `SELECT p.id, p.name, p.mime, p.size_bytes, p.created_at, e.photo_id IS NOT NULL
		FROM photo p LEFT JOIN photo_embedding e ON e.photo_id = p.id AND e.profile = $2
		WHERE p.user_id = $1 ORDER BY p.id DESC LIMIT 500`, userID, profile)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Photo, error) {
		var p Photo
		err := r.Scan(&p.ID, &p.Name, &p.Mime, &p.Size, &p.CreatedAt, &p.Indexed)
		return p, err
	})
}

var errNotFound = errors.New("not found")

func (s *Store) Photo(ctx context.Context, userID int, id int64) (Photo, error) {
	var p Photo
	err := s.db.QueryRow(ctx, `SELECT id, name, mime, size_bytes, created_at, false FROM photo
		WHERE user_id=$1 AND id=$2`, userID, id).Scan(&p.ID, &p.Name, &p.Mime, &p.Size, &p.CreatedAt, &p.Indexed)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, errNotFound
	}
	return p, err
}

// DeleteAllPhotos removes all of a user's photos; vectors go with them (ON DELETE CASCADE).
// Files stay on disk, as with DeletePhoto.
func (s *Store) DeleteAllPhotos(ctx context.Context, userID int) (int64, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM photo WHERE user_id=$1`, userID)
	return tag.RowsAffected(), err
}

func (s *Store) DeletePhoto(ctx context.Context, userID int, id int64) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM photo WHERE user_id=$1 AND id=$2`, userID, id)
	if err == nil && tag.RowsAffected() == 0 {
		return errNotFound
	}
	return err
}

// Search ranks only the caller's own photos, so a bad vector can only affect its uploader.
func (s *Store) Search(ctx context.Context, userID int, profile string, vec []float32, limit int, exclude int64) ([]Hit, error) {
	rows, err := s.db.Query(ctx, `SELECT p.id, p.name, p.mime, p.size_bytes, p.created_at, true,
			1 - (e.embedding <=> $3) AS score
		FROM photo_embedding e JOIN photo p ON p.id = e.photo_id
		WHERE p.user_id = $1 AND e.profile = $2 AND p.id <> $5
		ORDER BY e.embedding <=> $3 LIMIT $4`, userID, profile, pgvector.NewHalfVector(vec), limit, exclude)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Hit, error) {
		var h Hit
		err := r.Scan(&h.ID, &h.Name, &h.Mime, &h.Size, &h.CreatedAt, &h.Indexed, &h.Score)
		return h, err
	})
}

// Similar searches with a stored vector; the server needs no model for this.
func (s *Store) Similar(ctx context.Context, userID int, profile string, id int64, limit int) ([]Hit, error) {
	var v pgvector.HalfVector
	err := s.db.QueryRow(ctx, `SELECT e.embedding FROM photo_embedding e JOIN photo p ON p.id = e.photo_id
		WHERE p.user_id=$1 AND e.profile=$2 AND p.id=$3`, userID, profile, id).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.Search(ctx, userID, profile, v.Slice(), limit, id)
}

func (s *Store) Info(ctx context.Context) (map[string]string, error) {
	var server, vector string
	err := s.db.QueryRow(ctx, `SELECT current_setting('server_version'),
		(SELECT extversion FROM pg_extension WHERE extname='vector')`).Scan(&server, &vector)
	return map[string]string{"postgres": strings.Fields(server)[0], "pgvector": vector}, err
}
