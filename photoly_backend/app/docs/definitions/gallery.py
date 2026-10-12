"""Gallery contracts."""
from ..builders import operation as op, parameter as p, pages, json_body, token_parameter
from ..schemas import BOOLEAN, ref, array, EXAMPLES

OPERATIONS = [
    op('Gallery', 'insert', 'POST', '/gallery/insert', 'Create a gallery', BOOLEAN, True, missing=False, failure=False,
       body=json_body(ref('GalleryCreate'), {'gaName': 'Travel', 'coverId': None, 'coverColor': '#FFFFFF'})),
    op('Gallery', 'get_all', 'GET', '/gallery/getAll', 'List all owned galleries', array(ref('Gallery')), [EXAMPLES['Gallery']]),
    op('Gallery', 'get_page', 'GET', '/gallery/getPage', 'List galleries by page', array(ref('Gallery')), [EXAMPLES['Gallery']], shape='page', page_missing=-1, params=pages()),
    op('Gallery', 'delete', 'POST', '/gallery/delete', 'Delete a gallery', missing=-1, params=[p('gaId')], form=True,
       description='Removes the gallery and its associations; media files remain.'),
    op('Gallery', 'update', 'POST', '/gallery/update', 'Update a gallery', missing=-1,
       body=json_body(ref('GalleryUpdate'), {'gaId': 1, 'gaName': 'Travel', 'coverId': None, 'coverColor': '#FFFFFF'}),
       description='Omitted coverId and coverColor become null.'),
    op('Gallery', 'download', 'GET', '/gallery/downloadGa/{token}', 'Download gallery media as ZIP', auth=None, shape='binary',
       params=[token_parameter(), p('gaId')], mime='application/octet-stream', download='download.zip',
       description='Includes associated media beneath a gallery-named folder. Missing files are skipped.'),
]
