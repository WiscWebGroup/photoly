"""Map projection contracts."""
from ..builders import operation as op, parameter as p
from ..schemas import array, ref, EXAMPLES

OPERATIONS = [
    op('PhotoExtra', 'map_all', 'GET', '/photo-extra/getGlobalMapPhoto', 'List media for the map', array(ref('MapPhoto')), [EXAMPLES['MapPhoto']],
       description='Legacy snake_case projection. Dates contain only YYYY-MM-DD. Coordinates and taken dates may be null.'),
    op('PhotoExtra', 'map_gallery', 'GET', '/photo-extra/getGlobalMapPhotoByGallery', 'List gallery media for the map', array(ref('MapPhoto')), [EXAMPLES['MapPhoto']],
       params=[p('gaId')], description='Same map projection, restricted to one owned gallery.'),
]
