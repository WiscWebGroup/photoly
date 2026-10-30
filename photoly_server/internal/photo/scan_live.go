package photo

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/jackc/pgx/v5"
)

type scanAttachment struct {
	pid                                 int64
	uuid, path, identity, hash, version string
	size, mtime                         int64
	missing                             int
	found, preview                      string
}

func (s *Service) loadScanAttachments(ctx context.Context, snap *sourceSnapshot) error {
	rows, e := s.Pool.Query(ctx, `WITH RECURSIVE dirs AS(SELECT ns_id,''::text AS path FROM namespace WHERE source_id=$1 AND (is_mount OR ns_parent_id=-1) UNION ALL SELECT n.ns_id,CASE WHEN d.path='' THEN n.ns_name ELSE d.path||'/'||n.ns_name END FROM namespace n JOIN dirs d ON n.ns_parent_id=d.ns_id WHERE n.source_id=$1) SELECT p.photo_id,p.photo_uuid,CASE WHEN d.path='' THEN a.filename ELSE d.path||'/'||a.filename END,a.fs_identity,a.fs_size,a.fs_mtime,a.content_sha256,a.version,a.missing_scans FROM photo p JOIN media_asset a USING(photo_id) JOIN photo_media m USING(photo_id) JOIN dirs d ON d.ns_id=coalesce(a.namespace_id,p.ns_id) WHERE p.source_id=$1 AND a.role='live_original' AND m.live`, snap.source.ID)
	if e != nil {
		return e
	}
	defer rows.Close()
	for rows.Next() {
		a := &scanAttachment{}
		if e = rows.Scan(&a.pid, &a.uuid, &a.path, &a.identity, &a.size, &a.mtime, &a.hash, &a.version, &a.missing); e != nil {
			return e
		}
		if snap.contains(a.path) {
			snap.attachments = append(snap.attachments, a)
		}
	}
	return rows.Err()
}
func (s *Service) prepareScanAttachments(ctx context.Context, snap *sourceSnapshot) {
	for _, a := range snap.attachments {
		var identities, hashes []string
		for p, v := range snap.entries {
			if v.dir {
				continue
			}
			if a.identity != "" && v.identity == a.identity {
				identities = append(identities, p)
			}
			if a.hash != "" && v.sha == a.hash {
				hashes = append(hashes, p)
			}
		}
		if len(identities) == 1 {
			a.found = identities[0]
		} else if ent, ok := snap.entries[a.path]; ok && !ent.dir {
			a.found = a.path
		} else if len(hashes) == 1 {
			a.found = hashes[0]
		}
		if a.found == "" {
			continue
		}
		ent := snap.entries[a.found]
		if ent.sha != "" && ent.sha != a.hash && snap.failures[a.found] == "" {
			out := filepath.Join(snap.work, fmt.Sprintf("live-existing-%d.mp4", a.pid))
			if e := s.EncodeLive(ctx, snap.originals[a.found], out, false); e != nil {
				snap.failures[a.found] = "live_preview_processing_failed"
			} else {
				a.preview = out
			}
		}
	}
}
func (s *Service) commitScanAttachments(ctx context.Context, tx pgx.Tx, f *Files, u UserRow, snap *sourceSnapshot, dirs map[string]int64) error {
	for _, a := range snap.attachments {
		if a.found == "" {
			if a.missing < 1 {
				if _, e := tx.Exec(ctx, `UPDATE media_asset SET missing_scans=missing_scans+1 WHERE photo_id=$1 AND role='live_original'`, a.pid); e != nil {
					return e
				}
				continue
			}
			if _, e := tx.Exec(ctx, `UPDATE photo_media SET live=false,live_status=NULL,live_error=NULL WHERE photo_id=$1`, a.pid); e != nil {
				return e
			}
			if _, e := tx.Exec(ctx, `UPDATE media_asset SET namespace_id=NULL,missing_scans=0 WHERE photo_id=$1 AND role='live_original'`, a.pid); e != nil {
				return e
			}
			dest, e := storage.Resolve(ctx, tx, s.Root, u.UUID, a.uuid, "live_preview")
			if e != nil {
				return e
			}
			if _, e = f.Remove(dest); e != nil {
				return e
			}
			continue
		}
		ent := snap.entries[a.found]
		// Attachments participate in the same two-observation stability check.
		if _, e := tx.Exec(ctx, `INSERT INTO storage_observation(source_id,path,identity,size,mtime,version) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(source_id,path) DO UPDATE SET identity=EXCLUDED.identity,size=EXCLUDED.size,mtime=EXCLUDED.mtime,version=EXCLUDED.version`, snap.source.ID, a.found, ent.identity, ent.size, ent.mtime, ent.version); e != nil {
			return e
		}
		if ent.sha == "" || snap.failures[a.found] != "" {
			continue
		}
		if _, e := tx.Exec(ctx, `UPDATE media_asset SET filename=$2,display_name=$2,namespace_id=$3,fs_identity=$4,fs_size=$5,fs_mtime=$6,version=$7,content_sha256=$8,missing_scans=0,content_version=content_version+CASE WHEN content_sha256<>$8 THEN 1 ELSE 0 END WHERE photo_id=$1 AND role='live_original'`, a.pid, filepath.Base(a.found), dirs[filepath.Dir(a.found)], ent.identity, ent.size, ent.mtime, ent.version, ent.sha); e != nil {
			return e
		}
		if a.preview != "" {
			dest, e := storage.Resolve(ctx, tx, s.Root, u.UUID, a.uuid, "live_preview")
			if e != nil {
				return e
			}
			if _, e = f.InstallFile(a.preview, dest); e != nil {
				return e
			}
			if _, e = tx.Exec(ctx, `UPDATE photo_media SET live_status='ready',live_error=NULL WHERE photo_id=$1`, a.pid); e != nil {
				return e
			}
			if _, e = tx.Exec(ctx, `UPDATE photo SET content_version=content_version+1 WHERE photo_id=$1`, a.pid); e != nil {
				return e
			}
			if Hooks.PhotoChanged != nil {
				if e = Hooks.PhotoChanged(ctx, tx, a.pid, u.ID); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
