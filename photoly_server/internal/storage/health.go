package storage

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/minio/minio-go/v7"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// AccessCode is safe for clients: raw protocol errors may contain credentials or paths.
func AccessCode(err error) string {
	if err == nil {
		return "available"
	}
	if errors.Is(err, ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		return "file_missing"
	}
	if errors.Is(err, os.ErrPermission) {
		return "access_denied"
	}
	var ne net.Error
	if errors.As(err, &ne) || errors.Is(err, context.DeadlineExceeded) {
		return "unreachable"
	}
	v := strings.ToLower(err.Error())
	for _, part := range []string{"authentication", "access_denied", "access denied", "permission", "_401", "_403", "invalidaccesskeyid", "signaturedoesnotmatch", "accessdenied", "logon_failure"} {
		if strings.Contains(v, part) {
			return "access_denied"
		}
	}
	for _, part := range []string{"not_allowed", "invalid_", "unsupported_", "redirect_refused", "no such host configuration", "nosuchbucket", "bad_network_name"} {
		if strings.Contains(v, part) {
			return "configuration_error"
		}
	}
	response := minio.ToErrorResponse(err)
	if response.StatusCode == 401 || response.StatusCode == 403 {
		return "access_denied"
	}
	if response.Code == "NoSuchBucket" {
		return "configuration_error"
	}

	for _, part := range []string{"connection", "dns", "timeout", "tls", "certificate", "network", "host is down", "broken pipe", "eof"} {
		if strings.Contains(v, part) {
			return "unreachable"
		}
	}
	if v == "storage_source_disabled" {
		return "disabled"
	}
	return "read_failed"
}

type Health struct {
	SourceID  int64     `json:"sourceId"`
	State     string    `json:"state"`
	CheckedAt time.Time `json:"checkedAt"`
}
type healthEntry struct {
	done  chan struct{}
	value Health
	at    time.Time
}

var healthMu sync.Mutex
var healthCache = map[string]*healthEntry{}

func healthKey(s Source) string {
	return fmt.Sprintf("%d:%d:%d:%s:%t:%x", s.UserID, s.ID, s.ConfigVersion, s.Kind, s.Enabled, sha256.Sum256(append(append([]byte(s.Root), s.Config...), s.Secret...)))
}

// ProbeHealth only performs read operations, even for stores whose Check writes a probe object.
func ProbeHealth(ctx context.Context, q dbgen.DBTX, s Source, force bool) (Health, error) {
	key := healthKey(s)
	healthMu.Lock()
	if entry := healthCache[key]; entry != nil {
		select {
		case <-entry.done:
			if !force && time.Since(entry.at) < 30*time.Second {
				v := entry.value
				healthMu.Unlock()
				return v, nil
			}
		default:
			healthMu.Unlock()
			select {
			case <-ctx.Done():
				return Health{}, ctx.Err()
			case <-entry.done:
				return entry.value, nil
			}
		}
	}
	for k, v := range healthCache {
		select {
		case <-v.done:
			if time.Since(v.at) > time.Minute {
				delete(healthCache, k)
			}
		default:
		}
	}
	entry := &healthEntry{done: make(chan struct{})}
	healthCache[key] = entry
	healthMu.Unlock()
	// The shared probe has its own deadline so a disconnected browser cannot cancel other waiters.
	probe, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	state := "available"
	if !s.Enabled {
		state = "disabled"
	} else if s.Kind != "local" {
		st, e := SourceStore(s)
		if e != nil {
			state = "configuration_error"
		} else {
			switch store := st.(type) {
			case TreeStore:
				_, e = store.Property(probe, "")
				if errors.Is(e, ErrNotFound) {
					state = "configuration_error"
				}
			case *s3Store:
				var key string
				qe := q.QueryRow(probe, `SELECT u.uuid||'/'||p.source_id||'/'||p.photo_uuid||'/'||a.filename FROM photo p JOIN "user" u USING(user_id) JOIN media_asset a USING(photo_id) WHERE p.source_id=$1 AND a.role='original' ORDER BY p.photo_id LIMIT 1`, s.ID).Scan(&key)
				if qe == nil {
					_, e = store.Stat(probe, key)
					if errors.Is(e, ErrNotFound) {
						e = nil
					}
				} else {
					// BucketExists is a HEAD, not Check's write/delete round-trip.
					var exists bool
					exists, e = store.c.BucketExists(probe, store.bucket)
					if e == nil && !exists {
						state = "configuration_error"
					}
				}
			default:
				state = "unknown"
			}
			if e != nil && state == "available" {
				state = AccessCode(e)
			}
		}
	}
	entry.value = Health{SourceID: s.ID, State: state, CheckedAt: time.Now().UTC()}
	entry.at = time.Now()
	healthMu.Lock()
	close(entry.done)
	healthMu.Unlock()
	return entry.value, nil
}

func observeAccess(s Source, err error) {
	state := AccessCode(err)
	if state == "file_missing" || state == "read_failed" {
		return
	}
	healthMu.Lock()
	defer healthMu.Unlock()
	key := healthKey(s)
	if current := healthCache[key]; current != nil {
		select {
		case <-current.done:
		default:
			return
		}
	}
	entry := &healthEntry{done: make(chan struct{}), value: Health{SourceID: s.ID, State: state, CheckedAt: time.Now().UTC()}, at: time.Now()}
	close(entry.done)
	healthCache[key] = entry
}
