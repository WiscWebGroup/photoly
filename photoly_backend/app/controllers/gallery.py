from flask import Blueprint, request
from app.controllers.common import legacy, service, arg, body, changed, binary
from app.entities import Gallery
from app.services.catalog import CatalogService
from app.serialization import result

bp = Blueprint("gallery", __name__, url_prefix="/gallery")


@bp.post("/insert")
@legacy(missing=False, failure=False)
def insert(uid):
    return result(service(CatalogService).insert(Gallery, uid, body()))


@bp.get("/getAll")
@legacy()
def all_galleries(uid):
    return result(service(CatalogService).all(Gallery, uid, request.args.get("includeSmart") == "true"))


@bp.get("/getPage")
@legacy(shape="page", page_missing=-1)
def page(uid):
    return service(CatalogService).paged(Gallery, uid, arg("page", integer=True), arg("rowsPerPage", integer=True), request.args.get("includeSmart") == "true")


@bp.post("/delete")
@legacy(missing=-1)
def delete(uid):
    return changed(service(CatalogService).delete(Gallery, uid, arg("gaId", integer=True)))


@bp.post("/update")
@legacy(missing=-1)
def update(uid):
    return changed(service(CatalogService).update(Gallery, uid, body()))


@bp.get("/downloadGa/<token>")
@legacy(auth="path", shape="binary")
def download(uid, token):
    return binary(service(CatalogService).download_gallery(uid, arg("gaId", integer=True)), "application/octet-stream", "download.zip")
