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

	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
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
	m, err := dbgen.New(s.Pool).GetPhotoMedia(ctx, photoID)
	has := err == nil
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	animated, live, status, lerr := m.Animated, m.Live, m.LiveStatus, m.LiveError
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
		q := dbgen.New(tx)
		switch owner, err := q.GetUploadBatchOwner(ctx, key); err {
		case nil:
			if owner != uid {
				return Err("upload_id_conflict", 409)
			}
			first, err := q.GetFirstUploadItemOfBatch(ctx, key)
			if err == pgx.ErrNoRows || first.Kind != kind || !sameRequest(first.Options, request) {
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
				if live, err := q.GetPhotoLive(ctx, p.ID); err != nil || !live {
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
		if err := q.InsertSealedUploadBatch(ctx, dbgen.InsertSealedUploadBatchParams{ID: key, UserID: uid, NsID: nsID, Name: name, Now: now}); err != nil {
			return err
		}
		_, kb, _ := ParseUUID(key)
		raw, _ := json.Marshal(options)
		return q.InsertMediaUploadItem(ctx, dbgen.InsertMediaUploadItemParams{ID: UUID5(kb, kind), BatchID: key, Path: name, Kind: kind, Options: raw, UpdatedAt: now})
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
	if v, _ := options["version"].(float64); int64(v) != p.ContentVersion {
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
	job, err := dbgen.New(s.Pool).GetUploadItemForProcess(ctx, itemID)
	if err != nil {
		return err
	}
	batchID, kind, rawOpts, uid := job.BatchID, job.Kind, job.Options, job.UserID
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
		live, _ := dbgen.New(s.Pool).GetPhotoLive(ctx, p.ID)
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
		q := dbgen.New(tx)
		st, err := q.GetUploadItemStateForUser(ctx, dbgen.GetUploadItemStateForUserParams{ID: itemID, UserID: uid})
		if err == pgx.ErrNoRows {
			return Err("upload_not_found", 404)
		} else if err != nil {
			return err
		}
		nsID := st.NsID
		if st.Cancelled || st.State != "processing" {
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
			if live, err := q.GetPhotoLive(ctx, originalID); err != nil || !live {
				return Err("source_changed", 409)
			}
			dst, err := s.AssetPath(u.UUID, original.UUID, "live_preview")
			if err != nil {
				return err
			}
			if _, err = f.InstallFile(output, dst); err != nil {
				return err
			}
			if err = q.MarkLivePreviewReady(ctx, originalID); err != nil {
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
				taken, err := q.PhotoNameTaken(ctx, dbgen.PhotoNameTakenParams{UserID: uid, NsID: ns, PhotoName: candidate, Format: target})
				if err != nil {
					return err
				}
				if !taken {
					break
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
				if err = q.CopyPhotoExtra(ctx, dbgen.CopyPhotoExtraParams{ToPhotoID: p.ID, FromPhotoID: originalID}); err != nil {
					return err
				}
			}
			photoID = p.ID
		}
		now := uploadsNow()
		if err = q.MarkMediaUploadItemDone(ctx, dbgen.MarkMediaUploadItemDoneParams{ID: itemID, PhotoID: photoID, NsID: photoNS, Size: fi.Size(), UpdatedAt: now}); err != nil {
			return err
		}
		return q.TouchUploadBatch(ctx, dbgen.TouchUploadBatchParams{ID: batchID, UpdatedAt: now})
	})
}

// LivePath is MediaService.live_path.
func (s *Service) LivePath(ctx context.Context, uid, photoID int64) (string, error) {
	p, err := s.OwnedPhoto(ctx, s.Pool, photoID, uid, nil, 404)
	if err != nil {
		return "", err
	}
	if m, err := dbgen.New(s.Pool).GetPhotoLiveStatus(ctx, photoID); err != nil || !m.Live || m.LiveStatus == nil || *m.LiveStatus != "ready" {
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
	if live, err := dbgen.New(s.Pool).GetPhotoLive(ctx, photoID); err != nil || !live {
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
