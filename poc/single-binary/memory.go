package main

// Memory checks before the AI worker loads a model.

import (
	"os"
	"strconv"
	"strings"
)

// modelNeedMB is roughly how much a model adds to the worker's peak memory while it loads
// (largest of several runs on Linux arm64 with ONNX Runtime 1.29, plus ~10%; text
// includes its tokenizer). Peaks are well above what stays in use afterwards: fp32 text
// peaked at +1.85 to +2.04 GB and settles at +0.72 GB.
// ponytail: fixed estimates. Measure the worker's peak on each load and keep the largest
// seen, if they turn out wrong on other machines.
var modelNeedMB = map[string]map[string]int{
	"image": {"fp32": 900},
	"text":  {"fp32": 2250, "int8": 900},
}

// Memory the rest of the server keeps for itself when deciding whether a model fits.
const headroomMB = 150

// availableMB returns memory the worker can still use: the smaller of the container's
// remaining limit and the system's MemAvailable. ok is false where it can't be read
// (not Linux); the check is then skipped.
// ponytail: Linux only. macOS would need host_statistics64 via cgo.
func availableMB() (mb int, ok bool) {
	meminfo, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, false
	}
	avail := -1
	for _, line := range strings.Split(string(meminfo), "\n") {
		if rest, found := strings.CutPrefix(line, "MemAvailable:"); found {
			kb, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(rest), " kB"))
			avail = kb / 1024
		}
	}
	if avail < 0 {
		return 0, false
	}
	// cgroup v2 (Docker, systemd): limit minus usage, not counting reclaimable file cache.
	if limit, err := readInt("/sys/fs/cgroup/memory.max"); err == nil {
		used, _ := readInt("/sys/fs/cgroup/memory.current")
		used -= cgroupStat("inactive_file")
		avail = min(avail, int((limit-used)>>20))
	}
	return avail, true
}

func readInt(path string) (int64, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64) // "max" fails: no limit
}

func cgroupStat(key string) int64 {
	raw, _ := os.ReadFile("/sys/fs/cgroup/memory.stat")
	for _, line := range strings.Split(string(raw), "\n") {
		if v, found := strings.CutPrefix(line, key+" "); found {
			n, _ := strconv.ParseInt(v, 10, 64)
			return n
		}
	}
	return 0
}

// procMB reads a memory field such as "VmRSS" or "VmHWM" (peak) of this process, in MB.
func procMB(field string) int { return pidMB("self", field) }

// workerMB is a process's memory, in RAM or swapped out, in MB; 0 without /proc.
func workerMB(pid string) int { return pidMB(pid, "VmRSS") + pidMB(pid, "VmSwap") }

// pidMB is procMB for another process; 0 where /proc doesn't exist.
func pidMB(pid, field string) int {
	raw, _ := os.ReadFile("/proc/" + pid + "/status")
	for _, line := range strings.Split(string(raw), "\n") {
		if rest, found := strings.CutPrefix(line, field+":"); found {
			kb, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(rest), " kB"))
			return kb / 1024
		}
	}
	return 0
}
