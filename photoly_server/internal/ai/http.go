package ai

// HTTP glue shared by the AI routes: the ai_route envelope (controllers/ai.py). Unlike the
// legacy routes these answer with a real HTTP status: {"t":..., "msgCode":status} plus
// "error":code when something is wrong.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	var low *LowMemoryError
	switch {
	case errors.As(err, &low):
		// The admin page reads these two bodies to offer "load anyway" / show a restart.
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "low-memory", "kind": low.Kind,
			"variant": low.Variant, "needMB": low.NeedMB, "availableMB": low.AvailableMB})
		return
	case errors.Is(err, errWorkerDied):
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error(), "code": "worker-died"})
		return
	}
	e := asErr(err)
	if e.Code == "internal_error" {
		logf("ai: %v", err)
	}
	writeJSON(w, e.Status, map[string]any{"t": nil, "msgCode": e.Status, "error": e.Code})
}

// discard swallows the legacy rejection that app.Auth writes.
type discard struct{ h http.Header }

func (d *discard) Header() http.Header         { return d.h }
func (d *discard) Write(b []byte) (int, error) { return len(b), nil }
func (d *discard) WriteHeader(int)             {}

// Route adapts fn to an ai_route: authentication, an optional admin check, the envelope.
func Route(a *app.App, admin bool, fn func(r *http.Request, u app.User) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		called := false
		a.Auth(func(_ http.ResponseWriter, r *http.Request) {
			called = true
			u := a.UserFrom(r.Context())
			if admin && u.Role != "admin" {
				writeErr(w, fail("admin_required", 403))
				return
			}
			res, err := fn(r, u)
			if err != nil {
				writeErr(w, err)
				return
			}
			status := http.StatusOK
			if r.Method == http.MethodPost && r.URL.Path == "/admin/ai/operations" {
				status = http.StatusAccepted
			}
			writeJSON(w, status, map[string]any{"t": res, "msgCode": status})
		})(&discard{http.Header{}}, r)
		if !called {
			writeErr(w, fail("unauthenticated", 401))
		}
	}
}

// body decodes a JSON object (numbers stay exact so "type is int" checks work).
func body(r *http.Request) (map[string]any, error) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		return nil, fail("invalid_json")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v map[string]any
	if err := dec.Decode(&v); err != nil || v == nil {
		return nil, fail("invalid_json")
	}
	return v, nil
}

// optBody is body() for routes that accept an empty request.
func optBody(r *http.Request) map[string]any {
	if v, err := body(r); err == nil {
		return v
	}
	return map[string]any{}
}

func ctxOf(r *http.Request) context.Context { return r.Context() }
