package photo

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// uuidNS is uuid.UUID(key) as a namespace for uuid5.
func (s *Service) timeout() time.Duration {
	return time.Duration(s.Cfg.MediaProcessTimeout * float64(time.Second))
}

// Capabilities is MediaService.capabilities.
func (s *Service) Capabilities(ctx context.Context) map[string]any {
	return map[string]any{"onlineDownload": s.Setting(ctx, s.Pool, "OnlineDownload", "0") == "1", "convertFormats": Targets}
}

func (s *Service) requireOnline(ctx context.Context) error {
	if s.Setting(ctx, s.Pool, "OnlineDownload", "0") != "1" {
		return Err("online_download_disabled", 403)
	}
	return nil
}

// MediaInfo is MediaService.info.
func (s *Service) MediaInfo(ctx context.Context, uid, photoID int64) (any, error) {
	p, err := s.OwnedPhoto(ctx, s.Pool, photoID, uid, "photo_not_found", 404)
	if err != nil {
		return nil, err
	}
	var animated, live bool
	var status, lerr *string
	err = s.Pool.QueryRow(ctx, `SELECT animated, live, live_status, live_error FROM photo_media WHERE photo_id=$1`, photoID).Scan(&animated, &live, &status, &lerr)
	has := err == nil
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	decodable := DecodableFormats[strings.ToLower(p.Format)]
	if !has && decodable {
		if uu, err := s.userUUID(ctx, s.Pool, uid); err == nil {
			if path, err := s.MediaPath(uu, p.UUID, p.Format, false); err == nil {
				if a, err := ImageInfo(ctx, path); err == nil {
					animated = a
				}
			}
		}
	}
	formats := []string{}
	if decodable {
		formats = Targets
	}
	return map[string]any{"photoId": photoID, "animated": animated, "live": has && live, "liveStatus": nilStr(status),
		"liveError": nilStr(lerr), "convertFormats": formats, "nsId": nilInt(p.NsID)}, nil
}

// uploadsNow is uploads.now(): aware UTC.
func uploadsNow() time.Time { return time.Now().UTC() }

// Identifier is uploads.identifier: a canonical UUID string.
func Identifier(v any) (string, error) {
	str, ok := v.(string)
	if ok {
		if norm, _, ok := ParseUUID(str); ok {
			return norm, nil
		}
	}
	return "", Err("invalid_upload_id", 400)
}

// Submit is MediaService.submit: an idempotent upload_batch/upload_item pair processed by the worker.
func (s *Service) Submit(ctx context.Context, uid int64, data map[string]any, kind string) (any, error) {
	key, err := Identifier(data["id"])
	if err != nil {
		return nil, err
	}
	resource := "photoId"
	if kind == "online" {
		resource = "nsId"
	}
	if id, ok := intOf(data[resource]); !ok || id <= 0 {
		return nil, Err("invalid_resource_id", 400)
	}
	target := data["format"]
	if kind != "live_preview" {
		t, _ := target.(string)
		if t != "jpeg" && t != "png" && t != "gif" {
			return nil, Err("unsupported_target_format", 400)
		}
	}
	keys := []string{"photoId", "format"}
	if kind == "online" {
		keys = []string{"url", "nsId", "format"}
	}
	request := map[string]any{}
	for _, k := range keys {
		request[k] = data[k] // nil when absent, like dict.get
	}
	err = s.Atomic(ctx, uid, func(tx pgx.Tx, u UserRow, f *Files) error {
		var owner int64
		switch err := tx.QueryRow(ctx, `SELECT user_id FROM upload_batch WHERE id=$1`, key).Scan(&owner); err {
		case nil:
			if owner != uid {
				return Err("upload_id_conflict", 409)
			}
			var pk string
			var opts []byte
			err = tx.QueryRow(ctx, `SELECT kind, options FROM upload_item WHERE batch_id=$1 LIMIT 1`, key).Scan(&pk, &opts)
			if err == pgx.ErrNoRows || pk != kind || !sameRequest(opts, request) {
				return Err("upload_id_conflict", 409)
			} else if err != nil {
				return err
			}
			return nil
		case pgx.ErrNoRows:
		default:
			return err
		}
		options := map[string]any{"request": request, "format": target}
		var nsID int64
		var name string
		if kind == "online" {
			if err := s.requireOnline(ctx); err != nil {
				return err
			}
			raw, ok := data["url"].(string)
			if !ok || len(raw) > 8192 || strings.ContainsRune(raw, '\\') || strings.IndexFunc(raw, func(r rune) bool { return r < 33 }) >= 0 {
				return Err("invalid_image_url", 400)
			}
			if u, err := url.Parse(raw); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
				return Err("invalid_image_url", 400)
			}
			options["url"] = raw
			nsID, _ = intOf(data["nsId"])
			name = "Online Download"
		} else {
			id, _ := intOf(data["photoId"])
			p, err := s.OwnedPhoto(ctx, tx, id, uid, "photo_not_found", 404)
			if err != nil {
				return err
			}
			if !DecodableFormats[strings.ToLower(p.Format)] {
				return Err("unsupported_image", 400)
			}
			if kind == "live_preview" {
				var live bool
				if err := tx.QueryRow(ctx, `SELECT live FROM photo_media WHERE photo_id=$1`, p.ID).Scan(&live); err != nil || !live {
					return Err("not_live_photo", 400)
				}
			}
			options["photoId"], options["version"] = p.ID, p.ContentVersion
			if p.NsID != nil {
				nsID = *p.NsID
			}
			name = "Live preview"
			if kind == "convert" {
				name = "Format Convert"
			}
		}
		if err := s.Owned(ctx, tx, "namespace", nsID, uid, "destination_deleted", 404); err != nil {
			return err
		}
		now := uploadsNow()
		if _, err := tx.Exec(ctx, `INSERT INTO upload_batch (id, user_id, ns_id, name, sealed, cancelled, created_at, updated_at, folders, dismissed)
			VALUES ($1,$2,$3,$4,true,false,$5,$5,'{}',false)`, key, uid, nsID, name, now); err != nil {
			return err
		}
		_, kb, _ := ParseUUID(key)
		raw, _ := json.Marshal(options)
		_, err := tx.Exec(ctx, `INSERT INTO upload_item (id, batch_id, path, size, state, kind, options, updated_at)
			VALUES ($1,$2,$3,0,'ready',$4,$5,$6)`, UUID5(kb, kind), key, name, kind, raw, now)
		return err
	})
	if err != nil {
		return nil, err
	}
	return key, nil
}

func sameRequest(stored []byte, request map[string]any) bool {
	var o struct {
		Request map[string]any `json:"request"`
	}
	if json.Unmarshal(stored, &o) != nil {
		return false
	}
	raw, _ := json.Marshal(request)
	var want map[string]any
	_ = json.Unmarshal(raw, &want)
	return reflect.DeepEqual(o.Request, want)
}

func (s *Service) checkedSource(ctx context.Context, q DB, uid int64, options map[string]any) (*Photo, error) {
	id, _ := options["photoId"].(float64)
	p, err := s.OwnedPhoto(ctx, q, int64(id), uid, "source_changed", 409)
	if err != nil {
		return nil, err
	}
	if v, _ := options["version"].(float64); int32(v) != p.ContentVersion {
		return nil, Err("source_changed", 409)
	}
	return p, nil
}

// EncodeLive is infrastructure.media.encode_live (ffmpeg).
func (s *Service) EncodeLive(ctx context.Context, src, out string, gif bool) error {
	args := []string{"-protocol_whitelist", "file,pipe", "-i", src}
	if gif {
		args = append(args, "-filter_complex_threads", "1", "-filter_complex",
			"fps=15,scale=1280:-2:force_original_aspect_ratio=decrease,split[a][b];[a]palettegen[p];[b][p]paletteuse", "-an", "-loop", "0")
	} else {
		args = append(args, "-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2", "-c:v", "libx264", "-threads", "2",
			"-pix_fmt", "yuv420p", "-c:a", "aac", "-movflags", "+faststart")
	}
	c, cancel := context.WithTimeout(ctx, s.timeout())
	defer cancel()
	cmd := exec.CommandContext(c, "ffmpeg", append([]string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y", "-threads", "2"}, append(args, out)...)...)
	if err := cmd.Run(); err != nil {
		return Err("video_conversion_failed", 400)
	}
	return nil
}

// ProcessMedia runs a convert/online/live_preview item (MediaService.process). The caller owns the
// item's 'processing' state and records any returned error.
func (s *Service) ProcessMedia(ctx context.Context, itemID string) error {
	var batchID, kind string
	var rawOpts []byte
	var uid int64
	err := s.Pool.QueryRow(ctx, `SELECT i.batch_id, i.kind, i.options, b.user_id FROM upload_item i JOIN upload_batch b ON b.id=i.batch_id WHERE i.id=$1`, itemID).
		Scan(&batchID, &kind, &rawOpts, &uid)
	if err != nil {
		return err
	}
	options := map[string]any{}
	_ = json.Unmarshal(rawOpts, &options)
	target, _ := options["format"].(string)
	staging := filepath.Join(s.Root, ".staging")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return err
	}
	work, err := os.MkdirTemp(staging, "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	source := filepath.Join(work, "source")
	ext := target
	if kind == "live_preview" {
		ext = "mp4"
	}
	output := filepath.Join(work, "output."+ext)
	var originalID int64
	var name string
	if kind == "online" {
		if err := s.requireOnline(ctx); err != nil {
			return err
		}
		rawURL, _ := options["url"].(string)
		if err := s.Download(ctx, rawURL, source); err != nil {
			return err
		}
		name = "Downloaded image"
	} else {
		p, err := s.checkedSource(ctx, s.Pool, uid, options)
		if err != nil {
			return err
		}
		originalID = p.ID
		if p.Name != nil {
			name = *p.Name
		}
		uu, err := s.userUUID(ctx, s.Pool, uid)
		if err != nil {
			return err
		}
		var live bool
		_ = s.Pool.QueryRow(ctx, `SELECT live FROM photo_media WHERE photo_id=$1`, p.ID).Scan(&live)
		useVideo := live && (target == "gif" || kind == "live_preview")
		var path string
		if useVideo {
			path, err = s.AssetPath(uu, p.UUID, "live_original")
		} else {
			path, err = s.MediaPath(uu, p.UUID, p.Format, false)
		}
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return Err("source_changed", 409)
		}
		out, err := os.Create(source)
		if err == nil {
			_, err = io.Copy(out, in)
			out.Close()
		}
		in.Close()
		if err != nil {
			return err
		}
		if useVideo {
			if err := s.EncodeLive(ctx, source, output, target == "gif"); err != nil {
				return err
			}
		}
	}
	if _, err := os.Stat(output); err != nil {
		c, cancel := context.WithTimeout(ctx, s.timeout())
		err := EncodeImage(c, source, output, target, s.timeout())
		cancel()
		if err != nil {
			if b, ok := BizOf(err); ok && (b.Value == "invalid_image" || b.Value == "animation_too_large" || b.Value == "image_too_many_pixels") {
				return Err(b.Value, 400)
			}
			if c.Err() != nil {
				return Err("conversion_timeout", 400)
			}
			return Err("conversion_failed", 400)
		}
	}
	fi, err := os.Stat(output)
	if err != nil {
		return err
	}
	if fi.Size() > s.Cfg.MaxContentLength {
		return Err("file_too_large", 413)
	}
	var prepared *Prepared
	if kind != "live_preview" {
		if prepared, err = s.Prepare(ctx, output, work); err != nil {
			return err
		}
	}
	return s.Atomic(ctx, uid, func(tx pgx.Tx, u UserRow, f *Files) error {
		var state string
		var cancelled bool
		var nsID *int64
		err := tx.QueryRow(ctx, `SELECT i.state, b.cancelled, b.ns_id FROM upload_item i JOIN upload_batch b ON b.id=i.batch_id
			WHERE i.id=$1 AND b.user_id=$2`, itemID, uid).Scan(&state, &cancelled, &nsID)
		if err == pgx.ErrNoRows {
			return Err("upload_not_found", 404)
		} else if err != nil {
			return err
		}
		if cancelled || state != "processing" {
			return nil
		}
		if kind == "online" {
			if err := s.requireOnline(ctx); err != nil {
				return err
			}
		}
		var original *Photo
		if kind != "online" {
			if original, err = s.checkedSource(ctx, tx, uid, options); err != nil {
				return err
			}
		}
		var ns int64
		if nsID != nil {
			ns = *nsID
		}
		if err := s.Owned(ctx, tx, "namespace", ns, uid, "destination_deleted", 404); err != nil {
			return err
		}
		var photoID int64
		var photoNS = ns
		if kind == "live_preview" {
			var live bool
			if err := tx.QueryRow(ctx, `SELECT live FROM photo_media WHERE photo_id=$1`, originalID).Scan(&live); err != nil || !live {
				return Err("source_changed", 409)
			}
			dst, err := s.AssetPath(u.UUID, original.UUID, "live_preview")
			if err != nil {
				return err
			}
			if _, err = f.InstallFile(output, dst); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE photo_media SET live_status='ready', live_error=NULL WHERE photo_id=$1`, originalID); err != nil {
				return err
			}
			photoID = original.ID
			if original.NsID != nil {
				photoNS = *original.NsID
			}
		} else {
			stem := name
			if r := []rune(stem); len(r) > 230 {
				stem = string(r[:230])
			}
			if kind == "convert" {
				stem += " (converted)"
			}
			candidate, number := stem, 1
			for {
				var one int
				err := tx.QueryRow(ctx, `SELECT 1 FROM photo WHERE user_id=$1 AND ns_id=$2 AND photo_name=$3 AND format=$4 LIMIT 1`, uid, ns, candidate, target).Scan(&one)
				if err == pgx.ErrNoRows {
					break
				} else if err != nil {
					return err
				}
				candidate, number = fmt.Sprintf("%s (%d)", stem, number), number+1
			}
			in, err := os.Open(output)
			if err != nil {
				return err
			}
			p, err := s.InstallOne(ctx, tx, f, u, candidate+"."+target, in, ns, nil, prepared)
			in.Close()
			if err != nil {
				return err
			}
			if originalID != 0 {
				if _, err = tx.Exec(ctx, `UPDATE photo_extra n SET extra_photo_gps_lat=o.extra_photo_gps_lat, extra_photo_gps_lon=o.extra_photo_gps_lon,
					extra_photo_camera=o.extra_photo_camera, extra_photo_lens=o.extra_photo_lens, extra_photo_aperture=o.extra_photo_aperture,
					extra_photo_focal=o.extra_photo_focal, extra_photo_iso=o.extra_photo_iso, extra_photo_shutter_sec=o.extra_photo_shutter_sec,
					extra_photo_taken_date=o.extra_photo_taken_date, extra_photo_note=o.extra_photo_note
					FROM photo_extra o WHERE n.f_photo_id=$1 AND o.f_photo_id=$2`, p.ID, originalID); err != nil {
					return err
				}
			}
			photoID = p.ID
		}
		now := uploadsNow()
		if _, err = tx.Exec(ctx, `UPDATE upload_item SET photo_id=$2, ns_id=$3, size=$4, state='done', error=NULL, updated_at=$5 WHERE id=$1`,
			itemID, photoID, photoNS, fi.Size(), now); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE upload_batch SET updated_at=$2 WHERE id=$1`, batchID, now)
		return err
	})
}

// LivePath is MediaService.live_path.
func (s *Service) LivePath(ctx context.Context, uid, photoID int64) (string, error) {
	p, err := s.OwnedPhoto(ctx, s.Pool, photoID, uid, nil, 404)
	if err != nil {
		return "", err
	}
	var live bool
	var status *string
	if err := s.Pool.QueryRow(ctx, `SELECT live, live_status FROM photo_media WHERE photo_id=$1`, photoID).Scan(&live, &status); err != nil || !live || status == nil || *status != "ready" {
		return "", Err(nil, 404)
	}
	uu, err := s.userUUID(ctx, s.Pool, uid)
	if err != nil {
		return "", err
	}
	path, err := s.AssetPath(uu, p.UUID, "live_preview")
	if err != nil {
		return "", err
	}
	if !regular(path) {
		return "", Err(nil, 404)
	}
	return path, nil
}

// LiveArchive is MediaService.live_archive: the still and its movie in one zip.
func (s *Service) LiveArchive(ctx context.Context, uid, photoID int64) ([]byte, error) {
	p, err := s.OwnedPhoto(ctx, s.Pool, photoID, uid, nil, 404)
	if err != nil {
		return nil, err
	}
	var live bool
	if err := s.Pool.QueryRow(ctx, `SELECT live FROM photo_media WHERE photo_id=$1`, photoID).Scan(&live); err != nil || !live {
		return nil, Err(nil, 404)
	}
	uu, err := s.userUUID(ctx, s.Pool, uid)
	if err != nil {
		return nil, err
	}
	still, err := s.MediaPath(uu, p.UUID, p.Format, false)
	if err != nil {
		return nil, err
	}
	movie, err := s.AssetPath(uu, p.UUID, "live_original")
	if err != nil {
		return nil, err
	}
	name := ""
	if p.Name != nil {
		name = *p.Name
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	seen := map[string]bool{}
	for _, e := range [][2]string{{still, name + "." + p.Format}, {movie, name + ".mov"}} {
		fi, err := os.Stat(e[0])
		if err != nil || !fi.Mode().IsRegular() {
			return nil, Err(nil, 404)
		}
		cand := e[1]
		for i := 1; seen[cand]; i++ {
			ext := filepath.Ext(e[1])
			cand = fmt.Sprintf("%s (%d-%d)%s", strings.TrimSuffix(e[1], ext), photoID, i, ext)
		}
		seen[cand] = true
		h, _ := zip.FileInfoHeader(fi)
		h.Name, h.Method = cand, zip.Deflate
		w, err := zw.CreateHeader(h)
		if err != nil {
			return nil, err
		}
		in, err := os.Open(e[0])
		if err != nil {
			return nil, Err(nil, 404)
		}
		_, err = io.Copy(w, in)
		in.Close()
		if err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
