"""Offline reverse geocoding: the nearest town (GeoNames, population 1000+) to a coordinate.

Data: GeoNames (https://www.geonames.org/), CC BY 4.0. Rebuild with scripts/build_places.py.
"""
import gzip
from functools import lru_cache
from math import ceil, cos, radians, sqrt
from pathlib import Path

DATA = Path(__file__).with_name("cities.tsv.gz")
# A photo further than this from any town gets no place name.
MAX_KM = 30
# Cell size in degrees; small enough that dense regions stay quick to scan.
CELL = 0.25


class PlaceIndex:
    """Places bucketed into small grid cells, so a lookup only scans nearby cells."""

    def __init__(self, lines):
        self.countries = {}
        self.cells = {}
        for line in lines:
            if line.startswith("#"):
                code, label = line[1:].split("\t", 1)
                self.countries[code] = label
                continue
            name, country, lat, lon, population = line.split("\t")
            lat, lon = float(lat), float(lon)
            # Larger towns win over closer suburbs: distance is divided by population ** 0.25.
            weight = max(1, int(population)) ** 0.25
            self.cells.setdefault(self.cell(lat, lon), []).append((lat, lon, name, country, weight))

    @staticmethod
    def cell(lat, lon):
        return int(lat // CELL), int(lon // CELL)

    def nearest(self, lat, lon):
        """`{"place", "country"}` for the best-ranked town within MAX_KM, else None."""
        if lat is None or lon is None or not (-90 <= lat <= 90 and -180 <= lon <= 180):
            return None
        # Photos cluster, so nearby lookups share one answer per ~100 m.
        return self._nearest(round(lat, 3), round(lon, 3))

    @lru_cache(maxsize=65536)
    def _nearest(self, lat, lon):
        row, column = self.cell(lat, lon)
        rows = ceil(MAX_KM / (111 * CELL))
        km_per_column = 111 * CELL * cos(radians(min(89.0, abs(lat) + MAX_KM / 111)))
        columns = min(int(180 / CELL), ceil(MAX_KM / max(0.5, km_per_column)))
        span = int(360 / CELL)
        # At this range a flat-earth distance is within a fraction of a percent, and much cheaper.
        x_scale = 111.2 * cos(radians(lat))
        best, best_score = None, None
        for r in range(row - rows, row + rows + 1):
            for c in range(column - columns, column + columns + 1):
                for item in self.cells.get((r, (c + span // 2) % span - span // 2), ()):
                    dy = (item[0] - lat) * 111.2
                    dx = ((item[1] - lon + 540) % 360 - 180) * x_scale
                    km = sqrt(dx * dx + dy * dy)
                    if km <= MAX_KM and (best_score is None or km / item[4] < best_score):
                        best, best_score = item, km / item[4]
        if best is None:
            return None
        return {"place": best[2], "country": self.countries.get(best[3], best[3])}


@lru_cache(maxsize=1)
def index():
    with gzip.open(DATA, "rt", encoding="utf-8") as stream:
        return PlaceIndex(line.rstrip("\n") for line in stream)
