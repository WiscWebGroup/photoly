package backup

import (
	"context"
	"fmt"
	"os"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/jackc/pgx/v5"
)

// ReconcileLocalIdentities handles mounts that assign new inode numbers after a
// restart (including Docker Desktop file sharing). Call before starting requests
// or workers, while the maintenance controller holds the instance lock.
// An inode mismatch only triggers verification: all managed originals, including
// Live Photo companions, must match their saved checksums before anything changes.
func ReconcileLocalIdentities(ctx context.Context, cfg app.Config) (bool, error) {
	c, err := pgx.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return false, err
	}
	defer c.Close(ctx)
	rows, err := c.Query(ctx, `SELECT s.id,s.root_identity,EXISTS(SELECT 1 FROM photo p WHERE p.source_id=s.id) FROM storage_source s WHERE s.is_default AND s.kind='local' AND s.enabled AND s.root_identity<>''`)
	if err != nil {
		return false, err
	}
	type root struct {
		id           int64
		identity     string
		hasOriginals bool
	}
	var roots []root
	for rows.Next() {
		var r root
		if err = rows.Scan(&r.id, &r.identity, &r.hasOriginals); err != nil {
			rows.Close()
			return false, err
		}
		roots = append(roots, r)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return false, err
	}
	changed := false
	for _, r := range roots {
		source, err := storage.GetSource(ctx, c, r.id)
		if err != nil {
			return false, err
		}
		path, err := storage.SourceRoot(cfg.UploadFolder, source)
		if err != nil {
			return false, err
		}
		st, err := os.Lstat(path)
		if err != nil {
			return false, err
		}
		if !st.IsDir() {
			return false, fmt.Errorf("source %d: storage root is not a directory", r.id)
		}
		if storage.Identity(st) != r.identity {
			// A library without indexed originals may only rebind to an empty
			// directory; never adopt unverified content from a replacement root.
			if !r.hasOriginals {
				entries, err := os.ReadDir(path)
				if err != nil {
					return false, err
				}
				if len(entries) != 0 {
					return false, fmt.Errorf("source %d: cannot verify changed root without originals", r.id)
				}
			}
			changed = true
		}
	}
	if !changed {
		return false, nil
	}
	if err = finalizeLocalIdentities(ctx, c, cfg.UploadFolder); err != nil {
		return false, err
	}
	return true, nil
}
