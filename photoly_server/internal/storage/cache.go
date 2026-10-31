package storage

import (
	"os"
	"time"
)

// CacheTTL controls expendable S3 cache, never the local source or its derivatives.
func CacheTTL() time.Duration {
	if v := os.Getenv("STORAGE_CACHE_TTL"); v != "" {
		if d, e := time.ParseDuration(v); e == nil && d >= 0 {
			return d
		}
	}
	return 24 * time.Hour
}
