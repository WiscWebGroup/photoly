"""Gallery contracts."""
from ..builders import operation as op, parameter as p, pages, json_body, token_parameter
from ..schemas import BOOLEAN, ref, array, EXAMPLES

OPERATIONS = [
    op('Gallery', 'insert', 'POST', '/gallery/insert', 'Create a gallery', BOOLEAN, True, missing=False, failure=False,
       body=json_body(ref('GalleryCreate'), {'gaName': 'Travel', 'coverId': None, 'coverColor': '#FFFFFF'})),
    op('Gallery', 'get_all', 'GET', '/gallery/getAll', 'List all owned galleries', array(ref('SmartGallery')), [EXAMPLES['Gallery']]),
    op('Gallery', 'get_page', 'GET', '/gallery/getPage', 'List galleries by page', array(ref('SmartGallery')), [EXAMPLES['Gallery']], shape='page', page_missing=-1, params=pages()),
    op('Gallery', 'delete', 'POST', '/gallery/delete', 'Delete a gallery', missing=-1, params=[p('gaId')], form=True,
       description='Removes the gallery and its associations; media files remain.'),
    op('Gallery', 'update', 'POST', '/gallery/update', 'Update a gallery', missing=-1,
       body=json_body(ref('GalleryUpdate'), {'gaId': 1, 'gaName': 'Travel', 'coverId': None, 'coverColor': '#FFFFFF'}),
       description='Omitted coverId and coverColor become null.'),
    op('Gallery', 'download', 'GET', '/gallery/downloadGa/{token}', 'Download gallery media as ZIP', auth=None, shape='binary',
       params=[token_parameter(), p('gaId')], mime='application/octet-stream', download='download.zip',
       description='Includes associated media beneath a gallery-named folder. Missing files are skipped.'),
]


for path, method, operation in OPERATIONS:
    if method.lower() == 'get' and path in ('/tag/getAll', '/tag/getPage', '/gallery/getAll', '/gallery/getPage', '/photo/getTagByPhoto', '/photo/getGalleryByPhoto'):
        operation.setdefault('parameters', []).append(p('includeSmart', {'type': 'boolean'}, 'Include optional smart provenance. Omitted preserves the legacy response fields.', required=False))
    if path.endswith(('/update', '/delete')):
        operation['description'] = operation.get('description', '') + ' Smart item edits preserve custom names/covers; deletion suppresses automatic recreation until explicitly restored.'
