import io
import json
import subprocess
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
from zipfile import ZipFile
import pytest
from PIL import Image
from sqlalchemy import select, func
from werkzeug.datastructures import MultiDict
from werkzeug.security import check_password_hash
from app import create_app
from app.cli import initialize
from app.entities import User, Photo, PhotoExtra, Namespace, Gallery, GalleryPhoto, Tag, TagPhoto, Cred, NamespacePhoto, Setting
from app.extensions import db
from app.services.namespace import NamespaceService
from app.services.photo import PhotoService
from app.services.base import BusinessError


def post(world, path, **kwargs):
    return world.client.post(path, headers=world.headers, **kwargs)


def test_registration_login_password_and_logout(world):
    response = world.client.post("/user/signUp", json={"email": "new@test.local", "password": "secret", "userName": "New", "role": "admin", "userId": 1})
    assert response.json["msgCode"] == 200
    headers = {"HRD-Token": response.json["token"]}
    user = world.client.get("/user/getInfo", headers=headers).json["t"]
    assert user["role"] == "user" and user["userId"] != 1 and user["password"] is None
    assert world.client.get("/namespace/getRoot", headers=headers).json["t"]["nsName"] == "/"
    assert world.client.get("/gallery/getAll", headers=headers).json["t"][0]["gaName"] == "My Favorite"
    with world.app.app_context():
        stored = db.session.get(User, user["userId"]).password
        assert stored != "secret" and check_password_hash(stored, "secret")
    failure = world.client.post("/user/signIn", json={"email": "new@test.local", "password": "wrong"}).json
    assert failure == {"t": "Invalid Email or Password", "token": None, "msgCode": 400}
    assert world.client.post("/user/updatePassword", headers=headers, data={"oldPass": "bad", "newPass": "new"}).json == {"t": -2, "msgCode": 400}
    assert world.client.post("/user/updatePassword", headers=headers, data={"oldPass": "secret", "newPass": "new"}).json == {"t": 1, "msgCode": 200}
    assert world.client.post("/user/signIn", json={"email": "new@test.local", "password": "new"}).json["msgCode"] == 200
    world.client.post("/user/signOut", headers=headers)
    assert world.client.post("/user/pingToken", headers=headers).json == {"t": None, "msgCode": 403}


@pytest.mark.parametrize("data,message,code", [
    ({"password": "pw"}, "Email is null", 400),
    ({"email": "invalid", "password": "pw"}, "Email is incorrect", 400),
    ({"email": "alice@test.local", "password": "pw"}, "Repeated Email Address", 401),
])
def test_registration_errors(world, data, message, code):
    response = world.client.post("/user/signUp", json=data)
    assert response.status_code == 200
    assert response.json == {"t": message, "token": None, "msgCode": code}


def test_settings_expiry_and_closed_registration(world):
    world.client.post("/admin/setSignUpPermission", headers=world.admin, data={"permission": 0})
    response = world.client.post("/user/signUp", json={"email": "new@test.local", "password": "pw"})
    assert response.json == {"t": "Register is closed by Admin", "token": None, "msgCode": 400}
    world.client.post("/admin/setTokenDuration", headers=world.admin, data={"days": 2})
    token = world.client.post("/user/signIn", json={"email": "alice@test.local", "password": "password123"}).json["token"]
    sessions = world.app.extensions["photoly"].sessions
    if hasattr(sessions, "engine"):
        with sessions.engine.begin() as c:
            left = c.exec_driver_sql("SELECT extract(epoch FROM expires_at - now()) FROM photoly_session WHERE token=%s", (token,)).scalar()
            assert 172790 <= left <= 172800
            c.exec_driver_sql("UPDATE photoly_session SET expires_at=now() WHERE token=%s", (token,))
    else:
        assert 172790 <= sessions.client.ttl("photoly:session:" + token) <= 172800
        sessions.client.expire("photoly:session:" + token, 0)
    assert world.client.post("/user/pingToken", headers={"HRD-Token": token}).json["msgCode"] == 403
    assert world.client.post("/admin/setTokenDuration", headers=world.admin, data={"days": 366}).json == {"t": -1, "msgCode": 403}


@pytest.mark.parametrize("path,method,options", [
    ("/namespace/getNameById", "GET", {"query_string": {"nsId": 3}}),
    ("/namespace/trace", "GET", {"query_string": {"nsId": 3}}),
    ("/namespace/updateName", "POST", {"json": {"nsId": 3, "nsName": "bad"}}),
    ("/namespace/updateParent", "POST", {"json": {"nsId": 3, "nsParentId": 4}}),
    ("/namespace/insert", "POST", {"json": {"nsName": "bad", "nsParentId": 3}}),
    ("/namespace/delete", "POST", {"query_string": {"nsId": 3}}),
    ("/photo/update", "POST", {"json": {"photoId": 1, "photoName": "bad"}}),
    ("/photo/delete", "POST", {"query_string": {"photoId": 1}}),
    ("/photo/changeNamespace", "POST", {"query_string": {"photoId": 1, "nsId": 4}}),
    ("/photo/addTags", "POST", {"query_string": {"photoId": 1, "tagIds": 1}}),
    ("/photo/deleteTag", "POST", {"query_string": {"photoId": 1, "tagId": 1}}),
    ("/photo/addToGallery", "POST", {"query_string": {"photoId": 1, "gaId": 2}}),
    ("/photo/deleteFromGallery", "POST", {"query_string": {"photoId": 1, "gaId": 2}}),
    ("/photo/getByGallery", "GET", {"query_string": {"gaId": 2}}),
    ("/photo/getTagByPhoto", "GET", {"query_string": {"photoId": 1}}),
    ("/photo/getGalleryByPhoto", "GET", {"query_string": {"photoId": 1}}),
    ("/photo/getPath", "GET", {"query_string": {"photoId": 1}}),
    ("/photo/getPhotoExif", "GET", {"query_string": {"photoId": 1}}),
    ("/gallery/delete", "POST", {"query_string": {"gaId": 2}}),
    ("/gallery/update", "POST", {"json": {"gaId": 2, "gaName": "bad"}}),
    ("/tag/update", "POST", {"json": {"tagId": 1, "tagName": "bad"}}),
    ("/tag/delete", "POST", {"query_string": {"tagId": 1}}),
    ("/cred/update", "POST", {"json": {"credId": 1, "authorization": "D"}}),
    ("/cred/delete", "POST", {"query_string": {"credId": 1}}),
    ("/photo-extra/getGlobalMapPhotoByGallery", "GET", {"query_string": {"gaId": 2}}),
])
def test_cross_user_access_rejected(world, path, method, options):
    response = world.client.open(path, method=method, headers=world.other, **options)
    assert response.status_code == 200
    expected = {"/photo/getPath": 403, "/photo/getPhotoExif": 404}.get(path, 400)
    assert response.json["msgCode"] == expected
    with world.app.app_context():
        assert db.session.get(Photo, 1).photo_name == "sunset"


def test_cross_user_read_filters_and_binary(world):
    assert world.client.get("/photo/getByNamespace?nsId=3", headers=world.other).json == {"t": [], "msgCode": 200}
    assert world.client.get("/photo/searchPhoto?option=photoName&query=sun", headers=world.other).json == {"t": [], "msgCode": 404}
    assert world.client.get("/photo-extra/getGlobalMapPhoto", headers=world.other).json == {"t": [], "msgCode": 200}
    for url in ("/photo/render/token-bob?photoId=1", "/namespace/downloadNs/token-bob?nsId=3", "/gallery/downloadGa/token-bob?gaId=2"):
        assert world.client.get(url).status_code in (400, 403)


@pytest.mark.parametrize("path,options", [
    ("/admin/pingMySQL", {}), ("/admin/pingRedis", {}), ("/admin/getAddress", {}),
    ("/admin/getSettings", {}), ("/admin/getUserPage", {"query_string": {"page": 1, "rowsPerPage": 10}}),
])
def test_admin_read_requires_admin(world, path, options):
    assert world.client.get(path, headers=world.headers, **options).json["msgCode"] == 400


def test_admin_writes_and_safe_stop(world):
    assert post(world, "/admin/setGeneralSetting", data={"name": "SignUp", "val": "0"}).json == {"t": -1, "msgCode": 400}
    assert post(world, "/admin/resetUserRole", json={"userId": 2, "role": "admin"}).json == {"t": -2, "msgCode": 400}
    assert post(world, "/admin/stop").json == {"t": -2, "msgCode": 591023}
    assert world.app.extensions["photoly"].lifecycle.calls == 0


def test_tree_cycle_and_root_protection(world):
    post(world, "/namespace/insert", json={"nsName": "Nested", "nsParentId": 3})
    child = world.client.get("/namespace/getChildren?parentId=3", headers=world.headers).json["t"][0]["nsId"]
    for data in ({"nsId": 3, "nsParentId": 3}, {"nsId": 3, "nsParentId": child}, {"nsId": 3, "nsParentId": 4}, {"nsId": 2, "nsParentId": 3}):
        assert post(world, "/namespace/updateParent", json=data).json == {"t": -1, "msgCode": 400}
    assert post(world, "/namespace/delete?nsId=2").json["msgCode"] == 400
    assert post(world, "/namespace/updateName", json={"nsId": 2, "nsName": "renamed"}).json["msgCode"] == 400


@pytest.mark.parametrize("name", ["../outside", "a/b", "a\\b", "..", ".", "", "\x00"])
def test_unsafe_names(world, name):
    assert post(world, "/namespace/insert", json={"nsName": name, "nsParentId": 2}).json["msgCode"] == 400
    assert post(world, "/photo/update", json={"photoId": 1, "photoName": name}).json["msgCode"] == 400


def test_batch_verifies_all_namespaces_and_rolls_back(world):
    payload = MultiDict([("files", (io.BytesIO(world.jpeg), "one.jpg")), ("files", (io.BytesIO(world.jpeg), "two.jpg")),
                         ("photosStr", json.dumps([{"nsId": 3}, {"nsId": 4}]))])
    response = post(world, "/photo/inserts", data=payload)
    assert response.json == {"t": -2, "msgCode": 400}
    with world.app.app_context():
        assert db.session.scalar(select(func.count()).select_from(Photo)) == 2
    assert len(list(Path(world.app.config["UPLOAD_FOLDER"]).glob("user-alice/*"))) == 3


def test_upload_database_failure_removes_files(world, monkeypatch):
    def fail(*args, **kwargs):
        raise RuntimeError("injected metadata insertion failure")
    monkeypatch.setattr(world.app.extensions["photoly"].metadata, "extra", fail)
    with pytest.raises(RuntimeError, match="injected"):
        post(world, "/photo/insert", data={"file": (io.BytesIO(world.jpeg), "new.jpg"), "photo": '{"nsId":3}'})
    with world.app.app_context():
        assert db.session.scalar(select(func.count()).select_from(Photo)) == 2
    assert len(list(Path(world.app.config["UPLOAD_FOLDER"]).glob("user-alice/*"))) == 3
    assert list((Path(world.app.config["UPLOAD_FOLDER"]) / ".staging").iterdir()) == []


def png_bytes(size=(40, 30)):
    stream = io.BytesIO()
    Image.new("RGB", size, "navy").save(stream, "PNG")
    return stream.getvalue()


def test_replace_keeps_identity_and_swaps_content(world):
    root = Path(world.app.config["UPLOAD_FOLDER"]) / "user-alice"
    response = post(world, "/photo/replace", data={"photoId": 1, "file": (io.BytesIO(png_bytes()), "edited.png")})
    assert response.json == {"t": 1, "msgCode": 200}
    with world.app.app_context():
        photo = db.session.get(Photo, 1)
        assert (photo.format, photo.token, photo.photo_uuid, photo.photo_name, photo.ns_id) == ("png", "share-photo", "photo-one", "sunset", 3)
        assert db.session.scalar(select(func.count()).select_from(TagPhoto).where(TagPhoto.photo_id == 1)) == 1
        assert db.session.scalar(select(func.count()).select_from(GalleryPhoto).where(GalleryPhoto.photo_id == 1)) == 1
    assert not (root / "photo-one.jpg").exists() and not (root / "photo-one_thumbnail.jpg").exists()
    with Image.open(root / "photo-one.png") as image:
        assert image.size == (40, 30)
    assert (root / "photo-one_thumbnail.png").is_file()
    assert world.client.get("/photo/render/token-alice", query_string={"photoId": 1}).data == (root / "photo-one.png").read_bytes()


def test_replace_rejects_bad_input_and_other_users(world):
    root = Path(world.app.config["UPLOAD_FOLDER"]) / "user-alice"
    before = (root / "photo-one.jpg").read_bytes()
    for data in ({"photoId": 1, "file": (io.BytesIO(b"not an image"), "x.jpg")},
                 {"photoId": 1, "file": (io.BytesIO(png_bytes()), "x.exe")},
                 {"photoId": 2, "file": (io.BytesIO(png_bytes()), "x.png")},
                 {"photoId": 1}):
        assert post(world, "/photo/replace", data=data).json["msgCode"] == 400
    stranger = world.client.post("/photo/replace", headers=world.other, data={"photoId": 1, "file": (io.BytesIO(png_bytes()), "x.png")})
    assert stranger.json["msgCode"] == 400
    assert (root / "photo-one.jpg").read_bytes() == before
    with world.app.app_context():
        assert db.session.get(Photo, 1).format == "jpg"
    assert list((Path(world.app.config["UPLOAD_FOLDER"]) / ".staging").iterdir()) == []


def test_replace_database_failure_restores_files(world, monkeypatch):
    root = Path(world.app.config["UPLOAD_FOLDER"]) / "user-alice"
    before = (root / "photo-one.jpg").read_bytes()
    with world.app.app_context():
        session = db.session()
        def fail():
            raise RuntimeError("injected commit failure")
        monkeypatch.setattr(session, "commit", fail)
        upload = type("Upload", (), {"filename": "x.png", "stream": io.BytesIO(png_bytes())})()
        with pytest.raises(RuntimeError, match="injected"):
            PhotoService(session, world.app.extensions["photoly"]).replace(2, 1, upload)
    assert (root / "photo-one.jpg").read_bytes() == before
    assert (root / "photo-one_thumbnail.jpg").is_file() and not (root / "photo-one.png").exists()


def test_delete_database_failure_restores_files(world, monkeypatch):
    original = Path(world.app.config["UPLOAD_FOLDER"]) / "user-alice/photo-one.jpg"
    with world.app.app_context():
        session = db.session()
        def fail():
            raise RuntimeError("injected commit failure")
        monkeypatch.setattr(session, "commit", fail)
        with pytest.raises(RuntimeError, match="injected"):
            PhotoService(session, world.app.extensions["photoly"]).delete(2, 1)
        assert db.session.get(Photo, 1) is not None
    assert original.read_bytes() == world.jpeg


def test_missing_original_and_cascade_cleanup(world):
    root = Path(world.app.config["UPLOAD_FOLDER"])
    (root / "user-alice/photo-one.jpg").unlink()
    assert post(world, "/photo/delete?photoId=1").json == {"t": 1, "msgCode": 200}
    with world.app.app_context():
        assert db.session.get(Photo, 1) is None
        for model in (PhotoExtra, GalleryPhoto, TagPhoto):
            key = model.f_photo_id if model is PhotoExtra else model.photo_id
            assert db.session.scalar(select(func.count()).select_from(model).where(key == 1)) == 0
    assert not (root / "user-alice/photo-one_thumbnail.jpg").exists()


def test_account_deletion_revokes_sessions_credentials_and_files(world):
    result = world.client.get("/user/deleteUser", headers=world.headers).json
    assert result == {"t": 10, "msgCode": 200}
    assert world.client.get("/cred/queryRootNamespace/api-alice").data == b""
    assert world.client.get("/user/getInfo", headers=world.headers).json["msgCode"] == 403
    assert list((Path(world.app.config["UPLOAD_FOLDER"]) / "user-alice").iterdir()) == []
    with world.app.app_context():
        for model in (Namespace, Photo, Gallery, Tag, Cred):
            assert db.session.scalar(select(func.count()).select_from(model).where(model.user_id == 2)) == 0
        assert db.session.get(User, 3) is not None


@pytest.mark.parametrize("query", ["tagIds=1,2", "tagIds=1&tagIds=2", "tagIds=1,2&tagIds=1"])
def test_tag_intersection_and_parameter_encoding(world, query):
    assert world.client.get("/photo/getByTags?" + query, headers=world.headers).json["t"] == []
    assert post(world, "/photo/addTags?photoId=1&tagIds=2").json["t"] == 1
    records = world.client.get("/photo/getByTags?" + query, headers=world.headers).json["t"]
    assert [record["photoId"] for record in records] == [1]


@pytest.mark.parametrize("option,query", [("photoName", "SUN"), ("namespace", "Trav"), ("gallery", "Travel"), ("tag", "bea")])
def test_search_semantics(world, option, query):
    response = world.client.get("/photo/searchPhoto", headers=world.headers, query_string={"option": option, "query": query})
    assert [record["photoId"] for record in response.json["t"]] == [1]


def test_share_visibility_tokens_and_path_binding(world):
    url = "/photo/renderToken?path=user-alice/photo-one.jpg"
    assert world.client.get(url).status_code == 403
    assert world.client.get(url + "&token=share-photo").data == world.jpeg
    post(world, "/photo/update", json={"photoId": 1, "visibility": 0})
    assert world.client.get(url + "&token=share-photo").status_code == 403
    post(world, "/photo/update", json={"photoId": 1, "visibility": 2})
    assert world.client.get(url).data == world.jpeg
    for path in ("../photo-one.jpg", "user-bob/photo-one.jpg", "user-alice/photo-one.png", "user-alice/../photo-one.jpg"):
        assert world.client.get("/photo/renderToken", query_string={"path": path, "token": "share-photo"}).status_code == 403
    (Path(world.app.config["UPLOAD_FOLDER"]) / "user-alice/photo-one_thumbnail.jpg").unlink()
    assert world.client.get("/photo/renderThumbnail/token-alice?photoId=1").data == world.jpeg


@pytest.mark.parametrize("endpoint", ["/photo/renderV/token-alice?photoId=2", "/photo/renderVToken?path=user-alice/video-two.mp4&token=share-video"])
def test_video_range(world, endpoint):
    response = world.client.get(endpoint, headers={"Range": "bytes=4-11"})
    assert response.status_code == 206 and response.data == world.video[4:12]
    assert response.headers["Content-Range"] == f"bytes 4-11/{len(world.video)}"
    assert response.headers["Accept-Ranges"] == "bytes"
    assert world.client.get(endpoint, headers={"Range": "bytes=-5"}).data == world.video[-5:]
    invalid = world.client.get(endpoint, headers={"Range": "bytes=9999-"})
    assert invalid.status_code == 416
    assert invalid.headers["Content-Range"] == f"bytes */{len(world.video)}"


def test_api_permission_and_revocation(world):
    post(world, "/cred/update", json={"credId": 1, "authorization": "r"})
    response = world.client.post("/cred/uploadPhoto/api-alice", data={"file": (io.BytesIO(world.jpeg), "new.jpg"), "photo": '{"nsId":3}'})
    assert response.text == "Wrong Auth for current Token"
    assert world.client.post("/cred/deletePhoto/api-alice?uuid=photo-one").json == -2
    assert world.client.get("/cred/render/api-alice?uuid=photo-one").data == world.jpeg
    post(world, "/cred/update", json={"credId": 1, "authorization": "c"})
    assert world.client.get("/cred/queryRootNamespace/api-alice").data == b""
    post(world, "/cred/delete?credId=1")
    assert world.client.get("/cred/render/api-alice?uuid=photo-one").data == b""


def test_zip_duplicate_names_and_no_shared_temp_paths(world):
    post(world, "/photo/insert", data={"file": (io.BytesIO(world.jpeg), "sunset.jpg"), "photo": '{"nsId":3}'})
    with world.app.app_context():
        service = NamespaceService(db.session, world.app.extensions["photoly"])
        one = service.download(2, 3)
        two = service.download(2, 3)
        with ZipFile(one) as archive:
            assert len(set(archive.namelist())) == 2
            assert all(name.startswith("Travel/") for name in archive.namelist())
        with ZipFile(two) as archive:
            assert len(archive.namelist()) == 2
        one.close()
        two.close()
    assert not list(Path(world.app.config["UPLOAD_FOLDER"]).rglob("*.zip"))


def test_zip_file_deleted_during_export(world, monkeypatch):
    original_write = ZipFile.write
    def concurrent_delete(archive, filename, *args, **kwargs):
        Path(filename).unlink()
        return original_write(archive, filename, *args, **kwargs)
    monkeypatch.setattr(ZipFile, "write", concurrent_delete)
    response = world.client.get("/namespace/downloadNs/token-alice?nsId=3")
    assert response.status_code == 404
    assert not list(Path(world.app.config["UPLOAD_FOLDER"]).rglob("*.zip"))


def test_storage_rejects_symlink_escape(world, tmp_path):
    outside = tmp_path / "outside"
    outside.mkdir()
    storage = world.app.extensions["photoly"].storage
    (storage.root / "escaped-user").symlink_to(outside, target_is_directory=True)
    with pytest.raises(BusinessError):
        storage.path("escaped-user", "private.jpg")


def test_exif_gps_upload_map_and_camera_fields(world, tmp_path):
    image = tmp_path / "gps.jpg"
    image.write_bytes(world.jpeg)
    subprocess.run(["exiftool", "-overwrite_original", "-Make=Photoly", "-Model=TestCamera", "-LensMake=TestLens", "-LensModel=50mm",
                    "-GPSLatitude=33.9", "-GPSLatitudeRef=S", "-GPSLongitude=151.2", "-GPSLongitudeRef=E",
                    "-DateTimeOriginal=2024:03:04 12:30:00", "-FNumber=2.8", "-FocalLength=50", "-ISO=200", "-ExposureTime=1/125", str(image)],
                   check=True, capture_output=True)
    assert post(world, "/photo/insert", data={"file": (io.BytesIO(image.read_bytes()), "gps.jpg"), "photo": '{"nsId":3}'}).json["t"] == 1
    photo = world.client.get("/photo/getByNamespace?nsId=3", headers=world.headers).json["t"][-1]
    info = world.client.get("/photo/getPhotoExif", headers=world.headers, query_string={"photoId": photo["photoId"]}).json["t"]
    assert float(info["GPS_Latitude"]) == pytest.approx(-33.9)
    assert float(info["GPS_Longitude"]) == pytest.approx(151.2)
    assert info["Make"] == "Photoly" and info["ISO_Speed_Ratings"] == "200"
    with world.app.app_context():
        extra = db.session.scalar(select(PhotoExtra).where(PhotoExtra.f_photo_id == photo["photoId"]))
        assert extra.extra_photo_gps_lat == pytest.approx(-33.9)
        assert extra.extra_photo_camera == "Photoly TestCamera"
        assert extra.extra_photo_aperture == pytest.approx(2.8)
        assert extra.extra_photo_focal == 50 and extra.extra_photo_shutter_sec == 125


def test_empty_exif_invalid_images_and_upload_limit(world):
    response = post(world, "/photo/insert", data={"file": (io.BytesIO(world.jpeg), "no-exif.jpg"), "photo": '{"nsId":3}'})
    assert response.json == {"t": 1, "msgCode": 200}
    bad = post(world, "/photo/insert", data={"file": (io.BytesIO(b"invalid image"), "broken.jpg"), "photo": '{"nsId":3}'})
    assert bad.json["msgCode"] == 400
    world.app.config["MAX_CONTENT_LENGTH"] = 10
    assert post(world, "/photo/insert", data={"file": (io.BytesIO(world.jpeg), "huge.jpg"), "photo": '{"nsId":3}'}).status_code == 413


def test_bootstrap_idempotency_and_preserves_password(app):
    with app.app_context():
        initialize(app)
        admin = db.session.scalar(select(User))
        password = admin.password
        app.config["ADMIN_PASSWORD"] = "different-password"
        setting = db.session.scalar(select(Setting).where(Setting.setting_name == "SignUp"))
        setting.setting_value = "0"
        db.session.commit()
        initialize(app)
        assert setting.setting_value == "0"
        assert db.session.scalar(select(func.count()).select_from(User)) == 1
        assert db.session.scalar(select(func.count()).select_from(Namespace)) == 1
        assert db.session.scalar(select(func.count()).select_from(Gallery)) == 1
        assert db.session.get(User, admin.user_id).password == password


def test_application_factory_has_no_database_or_filesystem_side_effects(tmp_path):
    path = tmp_path / "unused"
    app = create_app({"SQLALCHEMY_DATABASE_URI": "postgresql+psycopg://nobody:invalid@127.0.0.1:1/nonexistent",
                      "REDIS_URL": "redis://127.0.0.1:1/0", "UPLOAD_FOLDER": str(path)})
    assert app is not None and not path.exists()


def test_cors_health_and_bad_parameter(world):
    response = world.client.options("/photo/insert", headers={"Origin": "http://localhost:5200", "Access-Control-Request-Headers": "HRD-Token"})
    assert response.status_code == 200
    assert response.headers["Access-Control-Allow-Origin"] == "*"
    assert "HRD-Token" in response.headers["Access-Control-Allow-Headers"]
    assert world.client.get("/healthz").json == {"status": "ok"}
    assert post(world, "/photo/delete?photoId=invalid").json["msgCode"] == 400
    assert post(world, "/photo/update", data="invalid", content_type="application/json").json["msgCode"] == 400
