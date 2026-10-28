// Package storage is the media storage layer. Files always live under the local upload
// root while they are being written or processed; a non-local Store (S3, …) is the
// durable home they are published to after commit and fetched back from on demand.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
)

var ErrNotFound = errors.New("storage: not found")

type Info struct{ Size int64 }

// Store is a durable key/value blob store. Keys are "<user_uuid>/<filename>".
type Store interface {
	Put(ctx context.Context, key string, r io.Reader, size int64) error
	Get(ctx context.Context, key string) (io.ReadCloser, error) // ErrNotFound if missing
	Stat(ctx context.Context, key string) (Info, error)         // ErrNotFound if missing
	Delete(ctx context.Context, key string) error               // missing is not an error
	DeletePrefix(ctx context.Context, prefix string) error
	// Check verifies connectivity and write access.
	Check(ctx context.Context) error
}

// Field describes one driver setting; the admin form is rendered from these.
type Field struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Kind     string `json:"kind"` // "text" or "bool"
	Secret   bool   `json:"secret,omitempty"`
	Required bool   `json:"required,omitempty"`
	Default  string `json:"default,omitempty"`
	Help     string `json:"help,omitempty"`
}

// Driver builds a Store from string settings. Adding a backend = one Register call.
type Driver struct {
	Type   string                                     `json:"type"`
	Label  string                                     `json:"label"`
	Fields []Field                                    `json:"fields"`
	New    func(cfg map[string]string) (Store, error) `json:"-"`
	// ID names the place the store points at (e.g. host/bucket) so two configs of one driver
	// that reach the same data compare equal. Optional.
	ID func(cfg map[string]string) string `json:"-"`
}

var drivers = map[string]Driver{}

func Register(d Driver) { drivers[d.Type] = d }

func Lookup(t string) (Driver, bool) { d, ok := drivers[t]; return d, ok }

func Drivers() []Driver {
	out := make([]Driver, 0, len(drivers))
	for _, d := range drivers {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

// BackendID identifies where cfg stores media: media is tagged with it, and only media tagged
// with the active ID is listed.
func BackendID(driver string, cfg map[string]string) string {
	if d, ok := drivers[driver]; ok && d.ID != nil {
		return driver + ":" + d.ID(cfg)
	}
	return driver
}

// Build validates cfg against the driver's fields and constructs the store.
func Build(driver string, cfg map[string]string) (Store, error) {
	d, ok := drivers[driver]
	if !ok {
		return nil, fmt.Errorf("unknown storage driver %q", driver)
	}
	c := map[string]string{}
	for _, f := range d.Fields {
		v := cfg[f.Name]
		if v == "" {
			v = f.Default
		}
		if f.Required && v == "" {
			return nil, fmt.Errorf("%s is required", f.Label)
		}
		c[f.Name] = v
	}
	return d.New(c)
}
