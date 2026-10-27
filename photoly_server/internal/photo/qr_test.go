package photo

import (
	"bytes"
	"image/png"
	"testing"
)

func TestQRPNG(t *testing.T) {
	b, err := QRPNG("https://example.com/photo/render/0a0b0c0d-0000-4000-8000-000000000000")
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil || img.Bounds().Dx() != 250 || img.Bounds().Dy() != 250 {
		t.Fatal(err, img.Bounds())
	}
}
