package catalog

import (
	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	"github.com/jackc/pgx/v5"
	"net/http"
	"slices"
)

func (s *svc) nsOrder(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	data, err := legacy.Body(r)
	if err != nil {
		return nil, err
	}
	parent, pok := legacy.Int(data["parentId"])
	id, iok := legacy.Int(data["nsId"])
	before, bok := legacy.Int(data["beforeNsId"])
	raw, present := data["beforeNsId"]
	if !pok || !iok || !present || (raw != nil && !bok) {
		return nil, legacy.Bad()
	}
	ctx := storage.ExclusiveUser(r.Context())
	var result []map[string]any
	err = legacy.Atomic(ctx, s.a.Pool, s.a.Cfg.UploadFolder, uid, false, func(tx pgx.Tx, f *legacy.Files, u *legacy.LockedUser) error {
		if e := s.owned(ctx, tx, parent, true, uid, -1); e != nil {
			return e
		}
		// Include disabled mounts so hiding a source never destroys its ordering.
		rows, e := tx.Query(ctx, `SELECT n.ns_id,s.enabled FROM namespace n JOIN storage_source s ON s.id=n.source_id WHERE n.user_id=$1 AND n.ns_parent_id=$2 ORDER BY n.display_order,n.ns_id`, uid, parent)
		if e != nil {
			return e
		}
		ids := []int64{}
		enabled := map[int64]bool{}
		for rows.Next() {
			var n int64
			var active bool
			if e = rows.Scan(&n, &active); e != nil {
				rows.Close()
				return e
			}
			ids = append(ids, n)
			enabled[n] = active
		}
		rows.Close()
		if e = rows.Err(); e != nil {
			return e
		}
		if !enabled[id] || (bok && !enabled[before]) {
			return legacy.Err("folder_order_conflict", 409)
		}
		if !bok || before != id {
			at := slices.Index(ids, id)
			ids = append(ids[:at], ids[at+1:]...)
			pos := len(ids)
			if bok {
				pos = slices.Index(ids, before)
			}
			ids = slices.Insert(ids, pos, id)
			for i, n := range ids {
				if _, e = tx.Exec(ctx, `UPDATE namespace SET display_order=$1 WHERE ns_id=$2`, i+1, n); e != nil {
					return e
				}
			}
			if _, e = tx.Exec(ctx, `UPDATE storage_source SET revision=revision+1 WHERE id=(SELECT source_id FROM namespace WHERE ns_id=$1)`, parent); e != nil {
				return e
			}
		}
		children, e := dbgen.New(tx).ListChildNamespaces(ctx, dbgen.ListChildNamespacesParams{UserID: uid, ParentID: &parent})
		result = legacy.NsListJSON(children)
		return e
	})
	return legacy.Res{V: result, Code: 200}, err
}
