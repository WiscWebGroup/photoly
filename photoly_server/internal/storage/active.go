package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
)

// The default root is only used when constructing the generic Local test driver.
// Production media paths and remote stores always resolve through storage_source.
var defaultRoot atomic.Pointer[string]

func Init(root string) {
	p, e := filepath.Abs(root)
	if e != nil {
		p = root
	}
	defaultRoot.Store(&p)
}
func uploadRoot() string {
	if p := defaultRoot.Load(); p != nil {
		return *p
	}
	return ""
}
func Fetch(path string) bool { return FetchError(context.Background(), path) == nil }
func Size(path string) (int64, error) {
	if fi, err := os.Lstat(path); err == nil {
		if !fi.Mode().IsRegular() {
			return 0, os.ErrNotExist
		}
		return fi.Size(), nil
	} else if !os.IsNotExist(err) {
		return 0, err
	}
	if loc, ok := remoteLocation(path); ok {
		info, err := loc.store.Stat(context.Background(), loc.key)
		if errors.Is(err, ErrNotFound) {
			err = os.ErrNotExist
		}
		return info.Size, err
	}
	return 0, os.ErrNotExist
}
