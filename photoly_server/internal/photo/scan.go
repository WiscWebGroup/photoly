package photo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/jackc/pgx/v5"
)

type diskEntry struct {
	path, identity string
	size, mtime    int64
	dir            bool
	sha            string
}
type indexedFolder struct {
	id, parent           int64
	name, identity, path string
	missing              int
}
type indexedPhoto struct {
	id, ns                                  int64
	uuid, name, format, identity, path, sha string
	size, mtime                             int64
	missing                                 int
}

func ScanInterval() time.Duration {
	if v := os.Getenv("STORAGE_SCAN_INTERVAL"); v != "" {
		if d, e := time.ParseDuration(v); e == nil && d > 0 {
			return d
		}
	}
	return time.Minute
}
func ScanWorker(ctx context.Context, a *app.App) {
	s := For(a)
	run := func() {
		rows, e := a.Pool.Query(ctx, `SELECT user_id FROM storage_source WHERE is_default AND enabled ORDER BY id`)
		if e != nil {
			log.Printf("storage scan: %v", e)
			return
		}
		var ids []int64
		for rows.Next() {
			var id int64
			if e = rows.Scan(&id); e != nil {
				break
			}
			ids = append(ids, id)
		}
		rows.Close()
		for _, id := range ids {
			if ctx.Err() != nil {
				return
			}
			if e = s.Scan(ctx, id); e != nil {
				log.Printf("storage scan user %d: %v", id, e)
				_, _ = a.Pool.Exec(ctx, `UPDATE storage_source SET scan_status='error',scan_error=$2 WHERE user_id=$1 AND is_default`, id, e.Error())
			}
		}
	}
	run()
	timer := time.NewTicker(ScanInterval())
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			run()
		}
	}
}

func hashFile(p string) (string, error) {
	f, e := storage.OpenFile(p)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func unchanged(p string, e diskEntry) bool {
	fi, err := os.Lstat(p)
	return err == nil && fi.Mode().IsRegular() && storage.Identity(fi) == e.identity && fi.Size() == e.size && fi.ModTime().UnixNano() == e.mtime
}

// Scan serializes its reconciliation with app mutations using the same user row
// lock. External changes are revalidated before commit. No originals are copied.
func (s *Service) Scan(ctx context.Context, uid int64) error {
	return s.atomic(ctx, uid, true, func(tx pgx.Tx, u UserRow, f *Files) error {
		var sid, rootID int64
		var rootIdentity string
		if e := tx.QueryRow(ctx, `SELECT s.id,n.ns_id,s.root_identity FROM storage_source s JOIN namespace n ON n.source_id=s.id AND n.ns_parent_id=-1 WHERE s.user_id=$1 AND s.is_default AND s.enabled`, uid).Scan(&sid, &rootID, &rootIdentity); e != nil {
			return e
		}
		root, source, e := storage.Folder(ctx, tx, s.Root, rootID)
		if e != nil {
			return e
		}
		fi, e := os.Lstat(root)
		if e != nil {
			return e
		}
		if !fi.IsDir() {
			return errors.New("storage root is not a directory")
		}
		identity := storage.Identity(fi)
		if rootIdentity != "" && rootIdentity != identity {
			return errors.New("storage root identity changed")
		}
		entries := map[string]diskEntry{}
		e = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if !info.IsDir() && !info.Mode().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			entries[rel] = diskEntry{path: rel, identity: storage.Identity(info), size: info.Size(), mtime: info.ModTime().UnixNano(), dir: info.IsDir()}
			return nil
		})
		if e != nil {
			return e
		}
		// Reserve identities across the complete snapshot before any path fallback.
		// An old name can be reused while its original inode moves elsewhere.
		identityPaths := map[string][]string{}
		for path, ent := range entries {
			if ent.identity != "" {
				identityPaths[ent.identity] = append(identityPaths[ent.identity], path)
			}
		}
		reservedElsewhere := func(identity, path string) bool {
			paths := identityPaths[identity]
			return len(paths) == 1 && paths[0] != path
		}
		folders := map[int64]*indexedFolder{}
		byFolderIdentity := map[string][]*indexedFolder{}
		byFolderPath := map[string]*indexedFolder{}
		rows, e := tx.Query(ctx, `SELECT ns_id,ns_parent_id,ns_name,fs_identity,missing_scans FROM namespace WHERE source_id=$1 AND NOT is_mount`, sid)
		if e != nil {
			return e
		}
		for rows.Next() {
			v := &indexedFolder{}
			if e = rows.Scan(&v.id, &v.parent, &v.name, &v.identity, &v.missing); e != nil {
				rows.Close()
				return e
			}
			folders[v.id] = v
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		var folderPath func(int64) string
		folderPath = func(id int64) string {
			v := folders[id]
			if v == nil || v.parent == -1 {
				return "."
			}
			if v.path != "" {
				return v.path
			}
			v.path = filepath.Join(folderPath(v.parent), v.name)
			return v.path
		}
		for id, v := range folders {
			v.path = folderPath(id)
			byFolderPath[v.path] = v
			if v.identity != "" {
				byFolderIdentity[v.identity] = append(byFolderIdentity[v.identity], v)
			}
		}
		// Read photo paths before reconciling directories, to recognize unique moved content.
		var photos []*indexedPhoto
		byPhotoIdentity := map[string][]*indexedPhoto{}
		byPhotoPath := map[string]*indexedPhoto{}
		rows, e = tx.Query(ctx, `SELECT photo_id,ns_id,photo_uuid,file_name,format,fs_identity,fs_size,fs_mtime,missing_scans,coalesce(content_sha256,'') FROM photo WHERE source_id=$1`, sid)
		if e != nil {
			return e
		}
		for rows.Next() {
			v := &indexedPhoto{}
			if e = rows.Scan(&v.id, &v.ns, &v.uuid, &v.name, &v.format, &v.identity, &v.size, &v.mtime, &v.missing, &v.sha); e != nil {
				rows.Close()
				return e
			}
			v.path = filepath.Join(folderPath(v.ns), v.name)
			photos = append(photos, v)
			byPhotoPath[v.path] = v
			if v.identity != "" {
				byPhotoIdentity[v.identity] = append(byPhotoIdentity[v.identity], v)
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		var dirs []string
		for p, v := range entries {
			if v.dir && p != "." {
				dirs = append(dirs, p)
			}
		}
		sort.Slice(dirs, func(i, j int) bool {
			di, dj := strings.Count(dirs[i], string(filepath.Separator)), strings.Count(dirs[j], string(filepath.Separator))
			if di != dj {
				return di < dj
			}
			return dirs[i] < dirs[j]
		})
		currentDirs := map[string]int64{".": rootID}
		seenFolders := map[int64]bool{rootID: true}
		// Move a virtual mount label aside if an external directory takes it.
		for _, p := range dirs {
			if filepath.Dir(p) != "." {
				continue
			}
			var mountID int64
			err := tx.QueryRow(ctx, `SELECT ns_id FROM namespace WHERE ns_parent_id=$1 AND is_mount AND ns_name=$2`, rootID, filepath.Base(p)).Scan(&mountID)
			if err == pgx.ErrNoRows {
				continue
			}
			if err != nil {
				return err
			}
			name, err := storage.AvailableName(ctx, tx, s.Root, rootID, "S3", 0, mountID)
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE namespace SET ns_name=$2 WHERE ns_id=$1`, mountID, name); err != nil {
				return err
			}
		}

		for _, p := range dirs {
			ent := entries[p]
			var old *indexedFolder
			if candidates := byFolderIdentity[ent.identity]; ent.identity != "" && len(candidates) == 1 && !seenFolders[candidates[0].id] {
				old = candidates[0]
			}
			if old == nil {
				v := byFolderPath[p]
				if v != nil && !seenFolders[v.id] && !reservedElsewhere(v.identity, p) {
					old = v
				}
			}
			parent := currentDirs[filepath.Dir(p)]
			name := filepath.Base(p)
			if old == nil {
				old = &indexedFolder{}
				e = tx.QueryRow(ctx, `INSERT INTO namespace(ns_name,ns_parent_id,user_id,fs_identity) VALUES($1,$2,$3,$4) RETURNING ns_id`, name, parent, uid, ent.identity).Scan(&old.id)
			} else if old.parent != parent || old.name != name || old.identity != ent.identity || old.missing != 0 {
				_, e = tx.Exec(ctx, `UPDATE namespace SET ns_name=$2,ns_parent_id=$3,fs_identity=$4,missing_scans=0 WHERE ns_id=$1`, old.id, name, parent, ent.identity)
			}
			if e != nil {
				return e
			}
			currentDirs[p] = old.id
			seenFolders[old.id] = true
		}
		// Only live originals actually attached to a Live Photo are excluded from indexing.
		livePaths := map[string]bool{}
		rows, e = tx.Query(ctx, `SELECT p.ns_id,a.filename FROM photo p JOIN media_asset a USING(photo_id) JOIN photo_media m USING(photo_id) WHERE p.source_id=$1 AND a.role='live_original' AND m.live`, sid)
		if e != nil {
			return e
		}
		for rows.Next() {
			var ns int64
			var name string
			if e = rows.Scan(&ns, &name); e != nil {
				rows.Close()
				return e
			}
			for p, id := range currentDirs {
				if id == ns {
					livePaths[filepath.Join(p, name)] = true
				}
			}
		}
		rows.Close()
		observations := map[string]diskEntry{}
		rows, e = tx.Query(ctx, `SELECT path,identity,size,mtime FROM storage_observation WHERE source_id=$1`, sid)
		if e != nil {
			return e
		}
		for rows.Next() {
			var v diskEntry
			if e = rows.Scan(&v.path, &v.identity, &v.size, &v.mtime); e != nil {
				rows.Close()
				return e
			}
			observations[v.path] = v
		}
		rows.Close()
		candidates := map[string]diskEntry{}
		seenPhotos := map[int64]bool{}
		for path, ent := range entries {
			if ent.dir || livePaths[path] {
				continue
			}
			ext := suffixOf(path)
			if !ImageFormats[ext] && !VideoFormats[ext] {
				continue
			}
			old := byPhotoPath[path]
			if old != nil && old.identity == ent.identity && old.size == ent.size && old.mtime == ent.mtime {
				seenPhotos[old.id] = true
				if old.missing != 0 {
					if _, e = tx.Exec(ctx, `UPDATE photo SET missing_scans=0 WHERE photo_id=$1`, old.id); e != nil {
						return e
					}
				}
				continue
			}
			if vs := byPhotoIdentity[ent.identity]; ent.identity != "" && len(vs) == 1 && len(identityPaths[ent.identity]) == 1 {
				v := vs[0]
				if v.size == ent.size && v.mtime == ent.mtime && v.sha != "" {
					// A namespace-only move of already indexed, stable content needs no
					// processing delay, and must release the old name in this commit.
					ent.sha = v.sha
					candidates[path] = ent
					continue
				}
			}
			prev, ok := observations[path]
			if _, e = tx.Exec(ctx, `INSERT INTO storage_observation(source_id,path,identity,size,mtime) VALUES($1,$2,$3,$4,$5) ON CONFLICT(source_id,path) DO UPDATE SET identity=EXCLUDED.identity,size=EXCLUDED.size,mtime=EXCLUDED.mtime`, sid, path, ent.identity, ent.size, ent.mtime); e != nil {
				return e
			}
			if !ok || prev.identity != ent.identity || prev.size != ent.size || prev.mtime != ent.mtime {
				if old != nil && !reservedElsewhere(old.identity, path) {
					seenPhotos[old.id] = true
				}
				for _, v := range byPhotoIdentity[ent.identity] {
					seenPhotos[v.id] = true
				}
				continue
			}
			ent.sha, e = hashFile(filepath.Join(root, path))
			if e != nil {
				return e
			}
			if !unchanged(filepath.Join(root, path), ent) {
				return errors.New("file changed during scan")
			}
			candidates[path] = ent
		}
		missingHashes := map[string][]*indexedPhoto{}
		newHashes := map[string]int{}
		for _, v := range photos {
			if _, present := entries[v.path]; !present && v.sha != "" {
				missingHashes[v.sha] = append(missingHashes[v.sha], v)
			}
		}
		for _, v := range candidates {
			newHashes[v.sha]++
		}
		paths := make([]string, 0, len(candidates))
		for p := range candidates {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, path := range paths {
			ent := candidates[path]
			var old *indexedPhoto
			if vs := byPhotoIdentity[ent.identity]; ent.identity != "" && len(vs) == 1 && !seenPhotos[vs[0].id] {
				old = vs[0]
			}
			if old == nil {
				v := byPhotoPath[path]
				if v != nil && !seenPhotos[v.id] && !reservedElsewhere(v.identity, path) {
					old = v
				}
			}
			if old == nil && newHashes[ent.sha] == 1 && len(missingHashes[ent.sha]) == 1 {
				v := missingHashes[ent.sha][0]
				if !seenPhotos[v.id] {
					old = v
				}
			}
			ns := currentDirs[filepath.Dir(path)]
			name := filepath.Base(path)
			if old == nil {
				old = &indexedPhoto{uuid: NewUUID(), format: strings.TrimPrefix(filepath.Ext(name), ".")}
				stem := strings.TrimSuffix(name, filepath.Ext(name))
				e = tx.QueryRow(ctx, `INSERT INTO photo(photo_name,upload_date,format,ns_id,visibility,token,photo_uuid,user_id,file_name,content_sha256) VALUES($1,$2,$3,$4,0,$5,$6,$7,$8,$9) RETURNING photo_id`, stem, s.Now(), old.format, ns, NewUUID(), old.uuid, uid, name, ent.sha).Scan(&old.id)
				if e != nil {
					return e
				}
				if e = storage.CreateAssets(ctx, tx, s.Root, old.id, old.uuid, name, old.format, ns); e != nil {
					return e
				}
			}
			changed := old.sha != ent.sha
			if _, e = tx.Exec(ctx, `UPDATE photo SET ns_id=$2,file_name=$3,photo_name=$4,format=$5,fs_identity=$6,fs_size=$7,fs_mtime=$8,missing_scans=0,content_sha256=$9::text,content_version=content_version+CASE WHEN content_sha256 IS DISTINCT FROM $9::text THEN 1 ELSE 0 END WHERE photo_id=$1`, old.id, ns, name, strings.TrimSuffix(name, filepath.Ext(name)), strings.TrimPrefix(filepath.Ext(name), "."), ent.identity, ent.size, ent.mtime, ent.sha); e != nil {
				return e
			}
			if _, e = tx.Exec(ctx, `UPDATE media_asset SET filename=$2 WHERE photo_id=$1 AND role='original'`, old.id, name); e != nil {
				return e
			}
			if changed {
				if e = s.indexDerivatives(ctx, tx, f, u, old.id, old.uuid, filepath.Join(root, path)); e != nil {
					return fmt.Errorf("index %s: %w", path, e)
				}
			}
			if !unchanged(filepath.Join(root, path), ent) {
				return errors.New("file changed during processing")
			}
			seenPhotos[old.id] = true
		}
		// A failed walk/processing never reaches deletion. Recheck absent paths now.
		for _, v := range photos {
			if seenPhotos[v.id] {
				continue
			}
			if ent, present := entries[v.path]; present && !ent.dir {
				continue
			}
			st, err := os.Lstat(filepath.Join(root, v.path))
			if err == nil && st.Mode().IsRegular() || err != nil && !os.IsNotExist(err) && !errors.Is(err, syscall.ENOTDIR) {
				return errors.New("library changed during scan")
			}
			if v.missing < 1 {
				if _, e = tx.Exec(ctx, `UPDATE photo SET missing_scans=missing_scans+1 WHERE photo_id=$1`, v.id); e != nil {
					return e
				}
				continue
			}
			// Remove derivatives only: an original may have moved outside the root.
			for _, role := range []string{"thumbnail", "preview", "live_preview"} {
				p, err := storage.Resolve(ctx, tx, s.Root, u.UUID, v.uuid, role)
				if err != nil {
					return err
				}
				if _, err = f.Remove(p); err != nil {
					return err
				}
			}
			if _, e = tx.Exec(ctx, `DELETE FROM photo WHERE photo_id=$1`, v.id); e != nil {
				return e
			}
		}
		var missing []*indexedFolder
		for _, v := range folders {
			if !seenFolders[v.id] {
				missing = append(missing, v)
			}
		}
		sort.Slice(missing, func(i, j int) bool { return len(missing[i].path) > len(missing[j].path) })
		for _, v := range missing {
			st, err := os.Lstat(filepath.Join(root, v.path))
			if err == nil && st.IsDir() || err != nil && !os.IsNotExist(err) && !errors.Is(err, syscall.ENOTDIR) {
				return errors.New("directory changed during scan")
			}
			if v.missing < 1 {
				_, e = tx.Exec(ctx, `UPDATE namespace SET missing_scans=missing_scans+1 WHERE ns_id=$1`, v.id)
			} else {
				_, e = tx.Exec(ctx, `DELETE FROM namespace WHERE ns_id=$1 AND NOT EXISTS(SELECT 1 FROM photo WHERE ns_id=$1) AND NOT EXISTS(SELECT 1 FROM namespace WHERE ns_parent_id=$1)`, v.id)
			}
			if e != nil {
				return e
			}
		}
		// Verify every observed directory still names the same inode. A detached mount
		// or rename during the scan must never be committed as a complete snapshot.
		for path, ent := range entries {
			if !ent.dir {
				continue
			}
			fi, e := os.Lstat(filepath.Join(root, path))
			if e != nil || !fi.IsDir() || storage.Identity(fi) != ent.identity {
				return errors.New("directory changed during scan")
			}
		}
		for path := range observations {
			if _, present := entries[path]; !present {
				if _, e = tx.Exec(ctx, `DELETE FROM storage_observation WHERE source_id=$1 AND path=$2`, sid, path); e != nil {
					return e
				}
			}
		}
		_, e = tx.Exec(ctx, `UPDATE storage_source SET root_identity=$2,scan_status='ready',scan_error=NULL,scanned_at=now() WHERE id=$1`, source.ID, identity)
		return e
	})
}

func (s *Service) indexDerivatives(ctx context.Context, tx pgx.Tx, f *Files, u UserRow, pid int64, uuid, path string) error {
	// Decode a private snapshot: external media tools must not follow a path that
	// an outside writer swaps for a symlink after the directory walk.
	in, err := storage.OpenFile(path)
	if err != nil {
		return err
	}
	defer in.Close()
	snapshot := filepath.Join(f.Work(), "scan-source"+filepath.Ext(path))
	out, err := os.Create(snapshot)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if e := out.Close(); err == nil {
		err = e
	}
	defer os.Remove(snapshot)
	if err != nil {
		return err
	}
	prep, err := s.Prepare(ctx, snapshot, f.Work())
	if err != nil {
		return err
	}
	for role, src := range map[string]string{"thumbnail": prep.Thumbnail, "preview": prep.Preview} {
		dest, e := storage.Resolve(ctx, tx, s.Root, u.UUID, uuid, role)
		if e != nil {
			return e
		}
		if src != "" {
			if _, e = f.InstallFile(src, dest); e != nil {
				return e
			}
		} else if _, e = f.Remove(dest); e != nil {
			return e
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO photo_media(photo_id,animated,has_preview) VALUES($1,$2,$3) ON CONFLICT(photo_id) DO UPDATE SET animated=EXCLUDED.animated,has_preview=EXCLUDED.has_preview`, pid, prep.Animated, prep.Preview != ""); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM photo_extra WHERE f_photo_id=$1`, pid); err != nil {
		return err
	}
	e := ExtraFrom(prep.Meta)
	if err = dbgen.New(tx).InsertPhotoExtra(ctx, dbgen.InsertPhotoExtraParams{FPhotoID: pid, GpsLat: e.Lat, GpsLon: e.Lon, Camera: e.Camera, Lens: e.Lens, Aperture: e.Aperture, Focal: e.Focal, Iso: e.ISO, ShutterSec: e.Shut, TakenDate: e.Taken}); err != nil {
		return err
	}
	if Hooks.PhotoChanged != nil {
		return Hooks.PhotoChanged(ctx, tx, pid, u.ID)
	}
	return nil
}
