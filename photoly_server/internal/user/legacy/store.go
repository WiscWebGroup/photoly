package legacy

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ISO is serialization.serialize for naive datetimes.
func ISO(t *time.Time) any {
	if t == nil {
		return nil
	}
	if t.Nanosecond()/1000 != 0 {
		return t.Format("2006-01-02T15:04:05.000000")
	}
	return t.Format("2006-01-02T15:04:05")
}

func Day(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format("2006-01-02")
}

const (
	NsCols    = "ns_id, ns_name, ns_parent_id, user_id"
	PhotoCols = "photo_id, photo_name, upload_date, format, ns_id, visibility, token, photo_uuid, user_id"
)

func NsMap(r pgx.Row) (map[string]any, error) {
	var id int64
	var name *string
	var parent, uid *int64
	if err := r.Scan(&id, &name, &parent, &uid); err != nil {
		return nil, err
	}
	return map[string]any{"nsId": id, "nsName": name, "nsParentId": parent, "userId": uid}, nil
}

func PhotoMap(r pgx.Row) (map[string]any, error) {
	var id int64
	var name, format, token, puuid *string
	var up *time.Time
	var ns, vis, uid *int64
	if err := r.Scan(&id, &name, &up, &format, &ns, &vis, &token, &puuid, &uid); err != nil {
		return nil, err
	}
	return map[string]any{"photoId": id, "photoName": name, "uploadDate": ISO(up), "format": format,
		"nsId": ns, "visibility": vis, "token": token, "photoUuid": puuid, "userId": uid}, nil
}

// Rows maps every row of a query; the result is never nil (JSON []).
func Rows(ctx context.Context, db DB, f func(pgx.Row) (map[string]any, error), sql string, args ...any) ([]map[string]any, error) {
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		m, err := f(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Owned is BaseService.owned: the key must exist and belong to uid, else Biz(failure, code).
func Owned(ctx context.Context, db DB, table, pk string, key int64, ok bool, uid int64, failure any, code int) error {
	if ok && InInt32(key) {
		var owner *int64
		err := db.QueryRow(ctx, "SELECT user_id FROM "+table+" WHERE "+pk+"=$1", key).Scan(&owner)
		if err != nil && err != pgx.ErrNoRows {
			return err
		}
		if err == nil && owner != nil && *owner == uid {
			return nil
		}
	}
	return Err(failure, code)
}

// JSONKey reads an id out of a request body field.
func JSONKey(m map[string]any, k string) (int64, bool) { return Int(m[k]) }

var assetSuffixes = []string{"_preview.jpg", "_live.mov", "_live.mp4"}

// DeletePhotos is PhotoService.delete_record over the given photo ids of one user; it
// returns the number of files removed.
func DeletePhotos(ctx context.Context, tx pgx.Tx, f *Files, root string, u *LockedUser, ids []int64) (int, error) {
	count := 0
	for _, id := range ids {
		var puuid, format string
		if err := tx.QueryRow(ctx, `SELECT photo_uuid, format FROM photo WHERE photo_id=$1`, id).Scan(&puuid, &format); err != nil {
			return 0, err
		}
		names := []string{}
		for _, s := range assetSuffixes {
			names = append(names, puuid+s)
		}
		names = append(names, puuid+"."+format, puuid+"_thumbnail."+format)
		for _, n := range names {
			p, err := MediaPath(root, u.UUID, n)
			if err != nil {
				return 0, err
			}
			c, err := f.Remove(p)
			if err != nil {
				return 0, err
			}
			count += c
		}
		if _, err := tx.Exec(ctx, `DELETE FROM photo_media WHERE photo_id=$1`, id); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM photo WHERE photo_id=$1`, id); err != nil {
			return 0, err
		}
	}
	return count, nil
}

// ServeFile is controllers.common.binary for an existing file.
func ServeFile(w http.ResponseWriter, r *http.Request, path, mime, download string) {
	fh, err := os.Open(path)
	st, e2 := os.Stat(path)
	if err != nil || e2 != nil || !st.Mode().IsRegular() {
		w.WriteHeader(404)
		return
	}
	defer fh.Close()
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "private, no-store")
	if download != "" {
		w.Header().Set("Content-Disposition", "attachment; filename="+download)
	}
	http.ServeContent(w, r, "", st.ModTime(), fh)
}

// Entry is one archive member: source file, name inside the zip, and the id used to
// disambiguate duplicate names.
type Entry struct {
	Path, Name string
	ID         int64
}

// ServeArchive is LocalStorage.archive + binary(download.zip). Any missing source is 404.
func ServeArchive(w http.ResponseWriter, r *http.Request, entries []Entry) {
	tmp, err := os.CreateTemp("", "photoly-zip-")
	if err != nil {
		http.Error(w, "internal error", 500)
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	zw := zip.NewWriter(tmp)
	names := map[string]bool{}
	for _, e := range entries {
		st, err := os.Stat(e.Path)
		if err != nil || !st.Mode().IsRegular() {
			w.WriteHeader(404)
			return
		}
		name, i := e.Name, 0
		for names[name] {
			i++
			dir, base := filepath.Split(e.Name)
			ext := filepath.Ext(base)
			if ext == base {
				ext = ""
			}
			name = dir + fmt.Sprintf("%s (%d-%d)%s", strings.TrimSuffix(base, ext), e.ID, i, ext)
		}
		names[name] = true
		hdr := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: st.ModTime()}
		fw, err := zw.CreateHeader(hdr)
		src, e2 := os.Open(e.Path)
		if err != nil || e2 != nil {
			w.WriteHeader(404)
			return
		}
		_, cerr := io.Copy(fw, src)
		src.Close()
		if cerr != nil {
			w.WriteHeader(404)
			return
		}
	}
	if zw.Close() != nil {
		http.Error(w, "internal error", 500)
		return
	}
	ServeFile(w, r, tmp.Name(), "application/octet-stream", "download.zip")
}
