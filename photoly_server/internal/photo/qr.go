package photo

import (
	"bytes"
	"math"
	"image"
	"image/color"
	"image/png"
	"regexp"
)

// A self-contained QR encoder that follows python-qrcode's defaults used by qrcode.make:
// error correction M, mixed numeric/alphanumeric/byte segments (optimize=20), smallest fitting
// version, mask by python-qrcode's penalty, 10px modules with a 4 module border, nearest-resized to 250px.

const qrAlnum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ $%*+-./:"

// level M: total codewords, EC codewords per block, blocks - versions 1..20.
var qrM = [][3]int{{26, 10, 1}, {44, 16, 1}, {70, 26, 1}, {100, 18, 2}, {134, 24, 2}, {172, 16, 4}, {196, 18, 4}, {242, 22, 4},
	{292, 22, 5}, {346, 26, 5}, {404, 30, 5}, {466, 22, 8}, {532, 22, 9}, {581, 24, 9}, {655, 24, 10}, {733, 28, 10},
	{815, 28, 11}, {901, 26, 13}, {991, 26, 14}, {1085, 26, 16}}

var qrAlign = [][]int{{}, {6, 18}, {6, 22}, {6, 26}, {6, 30}, {6, 34}, {6, 22, 38}, {6, 24, 42}, {6, 26, 46}, {6, 28, 50}, {6, 30, 54},
	{6, 32, 58}, {6, 34, 62}, {6, 26, 46, 66}, {6, 26, 48, 70}, {6, 26, 50, 74}, {6, 30, 54, 78}, {6, 30, 56, 82}, {6, 30, 58, 86}, {6, 34, 62, 90}}

type qrSeg struct {
	mode int // 1 numeric, 2 alnum, 4 byte
	data []byte
}

type bitBuf struct {
	b []byte
	n int
}

func (w *bitBuf) put(v, bits int) {
	for i := bits - 1; i >= 0; i-- {
		if w.n%8 == 0 {
			w.b = append(w.b, 0)
		}
		if v>>i&1 == 1 {
			w.b[w.n/8] |= 0x80 >> (w.n % 8)
		}
		w.n++
	}
}

func qrLenBits(mode, version int) int {
	k := 0
	if version >= 27 {
		k = 2
	} else if version >= 10 {
		k = 1
	}
	return map[int][3]int{1: {10, 12, 14}, 2: {9, 11, 13}, 4: {8, 16, 16}}[mode][k]
}

func (g qrSeg) write(w *bitBuf) {
	d := g.data
	switch g.mode {
	case 1:
		for i := 0; i < len(d); i += 3 {
			e := min(i+3, len(d))
			v := 0
			for _, c := range d[i:e] {
				v = v*10 + int(c-'0')
			}
			w.put(v, []int{0, 4, 7, 10}[e-i])
		}
	case 2:
		idx := func(c byte) int {
			for i := 0; i < len(qrAlnum); i++ {
				if qrAlnum[i] == c {
					return i
				}
			}
			return 0
		}
		for i := 0; i+1 < len(d); i += 2 {
			w.put(idx(d[i])*45+idx(d[i+1]), 11)
		}
		if len(d)%2 == 1 {
			w.put(idx(d[len(d)-1]), 6)
		}
	default:
		for _, c := range d {
			w.put(int(c), 8)
		}
	}
}

// qrSplit is util._optimal_split.
func qrSplit(data []byte, re *regexp.Regexp) (parts [][]byte, matched []bool) {
	last := 0
	for _, m := range re.FindAllIndex(data, -1) {
		if m[0] > last {
			parts, matched = append(parts, data[last:m[0]]), append(matched, false)
		}
		parts, matched = append(parts, data[m[0]:m[1]]), append(matched, true)
		last = m[1]
	}
	if last < len(data) {
		parts, matched = append(parts, data[last:]), append(matched, false)
	}
	return
}

// qrChunks is util.optimal_data_chunks(data, minimum=20).
func qrChunks(data []byte) []qrSeg {
	const minimum = 20
	class := "[" + regexp.QuoteMeta(qrAlnum) + "]"
	num, alpha := `[0-9]`, class
	var nre, are *regexp.Regexp
	if len(data) <= minimum {
		nre, are = regexp.MustCompile(`^`+num+`+$`), regexp.MustCompile(`^`+alpha+`+$`)
	} else {
		nre, are = regexp.MustCompile(num+`{20,}`), regexp.MustCompile(alpha+`{20,}`)
	}
	var out []qrSeg
	parts, isNum := qrSplit(data, nre)
	for i, p := range parts {
		if isNum[i] {
			out = append(out, qrSeg{1, p})
			continue
		}
		sub, isAlpha := qrSplit(p, are)
		for j, q := range sub {
			if isAlpha[j] {
				out = append(out, qrSeg{2, q})
			} else {
				out = append(out, qrSeg{4, q})
			}
		}
	}
	return out
}

var gfExp, gfLog = func() ([512]int, [256]int) {
	var e [512]int
	var l [256]int
	x := 1
	for i := 0; i < 255; i++ {
		e[i], l[x] = x, i
		x <<= 1
		if x&0x100 != 0 {
			x ^= 0x11d
		}
	}
	for i := 255; i < 512; i++ {
		e[i] = e[i-255]
	}
	return e, l
}()

func rsRemainder(data []byte, ec int) []byte {
	gen := []int{1}
	for i := 0; i < ec; i++ {
		next := make([]int, len(gen)+1)
		for j, c := range gen {
			next[j] ^= c
			if c != 0 {
				next[j+1] ^= gfExp[gfLog[c]+i]
			}
		}
		gen = next
	}
	rem := make([]byte, ec)
	for _, d := range data {
		f := int(d ^ rem[0])
		copy(rem, rem[1:])
		rem[ec-1] = 0
		if f != 0 {
			for j := 0; j < ec; j++ {
				if gen[j+1] != 0 {
					rem[j] ^= byte(gfExp[gfLog[gen[j+1]]+gfLog[f]])
				}
			}
		}
	}
	return rem
}

func bchBits(v, poly int) int {
	digit := func(x int) int {
		n := 0
		for x != 0 {
			n++
			x >>= 1
		}
		return n
	}
	d := v
	for digit(d)-digit(poly) >= 0 {
		d ^= poly << (digit(d) - digit(poly))
	}
	return d
}

type qrMatrix struct {
	n    int
	mod  [][]bool
	set  [][]bool
	size int
}

func newMatrix(version int) *qrMatrix {
	n := version*4 + 17
	m := &qrMatrix{n: n}
	m.mod, m.set = make([][]bool, n), make([][]bool, n)
	for i := range m.mod {
		m.mod[i], m.set[i] = make([]bool, n), make([]bool, n)
	}
	return m
}

func (m *qrMatrix) put(r, c int, v bool) { m.mod[r][c], m.set[r][c] = v, true }

func qrMask(p, i, j int) bool {
	switch p {
	case 0:
		return (i+j)%2 == 0
	case 1:
		return i%2 == 0
	case 2:
		return j%3 == 0
	case 3:
		return (i+j)%3 == 0
	case 4:
		return (i/2+j/3)%2 == 0
	case 5:
		return i*j%2+i*j%3 == 0
	case 6:
		return (i*j%2+i*j%3)%2 == 0
	}
	return ((i*j)%3+(i+j)%2)%2 == 0
}

// build returns the module matrix for data codewords at a version and mask.
func qrBuild(version int, codewords []byte, mask int) [][]bool {
	m := newMatrix(version)
	n := m.n
	probe := func(r, c int) {
		for dr := -1; dr <= 7; dr++ {
			for dc := -1; dc <= 7; dc++ {
				if r+dr < 0 || r+dr >= n || c+dc < 0 || c+dc >= n {
					continue
				}
				v := (dr >= 0 && dr <= 6 && (dc == 0 || dc == 6)) || (dc >= 0 && dc <= 6 && (dr == 0 || dr == 6)) || (dr >= 2 && dr <= 4 && dc >= 2 && dc <= 4)
				m.put(r+dr, c+dc, v)
			}
		}
	}
	probe(0, 0)
	probe(n-7, 0)
	probe(0, n-7)
	pos := qrAlign[version-1]
	for _, r := range pos {
		for _, c := range pos {
			if m.set[r][c] {
				continue
			}
			for dr := -2; dr <= 2; dr++ {
				for dc := -2; dc <= 2; dc++ {
					m.put(r+dr, c+dc, dr == -2 || dr == 2 || dc == -2 || dc == 2 || (dr == 0 && dc == 0))
				}
			}
		}
	}
	for i := 8; i < n-8; i++ {
		if !m.set[i][6] {
			m.put(i, 6, i%2 == 0)
		}
		if !m.set[6][i] {
			m.put(6, i, i%2 == 0)
		}
	}
	// format information (level M = 0b00)
	bits := ((0<<3 | mask) << 10) | bchBits((0<<3|mask)<<10, 0x537)
	bits ^= 0x5412
	for i := 0; i < 15; i++ {
		v := bits>>i&1 == 1
		if i < 6 {
			m.put(i, 8, v)
		} else if i < 8 {
			m.put(i+1, 8, v)
		} else {
			m.put(n-15+i, 8, v)
		}
		if i < 8 {
			m.put(8, n-i-1, v)
		} else if i < 9 {
			m.put(8, 15-i-1+1, v)
		} else {
			m.put(8, 15-i-1, v)
		}
	}
	m.put(n-8, 8, true)
	if version >= 7 {
		vb := version<<12 | bchBits(version<<12, 0x1f25)
		for i := 0; i < 18; i++ {
			v := vb>>i&1 == 1
			m.put(i/3, i%3+n-8-3, v)
			m.put(i%3+n-8-3, i/3, v)
		}
	}
	// data placement
	inc, row, bitIdx, byteIdx := -1, n-1, 7, 0
	for col := n - 1; col > 0; col -= 2 {
		if col == 6 {
			col--
		}
		for {
			for c := 0; c < 2; c++ {
				if !m.set[row][col-c] {
					dark := false
					if byteIdx < len(codewords) {
						dark = codewords[byteIdx]>>bitIdx&1 == 1
					}
					if qrMask(mask, row, col-c) {
						dark = !dark
					}
					m.mod[row][col-c] = dark
					bitIdx--
					if bitIdx == -1 {
						byteIdx++
						bitIdx = 7
					}
				}
			}
			row += inc
			if row < 0 || n <= row {
				row -= inc
				inc = -inc
				break
			}
		}
	}
	return m.mod
}

// qrLost is python-qrcode 8.2's util.lost_point (levels 1-4, including its skip quirks).
func qrLost(mod [][]bool) int {
	n := len(mod)
	lost := 0
	runs := make([]int, n+1)
	for pass := 0; pass < 2; pass++ {
		at := func(i, j int) bool {
			if pass == 0 {
				return mod[i][j]
			}
			return mod[j][i]
		}
		for i := 0; i < n; i++ {
			prev, length := at(i, 0), 0
			for j := 0; j < n; j++ {
				if at(i, j) == prev {
					length++
				} else {
					if length >= 5 {
						runs[length]++
					}
					length, prev = 1, at(i, j)
				}
			}
			if length >= 5 {
				runs[length]++
			}
		}
	}
	for l := 5; l <= n; l++ {
		lost += runs[l] * (l - 2)
	}
	for r := 0; r < n-1; r++ {
		for c := 0; c < n-1; c++ {
			tr := mod[r][c+1]
			if tr != mod[r+1][c+1] {
				c++
			} else if tr != mod[r][c] || tr != mod[r+1][c] {
				continue
			} else {
				lost += 3
			}
		}
	}
	for pass := 0; pass < 2; pass++ {
		at := func(i, j int) bool {
			if pass == 0 {
				return mod[i][j]
			}
			return mod[j][i]
		}
		for i := 0; i < n; i++ {
			for j := 0; j < n-10; j++ {
				if !at(i, j+1) && at(i, j+4) && !at(i, j+5) && at(i, j+6) && !at(i, j+9) &&
					((at(i, j) && at(i, j+2) && at(i, j+3) && !at(i, j+7) && !at(i, j+8) && !at(i, j+10)) ||
						(!at(i, j) && !at(i, j+2) && !at(i, j+3) && at(i, j+7) && at(i, j+8) && at(i, j+10))) {
					lost += 40
				}
				if at(i, j+10) {
					j++
				}
			}
		}
	}
	dark := 0
	for r := range mod {
		for c := range mod[r] {
			if mod[r][c] {
				dark++
			}
		}
	}
	diff := math.Abs(float64(dark)/float64(n*n)*100 - 50)
	return lost + int(diff/5)*10
}

// QRMatrix returns the QR modules for text (true = dark), without the border.
func QRMatrix(text string) ([][]bool, error) {
	segs := qrChunks([]byte(text))
	var version, dataCW int
	var buf *bitBuf
	for v := 1; v <= len(qrM); v++ {
		total, ec, blocks := qrM[v-1][0], qrM[v-1][1], qrM[v-1][2]
		dataCW = total - ec*blocks
		b := &bitBuf{}
		for _, g := range segs {
			b.put(g.mode, 4)
			ln := len(g.data)
			b.put(ln, qrLenBits(g.mode, v))
			g.write(b)
		}
		if b.n <= dataCW*8 {
			version, buf = v, b
			break
		}
	}
	if buf == nil {
		return nil, Err("qr_data_overflow", 400)
	}
	buf.put(0, min(dataCW*8-buf.n, 4))
	for buf.n%8 != 0 {
		buf.put(0, 1)
	}
	for pad := 0; len(buf.b) < dataCW; pad++ {
		buf.b = append(buf.b, []byte{0xEC, 0x11}[pad%2])
	}
	total, ec, blocks := qrM[version-1][0], qrM[version-1][1], qrM[version-1][2]
	_ = total
	short := blocks - dataCW%blocks
	var data, ecs [][]byte
	off := 0
	for i := 0; i < blocks; i++ {
		size := dataCW / blocks
		if i >= short {
			size++
		}
		d := buf.b[off : off+size]
		off += size
		data, ecs = append(data, d), append(ecs, rsRemainder(d, ec))
	}
	var out []byte
	for i := 0; i <= dataCW/blocks; i++ {
		for _, d := range data {
			if i < len(d) {
				out = append(out, d[i])
			}
		}
	}
	for i := 0; i < ec; i++ {
		for _, e := range ecs {
			out = append(out, e[i])
		}
	}
	best, bestLost := 0, 0
	var bestMod [][]bool
	for mask := 0; mask < 8; mask++ {
		mod := qrBuild(version, out, mask)
		if l := qrLost(mod); mask == 0 || l < bestLost {
			best, bestLost, bestMod = mask, l, mod
		}
	}
	_ = best
	return bestMod, nil
}

// QRPNG is QRCodeEncoder.encode: qrcode.make(value).resize((250, 250)) as PNG bytes.
func QRPNG(text string) ([]byte, error) {
	mod, err := QRMatrix(text)
	if err != nil {
		return nil, err
	}
	n := len(mod) + 8
	src := n * 10
	img := image.NewPaletted(image.Rect(0, 0, 250, 250), color.Palette{color.Gray{0}, color.Gray{255}})
	for y := 0; y < 250; y++ {
		sy := int((float64(y) + 0.5) * float64(src) / 250)
		for x := 0; x < 250; x++ {
			sx := int((float64(x) + 0.5) * float64(src) / 250)
			r, c := sy/10-4, sx/10-4
			if r >= 0 && c >= 0 && r < len(mod) && c < len(mod) && mod[r][c] {
				continue
			}
			img.SetColorIndex(x, y, 1)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
