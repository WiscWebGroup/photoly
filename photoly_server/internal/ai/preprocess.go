package ai

// Go port of web/preprocess.js, so photos embedded on the server are prepared the way the
// browser (and Photoly's Pillow path) prepares them: EXIF orientation, white behind
// transparency, RGB, Pillow's antialiased bilinear resize to 224 x 224, then SigLIP's
// normalisation to [-1, 1] in channel-first order.

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/photo"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

const imageSize = 224

// ponytail: decodes the full photo in memory (~7 bytes per pixel at peak, so ~350 MB at the
// limit). Decode JPEGs at reduced size if small servers need less.
const maxPixels = 40_000_000

// errBadImage marks problems with the uploaded file rather than with the server.
var errBadImage = errors.New("cannot prepare image")

// Reasons an image is refused, for the API error codes.
var (
	errUnsupportedFormat = errors.New("unsupported image format")
	errTooManyPixels     = errors.New("too many pixels")
)

// prepareImage returns pixel_values for a [1, 3, 224, 224] tensor.
func prepareImage(data []byte) ([]float32, error) {
	rgb, w, h, err := decodeRGB(data)
	if err != nil {
		return nil, err
	}
	small := resizeBilinear(rgb, w, h, imageSize, imageSize)
	const plane = imageSize * imageSize
	pixels := make([]float32, 3*plane)
	for i := 0; i < plane; i++ {
		for c := 0; c < 3; c++ {
			pixels[c*plane+i] = float32((float64(small[i*3+c])/255 - 0.5) / 0.5)
		}
	}
	return pixels, nil
}

// decodeRGB decodes JPEG, PNG, GIF, WebP, BMP or TIFF into 8-bit RGB on white, upright per EXIF.
// Like Pillow and the browser path, it ignores ICC colour profiles.
func decodeRGB(data []byte) (rgb []uint8, w, h int, err error) {
	var img image.Image
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	switch {
	case errors.Is(err, image.ErrFormat):
		// HEIC/HEIF/ICO: decode through the media CLIs, like Pillow with pillow-heif.
		if img, err = photo.DecodeBytes(context.Background(), data); err != nil {
			return nil, 0, 0, fmt.Errorf("%w: %w: the server reads JPEG, PNG, GIF, WebP, BMP, TIFF, HEIC and ICO; embed this photo in the browser instead", errBadImage, errUnsupportedFormat)
		}
		cfg.Width, cfg.Height = img.Bounds().Dx(), img.Bounds().Dy()
	case err != nil:
		return nil, 0, 0, fmt.Errorf("%w: %v", errBadImage, err)
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width*cfg.Height > maxPixels {
		return nil, 0, 0, fmt.Errorf("%w: %w: %d x %d pixels; the limit is %d megapixels", errBadImage, errTooManyPixels, cfg.Width, cfg.Height, maxPixels/1_000_000)
	}
	if img == nil {
		if img, _, err = image.Decode(bytes.NewReader(data)); err != nil {
			return nil, 0, 0, fmt.Errorf("%w: %s: %v", errBadImage, format, err)
		}
	}
	b := img.Bounds()
	w, h = b.Dx(), b.Dy()
	canvas := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(canvas, canvas.Bounds(), img, b.Min, draw.Over)
	rgb = make([]uint8, w*h*3)
	for i, j := 0, 0; i < len(canvas.Pix); i, j = i+4, j+3 {
		copy(rgb[j:j+3], canvas.Pix[i:i+3])
	}
	rgb, w, h = orient(rgb, w, h, exifOrientation(data))
	return rgb, w, h, nil
}

// exifOrientation returns the EXIF orientation of a JPEG, or 1 when there is none.
// ponytail: JPEG only; PNG eXIf and WebP EXIF chunks are rare and are treated as upright.
func exifOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xFF {
			return 1
		}
		marker := data[i+1]
		switch {
		case marker == 0xFF: // fill byte
			i++
			continue
		case marker == 0x01 || marker >= 0xD0 && marker <= 0xD8: // no length field
			i += 2
			continue
		case marker == 0xDA || marker == 0xD9: // image data starts: no metadata after this
			return 1
		}
		n := int(binary.BigEndian.Uint16(data[i+2:]))
		if n < 2 || i+2+n > len(data) {
			return 1
		}
		if seg := data[i+4 : i+2+n]; marker == 0xE1 && bytes.HasPrefix(seg, []byte("Exif\x00\x00")) {
			return tiffOrientation(seg[6:])
		}
		i += 2 + n
	}
	return 1
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	ifd := int(order.Uint32(t[4:]))
	if ifd < 8 || ifd+2 > len(t) {
		return 1
	}
	for k := range int(order.Uint16(t[ifd:])) {
		e := ifd + 2 + 12*k
		if e+12 > len(t) {
			return 1
		}
		if order.Uint16(t[e:]) == 0x0112 {
			if v := int(order.Uint16(t[e+8:])); v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// orient applies an EXIF orientation (2-8) so the image is upright.
func orient(rgb []uint8, w, h, o int) ([]uint8, int, int) {
	if o < 2 || o > 8 {
		return rgb, w, h
	}
	ow, oh := w, h
	if o >= 5 {
		ow, oh = h, w
	}
	out := make([]uint8, len(rgb))
	for y := range oh {
		for x := range ow {
			sx, sy := x, y
			switch o {
			case 2: // mirrored
				sx = w - 1 - x
			case 3: // rotated 180
				sx, sy = w-1-x, h-1-y
			case 4: // flipped
				sy = h - 1 - y
			case 5: // transposed
				sx, sy = y, x
			case 6: // needs 90 degrees clockwise
				sx, sy = y, h-1-x
			case 7: // transversed
				sx, sy = w-1-y, h-1-x
			case 8: // needs 90 degrees anticlockwise
				sx, sy = w-1-y, x
			}
			copy(out[(y*ow+x)*3:][:3], rgb[(sy*w+sx)*3:][:3])
		}
	}
	return out, ow, oh
}

// Pillow's resampling (ImagingResample, precompute_coeffs) for the bilinear filter: when
// shrinking, the filter widens with the scale, so it antialiases. Coefficients are 22-bit
// fixed point, and each pass rounds to 8 bits, exactly as Pillow and preprocess.js do.
const precisionBits = 22

type taps struct {
	min     int
	weights []int64
}

func coefficients(inSize, outSize int) []taps {
	scale := float64(inSize) / float64(outSize)
	filterScale := math.Max(scale, 1)
	support := filterScale // bilinear support is 1
	out := make([]taps, outSize)
	for o := range out {
		center := (float64(o) + 0.5) * scale
		lo := max(int(math.Trunc(center-support+0.5)), 0)
		hi := min(int(math.Trunc(center+support+0.5)), inSize)
		weights := make([]float64, 0, max(hi-lo, 0))
		total := 0.0
		for x := lo; x < hi; x++ {
			w := math.Max(0, 1-math.Abs((float64(x)-center+0.5)/filterScale))
			weights = append(weights, w)
			total += w
		}
		t := taps{min: lo, weights: make([]int64, len(weights))}
		for k, w := range weights {
			if total != 0 {
				t.weights[k] = int64(math.Floor(w/total*(1<<precisionBits) + 0.5))
			}
		}
		out[o] = t
	}
	return out
}

// clip8 rounds half up and clamps, like Pillow's fixed-point sums.
func clip8(sum int64) uint8 {
	v := (sum + 1<<(precisionBits-1)) >> precisionBits
	return uint8(min(max(v, 0), 255))
}

// resizeBilinear resizes RGB: horizontal pass, then vertical.
func resizeBilinear(rgb []uint8, w, h, outW, outH int) []uint8 {
	xs, ys := coefficients(w, outW), coefficients(h, outH)
	mid := make([]uint8, outW*h*3)
	for y := range h {
		for x, t := range xs {
			var r, g, b int64
			for k, wt := range t.weights {
				i := (y*w + t.min + k) * 3
				r += int64(rgb[i]) * wt
				g += int64(rgb[i+1]) * wt
				b += int64(rgb[i+2]) * wt
			}
			o := (y*outW + x) * 3
			mid[o], mid[o+1], mid[o+2] = clip8(r), clip8(g), clip8(b)
		}
	}
	out := make([]uint8, outW*outH*3)
	for y, t := range ys {
		for x := range outW {
			var r, g, b int64
			for k, wt := range t.weights {
				i := ((t.min+k)*outW + x) * 3
				r += int64(mid[i]) * wt
				g += int64(mid[i+1]) * wt
				b += int64(mid[i+2]) * wt
			}
			o := (y*outW + x) * 3
			out[o], out[o+1], out[o+2] = clip8(r), clip8(g), clip8(b)
		}
	}
	return out
}
