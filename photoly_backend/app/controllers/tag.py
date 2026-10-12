from flask import Blueprint
from app.controllers.common import legacy, service, arg, body, changed
from app.entities import Tag
from app.services.catalog import CatalogService
from app.serialization import result

bp = Blueprint("tag", __name__, url_prefix="/tag")


@bp.get("/getAll")
@legacy()
def all_tags(uid):
    return result(service(CatalogService).all(Tag, uid))


@bp.get("/getPage")
@legacy(shape="page", page_missing=-1)
def page(uid):
    return service(CatalogService).paged(Tag, uid, arg("page", integer=True), arg("rowsPerPage", integer=True))


@bp.post("/insert")
@legacy(missing=-1)
def insert(uid):
    return changed(service(CatalogService).insert(Tag, uid, body()))


@bp.post("/update")
@legacy()
def update(uid):
    return changed(service(CatalogService).update(Tag, uid, body()))


@bp.post("/delete")
@legacy()
def delete(uid):
    return changed(service(CatalogService).delete(Tag, uid, arg("tagId", integer=True)))
