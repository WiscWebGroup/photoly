package uploads

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/photo"
)

// Outcome of storing a browser-computed embedding.
const (
	Stored  = "stored"  // vector saved with source='client', state done
	Queued  = "queued"  // rejected or no sink: the server queue (photo_changed hook) embeds it
	Waiting = "waiting" // rejected and server placement is off: stays pending for a browser
	Held    = "held"    // photo not imported yet; applied after install
)

// EmbeddingSink is implemented by the AI package (set uploads.Sink at init).
type EmbeddingSink interface {
	// StoreClient stores the vector for an installed photo under the active profile. It returns
	// Stored, or Queued/Waiting when the fingerprint or content no longer matches.
	StoreClient(ctx context.Context, userID, photoID int64, fingerprint string, vector []float32) (string, error)
}

var Sink EmbeddingSink

// ValidVector is the 768-dim finite unit-length check.
func ValidVector(v []float32) bool {
	if len(v) != 768 {
		return false
	}
	var sum float64
	for _, x := range v {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return false
		}
		sum += float64(x) * float64(x)
	}
	return math.Abs(math.Sqrt(sum)-1) <= 1e-2
}

// StoreClientEmbedding validates and hands the vector to the sink; without a sink the photo stays
// on the server queue.
func StoreClientEmbedding(ctx context.Context, userID, photoID int64, fingerprint string, vector []float32) (string, error) {
	if fingerprint == "" || !ValidVector(vector) {
		return "", photo.Err("invalid_embedding", 400)
	}
	if Sink == nil {
		return Queued, nil
	}
	return Sink.StoreClient(ctx, userID, photoID, fingerprint, vector)
}

type heldVector struct {
	fingerprint string
	vector      []float32
	at          time.Time
}

var (
	heldMu sync.Mutex
	held   = map[string]heldVector{}
)

// ponytail: process-local; a restart drops held vectors and the server queue embeds them instead.
func hold(item, fingerprint string, v []float32) {
	heldMu.Lock()
	defer heldMu.Unlock()
	for k, h := range held {
		if time.Since(h.at) > time.Hour {
			delete(held, k)
		}
	}
	held[item] = heldVector{fingerprint, v, time.Now()}
}

func takeHeld(item string) (heldVector, bool) {
	heldMu.Lock()
	defer heldMu.Unlock()
	h, ok := held[item]
	delete(held, item)
	return h, ok
}
