"""Real HTTP/Gunicorn acceptance tests. Only run against the disposable test Compose."""
import io
import json
import os
import time
from concurrent.futures import ThreadPoolExecutor
from uuid import uuid4
from zipfile import ZipFile
import pytest
import requests

pytestmark = pytest.mark.http


@pytest.fixture
def http():
    base = os.getenv("PHOTOLY_TEST_HTTP_URL")
    if not base:
        pytest.skip("Run through compose.test.yml smoke service")
    # There is intentionally no default production URL for these destructive tests.
    assert base == "http://backend:8084"
    def call(method, path, **kwargs):
        return requests.request(method, base + path, timeout=30, **kwargs)
    return call


def register(http):
    email = "smoke-" + uuid4().hex + "@test.local"
    response = http("POST", "/user/signUp/", json={"email": email, "password": "smoke-password", "userName": "Smoke"})
    assert response.status_code == 200 and response.json()["msgCode"] == 200
    token = response.json()["token"]
    return {"HRD-Token": token}, token


def test_http_full_workflow(http, jpeg, video_bytes):
    assert http("GET", "/healthz").json() == {"status": "ok"}
    headers, token = register(http)
    root = http("GET", "/namespace/getRoot/", headers=headers).json()["t"]["nsId"]
    assert http("POST", "/namespace/insert", headers=headers, json={"nsName": "Trip", "nsParentId": root}).json()["t"] == 1
    folder = http("GET", "/namespace/getChildren", headers=headers, params={"parentId": root}).json()["t"][0]["nsId"]
    for filename, content in (("sunset.jpg", jpeg), ("movie.mp4", video_bytes)):
        response = http("POST", "/photo/insert", headers=headers, files={"file": (filename, content)}, data={"photo": json.dumps({"nsId": folder, "visibility": 1})})
        assert response.json() == {"t": 1, "msgCode": 200}
    photos = http("GET", "/photo/getByNamespace", headers=headers, params={"nsId": folder}).json()["t"]
    photo, video = photos
    assert http("GET", f"/photo/render/{token}", params={"photoId": photo["photoId"]}).content == jpeg
    ranged = http("GET", f"/photo/renderV/{token}", params={"photoId": video["photoId"]}, headers={"Range": "bytes=4-11"})
    assert ranged.status_code == 206 and ranged.content == video_bytes[4:12]
    assert http("POST", "/tag/insert", headers=headers, json={"tagName": "sunset"}).json()["t"] == 1
    tag = http("GET", "/tag/getAll", headers=headers).json()["t"][0]["tagId"]
    assert http("POST", "/photo/addTags", headers=headers, params={"photoId": photo["photoId"], "tagIds": tag}).json()["t"] == 1
    records = http("GET", "/photo/searchPhoto", headers=headers, params={"option": "tag", "query": "sun"}).json()["t"]
    assert [p["photoId"] for p in records] == [photo["photoId"]]
    url = http("GET", "/photo/getPath", headers=headers, params={"photoId": photo["photoId"]}).json()["t"]
    assert requests.get(url, timeout=30).content == jpeg
    assert http("POST", "/cred/create", headers=headers, json={"authorization": "CRD"}).json()["t"] == 1
    api = http("GET", "/cred/query", headers=headers).json()["t"][0]["token"]
    assert http("GET", "/cred/queryRootNamespace/" + api).json()["nsId"] == root
    archive = http("GET", "/namespace/downloadNs/" + token, params={"nsId": folder})
    with ZipFile(io.BytesIO(archive.content)) as zipped:
        assert set(zipped.namelist()) == {"Trip/sunset.jpg", "Trip/movie.mp4"}
        assert zipped.read("Trip/sunset.jpg") == jpeg
    assert http("POST", "/namespace/delete", headers=headers, params={"nsId": folder}).json()["msgCode"] == 200
    assert http("GET", "/photo/getByNamespace", headers=headers, params={"nsId": folder}).json()["t"] == []
    assert http("GET", "/user/deleteUser", headers=headers).json()["msgCode"] == 200
    assert http("POST", "/user/pingToken", headers=headers).json()["msgCode"] == 403
    assert http("GET", "/cred/queryRootNamespace/" + api).content == b""


def test_http_concurrent_moves_cannot_create_cycle(http):
    headers, _ = register(http)
    root = http("GET", "/namespace/getRoot", headers=headers).json()["t"]["nsId"]
    for name in ("A", "B"):
        http("POST", "/namespace/insert", headers=headers, json={"nsName": name, "nsParentId": root})
    children = http("GET", "/namespace/getChildren", headers=headers, params={"parentId": root}).json()["t"]
    a, b = [child["nsId"] for child in children]
    def move(pair):
        return http("POST", "/namespace/updateParent", headers=headers, json={"nsId": pair[0], "nsParentId": pair[1]}).json()["msgCode"]
    with ThreadPoolExecutor(max_workers=2) as pool:
        codes = list(pool.map(move, [(a, b), (b, a)]))
    assert sorted(codes) == [200, 400]
    for child in (a, b):
        assert http("GET", "/namespace/trace", headers=headers, params={"nsId": child}).json()["msgCode"] == 200
    assert http("GET", "/user/deleteUser", headers=headers).json()["msgCode"] == 200


def test_http_openapi_and_upload_contract(http, jpeg):
    from openapi_spec_validator import validate
    for path in ("/docs", "/docs/"):
        page = http("GET", path, allow_redirects=False)
        assert page.status_code == 200 and 'swagger-ui-bundle.js' in page.text
    spec = http("GET", "/openapi.json").json()
    validate(spec)
    from pathlib import Path
    legacy = json.loads((Path(__file__).parent / "contracts/spring_routes.json").read_text())
    expected = {r["path"] for r in legacy["routes"]} | {"/healthz", "/photo/replace", "/admin/pingDatabase", "/admin/databaseInfo",
        "/photo/searchSmart", "/photo/ocr", "/photo/ocr/recognize", "/user/getStorage", "/photo/getCovers",
        "/photo-extra/getMapPlaces", "/admin/getUsers"}
    from app.docs.definitions.ai import OPERATIONS as AI_OPERATIONS
    expected |= {path for path, _, _ in AI_OPERATIONS}
    assert spec["openapi"] == "3.2.0" and set(spec["paths"]) == expected
    for asset in ("vendor/swagger-ui-bundle.js", "vendor/swagger-ui.css", "swagger-init.js"):
        assert http("GET", "/docs/assets/" + asset).status_code == 200
    headers, _ = register(http)
    root = http("GET", "/namespace/getRoot", headers=headers).json()["t"]["nsId"]
    response = http("POST", "/photo/inserts", headers=headers,
                    files=[("files", ("first.jpg", jpeg)), ("files", ("second.jpg", jpeg))],
                    data={"photosStr": json.dumps([{"nsId": root}, {"nsId": root}])})
    assert response.json() == {"t": 2, "msgCode": 200}
    assert len(http("GET", "/photo/getByNamespace", headers=headers, params={"nsId": root}).json()["t"]) == 2
    assert http("GET", "/user/deleteUser", headers=headers).json()["msgCode"] == 200


def test_http_z_admin_stop_terminates_all_workers(http):
    response = http("POST", "/user/signIn", json={"email": "admin@test.local", "password": "test-password-only"})
    assert response.json()["msgCode"] == 200
    headers = {"HRD-Token": response.json()["token"]}
    response = http("POST", "/admin/stop", headers=headers)
    assert response.json() == {"t": 1, "msgCode": 591023}
    deadline = time.monotonic() + 15
    while time.monotonic() < deadline:
        try:
            http("GET", "/healthz")
        except requests.ConnectionError:
            return
        time.sleep(0.25)
    pytest.fail("Gunicorn master remained reachable after /admin/stop")
