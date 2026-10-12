"""API credential management and direct-response third-party API."""
from ..builders import operation as op, parameter as p, json_body, token_parameter, upload_body
from ..schemas import STRING, INTEGER, ref, array, EXAMPLES

OPERATIONS = [
    op('Cred', 'create', 'POST', '/cred/create', 'Create an API credential', failure=0,
       body=json_body(ref('CredCreate'), {'authorization': 'CRD'}), description='Use /cred/query to retrieve the generated credential. C=create, R=read, D=delete.'),
    op('Cred', 'query', 'GET', '/cred/query', 'List owned API credentials', array(ref('Cred')), [EXAMPLES['Cred']]),
    op('Cred', 'update', 'POST', '/cred/update', 'Change an API credential permission set', body=json_body(ref('CredUpdate'), {'credId': 1, 'authorization': 'R'})),
    op('Cred', 'delete', 'POST', '/cred/delete', 'Revoke an API credential', params=[p('credId')], form=True),
    op('Cred', 'upload', 'POST', '/cred/uploadPhoto/{token}', 'Upload one file with a C credential', auth=None, shape='raw',
       params=[token_parameter(kind='api')], body=upload_body(), raw_empty=True,
       raw_content={'text/plain': {'schema': STRING, 'examples': {'success': {'value': '00000000-0000-4000-8000-000000000002'},
                    'unknownToken': {'value': 'Unidentified Token'}, 'missingPermission': {'value': 'Wrong Auth for current Token'}}}},
       description='Requires C. Returns the created photo UUID as plain text; token errors are also plain text. Other business failures return an empty body.'),
    op('Cred', 'uploads', 'POST', '/cred/uploadPhotos/{token}', 'Upload multiple files with a C credential', array(STRING), ['00000000-0000-4000-8000-000000000002'],
       auth=None, shape='raw', params=[token_parameter(kind='api')], body=upload_body(multiple=True), raw_empty=True, description='Requires C. Returns a JSON array of photo UUIDs. Invalid token, missing permission or business failure returns an empty body.'),
    op('Cred', 'render', 'GET', '/cred/render/{token}', 'Read original media with an R credential', auth=None, shape='raw',
       params=[token_parameter(kind='api'), p('uuid', STRING, 'Photo UUID owned by this credential owner.')], raw_empty=True,
       raw_content={'image/jpeg': {'schema': {'type': 'string', 'format': 'binary'}}}, description='Requires R. Missing or inaccessible media returns an empty HTTP 200 response.'),
    op('Cred', 'delete_photo', 'POST', '/cred/deletePhoto/{token}', 'Permanently delete media with a D credential', auth=None, shape='raw',
       params=[token_parameter(kind='api'), p('uuid', STRING, 'Photo UUID owned by this credential owner.')], form=True,
       raw_content={'application/json': {'schema': INTEGER, 'examples': {'success': {'value': 1}, 'unknownTokenOrPhoto': {'value': -1}, 'missingPermission': {'value': -2}}}},
       description='DESTRUCTIVE. Requires D. Direct JSON integer: 1 deleted, -1 invalid token/missing media/business error, -2 missing D permission.'),
    op('Cred', 'root', 'GET', '/cred/queryRootNamespace/{token}', 'Read the root folder with an R credential', ref('Namespace'), EXAMPLES['Namespace'],
       auth=None, shape='raw', params=[token_parameter(kind='api')], raw_empty=True, description='Requires R. Invalid credentials or business failures return an empty body.'),
    op('Cred', 'children', 'GET', '/cred/queryNamespaces/{token}', 'List direct child folders with an R credential', array(ref('Namespace')), [EXAMPLES['Namespace']],
       auth=None, shape='raw', params=[token_parameter(kind='api'), p('parentId')], raw_empty=True, description='Requires R. Results are restricted to the credential owner.'),
    op('Cred', 'photos', 'GET', '/cred/queryPhotoList/{token}', 'List folder media with an R credential', array(ref('Photo')), [EXAMPLES['Photo']],
       auth=None, shape='raw', params=[token_parameter(kind='api'), p('nsId')], raw_empty=True, description='Requires R. The folder must exist and belong to the credential owner.'),
]
