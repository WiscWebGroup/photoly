package photo

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	dbpkg "github.com/WiscWebGroup/photoly/photoly_server/internal/db"
	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/jackc/pgx/v5"
)

// Format sets from services/photo.py.
var (
	ImageFormats    = setOf("jpg", "jpeg", "gif", "png", "webp", "ico", "bmp", "tif", "tiff", "svg", "psd", "raw", "arw", "pcd", "jfif", "heic", "heif")
	EditableFormats = setOf("jpg", "jpeg", "png", "webp")
	VideoFormats    = setOf("mp4", "avi", "wmv", "mpeg", "m4v", "mov", "flv", "asf", "f4v", "rmvb", "vob", "rm")
)

func setOf(v ...string) map[string]bool {
	m := make(map[string]bool, len(v))
	for _, s := range v {
		m[s] = true
	}
	return m
}

// Photo is a row of the photo table.
type Photo struct {
	ID             int64
	Name           *string
	UploadDate     *time.Time
	Format         string
	NsID           *int64
	Visibility     *int64
	Token          string
	UUID           string
	UserID         int64
	ContentVersion int64
	SourceID       int64
}

const photoCols = `p.photo_id, p.photo_name, p.upload_date, coalesce(p.format,''), p.ns_id, p.visibility, coalesce(p.token,''), coalesce(p.photo_uuid,''), coalesce(p.user_id,0), p.content_version, p.source_id`

// photoFromRow adapts a photo table row; NULL text/owner columns read as their zero value.
func photoFromRow(r dbgen.Photo) *Photo {
	return &Photo{ID: r.PhotoID, Name: r.PhotoName, UploadDate: r.UploadDate, Format: deref(r.Format), NsID: r.NsID, Visibility: r.Visibility,
		Token: deref(r.Token), UUID: deref(r.PhotoUuid), UserID: deref(r.UserID), ContentVersion: r.ContentVersion, SourceID: r.SourceID}
}

func deref[T any](p *T) T {
	var z T
	if p == nil {
		return z
	}
	return *p
}

// scanPhoto reads photoCols; only the hand-built queries (ByDate, SearchSmart) still need it.
func scanPhoto(row pgx.Row) (*Photo, error) {
	var p Photo
	if err := row.Scan(&p.ID, &p.Name, &p.UploadDate, &p.Format, &p.NsID, &p.Visibility, &p.Token, &p.UUID, &p.UserID, &p.ContentVersion, &p.SourceID); err != nil {
		return nil, err
	}
	return &p, nil
}

func nilInt(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nilStr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

// JSON is the legacy Photo projection (serialization.LEGACY_FIELDS['Photo']).
func (p *Photo) JSON() map[string]any {
	return map[string]any{"photoId": p.ID, "photoName": nilStr(p.Name), "uploadDate": optTime(p.UploadDate), "format": p.Format,
		"nsId": nilInt(p.NsID), "visibility": nilInt(p.Visibility), "token": p.Token, "photoUuid": p.UUID, "userId": p.UserID, "sourceId": p.SourceID}
}

// Owned is BaseService.owned: the row must exist and belong to uid.
func (s *Service) Owned(ctx context.Context, q DB, kind string, id, uid int64, failure any, code int) error {
	if id > 0 && inInt32(id) {
		owner, err := dbpkg.Owner(ctx, q, kind, id)
		if err == nil && owner == uid {
			return nil
		}
		if err != nil && err != pgx.ErrNoRows {
			return err
		}
	}
	return Err(failure, code)
}

// OwnedPhoto loads an owned photo, failing like BaseService.owned.
func (s *Service) OwnedPhoto(ctx context.Context, q DB, id, uid int64, failure any, code int) (*Photo, error) {
	if id > 0 && inInt32(id) {
		r, err := dbgen.New(q).GetPhoto(ctx, id)
		if err == nil && deref(r.UserID) == uid {
			return photoFromRow(r), nil
		}
		if err != nil && err != pgx.ErrNoRows {
			return nil, err
		}
	}
	return nil, Err(failure, code)
}

// Prepared are the derivatives and metadata computed before the user's write lock.
type Prepared struct {
	Meta      Meta
	Decodable bool
	Animated  bool
	Thumbnail string
	Preview   string
}

func suffixOf(path string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
}

// Prepare is PhotoService.prepare: inspect metadata and build previews in work.
func (s *Service) Prepare(ctx context.Context, path, work string) (*Prepared, error) {
	p := &Prepared{Meta: s.ReadMetadata(ctx, path)}
	suffix := suffixOf(path)
	if !DecodableFormats[suffix] {
		return p, nil
	}
	p.Decodable = true
	animated, err := ImageInfo(ctx, path)
	if err != nil {
		return nil, err
	}
	p.Animated = animated
	timeout := time.Duration(s.Cfg.MediaProcessTimeout * float64(time.Second))
	thumb := filepath.Join(work, "thumbnail.jpg")
	if err := MakeThumbnail(ctx, path, thumb, timeout); err != nil {
		return nil, err
	}
	p.Thumbnail = thumb
	if suffix == "heic" || suffix == "heif" {
		prev := filepath.Join(work, "preview.jpg")
		if err := EncodeImage(ctx, path, prev, "jpeg", timeout); err != nil {
			return nil, err
		}
		p.Preview = prev
	}
	return p, nil
}

var suffixRE = regexp.MustCompile(`^[a-zA-Z0-9]{1,20}$`)

// InstallOne creates a photo record and its files inside the caller's transaction.
func (s *Service) InstallOne(ctx context.Context, tx pgx.Tx, f *Files, u UserRow, filename string, content io.Reader, nsID int64, visibility any, prepared *Prepared) (*Photo, error) {
	i := strings.LastIndex(filename, ".")
	if filename == "" || i < 0 {
		return nil, Bad()
	}
	stem, suffix := filename[:i], filename[i+1:]
	if _, err := ValidName(stem, false); err != nil {
		return nil, err
	}
	if !suffixRE.MatchString(suffix) {
		return nil, Bad()
	}
	vis := int64(0)
	if v, ok := visibility.(int64); ok && v >= 0 && v <= 2 {
		vis = v
	}
	p := &Photo{Name: &stem, Format: suffix, NsID: &nsID, Visibility: &vis, Token: NewUUID(), UUID: NewUUID(), UserID: u.ID, ContentVersion: 1}
	now := s.Now()
	p.UploadDate = &now
	filename, err := storage.AvailableName(ctx, tx, s.Root, nsID, filename, 0, 0)
	if err != nil {
		return nil, err
	}
	stem = strings.TrimSuffix(filename, filepath.Ext(filename))
	p.Name = &stem
	q := dbgen.New(tx)
	p.ID, err = q.InsertPhoto(ctx, dbgen.InsertPhotoParams{PhotoName: stem, UploadDate: now, Format: suffix, NsID: nsID, Visibility: vis,
		Token: p.Token, PhotoUuid: p.UUID, UserID: u.ID, ContentSha256: ""})
	if err != nil {
		return nil, err
	}
	if err = tx.QueryRow(ctx, `SELECT source_id FROM photo WHERE photo_id=$1`, p.ID).Scan(&p.SourceID); err != nil {
		return nil, err
	}
	if err = storage.CreateAssets(ctx, tx, s.Root, p.ID, p.UUID, filename, suffix, nsID); err != nil {
		return nil, err
	}
	path, err := s.MediaPath(ctx, tx, u.UUID, p.UUID, suffix, false)
	if err != nil {
		return nil, err
	}
	dir, source, err := storage.Folder(ctx, tx, s.Root, nsID)
	if err != nil {
		return nil, err
	}
	var next func() (string, error)
	if source.Kind == "local" {
		next = func() (string, error) {
			name, err := storage.AvailableName(ctx, tx, s.Root, nsID, filename, p.ID, 0)
			if err != nil {
				return "", err
			}
			return filepath.Join(dir, name), nil
		}
	}
	sha, actual, err := f.InstallUnique(content, path, next)
	if err != nil {
		return nil, err
	}
	if actual != path {
		path = actual
		filename = filepath.Base(actual)
		stem = strings.TrimSuffix(filename, filepath.Ext(filename))
		p.Name = &stem
		if _, err = tx.Exec(ctx, `UPDATE photo SET file_name=$2,photo_name=$3 WHERE photo_id=$1`, p.ID, filename, stem); err != nil {
			return nil, err
		}
		if _, err = tx.Exec(ctx, `UPDATE media_asset SET filename=$2,display_name=$2 WHERE photo_id=$1 AND role='original'`, p.ID, filename); err != nil {
			return nil, err
		}
	}
	if fi, e := os.Lstat(path); e == nil {
		if _, e = tx.Exec(ctx, `UPDATE photo SET fs_identity=$2,fs_size=$3,fs_mtime=$4 WHERE photo_id=$1`, p.ID, storage.Identity(fi), fi.Size(), fi.ModTime().UnixNano()); e != nil {
			return nil, e
		}
	}
	if prepared == nil {
		prepared, err = s.Prepare(ctx, path, f.Work())
		if err != nil {
			if b, ok := err.(*Biz); ok && b.Code == 415 {
				return nil, Bad() // legacy synchronous upload failure envelope
			}
			return nil, err
		}
	}
	if prepared.Thumbnail != "" {
		tp, err := s.MediaPath(ctx, tx, u.UUID, p.UUID, suffix, true)
		if err != nil {
			return nil, err
		}
		if _, err = f.InstallFile(prepared.Thumbnail, tp); err != nil {
			return nil, err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE photo SET content_sha256=$2 WHERE photo_id=$1`, p.ID, sha); err != nil {
		return nil, err
	}
	if DecodableFormats[strings.ToLower(suffix)] {
		hasPreview := false
		if l := strings.ToLower(suffix); (l == "heic" || l == "heif") && prepared.Preview != "" {
			ap, err := s.AssetPath(ctx, tx, u.UUID, p.UUID, "preview")
			if err != nil {
				return nil, err
			}
			if _, err = f.InstallFile(prepared.Preview, ap); err != nil {
				return nil, err
			}
			hasPreview = true
		}
		if err = q.InsertPhotoMedia(ctx, dbgen.InsertPhotoMediaParams{PhotoID: p.ID, Animated: prepared.Animated, HasPreview: hasPreview}); err != nil {
			return nil, err
		}
	}
	e := ExtraFrom(prepared.Meta)
	if err = q.InsertPhotoExtra(ctx, dbgen.InsertPhotoExtraParams{FPhotoID: p.ID, GpsLat: e.Lat, GpsLon: e.Lon, Camera: e.Camera, Lens: e.Lens,
		Aperture: e.Aperture, Focal: e.Focal, Iso: e.ISO, ShutterSec: e.Shut, TakenDate: e.Taken}); err != nil {
		return nil, err
	}
	if Hooks.PhotoChanged != nil {
		if err = Hooks.PhotoChanged(ctx, tx, p.ID, u.ID); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// removeAssets deletes preview/live files and the photo_media row; returns files removed.
func (s *Service) removeAssets(ctx context.Context, tx pgx.Tx, f *Files, userUUID string, p *Photo) (int, error) {
	n := 0
	live, err := dbgen.New(tx).GetPhotoLive(ctx, p.ID)
	if err != nil && err != pgx.ErrNoRows {
		return 0, err
	}
	for _, role := range assetRoles {
		if role == "live_original" && !live {
			continue
		}
		ap, err := s.AssetPath(ctx, tx, userUUID, p.UUID, role)
		if err != nil {
			return 0, err
		}
		c, err := f.Remove(ap)
		if err != nil {
			return 0, err
		}
		n += c
	}
	return n, dbgen.New(tx).DeletePhotoMedia(ctx, p.ID)
}

// DeleteRecord removes the photo's files and rows (PhotoService.delete_record).
func (s *Service) DeleteRecord(ctx context.Context, tx pgx.Tx, f *Files, u UserRow, p *Photo) error {
	if e := storage.LockPhoto(ctx, tx, p.ID); e != nil {
		return e
	}
	if _, err := s.removeAssets(ctx, tx, f, u.UUID, p); err != nil {
		return err
	}
	for _, thumb := range []bool{false, true} {
		path, err := s.MediaPath(ctx, tx, u.UUID, p.UUID, p.Format, thumb)
		if err != nil {
			return err
		}
		if _, err = f.Remove(path); err != nil {
			return err
		}
	}
	return dbgen.New(tx).DeletePhoto(ctx, p.ID)
}

// Replace swaps an owned image's content in place (PhotoService.replace).
func (s *Service) Replace(ctx context.Context, uid, photoID int64, filename string, content io.Reader) (int, error) {
	i := strings.LastIndex(filename, ".")
	suffix := ""
	if i >= 0 {
		suffix = strings.ToLower(filename[i+1:])
	}
	if !EditableFormats[suffix] {
		return 0, Bad()
	}
	timeout := time.Duration(s.Cfg.MediaProcessTimeout * float64(time.Second))
	err := s.Atomic(ctx, uid, func(tx pgx.Tx, u UserRow, f *Files) error {
		p, err := s.OwnedPhoto(ctx, tx, photoID, uid, -1, 400)
		if err != nil {
			return err
		}
		if !EditableFormats[strings.ToLower(p.Format)] {
			return Bad()
		}
		q := dbgen.New(tx)
		if hasPreview, _ := q.GetPhotoHasPreview(ctx, p.ID); hasPreview {
			ap, err := s.AssetPath(ctx, tx, u.UUID, p.UUID, "preview")
			if err != nil {
				return err
			}
			if _, err = f.Remove(ap); err != nil {
				return err
			}
		}
		for _, thumb := range []bool{true, false} {
			path, err := s.MediaPath(ctx, tx, u.UUID, p.UUID, p.Format, thumb)
			if err != nil {
				return err
			}
			if _, err = f.Remove(path); err != nil {
				return err
			}
		}
		filename, err := storage.AvailableName(ctx, tx, s.Root, *p.NsID, deref(p.Name)+"."+suffix, p.ID, 0)
		if err != nil {
			return err
		}
		_, source, err := storage.Folder(ctx, tx, s.Root, *p.NsID)
		if err != nil {
			return err
		}
		originalName := filename
		if source.Kind == "s3" {
			originalName = "original." + suffix
		}
		if _, err = tx.Exec(ctx, `UPDATE photo SET file_name=$2,photo_name=$3 WHERE photo_id=$1`, p.ID, filename, strings.TrimSuffix(filename, filepath.Ext(filename))); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE media_asset SET filename=CASE role WHEN 'original' THEN $2 ELSE $3 END WHERE photo_id=$1 AND role IN ('original','thumbnail')`, p.ID, originalName, "thumbnail."+suffix); err != nil {
			return err
		}
		path, err := s.MediaPath(ctx, tx, u.UUID, p.UUID, suffix, false)
		if err != nil {
			return err
		}
		sha, err := f.Install(content, path)
		if err != nil {
			return err
		}
		if err = q.ReplacePhotoContent(ctx, dbgen.ReplacePhotoContentParams{Format: suffix, ContentSha256: sha, PhotoID: p.ID}); err != nil {
			return err
		}
		if Hooks.PhotoChanged != nil {
			if err = Hooks.PhotoChanged(ctx, tx, p.ID, uid); err != nil {
				return err
			}
		}
		thumbPath, err := s.MediaPath(ctx, tx, u.UUID, p.UUID, suffix, true)
		if err != nil {
			return err
		}
		tmp := filepath.Join(f.Work(), NewUUID())
		if err = MakeThumbnail(ctx, path, tmp, timeout); err != nil {
			return Bad()
		}
		if _, err = f.InstallFile(tmp, thumbPath); err != nil {
			return err
		}
		_ = os.Remove(tmp)
		animated, err := ImageInfo(ctx, path)
		if err != nil {
			return err
		}
		return q.UpsertPhotoMediaAnimated(ctx, dbgen.UpsertPhotoMediaAnimatedParams{PhotoID: p.ID, Animated: animated})
	})
	if err != nil {
		return 0, err
	}
	return 1, nil
}
