package photo

import (
	"context"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestVideoThumbnailFrames(t *testing.T) {
	if _, e := exec.LookPath("ffmpeg"); e != nil {
		t.Skip("ffmpeg required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, size := range []string{"800x400", "200x800"} {
		t.Run(size, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "short.mp4")
			dst := filepath.Join(dir, "thumbnail.jpg")
			if out, e := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c=red:s="+size+":d=0.2", "-pix_fmt", "yuv420p", src).CombinedOutput(); e != nil {
				t.Fatalf("fixture: %v %s", e, out)
			}
			if e := MakeVideoThumbnail(ctx, src, dst); e != nil {
				t.Fatal(e)
			}
			f, e := os.Open(dst)
			if e != nil {
				t.Fatal(e)
			}
			defer f.Close()
			im, e := jpeg.Decode(f)
			if e != nil {
				t.Fatal(e)
			}
			b := im.Bounds()
			if max(b.Dx(), b.Dy()) != 400 {
				t.Fatal(b)
			}
			r, g, _, _ := im.At(b.Dx()/2, b.Dy()/2).RGBA()
			if r < 40000 || g > 10000 {
				t.Fatal("wrong frame")
			}
		})
	}
	t.Run("display rotation", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "input.mp4")
		rot := filepath.Join(dir, "rotated.mp4")
		dst := filepath.Join(dir, "thumbnail.jpg")
		if e := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=640x360:d=0.2", "-pix_fmt", "yuv420p", src).Run(); e != nil {
			t.Fatal(e)
		}
		if e := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-i", src, "-c", "copy", "-metadata:s:v:0", "rotate=90", rot).Run(); e != nil {
			t.Fatal(e)
		}
		if e := MakeVideoThumbnail(ctx, rot, dst); e != nil {
			t.Fatal(e)
		}
		f, e := os.Open(dst)
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		cfg, e := jpeg.DecodeConfig(f)
		if e != nil || cfg.Width != 225 || cfg.Height != 400 {
			t.Fatal(cfg, e)
		}
	})

	src := filepath.Join(t.TempDir(), "broken.mp4")
	os.WriteFile(src, []byte("bad movie"), 0600)
	if MakeVideoThumbnail(ctx, src, filepath.Join(t.TempDir(), "bad.jpg")) == nil {
		t.Fatal("accepted corrupt video")
	}
}

func TestAnimatedWebPStaticThumbnail(t *testing.T) {
	if _, e := exec.LookPath("vips"); e != nil {
		t.Skip("libvips required")
	}
	if _, e := exec.LookPath("ffmpeg"); e != nil {
		t.Skip("ffmpeg required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	src := filepath.Join(t.TempDir(), "animated.webp")
	dst := filepath.Join(t.TempDir(), "thumbnail.jpg")
	if out, e := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=320x180:rate=5:duration=1", "-loop", "0", src).CombinedOutput(); e != nil {
		t.Fatalf("fixture: %v %s", e, out)
	}
	animated, e := ImageInfo(ctx, src)
	if e != nil || !animated {
		t.Fatal(animated, e)
	}
	if e = MakeThumbnail(ctx, src, dst, 15*time.Second); e != nil {
		t.Fatal(e)
	}
	f, e := os.Open(dst)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	im, e := jpeg.Decode(f)
	if e != nil || im.Bounds().Dx() != 320 || im.Bounds().Dy() != 180 {
		t.Fatal(im, e)
	}
}
