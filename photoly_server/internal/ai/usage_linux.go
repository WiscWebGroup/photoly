//go:build linux

package ai

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func readMemory() (total, avail int64, ok bool) {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		kb, _ := strconv.ParseInt(f[1], 10, 64)
		switch f[0] {
		case "MemTotal:":
			total = kb << 10
		case "MemAvailable:":
			avail = kb << 10
		}
	}
	if limit, err := readInt("/sys/fs/cgroup/memory.max"); err == nil {
		used, _ := readInt("/sys/fs/cgroup/memory.current")
		total = min(total, limit)
		avail = min(avail, max(0, limit-used))
	}
	return total, avail, total > 0
}

func readCPU() (cpuTimes, float64, bool) {
	raw, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuTimes{}, 0, false
	}
	line, _, _ := strings.Cut(string(raw), "\n")
	f := strings.Fields(line)
	if len(f) < 6 || f[0] != "cpu" {
		return cpuTimes{}, 0, false
	}
	var total, idle float64
	for i, s := range f[1:] {
		v, _ := strconv.ParseFloat(s, 64)
		if i < 8 { // guest columns are already counted in user/nice
			total += v
		}
		if i == 3 || i == 4 { // idle, iowait
			idle += v
		}
	}
	c := cpuTimes{at: time.Now(), total: total, idle: idle, usage: -1}
	capacity := float64(runtime.NumCPU())
	if stat, err := os.ReadFile("/sys/fs/cgroup/cpu.stat"); err == nil {
		for _, l := range strings.Split(string(stat), "\n") {
			if v, found := strings.CutPrefix(l, "usage_usec "); found {
				us, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
				c.usage = us / 1e6
			}
		}
		if mx, err := os.ReadFile("/sys/fs/cgroup/cpu.max"); err == nil {
			if q := strings.Fields(string(mx)); len(q) == 2 && q[0] != "max" {
				quota, _ := strconv.ParseFloat(q[0], 64)
				period, _ := strconv.ParseFloat(q[1], 64)
				if period > 0 {
					capacity = min(capacity, quota/period)
				}
			}
		}
	}
	return c, capacity, true
}
