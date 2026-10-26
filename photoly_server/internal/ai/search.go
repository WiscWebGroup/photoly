package ai

// Search (services/ai_search.py): text, image, similar-photo and browser-vector queries
// against the active profile, plus capabilities. Replaces Redis with in-process state: a
// per-user one-query-at-a-time guard and the short-lived similar-photos cursors.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	queryBudget = 60 * time.Second
	busyTTL     = 65 * time.Second
	cursorTTL   = 900 * time.Second
	cursorMax   = 2000
)

// ---- per-user guard ----

type guard struct {
	mu sync.Mutex
	m  map[int64]guardEntry
}
type guardEntry struct {
	token string
	until time.Time
}

func (g *guard) acquire(uid int64) (string, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.m == nil {
		g.m = map[int64]guardEntry{}
	}
	if e, ok := g.m[uid]; ok && time.Now().Before(e.until) {
		return "", false
	}
	t := newUUID()
	g.m[uid] = guardEntry{t, time.Now().Add(busyTTL)}
	return t, true
}

func (g *guard) release(uid int64, token string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.m[uid].token == token {
		delete(g.m, uid)
	}
}

// ---- similar-photos cursors (similar_cursor.py) ----

type simCtx struct {
	ID, Owner           string
	UID, Photo, Profile int64
	Version             int32
	Vectors             [][]float32
	Size                int
	Threshold           *float64
	Expires             time.Time
}
type simPage struct {
	Query   string
	After   [2]float64
	Expires time.Time
}
type cursors struct {
	mu      sync.Mutex
	queries map[string]*simCtx
	pages   map[string]simPage
}

func (c *cursors) sweep() {
	now := time.Now()
	for k, v := range c.queries {
		if !v.Expires.After(now) {
			delete(c.queries, k)
		}
	}
	for k, v := range c.pages {
		if !v.Expires.After(now) {
			delete(c.pages, k)
		}
	}
}

func owner(token string) string { s := sha256.Sum256([]byte(token)); return hex.EncodeToString(s[:]) }

func isHex32(s string) bool {
	if len(s) != 32 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func (c *cursors) load(cursor any, uid, photo int64, tok string) (*simCtx, [2]float64, error) {
	s, _ := cursor.(string)
	if !isHex32(s) {
		return nil, [2]float64{}, fail("invalid_cursor")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sweep()
	p, ok := c.pages[s]
	if !ok {
		return nil, [2]float64{}, fail("cursor_expired", 409)
	}
	q, ok := c.queries[p.Query]
	if !ok {
		return nil, [2]float64{}, fail("cursor_expired", 409)
	}
	if q.UID != uid || q.Owner != owner(tok) || q.Photo != photo {
		return nil, [2]float64{}, fail("invalid_cursor")
	}
	return q, p.After, nil
}

func (c *cursors) create(uid, photo, pid int64, version int32, vecs [][]float32, size int, th *float64, tok string) *simCtx {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.queries == nil {
		c.queries, c.pages = map[string]*simCtx{}, map[string]simPage{}
	}
	c.sweep()
	if len(c.queries) >= cursorMax { // bounded: refuse new cursors instead of growing
		for k := range c.queries {
			delete(c.queries, k)
			break
		}
	}
	q := &simCtx{ID: strings.ReplaceAll(newUUID(), "-", ""), Owner: owner(tok), UID: uid, Photo: photo, Profile: pid, Version: version,
		Vectors: vecs, Size: size, Threshold: th, Expires: time.Now().Add(cursorTTL)}
	c.queries[q.ID] = q
	return q
}

// next returns the continuation key; a retry of the same page gets the same key.
func (c *cursors) next(q *simCtx, after [2]float64) (string, error) {
	if !q.Expires.After(time.Now()) {
		return "", fail("cursor_expired", 409)
	}
	raw, _ := json.Marshal(after)
	sum := sha256.Sum256([]byte(q.ID + string(raw)))
	key := hex.EncodeToString(sum[:])[:32]
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pages == nil {
		c.queries, c.pages = map[string]*simCtx{}, map[string]simPage{}
	}
	c.pages[key] = simPage{q.ID, after, q.Expires}
	return key, nil
}

// ---- service ----

type Search struct {
	pool    *pgxpool.Pool
	rt      *Runtime
	store   *Store
	root    string
	busy    guard
	cursors cursors
}

func eqFloat(a, b *float64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func floatPtr(v any) *float64 {
	if f, ok := toFloat(v); ok && v != nil {
		return &f
	}
	return nil
}

func sortedFormats() []string {
	set := map[string]bool{}
	for k := range imageExt {
		set[k] = true
	}
	for k := range videoExt {
		set[k] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func sortedVideoFormats() []string {
	out := make([]string, 0, len(videoExt))
	for k := range videoExt {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// userPlacement returns the effective placement for one user plus what they chose.
func userPlacement(ctx context.Context, q Q, uid int64, cfg Settings) (map[string]any, error) {
	pref := map[string]string{}
	var img, txt string
	err := q.QueryRow(ctx, `SELECT image, text FROM user_ai_preference WHERE user_id=$1`, uid).Scan(&img, &txt)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		pref["image"], pref["text"] = img, txt
	}
	out := map[string]any{"choosable": map[string]bool{}, "preference": map[string]string{}}
	for _, kind := range []string{"image", "text"} {
		admin := cfg.Str("placement_" + kind)
		out[kind] = effectivePlacement(admin, pref[kind])
		out["choosable"].(map[string]bool)[kind] = admin == "user"
		if v, ok := pref[kind]; ok && admin == "user" {
			out["preference"].(map[string]string)[kind] = v
		}
	}
	return out, nil
}

func browserInfo(cfg Settings) map[string]any {
	p := ONNXProfile()
	files := map[string]string{}
	for name, f := range ModelFiles() {
		files[name] = f.SHA256
	}
	mb := func(name string) float64 { return math.Round(float64(modelFiles[name].Size)/1e5) / 10 }
	return map[string]any{"fingerprint": p.Fingerprint, "modelName": p.ModelName, "dtype": p.Dtype, "textModel": cfg.Str("browser_text_model"),
		"files": files, "preprocessing": p.Preprocessing,
		"sizesMB": map[string]float64{"image": mb(p.VisionFile), "textSmall": mb(textInt8File), "textFull": mb(p.TextFile)}}
}

func (s *Search) Capabilities(ctx context.Context, uid int64) (map[string]any, error) {
	cfg, err := loadSettings(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	var eligible bool
	if err := s.pool.QueryRow(ctx, `SELECT ai_search_enabled FROM "user" WHERE user_id=$1`, uid).Scan(&eligible); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var profile *profileRow
	if pid := cfg.PID("search_profile_id"); pid != nil {
		if profile, err = getProfile(ctx, s.pool, *pid); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	place, err := userPlacement(ctx, s.pool, uid, cfg)
	if err != nil {
		return nil, err
	}
	var reason any
	switch {
	case !cfg.Bool("enabled") || !cfg.Bool("search_enabled"):
		reason = "disabled"
	case !eligible:
		reason = "not_enabled_for_user"
	case cfg.Bool("maintenance"):
		reason = "maintenance"
	case profile == nil:
		reason = "no_active_profile"
	}
	ready := false
	var profileDTO any
	var profileID any
	if profile != nil {
		profileID = profile.ID
		profileDTO = map[string]any{"provider": profile.Provider, "dimension": profile.Dimension, "modelId": profile.Spec["model_id"]}
	}
	if reason == nil {
		ok := onnxCompatible(profile.Spec) && profile.IndexStatus == "ready"
		// Text queries embedded in the browser need no server model; everything else does.
		serverOK := s.rt.Ready()
		if ok && (serverOK || place["text"] == "browser") {
			ready = true
		} else {
			reason = "model_not_ready"
			if s.rt.Status()["state"] == "failed" && s.rt.Status()["reason"] == "runtime_offline" {
				reason = "runtime_offline"
			}
		}
	}
	cov, err := coverage(ctx, s.pool, cfg.PID("search_profile_id"), &uid)
	if err != nil {
		return nil, err
	}
	return map[string]any{"enabled": eligible, "globalEnabled": cfg.Bool("enabled") && cfg.Bool("search_enabled"), "ready": ready, "reason": reason,
		"profileId": profileID, "profile": profileDTO, "coverage": cov, "modalities": []string{"text", "image", "similar"},
		"limits": map[string]any{"textCharacters": 512, "imageBytes": maxImageBytes, "imagePixels": 40_000_000, "topK": 100,
			"videoBytes": maxVideoBytes(), "videoFrames": maxFrames, "videoExtractionSeconds": extractSeconds},
		"similarityThreshold": cfg["similarity_threshold"], "formats": sortedFormats(), "videoFormats": sortedVideoFormats(), "videoSamplingVersion": samplingVersion,
		"placement": place, "modelBaseUrl": ONNXProfile().ModelBaseURL, "browser": browserInfo(cfg)}, nil
}

// Query is one search request after validation by the HTTP layer.
type Query struct {
	TopK        any
	NS, GA      any
	Text        any
	Image       []byte
	PhotoID     *int64
	Vector      []float32 // from the browser
	Fingerprint string
	Page        bool
	Cursor      any
	PageSize    any
	Token       string // caller's session token (binds similar cursors)
}

func (s *Search) intFilter(ctx context.Context, uid int64, v any, table, col string) (*int64, error) {
	if v == nil {
		return nil, nil
	}
	n, ok := toInt(v)
	if !ok || n <= 0 {
		return nil, fail("invalid_filter")
	}
	var owner int64
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`SELECT user_id FROM %s WHERE %s=$1`, table, col), n).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && owner != uid) {
		return nil, fail("filter_not_found", 404)
	}
	return &n, err
}

func (s *Search) Page(ctx context.Context, uid, photoID int64, data map[string]any, token string) (map[string]any, error) {
	q := Query{PhotoID: &photoID, Page: true, Token: token}
	if c, ok := data["cursor"]; ok {
		if len(data) != 1 {
			return nil, fail("invalid_pagination")
		}
		q.Cursor = c
	} else {
		for k := range data {
			if k != "pageSize" {
				return nil, fail("invalid_pagination")
			}
		}
		size := any(int64(24))
		if v, ok := data["pageSize"]; ok {
			size = v
		}
		n, ok := toInt(size)
		if !ok || n < 1 || n > 60 {
			return nil, fail("invalid_page_size")
		}
		q.PageSize = n
	}
	return s.Search(ctx, uid, q)
}

func (s *Search) Search(ctx context.Context, uid int64, in Query) (out map[string]any, err error) {
	started := time.Now()
	ctx, cancel := context.WithTimeout(ctx, queryBudget+5*time.Second)
	defer cancel()
	vectorQuery := in.Vector != nil
	imageQuery := in.PhotoID != nil || in.Image != nil
	// Pagination fixes topK to the page size.
	var k int64 = 20
	topK := in.TopK
	if in.Page && in.PageSize != nil {
		topK = in.PageSize
	}
	if topK != nil {
		var ok bool
		if k, ok = toInt(topK); !ok || k < 1 || k > 100 {
			return nil, fail("invalid_top_k")
		}
	}
	ns, err := s.intFilter(ctx, uid, in.NS, "namespace", "ns_id")
	if err != nil {
		return nil, err
	}
	ga, err := s.intFilter(ctx, uid, in.GA, "gallery", "ga_id")
	if err != nil {
		return nil, err
	}
	text, _ := in.Text.(string)
	if !imageQuery && !vectorQuery && (strings.TrimSpace(text) == "" || len([]rune(text)) > 512) {
		return nil, fail("invalid_text")
	}
	if vectorQuery {
		if in.Fingerprint != ONNXProfile().Fingerprint {
			return nil, fail("profile_mismatch", 409)
		}
		if err := validateEmbedding(in.Vector); err != nil {
			return nil, fail("invalid_embedding")
		}
	}
	token, ok := s.busy.acquire(uid)
	if !ok {
		return nil, fail("user_query_busy", 429)
	}
	var jobID int64
	completed := false
	defer func() {
		s.busy.release(uid, token)
		if jobID != 0 {
			status := "failed"
			if completed {
				status = "succeeded"
			}
			_, _ = s.pool.Exec(context.WithoutCancel(ctx), `UPDATE ai_job SET status=$2, finished_at=now() WHERE job_id=$1`, jobID, status)
		}
	}()

	// Admission: same locks as every writer (user row, then the global control row).
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var enabled bool
	err = tx.QueryRow(ctx, `SELECT ai_search_enabled FROM "user" WHERE user_id=$1 FOR UPDATE`, uid).Scan(&enabled)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if _, err = control(ctx, tx, "global", true); err != nil {
		return nil, err
	}
	cfg, err := loadSettings(ctx, tx)
	if err != nil {
		return nil, err
	}
	var qctx *simCtx
	var after *[2]float64
	if in.Page && in.Cursor != nil {
		c, a, err := s.cursors.load(in.Cursor, uid, *in.PhotoID, in.Token)
		if err != nil {
			return nil, err
		}
		qctx, after = c, &a
		k = int64(c.Size)
		if !eqFloat(c.Threshold, floatPtr(cfg["similarity_threshold"])) {
			return nil, fail("search_settings_changed", 409)
		}
	}
	if !enabled || !cfg.Bool("enabled") || !cfg.Bool("search_enabled") {
		return nil, fail("ai_not_enabled", 403)
	}
	if cfg.Bool("maintenance") {
		return nil, fail("maintenance", 503)
	}
	var profile *profileRow
	if pid := cfg.PID("search_profile_id"); pid != nil {
		if profile, err = getProfile(ctx, tx, *pid); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	if profile == nil || profile.IndexStatus != "ready" {
		return nil, fail("profile_not_ready", 503)
	}
	pid := profile.ID
	if err = tx.QueryRow(ctx, `INSERT INTO ai_job (kind, profile_id, dedup_key, status, lease_token, lease_expires_at)
		VALUES ('search',$1,$2,'running',$3,now()+interval '60 seconds') RETURNING job_id`, pid, "search:"+token, token).Scan(&jobID); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	if !onnxCompatible(profile.Spec) {
		return nil, fail("model_not_ready", 503)
	}

	// Gather the query vectors.
	var vectors [][]float32
	var sourceVersion int32
	if in.PhotoID != nil {
		var owner int64
		err = s.pool.QueryRow(ctx, `SELECT user_id, content_version FROM photo WHERE photo_id=$1`, *in.PhotoID).Scan(&owner, &sourceVersion)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && owner != uid) {
			return nil, fail("photo_not_found", 404)
		}
		if err != nil {
			return nil, err
		}
		if qctx != nil {
			if qctx.Profile != pid {
				return nil, fail("profile_changed", 409)
			}
			if qctx.Version != sourceVersion {
				return nil, fail("stale_content", 409)
			}
			vectors = qctx.Vectors
		}
		if vectors == nil && profile.Spec["query_instruction"] == profile.Spec["document_instruction"] {
			if vectors, err = storedVectors(ctx, s.pool, *in.PhotoID, pid, sourceVersion); err != nil {
				return nil, err
			}
		}
	}
	switch {
	case vectorQuery:
		vectors = [][]float32{in.Vector}
	case vectors == nil:
		if s.rt.Status()["state"] != "ready" {
			return nil, fail("model_not_ready", 503)
		}
		var frames [][]byte
		if in.PhotoID != nil {
			sctx, c := context.WithDeadline(ctx, started.Add(queryBudget))
			snap, err := snapshotPhoto(sctx, s.pool, s.root, *in.PhotoID, &uid, nil)
			c()
			if err != nil {
				return nil, err
			}
			sourceVersion = snap.Version
			if snap.Frames != nil {
				for _, f := range snap.Frames {
					frames = append(frames, f.Data)
				}
			} else {
				frames = [][]byte{snap.Data}
			}
		} else if in.Image != nil {
			frames = [][]byte{in.Image}
		}
		fp := ONNXProfile().Fingerprint
		for _, data := range frames {
			left := queryBudget - time.Since(started)
			if left <= 0 {
				return nil, fail("query_timeout", 504)
			}
			pixels, err := prepareImage(data)
			if err != nil {
				return nil, mapImageErr(err)
			}
			vec, err := s.rt.Encode(&encItem{kind: "image", pixels: pixels, fingerprint: fp}, false, left)
			if err != nil {
				return nil, err
			}
			vectors = append(vectors, vec)
		}
		if frames == nil {
			left := queryBudget - time.Since(started)
			if left <= 0 {
				return nil, fail("query_timeout", 504)
			}
			vec, err := s.rt.Encode(&encItem{kind: "text", text: text, fingerprint: fp}, false, left)
			if err != nil {
				return nil, err
			}
			vectors = [][]float32{vec}
		}
	}

	// Re-check eligibility: settings may have changed while the model ran.
	cfg, err = loadSettings(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	if err = s.pool.QueryRow(ctx, `SELECT ai_search_enabled FROM "user" WHERE user_id=$1`, uid).Scan(&enabled); err != nil || !enabled || !cfg.Bool("enabled") {
		return nil, fail("ai_not_enabled", 403)
	}
	if cfg.Bool("maintenance") || !cfg.isPID("search_profile_id", pid) {
		return nil, fail("profile_changed", 409)
	}
	if in.PhotoID != nil {
		var v int32
		if err = s.pool.QueryRow(ctx, `SELECT content_version FROM photo WHERE photo_id=$1`, *in.PhotoID).Scan(&v); err != nil || v != sourceVersion {
			return nil, fail("stale_content", 409)
		}
	}
	var threshold *float64
	if imageQuery {
		threshold = floatPtr(cfg["similarity_threshold"])
	}
	if qctx != nil && !eqFloat(qctx.Threshold, threshold) {
		return nil, fail("search_settings_changed", 409)
	}
	left := queryBudget - time.Since(started)
	if left <= 0 {
		return nil, fail("query_timeout", 504)
	}
	f := filter{NS: ns, GA: ga, MinScore: threshold, After: after}
	f.Exclude = in.PhotoID
	cov := func() (map[string]any, error) { return coverage(ctx, s.pool, &pid, &uid) }
	if in.Page {
		hits, boundary, err := s.store.Page(ctx, pid, vectors, uid, clampInt(k), f, left)
		if err != nil {
			return nil, err
		}
		var next any
		if boundary != nil {
			if qctx == nil {
				qctx = s.cursors.create(uid, *in.PhotoID, pid, sourceVersion, vectors, clampInt(k), threshold, in.Token)
			}
			if next, err = s.cursors.next(qctx, *boundary); err != nil {
				return nil, err
			}
		}
		c, err := cov()
		if err != nil {
			return nil, err
		}
		completed = true
		return map[string]any{"items": hitsDTO(hits), "profileId": pid, "coverage": c, "requestId": token,
			"similarityThreshold": threshold, "nextCursor": next, "hasMore": next != nil}, nil
	}
	f.After = nil
	hits, err := s.store.Search(ctx, pid, vectors, uid, clampInt(k), f, left)
	if err != nil {
		return nil, err
	}
	c, err := cov()
	if err != nil {
		return nil, err
	}
	completed = true
	return map[string]any{"items": hitsDTO(hits), "profileId": pid, "coverage": c, "requestId": token, "similarityThreshold": threshold}, nil
}

func hitsDTO(hits []Hit) []map[string]any {
	out := make([]map[string]any, len(hits))
	for i, h := range hits {
		out[i] = h.dto()
	}
	return out
}

// mapImageErr turns a decode failure into the API's image error codes.
func mapImageErr(err error) error {
	switch {
	case errors.Is(err, errUnsupportedFormat):
		return fail("unsupported_image", 415)
	case errors.Is(err, errTooManyPixels):
		return fail("image_too_many_pixels", 413)
	}
	return fail("invalid_image", 415)
}

// ---- HTTP handlers ----

func readImageForm(r *http.Request) (map[string]any, []byte, error) {
	if err := r.ParseMultipartForm(maxImageBytes + 1<<20); err != nil {
		return nil, nil, fail("image_required")
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		return nil, nil, fail("image_required")
	}
	defer f.Close()
	data := map[string]any{}
	for _, key := range []string{"topK", "namespaceId", "galleryId"} {
		if v := r.FormValue(key); v != "" || r.Form.Has(key) {
			n, ok := toInt(json.Number(strings.TrimSpace(v)))
			if !ok {
				return nil, nil, fail("invalid_filter")
			}
			data[key] = n
		}
	}
	img, err := readBounded(f)
	return data, img, err
}

// vectorFrom reads a JSON array of numbers into float32s.
func vectorFrom(v any) ([]float32, bool) {
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]float32, len(arr))
	for i, x := range arr {
		if _, isBool := x.(bool); isBool {
			return nil, false
		}
		f, ok := toFloat(x)
		if !ok {
			return nil, false
		}
		out[i] = float32(f)
	}
	return out, true
}
