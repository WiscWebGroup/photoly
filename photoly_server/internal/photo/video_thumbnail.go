package photo

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/jackc/pgx/v5"
)

// MakeVideoThumbnail extracts a display-oriented frame without decoding the whole movie.
func MakeVideoThumbnail(ctx context.Context, src, dst string) error {
	var at float64
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-protocol_whitelist", "file,pipe", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", src).Output()
	if err == nil {
		if d, e := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); e == nil && d > 0 && !math.IsInf(d, 0) {
			at = math.Min(1, d*0.1)
		}
	}
	extract := func(second float64) error {
		args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y", "-threads", "2", "-protocol_whitelist", "file,pipe"}
		if second > 0 {
			args = append(args, "-ss", strconv.FormatFloat(second, 'f', 3, 64))
		}
		args = append(args, "-i", src, "-map", "0:v:0", "-frames:v", "1", "-an", "-vf", "scale=w='min(400,iw)':h='min(400,ih)':force_original_aspect_ratio=decrease,setsar=1", "-q:v", "3", dst)
		if e := exec.CommandContext(ctx, "ffmpeg", args...).Run(); e != nil {
			return e
		}
		if fileSize(dst) == 0 {
			return fmt.Errorf("video has no decodable frame")
		}
		return nil
	}
	if err = extract(at); err != nil && at > 0 && ctx.Err() == nil {
		err = extract(0)
	}
	return err
}

// VideoThumbnailWorker is independent of AI. Claims expire; installation is version checked.
func VideoThumbnailWorker(ctx context.Context, a *app.App) {
	s := For(a)
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		for i := 0; i < 4; i++ {
			if !s.videoThumbnailNext(ctx) {
				break
			}
		}
	}
}
func (s *Service) videoThumbnailNext(ctx context.Context) bool {
	token := NewUUID()
	job, err := dbgen.New(s.Pool).ClaimVideoThumbnail(ctx, token)
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("video thumbnail claim: %v", err)
		}
		return false
	}
	work, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	dir, err := os.MkdirTemp("", "photoly-video-thumb-")
	if err == nil {
		defer os.RemoveAll(dir)
	}
	unavailable := false
	if err == nil {
		var path string
		path, err = s.PhotoPath(work, job.UserID, job.PhotoID, false)
		unavailable = err != nil
		if err == nil {
			err = MakeVideoThumbnail(work, path, filepath.Join(dir, "thumbnail.jpg"))
		}
	}
	if err == nil {
		err = s.Atomic(work, job.UserID, func(tx pgx.Tx, u UserRow, f *Files) error {
			var uuid string
			var version int64
			// Lock in the same order as content replacement and verify the lease after locking the photo.
			e := tx.QueryRow(work, `SELECT photo_uuid,content_version FROM photo WHERE photo_id=$1 AND user_id=$2 FOR UPDATE`, job.PhotoID, job.UserID).Scan(&uuid, &version)
			if e != nil {
				return e
			}
			if version != job.ContentVersion {
				return nil
			}
			var valid bool
			e = tx.QueryRow(work, `SELECT EXISTS(SELECT 1 FROM video_thumbnail_job j JOIN photo p USING(photo_id) JOIN storage_source s ON s.id=p.source_id WHERE j.photo_id=$1 AND j.lease_token=$2 AND j.next_attempt>now() AND s.enabled)`, job.PhotoID, token).Scan(&valid)
			if e != nil {
				return e
			}
			if !valid {
				return nil
			}
			if _, e = tx.Exec(work, `UPDATE media_asset SET filename='thumbnail.jpg',content_version=$2 WHERE photo_id=$1 AND role='thumbnail'`, job.PhotoID, version); e != nil {
				return e
			}
			dest, e := s.AssetPath(work, tx, u.UUID, uuid, "thumbnail")
			if e != nil {
				return e
			}
			if _, e = f.InstallFile(filepath.Join(dir, "thumbnail.jpg"), dest); e != nil {
				return e
			}
			_, e = tx.Exec(work, `UPDATE video_thumbnail_job SET state='ready',lease_token=NULL,error=NULL WHERE photo_id=$1 AND lease_token=$2`, job.PhotoID, token)
			return e
		})
	}
	if err != nil && ctx.Err() == nil {
		// Storage failures are deferred rather than consuming the decoder retry budget.
		code := "thumbnail_failed"
		if unavailable {
			code = "media_unavailable"
		}
		_, e := s.Pool.Exec(ctx, `UPDATE video_thumbnail_job SET state=CASE WHEN NOT $3 AND attempts>=4 THEN 'failed' ELSE 'pending' END,attempts=attempts+CASE WHEN $3 THEN 0 ELSE 1 END,error=$4,lease_token=NULL,next_attempt=now()+make_interval(secs=>CASE WHEN $3 THEN 60 ELSE LEAST(3600,30*power(2,LEAST(attempts,7)))::int END) WHERE photo_id=$1 AND lease_token=$2`, job.PhotoID, token, unavailable, code)
		if e != nil {
			log.Printf("video thumbnail retry: %v", e)
		}
	}
	return true
}
