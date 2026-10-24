"""Media jobs reuse the durable upload queue; legacy Photo projections stay unchanged."""
from ..builders import operation as op, parameter as p, json_body, token_parameter
from ..schemas import INTEGER, STRING, BOOLEAN, obj, array, nullable

FORMAT = {'type': 'string', 'enum': ['jpeg', 'png', 'gif']}
KEY = '00000000-0000-4000-8000-000000000001'
INFO = obj({'photoId': INTEGER, 'animated': BOOLEAN, 'live': BOOLEAN, 'liveStatus': nullable(STRING),
            'liveError': nullable(STRING), 'convertFormats': array(FORMAT), 'nsId': INTEGER})
OPERATIONS = [
    op('Photo', 'media_capabilities', 'GET', '/photo/mediaCapabilities', 'Read enabled media features',
       obj({'onlineDownload': BOOLEAN, 'convertFormats': array(FORMAT)}),
       {'onlineDownload': False, 'convertFormats': ['jpeg', 'png', 'gif']}),
    op('Photo', 'media_info', 'GET', '/photo/mediaInfo', 'Read private animation and Live Photo capabilities', INFO,
       {'photoId': 1, 'animated': False, 'live': False, 'liveStatus': None, 'liveError': None, 'convertFormats': ['jpeg','png','gif'], 'nsId': 1}, params=[p('photoId')]),
    op('Photo', 'convert_photo', 'POST', '/photo/convert', 'Queue a format conversion copy', STRING, KEY,
       body=json_body(obj({'id': STRING, 'photoId': INTEGER, 'format': FORMAT}), {'id': KEY, 'photoId': 1, 'format': 'jpeg'}),
       description='Client UUID is idempotent and bound to request arguments. Returns an upload batch ID. Captures the original folder and content version at submission; rejects source replacement/deletion before saving. Copies are private with no tags/albums. JPEG quality 90 with white transparency; static targets use the first animation frame or Live cover. GIF preserves image animation or uses the Live movie without sound.'),
    op('Photo', 'online_download', 'POST', '/photo/onlineDownload', 'Queue a public image URL import', STRING, KEY,
       body=json_body(obj({'id': STRING, 'url': STRING, 'nsId': INTEGER, 'format': FORMAT}), {'id': KEY, 'url': 'https://example.com/image.png', 'nsId': 1, 'format': 'png'}),
       description='Requires OnlineDownload=1 at submission, execution and final commit. Only public HTTP/HTTPS direct images; DNS addresses are validated and connections pinned. Each redirect is validated, at most three. Default 50 MiB source, 5 second connect, 60 second download deadline. No proxies, cookies or private networks. Returns an upload batch ID; failures and retries use /uploads.'),
    op('Photo', 'retry_live_preview', 'POST', '/photo/retryLivePreview', 'Queue Live Photo playback regeneration', STRING, KEY,
       body=json_body(obj({'id': STRING, 'photoId': INTEGER}), {'id': KEY, 'photoId': 1})),
    op('Photo', 'render_live', 'GET', '/photo/renderLive/{token}', 'Play owned Live Photo MP4', params=[p('photoId'), token_parameter()],
       auth='path', shape='binary', mime='video/mp4', ranges=True),
    op('Photo', 'download_live', 'GET', '/photo/downloadLive/{token}', 'Download original Live Photo pair', params=[p('photoId'), token_parameter()],
       auth='path', shape='binary', mime='application/zip', download='Live Photo.zip'),
]
