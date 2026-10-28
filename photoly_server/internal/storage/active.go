package storage

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// The active backend is process-wide: media paths are resolved in many packages that
// have no shared service object, and they all route through Fetch/Publish below.
// ponytail: package-level state; thread a *Manager through app.App if tests need isolation.

type active struct {
	driver string
	store  Store
}

var (
	cur   atomic.Pointer[active]
	roots atomic.Pointer[[]string]
)

// Init records the upload root (and its symlink-resolved form, since callers use both).
func Init(uploadFolder string) {
	r, err := filepath.Abs(uploadFolder)
	if err != nil {
		r = uploadFolder
	}
	rs := []string{r}
	if e, err := filepath.EvalSymlinks(r); err == nil && e != r {
		rs = append(rs, e)
	}
	roots.Store(&rs)
}

func uploadRoot() string {
	if rs := roots.Load(); rs != nil {
		return (*rs)[0]
	}
	return ""
}

// Set makes store the active backend. driver "local" disables publish/fetch.
func Set(driver string, store Store) { cur.Store(&active{driver, store}) }

// ActiveDriver is the active driver type ("local" until configured).
func ActiveDriver() string {
	if a := cur.Load(); a != nil {
		return a.driver
	}
	return "local"
}

// Remote is the active non-local store, or nil when media lives on local disk only.
func Remote() Store {
	if a := cur.Load(); a != nil && a.driver != "local" {
		return a.store
	}
	return nil
}

// key maps an absolute media path to "<user_uuid>/<filename>"; ok is false for anything
// else (staging/upload scratch dirs, files outside the root).
func key(path string) (string, bool) {
	rs := roots.Load()
	if rs == nil {
		return "", false
	}
	for _, r := range *rs {
		rel, err := filepath.Rel(r, path)
		if err != nil {
			continue
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" && !strings.HasPrefix(parts[0], ".") && parts[0] != ".." {
			return strings.Join(parts, "/"), true
		}
	}
	return "", false
}

func exists(p string) bool { st, err := os.Stat(p); return err == nil && !st.IsDir() }

// Fetch ensures path exists locally, downloading it from the remote store when it was
// offloaded. It reports whether the file is now present. Free when the file is local.
// Fetched files are a cache that Sweep trims.
func Fetch(path string) bool {
	if exists(path) {
		touch(path)
		return true
	}
	st := Remote()
	k, ok := key(path)
	if st == nil || !ok {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	rc, err := st.Get(ctx, k)
	if err != nil {
		if err != ErrNotFound {
			log.Printf("storage: fetch %s: %v", k, err)
		}
		return false
	}
	defer rc.Close()
	scratch := filepath.Join(filepath.Dir(filepath.Dir(path)), ".staging")
	if os.MkdirAll(scratch, 0o755) != nil || os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return false
	}
	tmp, err := os.CreateTemp(scratch, "fetch-*")
	if err != nil {
		return false
	}
	_, err = io.Copy(tmp, rc)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path) // atomic; concurrent fetches of one file are harmless
	}
	if err != nil {
		os.Remove(tmp.Name())
		log.Printf("storage: fetch %s: %v", k, err)
		return false
	}
	touch(path)
	return true
}

// Publish mirrors a committed change to the remote store: deletes first, then uploads
// (a path that was replaced is in both lists). Uploaded files are dropped from local
// disk; a failed upload leaves the local copy in place, where reads still find it.
// ponytail: a failed upload is only logged, never retried; add a sweep job if it matters.
func Publish(removed, created []string) {
	st := Remote()
	if st == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	for _, p := range removed {
		if k, ok := key(p); ok {
			if err := st.Delete(ctx, k); err != nil {
				log.Printf("storage: delete %s: %v", k, err)
			}
		}
	}
	for _, p := range created {
		k, ok := key(p)
		if !ok {
			continue
		}
		f, err := os.Open(p)
		if err != nil {
			continue // removed again within the same transaction
		}
		fi, err := f.Stat()
		if err == nil {
			err = st.Put(ctx, k, f, fi.Size())
		}
		f.Close()
		if err != nil {
			log.Printf("storage: upload %s: %v", k, err)
			continue
		}
		os.Remove(p)
	}
}

// DeleteUser removes everything a user stored remotely.
func DeleteUser(uuid string) {
	if st := Remote(); st != nil && uuid != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if err := st.DeletePrefix(ctx, uuid+"/"); err != nil {
			log.Printf("storage: delete user %s: %v", uuid, err)
		}
	}
}

// Size is the byte size of a media file, answered from local disk or else the remote
// store without downloading. A missing file reports an error satisfying os.IsNotExist.
// ponytail: one remote HEAD per offloaded file; batch via a prefix listing if lists get slow.
func Size(path string) (int64, error) {
	if st, err := os.Stat(path); err == nil {
		if !st.Mode().IsRegular() {
			return 0, os.ErrNotExist
		}
		return st.Size(), nil
	} else if !os.IsNotExist(err) {
		return 0, err
	}
	st, k := Remote(), ""
	var ok bool
	if k, ok = key(path); st == nil || !ok {
		return 0, os.ErrNotExist
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	info, err := st.Stat(ctx, k)
	if err == ErrNotFound {
		return 0, os.ErrNotExist
	}
	return info.Size, err
}
