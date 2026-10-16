"""Media, associations, sharing and download contracts."""
from ..builders import operation as op, parameter as p, token_parameter, ids_parameter, upload_body, json_body
from ..schemas import STRING, array, ref, EXAMPLES

PHOTO_LIST = array(ref('Photo'))
PATH = p('path', STRING, 'Relative media path: user UUID / photo UUID and original extension. Never an absolute filesystem path.', example='00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000002.jpg')
SHARE = 'Private media (visibility 0) cannot be shared. Visibility 1 requires the per-photo share token; visibility 2 is public. HRD-Token is not used here.'
OPERATIONS = [
    op('Photo', 'insert', 'POST', '/photo/insert', 'Upload one image or video', body=upload_body(),
       description='Creates media and metadata in an owned logical folder. Missing EXIF is allowed. t is the number uploaded.'),
    op('Photo', 'inserts', 'POST', '/photo/inserts', 'Upload multiple images or videos', example=2, body=upload_body(multiple=True),
       description='Repeat the files part and provide one JSON metadata object per file in photosStr. Upload is atomic: invalid input rolls back the entire batch. t is the number uploaded.'),
    op('Photo', 'delete', 'POST', '/photo/delete', 'Permanently delete media', params=[p('photoId')], form=True,
       description='DESTRUCTIVE: deletes the original, thumbnail, metadata and gallery/tag associations.'),
    op('Photo', 'replace', 'POST', '/photo/replace', 'Replace an image file in place (extension to the legacy API)',
       params=[p('photoId')],
       body={"required": True, "content": {"multipart/form-data": {"schema": {"type": "object", "required": ["file"], "properties": {
           "file": {"type": "string", "format": "binary", "description": "Edited jpg, jpeg, png or webp image. The extension picks the stored format."}}}}}},
       description='Swaps the content of an owned jpg/jpeg/png/webp image and regenerates its thumbnail. The photo keeps its ID, UUID, share token and folder. Ordinary associations and manual smart memberships remain; automatic smart associations are invalidated. Previous OCR and embedding state are invalidated. Embedded EXIF comes only from the new file. t is 1 on success.'),
    op('Photo', 'update', 'POST', '/photo/update', 'Update a media name or visibility',
       body=json_body(ref('PhotoUpdate'), {'photoId': 1, 'photoName': 'landscape', 'visibility': 1})),
    op('Photo', 'by_namespace', 'GET', '/photo/getByNamespace', 'List media in a logical folder', PHOTO_LIST, [EXAMPLES['Photo']], params=[p('nsId')],
       description='Only owned media is returned. An unknown folder yields an empty list.'),
    op('Photo', 'by_tags', 'GET', '/photo/getByTags', 'List media matching every selected tag', PHOTO_LIST, [EXAMPLES['Photo']], params=[ids_parameter()],
       description='Intersection semantics: every selected tag must be linked. Empty tagIds returns an empty list.'),
    op('Photo', 'by_gallery', 'GET', '/photo/getByGallery', 'List gallery media', PHOTO_LIST, [EXAMPLES['Photo']], params=[p('gaId')]),
    op('Photo', 'move', 'POST', '/photo/changeNamespace', 'Move media to another logical folder', params=[p('photoId'), p('nsId')], form=True,
       description='Both resources must be owned. Only the database folder reference changes; physical media stays in place.'),
    op('Photo', 'add_tags', 'POST', '/photo/addTags', 'Link tags to media', params=[p('photoId'), ids_parameter()], form=True,
       description='t counts new links. Existing links are ignored; no new links returns t=0, msgCode=400.', errors={'noNewLinks': {'t': 0, 'msgCode': 400}}),
    op('Photo', 'delete_tag', 'POST', '/photo/deleteTag', 'Unlink one tag', params=[p('photoId'), p('tagId')], form=True),
    op('Photo', 'delete_tags', 'POST', '/photo/deleteTags', 'Unlink selected tags', params=[p('photoId'), ids_parameter()], form=True),
    op('Photo', 'add_gallery', 'POST', '/photo/addToGallery', 'Add media to a gallery', params=[p('photoId'), p('gaId')], form=True),
    op('Photo', 'delete_gallery', 'POST', '/photo/deleteFromGallery', 'Remove media from a gallery', params=[p('photoId'), p('gaId')], form=True),
    op('Photo', 'get_tags', 'GET', '/photo/getTagByPhoto', 'List tags linked to media', array(ref('SmartPhotoTag')), [{'photo_id': 1, 'tag_id': 1, 'tag_name': 'landscape'}], params=[p('photoId')]),
    op('Photo', 'get_galleries', 'GET', '/photo/getGalleryByPhoto', 'List galleries containing media', array(ref('SmartPhotoGallery')), [{'photo_id': 1, 'ga_id': 1, 'ga_name': 'Travel'}], params=[p('photoId')]),
    op('Photo', 'render', 'GET', '/photo/render/{token}', 'Read original media using a login session', shape='binary', auth=None,
       params=[token_parameter(), p('photoId')], mime='image/jpeg', description='The legacy MIME type is image/jpeg even when the stored image uses another format.'),
    op('Photo', 'thumbnail', 'GET', '/photo/renderThumbnail/{token}', 'Read a thumbnail using a login session', shape='binary', auth=None,
       params=[token_parameter(), p('photoId')], mime='image/jpeg', description='Falls back to the original when the thumbnail file is missing.'),
    op('Photo', 'shared', 'GET', '/photo/renderToken', 'Read a shared image', shape='binary', auth=None,
       params=[PATH, token_parameter('query', 'share')], mime='image/jpeg', description=SHARE),
    op('Photo', 'shared_thumbnail', 'GET', '/photo/renderToken/Thumbnail', 'Read a shared thumbnail', shape='binary', auth=None,
       params=[{**PATH, 'description': 'Relative thumbnail path: user UUID / photo UUID_thumbnail.extension.', 'example': '00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000002_thumbnail.jpg'}, token_parameter('query', 'share')],
       mime='image/jpeg', description=SHARE + ' Use the _thumbnail suffix. A missing thumbnail returns 404 without original fallback.'),
    op('Photo', 'video', 'GET', '/photo/renderV/{token}', 'Stream video using a login session', shape='binary', auth=None,
       params=[token_parameter(), p('photoId')], mime='application/octet-stream', ranges=True),
    op('Photo', 'shared_video', 'GET', '/photo/renderVToken', 'Stream shared video', shape='binary', auth=None,
       params=[{**PATH, 'example': '00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000002.mp4'}, token_parameter('query', 'share')],
       mime='application/octet-stream', ranges=True, description=SHARE),
    op('Photo', 'download', 'GET', '/photo/download/{token}', 'Download original media', shape='binary', auth=None,
       params=[token_parameter(), p('photoId')], mime='application/octet-stream', download='photo-uuid.jpg',
       description='The attachment filename is the stored photo UUID with its extension.'),
    op('Photo', 'get_path', 'GET', '/photo/getPath', 'Get a media share URL', STRING, 'http://localhost:8384/photo/renderToken?path=example-user/example-photo.jpg&token=example-photo-share-token',
       params=[p('photoId')], description='Absolute URL uses PUBLIC_BASE_URL and renderToken or renderVToken as appropriate. Generating a URL does not change visibility.',
       errors={'notFound': {'t': None, 'msgCode': 403}, 'unsupportedFormat': {'t': None, 'msgCode': 402}}),
    op('Photo', 'qr', 'GET', '/photo/getQR', 'Generate a share QR code using the session header', shape='binary',
       params=[p('photoId')], mime='image/jpeg', description='250 x 250 PNG bytes under the legacy image/jpeg header. Encodes the share URL without changing visibility.'),
    op('Photo', 'qr_query', 'GET', '/photo/getQR2', 'Generate a share QR code using a query session token', shape='binary', auth=None,
       params=[token_parameter('query'), p('photoId')], mime='image/jpeg', description='250 x 250 PNG bytes under the legacy image/jpeg header.'),
    op('Photo', 'smart_search', 'GET', '/photo/searchSmart', 'Rank media by name, tags and galleries', PHOTO_LIST, [EXAMPLES['Photo']],
       params=[p('query', {'type': 'string', 'maxLength': 512}, 'Case-insensitive text, split into whitespace-separated keywords.', example='summer beach')],
       description='Returns at most 100 owned photos and videos ordered by weighted text relevance, then photo ID. Name/tag/gallery weights are 40/30/30; each field scores 3 for equality, 2 for a whole-query substring, otherwise the matched keyword fraction. Associations use their maximum score. Wildcards are literal. Empty queries and no matches return t=[] with msgCode=200. No AI permission is required for this lexical endpoint. Namespace names are not searched. The Smart Search client separately requests up to 100 semantic candidates, uses equal-weight reciprocal rank fusion (1 / (60 + rank) per source), deduplicates by photo ID and displays at most 100 results.'),
    op('Photo', 'search', 'GET', '/photo/searchPhoto', 'Search media by one of four criteria', PHOTO_LIST, [EXAMPLES['Photo']],
       params=[p('option', {'type': 'string', 'enum': ['photoName', 'namespace', 'gallery', 'tag']}, 'Search field.', example='photoName'), p('query', STRING, 'Substring to match; an omitted value is treated as an empty string.', False, example='landscape')],
       description='An unknown option or no matches returns t=[] and msgCode=404.', errors={'noMatches': {'t': [], 'msgCode': 404}}),
    op('Photo', 'exif', 'GET', '/photo/getPhotoExif', 'Read available media metadata', ref('Exif'), {'Image_Width': '800', 'Image_Height': '600'},
       params=[p('photoId')], errors={'notFound': {'t': None, 'msgCode': 404}}, description='Partial or missing EXIF is allowed. Response keys retain the legacy underscore spelling.'),
]
# Search's empty-list failure is distinct from scalar business failures.
OPERATIONS[-2][2]['responses']['200']['content']['application/json']['schema']['anyOf'].append({
    'type': 'object', 'properties': {'t': {'type': 'array', 'maxItems': 0}, 'msgCode': {'const': 404}}, 'required': ['t', 'msgCode']})

OCR_VERSION = {'photoId': {'type': 'integer', 'minimum': 1},
               'contentVersion': {'type': 'integer', 'minimum': 1},
               'revision': {'type': 'integer', 'minimum': 0}}
OCR_VALUE = {'type': 'object', 'required': ['photoId', 'contentVersion', 'revision', 'text', 'languages', 'engineVersion', 'updatedAt'],
             'properties': {**OCR_VERSION, 'text': {'type': ['string', 'null'], 'maxLength': 100000},
                            'languages': STRING, 'engineVersion': {'type': ['string', 'null']},
                            'updatedAt': {'type': ['string', 'null']}}}
OCR_EXAMPLE = {'photoId': 1, 'contentVersion': 1, 'revision': 1, 'text': 'Hello 世界',
               'languages': 'chi_sim+chi_tra+eng+jpn', 'engineVersion': 'tesseract 5', 'updatedAt': '2026-10-16T12:00:00+00:00'}
OCR_ERRORS = {
    name: {'t': {'error': reason}, 'msgCode': code}
    for name, reason, code in [('conflict', 'ocr_conflict', 409), ('busy', 'ocr_busy', 429),
                              ('timeout', 'ocr_timeout', 504), ('missing', 'photo_not_found', 404),
                              ('unavailable', 'engine_unavailable', 503), ('invalid', 'invalid_image', 400)]}
OCR_OPERATIONS = [
    op('Photo', 'get_ocr', 'GET', '/photo/ocr', 'Read saved image text', OCR_VALUE, OCR_EXAMPLE,
       params=[p('photoId')], errors=OCR_ERRORS,
       description='Owner-only. Does not run OCR. No saved result has text=null and revision=0; empty saved text is an empty string.'),
    op('Photo', 'recognize_ocr', 'POST', '/photo/ocr/recognize', 'Recognize and save image text using Tesseract CPU', OCR_VALUE, OCR_EXAMPLE,
       body=json_body({'type': 'object', 'properties': OCR_VERSION, 'required': list(OCR_VERSION)},
                      {'photoId': 1, 'contentVersion': 1, 'revision': 0}), errors=OCR_ERRORS,
       description='Explicitly replaces saved text only after successful recognition. Requires current contentVersion and revision from GET. '
                   'Supports Pillow raster formats, first frame/page only. Limits: 20 MiB, 40 million pixels, longest side 4096 after resizing, '
                   '45-second total deadline, 30-second OCR deadline, one concurrent OCR operation across backend processes. '
                   'Languages: chi_sim+chi_tra+eng+jpn. AI enablement is not required. Failures leave previous text intact.'),
    op('Photo', 'save_ocr', 'PUT', '/photo/ocr', 'Save edited image text', OCR_VALUE, OCR_EXAMPLE,
       body=json_body({'type': 'object', 'properties': {**OCR_VERSION, 'text': {'type': 'string', 'maxLength': 100000}},
                       'required': [*OCR_VERSION, 'text']},
                      {'photoId': 1, 'contentVersion': 1, 'revision': 1, 'text': 'Edited text'}), errors=OCR_ERRORS,
       description='Owner-only optimistic update. Empty text is allowed; null characters are rejected. Version mismatch returns msgCode=409 without overwriting the saved text.'),
]
for _, _, operation in OCR_OPERATIONS:
    operation['description'] += ' Business errors use HTTP 200 with msgCode and t.error; inspect both HTTP status and msgCode. Responses are private and must not be cached.'
    operation['responses']['200']['headers'] = {'Cache-Control': {'description': 'OCR text must not be cached.', 'schema': {'type': 'string', 'const': 'private, no-store'}}}
    operation['responses']['200']['content']['application/json']['schema']['anyOf'].append({
        'type': 'object', 'properties': {'msgCode': {'enum': [400, 404, 409, 413, 429, 503, 504]},
                                       't': {'type': 'object', 'properties': {'error': STRING}, 'required': ['error']}},
        'required': ['t', 'msgCode']})
OPERATIONS.extend(OCR_OPERATIONS)


for path, method, operation in OPERATIONS:
    if method.lower() == 'get' and path in ('/tag/getAll', '/tag/getPage', '/gallery/getAll', '/gallery/getPage', '/photo/getTagByPhoto', '/photo/getGalleryByPhoto'):
        operation.setdefault('parameters', []).append(p('includeSmart', {'type': 'boolean'}, 'Include optional smart provenance. Omitted preserves the legacy response fields.', required=False))
    if path in ('/photo/addTags', '/photo/addToGallery'):
        operation['description'] = operation.get('description', '') + ' For smart items, explicitly added memberships are retained during automatic recomputation.'
    if path in ('/photo/deleteTag', '/photo/deleteTags', '/photo/deleteFromGallery'):
        operation['description'] = operation.get('description', '') + ' Removing a smart membership records an exclusion so automatic recomputation does not restore it.'
