// Command photoly is the single Photoly backend binary.
//
//	photoly serve      migrate (if permitted), then serve the API; unported routes go to FLASK_URL
//	photoly migrate    apply embedded migrations + seed settings/admin, then exit
//	photoly ai-worker  SigLIP2 inference child process (internal/ai)
//	photoly verify     self-checks (internal/ai)
//	photoly health     exit 0 if the local server answers /healthz
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	photolyserver "github.com/WiscWebGroup/photoly/photoly_server"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/config"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/migrations"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/server"
)

// Set by ai.go when internal/ai exposes them.
var aiWorker, aiVerify func(ctx context.Context) error

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, cmd); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, cmd string) error {
	cfg := config.Load()
	switch cmd {
	case "serve":
		murl := config.DatabaseURL(true)
		if os.Getenv("PHOTOLY_SKIP_MIGRATE") == "" {
			// Wait for PostgreSQL, then migrate; a role without DDL rights only checks.
			if err := server.Migrate(ctx, cfg, murl); err != nil {
				log.Printf("photoly: migrate failed (%v); checking schema instead", err)
				if err := migrations.Check(ctx, cfg.DatabaseURL); err != nil {
					return err
				}
			}
		}
		a, h, err := server.New(ctx, cfg, photolyserver.OpenAPI)
		if err != nil {
			return err
		}
		return server.Serve(ctx, a, h)
	case "migrate":
		if err := server.Migrate(ctx, cfg, config.DatabaseURL(true)); err != nil {
			return err
		}
		log.Print("photoly: database migrated and seeded")
		return nil
	case "ai-worker":
		return dispatch(ctx, "ai-worker", aiWorker)
	case "verify":
		return dispatch(ctx, "verify", aiVerify)
	case "health":
		addr := cfg.Addr
		if strings.HasPrefix(addr, ":") {
			addr = "127.0.0.1" + addr
		}
		ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/healthz", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("healthz: HTTP %d", resp.StatusCode)
		}
		return nil
	}
	fmt.Fprintln(os.Stderr, "usage: photoly [serve|migrate|ai-worker|verify|health]")
	os.Exit(2)
	return nil
}

func dispatch(ctx context.Context, name string, f func(context.Context) error) error {
	if f == nil {
		return fmt.Errorf("%s: internal/ai is not wired in this build yet", name)
	}
	return f(ctx)
}
