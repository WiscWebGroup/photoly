package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
)

func RehomeFolder(ctx context.Context, q dbgen.DBTX, f *Files, root string, uid, id, parent int64, newName string) (string, error) {
	if e := LockNamespace(ctx, q, id); e != nil {
		return "", e
	}
	var oldParent, owner int64
	var name string
	var mount bool
	if err := q.QueryRow(ctx, `SELECT user_id,ns_parent_id,ns_name,is_mount FROM namespace WHERE ns_id=$1`, id).Scan(&owner, &oldParent, &name, &mount); err != nil {
		return "", err
	}
	if owner != uid || mount || oldParent == -1 {
		return "", errors.New("protected_folder")
	}
	if parent == 0 {
		parent = oldParent
	}
	if newName != "" {
		name = newName
	}
	if parent == oldParent && newName == "" {
		return name, nil
	}
	var cycle bool
	if err := q.QueryRow(ctx, `WITH RECURSIVE d AS(SELECT ns_id FROM namespace WHERE ns_id=$1 UNION SELECT n.ns_id FROM namespace n JOIN d ON n.ns_parent_id=d.ns_id) SELECT EXISTS(SELECT 1 FROM d WHERE ns_id=$2)`, id, parent).Scan(&cycle); err != nil {
		return "", err
	}
	if cycle {
		return "", errors.New("folder_cycle")
	}
	src, s, err := Folder(ctx, q, root, id)
	if err != nil {
		return "", err
	}
	if e := SameSourceTarget(ctx, q, s, parent); e != nil {
		return "", e
	}
	dst, t, err := Folder(ctx, q, root, parent)
	if err != nil {
		return "", err
	}
	if t.UserID != uid || s.ID != t.ID {
		return "", errors.New("cross_source_move")
	}
	if parent == oldParent && filepath.Base(src) == name && s.Kind == "local" {
		return name, nil
	}
	name, err = AvailableName(ctx, q, root, parent, name, 0, id)
	if err != nil {
		return "", err
	}
	if err = LockSource(ctx, q, s); err != nil {
		return "", err
	}
	if s.Tree() {
		d, e := TreeSource(s)
		if e != nil {
			return "", e
		}
		info, e := d.Property(ctx, src)
		if e != nil {
			return "", e
		}
		if e = f.TreeMove(ctx, d, src, strings.TrimPrefix(dst+"/"+name, "/"), info.Version); e != nil {
			return "", e
		}
	}
	if s.Kind == "local" {
		for {
			err = f.Move(src, filepath.Join(dst, name))
			if err == nil {
				break
			}
			if !errors.Is(err, os.ErrExist) {
				return "", err
			}
			name, err = AvailableName(ctx, q, root, parent, name, 0, id)
			if err != nil {
				return "", err
			}
		}
	}
	_, err = q.Exec(ctx, `UPDATE namespace SET ns_name=$2,ns_parent_id=$3 WHERE ns_id=$1`, id, name, parent)
	return name, err
}

func RehomePhoto(ctx context.Context, q dbgen.DBTX, f *Files, root string, uid, pid, parent int64, newStem string) (string, error) {
	if e := LockPhoto(ctx, q, pid); e != nil {
		return "", e
	}
	var ns, owner int64
	var filename, uuid, userUUID, format string
	err := q.QueryRow(ctx, `SELECT p.ns_id,p.user_id,p.file_name,p.photo_uuid,u.uuid,p.format FROM photo p JOIN "user" u USING(user_id) WHERE photo_id=$1`, pid).Scan(&ns, &owner, &filename, &uuid, &userUUID, &format)
	if err != nil {
		return "", err
	}
	if owner != uid {
		return "", errors.New("photo_not_owned")
	}
	if parent == 0 {
		parent = ns
	}
	if newStem != "" {
		if !ValidComponent(newStem) {
			return "", errors.New("invalid_file_name")
		}
		newStem += "." + format
	} else {
		newStem = filename
	}
	if ns == parent && newStem == filename {
		return strings.TrimSuffix(filename, filepath.Ext(filename)), nil
	}
	_, source, err := Folder(ctx, q, root, ns)
	if err != nil {
		return "", err
	}
	if e := SameSourceTarget(ctx, q, source, parent); e != nil {
		return "", e
	}
	dir, target, err := Folder(ctx, q, root, parent)
	if err != nil {
		return "", err
	}
	if target.UserID != uid || source.ID != target.ID {
		return "", errors.New("cross_source_move")
	}
	name, err := AvailableName(ctx, q, root, parent, newStem, pid, 0)
	if err != nil {
		return "", err
	}
	if err = LockSource(ctx, q, source); err != nil {
		return "", err
	}
	if source.Tree() {
		oldDir, _, e := Folder(ctx, q, root, ns)
		if e != nil {
			return "", e
		}
		d, e := TreeSource(source)
		if e != nil {
			return "", e
		}
		rows, e := q.Query(ctx, `SELECT a.role,a.filename,a.version,coalesce(a.namespace_id,$2) FROM media_asset a WHERE a.photo_id=$1 AND (a.role='original' OR a.role='live_original' AND EXISTS(SELECT 1 FROM photo_media WHERE photo_id=$1 AND live))`, pid, ns)
		if e != nil {
			return "", e
		}
		type asset struct {
			role, name, version string
			ns                  int64
		}
		var assets []asset
		for rows.Next() {
			var a asset
			if e = rows.Scan(&a.role, &a.name, &a.version, &a.ns); e != nil {
				rows.Close()
				return "", e
			}
			assets = append(assets, a)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return "", e
		}
		for _, a := range assets {
			targetName := name
			if a.role == "live_original" {
				targetName, e = AvailableName(ctx, q, root, parent, strings.TrimSuffix(name, filepath.Ext(name))+"_live.mov", pid, 0)
				if e != nil {
					return "", e
				}
			}
			assetDir := oldDir
			if a.ns != ns {
				assetDir, _, e = Folder(ctx, q, root, a.ns)
				if e != nil {
					return "", e
				}
			}
			if e = f.TreeMove(ctx, d, strings.TrimPrefix(assetDir+"/"+a.name, "/"), strings.TrimPrefix(dir+"/"+targetName, "/"), a.version); e != nil {
				return "", e
			}
			if _, e = q.Exec(ctx, `UPDATE media_asset SET filename=$2,display_name=$2,namespace_id=$4 WHERE photo_id=$1 AND role=$3`, pid, targetName, a.role, parent); e != nil {
				return "", e
			}
		}
	}
	if source.Kind == "local" {
		src, err := Resolve(ctx, q, root, userUUID, uuid, "original")
		if err != nil {
			return "", err
		}
		for {
			err = f.Move(src, filepath.Join(dir, name))
			if err == nil {
				break
			}
			if !errors.Is(err, os.ErrExist) {
				return "", err
			}
			name, err = AvailableName(ctx, q, root, parent, name, pid, 0)
			if err != nil {
				return "", err
			}
		}
		if _, err = q.Exec(ctx, `UPDATE media_asset SET filename=$2,display_name=$2,namespace_id=$3 WHERE photo_id=$1 AND role='original'`, pid, name, parent); err != nil {
			return "", err
		}
		var isLive bool
		if err = q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM photo_media WHERE photo_id=$1 AND live)`, pid).Scan(&isLive); err != nil {
			return "", err
		}
		if isLive {
			live, err := Resolve(ctx, q, root, userUUID, uuid, "live_original")
			if err != nil {
				return "", err
			}
			if _, e := os.Lstat(live); e == nil {
				liveName, err := AvailableName(ctx, q, root, parent, strings.TrimSuffix(name, filepath.Ext(name))+"_live.mov", pid, 0)
				if err != nil {
					return "", err
				}
				if err = f.Move(live, filepath.Join(dir, liveName)); err != nil {
					return "", err
				}
				if _, err = q.Exec(ctx, `UPDATE media_asset SET filename=$2,display_name=$2,namespace_id=$3 WHERE photo_id=$1 AND role='live_original'`, pid, liveName, parent); err != nil {
					return "", err
				}
			} else if !os.IsNotExist(e) {
				return "", e
			}
		}
	}
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	_, err = q.Exec(ctx, `UPDATE photo SET ns_id=$2,file_name=$3,photo_name=$4 WHERE photo_id=$1`, pid, parent, name, stem)
	return stem, err
}
