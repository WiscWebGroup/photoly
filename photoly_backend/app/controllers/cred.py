from flask import Blueprint
from app.controllers.common import legacy, service, arg, body, changed, binary, multipart_photos
from app.services.cred import CredService
from app.services.photo import PhotoService
from app.services.namespace import NamespaceService
from app.entities import Namespace
from app.serialization import result

bp = Blueprint("cred", __name__, url_prefix="/cred")


@bp.post("/create")
@legacy(failure=0)
def create(uid):
    return changed(service(CredService).create(uid, body()))


@bp.get("/query")
@legacy()
def query(uid):
    return result(service(CredService).query(uid))


@bp.post("/update")
@legacy()
def update(uid):
    return changed(service(CredService).update(uid, body()))


@bp.post("/delete")
@legacy()
def delete(uid):
    return changed(service(CredService).delete(uid, arg("credId", integer=True)))


@bp.post("/uploadPhoto/<token>")
@legacy(auth=None, shape="raw")
def upload(token):
    cred = service(CredService).identify(token)
    if cred is None:
        return "Unidentified Token"
    if "C" not in cred.authorization:
        return "Wrong Auth for current Token"
    return service(PhotoService).upload(cred.user_id, *multipart_photos())[0].photo_uuid


@bp.post("/uploadPhotos/<token>")
@legacy(auth=None, shape="raw")
def uploads(token):
    cred = service(CredService).identify(token)
    if cred is None or "C" not in cred.authorization:
        return None
    return [p.photo_uuid for p in service(PhotoService).upload(cred.user_id, *multipart_photos(True))]


@bp.get("/render/<token>")
@legacy(auth=None, shape="raw")
def render(token):
    cred = service(CredService).identify(token)
    if cred is None or "R" not in cred.authorization:
        return None
    photo = service(PhotoService).by_uuid(cred.user_id, arg("uuid"))
    return binary(service(PhotoService).path(cred.user_id, photo.photo_id), "image/jpeg")


@bp.post("/deletePhoto/<token>")
@legacy(auth=None, shape="raw", failure=-1)
def delete_photo(token):
    cred = service(CredService).identify(token)
    if cred is None:
        return -1
    if "D" not in cred.authorization:
        return -2
    photo = service(PhotoService).by_uuid(cred.user_id, arg("uuid"))
    return service(PhotoService).delete(cred.user_id, photo.photo_id)


@bp.get("/queryRootNamespace/<token>")
@legacy(auth=None, shape="raw")
def root(token):
    cred = service(CredService).identify(token)
    if cred is None or "R" not in cred.authorization:
        return None
    return service(NamespaceService).root(cred.user_id)


@bp.get("/queryNamespaces/<token>")
@legacy(auth=None, shape="raw")
def children(token):
    cred = service(CredService).identify(token)
    if cred is None or "R" not in cred.authorization:
        return None
    return service(NamespaceService).children(cred.user_id, arg("parentId", integer=True))


@bp.get("/queryPhotoList/<token>")
@legacy(auth=None, shape="raw")
def photo_list(token):
    cred = service(CredService).identify(token)
    if cred is None or "R" not in cred.authorization:
        return None
    ns_id = arg("nsId", integer=True)
    # The original API requires the namespace to exist and belong to the API owner.
    service(PhotoService).owned(Namespace, ns_id, cred.user_id)
    return service(PhotoService).by_namespace(cred.user_id, ns_id)
