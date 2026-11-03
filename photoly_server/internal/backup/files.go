// Package backup owns the offline backup format and maintenance control plane.
package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"golang.org/x/sys/unix"
)

const partSize int64 = 4 << 30
const reserveBytes int64 = 512 << 20

func atomicJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.OpenFile(path+".tmp", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	if e = os.Rename(path+".tmp", path); e != nil {
		return e
	}
	return syncDir(filepath.Dir(path))
}
func syncDir(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}
func readJSON(path string, v any) error {
	st, e := os.Lstat(path)
	if e != nil {
		return e
	}
	if !st.Mode().IsRegular() || st.Size() > 16<<20 {
		return errors.New("invalid or oversized metadata file")
	}
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, 16<<20+1))
	if e != nil {
		return e
	}
	if len(b) > 16<<20 {
		return errors.New("metadata too large")
	}
	return json.Unmarshal(b, v)
}

func exists(path string) bool { _, e := os.Lstat(path); return e == nil }
func safe(root, relative string) (string, error) {
	if relative == "" || !filepath.IsLocal(relative) || strings.Contains(relative, "\\") {
		return "", errors.New("unsafe backup path")
	}
	p := root
	for _, part := range strings.Split(relative, "/") {
		if !storage.ValidComponent(part) {
			return "", errors.New("unsafe backup path")
		}
		p = filepath.Join(p, part)
		st, e := os.Lstat(p)
		if e == nil && st.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("symbolic links are not supported")
		}
		if e != nil && !os.IsNotExist(e) {
			return "", e
		}
	}
	return p, nil
}
func free(path string, need int64) error {
	var s unix.Statfs_t
	if e := unix.Statfs(path, &s); e != nil {
		return e
	}
	available := uint64(s.Bavail) * uint64(s.Bsize)
	if need < 0 || uint64(need)+uint64(reserveBytes) > available {
		return errors.New("insufficient disk space (512 MiB reserve required)")
	}
	return nil
}
func prepare(cfg app.Config) (*os.File, error) {
	root, e := filepath.Abs(cfg.BackupDir)
	if e != nil {
		return nil, e
	}
	for _, p := range []string{cfg.UploadFolder, os.Getenv("PGDATA"), "/var/lib/postgresql", os.Getenv("PHOTOLY_DATA"), "/var/lib/photoly"} {
		if p == "" {
			continue
		}
		p, _ = filepath.Abs(p)
		if storage.Within(p, root) || storage.Within(root, p) {
			return nil, errors.New("backup directory overlaps application data")
		}
	}
	if e = os.MkdirAll(root, 0700); e != nil {
		return nil, e
	}
	if _, e = storage.NoLinks(root); e != nil {
		return nil, e
	}
	control := filepath.Join(root, ".control")
	if e = os.MkdirAll(control, 0700); e != nil {
		return nil, e
	}
	if _, e = storage.NoLinks(control); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(filepath.Join(control, "instance.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, errors.New("another Photoly instance or restore process owns this backup volume")
	}
	return f, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.r.Read(p)
}
func digest(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func checkID(id string) error {
	if len(id) != 36 || !storage.ValidComponent(id) || strings.Contains(id, ".") {
		return fmt.Errorf("invalid backup ID")
	}
	return nil
}
