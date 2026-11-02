package user

import (
	"errors"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/httpx"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strings"
)

// CredentialHandler accepts only scoped API credentials, never account/share sessions.
func CredentialHandler(a *app.App, scope string, next func(http.ResponseWriter, *http.Request, int64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fail := func(code int, msg string) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(code)
			httpx.JSON(w, msg, code)
		}
		w.Header().Set("Cache-Control", "private, no-store")
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			fail(401, "invalid_token")
			return
		}
		c, e := dbgen.New(a.Pool).GetCredByToken(r.Context(), parts[1])
		if errors.Is(e, pgx.ErrNoRows) {
			fail(401, "invalid_token")
			return
		}
		if e != nil {
			fail(503, "service_unavailable")
			return
		}
		if !strings.Contains(c.Authorization, scope) {
			fail(403, "insufficient_scope")
			return
		}
		next(w, r, c.UserID)
	}
}
