package photo

import (
	"bytes"
	"cmp"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	_ "golang.org/x/image/bmp"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// DecodableFormats is infrastructure.media.DECODABLE.
var DecodableFormats = map[string]bool{"jpg": true, "jpeg": true, "jfif": true, "png": true, "gif": true, "webp": true, "bmp": true,
	"tif": true, "tiff": true, "ico": true, "heic": true, "heif": true}

// Targets are the conversion targets in API order (list(TARGETS)).
var Targets = []string{"jpeg", "png", "gif"}

const (
	maxPixels          = 50_000_000
	maxAnimationPixels = 200_000_000
)

// sniff identifies the container the way PIL reports image.format ("" when unknown).
func sniff(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	b := make([]byte, 32)
	n, _ := io.ReadFull(f, b)
	b = b[:n]
	switch {
	case bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		return "JPEG"
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return "PNG"
	case bytes.HasPrefix(b, []byte("GIF87a")) || bytes.HasPrefix(b, []byte("GIF89a")):
		return "GIF"
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "WEBP"
	case bytes.HasPrefix(b, []byte("BM")):
		return "BMP"
	case bytes.HasPrefix(b, []byte("II*\x00")) || bytes.HasPrefix(b, []byte("MM\x00*")):
		return "TIFF"
	case bytes.HasPrefix(b, []byte{0, 0, 1, 0}):
		return "ICO"
	case len(b) >= 12 && string(b[4:8]) == "ftyp":
		switch string(b[8:12]) {
		case "heic", "heix", "hevc", "hevx", "heim", "heis", "mif1", "msf1", "heif":
			return "HEIF"
		}
	}
	return ""
}

var pilMIME = map[string]string{"JPEG": "image/jpeg", "PNG": "image/png", "GIF": "image/gif", "WEBP": "image/webp", "BMP": "image/bmp",
	"TIFF": "image/tiff", "ICO": "image/x-icon", "HEIF": "image/heif"}

var extMIME = map[string]string{".mp4": "video/mp4", ".mov": "video/quicktime", ".avi": "video/x-msvideo", ".mpeg": "video/mpeg",
	".m4v": "video/x-m4v", ".wmv": "video/x-ms-wmv", ".flv": "video/x-flv", ".asf": "video/x-ms-asf", ".f4v": "video/mp4",
	".svg": "image/svg+xml", ".psd": "image/vnd.adobe.photoshop", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
	".gif": "image/gif", ".webp": "image/webp", ".bmp": "image/bmp", ".tif": "image/tiff", ".tiff": "image/tiff", ".ico": "image/x-icon",
	".heic": "image/heic", ".heif": "image/heif", ".vob": "video/mpeg", ".rm": "application/vnd.rn-realmedia", ".rmvb": "application/vnd.rn-realmedia-vbr"}

// MIME is infrastructure.media.mime: the real image type, else a guess from the extension.
func MIME(path string) string {
	if f := sniff(path); f != "" {
		return pilMIME[f]
	}
	ext := strings.ToLower(filepath.Ext(path))
	if m, ok := extMIME[ext]; ok {
		return m
	}
	if m := mime.TypeByExtension(ext); m != "" {
		return m
	}
	return "application/octet-stream"
}

func imageDims(path string) (int, int, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}

func ffprobeDims(ctx context.Context, path string) (int, int, bool) {
	m := ffprobeMeta(ctx, path)
	w, ok1 := num(m, "ImageWidth")
	h, ok2 := num(m, "ImageHeight")
	return int(w), int(h), ok1 && ok2 && w > 0 && h > 0
}

// ImageInfo is infrastructure.media.image_info: size guard + animation flag.
func ImageInfo(ctx context.Context, path string) (animated bool, err error) {
	w, h, ok := imageDims(path)
	if !ok {
		w, h, ok = ffprobeDims(ctx, path)
	}
	if !ok && sniff(path) == "HEIF" {
		// ffprobe builds without HEIF support can still use libheif's header reader.
		probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(probeCtx, "heif-info", path).Output(); err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if n, _ := fmt.Sscanf(line, "image: %dx%d", &w, &h); n == 2 && w > 0 && h > 0 {
					ok = true
					break
				}
			}
		}
	}
	if !ok {
		return false, Err("invalid_image", 415)
	}
	if w*h > maxPixels {
		return false, Err("image_too_many_pixels", 413)
	}
	return frameCount(path) > 1, nil
}

// frameCount reports the number of frames for animation-capable formats.
func frameCount(path string) int {
	switch sniff(path) {
	case "GIF":
		return gifFrames(path)
	case "WEBP":
		f, err := os.Open(path)
		if err != nil {
			return 1
		}
		defer f.Close()
		hdr := make([]byte, 21)
		if _, err := io.ReadFull(f, hdr); err == nil && string(hdr[12:16]) == "VP8X" && hdr[20]&0x02 != 0 {
			return 2 // animated; exact count is only needed for GIF targets
		}
	case "PNG":
		f, err := os.Open(path)
		if err != nil {
			return 1
		}
		defer f.Close()
		_, _ = f.Seek(8, io.SeekStart)
		hdr := make([]byte, 8)
		for {
			if _, err := io.ReadFull(f, hdr); err != nil {
				break
			}
			l := int64(binary.BigEndian.Uint32(hdr[:4]))
			switch string(hdr[4:]) {
			case "acTL":
				return 2
			case "IDAT", "IEND":
				return 1
			}
			if _, err := f.Seek(l+4, io.SeekCurrent); err != nil {
				break
			}
		}
	case "TIFF":
		return tiffPages(path)
	}
	return 1
}

func gifFrames(path string) int {
	b, err := os.ReadFile(path)
	if err != nil || len(b) < 13 {
		return 1
	}
	p := 13
	if b[10]&0x80 != 0 {
		p += 3 << ((b[10] & 7) + 1)
	}
	skip := func() {
		for p < len(b) {
			n := int(b[p])
			p++
			if n == 0 {
				return
			}
			p += n
		}
	}
	frames := 0
	for p < len(b) {
		switch b[p] {
		case 0x21:
			p += 2
			skip()
		case 0x2C:
			frames++
			if p+10 > len(b) {
				return frames
			}
			flags := b[p+9]
			p += 10
			if flags&0x80 != 0 {
				p += 3 << ((flags & 7) + 1)
			}
			p++
			skip()
		default:
			return max(frames, 1)
		}
	}
	return max(frames, 1)
}

func tiffPages(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 1
	}
	t := newTIFF(b)
	if t == nil {
		return 1
	}
	off, pages := t.bo.Uint32(b[4:]), 0
	for off != 0 && pages < 10000 && uint64(off)+2 <= uint64(len(b)) {
		pages++
		n := uint64(t.bo.Uint16(b[off:]))
		next := uint64(off) + 2 + n*12
		if next+4 > uint64(len(b)) {
			break
		}
		off = t.bo.Uint32(b[next:])
	}
	return max(pages, 1)
}

// ---- decoding ----

func ffmpegStill(ctx context.Context, src string, timeout time.Duration) (image.Image, error) {
	dir, err := os.MkdirTemp("", "photoly-still-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "frame.png")
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// HEIC/HEIF: libheif's heif-convert is more reliable than ffmpeg's partial HEVC-still support.
	if sniff(src) == "HEIF" {
		if err := exec.CommandContext(ctx, "heif-convert", src, out).Run(); err == nil {
			if f, err := os.Open(out); err == nil {
				defer f.Close()
				return png.Decode(f)
			}
		}
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-hide_banner", "-loglevel", "error", "-y", "-protocol_whitelist", "file,pipe",
		"-i", src, "-frames:v", "1", "-an", out)
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	f, err := os.Open(out)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

// decoded is a still image plus whether EXIF orientation is already applied to its pixels.
type decoded struct {
	img      image.Image
	oriented bool
	format   string
}

func decodeImage(ctx context.Context, path string, timeout time.Duration) (*decoded, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err == nil {
		return &decoded{img: img, format: sniff(path)}, nil
	}
	img, ferr := ffmpegStill(ctx, path, timeout)
	if ferr != nil {
		return nil, err
	}
	return &decoded{img: img, oriented: true, format: sniff(path)}, nil
}

func orientationOf(path string) int {
	if info, ok := inspectFile(path); ok {
		if o, ok := info.Meta["Orientation"].(int64); ok && o >= 1 && o <= 8 {
			return int(o)
		}
	}
	return 1
}

// transpose is ImageOps.exif_transpose for orientations 1-8.
func transpose(src image.Image, o int) image.Image {
	if o <= 1 || o > 8 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	nw, nh := w, h
	if o >= 5 {
		nw, nh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch o {
			case 2:
				dx, dy = w-1-x, y
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dx, dy = x, h-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// resizeFit scales down to fit within max x max, never enlarging (Image.thumbnail).
func resizeFit(src image.Image, maxSide int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxSide && h <= maxSide {
		return src
	}
	nw, nh := maxSide, maxSide
	if w >= h {
		nh = max(1, int(math.Round(float64(h)*float64(maxSide)/float64(w))))
	} else {
		nw = max(1, int(math.Round(float64(w)*float64(maxSide)/float64(h))))
	}
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
	return dst
}

// toRGB is convert('RGB'): drops alpha without compositing.
func toRGB(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	if n, ok := src.(*image.NRGBA); ok {
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				i, o := n.PixOffset(b.Min.X+x, b.Min.Y+y), dst.PixOffset(x, y)
				dst.Pix[o], dst.Pix[o+1], dst.Pix[o+2], dst.Pix[o+3] = n.Pix[i], n.Pix[i+1], n.Pix[i+2], 255
			}
		}
		return dst
	}
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := color.NRGBAModel.Convert(src.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			dst.SetRGBA(x, y, color.RGBA{c.R, c.G, c.B, 255})
		}
	}
	return dst
}

// flattenWhite is RGBA alpha-composited on white.
func flattenWhite(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Over)
	return dst
}

// MakeThumbnail writes the 400px JPEG thumbnail used by uploads (image.thumbnail((400,400))).
func MakeThumbnail(ctx context.Context, src, dst string, timeout time.Duration) error {
	d, err := decodeImage(ctx, src, timeout)
	if err != nil {
		return Err("invalid_image", 415)
	}
	img := resizeFit(d.img, 400)
	if !d.oriented {
		img = transpose(img, orientationOf(src))
	}
	return writeJPEG(dst, toRGB(img), 75, nil, nil)
}

func writeJPEG(dst string, img image.Image, quality int, exif, icc []byte) error {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return err
	}
	out := spliceJPEG(buf.Bytes(), exif, icc)
	return os.WriteFile(dst, out, 0o644)
}

// spliceJPEG inserts EXIF (APP1) and ICC (APP2) segments after SOI.
func spliceJPEG(b, exif, icc []byte) []byte {
	if len(exif) == 0 && len(icc) == 0 {
		return b
	}
	var seg bytes.Buffer
	if len(exif) > 0 && len(exif)+8 <= 0xFFFF {
		seg.Write([]byte{0xFF, 0xE1})
		_ = binary.Write(&seg, binary.BigEndian, uint16(len(exif)+8))
		seg.WriteString("Exif\x00\x00")
		seg.Write(exif)
	}
	const chunk = 65519 - 14
	if len(icc) > 0 {
		total := (len(icc) + chunk - 1) / chunk
		for i := 0; i < total; i++ {
			part := icc[i*chunk : min(len(icc), (i+1)*chunk)]
			seg.Write([]byte{0xFF, 0xE2})
			_ = binary.Write(&seg, binary.BigEndian, uint16(len(part)+16))
			seg.WriteString("ICC_PROFILE\x00")
			seg.WriteByte(byte(i + 1))
			seg.WriteByte(byte(total))
			seg.Write(part)
		}
	}
	return append(append(append([]byte{}, b[:2]...), seg.Bytes()...), b[2:]...)
}

// withOrientationOne patches IFD0 Orientation to 1 in a copy of a TIFF block.
func withOrientationOne(raw []byte) []byte {
	t := newTIFF(raw)
	if t == nil {
		return raw
	}
	out := append([]byte{}, raw...)
	off := uint64(t.bo.Uint32(raw[4:]))
	if off+2 > uint64(len(raw)) {
		return out
	}
	n := uint64(t.bo.Uint16(raw[off:]))
	for i := uint64(0); i < n; i++ {
		p := off + 2 + i*12
		if p+12 > uint64(len(raw)) {
			break
		}
		if t.bo.Uint16(raw[p:]) == 0x0112 {
			t.bo.PutUint16(out[p+8:], 1)
		}
	}
	return out
}

// ---- conversion (encode_image) ----

// EncodeImage converts src into jpeg/png/gif at dst. Originals are never rewritten.
func EncodeImage(ctx context.Context, src, dst, target string, timeout time.Duration) error {
	if target != "jpeg" && target != "png" && target != "gif" {
		return Err("unsupported_target_format", 400)
	}
	if _, err := ImageInfo(ctx, src); err != nil {
		return err
	}
	exifRaw, icc := RawEXIF(src)
	format := sniff(src)
	var frames []image.Image
	var durations []int // milliseconds
	loop := -1
	animated := target == "gif" && frameCount(src) > 1
	if animated && format == "GIF" {
		f, err := os.Open(src)
		if err != nil {
			return Err("conversion_failed", 400)
		}
		g, err := gif.DecodeAll(f)
		f.Close()
		if err != nil {
			return Err("conversion_failed", 400)
		}
		n := len(g.Image)
		b := image.Rect(0, 0, g.Config.Width, g.Config.Height)
		if n > 1000 || n*b.Dx()*b.Dy() > maxAnimationPixels {
			return Err("animation_too_large", 413)
		}
		frames = composeGIF(g, b)
		for _, d := range g.Delay {
			durations = append(durations, d*10)
		}
		loop = g.LoopCount
		for i := range frames {
			frames[i] = transposeIf(frames[i], orientationOf(src), false)
		}
	} else {
		d, err := decodeImage(ctx, src, timeout)
		if err != nil {
			return Err("conversion_failed", 400)
		}
		img := transposeIf(d.img, orientationOf(src), d.oriented)
		frames, durations = []image.Image{img}, []int{100}
		if format == "GIF" {
			if g, err := gifFirst(src); err == nil {
				durations[0] = g
			}
		}
	}
	var out []byte
	switch target {
	case "jpeg":
		var buf bytes.Buffer
		img := flattenWhite(frames[0])
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
			return Err("conversion_failed", 400)
		}
		var ex []byte
		if len(exifRaw) > 0 {
			ex = withOrientationOne(exifRaw)
		}
		out = spliceJPEG(buf.Bytes(), ex, icc)
	case "png":
		rgba := image.NewNRGBA(image.Rect(0, 0, frames[0].Bounds().Dx(), frames[0].Bounds().Dy()))
		draw.Draw(rgba, rgba.Bounds(), frames[0], frames[0].Bounds().Min, draw.Src)
		var buf bytes.Buffer
		if err := png.Encode(&buf, rgba); err != nil {
			return Err("conversion_failed", 400)
		}
		out = addPNGMeta(buf.Bytes(), withOrientationOne(exifRaw), icc)
	case "gif":
		g := &gif.GIF{LoopCount: loop}
		for i, f := range frames {
			g.Image = append(g.Image, quantize(f))
			g.Delay = append(g.Delay, max(0, durations[min(i, len(durations)-1)]/10))
			g.Disposal = append(g.Disposal, gif.DisposalBackground)
		}
		if !animated {
			g.LoopCount = -1
		}
		var buf bytes.Buffer
		if err := gif.EncodeAll(&buf, g); err != nil {
			return Err("conversion_failed", 400)
		}
		out = buf.Bytes()
	}
	if err := os.WriteFile(dst, out, 0o644); err != nil {
		return Err("conversion_failed", 400)
	}
	return nil
}

func transposeIf(img image.Image, o int, already bool) image.Image {
	if already {
		return img
	}
	return transpose(img, o)
}

func gifFirst(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil || len(g.Delay) == 0 {
		return 0, err
	}
	return g.Delay[0] * 10, nil
}

// composeGIF renders every frame onto a full canvas honoring disposal methods.
func composeGIF(g *gif.GIF, b image.Rectangle) []image.Image {
	canvas := image.NewNRGBA(b)
	var out []image.Image
	for i, f := range g.Image {
		var prev *image.NRGBA
		if i < len(g.Disposal) && g.Disposal[i] == gif.DisposalPrevious {
			prev = image.NewNRGBA(b)
			copy(prev.Pix, canvas.Pix)
		}
		draw.Draw(canvas, f.Bounds(), f, f.Bounds().Min, draw.Over)
		snap := image.NewNRGBA(b)
		copy(snap.Pix, canvas.Pix)
		out = append(out, snap)
		if i < len(g.Disposal) {
			switch g.Disposal[i] {
			case gif.DisposalBackground:
				draw.Draw(canvas, f.Bounds(), image.Transparent, image.Point{}, draw.Src)
			case gif.DisposalPrevious:
				if prev != nil {
					copy(canvas.Pix, prev.Pix)
				}
			}
		}
	}
	return out
}

// quantize builds an adaptive 255-colour palette (+ transparent index 0) from the frame.
func quantize(src image.Image) *image.Paletted {
	b := src.Bounds()
	type bin struct{ r, g, b, n uint64 }
	hist := map[uint16]*bin{}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			if c.A < 128 {
				continue
			}
			key := uint16(c.R>>3)<<10 | uint16(c.G>>3)<<5 | uint16(c.B>>3)
			e := hist[key]
			if e == nil {
				e = &bin{}
				hist[key] = e
			}
			e.r += uint64(c.R)
			e.g += uint64(c.G)
			e.b += uint64(c.B)
			e.n++
		}
	}
	type kv struct {
		k uint16
		v *bin
	}
	list := make([]kv, 0, len(hist))
	for k, v := range hist {
		list = append(list, kv{k, v})
	}
	// most frequent colours first; ties broken by key for determinism
	slices.SortFunc(list, func(a, c kv) int {
		if a.v.n != c.v.n {
			return cmp.Compare(c.v.n, a.v.n)
		}
		return cmp.Compare(a.k, c.k)
	})
	pal := color.Palette{color.NRGBA{0, 0, 0, 0}}
	for i := 0; i < len(list) && i < 255; i++ {
		v := list[i].v
		pal = append(pal, color.NRGBA{uint8(v.r / v.n), uint8(v.g / v.n), uint8(v.b / v.n), 255})
	}
	img := image.NewPaletted(b, pal)
	draw.FloydSteinberg.Draw(img, b, src, b.Min)
	// FloydSteinberg may dither opaque pixels into the transparent slot; restore alpha rules.
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			a := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA).A
			switch {
			case a < 128:
				img.SetColorIndex(x, y, 0)
			case img.ColorIndexAt(x, y) == 0 && len(pal) > 1:
				img.SetColorIndex(x, y, uint8(1+pal[1:].Index(src.At(x, y))))
			}
		}
	}
	return img
}

// DecodeBytes decodes any still format Photoly accepts, including HEIC/HEIF (and ICO via ffmpeg),
// the equivalent of Pillow with pillow-heif. EXIF orientation is NOT applied.
func DecodeBytes(ctx context.Context, data []byte) (image.Image, error) {
	if img, _, err := image.Decode(bytes.NewReader(data)); err == nil {
		return img, nil
	} else {
		f, terr := os.CreateTemp("", "photoly-decode-*")
		if terr != nil {
			return nil, err
		}
		defer os.Remove(f.Name())
		_, werr := f.Write(data)
		f.Close()
		if werr != nil {
			return nil, err
		}
		img, ferr := ffmpegStill(ctx, f.Name(), 30*time.Second)
		if ferr != nil {
			return nil, err
		}
		return img, nil
	}
}
