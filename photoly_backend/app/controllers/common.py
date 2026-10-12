import json
from functools import wraps
from flask import current_app, request, jsonify, Response, send_file
from sqlalchemy.exc import IntegrityError
from app.extensions import db
from app.serialization import serialize, result, token_result, page_result
from app.services.base import BusinessError
from app.services.user import UserService


def service(cls):
    return cls(db.session, current_app.extensions["photoly"])


def arg(name, integer=False, required=False):
    value = request.args.get(name)
    if value is None:
        value = request.form.get(name)
    if value is None:
        if required:
            raise BusinessError(-1)
        return None
    if integer:
        try:
            return int(value)
        except (TypeError, ValueError) as error:
            raise BusinessError(-1) from error
    return value


def ids(name):
    values = request.args.getlist(name) + request.form.getlist(name)
    try:
        return [int(item) for value in values for item in value.split(",") if item.strip()]
    except ValueError as error:
        raise BusinessError(-1) from error


def body():
    value = request.get_json(silent=True)
    if not isinstance(value, dict):
        raise BusinessError(-1)
    return value


def multipart_photos(multiple=False):
    try:
        value = json.loads(arg("photosStr" if multiple else "photo", required=True))
        if multiple:
            if not isinstance(value, list):
                raise ValueError()
            return request.files.getlist("files"), value
        if not isinstance(value, dict) or "file" not in request.files:
            raise ValueError()
        return [request.files["file"]], [value]
    except (ValueError, TypeError) as error:
        raise BusinessError(-1) from error


def changed(value, minimum=1):
    return result(value, 200 if value >= minimum else 400)


def binary(path, mime=None, download=None):
    response = send_file(path, mimetype=mime, as_attachment=download is not None,
                         download_name=download, conditional=True, max_age=0)
    response.headers["Cache-Control"] = "private, no-store"
    return response


def legacy(auth="header", shape="result", missing=None, page_missing=0, failure=None):
    """Preserve each legacy response family; auth failures remain JSON msgCode 403."""
    def decorate(function):
        @wraps(function)
        def invoke(*args, **kwargs):
            try:
                if auth:
                    token = request.headers.get("HRD-Token") if auth == "header" else kwargs.get("token") if auth == "path" else arg("token")
                    uid = service(UserService).identify(token)
                    if uid is None:
                        if shape == "binary":
                            return Response(status=403)
                        value = page_result(missing, page_missing, 403) if shape == "page" else result(missing, 403)
                        return jsonify(value)
                    value = function(uid, *args, **kwargs)
                else:
                    value = function(*args, **kwargs)
                if isinstance(value, Response):
                    return value
                if shape == "raw":
                    if value is None:
                        return Response(status=200)
                    if isinstance(value, str):
                        return Response(value, content_type="text/plain; charset=utf-8")
                return jsonify(serialize(value))
            except (BusinessError, IntegrityError) as error:
                db.session.rollback()
                if isinstance(error, IntegrityError):
                    error = BusinessError(-1)
                if shape == "binary":
                    return Response(status=error.code if error.code in (400, 403, 404) else 400)
                if shape == "raw":
                    return Response(status=200) if failure is None else jsonify(failure)
                if shape == "token":
                    return jsonify(token_result("Error Create User", code=error.code))
                if shape == "page":
                    return jsonify(page_result(None, 0, error.code))
                return jsonify(result(error.value if failure is None else failure, error.code))
        return invoke
    return decorate
