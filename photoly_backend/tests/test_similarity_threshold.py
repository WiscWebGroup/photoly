import io
import numpy as np
import pytest
from PIL import Image
from sqlalchemy import select
from app.ai.state import control
from app.entities.ai import PhotoEmbeddingState as State
from tests.test_ai import ai
from tests.test_similar_page import seed, page
from tests.openapi_assertions import assert_documented_response


def set_threshold(ai, value):
    settings = ai.client.get("/admin/ai/settings", headers=ai.admin).json["t"]
    response = ai.client.patch("/admin/ai/settings", headers=ai.admin,
        json={"generation":settings["generation"], "settings":{"similarity_threshold":value}})
    assert response.status_code == 200
    assert response.json["t"]["similarity_threshold"] == value
    assert_documented_response("PATCH", "/admin/ai/settings", response)
    return response.json["t"]


@pytest.mark.parametrize("invalid", [True, False, "0.5", [], {}, -1.01, 1.01, float("nan"), float("inf"), float("-inf")])
def test_invalid_threshold_is_rejected(ai, invalid):
    response=ai.client.patch("/admin/ai/settings", headers=ai.admin,
        json={"generation":0,"settings":{"similarity_threshold":invalid}})
    assert response.status_code==400
    assert response.json["error"]=="invalid_similarity_threshold"
    assert ai.client.get("/admin/ai/settings",headers=ai.admin).json["t"]["generation"]==0


def test_threshold_defaults_authorization_and_capabilities(ai):
    assert ai.client.get("/admin/ai/settings",headers=ai.admin).json["t"]["similarity_threshold"] is None
    assert ai.client.get("/ai/capabilities",headers=ai.headers).json["t"]["similarityThreshold"] is None
    response=ai.client.get("/admin/ai/capabilities",headers=ai.admin)
    assert_documented_response("GET","/admin/ai/capabilities",response)
    rule=response.json["t"]["fields"]["similarity_threshold"]
    assert (rule["default"],rule["minimum"],rule["maximum"],rule["apply"])==(None,-1,1,"next_query")
    for headers,code in (({},401),(ai.headers,403)):
        assert ai.client.patch("/admin/ai/settings",headers=headers,
            json={"generation":0,"settings":{"similarity_threshold":0.5}}).status_code==code
    changed=set_threshold(ai,0.75)
    assert changed["runtime"]=="loaded"  # Hot setting: no unload required.
    assert ai.client.get("/ai/capabilities",headers=ai.headers).json["t"]["similarityThreshold"]==0.75
    assert ai.client.patch("/admin/ai/settings",headers=ai.admin,
        json={"generation":0,"settings":{"similarity_threshold":None}}).status_code==409
    assert set_threshold(ai,None)["similarity_threshold"] is None


def test_threshold_boundaries_negative_scores_and_legacy_image_search(ai):
    ids=seed(ai,3)
    with ai.sessions.begin() as s:
        for photo_id,xy in zip(ids,[(1,0),(0,1),(-1,0)]):
            vector=np.zeros(768,dtype=np.float32);vector[:2]=xy
            state=s.scalar(select(State).where(State.photo_id==photo_id,State.profile_id==ai.ai_pid))
            state.embedding=vector
    for threshold,expected in ((None,ids),(-1,ids),(0,ids[:2]),(1,ids[:1]),(None,ids)):
        set_threshold(ai,threshold)
        response=page(ai)
        assert response.status_code==200
        result=response.json["t"]
        assert [x["photo"]["photoId"] for x in result["items"]]==expected
        assert result["similarityThreshold"]==threshold
        assert result["hasMore"] is False and result["nextCursor"] is None
        legacy=ai.client.post("/ai/photos/1/similar",headers=ai.headers,json={}).json["t"]
        assert [x["photo"]["photoId"] for x in legacy["items"]]==expected
    set_threshold(ai,0)
    stream=io.BytesIO();Image.new("RGB",(8,8)).save(stream,format="JPEG");stream.seek(0)
    response=ai.client.post("/ai/search/image",headers=ai.headers,data={"file":(stream,"query.jpg")})
    assert response.status_code==200
    assert response.json["t"]["similarityThreshold"]==0
    assert ids[2] not in [x["photo"]["photoId"] for x in response.json["t"]["items"]]
    text=ai.client.post("/ai/search/text",headers=ai.headers,json={"text":"test"}).json["t"]
    assert text["similarityThreshold"] is None
    assert ids[2] in [x["photo"]["photoId"] for x in text["items"]]


def collect(ai):
    data=page(ai,{"pageSize":24}).json["t"]
    items=list(data["items"])
    while data["hasMore"]:
        response=page(ai,{"cursor":data["nextCursor"]})
        assert response.status_code==200
        data=response.json["t"];items.extend(data["items"])
    assert data["nextCursor"] is None
    return items


def test_threshold_seek_pagination_filters_before_limit_and_includes_equal_scores(ai):
    seed(ai,127)
    baseline=collect(ai)
    threshold=baseline[60]["score"]
    set_threshold(ai,threshold)
    ai.runtime.after_encode=lambda:pytest.fail("Pagination should reuse the source vector")
    filtered=collect(ai)
    assert filtered==[hit for hit in baseline if hit["score"]>=threshold]
    ids=[x["photo"]["photoId"] for x in filtered]
    assert 24<len(ids)<127 and len(set(ids))==len(ids)
    # No matching candidates is a successful, terminal page.
    with ai.sessions.begin() as s:
        vector=np.zeros(768,dtype=np.float32);vector[2]=1
        s.scalar(select(State).where(State.photo_id==1,State.profile_id==ai.ai_pid)).embedding=vector
    result=page(ai).json["t"]
    assert result["items"]==[] and result["hasMore"] is False


@pytest.mark.parametrize("old,new",[(None,0.95),(0.95,0.9),(0.95,None)])
def test_threshold_change_invalidates_cursor_without_encoding(ai,old,new):
    seed(ai,127)
    set_threshold(ai,old)
    cursor=page(ai).json["t"]["nextCursor"]
    assert cursor
    set_threshold(ai,new)
    ai.runtime.after_encode=lambda:pytest.fail("Invalid cursor must not invoke runtime")
    response=page(ai,{"cursor":cursor})
    assert response.status_code==409 and response.json["error"]=="search_settings_changed"
    assert page(ai).json["t"]["similarityThreshold"]==new


def test_unrelated_setting_keeps_cursor_valid(ai):
    seed(ai,40)
    cursor=page(ai).json["t"]["nextCursor"]
    response=ai.client.patch("/admin/ai/settings",headers=ai.admin,
        json={"generation":0,"settings":{"background_interval_seconds":10}})
    assert response.status_code==200
    assert page(ai,{"cursor":cursor}).status_code==200


def test_threshold_changed_during_encoding_uses_latest_value(ai):
    seed(ai,30,source_ready=False)
    def change():
        with ai.sessions.begin() as s:
            row=control(s,lock=True)
            row.desired={**row.desired,"similarity_threshold":1}
            row.generation+=1
    ai.runtime.after_encode=change
    result=page(ai).json["t"]
    assert result["similarityThreshold"]==1
    assert len(result["items"])==3 and all(x["score"]==1 for x in result["items"])
