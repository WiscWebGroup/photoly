package photo

import (
	"context"
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLargeImagePreview(t *testing.T) {
	if _, err := exec.LookPath("vipsthumbnail"); err != nil {
		t.Skip("vipsthumbnail required")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "original.jpg")
	f, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	if err = jpeg.Encode(f, image.NewGray(image.Rect(0, 0, 8000, 7000)), nil); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err = ImageInfo(context.Background(), src); err == nil {
		t.Fatal("full decode guard was bypassed")
	}
	s := &Service{}
	p, err := s.prepareLargeImage(context.Background(), src, dir, &Prepared{Decodable: true})
	if err != nil {
		t.Fatal(err)
	}
	w, h, ok := imageDims(p.Preview)
	if !ok || w != 2560 || h > 2560 {
		t.Fatalf("preview: %dx%d", w, h)
	}
	w, h, ok = imageDims(p.Thumbnail)
	if !ok || w != 400 || h > 400 {
		t.Fatalf("thumbnail: %dx%d", w, h)
	}
	w, h, ok = imageDims(src)
	if !ok || w != 8000 || h != 7000 {
		t.Fatal("original modified")
	}
}
func TestFailureClassification(t *testing.T) {
	for _, code := range []string{"invalid_image", "image_too_many_pixels", "large_image_unsupported"} {
		if retryScanFailure(code) {
			t.Fatal(code)
		}
	}
	for _, code := range []string{"smb_connection_failed", "file_changed_during_scan"} {
		if !retryScanFailure(code) {
			t.Fatal(code)
		}
	}
	for _, part := range []string{"@Recycle", "#recycle", "$RECYCLE.BIN"} {
		if !recycleComponent(part) {
			t.Fatal(part)
		}
	}
	if recycleComponent(".photos") || recycleComponent("recycle holiday") {
		t.Fatal("ordinary directories excluded")
	}
}
