import os
from sqlalchemy.engine import URL


def database_url(migration=False):
    override = os.getenv("MIGRATION_DATABASE_URL" if migration else "DATABASE_URL")
    if override:
        return override
    return URL.create(
        "postgresql+psycopg", username="photoly_migrator" if migration else "photoly",
        password=os.getenv("POSTGRES_MIGRATION_PASSWORD" if migration else "POSTGRES_APP_PASSWORD", "photoly"),
        host=os.getenv("POSTGRES_HOST", "localhost"), port=int(os.getenv("POSTGRES_PORT", "5432")),
        database=os.getenv("POSTGRES_DB", "photoly"),
    )


def environment_config():
    return {
        "SQLALCHEMY_DATABASE_URI": database_url(),
        "SQLALCHEMY_TRACK_MODIFICATIONS": False,
        "SQLALCHEMY_ENGINE_OPTIONS": {"pool_pre_ping": True},
        "REDIS_URL": os.getenv("REDIS_URL", "redis://localhost:6390/14"),
        "UPLOAD_FOLDER": os.getenv("UPLOAD_FOLDER", "/photoly"),
        "PUBLIC_BASE_URL": os.getenv("PUBLIC_BASE_URL", "http://localhost:8384").rstrip("/"),
        "TIMEZONE": os.getenv("TIMEZONE", "America/New_York"),
        "MAX_CONTENT_LENGTH": int(os.getenv("MAX_CONTENT_LENGTH", str(600 * 1024 * 1024))),
        "MAX_FORM_MEMORY_SIZE": 2 * 1024 * 1024,
        "EXIFTOOL_PATH": os.getenv("EXIFTOOL_PATH", "exiftool"),
        "ADMIN_EMAIL": os.getenv("ADMIN_EMAIL", "admin@a.com"),
        "ADMIN_PASSWORD": os.getenv("ADMIN_PASSWORD"),
        "CORS_ORIGINS": os.getenv("CORS_ORIGINS", "*").split(","),
        "GUNICORN_PID_FILE": os.getenv("GUNICORN_PID_FILE"),
    }
