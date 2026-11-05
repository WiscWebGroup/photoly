// Package legal exposes build provenance and bundled notices without authentication.
package legal

import (
	"embed"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Set by release builds. Development builds deliberately do not claim upstream provenance.
var Revision = "development"
var SourceURL = ""

//go:embed data/*
var data embed.FS

// Serve also works while the database is offline or maintenance is active.
func Serve(w http.ResponseWriter, r *http.Request) bool {
	path := strings.TrimRight(r.URL.Path, "/")
	if path != "/legal" && !strings.HasPrefix(path, "/legal/") {
		return false
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return true
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if path == "/legal" {
		w.Header().Set("Cache-Control", "no-store")
		source := SourceURL
		if custom := os.Getenv("PHOTOLY_SOURCE_URL"); custom != "" {
			source = custom
		}
		u, err := url.Parse(source)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
			source = ""
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.Method != "HEAD" {
			_ = json.NewEncoder(w).Encode(map[string]string{"revision": Revision, "sourceURL": source, "license": "AGPL-3.0-or-later"})
		}
		return true
	}
	name := strings.TrimPrefix(path, "/legal/")
	if strings.Contains(name, "/") || strings.HasPrefix(name, ".") {
		http.NotFound(w, r)
		return true
	}
	b, err := data.ReadFile("data/" + name)
	if err != nil {
		http.NotFound(w, r)
		return true
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if strings.HasSuffix(name, ".json") {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	if r.Method != "HEAD" {
		_, _ = w.Write(b)
	}
	return true
}
