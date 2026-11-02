package resources

import (
	"testing"
	"time"
)

func TestCPUCounterDelta(t *testing.T) {
	prev, e := parseCPU("cpu 10 0 10 70 10 0 0 0 999 999\nother")
	if e != nil {
		t.Fatal(e)
	}
	cur, _ := parseCPU("cpu 20 0 20 90 10 0 0 0 1000 1000")
	if got := cpuPercent(prev, cur); got == nil || *got != 50 {
		t.Fatal(got)
	}
	for _, c := range []cpu{prev, {total: 1, idle: 1}, {total: 200, idle: 10}, {total: 101, idle: 100}} {
		if cpuPercent(prev, c) != nil {
			t.Fatal("accepted reset/invalid counters", c)
		}
	}
	if _, e := parseCPU("cpu 1 bad 3 4 5 6 7 8"); e == nil {
		t.Fatal("accepted corrupt counters")
	}
}
func TestHistoryAggregationPreservesSnapshotsAndMissingValues(t *testing.T) {
	p := Point{}
	now := time.Now()
	backend, ai := int64(100), int64(20)
	p.add(Snapshot{SampledAt: now, CPU: ptr(25), Backend: &backend, AI: &ai})
	old := p
	backend = 200
	ai = 0
	p.add(Snapshot{SampledAt: now.Add(10 * time.Second), CPU: ptr(75), Backend: &backend, AI: &ai})
	p.add(Snapshot{SampledAt: now.Add(20 * time.Second)})
	if *p.CPU != 50 || *p.CPUPeak != 75 || *p.Backend != 150 || *p.AI != 10 || *p.MemoryPeak != 200 || p.CPUSamples != 2 || p.MemorySamples != 2 {
		t.Fatal(p)
	}
	if *old.CPU != 25 || *old.Backend != 100 {
		t.Fatal("reader snapshot mutated")
	}
	if p.First != now || p.Last != now.Add(20*time.Second) {
		t.Fatal(p)
	}
	if WorkerRSS(-12345) != nil {
		t.Fatal("missing process reported as zero")
	}
}
