// Package db opens the pgx pool shared by every route package.
package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open connects and waits up to wait for PostgreSQL (like Flask's wait-ready).
func Open(ctx context.Context, url string, wait time.Duration) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(wait)
	for {
		if err = pool.Ping(ctx); err == nil {
			return pool, nil
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			pool.Close()
			return nil, err
		}
		time.Sleep(time.Second)
	}
}
