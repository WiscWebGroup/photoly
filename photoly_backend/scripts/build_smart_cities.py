"""Maintainer-only: pin an already downloaded GeoNames cities15000.zip snapshot."""
import argparse
import hashlib
import io
import json
from pathlib import Path
import sqlite3
import zipfile

parser = argparse.ArgumentParser()
parser.add_argument('archive', type=Path)
parser.add_argument('--date', required=True, help='Download date (YYYY-MM-DD)')
args = parser.parse_args()
root = Path(__file__).resolve().parents[1] / 'app/smart/data'
output = root / 'cities.sqlite3'
if output.exists():
    output.unlink()
with zipfile.ZipFile(args.archive) as archive, sqlite3.connect(output) as db:
    db.execute('CREATE TABLE city (id INTEGER PRIMARY KEY,name TEXT NOT NULL,lat REAL NOT NULL,lon REAL NOT NULL,zone TEXT NOT NULL)')
    for line in io.TextIOWrapper(archive.open('cities15000.txt'), encoding='utf-8'):
        row = line.rstrip('\n').split('\t')
        db.execute('INSERT INTO city VALUES (?,?,?,?,?)', (int(row[0]), row[1], float(row[4]), float(row[5]), row[17]))
    db.execute('CREATE INDEX city_lat ON city(lat)')
    count = db.execute('SELECT COUNT(*) FROM city').fetchone()[0]
manifest = {'source': 'https://download.geonames.org/export/dump/cities15000.zip', 'snapshotDate': args.date,
    'license': 'CC BY 4.0', 'attribution': 'GeoNames (https://www.geonames.org/)', 'records': count,
    'sourceSha256': hashlib.sha256(args.archive.read_bytes()).hexdigest(),
    'databaseSha256': hashlib.sha256(output.read_bytes()).hexdigest()}
(root / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
print(manifest)
