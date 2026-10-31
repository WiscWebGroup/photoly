// Package davserver provides the filesystem-backed WebDAV protocol fixture. The
// x/net handler implements DAV locking but delegates HTTP preconditions to its host.
package davserver

import (
	"golang.org/x/net/webdav"
	"net/http"
	"net/http/httptest"
)

func Handler(root string) http.Handler {
	h := &webdav.Handler{FileSystem: webdav.Dir(root), LockSystem: webdav.NewMemLS()}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" || r.Method == "MOVE" || r.Method == "DELETE" {
			head := r.Clone(r.Context())
			head.Method = "HEAD"
			head.Body = nil
			head.Header = make(http.Header)
			result := httptest.NewRecorder()
			h.ServeHTTP(result, head)
			if r.Header.Get("If-None-Match") == "*" && result.Code == 200 {
				w.WriteHeader(412)
				return
			}
			if want := r.Header.Get("If-Match"); want != "" && (result.Code != 200 || want != result.Header().Get("ETag")) {
				w.WriteHeader(412)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}
