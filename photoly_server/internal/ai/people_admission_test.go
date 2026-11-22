package ai

import "testing"

// Browser placement infers on the client, so server CPU/RAM pressure must not block it.
func TestPeopleBlockersIgnoreServerLoadForBrowser(t *testing.T) {
	cfg := Settings{"ram_policy": "free", "memory_pause_free_mib": int64(1 << 40), "cpu_pause_percent": int64(-1), "disk_pause_free_mib": int64(0)}
	p := &People{root: t.TempDir()}
	if got := p.blockers(cfg, "server"); len(got) == 0 {
		t.Fatal("server placement should be blocked by CPU and memory pressure")
	}
	if got := p.blockers(cfg, "browser"); len(got) != 0 {
		t.Fatalf("browser placement blocked by %v", got)
	}
	cfg["disk_pause_free_mib"] = int64(1 << 40)
	if got := p.blockers(cfg, "browser"); len(got) != 1 || got[0] != "disk_pressure" {
		t.Fatalf("browser placement should still honor disk pressure, got %v", got)
	}
}
