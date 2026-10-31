package user

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	"github.com/jackc/pgx/v5"
)

const s3OccupiedSQL = `SELECT EXISTS(SELECT 1 FROM photo WHERE source_id=$1) OR EXISTS(SELECT 1 FROM upload_batch b WHERE NOT b.cancelled AND (NOT b.sealed OR EXISTS(SELECT 1 FROM upload_item i WHERE i.batch_id=b.id AND i.state NOT IN ('done','skipped','cancelled'))) AND b.ns_id IN(SELECT ns_id FROM namespace WHERE source_id=$1))`

func s3Occupied(ctx context.Context, q legacy.DB, id int64) (bool, error) {
	var yes bool
	e := q.QueryRow(ctx, s3OccupiedSQL, id).Scan(&yes)
	return yes, e
}

func ownedS3(ctx context.Context, q legacy.DB, uid, id int64) (storage.Source, error) {
	v, e := storage.ReadSource(ctx, q, id)
	if e == pgx.ErrNoRows || (e == nil && (v.UserID != uid || v.Kind != "s3")) {
		return v, legacy.Err("storage_source_not_found", 404)
	}
	return v, e
}
func (s *svc) s3Settings(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	ctx := storage.ExclusiveUser(r.Context())
	rows, e := s.a.Pool.Query(ctx, `SELECT id,display_name,enabled,config,coalesce(octet_length(secret)>0,false) FROM storage_source WHERE user_id=$1 AND kind='s3' ORDER BY id`, uid)
	if e != nil {
		return nil, e
	}
	connections := []map[string]any{}
	for rows.Next() {
		var id int64
		var name string
		var enabled, secret bool
		var config []byte
		if e = rows.Scan(&id, &name, &enabled, &config, &secret); e != nil {
			rows.Close()
			return nil, e
		}
		values := map[string]string{}
		if e = json.Unmarshal(config, &values); e != nil {
			rows.Close()
			return nil, e
		}
		delete(values, "access_key")
		delete(values, "secret_key")
		connections = append(connections, map[string]any{"sourceId": id, "name": name, "enabled": enabled, "values": values, "hasSecret": secret})
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return nil, e
	}
	for _, c := range connections {
		occupied, e := s3Occupied(ctx, s.a.Pool, c["sourceId"].(int64))
		if e != nil {
			return nil, e
		}
		c["canRemove"] = !occupied
	}
	driver, _ := storage.Lookup("s3")
	return legacy.Res{V: map[string]any{"connections": connections, "fields": driver.Fields, "canSave": storage.SecretKeyConfigured()}, Code: 200}, nil
}
func s3Values(old storage.Source, data map[string]any) (map[string]string, error) {
	values := map[string]string{}
	var e error
	if old.ID != 0 {
		values, e = storage.SourceValues(old)
		if e != nil {
			return nil, e
		}
	}
	raw, ok := data["values"].(map[string]any)
	if data["values"] != nil && !ok {
		return nil, legacy.Bad()
	}
	driver, _ := storage.Lookup("s3")
	for _, field := range driver.Fields {
		if v, ok := raw[field.Name]; ok {
			switch x := v.(type) {
			case string:
				if x != "" || (field.Name != "secret_key" && field.Name != "access_key") {
					values[field.Name] = x
				}
			case bool:
				values[field.Name] = strconv.FormatBool(x)
			default:
				return nil, legacy.Bad()
			}
		}
	}
	return values, nil
}
func verifyS3(ctx context.Context, values map[string]string) error {
	if !storage.SecretKeyConfigured() {
		return legacy.Err("storage_secret_key_missing", 400)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	st, e := storage.Build("s3", values)
	if e != nil {
		return legacy.Err(e.Error(), 400)
	}
	if e = st.Check(ctx); e != nil {
		return legacy.Err("S3 connection or write check failed", 400)
	}
	return nil
}
func (s *svc) testS3(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	data, e := legacy.Body(r)
	if e != nil {
		return nil, e
	}
	old := storage.Source{}
	if v, ok := data["sourceId"]; ok {
		id, valid := legacy.Int(v)
		if !valid || id <= 0 {
			return nil, legacy.Bad()
		}
		old, e = ownedS3(r.Context(), s.a.Pool, uid, id)
		if e != nil {
			return nil, e
		}
	}
	values, e := s3Values(old, data)
	if e != nil {
		return nil, e
	}
	e = verifyS3(r.Context(), values)
	return legacy.Changed(1, 1), e
}
func (s *svc) saveS3(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	data, e := legacy.Body(r)
	if e != nil {
		return nil, e
	}
	var id int64
	if r.Method == "PATCH" {
		id, e = strconv.ParseInt(r.PathValue("sourceId"), 10, 64)
		if e != nil || id <= 0 {
			return nil, legacy.Bad()
		}
	}
	ctx := storage.ExclusiveUser(r.Context())
	var name string
	e = legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uid, false, func(tx pgx.Tx, f *legacy.Files, u *legacy.LockedUser) error {
		if r.Method == "PUT" {
			var count int
			var only int64
			if e := tx.QueryRow(ctx, `SELECT count(*),coalesce(min(id),0) FROM storage_source WHERE user_id=$1 AND kind='s3'`, uid).Scan(&count, &only); e != nil {
				return e
			}
			if count > 1 {
				return legacy.Err("multiple_s3_sources_use_source_id", 409)
			}
			id = only
		}
		old := storage.Source{}
		var e error
		if id != 0 {
			old, e = ownedS3(ctx, tx, uid, id)
			if e != nil {
				return e
			}
		}
		enabled := true
		if id != 0 {
			enabled = old.Enabled
		}
		if v, ok := data["enabled"]; ok {
			var valid bool
			enabled, valid = v.(bool)
			if !valid {
				return legacy.Bad()
			}
		}
		if id == 0 && !enabled && r.Method == "PUT" {
			return nil
		}
		// Disabling is a status-only operation: no credentials, filesystem or remote access.
		if id != 0 && !enabled && len(data) == 1 {
			return tx.QueryRow(ctx, `UPDATE storage_source SET enabled=false,revision=revision+1,config_version=config_version+1 WHERE id=$1 RETURNING display_name`, id).Scan(&name)
		}
		var parent, mount int64
		if e = tx.QueryRow(ctx, `SELECT ns_id FROM namespace WHERE user_id=$1 AND ns_parent_id=-1`, uid).Scan(&parent); e != nil {
			return e
		}
		name = "S3"
		if id != 0 {
			if e = tx.QueryRow(ctx, `SELECT ns_id,ns_name FROM namespace WHERE source_id=$1 AND is_mount`, id).Scan(&mount, &name); e != nil {
				return e
			}
		}
		if v, ok := data["name"]; ok {
			name, e = legacy.ValidName(v, false)
			if e != nil {
				return e
			}
		}
		if _, renaming := data["name"]; id == 0 || renaming {
			name, e = storage.AvailableName(ctx, tx, s.a.Cfg.UploadFolder, parent, name, 0, mount)
			if e != nil {
				return e
			}
		}
		plain, secret := old.Config, old.Secret
		if id == 0 || data["values"] != nil || (!old.Enabled && enabled) {
			values, e := s3Values(old, data)
			if e != nil {
				return e
			}
			if id != 0 {
				previous, e := storage.SourceValues(old)
				if e != nil {
					return e
				}
				if storage.BackendID("s3", previous) != storage.BackendID("s3", values) {
					occupied, e := s3Occupied(ctx, tx, id)
					if e != nil {
						return e
					}
					if occupied {
						return legacy.Err("storage_source_not_empty", 409)
					}
				}
			}
			if e = verifyS3(ctx, values); e != nil {
				return e
			}
			plain, secret, e = storage.SealValues(values)
			if e != nil {
				return e
			}
		}
		if id == 0 {
			if e = tx.QueryRow(ctx, `INSERT INTO storage_source(user_id,kind,config,secret,enabled,scan_status,display_name,managed_by) VALUES($1,'s3',$2,$3,$4,'not_applicable',$5,'user') RETURNING id`, uid, plain, secret, enabled, name).Scan(&id); e != nil {
				return e
			}
			_, e = tx.Exec(ctx, `INSERT INTO namespace(ns_name,ns_parent_id,user_id,source_id,is_mount) VALUES($1,$2,$3,$4,true)`, name, parent, uid, id)
		} else {
			if _, e = tx.Exec(ctx, `UPDATE storage_source SET config=$2,secret=$3,enabled=$4,display_name=$5,revision=revision+1,config_version=config_version+1 WHERE id=$1`, id, plain, secret, enabled, name); e != nil {
				return e
			}
			_, e = tx.Exec(ctx, `UPDATE namespace SET ns_name=$2 WHERE ns_id=$1`, mount, name)
		}
		return e
	})
	return legacy.Res{V: map[string]any{"sourceId": id, "name": name}, Code: 200}, e
}
func (s *svc) deleteS3(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	id, e := strconv.ParseInt(r.PathValue("sourceId"), 10, 64)
	if e != nil || id <= 0 {
		return nil, legacy.Bad()
	}
	ctx := storage.ExclusiveUser(r.Context())
	e = legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uid, false, func(tx pgx.Tx, f *legacy.Files, u *legacy.LockedUser) error {
		if _, e := ownedS3(ctx, tx, uid, id); e != nil {
			return e
		}
		occupied, e := s3Occupied(ctx, tx, id)
		if e != nil {
			return e
		}
		if occupied {
			return legacy.Err("storage_source_not_empty", 409)
		}
		var owner string
		if e = tx.QueryRow(ctx, `SELECT uuid FROM "user" WHERE user_id=$1`, uid).Scan(&owner); e != nil {
			return e
		}
		cache, e := storage.SafePath(s.a.Cfg.UploadFolder, ".remote", owner, fmt.Sprint(id))
		if e != nil {
			return e
		}
		if _, e = f.Remove(cache); e != nil {
			return e
		}
		// Only local index/cache cleanup. Never bind or delete objects in the remote bucket.
		for _, query := range []string{`DELETE FROM upload_batch WHERE ns_id IN(SELECT ns_id FROM namespace WHERE source_id=$1)`, `DELETE FROM namespace WHERE source_id=$1`, `DELETE FROM storage_source WHERE id=$1`} {
			if _, e = tx.Exec(ctx, query, id); e != nil {
				return e
			}
		}
		return nil
	})
	return legacy.Changed(1, 0), e
}
