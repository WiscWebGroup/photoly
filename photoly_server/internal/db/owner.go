package db

import (
	"context"
	"fmt"

	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
)

// Owner returns the user_id owning row id of an owned table (photo, namespace, gallery, tag or
// cred). A missing row yields pgx.ErrNoRows; a row without an owner yields 0.
func Owner(ctx context.Context, q dbgen.DBTX, table string, id int64) (int64, error) {
	qs := dbgen.New(q)
	switch table {
	case "photo":
		return qs.GetPhotoOwner(ctx, id)
	case "namespace":
		return qs.GetNamespaceOwner(ctx, id)
	case "gallery":
		return qs.GetGalleryOwner(ctx, id)
	case "tag":
		return qs.GetTagOwner(ctx, id)
	case "cred":
		return qs.GetCredOwner(ctx, id)
	}
	return 0, fmt.Errorf("db.Owner: unknown table %q", table)
}
