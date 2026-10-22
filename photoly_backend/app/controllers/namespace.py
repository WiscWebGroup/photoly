from flask import Blueprint
from app.controllers.common import legacy, service, arg, body, changed, binary
from app.services.namespace import NamespaceService
from app.serialization import result

bp = Blueprint("namespace", __name__, url_prefix="/namespace")


@bp.get("/getRoot")
@legacy()
def root(uid):
    return result(service(NamespaceService).root(uid))


@bp.get("/getChildren")
@legacy()
def children(uid):
    return result(service(NamespaceService).children(uid, arg("parentId", integer=True)))


@bp.get("/getDetails")
@legacy()
def details(uid):
    return result(service(NamespaceService).details(uid, arg("nsId", integer=True, required=True)))


@bp.get("/getNameById")
@legacy()
def name(uid):
    return result(service(NamespaceService).name(uid, arg("nsId", integer=True)))


@bp.post("/updateName")
@legacy()
def rename(uid):
    return changed(service(NamespaceService).rename(uid, body()))


@bp.post("/updateParent")
@legacy()
def move(uid):
    return changed(service(NamespaceService).move(uid, body()))


@bp.post("/insert")
@legacy(missing=-1)
def insert(uid):
    return changed(service(NamespaceService).insert(uid, body()))


@bp.post("/delete")
@legacy()
def delete(uid):
    return changed(service(NamespaceService).delete(uid, arg("nsId", integer=True)), minimum=0)


@bp.get("/trace")
@legacy()
def trace(uid):
    return result(service(NamespaceService).trace(uid, arg("nsId", integer=True)))


@bp.get("/downloadNs/<token>")
@legacy(auth="path", shape="binary")
def download(uid, token):
    return binary(service(NamespaceService).download(uid, arg("nsId", integer=True)), "application/octet-stream", "download.zip")
