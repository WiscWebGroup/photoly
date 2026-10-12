import io
import os
from datetime import datetime
from pathlib import Path
from types import SimpleNamespace
import pytest
import fakeredis
from PIL import Image
from redis import Redis
from sqlalchemy.engine import make_url
from werkzeug.security import generate_password_hash
from app import create_app
from app.extensions import db
from app.entities import User, Namespace, Gallery, Tag, Photo, PhotoExtra, GalleryPhoto, TagPhoto, Cred, Setting
from app.cli import DEFAULT_SETTINGS


class FakeLifecycle:
    def __init__(self):
        self.calls = 0

    def stop(self):
        self.calls += 1
        return 1


@pytest.fixture(scope="session")
def password_hash():
    return generate_password_hash("password123")


@pytest.fixture(scope="session")
def jpeg():
    stream = io.BytesIO()
    Image.new("RGB", (800, 600), "coral").save(stream, "JPEG")
    return stream.getvalue()


@pytest.fixture(scope="session")
def video_bytes():
    # Minimal ISO Base Media container, enough to exercise opaque media storage and Range.
    return b"\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom" + b"\x00\x00\x00\x10mdat" + bytes(range(8))


@pytest.fixture
def app(tmp_path):
    database = os.getenv("TEST_DATABASE_URL", "sqlite://")
    if os.getenv("TEST_DATABASE_URL"):
        assert (make_url(database).database or "").endswith("_test"), "Refusing to reset a non-test database"
    redis_url = os.getenv("TEST_REDIS_URL")
    if redis_url:
        assert redis_url.endswith("/15"), "Integration tests use Redis DB 15 only"
    redis = Redis.from_url(redis_url, decode_responses=True) if redis_url else fakeredis.FakeRedis(decode_responses=True)
    config = {"TESTING": True, "SQLALCHEMY_DATABASE_URI": database, "REDIS_CLIENT": redis,
              "UPLOAD_FOLDER": str(tmp_path / "uploads"), "ADMIN_EMAIL": "admin@test.local", "ADMIN_PASSWORD": "password123",
              "PUBLIC_BASE_URL": "http://localhost:8384", "LIFECYCLE": FakeLifecycle()}
    app = create_app(config)
    with app.app_context():
        db.drop_all()
        db.create_all()
    # Only touch session keys within the dedicated test database.
    for key in redis.scan_iter(match="photoly:session:*"):
        redis.delete(key)
    yield app
    with app.app_context():
        db.session.remove()
        db.drop_all()
        db.engine.dispose()
    redis.close()


@pytest.fixture
def client(app):
    return app.test_client()


@pytest.fixture
def world(app, client, password_hash, jpeg, video_bytes):
    stamp = datetime(2024, 3, 4, 12, 30, 0)
    with app.app_context():
        for identifier, name in ((1, "admin"), (2, "alice"), (3, "bob")):
            db.session.add(User(user_id=identifier, user_name=name, email=name + "@test.local", role="admin" if identifier == 1 else "user",
                                password=password_hash, uuid=f"user-{name}", create_date=stamp))
        db.session.flush()
        for name, (value, note) in DEFAULT_SETTINGS.items():
            db.session.add(Setting(setting_name=name, setting_value=value, note=note))
        db.session.add_all([Namespace(ns_id=1, user_id=1, ns_name="/", ns_parent_id=-1),
                            Namespace(ns_id=2, user_id=2, ns_name="/", ns_parent_id=-1),
                            Namespace(ns_id=3, user_id=2, ns_name="Travel", ns_parent_id=2),
                            Namespace(ns_id=4, user_id=3, ns_name="/", ns_parent_id=-1)])
        db.session.flush()
        db.session.add_all([Gallery(ga_id=1, user_id=2, ga_name="My Favorite", create_date=stamp, cover_id=2, cover_color="#FFFFFF"),
                            Gallery(ga_id=2, user_id=2, ga_name="Travel", create_date=stamp, cover_id=1, cover_color="#ABCDEF"),
                            Tag(tag_id=1, user_id=2, tag_name="beach"), Tag(tag_id=2, user_id=2, tag_name="red"),
                            Cred(cred_id=1, user_id=2, token="api-alice", authorization="CRD")])
        photo = Photo(photo_id=1, photo_name="sunset", upload_date=stamp, format="jpg", ns_id=3, visibility=1,
                      token="share-photo", photo_uuid="photo-one", user_id=2)
        video = Photo(photo_id=2, photo_name="movie", upload_date=stamp, format="mp4", ns_id=2, visibility=1,
                      token="share-video", photo_uuid="video-two", user_id=2)
        db.session.add_all([photo, video])
        db.session.flush()
        db.session.add_all([GalleryPhoto(ga_id=2, photo_id=1), TagPhoto(tag_id=1, photo_id=1, user_id=2),
                            PhotoExtra(f_photo_id=1, extra_photo_gps_lat=40.7128, extra_photo_gps_lon=-74.006,
                                       extra_photo_taken_date=stamp), PhotoExtra(f_photo_id=2)])
        db.session.commit()
        infra = app.extensions["photoly"]
        with infra.storage.mutation() as files:
            path = infra.storage.media("user-alice", photo)
            files.install(io.BytesIO(jpeg), path)
            infra.storage.thumbnail(path, infra.storage.media("user-alice", photo, True), files)
            files.install(io.BytesIO(video_bytes), infra.storage.media("user-alice", video))
        for name, identifier in (("admin", 1), ("alice", 2), ("bob", 3)):
            infra.sessions.client.setex("photoly:session:token-" + name, 86400, identifier)
    return SimpleNamespace(app=app, client=client, jpeg=jpeg, video=video_bytes,
        headers={"HRD-Token": "token-alice"}, admin={"HRD-Token": "token-admin"}, other={"HRD-Token": "token-bob"})
