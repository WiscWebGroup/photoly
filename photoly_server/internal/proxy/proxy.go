// Package proxy is the strangler: routes Go has not ported yet go to the Flask backend.
package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// New returns a reverse proxy to target, or a JSON 404 handler when target is empty.
func New(target string) (http.Handler, error) {
	if target == "" {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }), nil
	}
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(u)
			r.SetXForwarded()
			r.Out.Host = r.In.Host
		},
		FlushInterval: -1, // stream downloads and zip exports
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"t": nil, "msgCode": 503})
		},
	}, nil
}
