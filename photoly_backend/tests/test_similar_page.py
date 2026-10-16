import json
import numpy as np
import pytest
from sqlalchemy import select
from app.entities import Photo, User
from app.entities.ai import PhotoEmbeddingState as State
from app.ai.state import control
from app.ai.similar_cursor import PREFIX
from tests.test_ai import ai, ready
from tests.openapi_assertions import assert_documented_response

PATH = "/ai/photos/1/similar/page"


def seed(ai, count=127, source_ready=True):
    with ai.sessions.begin() as s:
        if source_ready: ready(s,ai.ai_pid)
        ids=[]
        for i in range(count):
            p=Photo(photo_name=f"page-{i}",format="jpg",ns_id=2,user_id=2)
            s.add(p);s.flush();ids.append(p.photo_id)
            vector=np.zeros(768,dtype=np.float32);vector[0]=1;vector[1]=i//3 / count
            ready(s,ai.ai_pid,p.photo_id,vector)
        # Another user's vector must never appear.
        p=Photo(photo_name="private",format="jpg",ns_id=4,user_id=3)
        s.add(p);s.flush();ready(s,ai.ai_pid,p.photo_id)
    return ids


def page(ai, data=None, headers=None):
    return ai.client.post(PATH,json=data or {},headers=headers or ai.headers)


def test_seek_more_than_100_ties_retries_and_no_reencoding(ai):
    expected=seed(ai,source_ready=False)
    calls=[];ai.runtime.after_encode=lambda:calls.append(1)
    response=page(ai)
    assert_documented_response("POST","/ai/photos/{photoId}/similar/page",response)
    assert response.status_code==200
    data=response.json["t"];ids=[x["photo"]["photoId"] for x in data["items"]]
    first_cursor=data["nextCursor"]
    while data["hasMore"]:
        response=page(ai,{"cursor":data["nextCursor"]});assert response.status_code==200
        data=response.json["t"];ids.extend(x["photo"]["photoId"] for x in data["items"])
    assert ids==expected and len(set(ids))==127
    assert calls==[1]
    assert data["nextCursor"] is None
    retry=page(ai,{"cursor":first_cursor}).json["t"]
    assert [x["photo"]["photoId"] for x in retry["items"]]==expected[24:48]
    assert calls==[1]


def test_reuse_ready_source_and_invalid_cursor_bindings(ai):
    seed(ai,30)
    ai.runtime.after_encode=lambda:pytest.fail("ready SigLIP vector should be reused")
    first=page(ai).json["t"]
    cursor=first["nextCursor"]
    assert page(ai,{"cursor":cursor},ai.other).status_code==400
    with ai.app.app_context():
        redis=ai.app.extensions["photoly"].sessions.client
        redis.setex("photoly:session:second-alice",60,2)
    assert page(ai,{"cursor":cursor},{"HRD-Token":"second-alice"}).status_code==400
    assert ai.client.post('/ai/photos/2/similar/page',json={"cursor":cursor},headers=ai.headers).status_code==400
    assert page(ai,{"cursor":"not-a-cursor"}).status_code==400
    assert page(ai,{"cursor":"a"*32}).json["error"]=="cursor_expired"
    with ai.app.app_context():
        redis.delete(PREFIX+"page:"+cursor)
    assert page(ai,{"cursor":cursor}).json["error"]=="cursor_expired"


@pytest.mark.parametrize("data",[{"pageSize":0},{"pageSize":61},{"pageSize":True},{"pageSize":1.5},{"cursor":None},{"cursor":"a"*32,"pageSize":24},{"topK":20}])
def test_page_validation(ai,data):
    assert page(ai,data).status_code==400


@pytest.mark.parametrize("change,code",[("disable","ai_not_enabled"),("version","stale_content"),("delete","photo_not_found"),("profile","profile_changed")])
def test_context_invalidated(ai,change,code):
    seed(ai,30);cursor=page(ai).json["t"]["nextCursor"]
    with ai.sessions.begin() as s:
        if change=="disable":s.get(User,2).ai_search_enabled=False
        elif change=="version":s.get(Photo,1).content_version+=1
        elif change=="delete":s.delete(s.get(Photo,1))
        else:
            from app.services.ai_admin import AIAdmin
            from app.entities.ai import EmbeddingProfile
            p=AIAdmin(s).create_profile("qwen3_vl")["profileId"]
            spec=s.get(EmbeddingProfile,p);spec.index_status="ready"
            row=control(s);row.desired={**row.desired,"search_profile_id":p}
            ai.runtime.spec=dict(spec.spec)
    assert page(ai,{"cursor":cursor}).json["error"]==code


def test_deleted_and_stale_results_not_returned(ai):
    ids=seed(ai,40);first=page(ai).json["t"]
    with ai.sessions.begin() as s:
        s.delete(s.get(Photo,ids[25]));s.get(Photo,ids[26]).content_version+=1
    remaining=page(ai,{"cursor":first["nextCursor"]}).json["t"]
    returned=[x["photo"]["photoId"] for x in remaining["items"]]
    assert ids[25] not in returned and ids[26] not in returned
    assert set(returned).isdisjoint(x["photo"]["photoId"] for x in first["items"])


def test_source_replace_during_encode_rejected(ai):
    seed(ai,30,source_ready=False)
    def replace():
        with ai.sessions.begin() as s:s.get(Photo,1).content_version+=1
    ai.runtime.after_encode=replace
    assert page(ai).json["error"]=="stale_content"
