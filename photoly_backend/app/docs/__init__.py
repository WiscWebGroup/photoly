"""Public API documentation. Importing or reading it does not access services."""
from copy import deepcopy
from flask import Blueprint, current_app, render_template
from .definitions import MODULES
from .schemas import SCHEMAS, ref

bp = Blueprint('docs', __name__, static_folder='static', static_url_path='/docs/assets', template_folder='templates')


def build_spec():
    paths = {}
    for module in MODULES:
        for path, method, operation in module.OPERATIONS:
            if method in paths.setdefault(path, {}):
                raise ValueError(f'Duplicate documented operation: {method} {path}')
            paths[path][method] = deepcopy(operation)
    paths['/healthz'] = {'get': {
        'tags': ['Health'], 'operationId': 'health_check', 'summary': 'Check database and Redis readiness',
        'description': 'Public readiness probe. Unlike reading this document, this endpoint contacts PostgreSQL and Redis.',
        'security': [], 'responses': {
            '200': {'description': 'Both dependencies are ready.', 'content': {'application/json': {
                'schema': {'type': 'object', 'properties': {'status': {'const': 'ok'}}, 'required': ['status']}, 'example': {'status': 'ok'}}}},
            '503': {'description': 'A dependency is unavailable.', 'content': {'application/json': {
                'schema': ref('HttpError'), 'example': {'t': None, 'msgCode': 503}}}},
        }}}
    return {
        'openapi': '3.2.0',
        'info': {'title': 'Photoly API', 'version': '1.0.0', 'description': '''Photoly media library API. These docs describe the existing Flask API without changing validation or business behavior.

**Start here:** call `/user/signIn`, copy its `token`, then click **Authorize** and paste the token as `HRD-Token` (no Bearer prefix). `/namespace/getRoot` gives the owned folder ID needed for uploads. All examples are fictitious; replace IDs with your own. The UI does not persist authorization across reloads.

**Responses:** ordinary business errors usually have HTTP 200 with a non-200 `msgCode`. Check both HTTP status and `msgCode`. Login responses also contain `token`; paginated responses contain `pageNum`, which means **total pages**, not the requested page. Cred third-party endpoints return raw text, integers, arrays, objects or empty bodies. A missing SQL/Redis dependency returns HTTP 503. Entity fields are camelCase, while association and map projections retain snake_case. Nulls are significant. Entity timestamps are local ISO-style timestamps in configured TIMEZONE without an offset; map dates contain only YYYY-MM-DD.

**Parameters:** query values take precedence over form fields for scalar parameters. Tag ID lists accept repeated values or comma-separated values (and combine query/form lists). Multipart metadata is a JSON-encoded **string**, not a nested object. `file`/`photo` are used for one upload; `files`/`photosStr` for batches. The default request limit is 600 MiB. Both trailing-slash forms are accepted without POST redirects; only canonical paths are listed.

**Tokens:** login sessions, per-photo share tokens and C/R/D API credentials are separate. Authorize applies to header-authenticated operations only. Path/query tokens must be supplied in their own fields. Public documentation does not bypass any API authorization.

**Destructive actions:** account/folder/media deletion removes stored files. `/admin/stop` stops the entire backend. Execute these only on data or disposable services you intend to remove.'''},
        'servers': [{'url': '/', 'description': 'Current backend origin'}],
        'tags': [{'name': name, 'description': description} for name, description in [
            ('User', 'Accounts, login sessions and avatars.'), ('Admin', 'Administrator controls and settings.'),
            ('Photo', 'Uploads, media, associations, sharing and downloads.'), ('Namespace', 'Logical folders and ZIP export.'),
            ('Gallery', 'Galleries and ZIP export.'), ('Tag', 'Media classification tags.'),
            ('Cred', 'API credentials and third-party C/R/D operations.'), ('PhotoExtra', 'Map projections.'),
            ('Health', 'Dependency readiness.'), ('AI Search', 'Private semantic photo and video retrieval.'), ('Smart Organization', 'Opt-in device tags, trip galleries, status and restoration.'), ('AI Admin', 'Asynchronous model, scheduler and profile controls.')]],
        'paths': paths,
        'components': {'securitySchemes': {'SessionToken': {'type': 'apiKey', 'in': 'header', 'name': 'HRD-Token',
            'description': 'Login session token from signIn/signUp. Paste the token alone; do not add Bearer.'}}, 'schemas': deepcopy(SCHEMAS)},
    }


@bp.get('/docs/')
def swagger_ui():
    return render_template('swagger.html')


@bp.get('/openapi.json')
def openapi():
    # Keep the intended operation/example order without changing business JSON output.
    return current_app.response_class(current_app.json.dumps(build_spec(), sort_keys=False), mimetype='application/json')
