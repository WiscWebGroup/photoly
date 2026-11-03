package catalog

import (
	"context"
	"net/http"

	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
)

func namespacePins(ctx context.Context, q *dbgen.Queries, uid int64) (any, error) {
	rows, err := q.ListNamespacePins(ctx, uid)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(rows))
	var defaultID *int64
	for _, row := range rows {
		item := legacy.NsJSON(row.Namespace)
		item["path"] = row.Path
		items = append(items, item)
		if row.IsDefault {
			id := row.Namespace.NsID
			defaultID = &id
		}
	}
	return map[string]any{"items": items, "defaultNsId": defaultID}, nil
}

func (s *svc) nsPins(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	w.Header().Set("Cache-Control", "private, no-store")
	result, err := namespacePins(r.Context(), dbgen.New(s.a.Pool), uid)
	return legacy.Res{V: result, Code: 200}, err
}

func (s *svc) nsPinAction(action string) legacy.Handler {
	return func(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
		data, err := legacy.Body(r)
		if err != nil {
			return nil, err
		}
		raw, present := data["nsId"]
		id, valid := legacy.Int(raw)
		clear := action == "default" && present && raw == nil
		if !clear && (!valid || id <= 0 || !legacy.InInt32(id)) {
			return nil, legacy.Bad()
		}
		ctx := r.Context()
		tx, err := s.a.Pool.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback(context.WithoutCancel(ctx))
		// Serialize preference writes for an account, including writes from other devices.
		var locked int64
		if err = tx.QueryRow(ctx, `SELECT user_id FROM "user" WHERE user_id=$1 FOR UPDATE`, uid).Scan(&locked); err != nil {
			return nil, err
		}
		q := dbgen.New(tx)
		switch action {
		case "pin":
			if err = s.owned(ctx, tx, id, true, uid, -1); err != nil {
				return nil, err
			}
			err = q.PinNamespace(ctx, dbgen.PinNamespaceParams{UserID: uid, NsID: id})
		case "unpin":
			// A deleted or foreign ID is a no-op: only this user's own pins can be removed.
			err = q.UnpinNamespace(ctx, dbgen.UnpinNamespaceParams{UserID: uid, NsID: id})
		case "default":
			if !clear {
				if err = s.owned(ctx, tx, id, true, uid, -1); err != nil {
					return nil, err
				}
			}
			if err = q.ClearDefaultNamespace(ctx, uid); err != nil {
				return nil, err
			}
			if !clear {
				var changed int64
				changed, err = q.SetDefaultNamespace(ctx, dbgen.SetDefaultNamespaceParams{UserID: uid, NsID: id})
				if err == nil && changed != 1 {
					return nil, legacy.Err("folder_not_pinned", 409)
				}
			}
		}
		if err != nil {
			return nil, err
		}
		result, err := namespacePins(ctx, q, uid)
		if err != nil {
			return nil, err
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return legacy.Res{V: result, Code: 200}, nil
	}
}
