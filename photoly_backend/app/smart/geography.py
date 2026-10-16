"""Read-only indexed GeoNames snapshot; no network and at most 256 cached queries."""
from contextlib import closing
import math
import sqlite3
from functools import lru_cache
from pathlib import Path

DATA = Path(__file__).parent / 'data' / 'cities.sqlite3'


def distance(lat, lon, other_lat, other_lon):
    a, b, delta = map(math.radians, (lat, other_lat, other_lon - lon))
    h = math.sin((b-a)/2)**2 + math.cos(a)*math.cos(b)*math.sin(delta/2)**2
    return 6371.0088 * 2 * math.asin(min(1, math.sqrt(max(0, h))))


class Gazetteer:
    @lru_cache(maxsize=256)
    def nearest(self, lat, lon):
        # Latitude bounds and antimeridian-aware longitude bounds limit the scan.
        radius = 100 / 111.0
        lon_radius = min(180, radius / max(0.001, math.cos(math.radians(lat))))
        with closing(sqlite3.connect(f'file:{DATA}?mode=ro&immutable=1', uri=True)) as connection:
            rows = connection.execute('SELECT id,name,lat,lon,zone FROM city WHERE lat BETWEEN ? AND ? AND (abs(lon-?) <= ? OR abs(lon-?) >= ?)',
                (lat-radius, lat+radius, lon, lon_radius, lon, 360-lon_radius))
            nearest, best = None, 100.0
            for key, name, x, y, zone in rows:
                km = distance(lat, lon, x, y)
                if km < best:
                    nearest, best = {'id': key, 'name': name, 'timezone': zone}, km
            return nearest
