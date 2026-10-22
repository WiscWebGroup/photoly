from flask import Blueprint, request
from app.controllers.common import legacy, service, arg, ids, body, changed, binary, multipart_photos
from app.services.photo import PhotoService
from app.serialization import result
from app.services.base import BusinessError

bp = Blueprint("photo", __name__, url_prefix="/photo")


@bp.post("/insert")
@legacy()
def insert(uid):
    return changed(len(service(PhotoService).upload(uid, *multipart_photos())))


@bp.post("/inserts")
@legacy()
def inserts(uid):
    return changed(len(service(PhotoService).upload(uid, *multipart_photos(True))))


@bp.post("/delete")
@legacy()
def delete(uid):
    return changed(service(PhotoService).delete(uid, arg("photoId", integer=True)))


@bp.post("/replace")
@legacy()
def replace(uid):
    if "file" not in request.files:
        raise BusinessError(-1)
    return changed(service(PhotoService).replace(uid, arg("photoId", integer=True, required=True), request.files["file"]))


@bp.post("/update")
@legacy()
def update(uid):
    return changed(service(PhotoService).update(uid, body()))


@bp.get("/getByNamespace")
@legacy()
def by_namespace(uid):
    return result(service(PhotoService).by_namespace(uid, arg("nsId", integer=True)))


@bp.get("/getByTags")
@legacy()
def by_tags(uid):
    return result(service(PhotoService).by_tags(uid, ids("tagIds")))


@bp.get("/getByGallery")
@legacy()
def by_gallery(uid):
    return result(service(PhotoService).by_gallery(uid, arg("gaId", integer=True)))


@bp.get("/getCovers")
@legacy()
def covers(uid):
    return result(service(PhotoService).covers(uid, arg("source", required=True), ids("ids")))


@bp.post("/changeNamespace")
@legacy()
def move(uid):
    return changed(service(PhotoService).move(uid, arg("photoId", integer=True), arg("nsId", integer=True)))


@bp.post("/addTags")
@legacy()
def add_tags(uid):
    return changed(service(PhotoService).add_tags(uid, arg("photoId", integer=True), ids("tagIds")))


@bp.post("/deleteTag")
@legacy()
def delete_tag(uid):
    return changed(service(PhotoService).delete_tags(uid, arg("photoId", integer=True), [arg("tagId", integer=True)]))


@bp.post("/deleteTags")
@legacy()
def delete_tags(uid):
    return changed(service(PhotoService).delete_tags(uid, arg("photoId", integer=True), ids("tagIds")))


@bp.post("/addToGallery")
@legacy()
def add_gallery(uid):
    return changed(service(PhotoService).gallery_link(uid, arg("photoId", integer=True), arg("gaId", integer=True)))


@bp.post("/deleteFromGallery")
@legacy()
def delete_gallery(uid):
    return changed(service(PhotoService).gallery_link(uid, arg("photoId", integer=True), arg("gaId", integer=True), True))


@bp.get("/getTagByPhoto")
@legacy()
def tags(uid):
    return result(service(PhotoService).tags(uid, arg("photoId", integer=True), request.args.get("includeSmart") == "true"))


@bp.get("/getGalleryByPhoto")
@legacy()
def galleries(uid):
    return result(service(PhotoService).galleries(uid, arg("photoId", integer=True), request.args.get("includeSmart") == "true"))


@bp.get("/render/<token>")
@legacy(auth="path", shape="binary")
def render(uid, token):
    return binary(service(PhotoService).path(uid, arg("photoId", integer=True)), "image/jpeg")


@bp.get("/renderThumbnail/<token>")
@legacy(auth="path", shape="binary")
def thumbnail(uid, token):
    return binary(service(PhotoService).path(uid, arg("photoId", integer=True), True), "image/jpeg")


@bp.get("/renderToken")
@legacy(auth=None, shape="binary")
def shared():
    return binary(service(PhotoService).shared_path(arg("path"), arg("token")), "image/jpeg")


@bp.get("/renderToken/Thumbnail")
@legacy(auth=None, shape="binary")
def shared_thumbnail():
    return binary(service(PhotoService).shared_path(arg("path"), arg("token"), True), "image/jpeg")


@bp.get("/renderV/<token>")
@legacy(auth="path", shape="binary")
def video(uid, token):
    return binary(service(PhotoService).path(uid, arg("photoId", integer=True)), "application/octet-stream")


@bp.get("/renderVToken")
@legacy(auth=None, shape="binary")
def shared_video():
    return binary(service(PhotoService).shared_path(arg("path"), arg("token")), "application/octet-stream")


@bp.get("/download/<token>")
@legacy(auth="path", shape="binary")
def download(uid, token):
    path = service(PhotoService).path(uid, arg("photoId", integer=True))
    return binary(path, "application/octet-stream", path.name)


@bp.get("/getPath")
@legacy()
def share_url(uid):
    return result(service(PhotoService).share_url(uid, arg("photoId", integer=True)))


def qr_response(uid):
    # Original QRGen sends PNG bytes under image/jpeg; keep that legacy header.
    return binary(service(PhotoService).qr(uid, arg("photoId", integer=True)), "image/jpeg")


@bp.get("/getQR")
@legacy(shape="binary")
def qr(uid):
    return qr_response(uid)


@bp.get("/getQR2")
@legacy(auth="query", shape="binary")
def qr2(uid):
    return qr_response(uid)


@bp.get("/searchSmart")
@legacy()
def smart_search(uid):
    from app.services.smart_search import search_smart
    photos = service(PhotoService)
    return result(search_smart(photos.session, uid, arg("query", required=True)))


@bp.get("/searchPhoto")
@legacy()
def search(uid):
    photos = service(PhotoService).search(uid, arg("option"), arg("query"))
    return result(photos, 200 if photos else 404)


@bp.get("/getPhotoExif")
@legacy()
def exif(uid):
    return result(service(PhotoService).exif(uid, arg("photoId", integer=True)))


@bp.get('/ocr')
@legacy()
def get_ocr(uid):
    from app.services.ocr import OCRService
    return result(service(OCRService).get(uid, arg('photoId', integer=True, required=True)))


@bp.post('/ocr/recognize')
@legacy()
def recognize_ocr(uid):
    from app.services.ocr import OCRService
    return result(service(OCRService).recognize(uid, body()))


@bp.put('/ocr')
@legacy()
def save_ocr(uid):
    from app.services.ocr import OCRService
    return result(service(OCRService).save(uid, body()))


@bp.after_request
def private_ocr_response(response):
    # The session is carried in a custom header, so shared caches must never store OCR text.
    if request.path.rstrip('/') in ('/photo/ocr', '/photo/ocr/recognize'):
        response.headers['Cache-Control'] = 'private, no-store'
    return response
