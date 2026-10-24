package user

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	xdraw "golang.org/x/image/draw"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	_ "image/gif"
	_ "image/png"
)

const (
	maxAvatarBytes  = 20 * 1024 * 1024
	maxAvatarPixels = 40_000_000
	avatarSize      = 512
)

// DecodeFallback decodes formats the stdlib cannot (HEIC/HEIF, ICO).
var DecodeFallback func(ctx context.Context, data []byte) (image.Image, error)

// avatarJPEG is services.avatar.avatar_jpeg: EXIF-upright, square-cropped 512px, white-matted JPEG q92.
func avatarJPEG(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxAvatarBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxAvatarBytes {
		return nil, legacy.Bad()
	}
	var src image.Image
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		if cfg.Width*cfg.Height > maxAvatarPixels {
			return nil, legacy.Bad()
		}
		src, _, err = image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, legacy.Bad()
		}
	} else if DecodeFallback != nil { // HEIC/ICO, set by photo.Register (import cycle otherwise)
		if src, err = DecodeFallback(context.Background(), data); err != nil || src.Bounds().Dx()*src.Bounds().Dy() > maxAvatarPixels {
			return nil, legacy.Bad()
		}
	} else {
		return nil, legacy.Bad()
	}
	src = orient(src, exifOrientation(data))
	b := src.Bounds()
	side := min(b.Dx(), b.Dy())
	x0, y0 := b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2
	sq := image.NewNRGBA(image.Rect(0, 0, avatarSize, avatarSize))
	xdraw.CatmullRom.Scale(sq, sq.Bounds(), src, image.Rect(x0, y0, x0+side, y0+side), xdraw.Src, nil)
	bg := image.NewRGBA(sq.Bounds())
	draw.Draw(bg, bg.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(bg, bg.Bounds(), sq, image.Point{}, draw.Over)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, bg, &jpeg.Options{Quality: 92}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func exifOrientation(d []byte) int {
	if len(d) < 4 || d[0] != 0xFF || d[1] != 0xD8 {
		return 1
	}
	for i := 2; i+4 <= len(d) && d[i] == 0xFF; {
		m, n := d[i+1], int(binary.BigEndian.Uint16(d[i+2:]))
		if m == 0xDA || n < 2 {
			break
		}
		seg := d[i+4 : min(len(d), i+2+n)]
		if m == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			t := seg[6:]
			var bo binary.ByteOrder = binary.BigEndian
			if t[0] == 'I' {
				bo = binary.LittleEndian
			}
			off := int(bo.Uint32(t[4:]))
			if off+2 > len(t) {
				return 1
			}
			for e, c := off+2, int(bo.Uint16(t[off:])); c > 0 && e+12 <= len(t); e, c = e+12, c-1 {
				if bo.Uint16(t[e:]) == 0x0112 {
					if v := int(bo.Uint16(t[e+8:])); v >= 1 && v <= 8 {
						return v
					}
				}
			}
			return 1
		}
		i += 2 + n
	}
	return 1
}

func orient(src image.Image, o int) image.Image {
	if o <= 1 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			var sx, sy int
			switch o {
			case 2:
				sx, sy = w-1-x, y
			case 3:
				sx, sy = w-1-x, h-1-y
			case 4:
				sx, sy = x, h-1-y
			case 5:
				sx, sy = y, x
			case 6:
				sx, sy = y, h-1-x
			case 7:
				sx, sy = w-1-y, h-1-x
			case 8:
				sx, sy = w-1-y, x
			}
			dst.Set(x, y, src.At(b.Min.X+sx, b.Min.Y+sy))
		}
	}
	return dst
}
