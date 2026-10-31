// Package app holds the contract between route packages and the server skeleton.
// Each route package exports `Register(a *app.App)` and only touches its own files.
package app

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	DatabaseURL  string
	UploadFolder string
	AdminEmail   string
	DemoEmail    string // DEMO_EMAIL: account barred from /admin writes and login changes (see server/demo.go)
	DemoPassword string // DEMO_PASSWORD: shown on the demo login page only; the account's real password is set elsewhere

	AdminPassword                string
	Addr                         string // listen address, PHOTOLY_ADDR (default :8084)
	PublicBaseURL                string
	Timezone                     string
	MaxContentLength             int64
	UploadZipMaxBytes            int64
	UploadDiskReserveBytes       int64
	UploadTempHours              int
	UploadHistoryDays            int
	OnlineDownloadMaxBytes       int64
	OnlineDownloadConnectTimeout float64 // seconds
	OnlineDownloadTimeout        float64
	MediaProcessTimeout          float64
	CORSOrigins                  []string
}

type User struct {
	ID    int64
	UUID  string
	Email string
	Role  string
}

type App struct {
	Cfg  Config
	Pool *pgxpool.Pool
	Mux  *http.ServeMux
	// Auth wraps a handler: resolves the session/token exactly like Flask and
	// responds with the legacy error envelope when missing. Set by the skeleton.
	Auth func(http.HandlerFunc) http.HandlerFunc
	// Admin is Auth plus an admin-role check.
	Admin func(http.HandlerFunc) http.HandlerFunc
	// UserFrom returns the authenticated user inside an Auth-wrapped handler.
	UserFrom func(ctx context.Context) User
}
