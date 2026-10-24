package ai

// Smart Tag metadata: gazetteer, EXIF facts and capture time (smart/{geography,metadata}.py).

import (
	"bufio"
	"compress/gzip"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

//go:embed smartdata/cities.tsv.gz
var cityData embed.FS

type city struct {
	ID, Name, Zone string
	Lat, Lon       float64
}

type gazetteer struct{ cities []city } // sorted by latitude

func loadGazetteer() (*gazetteer, error) {
	f, err := cityData.Open("smartdata/cities.tsv.gz")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	g := &gazetteer{}
	sc := bufio.NewScanner(z)
	for sc.Scan() {
		p := strings.Split(sc.Text(), "\t")
		if len(p) < 5 {
			continue
		}
		lat, e1 := strconv.ParseFloat(p[2], 64)
		lon, e2 := strconv.ParseFloat(p[3], 64)
		if e1 != nil || e2 != nil {
			continue
		}
		g.cities = append(g.cities, city{p[0], p[1], p[4], lat, lon})
	}
	sort.SliceStable(g.cities, func(i, j int) bool { return g.cities[i].Lat < g.cities[j].Lat })
	return g, sc.Err()
}

func distance(lat, lon, olat, olon float64) float64 {
	a, b, d := lat*math.Pi/180, olat*math.Pi/180, (olon-lon)*math.Pi/180
	h := math.Pow(math.Sin((b-a)/2), 2) + math.Cos(a)*math.Cos(b)*math.Pow(math.Sin(d/2), 2)
	return 6371.0088 * 2 * math.Asin(math.Min(1, math.Sqrt(math.Max(0, h))))
}

// nearest is the closest city within 100 km, or nil.
func (g *gazetteer) nearest(lat, lon float64) *city {
	radius := 100 / 111.0
	lonRadius := math.Min(180, radius/math.Max(0.001, math.Cos(lat*math.Pi/180)))
	start := sort.Search(len(g.cities), func(i int) bool { return g.cities[i].Lat >= lat-radius })
	var best *city
	bestKm := 100.0
	for i := start; i < len(g.cities) && g.cities[i].Lat <= lat+radius; i++ {
		c := &g.cities[i]
		dl := math.Abs(lon - c.Lon)
		if dl > lonRadius && dl < 360-lonRadius {
			continue
		}
		if km := distance(lat, lon, c.Lat, c.Lon); km < bestKm {
			best, bestKm = c, km
		}
	}
	return best
}

func deviceName(make_, model any) any {
	m, ok := model.(string)
	if !ok || strings.TrimSpace(m) == "" {
		return nil
	}
	m = strings.Join(strings.Fields(m), " ")
	mk := ""
	if s, ok := make_.(string); ok {
		mk = strings.Join(strings.Fields(s), " ")
	}
	for mk != "" && strings.HasPrefix(strings.ToLower(m), strings.ToLower(mk+" "+mk)) {
		m = m[len(mk)+1:]
	}
	if mk != "" && !strings.HasPrefix(strings.ToLower(m), strings.ToLower(mk)) {
		m = mk + " " + m
	}
	m = appleRe.ReplaceAllString(m, "$1")
	return trunc(m, 255)
}

var appleRe = regexp.MustCompile(`(?i)^Apple (iPhone|iPad)`)

func trunc(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func num(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, !math.IsNaN(n) && !math.IsInf(n, 0)
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f, err == nil && !math.IsNaN(f) && !math.IsInf(f, 0)
	}
	return 0, false
}

func coordinates(d map[string]any) (lat, lon float64, ok bool) {
	la, ok1 := num(d["GPSLatitude"])
	lo, ok2 := num(d["GPSLongitude"])
	if ok1 && ok2 && la >= -90 && la <= 90 && lo >= -180 && lo <= 180 {
		return la, lo, true
	}
	return 0, 0, false
}

var (
	dateRe   = regexp.MustCompile(`^(\d{4})[:-](\d{2})[:-](\d{2})(?:[T ](\d{2}):(\d{2})(?::(\d{2})(?:[.,](\d+))?)?)?\s*(Z|[+-]\d{2}(?::?\d{2})?)?$`)
	offsetRe = regexp.MustCompile(`^[+-]\d{2}:\d{2}$`)
)

func fixedZone(off string) *time.Location {
	sign := 1
	if off[0] == '-' {
		sign = -1
	}
	digits := strings.ReplaceAll(off[1:], ":", "")
	h, _ := strconv.Atoi(digits[:2])
	m := 0
	if len(digits) >= 4 {
		m, _ = strconv.Atoi(digits[2:4])
	}
	name := "UTC"
	if h != 0 || m != 0 {
		name = "UTC" + off[:1] + digits[:2] + ":" + (digits + "00")[2:4]
	}
	return time.FixedZone(name, sign*(h*3600+m*60))
}

// captureTime returns the UTC capture time and the evidence behind it.
func captureTime(d map[string]any, cityZone, appZone string) (*time.Time, map[string]any) {
	for _, kv := range [][2]string{{"SubSecDateTimeOriginal", "OffsetTimeOriginal"}, {"DateTimeOriginal", "OffsetTimeOriginal"},
		{"CreationDate", "OffsetTimeDigitized"}, {"CreateDate", "OffsetTimeDigitized"}, {"MediaCreateDate", "OffsetTimeDigitized"}} {
		raw, _ := d[kv[0]].(string)
		m := dateRe.FindStringSubmatch(strings.TrimSpace(raw))
		if m == nil {
			continue
		}
		atoi := func(s string) int { n, _ := strconv.Atoi(s); return n }
		year := atoi(m[1])
		if year < 1900 {
			continue
		}
		ns := 0
		if m[7] != "" {
			frac := (m[7] + "000000000")[:9]
			ns = atoi(frac)
		}
		source := "explicit"
		var loc *time.Location
		switch {
		case m[8] == "Z":
			loc = time.UTC
		case m[8] != "":
			loc = fixedZone(m[8])
		}
		if off, _ := d[kv[1]].(string); loc == nil && offsetRe.MatchString(off) {
			loc = fixedZone(off)
		}
		naive := loc == nil
		if naive {
			zone, src := appZone, "application"
			if cityZone != "" {
				zone, src = cityZone, "city"
			}
			l, err := time.LoadLocation(zone)
			if err != nil {
				continue
			}
			loc, source = l, src
		}
		t := time.Date(year, time.Month(atoi(m[2])), atoi(m[3]), atoi(m[4]), atoi(m[5]), atoi(m[6]), ns, loc)
		if t.Year() != year || int(t.Month()) != atoi(m[2]) || t.Day() != atoi(m[3]) || t.Hour() != atoi(m[4]) || t.Minute() != atoi(m[5]) {
			continue // invalid date or a wall time skipped by a DST change
		}
		u := t.UTC()
		return &u, map[string]any{"timeField": kv[0], "timezoneSource": source, "timezone": loc.String(), "localDate": t.Format("2006-01-02")}
	}
	return nil, nil
}

// mediaFacts is extract(): captured_at, coordinates and the facts JSON.
type mediaFacts struct {
	CapturedAt *time.Time
	Lat, Lon   *float64
	Facts      map[string]any
}

func extract(d map[string]any, g *gazetteer, appZone string) mediaFacts {
	var out mediaFacts
	var c *city
	if lat, lon, ok := coordinates(d); ok {
		out.Lat, out.Lon = &lat, &lon
		c = g.nearest(lat, lon)
	}
	zone := ""
	if c != nil {
		zone = c.Zone
	}
	taken, evidence := captureTime(d, zone, appZone)
	out.CapturedAt = taken
	out.Facts = map[string]any{"camera": deviceName(d["Make"], d["Model"]), "lens": deviceName(d["LensMake"], d["LensModel"]), "city": nil}
	for k, v := range evidence {
		out.Facts[k] = v
	}
	if c != nil {
		var id any = c.ID
		if n, err := strconv.ParseInt(c.ID, 10, 64); err == nil {
			id = n
		}
		out.Facts["city"] = map[string]any{"id": id, "name": c.Name, "timezone": c.Zone}
	}
	return out
}

// readMedia runs exiftool on an open descriptor, which pins the inode through a concurrent replacement.
func readMedia(ctx context.Context, path string, g *gazetteer, appZone string) (mediaFacts, string) {
	f, err := os.Open(path)
	if err != nil {
		return mediaFacts{}, "metadata_dependency_or_file_missing"
	}
	defer f.Close()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, env("EXIFTOOL_PATH", "exiftool"), "-json", "-n", "-Make", "-Model", "-LensMake", "-LensModel", "-GPSLatitude", "-GPSLongitude",
		"-SubSecDateTimeOriginal", "-DateTimeOriginal", "-CreationDate", "-CreateDate", "-MediaCreateDate",
		"-OffsetTimeOriginal", "-OffsetTimeDigitized", "-Error", "/dev/fd/3")
	cmd.ExtraFiles = []*os.File{f}
	out, err := cmd.Output()
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return mediaFacts{}, "metadata_timeout"
	case errors.Is(err, exec.ErrNotFound):
		return mediaFacts{}, "metadata_dependency_or_file_missing"
	case err != nil:
		return mediaFacts{}, "metadata_decode_failed"
	}
	var vals []map[string]any
	if json.Unmarshal(out, &vals) != nil || len(vals) == 0 || vals[0]["Error"] != nil {
		return mediaFacts{}, "metadata_decode_failed"
	}
	return extract(vals[0], g, appZone), ""
}
