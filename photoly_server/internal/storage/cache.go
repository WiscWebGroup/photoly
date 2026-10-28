package storage

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// With a remote backend the local upload root is a cache. Sweep drops files that have not
// been used for CacheTTL() once the remote is confirmed to hold the same bytes; a file the
// remote lacks (failed upload, in-flight write) is never dropped.

var lastUsed sync.Map // path -> time.Time of the latest Fetch

func touch(path string) {
	if Remote() != nil {
		lastUsed.Store(path, time.Now())
	}
}

// CacheTTL is how long an offloaded file may sit unused locally (STORAGE_CACHE_TTL, a Go
// duration; default 24h; "0" keeps everything).
func CacheTTL() time.Duration {
	if v := os.Getenv("STORAGE_CACHE_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 0 {
			return d
		}
		log.Printf("storage: invalid STORAGE_CACHE_TTL %q, using 24h", v)
	}
	return 24 * time.Hour
}

// Sweep evicts idle cached files until ctx ends.
func Sweep(ctx context.Context) {
	ttl := CacheTTL()
	if ttl == 0 {
		return
	}
	t := time.NewTicker(min(ttl/4, time.Hour))
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n := evict(ctx, ttl); n > 0 {
				log.Printf("storage: evicted %d cached files", n)
			}
		}
	}
}

// evict removes cached files idle for longer than ttl that the remote holds intact.
// ponytail: age-based only; add a size cap (oldest-first) if the cache must be bounded.
func evict(ctx context.Context, ttl time.Duration) (n int) {
	st, root := Remote(), uploadRoot()
	if st == nil || root == "" {
		return 0
	}
	now := time.Now()
	lastUsed.Range(func(p, t any) bool {
		if now.Sub(t.(time.Time)) > ttl {
			lastUsed.Delete(p)
		}
		return true
	})
	dirs, _ := os.ReadDir(root)
	for _, d := range dirs {
		if !d.IsDir() || d.Name()[0] == '.' {
			continue
		}
		files, _ := os.ReadDir(filepath.Join(root, d.Name()))
		for _, f := range files {
			if ctx.Err() != nil {
				return n
			}
			p := filepath.Join(root, d.Name(), f.Name())
			fi, err := f.Info()
			if err != nil || !fi.Mode().IsRegular() {
				continue
			}
			used := fi.ModTime()
			if t, ok := lastUsed.Load(p); ok && t.(time.Time).After(used) {
				used = t.(time.Time)
			}
			if now.Sub(used) <= ttl {
				continue
			}
			info, err := st.Stat(ctx, d.Name()+"/"+f.Name())
			if err != nil || info.Size != fi.Size() {
				continue
			}
			if os.Remove(p) == nil {
				n++
			}
		}
	}
	return n
}
