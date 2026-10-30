package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Files holds compensatable disk and remote changes for one database transaction.
// Every operation records its inverse immediately, in execution order.
type Files struct {
	Root           string
	work           string
	undo           []func() error
	before         map[string]string // original file backup, or empty for a new file
	remoteUndo     []func() error
	pinned         map[string]bool
	rollbackFailed bool
	extraWork      []string
	remoteCleanup  []func() error
	ctx            context.Context
}

func NewFiles(root string) (*Files, error) {
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if resolved, e := filepath.EvalSymlinks(root); e == nil {
		root = resolved
	}
	f := &Files{Root: root, before: map[string]string{}, pinned: map[string]bool{}}
	if err := mkdirAll(filepath.Join(root, ".staging")); err != nil {
		return nil, err
	}
	w, err := os.MkdirTemp(filepath.Join(root, ".staging"), "transaction-")
	f.work = w
	return f, err
}
func (f *Files) Work() string { return f.work }
func (f *Files) Close() {
	cacheMu.Lock()
	for p := range f.pinned {
		cachePins.Delete(p)
	}
	cacheMu.Unlock()
	if !f.rollbackFailed {
		_ = removeTree(f.work)
		for _, p := range f.extraWork {
			_ = removeTree(p)
		}
	}
}
func (f *Files) Rollback() {
	for i := len(f.remoteUndo) - 1; i >= 0; i-- {
		if err := f.remoteUndo[i](); err != nil {
			f.rollbackFailed = true
			log.Printf("storage remote rollback failed; retained %s: %v", f.work, err)
		}
	}
	for i := len(f.undo) - 1; i >= 0; i-- {
		if err := f.undo[i](); err != nil {
			f.rollbackFailed = true
			log.Printf("storage rollback failed; retained %s: %v", f.work, err)
		}
	}
}
func (f *Files) Created(p string) {
	f.undo = append(f.undo, func() error {
		err := removeEntry(p)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	})
}
func (f *Files) Mkdir(p string) error {
	if err := mkdir(p); err != nil {
		return err
	}
	f.Created(p)
	return nil
}
func (f *Files) Move(src, dst string) error {
	if src == dst {
		return nil
	}
	if err := RenameNoReplace(src, dst); err != nil {
		return err
	}
	f.undo = append(f.undo, func() error { return RenameNoReplace(dst, src) })
	return nil
}
func (f *Files) Remove(p string) (int, error) {
	f.pin(p)
	if loc, ok := remoteLocation(p); ok {
		if d, ok := loc.store.(TreeStore); ok {
			if err := f.TreeRemove(f.context(), d, loc.key, loc.version); err != nil {
				return 0, err
			}
		}
		if err := FetchError(context.Background(), p); err != nil && !errors.Is(err, ErrNotFound) && !os.IsNotExist(err) {
			return 0, err
		}
	}
	if _, err := os.Lstat(p); err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	backup, err := f.stageFor(p)
	if err != nil {
		return 0, err
	}
	if err := RenameNoReplace(p, backup); err != nil {
		return 0, err
	}
	if _, ok := f.before[p]; !ok {
		f.before[p] = backup
	}
	f.undo = append(f.undo, func() error {
		if err := f.ensureParent(p); err != nil {
			return err
		}
		return RenameNoReplace(backup, p)
	})
	return 1, nil
}
func (f *Files) Install(r io.Reader, dest string) (string, error) {
	tmp, err := f.stageFor(dest)
	if err != nil {
		return "", err
	}
	out, err := createFile(tmp)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(out, h), r)
	if e := out.Close(); err == nil {
		err = e
	}
	if err != nil {
		return "", err
	}
	if err = f.ensureParent(dest); err != nil {
		return "", err
	}
	if _, err = f.Remove(dest); err != nil {
		return "", err
	}
	if _, ok := f.before[dest]; !ok {
		f.before[dest] = ""
	}
	if err = RenameNoReplace(tmp, dest); err != nil {
		return "", err
	}
	f.Created(dest)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// InstallNew never replaces an existing file, including an unindexed external file.
func (f *Files) InstallNew(r io.Reader, dest string) (string, error) {
	f.pin(dest)
	if _, e := os.Lstat(dest); !os.IsNotExist(e) {
		if e != nil {
			return "", e
		}
		return "", os.ErrExist
	}
	sha, _, err := f.InstallUnique(r, dest, nil)
	return sha, err
}

// InstallUnique stages once and retries a collision without rereading the upload.
func (f *Files) InstallUnique(r io.Reader, dest string, next func() (string, error)) (string, string, error) {
	tmp, err := f.stageFor(dest)
	if err != nil {
		return "", "", err
	}
	out, err := createFile(tmp)
	if err != nil {
		return "", "", err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(out, h), r)
	if e := out.Close(); err == nil {
		err = e
	}
	if err != nil {
		return "", "", err
	}
	if err = f.ensureParent(dest); err != nil {
		return "", "", err
	}
	for {
		f.pin(dest)
		err = RenameNoReplace(tmp, dest)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrExist) || next == nil {
			return "", "", err
		}
		dest, err = next()
		if err != nil {
			return "", "", err
		}
	}
	f.before[dest] = ""
	f.Created(dest)
	return hex.EncodeToString(h.Sum(nil)), dest, nil
}
func (f *Files) InstallFile(src, dst string) (string, error) {
	in, e := os.Open(src)
	if e != nil {
		return "", e
	}
	defer in.Close()
	return f.Install(in, dst)
}

// Flush waits for remote writes before the database commits. Backups remain until
// commit, so normal write/commit failures can restore both stores.
func (f *Files) Flush(ctx context.Context) error {
	for p, backup := range f.before {
		loc, ok := remoteLocation(p)
		if !ok {
			continue
		}
		if d, ok := loc.store.(TreeStore); ok {
			if e := f.flushTree(ctx, p, loc, d); e != nil {
				return e
			}
			continue
		}
		restore := func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if backup == "" {
				return loc.store.Delete(ctx, loc.key)
			}
			in, err := os.Open(backup)
			if err != nil {
				return err
			}
			defer in.Close()
			st, err := in.Stat()
			if err != nil {
				return err
			}
			return loc.store.Put(ctx, loc.key, in, st.Size())
		}
		f.remoteUndo = append(f.remoteUndo, restore)
		if in, err := os.Open(p); err == nil {
			st, e := in.Stat()
			if e == nil {
				e = loc.store.Put(ctx, loc.key, in, st.Size())
			}
			in.Close()
			if e != nil {
				return e
			}
		} else if os.IsNotExist(err) {
			if e := loc.store.Delete(ctx, loc.key); e != nil {
				return e
			}
		} else {
			return err
		}
	}
	return nil
}
func (f *Files) Commit() {
	f.remoteUndo = nil
	for _, fn := range f.remoteCleanup {
		if err := fn(); err != nil {
			log.Printf("storage backup cleanup failed: %v", err)
		}
	}
}
func (f *Files) Changed() bool { return len(f.undo) > 0 || len(f.remoteUndo) > 0 }

// Delta remains useful to callers accounting a subset of files.
func (f *Files) Delta(dir string) int64 {
	var n int64
	for p, b := range f.before {
		if strings.HasPrefix(p, dir+string(filepath.Separator)) {
			if st, e := os.Stat(p); e == nil {
				n += st.Size()
			}
			if b != "" {
				if st, e := os.Stat(b); e == nil {
					n -= st.Size()
				}
			}
		}
	}
	return n
}

func (f *Files) pin(p string) {
	if _, ok := remoteLocation(p); ok {
		cacheMu.Lock()
		f.pinned[p] = true
		cachePins.Store(p, true)
		cacheMu.Unlock()
	}
}
