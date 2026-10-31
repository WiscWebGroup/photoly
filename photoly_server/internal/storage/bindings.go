package storage

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"sort"

	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
)

type BindingInput struct {
	Name         string `json:"name"`
	RootID       string `json:"rootId"`
	RelativePath string `json:"relativePath"`
	URL          string `json:"url"`
	Server       string `json:"server"`
	Share        string `json:"share"`
	Subdirectory string `json:"subdirectory"`
	Domain       string `json:"domain"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	Enabled      *bool  `json:"enabled"`
	ScanInterval int    `json:"scanInterval"`
}

func ReadSource(ctx context.Context, q dbgen.DBTX, id int64) (Source, error) {
	return GetSource(context.WithValue(ctx, inactiveKey{}, true), q, id)
}
func SaveBinding(ctx context.Context, q dbgen.DBTX, root string, uid, id int64, kind string, in BindingInput) (int64, error) {
	values := map[string]string{}
	managed := "user"
	location := ""
	identity := ""
	var old Source
	if id != 0 {
		var e error
		old, e = ReadSource(ctx, q, id)
		if e != nil {
			return 0, e
		}
		if old.UserID != uid || old.Kind != kind || old.Default || kind == "local" && old.ManagedBy != "admin" {
			return 0, errors.New("storage_source_not_owned")
		}
		if e = LockSource(ctx, q, old); e != nil {
			return 0, e
		}
		values, e = SourceValues(old)
		if e != nil {
			return 0, e
		}
		location = old.Root
	}
	if id != 0 {
		var oldName string
		var oldInterval int
		if e := q.QueryRow(ctx, `SELECT display_name,scan_interval FROM storage_source WHERE id=$1`, id).Scan(&oldName, &oldInterval); e != nil {
			return 0, e
		}
		if in.Name == "" {
			in.Name = oldName
		}
		if in.ScanInterval == 0 {
			in.ScanInterval = oldInterval
		}
		if in.Enabled == nil {
			enabled := old.Enabled
			in.Enabled = &enabled
		}
	}
	if !ValidComponent(in.Name) || in.Name == SystemDirectory {
		return 0, errors.New("invalid_entry_name")
	}
	if in.ScanInterval == 0 {
		in.ScanInterval = 60
	}
	if in.ScanInterval < 60 {
		return 0, errors.New("scan_interval_minimum_60")
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	if kind == "local" {
		managed = "admin"
		if id != 0 && (in.RootID != "" && in.RootID != values["root_id"] || (in.RelativePath != "" || in.RootID != "") && in.RelativePath != values["relative_path"]) {
			return 0, errors.New("storage_location_immutable")
		}
		if id == 0 {
			p, _, e := LocalBinding(root, in.RootID, in.RelativePath)
			if e != nil {
				return 0, e
			}
			location = p
			st, e := os.Lstat(p)
			if e != nil {
				return 0, e
			}
			identity = Identity(st)
			values = map[string]string{"root_id": in.RootID, "relative_path": in.RelativePath}
		}
	} else if kind == "webdav" || kind == "smb" {
		if !SecretKeyConfigured() {
			return 0, errors.New("storage_secret_key_missing")
		}
		var e error
		values, e = bindingValues(kind, id, values, in)
		if e != nil {
			return 0, e
		}
		// Connectivity checks run before the writing transaction in the HTTP handlers.
		if _, e := NewTree(kind, values); e != nil {
			return 0, e
		}
	} else {
		return 0, errors.New("unsupported_storage_source")
	}
	plain, secret, e := SealValues(values)
	if e != nil {
		return 0, e
	}
	var parent int64
	if e = q.QueryRow(ctx, `SELECT ns_id FROM namespace WHERE user_id=$1 AND ns_parent_id=-1`, uid).Scan(&parent); e != nil {
		return 0, e
	}
	var mount int64
	if id != 0 {
		if e = q.QueryRow(ctx, `SELECT ns_id FROM namespace WHERE source_id=$1 AND is_mount`, id).Scan(&mount); e != nil {
			return 0, e
		}
	}
	name, e := AvailableName(ctx, q, root, parent, in.Name, 0, mount)
	if e != nil {
		return 0, e
	}
	if id == 0 {
		e = q.QueryRow(ctx, `INSERT INTO storage_source(user_id,kind,managed_by,display_name,root_path,root_identity,config,secret,enabled,scan_interval) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`, uid, kind, managed, name, location, identity, plain, secret, enabled, in.ScanInterval).Scan(&id)
		if e != nil {
			return 0, e
		}
		_, e = q.Exec(ctx, `INSERT INTO namespace(ns_name,ns_parent_id,user_id,source_id,is_mount,fs_identity) VALUES($1,$2,$3,$4,true,$5)`, name, parent, uid, id, identity)
	} else {
		_, e = q.Exec(ctx, `UPDATE storage_source SET display_name=$2,config=$3,secret=$4,enabled=$5,scan_interval=$6,config_version=config_version+1,revision=revision+1,scan_requested=true WHERE id=$1`, id, name, plain, secret, enabled, in.ScanInterval)
		if e != nil {
			return 0, e
		}
		_, e = q.Exec(ctx, `UPDATE namespace SET ns_name=$2 WHERE ns_id=$1`, mount, name)
	}

	return id, e
}
func bindingValues(kind string, id int64, v map[string]string, in BindingInput) (map[string]string, error) {
	locations := map[string]string{"url": in.URL}
	if kind == "smb" {
		locations = map[string]string{"server": in.Server, "share": in.Share, "subdirectory": in.Subdirectory}
	}
	for k, value := range locations {
		if id != 0 && value != "" && value != v[k] {
			return nil, errors.New("storage_location_immutable")
		}
		if id == 0 {
			v[k] = value
		}
	}
	for k, value := range map[string]string{"username": in.Username, "password": in.Password, "domain": in.Domain} {
		if value != "" {
			v[k] = value
		}
	}
	return v, nil
}
func TestDAVBinding(ctx context.Context, q dbgen.DBTX, uid, id int64, in BindingInput) error {
	return TestTreeBinding(ctx, q, uid, id, "webdav", in)
}
func TestTreeBinding(ctx context.Context, q dbgen.DBTX, uid, id int64, kind string, in BindingInput) error {
	v := map[string]string{}
	if id != 0 {
		s, e := ReadSource(ctx, q, id)
		if e != nil {
			return e
		}
		if s.UserID != uid || s.Kind != kind {
			return errors.New("storage_source_not_owned")
		}
		v, e = SourceValues(s)
		if e != nil {
			return e
		}
	}
	v, e := bindingValues(kind, id, v, in)
	if e != nil {
		return e
	}
	d, e := NewTree(kind, v)
	if e != nil {
		return e
	}
	return d.Check(ctx)
}

type preserveKey struct{}

func PreserveExternal(ctx context.Context) context.Context {
	return context.WithValue(ctx, preserveKey{}, true)
}
func DetachBinding(ctx context.Context, q dbgen.DBTX, f *Files, root string, uid, id int64, kind string) error {
	source, e := ReadSource(ctx, q, id)
	if e != nil {
		return e
	}
	if source.UserID != uid || source.Kind != kind || !source.External() {
		return errors.New("storage_source_not_owned")
	}
	if e = LockSource(ctx, q, source); e != nil {
		return e
	}
	rows, e := q.Query(ctx, `SELECT u.uuid,p.photo_uuid FROM photo p JOIN "user" u USING(user_id) WHERE p.source_id=$1`, id)
	if e != nil {
		return e
	}
	var paths []string
	for rows.Next() {
		var owner, uuid string
		if e = rows.Scan(&owner, &uuid); e != nil {
			break
		}
		p, e := SafePath(root, ".assets", owner, uuid)
		if e != nil {
			rows.Close()
			return e
		}
		paths = append(paths, p)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, p := range paths {
		if _, e = f.Remove(p); e != nil {
			return e
		}
	}
	var owner string
	if e = q.QueryRow(ctx, `SELECT uuid FROM "user" WHERE user_id=$1`, uid).Scan(&owner); e != nil {
		return e
	}
	cache, e := SafePath(root, ".remote", owner, fmt.Sprint(id))
	if e != nil {
		return e
	}
	if _, e = f.Remove(cache); e != nil {
		return e
	}
	// Upload sessions reference namespaces; cancel them before removing the index.
	if _, e = q.Exec(ctx, `DELETE FROM upload_batch WHERE ns_id IN(SELECT ns_id FROM namespace WHERE source_id=$1)`, id); e != nil {
		return e
	}
	if _, e = q.Exec(ctx, `DELETE FROM photo WHERE source_id=$1`, id); e != nil {
		return e
	}
	// Namespace parent links are application-managed; delete the entire source together.
	if _, e = q.Exec(ctx, `DELETE FROM media_asset WHERE namespace_id IN(SELECT ns_id FROM namespace WHERE source_id=$1)`, id); e != nil {
		return e
	}
	if _, e = q.Exec(ctx, `DELETE FROM namespace WHERE source_id=$1`, id); e != nil {
		return e
	}
	if _, e = q.Exec(ctx, `DELETE FROM storage_source WHERE id=$1`, id); e != nil {
		return e
	}
	return nil
}

// Same-filesystem workspaces live under the configured allow-root, never in a
// user-controlled child directory. Missing mount roots are never recreated.
func externalWorkspace(p string) (string, error) {
	roots, e := LocalRoots()
	if e != nil {
		return "", e
	}
	sort.Slice(roots, func(i, j int) bool { return len(roots[i].Path) > len(roots[j].Path) })
	for _, r := range roots {
		if Within(r.Path, p) {
			base, e := NoLinks(r.Path)
			if e != nil {
				return "", e
			}
			st, e := os.Lstat(base)
			if e != nil || !st.IsDir() {
				return "", errors.New("storage_folder_unavailable")
			}
			work, e := StrictPath(base, SystemDirectory)
			if e != nil {
				return "", e
			}
			if e = mkdir(work); e != nil && !os.IsExist(e) {
				return "", e
			}
			tmp := filepath.Join(work, "transaction-"+uuid.NewString())
			if e = mkdir(tmp); e != nil {
				return "", e
			}
			return tmp, nil
		}
	}
	return "", errors.New("external_path_not_allowed")
}
