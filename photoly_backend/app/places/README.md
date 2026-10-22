# Place names

`cities.tsv.gz` lists every town with a population of 1,000 or more (name, country code,
latitude, longitude, population), plus country names. It backs `GET /photo-extra/getMapPlaces`,
which names map places without calling any outside service.

Data © [GeoNames](https://www.geonames.org/), licensed under
[CC BY 4.0](https://creativecommons.org/licenses/by/4.0/). Rebuild it from the latest dump with
`scripts/build_places.py` (see the script for the download links).
