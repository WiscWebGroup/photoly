package ai

import "testing"

func TestCaptureTimeAndDevice(t *testing.T) {
	d := map[string]any{"DateTimeOriginal": "2024:05:01 10:00:00", "OffsetTimeOriginal": "+02:00"}
	at, ev := captureTime(d, "", "America/New_York")
	if at == nil || at.Format("15:04") != "08:00" || ev["timezoneSource"] != "explicit" || ev["timezone"] != "UTC+02:00" {
		t.Fatalf("offset: %v %v", at, ev)
	}
	at, ev = captureTime(map[string]any{"DateTimeOriginal": "2024:05:01 10:00:00"}, "Asia/Tokyo", "America/New_York")
	if at == nil || at.Format("15:04") != "01:00" || ev["timezoneSource"] != "city" {
		t.Fatalf("city: %v %v", at, ev)
	}
	if at, _ = captureTime(map[string]any{"DateTimeOriginal": "2024:03:10 02:30:00"}, "", "America/New_York"); at != nil {
		t.Fatal("nonexistent wall time accepted")
	}
	if got := deviceName("Apple", "Apple iPhone 15"); got != "iPhone 15" {
		t.Fatalf("device %v", got)
	}
	if deviceName("Canon", " ") != nil {
		t.Fatal("blank model")
	}
}

func TestGazetteer(t *testing.T) {
	g, err := loadGazetteer()
	if err != nil || len(g.cities) < 30000 {
		t.Fatalf("load: %v %d", err, len(g.cities))
	}
	if c := g.nearest(-54.81, -68.31); c == nil || c.Name != "Ushuaia" {
		t.Fatalf("nearest: %+v", c)
	}
	if g.nearest(0, -30) != nil {
		t.Fatal("mid-ocean should have no city")
	}
}
