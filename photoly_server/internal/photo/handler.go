package photo

import (
	"fmt"
	"hash/adler32"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/httpx"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
)

// rc is a result with an explicit msgCode (default 200).
type rc struct {
	v    any
	code int
}

// changed is controllers.common.changed.
func changed(n int) rc {
	if n >= 1 {
		return rc{n, 200}
	}
	return rc{n, 400}
}

func emit(w http.ResponseWriter, v any, err error) {
	if err != nil {
		if b, ok := BizOf(err); ok {
			httpx.JSON(w, b.Value, b.Code)
			return
		}
		log.Printf("photo: internal error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if r, ok := v.(rc); ok {
		httpx.JSON(w, r.v, r.code)
		return
	}
	httpx.JSON(w, v, 200)
}

// JSONHandler wraps a legacy `result` route that needs the header session.
func (s *Service) JSONHandler(fn func(r *http.Request, uid int64) (any, error)) http.HandlerFunc {
	return s.app.Auth(func(w http.ResponseWriter, r *http.Request) {
		v, err := fn(r, s.app.UserFrom(r.Context()).ID)
		emit(w, v, err)
	})
}

type discard struct{ h http.Header }

func (d discard) Header() http.Header         { return d.h }
func (d discard) Write(b []byte) (int, error) { return len(b), nil }
func (d discard) WriteHeader(int)             {}

// BinaryHandler wraps a shape="binary" route. mode is "header", "path" (token path value),
// "query" (token argument) or "none". Failures are empty responses with a status code.
func (s *Service) BinaryHandler(mode string, fn func(w http.ResponseWriter, r *http.Request, uid int64) error) http.HandlerFunc {
	run := func(w http.ResponseWriter, r *http.Request, uid int64) {
		if err := fn(w, r, uid); err != nil {
			code := 400
			if b, ok := BizOf(err); ok {
				if b.Code == 403 || b.Code == 404 {
					code = b.Code
				}
			} else {
				log.Printf("photo: internal error: %v", err)
				code = 500
			}
			w.Header().Del("Content-Type")
			w.WriteHeader(code)
		}
	}
	if mode == "none" {
		return func(w http.ResponseWriter, r *http.Request) { run(w, r, 0) }
	}
	return func(w http.ResponseWriter, r *http.Request) {
		r2 := r
		switch mode {
		case "path":
			r2 = r.Clone(r.Context())
			r2.Header.Set("HRD-Token", r.PathValue("token"))
		case "query":
			r2 = r.Clone(r.Context())
			t, _ := argStr(r, "token")
			r2.Header.Set("HRD-Token", t)
		}
		called := false
		s.app.Auth(func(_ http.ResponseWriter, r3 *http.Request) {
			called = true
			run(w, r, s.app.UserFrom(r3.Context()).ID)
		})(discard{http.Header{}}, r2)
		if !called {
			w.WriteHeader(http.StatusForbidden)
		}
	}
}

// serveFile is controllers.common.binary: send_file(conditional=True) with no-store caching.
func serveFile(w http.ResponseWriter, r *http.Request, path, mime, download string) error {
	f, err := storage.OpenFile(path)
	if err != nil {
		return Err(nil, 404)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || fi.IsDir() {
		return Err(nil, 404)
	}
	h := w.Header()
	if mime != "" {
		h.Set("Content-Type", mime)
	}
	h.Set("Cache-Control", "private, no-store")
	h.Set("ETag", fmt.Sprintf(`"%d-%d-%d"`, fi.ModTime().Unix(), fi.Size(), adler32.Checksum([]byte(path))))
	if download != "" {
		h.Set("Content-Disposition", contentDisposition(download))
	}
	http.ServeContent(w, r, "", fi.ModTime(), f)
	return nil
}

func contentDisposition(name string) string {
	ascii := true
	for _, c := range name {
		if c > unicode.MaxASCII || c < 32 {
			ascii = false
		}
	}
	if ascii && !strings.ContainsAny(name, `"\;, `) {
		return "attachment; filename=" + name
	}
	if ascii {
		return `attachment; filename="` + strings.ReplaceAll(name, `"`, `\"`) + `"`
	}
	fallback := strings.Map(func(r rune) rune {
		if r > unicode.MaxASCII || r < 32 {
			return -1
		}
		return r
	}, name)
	return "attachment; filename=" + fallback + "; filename*=UTF-8''" + url.PathEscape(name)
}

func serveReader(w http.ResponseWriter, body io.Reader, mime string) {
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = io.Copy(w, body)
}
