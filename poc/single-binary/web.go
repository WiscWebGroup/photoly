package main

import (
	"compress/gzip"
	"embed"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

// web/vendor holds the gzipped browser runtime from scripts/vendor.sh. Files stay
// compressed inside the binary and are sent as-is to browsers that accept gzip.
//
//go:embed web
var webFiles embed.FS

func staticHandler() http.Handler {
	root, _ := fs.Sub(webFiles, "web")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if ctype := mime.TypeByExtension(path.Ext(name)); ctype != "" {
			w.Header().Set("Content-Type", ctype)
		}
		switch path.Ext(name) {
		case ".mjs":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		case ".wasm":
			w.Header().Set("Content-Type", "application/wasm")
		}
		if strings.HasPrefix(name, "vendor/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}

		if data, err := fs.ReadFile(root, name); err == nil {
			w.Write(data)
			return
		}
		f, err := root.Open(name + ".gz")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		w.Header().Add("Vary", "Accept-Encoding")
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			io.Copy(w, f)
			return
		}
		zr, err := gzip.NewReader(f)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		io.Copy(w, zr)
	})
}
