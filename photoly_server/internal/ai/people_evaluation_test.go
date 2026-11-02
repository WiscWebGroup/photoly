//go:build cgo

package ai

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/image/tiff"
)

// Fixed, reproducible release smoke set. These derivatives do not substitute for
// a consented multi-identity album benchmark; see docs/administration/people.md.
func TestPeopleEvaluation(t *testing.T) {
	if os.Getenv("PHOTOLY_TEST_FACES") == "" {
		t.Skip("set PHOTOLY_TEST_FACES=1")
	}
	t.Setenv("AI_TEST_RUNTIME", "")
	e, err := newEmbedder(newModelCache(t.TempDir()), ONNXProfile())
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if _, err = e.Load(context.Background(), "people", "fp32"); err != nil {
		t.Fatal(err)
	}
	read := func(path string) image.Image {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		im, _, err := image.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		return im
	}
	original := read("testdata/ai/astronaut.png")
	camera := read("testdata/people/camera.png")
	dark := image.NewRGBA(original.Bounds())
	small := image.NewRGBA(image.Rect(0, 0, 256, 256))
	group := image.NewRGBA(image.Rect(0, 0, 1024, 512))
	tilted := image.NewRGBA(original.Bounds())
	rotated := image.NewRGBA(original.Bounds())
	for y := 0; y < 512; y++ {
		for x := 0; x < 512; x++ {
			rotated.Set(x, y, original.At(511-y, x))
			r, g, b, _ := original.At(x, y).RGBA()
			dark.Set(x, y, color.RGBA{uint8(float64(r>>8) * .7), uint8(float64(g>>8) * .7), uint8(float64(b>>8) * .7), 255})
			group.Set(x, y, original.At(x, y))
			group.Set(x+512, y, camera.At(x, y))
			a := 10 * math.Pi / 180
			dx, dy := float64(x-256), float64(y-256)
			tilted.Set(x, y, original.At(int(dx*math.Cos(a)-dy*math.Sin(a))+256, int(dx*math.Sin(a)+dy*math.Cos(a))+256))
		}
	}
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			small.Set(x, y, original.At(x*2, y*2))
		}
	}
	samples := []struct {
		name     string
		im       image.Image
		format   string
		minFaces int
		identity int
	}{
		{"original", original, "png", 1, 1}, {"different-person", camera, "png", 1, 2}, {"dark", dark, "png", 1, 1}, {"small", small, "png", 1, 1}, {"tilt10", tilted, "png", 1, 1}, {"group", group, "png", 2, 0}, {"jpeg", original, "jpeg", 1, 1}, {"animated-first-frame", original, "gif", 1, 1}, {"tiff", original, "tiff", 1, 1}, {"exif-rotated", rotated, "jpeg-exif", 1, 1},
	}
	var baseline []float32
	report := []map[string]any{}
	minPositive, maxNegative := 1.0, -1.0
	for _, sample := range samples {
		var buf bytes.Buffer
		switch sample.format {
		case "png":
			err = png.Encode(&buf, sample.im)
		case "jpeg", "jpeg-exif":
			err = jpeg.Encode(&buf, sample.im, &jpeg.Options{Quality: 85})
		case "gif":
			first := image.NewPaletted(original.Bounds(), palette.Plan9)
			draw.FloydSteinberg.Draw(first, first.Bounds(), sample.im, image.Point{})
			// Real multi-frame GIF; the unrelated second frame must never become a face.
			err = gif.EncodeAll(&buf, &gif.GIF{Image: []*image.Paletted{first, image.NewPaletted(original.Bounds(), first.Palette)}, Delay: []int{10, 10}})
		case "tiff":
			err = tiff.Encode(&buf, sample.im, nil)
		}
		if err != nil {
			t.Fatal(err)
		}
		encoded := buf.Bytes()
		if sample.format == "jpeg-exif" {
			meta := []byte("Exif\x00\x00II\x2a\x00\x08\x00\x00\x00\x01\x00\x12\x01\x03\x00\x01\x00\x00\x00\x06\x00\x00\x00\x00\x00\x00\x00")
			header := binary.BigEndian.AppendUint16([]byte{0xff, 0xe1}, uint16(len(meta)+2))
			encoded = append(append(append(append([]byte{}, encoded[:2]...), header...), meta...), encoded[2:]...)
		}
		im, err := canonicalFaceImage(encoded)
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		faces, err := e.DetectFaces(im)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		if len(faces) < sample.minFaces {
			t.Fatalf("%s: %d faces", sample.name, len(faces))
		}
		row := map[string]any{"sample": sample.name, "faces": len(faces), "ms": elapsed.Milliseconds(), "rssMB": procMB("VmRSS"), "peakMB": procMB("VmHWM"), "device": "cpu"}
		if sample.name == "original" {
			baseline = faces[0].Vector
		}
		if sample.identity != 0 && len(faces) == 1 {
			var score float64
			for i, v := range faces[0].Vector {
				score += float64(v * baseline[i])
			}
			row["cosineToOriginal"] = score
			if sample.identity == 1 {
				minPositive = min(minPositive, score)
			} else {
				maxNegative = max(maxNegative, score)
			}
		}
		report = append(report, row)
		t.Logf("%v", row)
		if dir := os.Getenv("PHOTOLY_FACE_REPORT"); dir != "" {
			raw, _ := json.Marshal(map[string]any{"faces": faces})
			os.WriteFile(filepath.Join(dir, sample.name+".json"), raw, 0644)
			os.WriteFile(filepath.Join(dir, sample.name+".rgb"), im.Bytes(), 0644)
		}
	}
	if minPositive < faceMatchThreshold || maxNegative >= faceMatchThreshold {
		t.Fatalf("profile fails smoke calibration: positive %.4f negative %.4f", minPositive, maxNegative)
	}
	t.Logf("smoke calibration: minimum positive %.6f maximum negative %.6f; threshold %.2f margin %.2f", minPositive, maxNegative, faceMatchThreshold, faceMatchMargin)
	if dir := os.Getenv("PHOTOLY_FACE_REPORT"); dir != "" {
		raw, _ := json.MarshalIndent(map[string]any{"fingerprint": faceFingerprint, "samples": report, "minPositive": minPositive, "maxNegative": maxNegative, "threshold": faceMatchThreshold, "margin": faceMatchMargin, "limitation": "Two identities with synthetic same-photo derivatives; not an album accuracy benchmark."}, "", "  ")
		os.WriteFile(filepath.Join(dir, "evaluation.json"), raw, 0644)
	}
}
