// Package app holds the contract between route packages and the server skeleton.
// Each route package exports `Register(a *app.App)` and only touches its own files.
package app

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	DatabaseURL          string
	UploadFolder         string
	BackupDir            string
	BackupDatabaseURL    string
	MigrationDatabaseURL string
	AdminEmail           string
	DemoEmail            string // DEMO_EMAIL: account barred from /admin writes and login changes (see server/demo.go)
	DemoPassword         string // DEMO_PASSWORD: shown on the demo login page only; the account's real password is set elsewhere

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

// Router is implemented by http.ServeMux. Tests can observe registration while
// delegating all matching and dispatch to the standard library.
type Router interface {
	http.Handler
	Handle(string, http.Handler)
	HandleFunc(string, func(http.ResponseWriter, *http.Request))
	Handler(*http.Request) (http.Handler, string)
}

type App struct {
	Context context.Context
	Workers sync.WaitGroup
	OnClose []func()

	StartedAt   time.Time
	BootID      string
	WorkerRSS   func() *int64
	ResourceRun func(context.Context)
	CredRender  func(http.ResponseWriter, *http.Request, int64, int64) error
	CredDelete  func(context.Context, int64, int64) error
	Cfg         Config
	Pool        *pgxpool.Pool
	Mux         Router
	// Auth wraps a handler: resolves the session/token exactly like Flask and
	// responds with the legacy error envelope when missing. Set by the skeleton.
	Auth func(http.HandlerFunc) http.HandlerFunc
	// Admin is Auth plus an admin-role check.
	Admin func(http.HandlerFunc) http.HandlerFunc
	// UserFrom returns the authenticated user inside an Auth-wrapped handler.
	UserFrom func(ctx context.Context) User
}

// Go owns a background task for the entire application lifetime.
func (a *App) Go(fn func(context.Context)) {
	a.Workers.Add(1)
	go func() { defer a.Workers.Done(); fn(a.Context) }()
}

// Wait is called after cancelling Context and draining HTTP requests.
func (a *App) Wait() {
	a.Workers.Wait()
	for _, close := range a.OnClose {
		close()
	}
	a.Pool.Close()
}
