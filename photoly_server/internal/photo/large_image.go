package photo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const largeImageMaxPixels = 250_000_000

var largeImageSlot = make(chan struct{}, 1)

// Only preview generation may exceed the full-resolution decoder's pixel guard.
// libvips runs out of process; never pass the original to Go's full decoder here.
func (s *Service) prepareLargeImage(ctx context.Context, source, work string, p *Prepared) (*Prepared, error) {
	w, h, ok := imageDims(source)
	if !ok {
		w, h, ok = ffprobeDims(ctx, source)
	}
	if !ok {
		mw, wok := num(p.Meta, "ImageWidth")
		mh, hok := num(p.Meta, "ImageHeight")
		w, h, ok = int(mw), int(mh), wok && hok
	}
	if !ok || w <= 0 || h <= 0 {
		return nil, Err("invalid_image", 415)
	}
	if int64(w) > largeImageMaxPixels/int64(h) {
		return nil, Err("image_too_many_pixels", 413)
	}
	switch sniff(source) {
	case "JPEG", "PNG", "TIFF", "WEBP", "HEIF":
	default:
		return nil, Err("large_image_unsupported", 415)
	}
	if frameCount(source) > 1 {
		return nil, Err("large_image_unsupported", 415)
	}
	select {
	case largeImageSlot <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-largeImageSlot }()
	tool, err := exec.LookPath("vipsthumbnail")
	if err != nil {
		return nil, Err("large_image_processor_unavailable", 503)
	}
	preview := filepath.Join(work, "preview.jpg")
	args := []string{source, "--size=2560x2560>", "--output=" + preview + "[Q=85]", "--vips-concurrency=2", "--vips-cache-max-memory=67108864", "--vips-cache-max=0"}
	if runtime.GOOS == "linux" {
		limiter, e := exec.LookPath("prlimit")
		if e != nil {
			return nil, Err("large_image_processor_unavailable", 503)
		}
		args = append([]string{"--as=2147483648", "--cpu=120", "--fsize=536870912", "--", tool}, args...)
		tool = limiter
	}
	c, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(c, tool, args...)
	cmd.Env = append(os.Environ(), "VIPS_DISC_THRESHOLD=67108864", "VIPS_CONCURRENCY=2")
	if err = cmd.Run(); err != nil {
		os.Remove(preview)
		return nil, Err("large_image_processing_failed", 422)
	}
	pw, ph, ok := imageDims(preview)
	if !ok || pw > 2560 || ph > 2560 {
		os.Remove(preview)
		return nil, Err("large_image_processing_failed", 422)
	}
	thumb := filepath.Join(work, "thumbnail.jpg")
	if err = MakeThumbnail(c, preview, thumb, 30*time.Second); err != nil {
		return nil, err
	}
	p.Thumbnail, p.Preview = thumb, preview
	return p, nil
}
