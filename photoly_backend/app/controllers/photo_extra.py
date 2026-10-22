from flask import Blueprint
from app.controllers.common import legacy, service, arg
from app.services.photo import PhotoService
from app.serialization import result

bp = Blueprint("photo_extra", __name__, url_prefix="/photo-extra")


@bp.get("/getGlobalMapPhoto")
@legacy()
def all_photos(uid):
    return result(service(PhotoService).map_photos(uid))


@bp.get("/getMapPlaces")
@legacy()
def places(uid):
    return result(service(PhotoService).map_places(uid))


@bp.get("/getGlobalMapPhotoByGallery")
@legacy()
def gallery_photos(uid):
    return result(service(PhotoService).map_photos(uid, arg("gaId", integer=True, required=True)))
