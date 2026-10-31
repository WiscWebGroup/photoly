package storage

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	Register(Driver{Type: "local", Label: "Local disk", Fields: []Field{}, New: func(map[string]string) (Store, error) {
		return Local{Dir: uploadRoot()}, nil
	}})
}

// Local is a Store over a directory. It is the default; the active "local" driver
// never publishes/fetches (files already live there), but it satisfies the same
// contract so other code and tests can treat all backends alike.
type Local struct{ Dir string }

func (l Local) path(key string) (string, error) {
	if !filepath.IsLocal(key) || !fs.ValidPath(key) || key == "." {
		return "", errors.New("invalid key")
	}
	return SafePath(l.Dir, strings.Split(key, "/")...)
}

func (l Local) Put(_ context.Context, key string, r io.Reader, _ int64) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".put-*")
	if err != nil {
		return err
	}
	_, err = io.Copy(tmp, r)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), p)
}

func (l Local) Get(_ context.Context, key string) (io.ReadCloser, error) {
	p, err := l.path(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	return f, err
}

func (l Local) Stat(_ context.Context, key string) (Info, error) {
	p, err := l.path(key)
	if err != nil {
		return Info{}, err
	}
	st, err := os.Stat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return Info{}, ErrNotFound
	}
	if err != nil {
		return Info{}, err
	}
	return Info{Size: st.Size()}, nil
}

func (l Local) Delete(_ context.Context, key string) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	if err = os.Remove(p); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func (l Local) DeletePrefix(_ context.Context, prefix string) error {
	p, err := l.path(strings.TrimSuffix(prefix, "/"))
	if err != nil {
		return err
	}
	return os.RemoveAll(p)
}

func (l Local) Check(context.Context) error { return os.MkdirAll(l.Dir, 0o755) }
