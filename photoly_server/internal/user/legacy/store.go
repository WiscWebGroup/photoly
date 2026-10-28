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

	dbpkg "github.com/WiscWebGroup/photoly/photoly_server/internal/db"
	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
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

// NsJSON is the wire shape of a namespace row.
func NsJSON(n dbgen.Namespace) map[string]any {
	return map[string]any{"nsId": n.NsID, "nsName": n.NsName, "nsParentId": n.NsParentID, "userId": n.UserID}
}

// NsListJSON maps namespace rows; the result is never nil (JSON []).
func NsListJSON(rows []dbgen.Namespace) []map[string]any {
	out := make([]map[string]any, len(rows))
	for i, n := range rows {
		out[i] = NsJSON(n)
	}
	return out
}

// PhotoJSON is the wire shape of a photo row.
func PhotoJSON(p dbgen.Photo) map[string]any {
	return map[string]any{"photoId": p.PhotoID, "photoName": p.PhotoName, "uploadDate": ISO(p.UploadDate), "format": p.Format,
		"nsId": p.NsID, "visibility": p.Visibility, "token": p.Token, "photoUuid": p.PhotoUuid, "userId": p.UserID}
}

// PhotoListJSON maps photo rows; the result is never nil (JSON []).
func PhotoListJSON(rows []dbgen.Photo) []map[string]any {
	out := make([]map[string]any, len(rows))
	for i, p := range rows {
		out[i] = PhotoJSON(p)
	}
	return out
}

// Owned is BaseService.owned: the key must exist and belong to uid, else Biz(failure, code).
func Owned(ctx context.Context, db DB, table string, key int64, ok bool, uid int64, failure any, code int) error {
	if ok && InInt32(key) {
		owner, err := dbpkg.Owner(ctx, db, table, key)
		if err != nil && err != pgx.ErrNoRows {
			return err
		}
		if err == nil && owner == uid {
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
		q := dbgen.New(tx)
		file, err := q.GetPhotoFileNames(ctx, id)
		if err != nil {
			return 0, err
		}
		names := []string{}
		for _, s := range assetSuffixes {
			names = append(names, file.PhotoUuid+s)
		}
		names = append(names, file.PhotoUuid+"."+file.Format, file.PhotoUuid+"_thumbnail."+file.Format)
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
		if err := q.DeletePhotoMedia(ctx, id); err != nil {
			return 0, err
		}
		if err := q.DeletePhoto(ctx, id); err != nil {
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
