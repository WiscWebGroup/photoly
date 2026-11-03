package ai

import (
	"context"
	"sync"
	"time"
)

// Crop bytes are ephemeral and bounded. Authorization and source visibility must be
// checked by the caller on every request, even on a hit.
type cropEntry struct {
	data          []byte
	expires, used time.Time
}
type cropFlight struct {
	done chan struct{}
	data []byte
	err  error
}
type cropCache struct {
	mu      sync.Mutex
	entries map[string]cropEntry
	flights map[string]*cropFlight
	bytes   int
	epoch   uint64
}

const cropBudget = 32 << 20
const cropTTL = 10 * time.Minute

func (c *cropCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = nil
	c.bytes = 0
	c.epoch++
	c.flights = nil
}
func (c *cropCache) get(ctx context.Context, key string, generate func() ([]byte, error)) ([]byte, error) {
	c.mu.Lock()
	now := time.Now()
	for k, e := range c.entries {
		if !now.Before(e.expires) {
			delete(c.entries, k)
			c.bytes -= len(e.data)
		}
	}
	if e, ok := c.entries[key]; ok {
		e.used = now
		c.entries[key] = e
		c.mu.Unlock()
		return e.data, nil
	}
	if f, ok := c.flights[key]; ok {
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-f.done:
			return f.data, f.err
		}
	}
	if c.flights == nil {
		c.flights = map[string]*cropFlight{}
	}
	f := &cropFlight{done: make(chan struct{})}
	c.flights[key] = f
	epoch := c.epoch
	c.mu.Unlock()
	data, err := generate()
	c.mu.Lock()
	defer c.mu.Unlock()
	f.data, f.err = data, err
	if c.epoch == epoch {
		delete(c.flights, key)
		if err == nil && len(data) <= cropBudget {
			if c.entries == nil {
				c.entries = map[string]cropEntry{}
			}
			for c.bytes+len(data) > cropBudget {
				var oldest string
				var used time.Time
				for k, e := range c.entries {
					if used.IsZero() || e.used.Before(used) {
						oldest, used = k, e.used
					}
				}
				c.bytes -= len(c.entries[oldest].data)
				delete(c.entries, oldest)
			}
			c.entries[key] = cropEntry{data, time.Now().Add(cropTTL), time.Now()}
			c.bytes += len(data)
		}
	}
	close(f.done)
	return data, err
}
