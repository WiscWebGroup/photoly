# Offline city snapshot

Source: [GeoNames cities15000](https://download.geonames.org/export/dump/cities15000.zip), downloaded 2026-10-16. Attribution: **GeoNames**, https://www.geonames.org/, licensed under [Creative Commons Attribution 4.0](https://creativecommons.org/licenses/by/4.0/).

`manifest.json` records the download date, source archive SHA-256, derived SQLite SHA-256 and row count. The archive is converted to a read-only, indexed SQLite file containing city ID, display name, latitude, longitude and IANA time zone. Alternate names and other source columns are omitted. The packaged database contains 34,165 records and is about 2.5 MB.

Runtime does not download or query a geocoder. Nearest-city results are approximate and limited to 100 km; they do not establish a country's borders, precise address or a person's home. The read-only SQLite reader has a 256-entry bounded cache. A maintainer can update the pinned artifact with `scripts/build_smart_cities.py archive.zip --date YYYY-MM-DD`, review/check in both the database and manifest, and request a refresh after invalidating older cached city facts if changing the geography version.
