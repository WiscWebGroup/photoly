//go:build !linux

package ai

// Development hosts (macOS): no /proc, so report an idle, roomy machine instead of
// blocking background work forever. Production images are Linux.
// ponytail: fake numbers off Linux; read sysctl/host_statistics if Mac servers matter.

import "time"

func readMemory() (total, avail int64, ok bool) { return 1 << 40, 1 << 40, true }

func readCPU() (cpuTimes, float64, bool) {
	return cpuTimes{at: time.Now(), total: float64(time.Now().UnixNano()), idle: float64(time.Now().UnixNano()), usage: -1}, 1, true
}
