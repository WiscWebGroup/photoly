package storage

import (
	"context"
	"errors"
	"fmt"
	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func FetchError(ctx context.Context, p string) error {
	if st, err := os.Lstat(p); err == nil {
		if !st.Mode().IsRegular() {
			return errors.New("not_a_regular_file")
		}
		if _, remote := remoteLocation(p); remote {
			cacheMu.Lock()
			_ = os.Chtimes(p, time.Now(), time.Now())
			cacheMu.Unlock()
		}
		return nil
	}
	loc, ok := remoteLocation(p)
	if !ok {
		return os.ErrNotExist
	}
	r, err := loc.store.Get(ctx, loc.key)
	if err != nil {
		return err
	}
	defer r.Close()
	if err = mkdirAll(filepath.Dir(p)); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".fetch-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = io.Copy(f, r)
	if e := f.Close(); err == nil {
		err = e
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), p)
}

// SweepSources only removes source-bound cache files, never the user's originals.
var cachePins sync.Map
var cacheMu sync.Mutex

func SweepSources(ctx context.Context, q dbgen.DBTX, root string) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ttl := CacheTTL()
			if ttl == 0 {
				continue
			}
			// Register persisted cache entries after restart as well as recently used paths.
			rows, e := q.Query(ctx, `SELECT s.id,s.user_id,s.kind,s.root_path,s.enabled,s.config,s.secret,u.uuid FROM storage_source s JOIN "user" u USING(user_id) WHERE s.kind='s3' AND s.enabled`)
			if e == nil {
				for rows.Next() {
					var source Source
					var owner string
					if e = rows.Scan(&source.ID, &source.UserID, &source.Kind, &source.Root, &source.Enabled, &source.Config, &source.Secret, &owner); e != nil {
						break
					}
					st, e := SourceStore(source)
					if e != nil {
						continue
					}
					base, e := SafePath(root, ".remote", owner, fmt.Sprint(source.ID))
					if e != nil {
						continue
					}
					_ = filepath.WalkDir(base, func(p string, d os.DirEntry, e error) error {
						if e != nil {
							return nil
						}
						if d.Type().IsRegular() {
							rel, e := filepath.Rel(base, p)
							if e == nil && !strings.HasPrefix(d.Name(), ".") {
								Bind(p, st, owner+"/"+fmt.Sprint(source.ID)+"/"+filepath.ToSlash(rel))
							}
						}
						return nil
					})
				}
				rows.Close()
			}
			locations.Range(func(key, value any) bool {
				p := key.(string)
				loc := value.(location)
				if _, pinned := cachePins.Load(p); pinned {
					return true
				}
				if st, e := os.Stat(p); e == nil && time.Since(st.ModTime()) > ttl {
					if info, e := loc.store.Stat(ctx, loc.key); e == nil && info.Size == st.Size() {
						cacheMu.Lock()
						if _, pinned := cachePins.Load(p); !pinned {
							if now, err := os.Lstat(p); err == nil && now.ModTime() == st.ModTime() && now.Size() == st.Size() {
								_ = os.Remove(p)
							}
						}
						cacheMu.Unlock()
					}
				}
				return ctx.Err() == nil
			})
		}
	}
}
