package ai

import (
	"context"
	"encoding/binary"
	"math"
	"sort"
)

// Bound original-image decoding across gallery avatars and browser/server jobs.
// The returned canonical image is at most 7.7 MB; original decodes can be much larger.
var faceDecodeSlots = make(chan struct{}, 2)

func faceDecodeSlot(ctx context.Context) (func(), error) {
	select {
	case faceDecodeSlots <- struct{}{}:
		return func() { <-faceDecodeSlots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Coordinates are normalized against the upright canonical RGB image.
type FaceDetection struct {
	Box       []float32 `json:"box"`
	Landmarks []float32 `json:"landmarks"`
	Score     float32   `json:"score"`
	Vector    []float32 `json:"vector"`
}
type FaceImage struct {
	RGB           []byte
	Width, Height int
}

func canonicalFaceImage(data []byte) (FaceImage, error) {
	rgb, w, h, err := decodeRGB(data)
	if err != nil {
		return FaceImage{}, err
	}
	if max(w, h) > 1600 {
		s := 1600 / float64(max(w, h))
		nw, nh := max(1, int(math.Round(float64(w)*s))), max(1, int(math.Round(float64(h)*s)))
		rgb = resizeBilinear(rgb, w, h, nw, nh)
		w, h = nw, nh
	}
	return FaceImage{rgb, w, h}, nil
}
func (im FaceImage) Bytes() []byte {
	out := make([]byte, 8+len(im.RGB))
	binary.LittleEndian.PutUint32(out, uint32(im.Width))
	binary.LittleEndian.PutUint32(out[4:], uint32(im.Height))
	copy(out[8:], im.RGB)
	return out
}
func detectorTensor(im FaceImage) ([]float32, float64) {
	s := float64(faceDetectorSize) / float64(max(im.Width, im.Height))
	out := make([]float32, 3*faceDetectorSize*faceDetectorSize)
	w, h := int(math.Round(float64(im.Width)*s)), int(math.Round(float64(im.Height)*s))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for c := 0; c < 3; c++ {
				out[c*faceDetectorSize*faceDetectorSize+y*faceDetectorSize+x] = float32(sampleFace(im, (float64(x)+.5)/s-.5, (float64(y)+.5)/s-.5, 2-c, true))
			}
		}
	}
	return out, s
}
func sampleFace(im FaceImage, x, y float64, c int, edge bool) byte {
	if edge {
		x = math.Max(0, math.Min(float64(im.Width-1), x))
		y = math.Max(0, math.Min(float64(im.Height-1), y))
	}
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	dx, dy := x-float64(x0), y-float64(y0)
	at := func(x, y int) float64 {
		if x < 0 || y < 0 || x >= im.Width || y >= im.Height {
			return 0
		}
		return float64(im.RGB[(y*im.Width+x)*3+c])
	}
	v := at(x0, y0)*(1-dx)*(1-dy) + at(x0+1, y0)*dx*(1-dy) + at(x0, y0+1)*(1-dx)*dy + at(x0+1, y0+1)*dx*dy
	return byte(math.Floor(v + .5))
}

var faceOutputs = []string{"cls_8", "cls_16", "cls_32", "obj_8", "obj_16", "obj_32", "bbox_8", "bbox_16", "bbox_32", "kps_8", "kps_16", "kps_32"}

func decodeFaces(outputs [][]float32, im FaceImage, scale float64) []FaceDetection {
	candidates := []FaceDetection{}
	for level, stride := range []int{8, 16, 32} {
		cls, obj, box, kps := outputs[level], outputs[level+3], outputs[level+6], outputs[level+9]
		cols := faceDetectorSize / stride
		for i := 0; i < len(cls); i++ {
			score := float32(math.Sqrt(float64(max(0, min(1, cls[i])) * max(0, min(1, obj[i])))))
			if score < .8 {
				continue
			}
			x, y := float64(i%cols), float64(i/cols)
			st := float64(stride)
			w, h := math.Exp(float64(box[i*4+2]))*st/scale, math.Exp(float64(box[i*4+3]))*st/scale
			cx, cy := (x+float64(box[i*4]))*st/scale, (y+float64(box[i*4+1]))*st/scale
			x1, y1 := math.Max(0, cx-w/2), math.Max(0, cy-h/2)
			x2, y2 := math.Min(float64(im.Width), cx+w/2), math.Min(float64(im.Height), cy+h/2)
			if x2-x1 < 12 || y2-y1 < 12 {
				continue
			}
			f := FaceDetection{Box: []float32{float32(x1 / float64(im.Width)), float32(y1 / float64(im.Height)), float32((x2 - x1) / float64(im.Width)), float32((y2 - y1) / float64(im.Height))}, Score: score, Landmarks: make([]float32, 10)}
			valid := true
			for j := 0; j < 5; j++ {
				f.Landmarks[j*2] = float32((x + float64(kps[i*10+j*2])) * st / scale / float64(im.Width))
				f.Landmarks[j*2+1] = float32((y + float64(kps[i*10+j*2+1])) * st / scale / float64(im.Height))
				if f.Landmarks[j*2] < 0 || f.Landmarks[j*2] > 1 || f.Landmarks[j*2+1] < 0 || f.Landmarks[j*2+1] > 1 {
					valid = false
				}
			}
			if valid {
				candidates = append(candidates, f)
			}
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].Score > candidates[j].Score })
	keep := []FaceDetection{}
	for _, f := range candidates {
		overlap := false
		for _, k := range keep {
			if faceIoU(f.Box, k.Box) > .3 {
				overlap = true
				break
			}
		}
		if !overlap {
			keep = append(keep, f)
			if len(keep) == faceMaxCount {
				break
			}
		}
	}
	return keep
}
func faceIoU(a, b []float32) float32 {
	x, y := max(a[0], b[0]), max(a[1], b[1])
	w, h := max(float32(0), min(a[0]+a[2], b[0]+b[2])-x), max(float32(0), min(a[1]+a[3], b[1]+b[3])-y)
	i := w * h
	return i / max(float32(1e-9), a[2]*a[3]+b[2]*b[3]-i)
}

var faceTarget = [10]float64{38.2946, 51.6963, 73.5318, 51.5014, 56.0252, 71.7366, 41.5493, 92.3655, 70.7299, 92.2041}

// Least-squares similarity transform (rotation + uniform scale, no reflection).
func alignedFaceTensor(im FaceImage, landmarks []float32) []float32 {
	var sx, sy, dx, dy float64
	for i := 0; i < 5; i++ {
		sx += float64(landmarks[2*i]) * float64(im.Width) / 5
		sy += float64(landmarks[2*i+1]) * float64(im.Height) / 5
		dx += faceTarget[2*i] / 5
		dy += faceTarget[2*i+1] / 5
	}
	var a, b, den float64
	for i := 0; i < 5; i++ {
		x, y := float64(landmarks[2*i])*float64(im.Width)-sx, float64(landmarks[2*i+1])*float64(im.Height)-sy
		u, v := faceTarget[2*i]-dx, faceTarget[2*i+1]-dy
		a += x*u + y*v
		b += x*v - y*u
		den += x*x + y*y
	}
	if den < 1e-9 {
		return nil
	}
	a /= den
	b /= den
	norm := a*a + b*b
	if norm < 1e-9 {
		return nil
	}
	tx, ty := dx-a*sx+b*sy, dy-b*sx-a*sy
	out := make([]float32, 3*112*112)
	for y := 0; y < 112; y++ {
		for x := 0; x < 112; x++ {
			u, v := float64(x)-tx, float64(y)-ty
			px, py := (a*u+b*v)/norm, (-b*u+a*v)/norm
			for c := 0; c < 3; c++ {
				out[c*112*112+y*112+x] = float32(sampleFace(im, px, py, c, false))
			}
		}
	}
	return out
}
func validFace(f FaceDetection) bool {
	if len(f.Box) != 4 || len(f.Landmarks) != 10 || len(f.Vector) != faceDimension || !finiteFace(f.Score) || f.Score < .8 || f.Score > 1 {
		return false
	}
	for _, v := range append(append([]float32{}, f.Box...), f.Landmarks...) {
		if !finiteFace(v) || v < 0 || v > 1 {
			return false
		}
	}
	if f.Box[2] <= 0 || f.Box[3] <= 0 || f.Box[0]+f.Box[2] > 1.00001 || f.Box[1]+f.Box[3] > 1.00001 {
		return false
	}
	sum := float64(0)
	for _, v := range f.Vector {
		if !finiteFace(v) {
			return false
		}
		sum += float64(v) * float64(v)
	}
	return sum > .98 && sum < 1.02
}
func finiteFace(v float32) bool { return !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0) }
