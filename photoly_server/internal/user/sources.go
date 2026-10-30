package user

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	"github.com/jackc/pgx/v5"
)

func (s *svc) registerSources() {
	s.registerWebDAV()
	opt := legacy.Opt{Auth: "header"}
	s.a.Mux.HandleFunc("GET /user/storage/sources", opt.Wrap(s.a, s.sources))
	s.a.Mux.HandleFunc("GET /user/storage/s3", opt.Wrap(s.a, s.s3Settings))
	s.a.Mux.HandleFunc("PUT /user/storage/s3", opt.Wrap(s.a, s.saveS3))
	s.a.Mux.HandleFunc("POST /user/storage/s3/test", opt.Wrap(s.a, s.testS3))
}
func (s *svc) sources(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	rows, err := s.a.Pool.Query(r.Context(), `SELECT id,kind,enabled,revision,scan_status,scan_error,scanned_at,display_name,managed_by,scan_interval,scan_completed,scan_total,scan_retries,scan_phase,scan_discovered,scan_waiting,scan_unchanged,EXISTS(SELECT 1 FROM storage_scan_job j WHERE j.source_id=storage_source.id AND j.state='queued'),(SELECT id FROM storage_scan_job j WHERE j.source_id=storage_source.id ORDER BY id DESC LIMIT 1) FROM storage_source WHERE user_id=$1 ORDER BY storage_source.id`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, rev int64
		var kind, status, name, managed string
		var interval int
		var completed, total, retries int64
		var enabled, queued bool
		var phase string
		var discovered, waiting, unchanged int64
		var jobID *int64
		var failure *string
		var at *time.Time
		if err = rows.Scan(&id, &kind, &enabled, &rev, &status, &failure, &at, &name, &managed, &interval, &completed, &total, &retries, &phase, &discovered, &waiting, &unchanged, &queued, &jobID); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"sourceId": id, "sourceType": kind, "enabled": enabled, "revision": rev, "scanStatus": status, "scanError": failure, "scannedAt": at, "name": name, "managedBy": managed, "scanInterval": interval, "scanCompleted": completed, "scanTotal": total, "scanRetries": retries, "scanPhase": phase, "scanDiscovered": discovered, "scanWaiting": waiting, "scanUnchanged": unchanged, "scanQueued": queued, "scanJobId": jobID, "capabilities": map[string]bool{"write": enabled, "scan": kind != "s3", "detach": managed != "system"}})
	}
	return legacy.Res{V: out, Code: 200}, rows.Err()
}
func (s *svc) readS3(ctx context.Context, q legacy.DB, uid int64) (storage.Source, error) {
	var v storage.Source
	err := q.QueryRow(ctx, `SELECT id,user_id,kind,root_path,enabled,config,secret FROM storage_source WHERE user_id=$1 AND kind='s3'`, uid).Scan(&v.ID, &v.UserID, &v.Kind, &v.Root, &v.Enabled, &v.Config, &v.Secret)
	return v, err
}
func (s *svc) s3Settings(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	v, err := s.readS3(r.Context(), s.a.Pool, uid)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	values := map[string]string{}
	if len(v.Config) > 0 {
		if err = json.Unmarshal(v.Config, &values); err != nil {
			return nil, err
		}
	}
	delete(values, "access_key")
	delete(values, "secret_key")
	driver, _ := storage.Lookup("s3")
	return legacy.Res{V: map[string]any{"configured": v.ID != 0, "enabled": v.Enabled, "values": values, "hasSecret": len(v.Secret) > 0, "fields": driver.Fields, "canSave": storage.SecretKeyConfigured()}, Code: 200}, nil
}
func (s *svc) s3Input(ctx context.Context, q legacy.DB, uid int64, data map[string]any) (storage.Source, map[string]string, bool, error) {
	old, err := s.readS3(ctx, q, uid)
	if err != nil && err != pgx.ErrNoRows {
		return old, nil, false, err
	}
	values := map[string]string{}
	if old.ID != 0 {
		values, err = storage.SourceValues(old)
		if err != nil {
			return old, nil, false, err
		}
	}
	raw, _ := data["values"].(map[string]any)
	driver, _ := storage.Lookup("s3")
	for _, field := range driver.Fields {
		if v, ok := raw[field.Name]; ok {
			switch x := v.(type) {
			case string:
				if x != "" || field.Name != "secret_key" && field.Name != "access_key" {
					values[field.Name] = x
				}
			case bool:
				if x {
					values[field.Name] = "true"
				} else {
					values[field.Name] = "false"
				}
			}
		}
	}
	enabled := true
	if v, ok := data["enabled"].(bool); ok {
		enabled = v
	}
	return old, values, enabled, nil
}
func verifyS3(ctx context.Context, values map[string]string) error {
	if !storage.SecretKeyConfigured() {
		return legacy.Err("storage_secret_key_missing", 400)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	st, err := storage.Build("s3", values)
	if err != nil {
		return legacy.Err(err.Error(), 400)
	}
	if err = st.Check(ctx); err != nil {
		return legacy.Err("S3 connection or write check failed", 400)
	}
	return nil
}
func (s *svc) testS3(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	data, err := legacy.Body(r)
	if err != nil {
		return nil, err
	}
	_, values, _, err := s.s3Input(r.Context(), s.a.Pool, uid, data)
	if err != nil {
		return nil, err
	}
	if err = verifyS3(r.Context(), values); err != nil {
		return nil, err
	}
	return legacy.Changed(1, 1), nil
}
func (s *svc) saveS3(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	data, err := legacy.Body(r)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	err = legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uid, false, func(tx pgx.Tx, f *legacy.Files, u *legacy.LockedUser) error {
		old, values, enabled, err := s.s3Input(ctx, tx, uid, data)
		if err != nil {
			return err
		}
		if old.ID == 0 && !enabled {
			return nil
		}
		if old.ID != 0 {
			oldValues, e := storage.SourceValues(old)
			if e != nil {
				return e
			}
			if storage.BackendID("s3", oldValues) != storage.BackendID("s3", values) {
				var occupied bool
				if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM photo WHERE source_id=$1) OR EXISTS(SELECT 1 FROM upload_batch b WHERE NOT b.cancelled AND (NOT b.sealed OR EXISTS(SELECT 1 FROM upload_item i WHERE i.batch_id=b.id AND i.state NOT IN ('done','skipped','cancelled'))) AND b.ns_id IN(SELECT ns_id FROM namespace WHERE source_id=$1))`, old.ID).Scan(&occupied); e != nil {
					return e
				}
				if occupied {
					return legacy.Err("storage_source_not_empty", 409)
				}
			}
		}
		if enabled {
			if err = verifyS3(ctx, values); err != nil {
				return err
			}
		}
		plain, secret, err := storage.SealValues(values)
		if err != nil {
			return err
		}
		var sid int64
		err = tx.QueryRow(ctx, `INSERT INTO storage_source(user_id,kind,config,secret,enabled,scan_status) VALUES($1,'s3',$2,$3,$4,'not_applicable') ON CONFLICT(user_id) WHERE kind='s3' DO UPDATE SET config=EXCLUDED.config,secret=EXCLUDED.secret,enabled=EXCLUDED.enabled,revision=storage_source.revision+1 RETURNING id`, uid, plain, secret, enabled).Scan(&sid)
		if err != nil {
			return err
		}
		if old.ID == 0 {
			var parent int64
			if err = tx.QueryRow(ctx, `SELECT ns_id FROM namespace WHERE user_id=$1 AND ns_parent_id=-1`, uid).Scan(&parent); err != nil {
				return err
			}
			name, err := storage.AvailableName(ctx, tx, s.a.Cfg.UploadFolder, parent, "S3", 0, 0)
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO namespace(ns_name,ns_parent_id,user_id,source_id,is_mount) VALUES($1,$2,$3,$4,true)`, name, parent, uid, sid); err != nil {
				return err
			}
		}
		return nil
	})
	return legacy.Changed(1, 1), err
}
