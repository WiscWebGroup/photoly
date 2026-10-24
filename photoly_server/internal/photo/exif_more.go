package photo

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func inflateICC(data []byte) []byte {
	i := bytes.IndexByte(data, 0)
	if i < 0 || i+2 > len(data) {
		return nil
	}
	r, err := zlib.NewReader(bytes.NewReader(data[i+2:]))
	if err != nil {
		return nil
	}
	defer r.Close()
	out, _ := io.ReadAll(io.LimitReader(r, 16<<20))
	return out
}

// ---- ISO base media (HEIC/HEIF) ----

type isoBox struct {
	typ  string
	data []byte
}

func isoBoxes(b []byte) []isoBox {
	var out []isoBox
	for len(b) >= 8 {
		size := uint64(binary.BigEndian.Uint32(b[:4]))
		typ := string(b[4:8])
		hdr := uint64(8)
		if size == 1 && len(b) >= 16 {
			size, hdr = binary.BigEndian.Uint64(b[8:16]), 16
		} else if size == 0 {
			size = uint64(len(b))
		}
		if size < hdr || size > uint64(len(b)) {
			break
		}
		out = append(out, isoBox{typ, b[hdr:size]})
		b = b[size:]
	}
	return out
}

func findBox(boxes []isoBox, typ string) []byte {
	for _, b := range boxes {
		if b.typ == typ {
			return b.data
		}
	}
	return nil
}

// readHEIF extracts the EXIF item and primary dimensions from a HEIC/HEIF/AVIF file.
func readHEIF(f *os.File, info *fileInfo) bool {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return false
	}
	// Only metadata boxes are needed; read through meta, bounded.
	all, err := io.ReadAll(io.LimitReader(f, 64<<20))
	if err != nil {
		return false
	}
	top := isoBoxes(all)
	ftyp := findBox(top, "ftyp")
	if len(ftyp) < 4 {
		return false
	}
	brand := string(ftyp[:4])
	switch brand {
	case "heic", "heix", "hevc", "hevx":
		info.Meta["FileType"] = "HEIC"
	case "mif1", "msf1", "heif":
		info.Meta["FileType"] = "HEIF"
	case "avif", "avis":
		info.Meta["FileType"] = "AVIF"
	default:
		return false // QuickTime/MP4: handled by ffprobe
	}
	meta := findBox(top, "meta")
	if len(meta) < 4 {
		return true
	}
	kids := isoBoxes(meta[4:])
	// item types
	exifID := uint32(0)
	if iinf := findBox(kids, "iinf"); len(iinf) >= 6 {
		ver := iinf[0]
		off := 4 + 2
		if ver != 0 {
			off = 4 + 4
		}
		if off <= len(iinf) {
			for _, infe := range isoBoxes(iinf[off:]) {
				d := infe.data
				if infe.typ != "infe" || len(d) < 4 || d[0] < 2 {
					continue
				}
				var id uint32
				var rest []byte
				if d[0] == 2 {
					if len(d) < 4+2+2+4 {
						continue
					}
					id, rest = uint32(binary.BigEndian.Uint16(d[4:])), d[4+2+2:]
				} else {
					if len(d) < 4+4+2+4 {
						continue
					}
					id, rest = binary.BigEndian.Uint32(d[4:]), d[4+4+2:]
				}
				if string(rest[:4]) == "Exif" {
					exifID = id
				}
			}
		}
	}
	if exifID != 0 {
		if iloc := findBox(kids, "iloc"); len(iloc) > 8 {
			if payload := ilocRead(all, iloc, exifID); len(payload) > 4 {
				skip := int(binary.BigEndian.Uint32(payload[:4]))
				if 4+skip <= len(payload) {
					info.Exif = payload[4+skip:]
				}
			}
		}
	}
	// largest ispe = primary image size (grid tiles are smaller)
	var bestW, bestH uint32
	if iprp := findBox(kids, "iprp"); iprp != nil {
		for _, ipco := range isoBoxes(iprp) {
			if ipco.typ != "ipco" {
				continue
			}
			for _, p := range isoBoxes(ipco.data) {
				if p.typ == "ispe" && len(p.data) >= 12 {
					w, h := binary.BigEndian.Uint32(p.data[4:]), binary.BigEndian.Uint32(p.data[8:])
					if uint64(w)*uint64(h) > uint64(bestW)*uint64(bestH) {
						bestW, bestH = w, h
					}
				}
			}
		}
	}
	if bestW > 0 {
		info.Meta["ImageWidth"], info.Meta["ImageHeight"] = int64(bestW), int64(bestH)
	}
	return true
}

func ilocRead(file, iloc []byte, want uint32) []byte {
	ver := iloc[0]
	b := iloc[4:]
	if len(b) < 4 {
		return nil
	}
	offSize, lenSize := int(b[0]>>4), int(b[0]&15)
	baseSize, idxSize := int(b[1]>>4), 0
	if ver == 1 || ver == 2 {
		idxSize = int(b[1] & 15)
	}
	b = b[2:]
	var count uint32
	if ver < 2 {
		count, b = uint32(binary.BigEndian.Uint16(b)), b[2:]
	} else {
		if len(b) < 4 {
			return nil
		}
		count, b = binary.BigEndian.Uint32(b), b[4:]
	}
	rd := func(n int) uint64 {
		if n == 0 || len(b) < n {
			b = b[min(n, len(b)):]
			return 0
		}
		var v uint64
		for _, c := range b[:n] {
			v = v<<8 | uint64(c)
		}
		b = b[n:]
		return v
	}
	for i := uint32(0); i < count && len(b) > 0; i++ {
		var id uint32
		if ver < 2 {
			id = uint32(rd(2))
		} else {
			id = uint32(rd(4))
		}
		method := uint64(0)
		if ver == 1 || ver == 2 {
			method = rd(2) & 15
		}
		rd(2) // data reference
		base := rd(baseSize)
		extents := int(rd(2))
		var out []byte
		for e := 0; e < extents; e++ {
			if (ver == 1 || ver == 2) && idxSize > 0 {
				rd(idxSize)
			}
			off, l := rd(offSize), rd(lenSize)
			if id == want && method == 0 {
				s := base + off
				if s+l <= uint64(len(file)) {
					out = append(out, file[s:s+l]...)
				}
			}
		}
		if id == want {
			return out
		}
	}
	return nil
}

// ---- video metadata through ffprobe ----

type probe struct {
	Streams []struct {
		CodecType string            `json:"codec_type"`
		Width     int               `json:"width"`
		Height    int               `json:"height"`
		Tags      map[string]string `json:"tags"`
	} `json:"streams"`
	Format struct {
		FormatName string            `json:"format_name"`
		Tags       map[string]string `json:"tags"`
	} `json:"format"`
}

var iso6709 = regexp.MustCompile(`^([+-][0-9.]+)([+-][0-9.]+)([+-][0-9.]+)?`)

// ffprobeMeta reads video container dates and QuickTime keys (replaces exiftool for movies).
func ffprobeMeta(ctx context.Context, path string) Meta {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", path).Output()
	if err != nil {
		return Meta{}
	}
	var p probe
	if json.Unmarshal(out, &p) != nil {
		return Meta{}
	}
	m := Meta{}
	tag := func(k string) string {
		for key, v := range p.Format.Tags {
			if strings.EqualFold(key, k) {
				return v
			}
		}
		return ""
	}
	names := p.Format.FormatName
	switch {
	case strings.Contains(names, "mov") && strings.EqualFold(strings.TrimSpace(tag("major_brand")), "qt"):
		m["FileType"] = "MOV"
	case strings.Contains(names, "mov"):
		m["FileType"] = "MP4"
	case names != "":
		m["FileType"] = strings.ToUpper(strings.Split(names, ",")[0])
	}
	for _, s := range p.Streams {
		if s.CodecType == "video" && s.Width > 0 {
			m["ImageWidth"], m["ImageHeight"] = int64(s.Width), int64(s.Height)
			break
		}
	}
	if v := tag("com.apple.quicktime.content.identifier"); v != "" {
		m["ContentIdentifier"] = v
	}
	if v := tag("com.apple.quicktime.make"); v != "" {
		m["Make"] = v
	}
	if v := tag("com.apple.quicktime.model"); v != "" {
		m["Model"] = v
	}
	if v := tag("com.apple.quicktime.creationdate"); v != "" {
		if s := qtDate(v, true); s != "" {
			m["CreationDate"] = s
		}
	}
	ct := tag("creation_time")
	if ct == "" {
		for _, s := range p.Streams {
			if v := s.Tags["creation_time"]; v != "" {
				ct = v
				break
			}
		}
	}
	if s := qtDate(ct, false); s != "" {
		m["CreateDate"] = s
	}
	if loc := iso6709.FindStringSubmatch(strings.TrimSuffix(tag("com.apple.quicktime.location.iso6709"), "/")); loc != nil {
		if lat, err := strconv.ParseFloat(loc[1], 64); err == nil {
			m["GPSLatitude"] = normNum(lat)
		}
		if lon, err := strconv.ParseFloat(loc[2], 64); err == nil {
			m["GPSLongitude"] = normNum(lon)
		}
		if loc[3] != "" {
			if alt, err := strconv.ParseFloat(loc[3], 64); err == nil {
				m["GPSAltitude"] = normNum(alt)
			}
		}
	}
	return m
}

// qtDate converts ffprobe timestamps to exiftool's "YYYY:MM:DD HH:MM:SS[+TZ]". Implausible
// zero timestamps are dropped.
func qtDate(v string, keepZone bool) string {
	if v == "" {
		return ""
	}
	var t time.Time
	var err error
	for _, layout := range []string{"2006-01-02T15:04:05.999999999Z07:00", "2006-01-02T15:04:05-0700", "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		if t, err = time.Parse(layout, v); err == nil {
			break
		}
	}
	if err != nil || t.Year() < 1970 {
		return ""
	}
	out := t.Format("2006:01:02 15:04:05")
	if keepZone {
		z := t.Format("-07:00")
		if t.Location() == time.UTC && strings.HasSuffix(v, "Z") {
			z = "Z"
		}
		out += z
	}
	return out
}

// ---- backend selection ----

// ExifConfig selects the metadata backend.
type ExifConfig struct {
	// Backend: "go" (pure Go + ffprobe, default), "exiftool" (always), or "auto"
	// (pure Go first; exiftool when the Go reader found no EXIF at all).
	Backend string
	// Exiftool is the executable (EXIFTOOL_PATH).
	Exiftool string
}

// ExifConfigFromEnv reads EXIF_BACKEND and EXIFTOOL_PATH.
func ExifConfigFromEnv() ExifConfig {
	c := ExifConfig{Backend: strings.ToLower(os.Getenv("EXIF_BACKEND")), Exiftool: os.Getenv("EXIFTOOL_PATH")}
	if c.Exiftool == "" {
		c.Exiftool = "exiftool"
	}
	if c.Backend == "" {
		c.Backend = "go"
	}
	return c
}

// ReadMetadata is MetadataReader.read: it never fails; unreadable files yield an empty Meta.
func (s *Service) ReadMetadata(ctx context.Context, path string) Meta {
	m, _ := s.readMetadataInfo(ctx, path)
	return m
}

func (s *Service) readMetadataInfo(ctx context.Context, path string) (Meta, *fileInfo) {
	if s.Exif.Backend == "exiftool" {
		return runExiftool(ctx, s.Exif.Exiftool, path), nil
	}
	m, info := goMetadata(ctx, path)
	if s.Exif.Backend == "auto" && (info == nil || info.Exif == nil) {
		if et := runExiftool(ctx, s.Exif.Exiftool, path); len(et) > 0 {
			return et, info
		}
	}
	return m, info
}

func goMetadata(ctx context.Context, path string) (Meta, *fileInfo) {
	if info, ok := inspectFile(path); ok {
		m := info.Meta
		if _, ok := m["ImageWidth"]; !ok {
			if w, h, ok := imageDims(path); ok {
				m["ImageWidth"], m["ImageHeight"] = int64(w), int64(h)
			}
		}
		return m, info
	}
	if w, h, ok := imageDims(path); ok { // GIF, BMP, ...
		return Meta{"ImageWidth": int64(w), "ImageHeight": int64(h)}, nil
	}
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(path), ".")) {
	case "jpg", "jpeg", "png", "gif", "webp", "bmp", "tif", "tiff", "ico", "svg", "psd", "raw", "arw", "pcd", "jfif", "heic", "heif":
		return Meta{}, nil
	}
	return ffprobeMeta(ctx, path), nil
}

// runExiftool is the legacy path: exiftool -json -n.
func runExiftool(ctx context.Context, exe, path string) Meta {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	abs, err := filepath.Abs(path)
	if err != nil {
		return Meta{}
	}
	out, err := exec.CommandContext(ctx, exe, "-json", "-n", abs).Output()
	if err != nil && len(out) == 0 {
		return Meta{}
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	dec.UseNumber()
	var recs []map[string]any
	if dec.Decode(&recs) != nil || len(recs) == 0 {
		return Meta{}
	}
	m := Meta{}
	for k, v := range recs[0] {
		switch x := v.(type) {
		case json.Number:
			if i, err := x.Int64(); err == nil {
				m[k] = i
			} else if f, err := x.Float64(); err == nil {
				m[k] = f
			}
		default:
			m[k] = v
		}
	}
	return m
}

// RawEXIF returns the raw TIFF EXIF block and ICC profile of an image (nil when absent).
func RawEXIF(path string) (exif, icc []byte) {
	if info, ok := inspectFile(path); ok {
		return info.Exif, info.ICC
	}
	return nil, nil
}
