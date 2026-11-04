package server

import (
	"encoding/json"
	"fmt"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/ai"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/photo"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type recordingRouter struct {
	*http.ServeMux
	patterns []string
}

func (m *recordingRouter) Handle(p string, h http.Handler) {
	m.patterns = append(m.patterns, p)
	m.ServeMux.Handle(p, h)
}
func (m *recordingRouter) HandleFunc(p string, h func(http.ResponseWriter, *http.Request)) {
	m.Handle(p, http.HandlerFunc(h))
}

var pathVariable = regexp.MustCompile(`\{[^}]+\}`)

func canonicalOperation(method, path string) string {
	return strings.ToUpper(method) + " " + strings.TrimRight(pathVariable.ReplaceAllString(path, "{}"), "/")
}

func TestOpenAPIRouteCoverage(t *testing.T) {
	mux := &recordingRouter{ServeMux: http.NewServeMux()}
	newLibraryTestWithRouter(t, mux)
	raw, err := os.ReadFile("../../openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err = json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	// Not business operations. Each exclusion is exact; newly added routes cannot hide here.
	infrastructure := map[string]string{
		"GET /docs":         "Swagger HTML (both trailing-slash aliases)",
		"GET /docs/assets":  "Bundled Swagger static assets",
		"GET /openapi.json": "This contract itself",
	}
	actual := map[string]bool{}
	for _, p := range append(mux.patterns, controllerOperations(t)...) {
		method, path, ok := strings.Cut(p, " ")
		if !ok {
			t.Fatalf("unrecognized registration %q", p)
		}
		key := canonicalOperation(method, path)
		if _, skip := infrastructure[key]; !skip {
			actual[key] = true
		}
	}
	documented := map[string]bool{}
	for path, methods := range spec.Paths {
		for method := range methods {
			if !strings.Contains(" get post put patch delete head options ", " "+method+" ") {
				continue
			}
			key := canonicalOperation(method, path)
			if documented[key] {
				t.Errorf("duplicate normalized operation %s", key)
			}
			documented[key] = true
			if !actual[key] {
				t.Errorf("stale OpenAPI operation: %s %s", method, path)
			}
		}
	}
	var inventory []string
	for key := range actual {
		inventory = append(inventory, key)
		if !documented[key] {
			t.Errorf("missing OpenAPI operation: %s", key)
		}
	}
	sort.Strings(inventory)
	t.Logf("%d actual operations, %d documented operations", len(actual), len(documented))
	if path := os.Getenv("OPENAPI_ROUTE_OUTPUT"); path != "" {
		b, _ := json.MarshalIndent(inventory, "", "  ")
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

type contractSample struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Status int    `json:"status"`
	Body   any    `json:"body"`
}

func TestOpenAPIResponses(t *testing.T) {
	f := newLibraryTest(t)
	var samples []contractSample
	capture := func(method, path, documentPath, body string, status int) {
		t.Helper()
		w := f.request(method, path, "application/json", strings.NewReader(body))
		if w.Code != status {
			t.Fatalf("%s %s: HTTP %d, want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		var v any
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err, w.Body.String())
		}
		samples = append(samples, contractSample{strings.ToLower(method), documentPath, w.Code, v})
	}
	capture("GET", "/demo", "/demo", "", 200)
	if w := f.request("GET", "/models/invalid/model/unknown.json", "", nil); w.Code != 404 {
		t.Fatalf("unknown model file: %d", w.Code)
	}
	capture("GET", "/ai/capabilities", "/ai/capabilities", "", 200)
	capture("PUT", "/ai/preference", "/ai/preference", `{}`, 400)
	capture("PUT", "/ai/preference", "/ai/preference", `{"image":"browser"}`, 200)
	capture("POST", "/ai/index", "/ai/index", "", 403)
	capture("GET", "/ai/index/pending", "/ai/index/pending", "", 403)
	capture("POST", "/ai/search/vector", "/ai/search/vector", `{"vector":false}`, 400)
	capture("PUT", "/ai/photos/1/embedding", "/ai/photos/{photoId}/embedding", `{"vector":false}`, 400)
	capture("POST", "/admin/ai/models/invalid/load", "/admin/ai/models/{kind}/load", "", 403)
	capture("POST", "/admin/ai/models/invalid/unload", "/admin/ai/models/{kind}/unload", "", 403)
	if _, err := f.a.Pool.Exec(f.ctx, `UPDATE "user" SET role='admin' WHERE user_id=$1`, f.uid); err != nil {
		t.Fatal(err)
	}
	capture("POST", "/admin/ai/models/invalid/load", "/admin/ai/models/{kind}/load", "", 404)
	capture("POST", "/admin/ai/models/invalid/unload", "/admin/ai/models/{kind}/unload", "", 404)
	capture("GET", "/admin/ai/status", "/admin/ai/status", "", 200)
	capture("GET", "/admin/ai/capabilities", "/admin/ai/capabilities", "", 200)
	capture("GET", "/admin/ai/settings", "/admin/ai/settings", "", 200)
	capture("GET", "/namespace/pins", "/namespace/pins", "", 200)
	capture("GET", "/gallery/getAll", "/gallery/getAll", "", 200)

	// Exercise success paths without loading/downloading a model. Schedulers stay stopped.
	var oldDesired []byte
	if err := f.a.Pool.QueryRow(f.ctx, `SELECT desired FROM ai_control_state WHERE name='global'`).Scan(&oldDesired); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = f.a.Pool.Exec(f.ctx, `UPDATE ai_control_state SET desired=$1::jsonb WHERE name='global'`, string(oldDesired))
	}()
	if _, err := f.a.Pool.Exec(f.ctx, `UPDATE ai_control_state SET desired=desired || '{"placement_image":"server"}'::jsonb WHERE name='global'`); err != nil {
		t.Fatal(err)
	}
	capture("PUT", "/ai/preference", "/ai/preference", `{"image":"browser"}`, 409)
	capture("POST", "/admin/ai/profiles", "/admin/ai/profiles", `{"provider":"siglip2"}`, 200)
	var pid int64
	if err := f.a.Pool.QueryRow(f.ctx, `SELECT profile_id FROM embedding_profile WHERE fingerprint=$1`, ai.ONNXProfile().Fingerprint).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	if _, err := f.a.Pool.Exec(f.ctx, `UPDATE ai_control_state SET desired=desired || jsonb_build_object('enabled',true,'search_profile_id',$1::bigint,'placement_image','user') WHERE name='global'`, pid); err != nil {
		t.Fatal(err)
	}
	if _, err := f.a.Pool.Exec(f.ctx, `UPDATE "user" SET ai_search_enabled=true WHERE user_id=$1`, f.uid); err != nil {
		t.Fatal(err)
	}
	capture("POST", "/ai/index", "/ai/index", "", 200)
	photoID := f.upload(f.root, "Audit.png")
	capture("GET", "/ai/index/pending?limit=1", "/ai/index/pending", "", 200)
	vector := make([]float64, 768)
	vector[0] = 1
	vectorBody, _ := json.Marshal(map[string]any{"fingerprint": ai.ONNXProfile().Fingerprint, "vector": vector})
	capture("PUT", fmt.Sprintf("/ai/photos/%d/embedding", photoID), "/ai/photos/{photoId}/embedding", string(vectorBody), 200)
	capture("PUT", "/ai/photos/2147483647/embedding", "/ai/photos/{photoId}/embedding", string(vectorBody), 404)
	if _, err := f.a.Pool.Exec(f.ctx, `UPDATE embedding_profile SET index_status='ready' WHERE profile_id=$1`, pid); err != nil {
		t.Fatal(err)
	}
	capture("POST", "/ai/search/vector", "/ai/search/vector", string(vectorBody), 200) // vectors do not need a loaded model
	batch, item := photo.NewUUID(), photo.NewUUID()
	f.ok("POST", "/uploads", map[string]any{"id": batch, "nsId": f.root, "name": "Audit upload"})
	f.ok("POST", "/uploads/"+batch+"/items", map[string]any{"items": []any{map[string]any{"id": item, "path": "sample.png", "size": 10}}})
	embeddingPath := "/uploads/" + batch + "/items/" + item + "/embedding"
	capture("PUT", embeddingPath, "/uploads/{batch_id}/items/{item_id}/embedding", string(vectorBody), 200) // pending: msgCode 409
	if _, err := f.a.Pool.Exec(f.ctx, `UPDATE upload_item SET state='ready' WHERE id=$1`, item); err != nil {
		t.Fatal(err)
	}
	capture("PUT", embeddingPath, "/uploads/{batch_id}/items/{item_id}/embedding", string(vectorBody), 200)                      // held
	capture("PUT", embeddingPath, "/uploads/{batch_id}/items/{item_id}/embedding", `{"fingerprint":"sample","vector":[1]}`, 200) // msgCode 400
	// Probe every documented account GET with synthetic IDs. Binary and public
	// model/credential endpoints have separate auth/stream tests; never download models.
	raw, _ := os.ReadFile("../../openapi.json")
	var spec struct {
		Paths map[string]map[string]map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	for path, methods := range spec.Paths {
		op, exists := methods["get"]
		if !exists {
			continue
		}
		if path == "/healthz" || strings.HasPrefix(path, "/models/") || strings.HasPrefix(path, "/cred/") || strings.HasPrefix(path, "/maintenance/") || strings.HasPrefix(path, "/admin/maintenance") || strings.HasPrefix(path, "/admin/back") {
			continue
		}
		route := pathVariable.ReplaceAllString(path, "1")
		route = strings.ReplaceAll(route, "/render/1", "/render/"+f.token)
		query := url.Values{}
		if params, ok := op["parameters"].([]any); ok {
			for _, p := range params {
				param := p.(map[string]any)
				if param["in"] != "query" {
					continue
				}
				name := param["name"].(string)
				switch name {
				case "nsId", "parentId":
					query.Set(name, fmt.Sprint(f.root))
				case "photoId":
					query.Set(name, fmt.Sprint(photoID))
				case "userId":
					query.Set(name, fmt.Sprint(f.uid))
				case "page", "limit", "rowsPerPage":
					query.Set(name, "1")
				case "month":
					query.Set(name, "2026-01")
				case "order":
					query.Set(name, "latest")
				}
			}
		}
		w := f.request("GET", route+"?"+query.Encode(), "", nil)
		if !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
			continue
		}
		var v any
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		samples = append(samples, contractSample{"get", path, w.Code, v})
	}
	f.token = ""
	for _, op := range []string{"POST /ai/index", "GET /ai/index/pending", "PUT /ai/preference", "POST /ai/search/vector", "POST /admin/ai/models/{kind}/load", "POST /admin/ai/models/{kind}/unload", "PUT /ai/photos/{photoId}/embedding"} {
		method, path, _ := strings.Cut(op, " ")
		capture(method, pathVariable.ReplaceAllString(path, "1"), path, `{}`, 401)
	}
	capture("PUT", "/uploads/00000000-0000-4000-8000-000000000001/items/00000000-0000-4000-8000-000000000002/embedding", "/uploads/{batch_id}/items/{item_id}/embedding", `{}`, 200)
	// All session-authenticated operations must reject a missing session, including writes.
	for path, methods := range spec.Paths {
		for method, op := range methods {
			security, _ := op["security"].([]any)
			session := false
			for _, entry := range security {
				if _, ok := entry.(map[string]any)["SessionToken"]; ok {
					session = true
				}
			}
			if !session || strings.HasPrefix(path, "/admin/maintenance") || strings.HasPrefix(path, "/admin/back") {
				continue
			}
			w := f.request(strings.ToUpper(method), pathVariable.ReplaceAllString(path, "1"), "application/json", strings.NewReader(`{}`))
			if !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
				continue
			}
			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if code, ok := body["msgCode"].(float64); !ok || (code != 401 && code != 403 && code != 400) {
				t.Fatalf("unauthenticated %s %s: %s", method, path, w.Body.String())
			}
			samples = append(samples, contractSample{method, path, w.Code, body})
		}
	}
	if path := os.Getenv("OPENAPI_SAMPLE_OUTPUT"); path != "" {
		b, err := json.MarshalIndent(samples, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Log(fmt.Sprintf("captured %d real response samples", len(samples)))
}
