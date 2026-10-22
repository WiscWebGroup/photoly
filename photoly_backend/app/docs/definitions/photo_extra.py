"""Map projection contracts."""
from ..builders import operation as op, parameter as p
from ..schemas import INTEGER, STRING, array, obj, ref, EXAMPLES

OPERATIONS = [
    op('PhotoExtra', 'map_all', 'GET', '/photo-extra/getGlobalMapPhoto', 'List media for the map', array(ref('MapPhoto')), [EXAMPLES['MapPhoto']],
       description='Legacy snake_case projection. Dates contain only YYYY-MM-DD. Coordinates and taken dates may be null.'),
    op('PhotoExtra', 'map_places', 'GET', '/photo-extra/getMapPlaces', 'Name the places where media was taken',
       array(obj({'photoId': INTEGER, 'place': STRING, 'country': STRING})), [{'photoId': 1, 'place': 'Reykjavík', 'country': 'Iceland'}],
       description='Offline lookup of the nearest town (population 1,000 or more, larger towns preferred) within 30 km of each geotagged item, from bundled GeoNames data (CC BY 4.0). Items with no town in range are left out.'),
    op('PhotoExtra', 'map_gallery', 'GET', '/photo-extra/getGlobalMapPhotoByGallery', 'List gallery media for the map', array(ref('MapPhoto')), [EXAMPLES['MapPhoto']],
       params=[p('gaId')], description='Same map projection, restricted to one owned gallery.'),
]
