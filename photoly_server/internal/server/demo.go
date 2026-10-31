package server

import (
	"context"
	"net/http"
	"path"
	"strings"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/httpx"
)

// demoBlocked reports requests the demo account may not make: every write under /admin (users,
// roles, passwords, groups, quotas, settings, storage, AI, stop) and changes to its own login.
// Blocking all /admin writes, not just quotas, keeps it from creating or promoting another admin
// that would escape the limits.
func demoBlocked(r *http.Request) bool {
	switch p := path.Clean(r.URL.Path); {
	case strings.HasPrefix(p, "/admin/"):
		return r.Method != http.MethodGet && p != "/admin/getGeneralSetting" // the one read that is a POST
	case p == "/user/updateEmail", p == "/user/updatePassword", p == "/user/deleteUser":
		return true
	}
	return false
}

// demoGuard rejects demoBlocked requests from the account with this email (DEMO_EMAIL), and
// closes sign-up for everyone, whatever the SignUp setting says. Only blocked paths pay for the
// session lookup. The reply is BusinessError(-2) like requireAdmin: a 403 envelope would make
// the frontend sign the user out.
func demoGuard(email string, lookup func(context.Context, string) (app.User, bool), next http.Handler) http.Handler {
	if email == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if path.Clean(r.URL.Path) == "/user/signUp" {
			httpx.Token(w, "Register is closed by Admin", "", 400)
			return
		}
		if demoBlocked(r) {
			if u, ok := lookup(r.Context(), r.Header.Get("HRD-Token")); ok && strings.EqualFold(u.Email, email) {
				httpx.JSON(w, -2, 400)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
