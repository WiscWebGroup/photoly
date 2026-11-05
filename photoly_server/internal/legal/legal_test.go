package legal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicNotices(t *testing.T) {
	for _, name := range []string{"AGPL-3.0.txt", "GPL-3.0.txt", "third-party-notices.txt", "dependencies.json"} {
		r := httptest.NewRequest("GET", "/legal/"+name, nil)
		w := httptest.NewRecorder()
		if !Serve(w, r) || w.Code != 200 || w.Body.Len() < 100 {
			t.Fatalf("%s: %d", name, w.Code)
		}
		if !strings.Contains(w.Header().Get("Content-Type"), "text/plain") && !strings.HasSuffix(name, ".json") {
			t.Fatal("not plain text")
		}
	}
	for _, p := range []string{"/legal/missing.txt", "/legal/../go.mod", "/legal/data/AGPL-3.0.txt"} {
		w := httptest.NewRecorder()
		Serve(w, httptest.NewRequest("GET", p, nil))
		if w.Code != 404 {
			t.Fatalf("%s: %d", p, w.Code)
		}
	}
	w := httptest.NewRecorder()
	Serve(w, httptest.NewRequest("POST", "/legal", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatal(w.Code)
	}
}
func TestSourceOverride(t *testing.T) {
	for _, source := range []string{"https://example.org/source.tar.gz", "javascript:alert(1)", "/upstream"} {
		t.Setenv("PHOTOLY_SOURCE_URL", source)
		w := httptest.NewRecorder()
		Serve(w, httptest.NewRequest("GET", "/legal", nil))
		var d map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(source, "https:") {
			if d["sourceURL"] != source {
				t.Fatal(d)
			}
		} else if d["sourceURL"] != "" {
			t.Fatal("unsafe source URL", d)
		}
	}
}
