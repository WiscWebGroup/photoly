package photo

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

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
	Visibility     *int32
	Token          string
	UUID           string
	UserID         int64
	ContentVersion int32
}

const photoCols = `p.photo_id, p.photo_name, p.upload_date, coalesce(p.format,''), p.ns_id, p.visibility, coalesce(p.token,''), coalesce(p.photo_uuid,''), coalesce(p.user_id,0), p.content_version`

func scanPhoto(row pgx.Row) (*Photo, error) {
	var p Photo
	if err := row.Scan(&p.ID, &p.Name, &p.UploadDate, &p.Format, &p.NsID, &p.Visibility, &p.Token, &p.UUID, &p.UserID, &p.ContentVersion); err != nil {
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

func nilInt32(p *int32) any {
	if p == nil {
		return nil
	}
	return int64(*p)
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
		"nsId": nilInt(p.NsID), "visibility": nilInt32(p.Visibility), "token": p.Token, "photoUuid": p.UUID, "userId": p.UserID}
}

var tables = map[string][2]string{"photo": {"photo", "photo_id"}, "namespace": {"namespace", "ns_id"}, "gallery": {"gallery", "ga_id"}, "tag": {"tag", "tag_id"}}

// Owned is BaseService.owned: the row must exist and belong to uid.
func (s *Service) Owned(ctx context.Context, q DB, kind string, id, uid int64, failure any, code int) error {
	t := tables[kind]
	if id > 0 && inInt32(id) {
		var owner *int64
		err := q.QueryRow(ctx, `SELECT user_id FROM `+t[0]+` WHERE `+t[1]+`=$1`, id).Scan(&owner)
		if err == nil && owner != nil && *owner == uid {
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
		p, err := scanPhoto(q.QueryRow(ctx, `SELECT `+photoCols+` FROM photo p WHERE p.photo_id=$1`, id))
		if err == nil && p.UserID == uid {
			return p, nil
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

func suffixOf(path string) string { return strings.ToLower(strings.TrimPrefix(filepath.Ext(path), ".")) }

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
	vis := int32(0)
	if v, ok := visibility.(int64); ok && v >= 0 && v <= 2 {
		vis = int32(v)
	}
	p := &Photo{Name: &stem, Format: suffix, NsID: &nsID, Visibility: &vis, Token: NewUUID(), UUID: NewUUID(), UserID: u.ID, ContentVersion: 1}
	now := s.Now()
	p.UploadDate = &now
	path, err := s.MediaPath(u.UUID, p.UUID, suffix, false)
	if err != nil {
		return nil, err
	}
	sha, err := f.Install(content, path)
	if err != nil {
		return nil, err
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
		tp, err := s.MediaPath(u.UUID, p.UUID, suffix, true)
		if err != nil {
			return nil, err
		}
		if _, err = f.InstallFile(prepared.Thumbnail, tp); err != nil {
			return nil, err
		}
	}
	err = tx.QueryRow(ctx, `INSERT INTO photo (photo_name, upload_date, format, ns_id, visibility, token, photo_uuid, user_id, content_version, content_sha256)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,1,$9) RETURNING photo_id`, stem, now, suffix, nsID, vis, p.Token, p.UUID, u.ID, sha).Scan(&p.ID)
	if err != nil {
		return nil, err
	}
	if DecodableFormats[strings.ToLower(suffix)] {
		hasPreview := false
		if l := strings.ToLower(suffix); (l == "heic" || l == "heif") && prepared.Preview != "" {
			ap, err := s.AssetPath(u.UUID, p.UUID, "preview")
			if err != nil {
				return nil, err
			}
			if _, err = f.InstallFile(prepared.Preview, ap); err != nil {
				return nil, err
			}
			hasPreview = true
		}
		if _, err = tx.Exec(ctx, `INSERT INTO photo_media (photo_id, animated, has_preview, live) VALUES ($1,$2,$3,false)`, p.ID, prepared.Animated, hasPreview); err != nil {
			return nil, err
		}
	}
	e := ExtraFrom(prepared.Meta)
	if _, err = tx.Exec(ctx, `INSERT INTO photo_extra (f_photo_id, extra_photo_gps_lat, extra_photo_gps_lon, extra_photo_camera, extra_photo_lens,
		extra_photo_aperture, extra_photo_focal, extra_photo_iso, extra_photo_shutter_sec, extra_photo_taken_date) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		p.ID, e.Lat, e.Lon, e.Camera, e.Lens, e.Aperture, e.Focal, e.ISO, e.Shut, e.Taken); err != nil {
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
	for _, role := range assetRoles {
		ap, err := s.AssetPath(userUUID, p.UUID, role)
		if err != nil {
			return 0, err
		}
		c, err := f.Remove(ap)
		if err != nil {
			return 0, err
		}
		n += c
	}
	_, err := tx.Exec(ctx, `DELETE FROM photo_media WHERE photo_id=$1`, p.ID)
	return n, err
}

// DeleteRecord removes the photo's files and rows (PhotoService.delete_record).
func (s *Service) DeleteRecord(ctx context.Context, tx pgx.Tx, f *Files, u UserRow, p *Photo) error {
	if _, err := s.removeAssets(ctx, tx, f, u.UUID, p); err != nil {
		return err
	}
	for _, thumb := range []bool{false, true} {
		path, err := s.MediaPath(u.UUID, p.UUID, p.Format, thumb)
		if err != nil {
			return err
		}
		if _, err = f.Remove(path); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `DELETE FROM photo WHERE photo_id=$1`, p.ID)
	return err
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
		var hasPreview bool
		_ = tx.QueryRow(ctx, `SELECT has_preview FROM photo_media WHERE photo_id=$1`, p.ID).Scan(&hasPreview)
		if hasPreview {
			ap, err := s.AssetPath(u.UUID, p.UUID, "preview")
			if err != nil {
				return err
			}
			if _, err = f.Remove(ap); err != nil {
				return err
			}
		}
		for _, thumb := range []bool{true, false} {
			path, err := s.MediaPath(u.UUID, p.UUID, p.Format, thumb)
			if err != nil {
				return err
			}
			if _, err = f.Remove(path); err != nil {
				return err
			}
		}
		path, err := s.MediaPath(u.UUID, p.UUID, suffix, false)
		if err != nil {
			return err
		}
		sha, err := f.Install(content, path)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE photo SET format=$1, content_sha256=$2, content_version=content_version+1 WHERE photo_id=$3`, suffix, sha, p.ID); err != nil {
			return err
		}
		if Hooks.PhotoChanged != nil {
			if err = Hooks.PhotoChanged(ctx, tx, p.ID, uid); err != nil {
				return err
			}
		}
		thumbPath, err := s.MediaPath(u.UUID, p.UUID, suffix, true)
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
		_, err = tx.Exec(ctx, `INSERT INTO photo_media (photo_id, animated) VALUES ($1,$2)
			ON CONFLICT (photo_id) DO UPDATE SET animated=EXCLUDED.animated, has_preview=false`, p.ID, animated)
		return err
	})
	if err != nil {
		return 0, err
	}
	return 1, nil
}
