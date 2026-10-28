//go:build !cgo

package ai

// Without cgo there is no ONNX Runtime: the binary stays fully static, the AI worker
// reports why it can't start, and clients embed in the browser only.

import (
	"context"
	"errors"
	"time"
)

type Embedder struct{}

var errNoCgo = errors.New("this binary was built without cgo, so it cannot run ONNX Runtime")

func newEmbedder(*ModelCache, Profile) (*Embedder, error) { return nil, errNoCgo }

func (*Embedder) Load(context.Context, string, string) (LoadInfo, error) { return LoadInfo{}, errNoCgo }
func (*Embedder) SetThreads(int)                                         {}
func (*Embedder) SetDevice(string)                                       {}
func (*Embedder) ProbeDevices() map[string]string                        { return nil }
func (*Embedder) Unload(string) error                                    { return errNoCgo }
func (*Embedder) Status() map[string]LoadInfo                            { return nil }
func (*Embedder) EmbedImage([]float32) ([]float32, error)                { return nil, errNoCgo }
func (*Embedder) EmbedText(string) ([]float32, time.Duration, time.Duration, error) {
	return nil, 0, 0, errNoCgo
}
