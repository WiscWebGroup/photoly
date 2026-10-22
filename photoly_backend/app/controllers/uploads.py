from flask import Blueprint, request
from app.controllers.common import legacy, service, body, arg
from app.serialization import result
from app.services.uploads import UploadService

bp = Blueprint('uploads', __name__, url_prefix='/uploads')


@bp.post('')
@legacy()
def create(uid):
    return result(service(UploadService).create(uid, body()))


@bp.get('')
@legacy()
def batches(uid):
    return result(service(UploadService).listing(uid, (arg('page', integer=True) if arg('page') is not None else 1)))


@bp.post('/<batch_id>/items')
@legacy()
def register(uid, batch_id):
    return result(service(UploadService).register(uid, batch_id, body().get('items')))


@bp.get('/<batch_id>/items')
@legacy()
def items(uid, batch_id):
    return result(service(UploadService).listing(uid, (arg('page', integer=True) if arg('page') is not None else 1), batch_id))


@bp.put('/<batch_id>/items/<item_id>/content')
@legacy()
def receive(uid, batch_id, item_id):
    return result(service(UploadService).receive(uid, batch_id, item_id, request.stream))


@bp.post('/<batch_id>/finish')
@legacy()
def finish(uid, batch_id):
    return result(service(UploadService).finish(uid, batch_id))


@bp.post('/<batch_id>/cancel')
@legacy()
def cancel(uid, batch_id):
    return result(service(UploadService).cancel(uid, batch_id))


@bp.post('/<batch_id>/retry')
@legacy()
def retry(uid, batch_id):
    return result(service(UploadService).retry(uid, batch_id))
