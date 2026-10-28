package catalog

import "testing"

// Expected values come from app.places.index().nearest() in the Python backend.
func TestPlacesParity(t *testing.T) {
	cases := []struct {
		lat, lon       float64
		place, country string
	}{
		{40.7128, -74.0060, "New York City", "United States"}, {48.8566, 2.3522, "Paris", "France"},
		{35.6762, 139.6503, "Tokyo", "Japan"}, {-33.8688, 151.2093, "Sydney", "Australia"},
		{51.5, -0.12, "London", "United Kingdom"}, {64.2, -51.7, "Nuuk", "Greenland"},
		{37.7749, -122.4194, "San Francisco", "United States"}, {21.3, -157.85, "Honolulu", "United States"},
		{55.75, 37.62, "Moscow", "Russia"}, {0, -160, "", ""}, {-77.8, 166.7, "", ""}, {40.7, 179.99, "", ""}, {10, -179.99, "", ""},
	}
	for _, c := range cases {
		lat, lon := c.lat, c.lon
		got := places().nearest(&lat, &lon)
		if c.place == "" {
			if got != nil {
				t.Errorf("(%v,%v) = %v, want none", c.lat, c.lon, *got)
			}
		} else if got == nil || got.Place != c.place || got.Country != c.country {
			t.Errorf("(%v,%v) = %v, want %s/%s", c.lat, c.lon, got, c.place, c.country)
		}
	}
}
