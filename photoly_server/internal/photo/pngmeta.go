package photo

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
)

func pngChunk(typ string, data []byte) []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.BigEndian, uint32(len(data)))
	b.WriteString(typ)
	b.Write(data)
	crc := crc32.NewIEEE()
	crc.Write([]byte(typ))
	crc.Write(data)
	_ = binary.Write(&b, binary.BigEndian, crc.Sum32())
	return b.Bytes()
}

// addPNGMeta inserts iCCP and eXIf chunks right after IHDR (before any IDAT).
func addPNGMeta(png, exif, icc []byte) []byte {
	if len(png) < 33 || (len(exif) == 0 && len(icc) == 0) {
		return png
	}
	var extra bytes.Buffer
	if len(icc) > 0 {
		var z bytes.Buffer
		w := zlib.NewWriter(&z)
		_, _ = w.Write(icc)
		_ = w.Close()
		extra.Write(pngChunk("iCCP", append([]byte("ICC Profile\x00\x00"), z.Bytes()...)))
	}
	if len(exif) > 0 {
		extra.Write(pngChunk("eXIf", exif))
	}
	out := append([]byte{}, png[:33]...) // signature + IHDR chunk (8+25)
	out = append(out, extra.Bytes()...)
	return append(out, png[33:]...)
}
