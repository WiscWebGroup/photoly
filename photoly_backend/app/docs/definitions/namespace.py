"""Logical folder contracts."""
from ..builders import operation as op, parameter as p, token_parameter, json_body
from ..schemas import STRING, ref, array, EXAMPLES

OPERATIONS = [
    op('Namespace', 'get_root', 'GET', '/namespace/getRoot', 'Get the current user root folder', ref('Namespace'), EXAMPLES['Namespace'], failure=None,
       description="The root is named '/' and has nsParentId=-1."),
    op('Namespace', 'get_children', 'GET', '/namespace/getChildren', 'List direct child folders', array(ref('Namespace')), [EXAMPLES['Namespace']],
       params=[p('parentId')], description='Only folders owned by the current user are returned. An unknown parent produces an empty list.'),
    op('Namespace', 'get_name', 'GET', '/namespace/getNameById', 'Read a folder name', STRING, 'Travel', params=[p('nsId')]),
    op('Namespace', 'rename', 'POST', '/namespace/updateName', 'Rename a logical folder',
       body=json_body(ref('NamespaceRename'), {'nsId': 2, 'nsName': 'Travel'})),
    op('Namespace', 'move', 'POST', '/namespace/updateParent', 'Move a logical folder',
       body=json_body(ref('NamespaceMove'), {'nsId': 2, 'nsParentId': 1}),
       description='Changes logical parentage only; original media stays in its existing physical location. Root moves, cycles and cross-user moves are rejected.'),
    op('Namespace', 'insert', 'POST', '/namespace/insert', 'Create a folder', missing=-1,
       body=json_body(ref('NamespaceCreate'), {'nsName': 'Travel', 'nsParentId': 1})),
    op('Namespace', 'delete', 'POST', '/namespace/delete', 'Delete a folder tree and its media', params=[p('nsId')], form=True,
       description='DESTRUCTIVE: recursively removes folders and media, including originals and thumbnails. The root cannot be deleted. t counts deleted files and folder records.'),
    op('Namespace', 'trace', 'GET', '/namespace/trace', 'Get the folder breadcrumb', array(ref('FolderTraceEntry')), [{'nsId': 1, 'nsName': '/'}],
       params=[p('nsId')], description='Ordered from root to the requested folder.'),
    op('Namespace', 'download', 'GET', '/namespace/downloadNs/{token}', 'Download a folder tree as ZIP', auth=None, shape='binary',
       params=[token_parameter(), p('nsId')], mime='application/octet-stream', download='download.zip',
       description="Preserves logical folder hierarchy. The root folder is represented as 'root/' in the archive. Missing media files are skipped."),
]
