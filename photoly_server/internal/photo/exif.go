package photo

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"os"
	"strings"
)

// Meta is a subset of `exiftool -json -n` output: the tag names and value kinds the
// application consumes (see metadata.go). Numbers are int64 or float64, the rest strings.
type Meta map[string]any

// tiffData is a parsed TIFF/EXIF block.
type tiffData struct {
	b  []byte
	bo binary.ByteOrder
}

type ifdEntry struct {
	tag   uint16
	typ   uint16
	count uint32
	data  []byte // value bytes (inline or referenced)
}

func newTIFF(b []byte) *tiffData {
	if len(b) < 8 {
		return nil
	}
	var bo binary.ByteOrder
	switch string(b[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return nil
	}
	if bo.Uint16(b[2:4]) != 42 {
		return nil
	}
	return &tiffData{b: b, bo: bo}
}

var typeSize = map[uint16]int{1: 1, 2: 1, 3: 2, 4: 4, 5: 8, 6: 1, 7: 1, 8: 2, 9: 4, 10: 8, 11: 4, 12: 8, 13: 4}

// ifd parses the entries of the IFD at off (offsets relative to base).
func (t *tiffData) ifd(off uint32, base uint32) []ifdEntry {
	b := t.b
	if uint64(off)+2 > uint64(len(b)) {
		return nil
	}
	n := int(t.bo.Uint16(b[off:]))
	var out []ifdEntry
	for i := 0; i < n; i++ {
		p := uint64(off) + 2 + uint64(i)*12
		if p+12 > uint64(len(b)) {
			break
		}
		e := ifdEntry{tag: t.bo.Uint16(b[p:]), typ: t.bo.Uint16(b[p+2:]), count: t.bo.Uint32(b[p+4:])}
		sz, ok := typeSize[e.typ]
		if !ok {
			continue
		}
		total := uint64(sz) * uint64(e.count)
		if total <= 4 {
			e.data = b[p+8 : p+8+total]
		} else {
			vo := uint64(t.bo.Uint32(b[p+8:])) + uint64(base)
			if vo+total > uint64(len(b)) {
				continue
			}
			e.data = b[vo : vo+total]
		}
		out = append(out, e)
	}
	return out
}

func (t *tiffData) nums(e ifdEntry) []float64 {
	var out []float64
	switch e.typ {
	case 1, 7:
		for _, v := range e.data {
			out = append(out, float64(v))
		}
	case 6:
		for _, v := range e.data {
			out = append(out, float64(int8(v)))
		}
	case 3:
		for i := 0; i+2 <= len(e.data); i += 2 {
			out = append(out, float64(t.bo.Uint16(e.data[i:])))
		}
	case 8:
		for i := 0; i+2 <= len(e.data); i += 2 {
			out = append(out, float64(int16(t.bo.Uint16(e.data[i:]))))
		}
	case 4, 13:
		for i := 0; i+4 <= len(e.data); i += 4 {
			out = append(out, float64(t.bo.Uint32(e.data[i:])))
		}
	case 9:
		for i := 0; i+4 <= len(e.data); i += 4 {
			out = append(out, float64(int32(t.bo.Uint32(e.data[i:]))))
		}
	case 5:
		for i := 0; i+8 <= len(e.data); i += 8 {
			n, d := float64(t.bo.Uint32(e.data[i:])), float64(t.bo.Uint32(e.data[i+4:]))
			if d == 0 {
				out = append(out, math.Inf(1))
			} else {
				out = append(out, n/d)
			}
		}
	case 10:
		for i := 0; i+8 <= len(e.data); i += 8 {
			n, d := float64(int32(t.bo.Uint32(e.data[i:]))), float64(int32(t.bo.Uint32(e.data[i+4:])))
			if d == 0 {
				out = append(out, math.Inf(1))
			} else {
				out = append(out, n/d)
			}
		}
	case 11:
		for i := 0; i+4 <= len(e.data); i += 4 {
			out = append(out, float64(math.Float32frombits(t.bo.Uint32(e.data[i:]))))
		}
	case 12:
		for i := 0; i+8 <= len(e.data); i += 8 {
			out = append(out, math.Float64frombits(t.bo.Uint64(e.data[i:])))
		}
	}
	return out
}

func (t *tiffData) str(e ifdEntry) string {
	d := e.data
	if i := bytes.IndexByte(d, 0); i >= 0 {
		d = d[:i]
	}
	return strings.TrimRight(string(d), " \t\r\n")
}

// parseTIFF fills m with the tags we expose, and returns the orientation (1 if unknown).
func parseTIFF(raw []byte, m Meta, tiffFile bool) {
	t := newTIFF(raw)
	if t == nil {
		return
	}
	ifd0 := t.ifd(t.bo.Uint32(raw[4:]), 0)
	var exifPtr, gpsPtr uint32
	for _, e := range ifd0 {
		switch e.tag {
		case 0x010F:
			setStr(m, "Make", t.str(e))
		case 0x0110:
			setStr(m, "Model", t.str(e))
		case 0x013B:
			setStr(m, "Artist", t.str(e))
		case 0x011A:
			setNum(m, "XResolution", t.nums(e))
		case 0x011B:
			setNum(m, "YResolution", t.nums(e))
		case 0x0112:
			if v := t.nums(e); len(v) > 0 {
				m["Orientation"] = int64(v[0])
			}
		case 0x8769:
			if v := t.nums(e); len(v) > 0 {
				exifPtr = uint32(v[0])
			}
		case 0x8825:
			if v := t.nums(e); len(v) > 0 {
				gpsPtr = uint32(v[0])
			}
		case 0x0100, 0x0101, 0x0102:
			if tiffFile {
				name := map[uint16]string{0x100: "ImageWidth", 0x101: "ImageHeight", 0x102: "BitsPerSample"}[e.tag]
				v := t.nums(e)
				if e.tag == 0x102 && len(v) > 1 {
					parts := make([]string, len(v))
					for i, x := range v {
						parts[i] = pyStr(normNum(x))
					}
					m[name] = strings.Join(parts, " ")
				} else {
					setNum(m, name, v)
				}
			}
		}
	}
	if exifPtr != 0 {
		for _, e := range t.ifd(exifPtr, 0) {
			switch e.tag {
			case 0x829A:
				setNum(m, "ExposureTime", t.nums(e))
			case 0x829D:
				setNum(m, "FNumber", t.nums(e))
			case 0x8827:
				setNum(m, "ISO", t.nums(e))
			case 0x9003:
				setStr(m, "DateTimeOriginal", t.str(e))
			case 0x9004:
				setStr(m, "CreateDate", t.str(e))
			case 0x9201:
				if v := t.nums(e); len(v) > 0 && !math.IsInf(v[0], 0) {
					if math.Abs(v[0]) < 100 {
						m["ShutterSpeedValue"] = normNum(1 / math.Pow(2, v[0]))
					} else {
						m["ShutterSpeedValue"] = int64(0)
					}
				}
			case 0x9202:
				if v := t.nums(e); len(v) > 0 && !math.IsInf(v[0], 0) {
					m["ApertureValue"] = normNum(math.Pow(math.Sqrt2, v[0]))
				}
			case 0x920A:
				setNum(m, "FocalLength", t.nums(e))
			case 0xA001:
				setNum(m, "ColorSpace", t.nums(e))
			case 0xA433:
				setStr(m, "LensMake", t.str(e))
			case 0xA434:
				setStr(m, "LensModel", t.str(e))
			case 0x927C:
				parseMakerNote(e.data, m)
			}
		}
	}
	if gpsPtr != 0 {
		parseGPS(t, t.ifd(gpsPtr, 0), m)
	}
}

func setStr(m Meta, k, v string) {
	if v != "" {
		m[k] = v
	}
}

func setNum(m Meta, k string, v []float64) {
	if len(v) == 0 || math.IsInf(v[0], 0) || math.IsNaN(v[0]) {
		return
	}
	m[k] = normNum(v[0])
}

func parseGPS(t *tiffData, entries []ifdEntry, m Meta) {
	var latRef, lonRef string
	var lat, lon []float64
	var alt float64
	var haveAlt bool
	var altRef int64 = -1
	dms := func(v []float64) []float64 {
		if len(v) == 3 {
			return v
		}
		return nil
	}
	for _, e := range entries {
		switch e.tag {
		case 0:
			v := t.nums(e)
			parts := make([]string, len(v))
			for i, x := range v {
				parts[i] = pyStr(int64(x))
			}
			m["GPSVersionID"] = strings.Join(parts, " ")
		case 1:
			latRef = t.str(e)
			setStr(m, "GPSLatitudeRef", latRef)
		case 2:
			lat = dms(t.nums(e))
		case 3:
			lonRef = t.str(e)
			setStr(m, "GPSLongitudeRef", lonRef)
		case 4:
			lon = dms(t.nums(e))
		case 5:
			if v := t.nums(e); len(v) > 0 {
				altRef = int64(v[0])
				m["GPSAltitudeRef"] = altRef
			}
		case 6:
			if v := t.nums(e); len(v) > 0 && !math.IsInf(v[0], 0) {
				alt, haveAlt = v[0], true
			}
		}
	}
	conv := func(v []float64, ref string) (float64, bool) {
		if v == nil {
			return 0, false
		}
		d := v[0] + v[1]/60 + v[2]/3600
		if ref == "S" || ref == "W" {
			d = -d
		}
		return d, true
	}
	if d, ok := conv(lat, latRef); ok {
		m["GPSLatitude"] = normNum(d)
	}
	if d, ok := conv(lon, lonRef); ok {
		m["GPSLongitude"] = normNum(d)
	}
	if haveAlt {
		if altRef == 1 {
			alt = -alt
		}
		m["GPSAltitude"] = normNum(alt)
	}
}

// parseMakerNote reads Apple's ContentIdentifier (tag 0x0011), which pairs Live Photo parts.
func parseMakerNote(mn []byte, m Meta) {
	const header = "Apple iOS\x00"
	if !bytes.HasPrefix(mn, []byte(header)) || len(mn) < 16 {
		return
	}
	var bo binary.ByteOrder
	switch string(mn[12:14]) {
	case "MM":
		bo = binary.BigEndian
	case "II":
		bo = binary.LittleEndian
	default:
		return
	}
	t := &tiffData{b: mn, bo: bo}
	for _, e := range t.ifd(14, 0) {
		if e.tag == 0x0011 && e.typ == 2 {
			setStr(m, "ContentIdentifier", t.str(e))
		}
	}
}

// fileInfo is the result of inspecting a file container.
type fileInfo struct {
	Meta Meta
	Exif []byte // raw TIFF block (EXIF), if any
	ICC  []byte
}

// inspectFile reads container-level metadata and EXIF using pure Go.
func inspectFile(path string) (*fileInfo, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	head := make([]byte, 16)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, false
	}
	info := &fileInfo{Meta: Meta{}}
	switch {
	case bytes.HasPrefix(head, []byte{0xFF, 0xD8}):
		info.Meta["FileType"] = "JPEG"
		readJPEG(f, info)
	case bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n")):
		info.Meta["FileType"] = "PNG"
		readPNG(f, info)
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		info.Meta["FileType"] = "WEBP"
		readWebP(f, info)
	case len(head) >= 4 && (string(head[:4]) == "II*\x00" || string(head[:4]) == "MM\x00*"):
		info.Meta["FileType"] = "TIFF"
		b, err := io.ReadAll(io.LimitReader(f, 64<<20))
		if err != nil {
			return nil, false
		}
		parseTIFF(b, info.Meta, true)
		info.Exif = nil
	case len(head) >= 12 && string(head[4:8]) == "ftyp":
		if !readHEIF(f, info) {
			return nil, false
		}
	default:
		return nil, false
	}
	if info.Exif != nil {
		parseTIFF(info.Exif, info.Meta, false)
	}
	return info, true
}

func readJPEG(f *os.File, info *fileInfo) {
	var icc [][]byte
	if _, err := f.Seek(2, io.SeekStart); err != nil {
		return
	}
	hdr := make([]byte, 4)
	for {
		if _, err := io.ReadFull(f, hdr[:2]); err != nil {
			break
		}
		if hdr[0] != 0xFF {
			break
		}
		marker := hdr[1]
		for marker == 0xFF { // fill bytes
			if _, err := io.ReadFull(f, hdr[1:2]); err != nil {
				return
			}
			marker = hdr[1]
		}
		if marker == 0xD8 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			continue
		}
		if marker == 0xD9 || marker == 0xDA {
			break
		}
		if _, err := io.ReadFull(f, hdr[2:4]); err != nil {
			break
		}
		l := int(binary.BigEndian.Uint16(hdr[2:4])) - 2
		if l < 0 {
			break
		}
		isSOF := marker >= 0xC0 && marker <= 0xCF && marker != 0xC4 && marker != 0xC8 && marker != 0xCC
		need := l
		if !(isSOF || marker == 0xE1 || marker == 0xE2) {
			if _, err := f.Seek(int64(l), io.SeekCurrent); err != nil {
				break
			}
			continue
		}
		seg := make([]byte, need)
		if _, err := io.ReadFull(f, seg); err != nil {
			break
		}
		switch {
		case isSOF && len(seg) >= 6:
			info.Meta["BitsPerSample"] = int64(seg[0])
			info.Meta["ImageHeight"] = int64(binary.BigEndian.Uint16(seg[1:3]))
			info.Meta["ImageWidth"] = int64(binary.BigEndian.Uint16(seg[3:5]))
		case marker == 0xE1 && bytes.HasPrefix(seg, []byte("Exif\x00\x00")) && info.Exif == nil:
			info.Exif = seg[6:]
		case marker == 0xE2 && bytes.HasPrefix(seg, []byte("ICC_PROFILE\x00")) && len(seg) > 14:
			icc = append(icc, seg[14:])
		}
	}
	if len(icc) > 0 {
		info.ICC = bytes.Join(icc, nil)
	}
}

func readPNG(f *os.File, info *fileInfo) {
	if _, err := f.Seek(8, io.SeekStart); err != nil {
		return
	}
	hdr := make([]byte, 8)
	for {
		if _, err := io.ReadFull(f, hdr); err != nil {
			return
		}
		l := int64(binary.BigEndian.Uint32(hdr[:4]))
		typ := string(hdr[4:])
		switch typ {
		case "IHDR", "eXIf", "iCCP":
			if l > 32<<20 {
				return
			}
			data := make([]byte, l)
			if _, err := io.ReadFull(f, data); err != nil {
				return
			}
			switch typ {
			case "IHDR":
				if len(data) >= 9 {
					info.Meta["ImageWidth"] = int64(binary.BigEndian.Uint32(data[0:4]))
					info.Meta["ImageHeight"] = int64(binary.BigEndian.Uint32(data[4:8]))
					info.Meta["BitsPerSample"] = int64(data[8])
				}
			case "eXIf":
				info.Exif = data
			case "iCCP":
				info.ICC = inflateICC(data)
			}
			_, _ = f.Seek(4, io.SeekCurrent) // CRC
		case "IEND":
			return
		default:
			if _, err := f.Seek(l+4, io.SeekCurrent); err != nil {
				return
			}
		}
	}
}

func readWebP(f *os.File, info *fileInfo) {
	if _, err := f.Seek(12, io.SeekStart); err != nil {
		return
	}
	hdr := make([]byte, 8)
	for {
		if _, err := io.ReadFull(f, hdr); err != nil {
			return
		}
		l := int64(binary.LittleEndian.Uint32(hdr[4:]))
		typ := string(hdr[:4])
		pad := l & 1
		switch typ {
		case "EXIF", "ICCP", "VP8X":
			if l > 32<<20 {
				return
			}
			data := make([]byte, l)
			if _, err := io.ReadFull(f, data); err != nil {
				return
			}
			_, _ = f.Seek(pad, io.SeekCurrent)
			switch typ {
			case "EXIF":
				data = bytes.TrimPrefix(data, []byte("Exif\x00\x00"))
				info.Exif = data
			case "ICCP":
				info.ICC = data
			case "VP8X":
				if len(data) >= 10 {
					w := 1 + (int64(data[4]) | int64(data[5])<<8 | int64(data[6])<<16)
					h := 1 + (int64(data[7]) | int64(data[8])<<8 | int64(data[9])<<16)
					info.Meta["ImageWidth"], info.Meta["ImageHeight"] = w, h
				}
			}
		default:
			if _, err := f.Seek(l+pad, io.SeekCurrent); err != nil {
				return
			}
		}
	}
}
