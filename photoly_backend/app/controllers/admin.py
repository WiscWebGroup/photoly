from flask import Blueprint
from app.controllers.common import legacy, service, arg, body, changed
from app.services.admin import AdminService
from app.serialization import result

bp = Blueprint("admin", __name__, url_prefix="/admin")


@bp.get("/isAdmin")
@legacy(missing=False)
def is_admin(uid):
    return result(service(AdminService).is_admin(uid))


@bp.get("/pingMySQL")
@legacy(missing=-2)
def ping_mysql(uid):
    return changed(service(AdminService).ping_mysql(uid))


@bp.get("/pingDatabase")
@legacy(missing=-2)
def ping_database(uid):
    return changed(service(AdminService).ping_database(uid))


@bp.get("/databaseInfo")
@legacy()
def database_info(uid):
    return result(service(AdminService).database_info(uid))


@bp.get("/pingRedis")
@legacy(missing=-2)
def ping_redis(uid):
    return changed(service(AdminService).ping_redis(uid))


@bp.get("/getAddress")
@legacy()
def addresses(uid):
    if not service(AdminService).is_admin(uid):
        return result(None, 400)
    return result(service(AdminService).addresses(uid))


@bp.get("/getSettings")
@legacy()
def settings(uid):
    if not service(AdminService).is_admin(uid):
        return result(None, 400)
    return result(service(AdminService).settings(uid))


@bp.post("/stop")
@legacy(missing=-2)
def stop(uid):
    return result(service(AdminService).stop(uid), 591023)


@bp.post("/setSignUpPermission")
@legacy(missing=-2)
def signup(uid):
    return changed(service(AdminService).set_setting(uid, "SignUp", arg("permission", integer=True)))


@bp.post("/setSSafeUUIDMode")
@legacy(missing=-2)
def safe_uuid(uid):
    return changed(service(AdminService).set_setting(uid, "SSafeUUID", arg("mode", integer=True)))


@bp.post("/setTokenDuration")
@legacy(missing=-2)
def duration(uid):
    return changed(service(AdminService).set_setting(uid, "TokenDuration", arg("days", integer=True)))


@bp.get("/getUserPage")
@legacy(shape="page")
def users(uid):
    return service(AdminService).users(uid, arg("page", integer=True), arg("rowsPerPage", integer=True))


@bp.get("/searchUserPage")
@legacy(shape="page")
def search_users(uid):
    return service(AdminService).users(uid, arg("page", integer=True), arg("rowsPerPage", integer=True), arg("search"))


@bp.get("/getUsers")
@legacy(missing=-2)
def user_list(uid):
    return result(service(AdminService).user_list(uid, arg("page", integer=True), arg("rowsPerPage", integer=True),
                                                  arg("search"), arg("role"), arg("groupId", integer=True)))


@bp.post("/addUser")
@legacy(missing=-2)
def add_user(uid):
    return changed(service(AdminService).add_user(uid, body()))


@bp.post("/deleteUser")
@legacy(missing=-2)
def delete_user(uid):
    return changed(service(AdminService).delete_user(uid, arg("userId", integer=True)))


@bp.post("/resetUserPassword")
@legacy(missing=-2)
def reset_password(uid):
    return changed(service(AdminService).reset(uid, body(), "password"))


@bp.post("/resetUserEmail")
@legacy(missing=-2)
def reset_email(uid):
    return changed(service(AdminService).reset(uid, body(), "email"))


@bp.post("/resetUserRole")
@legacy(missing=-2)
def reset_role(uid):
    return changed(service(AdminService).reset(uid, body(), "role"))


@bp.post("/resetUsername")
@legacy(missing=-2)
def reset_name(uid):
    return changed(service(AdminService).reset(uid, body(), "user_name"))


@bp.post("/getGeneralSetting")
@legacy()
def get_general(uid):
    if not service(AdminService).is_admin(uid):
        return result(None, 400)
    value = service(AdminService).get_setting(uid, arg("name"))
    return result(value, 200 if value is not None else 400)


@bp.post("/setGeneralSetting")
@legacy(missing=-2)
def set_general(uid):
    return changed(service(AdminService).set_general_setting(uid, arg("name"), arg("val", required=True)))


from app.services.quota import QuotaService


@bp.get('/user-groups')
@legacy()
def groups(uid):
    return result(service(QuotaService).groups(uid))


@bp.post('/user-groups')
@legacy()
def create_group(uid):
    return result(service(QuotaService).save_group(uid, body()))


@bp.put('/user-groups/<int:group_id>')
@legacy()
def edit_group(uid, group_id):
    return result(service(QuotaService).save_group(uid, body(), group_id))


@bp.delete('/user-groups/<int:group_id>')
@legacy()
def delete_group(uid, group_id):
    return result(service(QuotaService).delete_group(uid, group_id, body().get('reassignTo')))


@bp.put('/users/<int:user_id>/storage')
@legacy()
def set_user_storage(uid, user_id):
    return result(service(QuotaService).set_user(uid, user_id, body()))
