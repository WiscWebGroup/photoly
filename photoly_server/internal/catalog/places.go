package catalog

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"math"
	"strconv"
	"strings"
	"sync"
)

//go:embed cities.tsv.gz
var citiesGz []byte

// Port of app/places: nearest GeoNames town (population 1000+) within maxKM of a coordinate.
const (
	maxKM    = 30.0
	cellSize = 0.25
)

type town struct {
	lat, lon float64
	name, cc string
	weight   float64
}

type placeIndex struct {
	countries map[string]string
	cells     map[[2]int][]town
}

type place struct{ Place, Country string }

var (
	placesOnce sync.Once
	placesIdx  *placeIndex
)

func places() *placeIndex {
	placesOnce.Do(func() {
		idx := &placeIndex{countries: map[string]string{}, cells: map[[2]int][]town{}}
		zr, err := gzip.NewReader(bytes.NewReader(citiesGz))
		if err == nil {
			sc := bufio.NewScanner(zr)
			sc.Buffer(make([]byte, 1<<20), 1<<20)
			for sc.Scan() {
				line := sc.Text()
				if strings.HasPrefix(line, "#") {
					if code, label, ok := strings.Cut(line[1:], "\t"); ok {
						idx.countries[code] = label
					}
					continue
				}
				f := strings.Split(line, "\t")
				if len(f) != 5 {
					continue
				}
				lat, _ := strconv.ParseFloat(f[2], 64)
				lon, _ := strconv.ParseFloat(f[3], 64)
				pop, _ := strconv.Atoi(f[4])
				w := math.Pow(float64(max(1, pop)), 0.25)
				k := cell(lat, lon)
				idx.cells[k] = append(idx.cells[k], town{lat, lon, f[0], f[1], w})
			}
		}
		placesIdx = idx
	})
	return placesIdx
}

func cell(lat, lon float64) [2]int {
	return [2]int{int(math.Floor(lat / cellSize)), int(math.Floor(lon / cellSize))}
}

func posMod(a, b int) int { return ((a % b) + b) % b }

func (p *placeIndex) nearest(lat0, lon0 *float64) *place {
	if lat0 == nil || lon0 == nil || !(*lat0 >= -90 && *lat0 <= 90 && *lon0 >= -180 && *lon0 <= 180) {
		return nil
	}
	lat, lon := math.Round(*lat0*1000)/1000, math.Round(*lon0*1000)/1000
	c := cell(lat, lon)
	rows := int(math.Ceil(maxKM / (111 * cellSize)))
	kmPerCol := 111 * cellSize * math.Cos(math.Min(89.0, math.Abs(lat)+maxKM/111)*math.Pi/180)
	cols := min(int(180/cellSize), int(math.Ceil(maxKM/math.Max(0.5, kmPerCol))))
	span := int(360 / cellSize)
	xScale := 111.2 * math.Cos(lat*math.Pi/180)
	var best *town
	bestScore := 0.0
	for r := c[0] - rows; r <= c[0]+rows; r++ {
		for cc := c[1] - cols; cc <= c[1]+cols; cc++ {
			key := [2]int{r, posMod(cc+span/2, span) - span/2}
			for i := range p.cells[key] {
				t := &p.cells[key][i]
				dy := (t.lat - lat) * 111.2
				dx := (math.Mod(math.Mod(t.lon-lon+540, 360)+360, 360) - 180) * xScale
				km := math.Sqrt(dx*dx + dy*dy)
				if km <= maxKM && (best == nil || km/t.weight < bestScore) {
					best, bestScore = t, km/t.weight
				}
			}
		}
	}
	if best == nil {
		return nil
	}
	country := best.cc
	if l, ok := p.countries[best.cc]; ok {
		country = l
	}
	return &place{best.name, country}
}
