// Package server wires config, database, sessions, auth and the routes.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/db"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/httpx"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/migrations"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/session"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	"github.com/jackc/pgx/v5"
)

type ctxKey struct{}

// Hooks are optional extension points filled by routes.go (and cmd) so a missing package
// never breaks the build.
var (
	Registrars []func(*app.App)
	// Background goroutines started with serve (upload worker, AI scheduler...); stop on ctx.
	Background []func(ctx context.Context, a *app.App)
)

// Migrate applies migrations and seeds the database (the `migrate` subcommand).
func Migrate(ctx context.Context, cfg app.Config, migrationURL string) error {
	if err := migrations.Run(ctx, migrationURL); err != nil {
		return err
	}
	conn, err := pgx.Connect(ctx, migrationURL)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	return migrations.Seed(ctx, conn, cfg)
}

// New builds the App and the root handler. openapi is served at /openapi.json.
func New(ctx context.Context, cfg app.Config, openapi []byte) (*app.App, http.Handler, error) {
	pool, err := db.Open(ctx, cfg.DatabaseURL, 60*time.Second)
	if err != nil {
		return nil, nil, err
	}
	storage.Init(cfg.UploadFolder)
	if err := storage.Startup(ctx, pool); err != nil {
		return nil, nil, fmt.Errorf("storage: %w", err)
	}
	store := session.Store{Pool: pool}
	legacy.Sessions = store
	go store.Sweep(ctx)
	go storage.Sweep(ctx)

	mux := http.NewServeMux()
	a := &app.App{Cfg: cfg, Pool: pool, Mux: mux}
	a.Auth = func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			u, ok := store.Lookup(r.Context(), r.Header.Get("HRD-Token"))
			if !ok {
				httpx.JSON(w, nil, 403) // HTTP 200 with msgCode 403, like Flask
				return
			}
			next(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
		}
	}
	a.Admin = func(next http.HandlerFunc) http.HandlerFunc {
		return a.Auth(func(w http.ResponseWriter, r *http.Request) {
			if a.UserFrom(r.Context()).Role != "admin" {
				httpx.JSON(w, -2, 400) // BusinessError(-2)
				return
			}
			next(w, r)
		})
	}
	a.UserFrom = func(ctx context.Context) app.User {
		u, _ := ctx.Value(ctxKey{}).(app.User)
		return u
	}

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			httpx.JSON(w, nil, 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(openapi)
	})
	for _, register := range Registrars {
		register(a)
	}
	// "/x/" and "/x" are the same route.
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p := r.URL.Path; len(p) > 1 && p[len(p)-1] == '/' {
			r.URL.Path = p[:len(p)-1]
			r.URL.RawPath = ""
		}
		if cfg.MaxContentLength > 0 {
			r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxContentLength)
		}
		mux.ServeHTTP(w, r)
	})
	return a, h, nil
}

// Serve runs the HTTP server until ctx ends, then drains for 15s.
func Serve(ctx context.Context, a *app.App, h http.Handler) error {
	srv := &http.Server{Addr: a.Cfg.Addr, Handler: h, ReadHeaderTimeout: 15 * time.Second}
	for _, bg := range Background {
		go bg(ctx, a)
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Printf("photoly: listening on %s", a.Cfg.Addr)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	sctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	a.Pool.Close()
	return nil
}
