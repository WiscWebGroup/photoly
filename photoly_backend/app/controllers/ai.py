from functools import wraps
from flask import Blueprint, request, current_app, jsonify
from sqlalchemy import select
from app.extensions import db
from app.entities import User
from app.entities.ai import AIJob
from app.ai.types import AIError
from app.ai.preprocessing import read_bounded
from app.services.user import UserService
from app.services.ai_search import AISearch
from app.services.ai_admin import AIAdmin, job_dto
from app.serialization import serialize

bp=Blueprint("ai",__name__)


def ai_route(admin=False):
    def decorate(function):
        @wraps(function)
        def call(*args,**kwargs):
            try:
                infra=current_app.extensions["photoly"]
                uid=UserService(db.session,infra).identify(request.headers.get("HRD-Token"))
                if uid is None:raise AIError("unauthenticated",401)
                if admin and db.session.get(User,uid).role!="admin":raise AIError("admin_required",403)
                result=function(uid,*args,**kwargs)
                status=202 if request.path=="/admin/ai/operations" and request.method=="POST" else 200
                db.session.commit()
                return jsonify(t=serialize(result),msgCode=status),status
            except AIError as error:
                db.session.rollback()
                return jsonify(t=None,msgCode=error.status,error=error.code),error.status
        return call
    return decorate


def body():
    value=request.get_json(silent=True)
    if not isinstance(value,dict):raise AIError("invalid_json")
    return value


def search_service():
    return AISearch(db.session,current_app.extensions["photoly"],current_app.config.get("AI_RUNTIME_CLIENT"))


@bp.get("/ai/capabilities")
@ai_route()
def capabilities(uid):return search_service().capabilities(uid)


@bp.post("/ai/search/text")
@ai_route()
def text_search(uid):return search_service().search(uid,body())


@bp.post("/ai/search/image")
@ai_route()
def image_search(uid):
    if "file" not in request.files:raise AIError("image_required")
    data={}
    for key in ("topK","namespaceId","galleryId"):
        if key in request.form:
            try:data[key]=int(request.form[key])
            except ValueError:raise AIError("invalid_filter")
    return search_service().search(uid,data,image=read_bounded(request.files["file"].stream))


@bp.post("/ai/photos/<int:photoId>/similar")
@ai_route()
def similar(uid,photoId):return search_service().search(uid,body(),photo_id=photoId)


@bp.post("/ai/photos/<int:photoId>/similar/page")
@ai_route()
def similar_page(uid,photoId):
    return search_service().page(uid,body(),photoId,request.headers.get("HRD-Token"))


@bp.get("/admin/ai/status")
@ai_route(admin=True)
def status(uid):return AIAdmin(db.session).status()


@bp.get("/admin/ai/capabilities")
@ai_route(admin=True)
def admin_capabilities(uid):return AIAdmin(db.session).capabilities()


@bp.get("/admin/ai/settings")
@ai_route(admin=True)
def get_settings(uid):return AIAdmin(db.session).get_settings()


@bp.patch("/admin/ai/settings")
@ai_route(admin=True)
def patch_settings(uid):
    data=body()
    return AIAdmin(db.session).patch_settings(data.get("generation"),data.get("settings"))


@bp.get("/admin/ai/profiles")
@ai_route(admin=True)
def profiles(uid):return AIAdmin(db.session).profiles()


@bp.post("/admin/ai/profiles")
@ai_route(admin=True)
def create_profile(uid):
    data=body()
    if set(data)!={"provider"}:raise AIError("invalid_profile")
    return AIAdmin(db.session).create_profile(data["provider"])


@bp.patch("/admin/users/<int:userId>/ai")
@ai_route(admin=True)
def set_user(uid,userId):return AIAdmin(db.session).set_user(userId,body().get("enabled"))


@bp.get("/admin/ai/users")
@ai_route(admin=True)
def users(uid):
    try:ids=[int(x) for x in request.args.get("ids","").split(",") if x]
    except ValueError:raise AIError("invalid_user_ids")
    return AIAdmin(db.session).user_states(ids)


@bp.post("/admin/ai/operations")
@ai_route(admin=True)
def operate(uid):return AIAdmin(db.session).operation(uid,body(),request.headers.get("Idempotency-Key"))


@bp.get("/admin/ai/operations/<int:operationId>")
@ai_route(admin=True)
def operation(uid,operationId):
    job=db.session.get(AIJob,operationId)
    if not job or job.kind!="operation":raise AIError("operation_not_found",404)
    return job_dto(job)


@bp.get("/admin/ai/jobs")
@ai_route(admin=True)
def jobs(uid):
    try:limit,after=int(request.args.get("limit",50)),int(request.args.get("after",0))
    except ValueError:raise AIError("invalid_pagination")
    if not 1<=limit<=100 or after<0:raise AIError("invalid_pagination")
    order=request.args.get("order","asc")
    try:before=int(request.args.get("before",0))
    except ValueError:raise AIError("invalid_pagination")
    if order not in ("asc","desc") or before<0 or (order=="desc" and after) or (order=="asc" and before):raise AIError("invalid_pagination")
    query=select(AIJob)
    if order=="asc":query=query.where(AIJob.job_id>after)
    elif before:query=query.where(AIJob.job_id<before)
    for key,column in (("status",AIJob.status),("kind",AIJob.kind)):
        if key in request.args:query=query.where(column==request.args[key])
    rows=db.session.scalars(query.order_by(AIJob.job_id.desc() if order=="desc" else AIJob.job_id).limit(limit)).all()
    return {"items":[job_dto(job) for job in rows],"nextAfter":rows[-1].job_id if len(rows)==limit and order=="asc" else None,"nextBefore":rows[-1].job_id if len(rows)==limit and order=="desc" else None}


# Opt-in organization stays editable when automatic AI processing is disabled.
from app.services.smart import SmartService


@bp.get('/ai/smart/settings')
@ai_route()
def smart_settings(uid): return SmartService(db.session).get_settings(uid)


@bp.patch('/ai/smart/settings')
@ai_route()
def smart_patch_settings(uid): return SmartService(db.session).patch_settings(uid, body())


@bp.get('/ai/smart/status')
@ai_route()
def smart_status(uid): return SmartService(db.session).status(uid)


@bp.post('/ai/smart/refresh')
@ai_route()
def smart_refresh(uid): return SmartService(db.session).refresh(uid)


@bp.get('/ai/smart/dismissed')
@ai_route()
def smart_dismissed(uid):
    try: after = int(request.args.get('after', 0))
    except ValueError: raise AIError('invalid_pagination')
    if after < 0: raise AIError('invalid_pagination')
    return SmartService(db.session).dismissed(uid, after)


@bp.post('/ai/smart/items/<int:itemId>/restore')
@ai_route()
def smart_restore(uid, itemId): return SmartService(db.session).restore(uid, itemId)
