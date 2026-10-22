from flask import Blueprint, request
from app.controllers.common import legacy, service, body, arg, changed, binary
from app.services.user import UserService
from app.serialization import result

bp = Blueprint("user", __name__, url_prefix="/user")


@bp.post("/signUp")
@legacy(auth=None, shape="token")
def sign_up():
    return service(UserService).sign_up(body())


@bp.post("/signIn")
@legacy(auth=None, shape="token")
def sign_in():
    return service(UserService).sign_in(body())


@bp.post("/signOut")
@legacy(auth=None)
def sign_out():
    return result(service(UserService).sign_out(request.headers.get("HRD-Token")))


@bp.get("/getInfo")
@legacy()
def info(uid):
    return result(service(UserService).info(uid))


@bp.get("/getStorage")
@legacy()
def storage(uid):
    return result(service(UserService).storage_usage(uid))


@bp.post("/updateUsername")
@legacy()
def username(uid):
    return changed(service(UserService).update(uid, "user_name", arg("username")))


@bp.post("/updateEmail")
@legacy()
def email(uid):
    return changed(service(UserService).update(uid, "email", arg("email")))


@bp.post("/updatePassword")
@legacy()
def password(uid):
    return changed(service(UserService).update(uid, "password", arg("newPass"), arg("oldPass")))


@bp.post("/updateAvatar")
@legacy()
def avatar_update(uid):
    return changed(service(UserService).update_avatar(uid, request.files.get("file")))


@bp.get("/getAvatar/<token>")
@legacy(auth="path", shape="binary")
def avatar(uid, token):
    return binary(service(UserService).avatar(uid), "image/jpeg")


@bp.get("/deleteUser")
@legacy()
def delete(uid):
    return changed(service(UserService).delete(uid, uid))


@bp.post("/pingToken")
@legacy()
def ping(uid):
    return result(1)
