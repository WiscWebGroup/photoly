package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/google/uuid"
)

const SystemDirectory = ".photoly-system"

type LocalRoot struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"-"`
}

// LocalRoots is deployment-owned. Clients only receive IDs and labels.
func LocalRoots() ([]LocalRoot, error) {
	var raw []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Path string `json:"path"`
	}
	value := os.Getenv("STORAGE_LOCAL_ROOTS")
	if value == "" {
		return []LocalRoot{}, nil
	}
	if err := json.Unmarshal([]byte(value), &raw); err != nil {
		return nil, errors.New("invalid STORAGE_LOCAL_ROOTS")
	}
	roots := []LocalRoot{}
	ids := map[string]bool{}
	for _, r := range raw {
		if !ValidComponent(r.ID) || ids[r.ID] || !filepath.IsAbs(r.Path) {
			return nil, errors.New("invalid local root configuration")
		}
		ids[r.ID] = true
		roots = append(roots, LocalRoot{r.ID, r.Name, filepath.Clean(r.Path)})
	}
	return roots, nil
}
func Within(root, p string) bool {
	rel, e := filepath.Rel(root, p)
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
func ValidateRelative(p string) ([]string, error) {
	if p == "" || p == "." {
		return nil, nil
	}
	if filepath.IsAbs(p) || strings.Contains(p, "\\") {
		return nil, errors.New("invalid_relative_path")
	}
	parts := strings.Split(p, "/")
	for _, part := range parts {
		if !ValidComponent(part) || part == SystemDirectory {
			return nil, errors.New("invalid_relative_path")
		}
	}
	return parts, nil
}

// NoLinks checks from the filesystem root, including the deployment mount itself.
func NoLinks(p string) (string, error) {
	if !filepath.IsAbs(p) {
		return "", errors.New("absolute_path_required")
	}
	return SafePath(string(filepath.Separator), strings.Split(strings.TrimPrefix(filepath.Clean(p), string(filepath.Separator)), string(filepath.Separator))...)
}
func LocalBinding(uploadRoot, rootID, relative string) (string, string, error) {
	roots, err := LocalRoots()
	if err != nil {
		return "", "", err
	}
	parts, err := ValidateRelative(relative)
	if err != nil {
		return "", "", err
	}
	for _, r := range roots {
		if r.ID == rootID {
			base, err := NoLinks(r.Path)
			if err != nil {
				return "", "", err
			}
			p, err := StrictPath(base, parts...)
			if err != nil {
				return "", "", err
			}
			// Deny both containment directions: binding '/' would expose internal assets too.
			forbidden := []string{uploadRoot, os.Getenv("PHOTOLY_DATA"), "/var/lib/photoly", os.Getenv("AI_MODEL_DIR"), os.Getenv("MODEL_DIR"), os.Getenv("POSTGRES_DATA_DIR"), os.Getenv("PGDATA"), "/app/models", "/var/lib/postgresql"}
			for _, v := range forbidden {
				if v == "" {
					continue
				}
				v, _ = canonicalRoot(v)
				if Within(v, p) || Within(p, v) {
					return "", "", errors.New("application_directory_not_allowed")
				}
			}
			st, err := os.Lstat(p)
			if err != nil {
				return "", "", err
			}
			if !st.IsDir() {
				return "", "", errors.New("not_a_directory")
			}
			// Opening a temporary probe verifies access without recreating a missing mount.
			probe, err := createFile(filepath.Join(p, ".photoly-check-"+uuid.NewString()))
			if err != nil {
				return "", "", errors.New("local_directory_not_writable")
			}
			name := probe.Name()
			err = probe.Close()
			if e := removeEntry(name); err == nil {
				err = e
			}
			if err != nil {
				return "", "", err
			}
			return p, base, nil
		}
	}
	return "", "", errors.New("unknown_local_root")
}
func (s Source) External() bool {
	return s.Tree() || s.Kind == "local" && s.ManagedBy == "admin"
}

// All external local bindings share a physical lock, including overlaps across users.
// Remote bindings share an origin lock. PostgreSQL locks also cover separate workers.
func LockSource(ctx context.Context, q dbgen.DBTX, s Source) error {
	key := fmt.Sprintf("source:%d", s.ID)
	if s.Kind == "local" && s.External() {
		key = "external-local"
	}
	if s.Tree() {
		v, e := SourceValues(s)
		if e != nil {
			return e
		}
		d, e := NewTree(s.Kind, v)
		if e != nil {
			return e
		}
		key = d.origin()
	}
	_, e := q.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,482610))`, key)
	return e
}

// StrictPath is for a resolved user directory. Unlike the deployment upload root,
// it must never be canonicalized through a newly swapped symlink.
func StrictPath(root string, parts ...string) (string, error) {
	if !filepath.IsAbs(root) {
		return "", errors.New("absolute_path_required")
	}
	base := []string{}
	if clean := strings.TrimPrefix(filepath.Clean(root), "/"); clean != "" {
		base = strings.Split(clean, "/")
	}
	return SafePath("/", append(base, parts...)...)
}
func LockNamespace(ctx context.Context, q dbgen.DBTX, id int64) error {
	var sid int64
	if e := q.QueryRow(ctx, `SELECT source_id FROM namespace WHERE ns_id=$1`, id).Scan(&sid); e != nil {
		return e
	}
	source, e := GetSource(ctx, q, sid)
	if e != nil {
		return e
	}
	return LockSource(ctx, q, source)
}
func SameSourceTarget(ctx context.Context, q dbgen.DBTX, source Source, parent int64) error {
	var sid, uid int64
	if e := q.QueryRow(ctx, `SELECT source_id,user_id FROM namespace WHERE ns_id=$1`, parent).Scan(&sid, &uid); e != nil {
		return e
	}
	if sid != source.ID || uid != source.UserID {
		return errors.New("cross_source_move")
	}
	return nil
}
