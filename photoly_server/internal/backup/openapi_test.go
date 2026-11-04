package backup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
)

// Exercise the controller's separate dispatcher without starting a real backup,
// database switch, or model worker. Existing integration tests cover those jobs.
func TestOpenAPIControllerResponses(t *testing.T) {
	cfg := app.Config{BackupDir: t.TempDir(), UploadFolder: t.TempDir()}
	id := "00000000-0000-4000-8000-000000000001"
	job := Job{ID: id, Kind: "backup", BackupID: id, State: "running", Stage: "Exporting database", Started: time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)}
	c := &Controller{cfg: cfg, engine: &Engine{Cfg: cfg}, ctx: context.Background(), state: State{Mode: "on", Job: &job}, changed: make(chan struct{})}
	if err := os.MkdirAll(filepath.Join(cfg.BackupDir, ".control"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := atomicJSON(filepath.Join(cfg.BackupDir, ".control", "job-"+id+".json"), job); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		method, path, document, body string
		status                       int
		public                       bool
	}{
		{"GET", "/maintenance/status", "/maintenance/status", "", 200, true},
		{"GET", "/healthz", "/healthz", "", 200, true},
		{"GET", "/demo", "/demo", "", 200, true},
		{"GET", "/admin/maintenance", "/admin/maintenance", "", 200, false},
		{"POST", "/admin/maintenance/enter", "/admin/maintenance/enter", "", 409, false},
		{"POST", "/admin/maintenance/exit", "/admin/maintenance/exit", "", 409, false},
		{"GET", "/admin/backups", "/admin/backups", "", 200, false},
		{"POST", "/admin/backups", "/admin/backups", `{"password":"synthetic-test-password"}`, 409, false},
		{"GET", "/admin/backup-jobs/" + id, "/admin/backup-jobs/{id}", "", 200, false},
		{"POST", "/admin/backups/" + id + "/restore", "/admin/backups/{id}/restore", `{"password":"synthetic-test-password","confirm":true}`, 409, false},
		{"POST", "/admin/backups/" + id + "/restore", "/admin/backups/{id}/restore", `{"confirm":false}`, 400, false},
		{"GET", "/admin/backup-jobs/invalid", "/admin/backup-jobs/{id}", "", 400, false},
	}
	var samples []map[string]any
	for _, tc := range cases {
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		w := httptest.NewRecorder()
		if tc.public {
			c.ServeHTTP(w, r)
		} else {
			c.control(w, r)
		}
		if w.Code != tc.status {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
		var body any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		samples = append(samples, map[string]any{"method": strings.ToLower(tc.method), "path": tc.document, "status": w.Code, "body": body})
	}
	// The public dispatcher must enforce administrator access before control handlers.
	for _, tc := range cases {
		if tc.public {
			continue
		}
		w := httptest.NewRecorder()
		c.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		if w.Code != 403 {
			t.Fatalf("unauthenticated %s %s: %d", tc.method, tc.path, w.Code)
		}
		var body any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		samples = append(samples, map[string]any{"method": strings.ToLower(tc.method), "path": tc.document, "status": w.Code, "body": body})
	}
	// Transitions use a disposable no-op runtime and local state files only.
	c.state = State{Mode: "off"}
	c.start = func(context.Context) (Runtime, error) {
		return Runtime{Handler: http.NotFoundHandler(), Stop: func() {}}, nil
	}
	for _, action := range []string{"enter", "exit"} {
		path := "/admin/maintenance/" + action
		w := httptest.NewRecorder()
		c.control(w, httptest.NewRequest("POST", path, nil))
		c.workers.Wait()
		if w.Code != 202 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		var body any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		samples = append(samples, map[string]any{"method": "post", "path": path, "status": w.Code, "body": body})
	}
	if out := os.Getenv("OPENAPI_BACKUP_SAMPLE_OUTPUT"); out != "" {
		raw, _ := json.MarshalIndent(samples, "", "  ")
		if err := os.WriteFile(out, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
