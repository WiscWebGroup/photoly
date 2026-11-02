package photo

import (
	"encoding/binary"
	"reflect"
	"testing"
)

func TestMediaClassification(t *testing.T) {
	for _, c := range []struct {
		name, format string
		m            Meta
		want         []string
	}{
		{"wide", "jpg", Meta{"ImageWidth": int64(6000), "ImageHeight": int64(2000)}, []string{"panorama"}},
		{"Screenshot_2026", "png", Meta{"ImageWidth": int64(6000), "ImageHeight": int64(1000)}, []string{"screenshot"}},
		{"IMG_001", "heic", Meta{"ImageCaptureType": float64(2)}, []string{"portrait"}},
		{"people", "jpg", Meta{}, []string{}},
		{"Screenshot", "mp4", Meta{"ImageCaptureType": int64(2)}, []string{}},
		{"wide", "raw", Meta{}, []string{}},
		{"narrow", "jpg", Meta{"ImageWidth": int64(5999), "ImageHeight": int64(2000)}, []string{}},
		{"IMG", "jpg", Meta{"ProjectionType": "equirectangular"}, []string{"panorama"}},
	} {
		t.Run(c.name+"."+c.format, func(t *testing.T) {
			got := classifyMedia(c.name, c.format, c.m)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}
func TestPortraitMakerNoteAndXMP(t *testing.T) {
	b := make([]byte, 32)
	copy(b, "Apple iOS\x00")
	copy(b[12:], "MM")
	binary.BigEndian.PutUint16(b[14:], 1)
	binary.BigEndian.PutUint16(b[16:], 0x14)
	binary.BigEndian.PutUint16(b[18:], 9)
	binary.BigEndian.PutUint32(b[20:], 1)
	binary.BigEndian.PutUint32(b[24:], 2)
	m := Meta{}
	parseMakerNote(b, m)
	if v, _ := num(m, "ImageCaptureType"); v != 2 {
		t.Fatal(m)
	}
	readCategoryXMP([]byte(`<rdf:Description xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:GPano="http://ns.google.com/photos/1.0/panorama/" GPano:ProjectionType="equirectangular"/>`), m)
	if m.String("ProjectionType") != "equirectangular" {
		t.Fatal(m)
	}
}
