//go:build cgo

package ai

import (
	"context"
	"fmt"
	"math"
	"time"

	ort "github.com/yalue/onnxruntime_go"
)

func (e *Embedder) loadFaceModels(ctx context.Context) (LoadInfo, error) {
	m := &e.face
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session != nil {
		return m.info, nil
	}
	start := time.Now()
	paths := map[string]string{}
	for _, kind := range []string{"detector", "embedder"} {
		p, err := ensureFaceModel(ctx, kind)
		if err != nil {
			return LoadInfo{}, err
		}
		paths[kind] = p
	}
	opts, err := ort.NewSessionOptions()
	if err != nil {
		return LoadInfo{}, err
	}
	defer func() {
		if opts != nil {
			opts.Destroy()
		}
	}()
	if e.threads > 0 {
		opts.SetIntraOpNumThreads(e.threads)
	}
	used, note := "cpu", ""
	if e.device != "" && e.device != "cpu" {
		if err = useDevice(opts, e.device); err != nil {
			note = err.Error()
			opts.Destroy()
			opts, err = ort.NewSessionOptions()
			if err != nil {
				return LoadInfo{}, err
			}
			if e.threads > 0 {
				opts.SetIntraOpNumThreads(e.threads)
			}
		} else {
			used = e.device
		}
	}
	detector, err := ort.NewDynamicAdvancedSession(paths["detector"], []string{"input"}, faceOutputs, opts)
	if err != nil {
		return LoadInfo{}, err
	}
	embedder, err := ort.NewDynamicAdvancedSession(paths["embedder"], []string{"data"}, []string{"fc1"}, opts)
	if err != nil {
		detector.Destroy()
		return LoadInfo{}, err
	}
	e.faceDetector = detector
	m.session = embedder
	m.info = LoadInfo{Loaded: true, Variant: "fp32", Device: used, DeviceNote: note, LoadMs: ms(time.Since(start))}
	return m.info, nil
}
func (e *Embedder) DetectFaces(im FaceImage) ([]FaceDetection, error) {
	e.face.mu.Lock()
	defer e.face.mu.Unlock()
	if e.face.session == nil || e.faceDetector == nil {
		return nil, errNotLoaded
	}
	if im.Width < 1 || im.Height < 1 || im.Width > 1600 || im.Height > 1600 || len(im.RGB) != im.Width*im.Height*3 {
		return nil, fail("invalid_face_image")
	}
	pixels, scale := detectorTensor(im)
	in, err := ort.NewTensor(ort.NewShape(1, 3, 640, 640), pixels)
	if err != nil {
		return nil, err
	}
	defer in.Destroy()
	out := make([]ort.Value, len(faceOutputs))
	defer func() {
		for _, o := range out {
			if o != nil {
				o.Destroy()
			}
		}
	}()
	if err = e.faceDetector.Run([]ort.Value{in}, out); err != nil {
		return nil, err
	}
	arrays := make([][]float32, len(out))
	for i, o := range out {
		t, ok := o.(*ort.Tensor[float32])
		if !ok {
			return nil, fmt.Errorf("unexpected face output")
		}
		arrays[i] = t.GetData()
	}
	faces := decodeFaces(arrays, im, scale)
	result := []FaceDetection{}
	for _, f := range faces {
		tensor := alignedFaceTensor(im, f.Landmarks)
		if tensor == nil {
			continue
		}
		input, err := ort.NewTensor(ort.NewShape(1, 3, 112, 112), tensor)
		if err != nil {
			return nil, err
		}
		output := []ort.Value{nil}
		err = e.face.session.Run([]ort.Value{input}, output)
		input.Destroy()
		if err != nil {
			return nil, err
		}
		data, ok := output[0].(*ort.Tensor[float32])
		if !ok {
			output[0].Destroy()
			return nil, fmt.Errorf("unexpected SFace output")
		}
		f.Vector = append([]float32{}, data.GetData()...)
		output[0].Destroy()
		var norm float64
		for _, v := range f.Vector {
			norm += float64(v) * float64(v)
		}
		norm = math.Sqrt(norm)
		for i := range f.Vector {
			f.Vector[i] = float32(float64(f.Vector[i]) / norm)
		}
		if validFace(f) {
			result = append(result, f)
		}
	}
	return result, nil
}
