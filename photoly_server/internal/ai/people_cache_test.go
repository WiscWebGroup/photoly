package ai

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCropCacheCoalescesAndInvalidates(t *testing.T) {
	var cache cropCache
	var calls atomic.Int32
	generate := func() ([]byte, error) { calls.Add(1); time.Sleep(15 * time.Millisecond); return []byte("crop"), nil }
	var wg sync.WaitGroup
	start := time.Now()
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := cache.get(context.Background(), "user:face:v1", generate)
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	cold := time.Since(start)
	if calls.Load() != 1 {
		t.Fatal("duplicate decodes", calls.Load())
	}
	start = time.Now()
	for range 20 {
		cache.get(context.Background(), "user:face:v1", generate)
	}
	t.Logf("20 avatar requests: cold=%s warm=%s decodes=%d", cold, time.Since(start), calls.Load())
	cache.clear()
	cache.get(context.Background(), "user:face:v1", generate)
	if calls.Load() != 2 {
		t.Fatal("clear retained crop")
	}
	e := cache.entries["user:face:v1"]
	e.expires = time.Now().Add(-time.Second)
	cache.entries["user:face:v1"] = e
	cache.get(context.Background(), "user:face:v1", generate)
	if calls.Load() != 3 {
		t.Fatal("expired crop reused")
	}
}
func TestCropCacheBoundAndAccountIsolation(t *testing.T) {
	var cache cropCache
	generate := func() ([]byte, error) { return make([]byte, 20<<20), nil }
	cache.get(context.Background(), "user1:face", generate)
	cache.get(context.Background(), "user2:face", generate)
	if cache.bytes > cropBudget || len(cache.entries) != 1 {
		t.Fatal("unbounded cache")
	}
	if _, ok := cache.entries["user1:face"]; ok {
		t.Fatal("did not evict oldest")
	}
}
