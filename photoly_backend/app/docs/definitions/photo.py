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
       description='Swaps the content of an owned jpg/jpeg/png/webp image and regenerates its thumbnail. The photo keeps its ID, UUID, share token, tags, galleries and folder. Embedded EXIF comes only from the new file. t is 1 on success.'),
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
    op('Photo', 'get_tags', 'GET', '/photo/getTagByPhoto', 'List tags linked to media', array(ref('PhotoTag')), [{'photo_id': 1, 'tag_id': 1, 'tag_name': 'landscape'}], params=[p('photoId')]),
    op('Photo', 'get_galleries', 'GET', '/photo/getGalleryByPhoto', 'List galleries containing media', array(ref('PhotoGallery')), [{'photo_id': 1, 'ga_id': 1, 'ga_name': 'Travel'}], params=[p('photoId')]),
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
    op('Photo', 'search', 'GET', '/photo/searchPhoto', 'Search media by one of four criteria', PHOTO_LIST, [EXAMPLES['Photo']],
       params=[p('option', {'type': 'string', 'enum': ['photoName', 'namespace', 'gallery', 'tag']}, 'Search field.', example='photoName'), p('query', STRING, 'Substring to match; an omitted value is treated as an empty string.', False, example='landscape')],
       description='An unknown option or no matches returns t=[] and msgCode=404.', errors={'noMatches': {'t': [], 'msgCode': 404}}),
    op('Photo', 'exif', 'GET', '/photo/getPhotoExif', 'Read available media metadata', ref('Exif'), {'Image_Width': '800', 'Image_Height': '600'},
       params=[p('photoId')], errors={'notFound': {'t': None, 'msgCode': 404}}, description='Partial or missing EXIF is allowed. Response keys retain the legacy underscore spelling.'),
]
# Search's empty-list failure is distinct from scalar business failures.
OPERATIONS[-2][2]['responses']['200']['content']['application/json']['schema']['anyOf'].append({
    'type': 'object', 'properties': {'t': {'type': 'array', 'maxItems': 0}, 'msgCode': {'const': 404}}, 'required': ['t', 'msgCode']})
