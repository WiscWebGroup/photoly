// Package docs serves Swagger UI at /docs/ (same paths as the Flask blueprint).
package docs

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
)

//go:embed assets
var assets embed.FS

const page = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Photoly API — Swagger UI</title>
  <link rel="icon" href="data:,">
  <link rel="stylesheet" href="/docs/assets/vendor/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="/docs/assets/vendor/swagger-ui-bundle.js"></script>
  <script src="/docs/assets/swagger-init.js" data-spec-url="/openapi.json"></script>
</body>
</html>`

func Register(a *app.App) {
	sub, _ := fs.Sub(assets, "assets")
	a.Mux.Handle("GET /docs/assets/", http.StripPrefix("/docs/assets/", http.FileServer(http.FS(sub))))
	h := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	}
	a.Mux.HandleFunc("GET /docs/", h)
	a.Mux.HandleFunc("GET /docs", h)
}
