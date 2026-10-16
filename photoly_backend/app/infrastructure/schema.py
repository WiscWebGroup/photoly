from pathlib import Path
from alembic.config import Config
from alembic.script import ScriptDirectory
from sqlalchemy import text


def migration_config():
    return Config(str(Path(__file__).resolve().parents[2] / "alembic.ini"))


def require_schema(session):
    expected = ScriptDirectory.from_config(migration_config()).get_heads()
    actual = list(session.scalars(text("SELECT version_num FROM alembic_version")))
    if set(actual) != set(expected):
        raise RuntimeError("Database schema is not at the application revision; run the migration service")
    version = session.scalar(text("SELECT extversion FROM pg_extension WHERE extname='vector'"))
    if version != "0.8.7":
        raise RuntimeError("Expected pgvector 0.8.7; check the database image and bootstrap")
