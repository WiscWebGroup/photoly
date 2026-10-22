"""Rebuild app/places/cities.tsv.gz from GeoNames dumps (CC BY 4.0, https://www.geonames.org/).

Download cities1000.zip and countryInfo.txt from https://download.geonames.org/export/dump/,
then run: python scripts/build_places.py cities1000.zip countryInfo.txt
"""
import gzip
import io
import sys
import zipfile
from pathlib import Path

OUT = Path(__file__).parents[1] / "app/places/cities.tsv.gz"


def main(cities_zip, country_info):
    countries = {}
    for line in Path(country_info).read_text(encoding="utf-8").splitlines():
        if line and not line.startswith("#"):
            columns = line.split("\t")
            countries[columns[0]] = columns[4]
    rows = []
    with zipfile.ZipFile(cities_zip) as archive:
        name = next(n for n in archive.namelist() if n.endswith(".txt"))
        for line in io.TextIOWrapper(archive.open(name), encoding="utf-8"):
            c = line.rstrip("\n").split("\t")
            rows.append(f"{c[1]}\t{c[8]}\t{float(c[4]):.3f}\t{float(c[5]):.3f}\t{int(c[14] or 0)}")
    rows.sort(key=lambda row: row.split("\t")[1])
    header = [f"#{code}\t{label}" for code, label in sorted(countries.items())]
    OUT.write_bytes(gzip.compress("\n".join(header + rows).encode("utf-8") + b"\n", 9, mtime=0))
    print(f"{len(rows)} places, {len(countries)} countries -> {OUT}")


if __name__ == "__main__":
    main(*sys.argv[1:3])
