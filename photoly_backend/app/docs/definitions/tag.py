"""Tag contracts."""
from ..builders import operation as op, parameter as p, pages, json_body
from ..schemas import ref, array, EXAMPLES

OPERATIONS = [
    op('Tag', 'get_all', 'GET', '/tag/getAll', 'List owned tags', array(ref('SmartTag')), [EXAMPLES['Tag']]),
    op('Tag', 'get_page', 'GET', '/tag/getPage', 'List tags by page', array(ref('SmartTag')), [EXAMPLES['Tag']], shape='page', page_missing=-1, params=pages()),
    op('Tag', 'insert', 'POST', '/tag/insert', 'Create a tag', missing=-1, body=json_body(ref('TagCreate'), {'tagName': 'landscape'})),
    op('Tag', 'update', 'POST', '/tag/update', 'Rename a tag', body=json_body(ref('TagUpdate'), {'tagId': 1, 'tagName': 'landscape'})),
    op('Tag', 'delete', 'POST', '/tag/delete', 'Delete a tag and its associations', params=[p('tagId')], form=True,
       description='Media files remain.'),
]


for path, method, operation in OPERATIONS:
    if method.lower() == 'get' and path in ('/tag/getAll', '/tag/getPage', '/gallery/getAll', '/gallery/getPage', '/photo/getTagByPhoto', '/photo/getGalleryByPhoto'):
        operation.setdefault('parameters', []).append(p('includeSmart', {'type': 'boolean'}, 'Include optional smart provenance. Omitted preserves the legacy response fields.', required=False))
    if path.endswith(('/update', '/delete')):
        operation['description'] = operation.get('description', '') + ' Smart item edits preserve custom names/covers; deletion suppresses automatic recreation until explicitly restored.'
