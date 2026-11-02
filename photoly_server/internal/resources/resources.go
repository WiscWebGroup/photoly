// Package resources collects bounded, admin-only telemetry independently of AI admission.
package resources

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/httpx"
)

type Point struct {
	BootID        string    `json:"bootId"`
	Bucket        time.Time `json:"bucket"`
	First         time.Time `json:"firstSampleAt"`
	Last          time.Time `json:"lastSampleAt"`
	CPUSamples    int       `json:"cpuSamples"`
	MemorySamples int       `json:"memorySamples"`
	CPU           *float64  `json:"cpuMean"`
	CPUPeak       *float64  `json:"cpuPeak"`
	Backend       *float64  `json:"backendMeanBytes"`
	AI            *float64  `json:"aiMeanBytes"`
	MemoryPeak    *float64  `json:"memoryPeakBytes"`
	Partial       bool      `json:"partial"`
}

func ptr(v float64) *float64 { return &v }
func (p *Point) add(s Snapshot) {
	if p.First.IsZero() {
		p.First = s.SampledAt
	}
	p.Last = s.SampledAt
	if s.CPU != nil {
		p.CPUSamples++
		if p.CPU == nil {
			p.CPU = ptr(0)
			p.CPUPeak = ptr(0)
		}
		p.CPU = ptr(*p.CPU + (*s.CPU-*p.CPU)/float64(p.CPUSamples))
		p.CPUPeak = ptr(max(*p.CPUPeak, *s.CPU))
	}
	if s.Backend != nil && s.AI != nil {
		p.MemorySamples++
		if p.Backend == nil {
			p.Backend = ptr(0)
			p.AI = ptr(0)
			p.MemoryPeak = ptr(0)
		}
		p.Backend = ptr(*p.Backend + (float64(*s.Backend)-*p.Backend)/float64(p.MemorySamples))
		p.AI = ptr(*p.AI + (float64(*s.AI)-*p.AI)/float64(p.MemorySamples))
		p.MemoryPeak = ptr(max(*p.MemoryPeak, float64(*s.Backend)+float64(*s.AI)))
	}
}

type collector struct {
	mu      sync.RWMutex
	current Snapshot
	point   Point
	prev    *cpu
	a       *app.App
	pending []Point
}

func (c *collector) sample(now time.Time) {
	s := Snapshot{SampledAt: now.UTC(), Scope: "Host CPU (container-visible Linux host / VM)", Backend: readRSS("self")}
	zero := int64(0)
	s.AI = &zero
	if c.a.WorkerRSS != nil {
		s.AI = c.a.WorkerRSS()
	}
	raw, e := os.ReadFile("/proc/stat")
	if e == nil {
		cur, e := parseCPU(string(raw))
		if e == nil {
			if c.prev != nil {
				s.CPU = cpuPercent(*c.prev, cur)
			}
			c.prev = &cur
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = s
	bucket := s.SampledAt.Truncate(10 * time.Minute)
	if c.point.Bucket != bucket {
		if !c.point.Bucket.IsZero() {
			c.point.Partial = false
			c.pending = append(c.pending, c.point)
		}
		c.point = Point{BootID: c.a.BootID, Bucket: bucket, Partial: true}
	}
	c.point.add(s)
	// Bound pending history even when the database is unavailable for days.
	for len(c.pending) > 0 && c.pending[0].Bucket.Before(now.Add(-48*time.Hour)) {
		c.pending = c.pending[1:]
	}
}
func (c *collector) flush(ctx context.Context, partial bool) {
	c.mu.RLock()
	points := append([]Point(nil), c.pending...)
	if partial && !c.point.Bucket.IsZero() {
		points = append(points, c.point)
	}
	c.mu.RUnlock()
	q := dbgen.New(c.a.Pool)
	for _, p := range points {
		data, e := json.Marshal(p)
		if e == nil {
			e = q.SaveResourceHistory(ctx, dbgen.SaveResourceHistoryParams{BootID: p.BootID, Bucket: p.Bucket, Payload: data})
		}
		if e != nil {
			log.Printf("resource history: %v", e)
			return
		}
		c.mu.Lock()
		if len(c.pending) > 0 && c.pending[0].Bucket.Equal(p.Bucket) {
			c.pending = c.pending[1:]
		}
		c.mu.Unlock()
	}
	if e := q.PruneResourceHistory(ctx, time.Now().Add(-48*time.Hour)); e != nil {
		log.Printf("resource history cleanup: %v", e)
	}
}
func Register(a *app.App) {
	c := &collector{a: a}
	a.ResourceRun = func(ctx context.Context) {
		c.sample(time.Now())
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			c.flush(ctx, true)
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				c.sample(now)
				c.mu.RLock()
				pending := len(c.pending) > 0
				c.mu.RUnlock()
				if pending {
					write, cancel := context.WithTimeout(ctx, 3*time.Second)
					c.flush(write, false)
					cancel()
				}
			}
		}
	}
	a.Mux.HandleFunc("GET /admin/runtime", a.Admin(func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, map[string]any{"startedAt": a.StartedAt, "uptimeSeconds": max(0, int64(time.Since(a.StartedAt).Seconds())), "bootId": a.BootID}, 200)
	}))
	a.Mux.HandleFunc("GET /admin/resources/current", a.Admin(func(w http.ResponseWriter, r *http.Request) {
		c.mu.RLock()
		defer c.mu.RUnlock()
		w.Header().Set("Cache-Control", "no-store")
		httpx.JSON(w, c.current, 200)
	}))
	a.Mux.HandleFunc("GET /admin/resources/history", a.Admin(func(w http.ResponseWriter, r *http.Request) {
		hours := 24
		switch r.URL.Query().Get("hours") {
		case "", "24":
		case "12":
			hours = 12
		default:
			w.WriteHeader(400)
			httpx.JSON(w, "invalid_hours", 400)
			return
		}
		since := time.Now().Add(-time.Duration(hours) * time.Hour).Truncate(10 * time.Minute)
		rows, e := dbgen.New(a.Pool).ListResourceHistory(r.Context(), since)
		if e != nil {
			w.WriteHeader(503)
			httpx.JSON(w, "history_unavailable", 503)
			return
		}
		points := []Point{}
		for _, raw := range rows {
			var p Point
			if json.Unmarshal(raw, &p) == nil {
				points = append(points, p)
			}
		}
		c.mu.RLock()
		live := append([]Point(nil), c.pending...)
		if !c.point.Bucket.IsZero() {
			live = append(live, c.point)
		}
		c.mu.RUnlock()
		for _, p := range live {
			if p.Bucket.Before(since) {
				continue
			}
			found := false
			for i := range points {
				if points[i].BootID == p.BootID && points[i].Bucket.Equal(p.Bucket) {
					points[i] = p
					found = true
					break
				}
			}
			if !found {
				points = append(points, p)
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		httpx.JSON(w, map[string]any{"items": points, "hours": hours, "bucketSeconds": 600}, 200)
	}))
}
func Run(ctx context.Context, a *app.App) {
	if a.ResourceRun != nil {
		a.ResourceRun(ctx)
	}
}
