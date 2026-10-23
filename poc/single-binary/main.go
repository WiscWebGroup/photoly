// Command photoly is a proof of concept for running the Photoly backend as one Go binary,
// with SigLIP2 image and text embeddings computed in the browser over WebGPU.
//
//	photoly serve        migrate, then serve the API, web UI and model files
//	photoly migrate      apply database migrations and exit
//	photoly fetch-model  download and verify the model files ahead of time
//	photoly health       exit 0 if the local server answers /healthz (for containers)
//
// Configuration comes from the environment; see README.md.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch cmd {
	case "serve":
		err = serve(ctx)
	case "migrate":
		err = withStore(ctx, func(s *Store) error { return s.Migrate(ctx) })
	case "fetch-model":
		err = fetchModel(ctx)
	case "health":
		err = health(ctx)
	default:
		fmt.Fprintln(os.Stderr, "usage: photoly [serve|migrate|fetch-model|health]")
		os.Exit(2)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func dataDir() string { return env("PHOTOLY_DATA", "data") }

func withStore(ctx context.Context, fn func(*Store) error) error {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("set DATABASE_URL, e.g. postgres://user:pass@localhost:5432/photoly")
	}
	s, err := openStore(ctx, url)
	if err != nil {
		return err
	}
	defer s.Close()
	return fn(s)
}

// health lets a container check the server without curl or a shell in the image.
func health(ctx context.Context) error {
	addr := env("PHOTOLY_ADDR", ":8084")
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/healthz", nil)
	if err != nil {
		return err
	}
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

func fetchModel(ctx context.Context) error {
	profile, err := newProfile(env("PHOTOLY_MODEL_DTYPE", "fp32"))
	if err != nil {
		return err
	}
	cache := newModelCache(filepath.Join(dataDir(), "models"))
	for name := range modelFiles {
		if filepath.Ext(name) == ".onnx" && name != profile.VisionFile && name != profile.TextFile {
			continue
		}
		if _, err := cache.Ensure(ctx, name); err != nil {
			return err
		}
	}
	return nil
}

func serve(ctx context.Context) error {
	profile, err := newProfile(env("PHOTOLY_MODEL_DTYPE", "fp32"))
	if err != nil {
		return err
	}
	photos := filepath.Join(dataDir(), "photos")
	if err := os.MkdirAll(photos, 0o755); err != nil {
		return err
	}
	return withStore(ctx, func(store *Store) error {
		if err := store.Migrate(ctx); err != nil {
			return err
		}
		mux := http.NewServeMux()
		(&API{store: store, profile: profile, dir: photos}).routes(mux)
		mux.Handle("GET /models/{rev}/{model}/{file...}", newModelCache(filepath.Join(dataDir(), "models")))
		mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
			if err := store.db.Ping(r.Context()); err != nil {
				http.Error(w, "database unavailable", http.StatusServiceUnavailable)
				return
			}
			w.Write([]byte("ok"))
		})
		mux.Handle("GET /", staticHandler())

		srv := &http.Server{
			Addr:              env("PHOTOLY_ADDR", ":8084"),
			Handler:           isolate(mux),
			ReadHeaderTimeout: 10 * time.Second,
		}
		go func() {
			<-ctx.Done()
			shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			srv.Shutdown(shutdown)
		}()
		log.Printf("photoly: listening on %s (model %s, fingerprint %s)", srv.Addr, profile.Dtype, profile.Fingerprint[:12])
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
}

// isolate makes the page cross-origin isolated, which multi-threaded WebAssembly needs
// (SharedArrayBuffer). Everything is served from this origin, so nothing else breaks.
func isolate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}
