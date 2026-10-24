package ai

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// Vectors come from the browser, so the server cannot prove which model produced them.
// It can reject anything that is not shaped like a SigLIP2 embedding: the right length,
// finite values and unit length (the client L2-normalizes before sending).
const normTolerance = 1e-3

var errVector = errors.New("invalid embedding")

func validateEmbedding(v []float32) error {
	if len(v) != dimension {
		return fmt.Errorf("%w: %d values, want %d", errVector, len(v), dimension)
	}
	var sum float64
	for i, x := range v {
		f := float64(x)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Errorf("%w: value %d is not finite", errVector, i)
		}
		sum += f * f
	}
	if norm := math.Sqrt(sum); math.Abs(norm-1) > normTolerance {
		return fmt.Errorf("%w: length %.6f, want 1", errVector, norm)
	}
	return nil
}

// unitVector L2-normalizes a model output into a new fp32 slice.
func unitVector(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	norm := math.Sqrt(sum)
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = float32(float64(x) / norm)
	}
	return out
}

// LoadInfo reports a server model's load and warm-up time.
type LoadInfo struct {
	Loaded  bool    `json:"loaded"`
	Variant string  `json:"variant,omitempty"` // fp32, or int8 for the smaller text model
	LoadMs  float64 `json:"loadMs,omitempty"`
	WarmMs  float64 `json:"warmMs,omitempty"`
	// Device is the provider the session runs on; DeviceNote says why it isn't the requested one.
	Device     string `json:"device,omitempty"`
	DeviceNote string `json:"deviceNote,omitempty"`
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
