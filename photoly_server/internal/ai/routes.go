package ai

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/uploads"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// sink stores browser-made embeddings (uploads.EmbeddingSink).
type sink struct{ pool *pgxpool.Pool }

func (k sink) StoreClient(ctx context.Context, userID, photoID int64, fingerprint string, vector []float32) (string, error) {
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var enabled bool
	err = tx.QueryRow(ctx, `SELECT ai_search_enabled FROM "user" WHERE user_id=$1 FOR UPDATE`, userID).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return uploads.Queued, nil
	}
	if err != nil {
		return "", err
	}
	cfg, err := loadSettings(ctx, tx)
	if err != nil {
		return "", err
	}
	var choice string
	_ = tx.QueryRow(ctx, `SELECT image FROM user_ai_preference WHERE user_id=$1`, userID).Scan(&choice)
	rejected := uploads.Queued
	if effectivePlacement(cfg.Str("placement_image"), choice) == "browser" {
		rejected = uploads.Waiting
	}
	if !enabled || !cfg.Bool("enabled") || fingerprint != ONNXProfile().Fingerprint {
		return rejected, nil
	}
	var version int32
	var sha *string
	if err := tx.QueryRow(ctx, `SELECT content_version, content_sha256 FROM photo WHERE photo_id=$1 AND user_id=$2`, photoID, userID).Scan(&version, &sha); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return rejected, nil
		}
		return "", err
	}
	h := ""
	if sha != nil {
		h = *sha
	}
	stored := false
	for _, key := range []string{"search_profile_id", "building_profile_id"} {
		pid := cfg.PID(key)
		if pid == nil {
			continue
		}
		p, err := getProfile(ctx, tx, *pid)
		if err != nil || !onnxCompatible(p.Spec) || p.Dimension != len(vector) {
			continue
		}
		if err := upsertState(ctx, tx, photoID, *pid, version, h, [][]float32{vector}, nil, 0, "client"); err != nil {
			return "", err
		}
		if _, err := tx.Exec(ctx, `UPDATE ai_job SET status='cancelled', reserved_bytes=0 WHERE kind='embedding' AND photo_id=$1 AND profile_id=$2 AND status='queued'`, photoID, *pid); err != nil {
			return "", err
		}
		stored = true
	}
	if !stored {
		return rejected, nil
	}
	return uploads.Stored, tx.Commit(ctx)
}

func idParam(r *http.Request, name string) (int64, error) {
	n, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil {
		return 0, fail("not_found", 404)
	}
	return n, nil
}

func queryMap(r *http.Request) map[string]string {
	m := map[string]string{}
	for k, v := range r.URL.Query() {
		m[k] = v[0]
	}
	return m
}

// Register mounts the AI routes, starts the runtime, scheduler and maintenance loops, and refuses
// to start while a Qwen profile is active.
func Register(a *app.App) {
	ctx := context.Background()
	must(CheckStartup(ctx, a.Pool))
	rt := NewRuntime(a.Cfg.UploadFolder)
	gaz, err := loadGazetteer()
	must(err)
	smart := &Smart{pool: a.Pool, root: a.Cfg.UploadFolder, gaz: gaz}
	search := &Search{pool: a.Pool, rt: rt, store: &Store{pool: a.Pool}, root: a.Cfg.UploadFolder}
	admin := &Admin{pool: a.Pool, rt: rt, smart: smart, root: a.Cfg.UploadFolder}
	uploads.Sink = sink{a.Pool}
	owner := newUUID()
	sched := &Scheduler{pool: a.Pool, rt: rt, root: a.Cfg.UploadFolder, owner: owner}
	sched.smart = &SmartSched{pool: a.Pool, root: a.Cfg.UploadFolder, owner: owner, gaz: gaz, appZone: env("TIMEZONE", "America/New_York")}
	sched.Start(ctx)

	m := a.Mux
	user := func(pattern string, fn func(r *http.Request, u app.User) (any, error)) {
		m.HandleFunc(pattern, Route(a, false, fn))
	}
	adm := func(pattern string, fn func(r *http.Request, u app.User) (any, error)) {
		m.HandleFunc(pattern, Route(a, true, fn))
	}

	user("GET /ai/capabilities", func(r *http.Request, u app.User) (any, error) { return search.Capabilities(r.Context(), u.ID) })
	user("POST /ai/search/text", func(r *http.Request, u app.User) (any, error) {
		d, err := body(r)
		if err != nil {
			return nil, err
		}
		return search.Search(r.Context(), u.ID, Query{TopK: d["topK"], NS: d["namespaceId"], GA: d["galleryId"], Text: d["text"]})
	})
	user("POST /ai/search/vector", func(r *http.Request, u app.User) (any, error) {
		d, err := body(r)
		if err != nil {
			return nil, err
		}
		vec, ok := vectorFrom(d["vector"])
		fp, _ := d["fingerprint"].(string)
		if !ok {
			return nil, fail("invalid_embedding")
		}
		return search.Search(r.Context(), u.ID, Query{TopK: d["topK"], NS: d["namespaceId"], GA: d["galleryId"], Vector: vec, Fingerprint: fp})
	})
	user("POST /ai/search/image", func(r *http.Request, u app.User) (any, error) {
		d, img, err := readImageForm(r)
		if err != nil {
			return nil, err
		}
		return search.Search(r.Context(), u.ID, Query{TopK: d["topK"], NS: d["namespaceId"], GA: d["galleryId"], Image: img})
	})
	user("POST /ai/photos/{photoId}/similar", func(r *http.Request, u app.User) (any, error) {
		id, err := idParam(r, "photoId")
		if err != nil {
			return nil, err
		}
		d, err := body(r)
		if err != nil {
			return nil, err
		}
		return search.Search(r.Context(), u.ID, Query{TopK: d["topK"], NS: d["namespaceId"], GA: d["galleryId"], PhotoID: &id})
	})
	user("POST /ai/photos/{photoId}/similar/page", func(r *http.Request, u app.User) (any, error) {
		id, err := idParam(r, "photoId")
		if err != nil {
			return nil, err
		}
		d, err := body(r)
		if err != nil {
			return nil, err
		}
		return search.Page(r.Context(), u.ID, id, d, r.Header.Get("HRD-Token"))
	})
	index := &Indexer{pool: a.Pool, sink: sink{a.Pool}}
	user("POST /ai/index", func(r *http.Request, u app.User) (any, error) { return index.Trigger(r.Context(), u.ID) })
	user("GET /ai/index/pending", func(r *http.Request, u app.User) (any, error) { return index.Pending(r.Context(), u.ID, queryMap(r)) })
	user("PUT /ai/photos/{photoId}/embedding", func(r *http.Request, u app.User) (any, error) {
		id, err := idParam(r, "photoId")
		if err != nil {
			return nil, err
		}
		d, err := body(r)
		if err != nil {
			return nil, err
		}
		return index.Store(r.Context(), u.ID, id, d)
	})
	user("PUT /ai/preference", func(r *http.Request, u app.User) (any, error) {
		d, err := body(r)
		if err != nil || len(d) == 0 {
			return nil, fail("invalid_preference")
		}
		cfg, err := loadSettings(r.Context(), a.Pool)
		if err != nil {
			return nil, err
		}
		for k, v := range d {
			s, _ := v.(string)
			if (k != "image" && k != "text") || (s != "server" && s != "browser") {
				return nil, fail("invalid_preference")
			}
			if cfg.Str("placement_"+k) != "user" {
				return nil, fail("placement_not_allowed", 409)
			}
		}
		if _, err := a.Pool.Exec(r.Context(), `INSERT INTO user_ai_preference (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, u.ID); err != nil {
			return nil, err
		}
		for k, v := range d {
			if _, err := a.Pool.Exec(r.Context(), `UPDATE user_ai_preference SET `+k+`=$2, updated_at=now() WHERE user_id=$1`, u.ID, v); err != nil {
				return nil, err
			}
		}
		return userPlacement(r.Context(), a.Pool, u.ID, cfg)
	})

	user("GET /ai/smart/settings", func(r *http.Request, u app.User) (any, error) { return smart.GetSettings(r.Context(), u.ID) })
	user("PATCH /ai/smart/settings", func(r *http.Request, u app.User) (any, error) {
		d, err := body(r)
		if err != nil {
			return nil, err
		}
		return smart.PatchSettings(r.Context(), u.ID, d)
	})
	user("GET /ai/smart/status", func(r *http.Request, u app.User) (any, error) { return smart.Status(r.Context(), u.ID) })
	user("POST /ai/smart/refresh", func(r *http.Request, u app.User) (any, error) { return smart.Refresh(r.Context(), u.ID) })
	user("GET /ai/smart/dismissed", func(r *http.Request, u app.User) (any, error) {
		after, err := int64(0), error(nil)
		if v := r.URL.Query().Get("after"); v != "" {
			after, err = strconv.ParseInt(v, 10, 64)
		}
		if err != nil {
			return nil, fail("invalid_pagination")
		}
		return smart.Dismissed(r.Context(), u.ID, after)
	})
	user("POST /ai/smart/items/{itemId}/restore", func(r *http.Request, u app.User) (any, error) {
		id, err := idParam(r, "itemId")
		if err != nil {
			return nil, err
		}
		return smart.Restore(r.Context(), u.ID, id)
	})

	adm("GET /admin/ai/status", func(r *http.Request, u app.User) (any, error) { return admin.Status(r.Context()) })
	adm("GET /admin/ai/capabilities", func(r *http.Request, u app.User) (any, error) { return admin.Capabilities(), nil })
	adm("GET /admin/ai/settings", func(r *http.Request, u app.User) (any, error) { return admin.GetSettings(r.Context()) })
	adm("PATCH /admin/ai/settings", func(r *http.Request, u app.User) (any, error) {
		d, err := body(r)
		if err != nil {
			return nil, err
		}
		return admin.PatchSettings(r.Context(), d["generation"], d["settings"])
	})
	adm("GET /admin/ai/profiles", func(r *http.Request, u app.User) (any, error) { return admin.Profiles(r.Context()) })
	adm("POST /admin/ai/profiles", func(r *http.Request, u app.User) (any, error) {
		d, err := body(r)
		if err != nil || len(d) != 1 || d["provider"] == nil {
			return nil, fail("invalid_profile")
		}
		return admin.CreateProfile(r.Context(), d["provider"])
	})
	adm("PATCH /admin/users/{userId}/ai", func(r *http.Request, u app.User) (any, error) {
		id, err := idParam(r, "userId")
		if err != nil {
			return nil, err
		}
		d, err := body(r)
		if err != nil {
			return nil, err
		}
		return admin.SetUser(r.Context(), id, d["enabled"])
	})
	adm("GET /admin/ai/users", func(r *http.Request, u app.User) (any, error) {
		var ids []int64
		for _, p := range strings.Split(r.URL.Query().Get("ids"), ",") {
			if p == "" {
				continue
			}
			n, err := strconv.ParseInt(p, 10, 64)
			if err != nil {
				return nil, fail("invalid_user_ids")
			}
			ids = append(ids, n)
		}
		return admin.UserStates(r.Context(), ids)
	})
	adm("POST /admin/ai/operations", func(r *http.Request, u app.User) (any, error) {
		d, err := body(r)
		if err != nil {
			return nil, err
		}
		return admin.Operation(r.Context(), u.ID, d, r.Header.Get("Idempotency-Key"))
	})
	adm("GET /admin/ai/operations/{operationId}", func(r *http.Request, u app.User) (any, error) {
		id, err := idParam(r, "operationId")
		if err != nil {
			return nil, err
		}
		return admin.GetOperation(r.Context(), id)
	})
	adm("GET /admin/ai/jobs", func(r *http.Request, u app.User) (any, error) { return admin.Jobs(r.Context(), queryMap(r)) })
	adm("POST /admin/ai/models/{kind}/load", func(r *http.Request, u app.User) (any, error) {
		return admin.LoadModel(r.PathValue("kind"), r.URL.Query().Get("force") == "1")
	})
	adm("POST /admin/ai/models/{kind}/unload", func(r *http.Request, u app.User) (any, error) { return admin.UnloadModel(r.PathValue("kind")) })

	// Browser model files: public, immutable, pinned by SHA-256 (the revision is in the URL).
	cache := newModelCache(filepath.Join(env("PHOTOLY_DATA", "data"), "models"))
	_ = os.MkdirAll(cache.dir, 0o755)
	m.Handle("GET /models/{rev}/{model}/{file...}", cache)
}
