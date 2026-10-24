package ai

// System usage snapshot and background admission policy
// (app/util/sys_usage_util.py, scheduler/resource_policy.py).

import (
	"sync"
	"syscall"
	"time"
)

type Usage struct {
	SampledAt       time.Time
	CPUPercent      *float64
	MemoryTotal     int64
	MemoryAvailable int64
	DiskFree        int64
	Missing         []string
}

func (u Usage) toMap() map[string]any {
	return map[string]any{"sampled_at": u.SampledAt.UTC().Format(time.RFC3339Nano), "cpu_percent": u.CPUPercent,
		"memory_total": u.MemoryTotal, "memory_available": u.MemoryAvailable, "disk_free": u.DiskFree,
		"gpu_percent": nil, "vram_available": nil, "vram_total": nil, "missing": u.Missing}
}

func diskFree(path string) int64 {
	var st syscall.Statfs_t
	if syscall.Statfs(path, &st) != nil {
		return 0
	}
	return int64(st.Bavail) * int64(st.Bsize)
}

type cpuTimes struct {
	at          time.Time
	total, idle float64
	usage       float64 // cgroup CPU seconds, -1 if unknown
}

var (
	cpuMu   sync.Mutex
	cpuPrev *cpuTimes
)

// sampleCPU returns the larger of host and container CPU use since the previous call, so
// it is shared by every caller (nil on the first call or where unknown).
func sampleCPU() *float64 {
	cur, capacity, ok := readCPU()
	if !ok {
		return nil
	}
	cpuMu.Lock()
	prev := cpuPrev
	cpuPrev = &cur
	cpuMu.Unlock()
	if prev == nil {
		return nil
	}
	var best float64 = -1
	if d := cur.total - prev.total; d > 0 {
		best = max(best, max(0, min(100, 100*(1-(cur.idle-prev.idle)/d))))
	}
	if el := cur.at.Sub(prev.at).Seconds(); cur.usage >= 0 && prev.usage >= 0 && capacity > 0 && el > 0 {
		best = max(best, max(0, min(100, 100*(cur.usage-prev.usage)/el/capacity)))
	}
	if best < 0 {
		return nil
	}
	return &best
}

// SystemUsage samples the machine (path: the disk to report free space for).
func SystemUsage(path string) Usage {
	total, avail, ok := readMemory()
	u := Usage{SampledAt: time.Now(), MemoryTotal: total, MemoryAvailable: avail, DiskFree: diskFree(path)}
	if !ok {
		u.Missing = append(u.Missing, "memory")
	}
	if u.CPUPercent = sampleCPU(); u.CPUPercent == nil {
		u.Missing = append(u.Missing, "cpu_first_sample")
	}
	return u
}

// Policy decides whether background work may start; hysteresis via blocked (ResourcePolicy).
type Policy struct {
	mu      sync.Mutex
	blocked bool
	cpuAvg  *float64
}

type Decision struct {
	Allowed bool
	Reasons []string
}

func (p *Policy) Decide(u map[string]any, limits Settings) Decision {
	p.mu.Lock()
	defer p.mu.Unlock()
	mode := "pause"
	if p.blocked {
		mode = "resume"
	}
	var reasons []string
	at, _ := u["sampled_at"].(string)
	if t, err := time.Parse(time.RFC3339Nano, at); err != nil {
		reasons = append(reasons, "missing_metrics")
	} else if time.Since(t) > 15*time.Second {
		reasons = append(reasons, "stale_metrics")
	}
	cpu, hasCPU := toFloat(u["cpu_percent"])
	if cp, ok := u["cpu_percent"].(*float64); ok && cp != nil {
		cpu, hasCPU = *cp, true
	}
	switch {
	case !hasCPU:
		p.cpuAvg = nil
	case p.cpuAvg == nil:
		v := cpu
		p.cpuAvg = &v
	default:
		v := 0.7**p.cpuAvg + 0.3*cpu
		p.cpuAvg = &v
	}
	if p.cpuAvg == nil || *p.cpuAvg > float64(limits.Int("cpu_"+mode+"_percent")) {
		reasons = append(reasons, "cpu_busy_or_unknown")
	}
	total, _ := toInt(u["memory_total"])
	avail, _ := toInt(u["memory_available"])
	if limits.Str("ram_policy") == "percent" {
		maximum := float64(limits.Int("ram_max_percent"))
		if p.blocked {
			maximum -= 5
		}
		if total == 0 || 100*(1-float64(avail)/float64(total)) > maximum {
			reasons = append(reasons, "memory_pressure")
		}
	} else if avail < limits.Int("memory_"+mode+"_free_mib")<<20 {
		reasons = append(reasons, "memory_pressure")
	}
	if free, _ := toInt(u["disk_free"]); free < limits.Int("disk_"+mode+"_free_mib")<<20 {
		reasons = append(reasons, "disk_pressure")
	}
	p.blocked = len(reasons) > 0
	return Decision{!p.blocked, reasons}
}
