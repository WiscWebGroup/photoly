package resources

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type cpu struct{ total, idle uint64 }

func parseCPU(raw string) (cpu, error) {
	line, _, _ := strings.Cut(raw, "\n")
	fields := strings.Fields(line)
	if len(fields) < 9 || fields[0] != "cpu" {
		return cpu{}, fmt.Errorf("CPU counters unavailable")
	}
	var c cpu
	for i, s := range fields[1:9] {
		n, e := strconv.ParseUint(s, 10, 64)
		if e != nil {
			return cpu{}, e
		}
		c.total += n
		if i == 3 || i == 4 {
			c.idle += n
		}
	}
	return c, nil
}
func cpuPercent(prev, cur cpu) *float64 {
	if cur.total <= prev.total || cur.idle < prev.idle {
		return nil
	}
	total, idle := cur.total-prev.total, cur.idle-prev.idle
	if idle > total {
		return nil
	}
	v := 100 * float64(total-idle) / float64(total)
	return &v
}
func readRSS(pid string) *int64 {
	raw, e := os.ReadFile("/proc/" + pid + "/status")
	if e != nil {
		return nil
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			f := strings.Fields(line)
			if len(f) != 3 {
				return nil
			}
			n, e := strconv.ParseInt(f[1], 10, 64)
			if e != nil || n < 0 || n > 1<<50 {
				return nil
			}
			n *= 1024
			return &n
		}
	}
	return nil
}

// WorkerRSS is a read-only OS query; nil means unavailable, not zero.
func WorkerRSS(pid int) *int64 { return readRSS(strconv.Itoa(pid)) }

type Snapshot struct {
	SampledAt time.Time `json:"sampledAt"`
	Scope     string    `json:"scope"`
	CPU       *float64  `json:"cpuPercent"`
	Backend   *int64    `json:"backendBytes"`
	AI        *int64    `json:"aiBytes"`
}
