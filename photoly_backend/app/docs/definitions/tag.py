"""Tag contracts."""
from ..builders import operation as op, parameter as p, pages, json_body
from ..schemas import ref, array, EXAMPLES

OPERATIONS = [
    op('Tag', 'get_all', 'GET', '/tag/getAll', 'List owned tags', array(ref('Tag')), [EXAMPLES['Tag']]),
    op('Tag', 'get_page', 'GET', '/tag/getPage', 'List tags by page', array(ref('Tag')), [EXAMPLES['Tag']], shape='page', page_missing=-1, params=pages()),
    op('Tag', 'insert', 'POST', '/tag/insert', 'Create a tag', missing=-1, body=json_body(ref('TagCreate'), {'tagName': 'landscape'})),
    op('Tag', 'update', 'POST', '/tag/update', 'Rename a tag', body=json_body(ref('TagUpdate'), {'tagId': 1, 'tagName': 'landscape'})),
    op('Tag', 'delete', 'POST', '/tag/delete', 'Delete a tag and its associations', params=[p('tagId')], form=True,
       description='Media files remain.'),
]
