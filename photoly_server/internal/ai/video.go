package ai

// Bounded local-file video sampling (app/ai/video.py): five frames at fixed fractions,
// decoded by ffmpeg; only decoded frames leave this file.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

var videoExt = map[string]bool{"mp4": true, "avi": true, "wmv": true, "mpeg": true, "m4v": true, "mov": true, "flv": true,
	"asf": true, "f4v": true, "rmvb": true, "vob": true, "rm": true}
var imageExt = map[string]bool{"jpg": true, "jpeg": true, "jfif": true, "png": true, "webp": true, "bmp": true, "tif": true,
	"tiff": true, "gif": true, "ico": true, "heic": true, "heif": true}

const (
	samplingVersion = 1
	maxFrames       = 5
	extractSeconds  = 30
	maxFrameEdge    = 1280
	maxImageBytes   = 20 * 1024 * 1024
	// Reject playlist/image demuxers: uploaded media must not reference other local files.
	demuxers = "mov,avi,asf,mpeg,mpegts,flv,rm"
)

var sampleFractions = [maxFrames]float64{0.05, 0.25, 0.5, 0.75, 0.95}

func isVideo(format string) bool { return videoExt[strings.ToLower(format)] }

func maxVideoBytes() int64 {
	if n, err := strconv.ParseInt(os.Getenv("MAX_CONTENT_LENGTH"), 10, 64); err == nil && n > 0 {
		return n
	}
	return 600 << 20
}

func runMedia(ctx context.Context, args ...string) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, fail("video_decode_timeout", 504)
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	var ee *exec.ExitError
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return nil, fail("video_decode_timeout", 504)
	case errors.As(err, &ee):
		return nil, fail("invalid_video", 415)
	case err != nil:
		return nil, fail("video_decoder_unavailable", 503)
	}
	return out.Bytes(), nil
}

type probeStream struct {
	Index       int            `json:"index"`
	CodecType   string         `json:"codec_type"`
	Width       int            `json:"width"`
	Height      int            `json:"height"`
	Duration    string         `json:"duration"`
	NbFrames    string         `json:"nb_frames"`
	Disposition map[string]int `json:"disposition"`
}

func extractFrames(ctx context.Context, path string) ([]Frame, error) {
	ctx, cancel := context.WithTimeout(ctx, extractSeconds*time.Second)
	defer cancel()
	raw, err := runMedia(ctx, "ffprobe", "-v", "error", "-protocol_whitelist", "file,pipe", "-format_whitelist", demuxers,
		"-show_entries", "stream=index,codec_type,width,height,duration,nb_frames,avg_frame_rate:stream_disposition=attached_pic:format=duration",
		"-of", "json", path)
	if err != nil {
		return nil, err
	}
	var meta struct {
		Streams []probeStream `json:"streams"`
		Format  struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if json.Unmarshal(raw, &meta) != nil {
		return nil, fail("invalid_video", 415)
	}
	si := slices.IndexFunc(meta.Streams, func(s probeStream) bool {
		return s.CodecType == "video" && s.Disposition["attached_pic"] == 0
	})
	if si < 0 {
		return nil, fail("video_stream_missing", 415)
	}
	st := meta.Streams[si]
	ds := st.Duration
	if ds == "" || ds == "N/A" {
		ds = meta.Format.Duration
	}
	duration, derr := strconv.ParseFloat(ds, 64)
	if derr != nil || math.IsNaN(duration) || math.IsInf(duration, 0) || duration <= 0 || st.Width <= 0 || st.Height <= 0 {
		return nil, fail("invalid_video", 415)
	}
	if st.Width*st.Height > 40_000_000 {
		return nil, fail("video_frame_too_large", 413)
	}
	frameCount, _ := strconv.Atoi(st.NbFrames)
	scale := fmt.Sprintf(`scale=w=min(%d\,iw):h=min(%d\,ih):force_original_aspect_ratio=decrease`, maxFrameEdge, maxFrameEdge)
	short := frameCount > 0 && frameCount <= maxFrames
	var times []float64
	if short || (frameCount == 0 && duration < 1) {
		rawT, err := runMedia(ctx, "ffprobe", "-v", "error", "-protocol_whitelist", "file,pipe", "-format_whitelist", demuxers,
			"-select_streams", strconv.Itoa(st.Index), "-read_intervals", "%+#32",
			"-show_entries", "frame=best_effort_timestamp_time", "-of", "json", path)
		if err != nil {
			return nil, err
		}
		var fr struct {
			Frames []struct {
				T string `json:"best_effort_timestamp_time"`
			} `json:"frames"`
		}
		if json.Unmarshal(rawT, &fr) != nil {
			return nil, fail("invalid_video", 415)
		}
		for _, f := range fr.Frames {
			v, err := strconv.ParseFloat(f.T, 64)
			if err != nil {
				return nil, fail("invalid_video", 415)
			}
			times = append(times, v)
		}
		short = short || (len(times) > 0 && len(times) <= maxFrames)
	}
	dir, err := os.MkdirTemp("", "photoly-frames-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	base := []string{"ffmpeg", "-v", "error", "-nostdin", "-threads", "2", "-protocol_whitelist", "file,pipe", "-format_whitelist", demuxers}
	out := []string{"-map", "0:" + strconv.Itoa(st.Index), "-an", "-sn", "-dn", "-vf", scale, "-threads", "1", "-y"}
	var frames []Frame
	add := func(file string, stamp float64) error {
		data, err := os.ReadFile(file)
		if err != nil || len(data) == 0 {
			return fail("invalid_video", 415)
		}
		frames = append(frames, Frame{TimestampMS: int64(math.Round(stamp * 1000)), Data: data})
		return nil
	}
	if short {
		args := slices.Concat(base, []string{"-i", path}, out, []string{"-vsync", "0", "-frames:v", strconv.Itoa(maxFrames), filepath.Join(dir, "%02d.png")})
		if _, err := runMedia(ctx, args...); err != nil {
			return nil, err
		}
		files, _ := filepath.Glob(filepath.Join(dir, "*.png"))
		slices.Sort(files)
		for i, f := range files {
			stamp := duration * float64(i) / math.Max(1, float64(len(files)))
			if i < len(times) {
				stamp = math.Max(0, times[i]-times[0])
			}
			if err := add(f, stamp); err != nil {
				return nil, err
			}
		}
	} else {
		for _, frac := range sampleFractions {
			target := duration * frac
			file := filepath.Join(dir, "frame.png")
			os.Remove(file)
			args := slices.Concat(base, []string{"-ss", fmt.Sprintf("%.6f", target), "-i", path}, out, []string{"-frames:v", "1", file})
			if _, err := runMedia(ctx, args...); err != nil {
				return nil, err
			}
			if _, err := os.Stat(file); err != nil {
				continue // a VFR tail can end early; never invent a duplicate frame
			}
			if err := add(file, target); err != nil {
				return nil, err
			}
		}
	}
	if len(frames) == 0 {
		return nil, fail("video_frame_missing", 415)
	}
	return frames, nil
}

// snapshotVideo copies the open file to a temp file (bounded, hashed) and samples frames.
func snapshotVideo(ctx context.Context, f io.Reader, expected string) (hash string, frames []Frame, err error) {
	tmp, err := os.CreateTemp("", "photoly-video-")
	if err != nil {
		return "", nil, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	h := sha256.New()
	limit := maxVideoBytes()
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(f, limit+1))
	if err != nil {
		return "", nil, err
	}
	if n > limit {
		return "", nil, fail("video_too_large", 413)
	}
	if n == 0 {
		return "", nil, fail("invalid_video", 415)
	}
	hash = hex.EncodeToString(h.Sum(nil))
	if expected != "" && hash != expected {
		return "", nil, fail("content_hash_mismatch", 409)
	}
	frames, err = extractFrames(ctx, tmp.Name())
	return hash, frames, err
}
