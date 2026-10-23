package main

import (
	"errors"
	"fmt"
	"math"
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
