import io
import json
import re
from pathlib import Path
from uuid import UUID
from zipfile import ZipFile
from copy import deepcopy
import pytest
import zxingcpp
from PIL import Image
from werkzeug.datastructures import MultiDict
from sqlalchemy.dialects import postgresql
from sqlalchemy import inspect
from app.extensions import db
from app.docs.definitions.ai import OPERATIONS as AI_OPERATIONS
from app.docs.definitions.uploads_storage import OPERATIONS as UPLOAD_OPERATIONS
ADDITIONAL_TABLES = {"user_group", "upload_batch", "upload_item", "smart_preference", "smart_media", "smart_item", "smart_member", "smart_lineage", "photo_ocr", "video_frame_embedding", "embedding_profile", "photo_embedding_state", "ai_job", "ai_control_state"}
from tests.contract_cases import CASES
from tests.openapi_assertions import assert_documented_response

CONTRACTS = Path(__file__).parent / "contracts"
ENTITY_FIELDS = json.loads((CONTRACTS / "spring_entities.json").read_text())
FIELD_TYPES = json.loads((CONTRACTS / "spring_field_types.json").read_text())
SHARE = "http://localhost:8384/photo/renderToken?path=user-alice/photo-one.jpg&token=share-photo"


def normalized(path):
    return re.sub(r"/(?:token|api)-alice$", "/{token}", path)


def entity(value, name):
    assert isinstance(value, dict)
    assert set(value) == set(ENTITY_FIELDS[name])
    for key, item in value.items():
        if item is not None:
            expected_type = {"Integer": int, "String": str, "Double": float, "LocalDateTime": str}[FIELD_TYPES[name][key]]
            assert type(item) is expected_type, (name, key, item)
        if key.endswith("Id"):
            assert item is None or type(item) is int
        if key.endswith("Date") and item is not None:
            assert re.fullmatch(r"\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?", item)
    if name == "User":
        assert value["password"] is None


def request_options(options, jpeg):
    opts = deepcopy(options)
    upload = opts.pop("upload", None)
    if upload == "avatar":
        opts["data"] = {"file": (io.BytesIO(jpeg), "avatar.jpg")}
    elif upload == "one":
        opts["data"] = {"file": (io.BytesIO(jpeg), "uploaded.jpg"), "photo": json.dumps({"nsId": 3})}
    elif upload == "many":
        opts["data"] = MultiDict([("files", (io.BytesIO(jpeg), "one.jpg")), ("files", (io.BytesIO(jpeg), "two.jpg")),
                                  ("photosStr", json.dumps([{"nsId": 3}, {"nsId": 3}]))])
    return opts


# Documented additions to the frozen Spring inventory (see COMPATIBILITY.md).
EXTENSIONS = {("GET", "/photo/ocr"), ("PUT", "/photo/ocr"), ("POST", "/photo/ocr/recognize"), ("GET", "/photo/searchSmart"), ("POST", "/photo/replace"), ("GET", "/admin/pingDatabase"), ("GET", "/admin/databaseInfo"),
              ("GET", "/user/getStorage"), ("GET", "/photo/getCovers"), ("GET", "/photo-extra/getMapPlaces"), ("GET", "/admin/getUsers"), ("GET", "/photo/getByDate"), ("GET", "/photo/getDateDays"), ("GET", "/namespace/getDetails")}


def test_route_inventory_is_exact(app):
    source = json.loads((CONTRACTS / "spring_routes.json").read_text())
    expected = {(row["method"], row["path"]) for row in source["routes"]}
    actual = {(method, re.sub(r"<(?:int:)?(\w+)>", r"{\1}", rule.rule)) for rule in app.url_map.iter_rules()
              for method in rule.methods if method in ("GET", "POST", "PATCH", "PUT", "DELETE") and rule.rule != "/healthz"
              and not rule.endpoint.startswith("docs.")}
    assert len(expected) == len(CASES) == 90
    assert actual == expected | EXTENSIONS | {(method.upper(),path) for path,method,_ in AI_OPERATIONS + UPLOAD_OPERATIONS}
    assert {(method, normalized(path)) for method, path, _, _ in CASES} == expected


def test_table_columns_types_and_primary_keys_match_postgresql_schema(app):
    source = json.loads((CONTRACTS / "postgresql_schema.json").read_text())
    assert set(db.metadata.tables) == set(source) | ADDITIONAL_TABLES
    for name, expected in source.items():
        table = db.metadata.tables[name]
        actual = {column.name: {"type": column.type.compile(dialect=postgresql.dialect()), "nullable": column.nullable}
                  for column in table.columns}
        assert actual == expected["columns"], name
        assert [column.name for column in table.primary_key] == expected["primary_key"]
    with app.app_context():
        inspector = inspect(db.engine)
        assert set(inspector.get_table_names()) == set(source) | ADDITIONAL_TABLES | {"alembic_version"}
        for name, expected in source.items():
            actual = {c["name"]: {"type": c["type"].compile(dialect=postgresql.dialect()),
                                   "nullable": c["nullable"]} for c in inspector.get_columns(name)}
            assert actual == expected["columns"], name
            assert inspector.get_pk_constraint(name)["constrained_columns"] == expected["primary_key"]



@pytest.mark.parametrize("method,path,options,expected", CASES, ids=[f"{r[0]} {r[1]}" for r in CASES])
def test_spring_success_contract(world, method, path, options, expected):
    headers = world.admin if path.startswith("/admin/") else world.headers
    response = world.client.open(path, method=method, headers=headers, **request_options(options, world.jpeg))
    assert response.status_code == 200, response.get_data(as_text=True)
    try:
        assert_documented_response(method, path, response)
        if expected in ("image", "thumbnail", "qr"):
            assert response.mimetype == "image/jpeg"
            image = Image.open(io.BytesIO(response.data))
            if expected == "thumbnail":
                assert image.size == (400, 300)
            elif expected == "qr":
                assert image.size == (250, 250)
                assert zxingcpp.read_barcode(image).text == SHARE
            elif not path.startswith("/user/getAvatar"):
                assert response.data == world.jpeg
            return
        if expected in ("video", "download"):
            assert response.mimetype == "application/octet-stream"
            assert response.data == (world.video if expected == "video" else world.jpeg)
            if expected == "download":
                assert "attachment" in response.headers["Content-Disposition"]
                assert "photo-one.jpg" in response.headers["Content-Disposition"]
            return
        if expected == "zip":
            assert response.mimetype == "application/octet-stream"
            assert "download.zip" in response.headers["Content-Disposition"]
            with ZipFile(io.BytesIO(response.data)) as archive:
                assert archive.namelist() == ["Travel/sunset.jpg"]
                assert archive.read("Travel/sunset.jpg") == world.jpeg
            return
        if expected == "uuid_raw":
            assert response.mimetype == "text/plain"
            UUID(response.text)
            return
        data = response.get_json()
        if expected == "uuids_raw":
            assert len(data) == 2
            for value in data:
                UUID(value)
            return
        if expected == "one_raw":
            assert type(data) is int and data == 1
            return
        if expected == "namespace_raw":
            entity(data, "Namespace")
            assert data["nsId"] == 2
            return
        if expected in ("namespaces_raw", "photos_raw"):
            assert len(data) == 1
            entity(data[0], "Namespace" if expected == "namespaces_raw" else "Photo")
            return
        assert data["msgCode"] == (591023 if expected == "stop" else 200)
        if expected == "token":
            assert set(data) == {"t", "token", "msgCode"}
            assert data["t"] == "Succeed"
            UUID(data["token"])
            return
        if isinstance(expected, str) and expected.endswith("Page"):
            assert set(data) == {"t", "pageNum", "msgCode"}
            assert data["pageNum"] == (1 if expected == "UserSearchPage" else 2)
            name = "User" if expected.startswith("User") else expected[:-4]
            for item in data["t"]:
                entity(item, name)
            return
        assert set(data) == {"t", "msgCode"}
        value = data["t"]
        if expected in ENTITY_FIELDS:
            entity(value, expected)
        elif isinstance(expected, str) and expected.endswith("List"):
            assert value
            for item in value:
                entity(item, expected[:-4])
        elif expected == "addresses":
            assert set(value) == {"mysql", "redis", "upload", "config"}
        elif expected == "settings":
            assert value == {"SignUp": "1", "SSafeUUID": "1", "TokenDuration": "30", "ping": "pong"}
        elif expected == "trace":
            assert value == [{"nsId": 2, "nsName": "/"}, {"nsId": 3, "nsName": "Travel"}]
        elif expected == "tag_links":
            assert value == [{"photo_id": 1, "tag_id": 1, "tag_name": "beach"}]
        elif expected == "gallery_links":
            assert value == [{"photo_id": 1, "ga_id": 2, "ga_name": "Travel"}]
        elif expected == "share":
            assert value == SHARE
        elif expected == "exif":
            assert value["Image_Width"] == "800 pixels"
            assert value["Image_Height"] == "600 pixels"
            assert value["File_Size"].endswith(" MB")
        elif expected == "map":
            assert value
            keys = {"photo_id", "photo_name", "upload_date", "format", "ns_id", "token", "photo_uuid",
                    "extra_photo_gps_lat", "extra_photo_gps_lon", "extra_photo_taken_date"}
            assert set(value[0]) == keys
            assert value[0]["extra_photo_gps_lon"] == -74.006
            assert value[0]["upload_date"] == value[0]["extra_photo_taken_date"] == "2024-03-04"
        elif expected == "positive":
            assert type(value) is int and value > 0
        elif expected == "stop":
            assert value == 1
            assert world.app.extensions["photoly"].lifecycle.calls == 1
        else:
            assert type(value) is type(expected) and value == expected
    finally:
        response.close()


AUTH_CASES = [case for case in CASES if case[1] not in ("/user/signUp", "/user/signIn", "/user/signOut")]


@pytest.mark.parametrize("method,path,options,expected", AUTH_CASES, ids=[f"{r[0]} {r[1]}" for r in AUTH_CASES])
def test_unauthenticated_contract(world, method, path, options, expected):
    opts = request_options(options, world.jpeg)
    bad_path = path.replace("token-alice", "invalid-token").replace("api-alice", "invalid-api")
    if "token" in opts.get("query_string", {}):
        opts["query_string"]["token"] = "invalid"
    response = world.client.open(bad_path, method=method, **opts)
    assert_documented_response(method, path, response)
    if "api-alice" in path:
        assert response.status_code == 200
        if "/uploadPhoto/" in path:
            assert response.text == "Unidentified Token"
        elif "/deletePhoto/" in path:
            assert response.json == -1
        else:
            assert response.data == b""
        return
    if expected in ("image", "thumbnail", "video", "download", "zip", "qr"):
        assert response.status_code == 403
        return
    assert response.status_code == 200
    missing = None
    if path in ("/admin/isAdmin", "/gallery/insert"):
        missing = False
    elif path in ("/namespace/insert", "/tag/insert", "/gallery/delete", "/gallery/update"):
        missing = -1
    elif path.startswith("/admin/") and path not in ("/admin/getAddress", "/admin/getSettings", "/admin/getUserPage", "/admin/searchUserPage", "/admin/getGeneralSetting"):
        missing = -2
    expected_body = {"t": missing, "msgCode": 403}
    if path.endswith("Page"):
        expected_body["pageNum"] = -1 if path in ("/tag/getPage", "/gallery/getPage") else 0
    assert response.json == expected_body


@pytest.mark.parametrize("path,method,options", [
    ("/user/signIn/", "POST", {"json": {"email": "alice@test.local", "password": "password123"}}),
    ("/user/getInfo/", "GET", {}), ("/namespace/getRoot/", "GET", {}),
    ("/user/pingToken/", "POST", {}), ("/user/signOut/", "POST", {}),
])
def test_frontend_trailing_slashes_do_not_redirect(world, path, method, options):
    response = world.client.open(path, method=method, headers=world.headers, **options)
    assert response.status_code == 200
    assert "Location" not in response.headers
    assert response.json["msgCode"] == 200


@pytest.mark.parametrize("path,code", [("/photo/getPath", 403), ("/photo/getPhotoExif", 404)])
def test_missing_photo_defined_business_failure(world, path, code):
    # Spring PhotoController explicitly returned these JSON codes at HTTP 200.
    response = world.client.get(path, headers=world.headers, query_string={"photoId": 999999})
    assert response.status_code == 200
    assert response.json == {"t": None, "msgCode": code}
