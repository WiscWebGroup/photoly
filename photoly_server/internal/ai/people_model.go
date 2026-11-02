package ai

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const faceDetectorVersion = "yunet-2023mar-8f2383e4-preprocess-v1"
const faceEmbeddingVersion = "sface-2021dec-0ba9fbfa-rgb-align-v1"
const faceFingerprint = "yunet2023-sface2021-rgb-v1"
const faceRevision = "48b10fa505aeb51fbd127fc60a5c660040e823b7"
const faceDimension = 128
const faceDetectorSize = 640
const faceMaxCount = 100
const faceStorageCharge int64 = 2048

//go:embed peopledata/LICENSE.*
var faceLicenses embed.FS

type faceModelFile struct {
	File      string `json:"file"`
	Directory string `json:"-"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
	License   string `json:"license"`
}

var faceModelFiles = map[string]faceModelFile{
	"detector": {"face_detection_yunet_2023mar.onnx", "face_detection_yunet", "8f2383e4dd3cfbb4553ea8718107fc0423210dc964f9f4280604804ed2552fa4", 232589, "LICENSE.YuNet"},
	"embedder": {"face_recognition_sface_2021dec.onnx", "face_recognition_sface", "0ba9fbfa01b5270c96627c4ef784da859931e02f04419c829e83484087c34e79", 38696353, "LICENSE.SFace"},
}
var faceDownloadMu sync.Mutex

func faceManifest() map[string]any {
	return map[string]any{"fingerprint": faceFingerprint, "detectorVersion": faceDetectorVersion, "embeddingVersion": faceEmbeddingVersion, "revision": faceRevision, "dimension": faceDimension, "detectorSize": faceDetectorSize, "files": faceModelFiles, "baseUrl": "/models/people/" + faceFingerprint + "/", "matchThreshold": faceMatchThreshold, "matchMargin": faceMatchMargin,
		"input":          "upright RGB8, longest edge at most 1600, little-endian uint32 width/height + interleaved RGB",
		"detectorInput":  "1x3x640x640 BGR float32 0..255, top-left letterbox, zero padding",
		"alignment":      "five-landmark similarity transform to OpenCV SFace 112x112 template",
		"embeddingInput": "1x3x112x112 RGB float32 0..255 (normalization inside ONNX)",
		"normalization":  "L2", "groupingVersion": "conservative-v1"}
}
func ensureFaceModel(ctx context.Context, kind string) (string, error) {
	f, ok := faceModelFiles[kind]
	if !ok {
		return "", fail("unknown_face_model")
	}
	faceDownloadMu.Lock()
	defer faceDownloadMu.Unlock()
	dir := filepath.Join(dataDir(), "models", "people", faceFingerprint)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	license, err := faceLicenses.ReadFile("peopledata/" + f.License)
	if err != nil {
		return "", err
	}
	if err = os.WriteFile(filepath.Join(dir, f.License), license, 0644); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, f.File)
	if data, err := os.ReadFile(dst); err == nil {
		sum := sha256.Sum256(data)
		if int64(len(data)) == f.Size && hex.EncodeToString(sum[:]) == f.SHA256 {
			return dst, nil
		}
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://media.githubusercontent.com/media/opencv/opencv_zoo/"+faceRevision+"/models/"+f.Directory+"/"+f.File, nil)
	if err != nil {
		return "", err
	}
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("face model download: HTTP %d", resp.StatusCode)
	}
	tmp, err := os.CreateTemp(dir, "download-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(resp.Body, f.Size+1))
	ce := tmp.Close()
	if err != nil {
		return "", err
	}
	if ce != nil {
		return "", ce
	}
	if n != f.Size || hex.EncodeToString(h.Sum(nil)) != f.SHA256 {
		return "", fail("face_model_checksum")
	}
	if err = os.Rename(tmp.Name(), dst); err != nil {
		return "", err
	}
	return dst, nil
}
