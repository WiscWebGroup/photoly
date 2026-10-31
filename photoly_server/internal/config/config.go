// Package config reads the .env.example environment into app.Config.
package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
)

func str(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func num[T int | int64 | float64](k string, d T) T {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return d
	}
	return T(f)
}

// Load reads the environment; unset values take the defaults below.
func Load() app.Config {
	return app.Config{
		DatabaseURL:                  DatabaseURL(false),
		UploadFolder:                 str("UPLOAD_FOLDER", "/photoly"),
		AdminEmail:                   str("ADMIN_EMAIL", "admin@a.com"),
		DemoEmail:                    os.Getenv("DEMO_EMAIL"),
		DemoPassword:                 os.Getenv("DEMO_PASSWORD"),
		AdminPassword:                os.Getenv("ADMIN_PASSWORD"),
		Addr:                         str("PHOTOLY_ADDR", ":8084"),
		PublicBaseURL:                strings.TrimRight(str("PUBLIC_BASE_URL", "http://localhost:8384"), "/"),
		Timezone:                     str("TIMEZONE", "America/New_York"),
		MaxContentLength:             num("MAX_CONTENT_LENGTH", int64(600<<20)),
		UploadZipMaxBytes:            num("UPLOAD_ZIP_MAX_BYTES", int64(50_000_000_000)),
		UploadDiskReserveBytes:       num("UPLOAD_DISK_RESERVE_BYTES", int64(512<<20)),
		UploadTempHours:              num("UPLOAD_TEMP_HOURS", 24),
		UploadHistoryDays:            num("UPLOAD_HISTORY_DAYS", 30),
		OnlineDownloadMaxBytes:       num("ONLINE_DOWNLOAD_MAX_BYTES", int64(50<<20)),
		OnlineDownloadConnectTimeout: num("ONLINE_DOWNLOAD_CONNECT_TIMEOUT", 5.0),
		OnlineDownloadTimeout:        num("ONLINE_DOWNLOAD_TIMEOUT", 60.0),
		MediaProcessTimeout:          num("MEDIA_PROCESS_TIMEOUT", 60.0),
		CORSOrigins:                  strings.Split(str("CORS_ORIGINS", "*"), ","),
	}
}

// DatabaseURL returns DATABASE_URL (or MIGRATION_DATABASE_URL), else builds one from
// POSTGRES_* like Flask does. Passwords are escaped, never interpolated.
func DatabaseURL(migration bool) string {
	if migration {
		if u := os.Getenv("MIGRATION_DATABASE_URL"); u != "" {
			return u
		}
		if os.Getenv("POSTGRES_MIGRATION_PASSWORD") == "" {
			return DatabaseURL(false) // single-role setups migrate as the app role
		}
	} else if u := os.Getenv("DATABASE_URL"); u != "" {
		return u
	}
	user, pass := "photoly", str("POSTGRES_APP_PASSWORD", "photoly")
	if migration {
		user, pass = "photoly_migrator", os.Getenv("POSTGRES_MIGRATION_PASSWORD")
	}
	u := "postgres://" + urlUser(user, pass) + "@" + str("POSTGRES_HOST", "localhost") + ":" +
		str("POSTGRES_PORT", "5432") + "/" + str("POSTGRES_DB", "photoly")
	return u
}
