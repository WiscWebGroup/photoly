package legacy

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	mimetype "mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	dbpkg "github.com/WiscWebGroup/photoly/photoly_server/internal/db"
	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
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
	return map[string]any{"nsId": n.NsID, "displayOrder": n.DisplayOrder, "nsName": n.NsName, "nsParentId": n.NsParentID, "userId": n.UserID, "sourceId": n.SourceID, "sourceType": n.SourceType, "isMount": n.IsMount, "canMove": !n.IsMount && n.NsParentID != nil && *n.NsParentID != -1, "canRename": !n.IsMount && n.NsParentID != nil && *n.NsParentID != -1, "canDelete": !n.IsMount && n.NsParentID != nil && *n.NsParentID != -1}
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
		"nsId": p.NsID, "visibility": p.Visibility, "token": p.Token, "photoUuid": p.PhotoUuid, "userId": p.UserID, "sourceId": p.SourceID}
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
		paths, err := storage.AssetPaths(ctx, tx, root, id)
		if err != nil {
			return 0, err
		}
		for _, p := range paths {
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
	storage.Fetch(path)
	fh, err := storage.OpenFile(path)
	if err != nil {
		w.WriteHeader(404)
		return
	}
	defer fh.Close()
	st, e2 := fh.Stat()
	if e2 != nil || !st.Mode().IsRegular() {
		w.WriteHeader(404)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "private, no-store")
	if download != "" {
		w.Header().Set("Content-Disposition", mimetype.FormatMediaType("attachment", map[string]string{"filename": download}))
	}
	http.ServeContent(w, r, "", st.ModTime(), fh)
}

// Entry is one archive member: source file, name inside the zip, and the id used to
// disambiguate duplicate names.
type Entry struct {
	Path, Name string
	ID         int64
	Dir        bool
}

// ServeArchive is LocalStorage.archive + binary(download.zip). Any missing source is 404.
func ServeArchive(w http.ResponseWriter, r *http.Request, entries []Entry) {
	ServeArchiveNamed(w, r, entries, "download.zip")
}
func ServeArchiveNamed(w http.ResponseWriter, r *http.Request, entries []Entry, downloadName string) {
	tempDir, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		http.Error(w, "temporary directory unavailable", 500)
		return
	}
	tmp, err := os.CreateTemp(tempDir, "photoly-zip-")
	if err != nil {
		http.Error(w, "internal error", 500)
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	zw := zip.NewWriter(tmp)
	names := map[string]bool{}
	for _, e := range entries {
		if r.Context().Err() != nil {
			return
		}
		if e.Name == "" || strings.Contains(e.Name, "\\") || strings.HasPrefix(e.Name, "/") || path.Clean(e.Name) != e.Name || strings.HasPrefix(e.Name, "../") {
			http.Error(w, "invalid archive path", 400)
			return
		}
		if e.Dir {
			if _, err := zw.CreateHeader(&zip.FileHeader{Name: e.Name + "/", Method: zip.Store}); err != nil {
				http.Error(w, "archive failed", 500)
				return
			}
			continue
		}
		if err := storage.FetchError(r.Context(), e.Path); err != nil {
			http.Error(w, "archive source unavailable", 404)
			return
		}
		st, err := os.Lstat(e.Path)
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
		src, e2 := storage.OpenFile(e.Path)
		if err != nil || e2 != nil {
			w.WriteHeader(404)
			return
		}
		_, cerr := io.Copy(fw, &archiveReader{r, src})
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
	ServeFile(w, r, tmp.Name(), "application/zip", downloadName)
}

type archiveReader struct {
	request *http.Request
	src     io.Reader
}

func (r *archiveReader) Read(p []byte) (int, error) {
	if e := r.request.Context().Err(); e != nil {
		return 0, e
	}
	return r.src.Read(p)
}

// RemoveUserFiles also removes unindexed files and empty directories.
func RemoveUserFiles(f *Files, root, uuid string) error {
	for _, parts := range [][]string{{uuid}, {".assets", uuid}, {".remote", uuid}} {
		p, e := storage.SafePath(root, parts...)
		if e != nil {
			return e
		}
		if _, e = f.Remove(p); e != nil {
			return e
		}
	}
	return nil
}
