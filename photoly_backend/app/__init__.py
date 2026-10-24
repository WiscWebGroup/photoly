import os
from dataclasses import dataclass
from importlib import import_module
from flask import Flask, jsonify, request
from redis import Redis
from redis.exceptions import RedisError
from sqlalchemy import text
from sqlalchemy.exc import SQLAlchemyError, IntegrityError
from werkzeug.exceptions import HTTPException
from app.config import environment_config
from app.extensions import db
from app.infrastructure.sessions import SessionStore, PostgresSessionStore
from app.infrastructure.storage import LocalStorage
from app.infrastructure.metadata import MetadataReader
from app.infrastructure.lifecycle import ServerLifecycle
from app.infrastructure.qr import QRCodeEncoder
from app.serialization import result


@dataclass
class Infrastructure:
    config: dict
    sessions: SessionStore
    storage: LocalStorage
    metadata: MetadataReader
    lifecycle: ServerLifecycle
    qr: QRCodeEncoder


def _engine(app):
    with app.app_context():
        return db.engine


def _sessions(app):
    # Redis stays in use for AI locks/cursors even when login sessions live in Postgres.
    redis = app.config.get("REDIS_CLIENT") or Redis.from_url(app.config["REDIS_URL"], decode_responses=True,
                                                             socket_timeout=5, socket_connect_timeout=5)
    return PostgresSessionStore(_engine(app), redis) if os.getenv("SESSION_BACKEND") == "postgres" else SessionStore(redis)


def create_app(config=None):
    app = Flask(__name__, static_folder=None)
    app.config.from_mapping(environment_config())
    if config:
        app.config.update(config)
    if str(app.config["SQLALCHEMY_DATABASE_URI"]).startswith("postgresql"):
        # Authentication reads occur before a mutation takes the user's row lock.
        # READ COMMITTED prevents an older repeatable-read snapshot surviving that wait.
        app.config["SQLALCHEMY_ENGINE_OPTIONS"] = {
            **app.config["SQLALCHEMY_ENGINE_OPTIONS"], "isolation_level": "READ COMMITTED"}
    app.url_map.strict_slashes = False
    db.init_app(app)
    app.extensions["photoly"] = Infrastructure(
        app.config,
        _sessions(app),
        app.config.get("STORAGE") or LocalStorage(app.config["UPLOAD_FOLDER"]),
        app.config.get("METADATA_READER") or MetadataReader(app.config["EXIFTOOL_PATH"]),
        app.config.get("LIFECYCLE") or ServerLifecycle(app.config["GUNICORN_PID_FILE"]),
        app.config.get("QR_ENCODER") or QRCodeEncoder(),
    )
    for name in ("user", "admin", "photo", "namespace", "gallery", "tag", "cred", "photo_extra", "ai", "uploads"):
        app.register_blueprint(import_module("app.controllers." + name).bp)
    from app.docs import bp as docs_bp
    app.register_blueprint(docs_bp)

    @app.after_request
    def cors(response):
        origins = app.config["CORS_ORIGINS"]
        origin = request.headers.get("Origin")
        if "*" in origins:
            response.headers["Access-Control-Allow-Origin"] = "*"
        elif origin in origins:
            response.headers["Access-Control-Allow-Origin"] = origin
            response.vary.add("Origin")
        response.headers["Access-Control-Allow-Headers"] = "Content-Type, HRD-Token, Range, Idempotency-Key"
        response.headers["Access-Control-Allow-Methods"] = "POST, GET, OPTIONS, DELETE, PUT, PATCH"
        response.headers["Access-Control-Expose-Headers"] = "Content-Disposition, Content-Range, Accept-Ranges"
        response.headers["Access-Control-Max-Age"] = "3600"
        return response

    @app.errorhandler(IntegrityError)
    def conflict(error):
        db.session.rollback()
        return jsonify(result(-1, 400))

    @app.errorhandler(SQLAlchemyError)
    @app.errorhandler(RedisError)
    def dependency_error(error):
        db.session.rollback()
        app.logger.exception("Backend dependency failure")
        return jsonify(result(None, 503)), 503

    @app.errorhandler(HTTPException)
    def http_error(error):
        # Preserve protocol headers (notably Content-Range on 416 responses).
        response = error.get_response()
        response.data = app.json.dumps(result(None, error.code))
        response.content_type = "application/json"
        return response

    @app.get("/healthz")
    def health():
        db.session.execute(text("SELECT 1"))
        app.extensions["photoly"].sessions.client.ping()
        return jsonify({"status": "ok"})

    from app.cli import register_cli
    register_cli(app)
    return app
