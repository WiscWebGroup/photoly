import os


def environment_config():
    return {
        "SQLALCHEMY_DATABASE_URI": os.getenv("DATABASE_URL", "mysql+pymysql://photoly:photoly@localhost:3308/photoly?charset=utf8mb4"),
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
