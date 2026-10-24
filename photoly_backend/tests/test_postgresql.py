"""PostgreSQL behavior exercised against real migrations and restricted app grants."""
from concurrent.futures import ThreadPoolExecutor
from threading import Barrier
import numpy as np
import pytest
from alembic import command
from pgvector.sqlalchemy import HALFVEC, Vector
from sqlalchemy import Column, Integer, MetaData, Table, func, inspect, select, text
from sqlalchemy.exc import DBAPIError, IntegrityError
from app.config import database_url
from app.entities import User
from app.extensions import db
from app.infrastructure.schema import migration_config, require_schema
from tests.openapi_assertions import assert_documented_response


def test_database_api_and_legacy_alias(world):
    for route in ("/admin/pingDatabase", "/admin/pingMySQL", "/admin/databaseInfo"):
        response = world.client.get(route, headers=world.admin)
        assert response.json["msgCode"] == 200
        assert_documented_response("GET", route, response)
        assert world.client.get(route, headers=world.headers).json["msgCode"] != 200
        assert world.client.get(route).json["msgCode"] == 403
    info = world.client.get("/admin/databaseInfo", headers=world.admin).json["t"]
    assert info["type"] == "postgresql" and info["version"].startswith("17.")
    assert info["pgvectorVersion"] == "0.8.7" and info["schemaRevision"] == "0008_upload_activity"
    assert set(info) == {"type", "version", "address", "pgvectorVersion", "schemaRevision"}
    address = world.client.get("/admin/getAddress", headers=world.admin).json["t"]
    assert address["mysql"].endswith(":5432")


def test_repeated_migration_preserves_data_and_schema(world, migration_engine):
    with migration_engine.begin() as connection:
        config = migration_config()
        config.attributes["connection"] = connection
        command.upgrade(config, "head")
        command.upgrade(config, "head")
    with world.app.app_context():
        require_schema(db.session)
        assert db.session.get(User, 2).email == "alice@test.local"
        inspector = inspect(db.engine)
        assert len(inspector.get_table_names()) == 27
        assert inspector.get_columns("user")[0]["identity"]["always"] is False
        assert any(i["name"] == "uq_user_email_lower" and i["unique"] for i in inspector.get_indexes("user"))
        assert db.session.scalar(text("SHOW transaction_isolation")) == "read committed"


@pytest.mark.parametrize("statement", [
    "CREATE TABLE forbidden_ddl (id integer)",
    "CREATE SCHEMA forbidden_schema",
    "CREATE TEMP TABLE forbidden_temp (id integer)",
    'ALTER TABLE "user" ADD COLUMN forbidden integer',
    "SET ROLE photoly_migrator",
    "UPDATE alembic_version SET version_num='forbidden'",
])
def test_application_role_cannot_perform_ddl(app, statement):
    with app.app_context():
        assert db.session.scalar(text("SELECT current_user")) == "photoly"
        with pytest.raises(DBAPIError) as error:
            db.session.execute(text(statement))
        assert error.value.orig.sqlstate == "42501"
        db.session.rollback()


def test_schema_check_rejects_unexpected_revision(app, migration_engine):
    with migration_engine.begin() as connection:
        connection.execute(text("UPDATE alembic_version SET version_num='unexpected'"))
    try:
        with app.app_context():
            with pytest.raises(RuntimeError, match="schema"):
                require_schema(db.session)
    finally:
        with migration_engine.begin() as connection:
            connection.execute(text("UPDATE alembic_version SET version_num='0008_upload_activity'"))


def test_case_insensitive_email_unique_under_concurrent_writes(app):
    barrier = Barrier(2)
    def insert(email):
        with app.app_context():
            db.session.add(User(email=email, user_name="Concurrent"))
            barrier.wait(timeout=10)
            try:
                db.session.commit()
                return "created"
            except IntegrityError:
                db.session.rollback()
                return "duplicate"
    with ThreadPoolExecutor(max_workers=2) as pool:
        results = list(pool.map(insert, ["Case@Test.local", "case@test.local"]))
    assert sorted(results) == ["created", "duplicate"]
    with app.app_context():
        assert db.session.scalar(select(func.count()).select_from(User)) == 1


def test_postgres_password_is_not_interpolated_into_url(monkeypatch):
    monkeypatch.delenv("DATABASE_URL", raising=False)
    monkeypatch.setenv("POSTGRES_APP_PASSWORD", "a@b:/?#%'")
    url = database_url()
    assert url.password == "a@b:/?#%'"
    assert url.drivername == "postgresql+psycopg"
    assert "a@b" not in str(url)


def test_halfvec_roundtrip_distance_hnsw_and_rollback(migration_engine):
    metadata = MetaData()
    vectors = Table("halfvec_acceptance", metadata,
                    Column("id", Integer, primary_key=True),
                    Column("embedding", HALFVEC(2048)), Column("reference", Vector(2048)))
    rng = np.random.default_rng(42)
    data = rng.normal(size=(160, 2048)).astype(np.float32)
    data /= np.linalg.norm(data, axis=1, keepdims=True)
    query = data[0]
    try:
        with migration_engine.begin() as connection:
            vectors.create(connection)
            connection.execute(vectors.insert(), [dict(id=i, embedding=v, reference=v) for i, v in enumerate(data)])
            distance = vectors.c.embedding.cosine_distance(query)
            reference = vectors.c.reference.cosine_distance(query)
            rows = connection.execute(select(vectors.c.id, distance, reference).order_by(vectors.c.id)).all()
            max_error = max(abs(a-b) for _, a, b in rows)
            assert max_error < 0.001
            print(f"halfvec2048 maximum cosine error vs float32: {max_error:.9f}")
            exact = list(connection.scalars(select(vectors.c.id).order_by(reference).limit(10)))
            first = connection.scalar(select(vectors.c.embedding).where(vectors.c.id == 0)).to_numpy()
            assert first.shape == (2048,) and np.allclose(first, query, atol=0.00005)
            # Zero vectors can be stored, but cosine HNSW does not index them.
            connection.execute(vectors.insert(), {"id": 999, "embedding": np.zeros(2048), "reference": np.zeros(2048)})
            connection.exec_driver_sql("CREATE INDEX halfvec_acceptance_hnsw ON halfvec_acceptance USING hnsw (embedding halfvec_cosine_ops)")
            connection.exec_driver_sql("ANALYZE halfvec_acceptance")
            connection.exec_driver_sql("SET LOCAL enable_seqscan = off")
            connection.exec_driver_sql("SET LOCAL hnsw.ef_search = 200")
            statement = select(vectors.c.id).order_by(distance).limit(10)
            ann = list(connection.scalars(statement))
            assert ann[0] == 0 and len(set(exact) & set(ann)) >= 9
            # Literal compilation is exclusively for this synthetic EXPLAIN fixture.
            plan = connection.exec_driver_sql("EXPLAIN " + str(statement.compile(dialect=connection.dialect, compile_kwargs={"literal_binds": True}))).all()
            assert "halfvec_acceptance_hnsw" in str(plan)
            indexed = list(connection.scalars(select(vectors.c.id).order_by(distance).limit(200)))
            assert 999 not in indexed
        with migration_engine.connect() as connection:
            with connection.begin():
                connection.execute(vectors.insert(), {"id": 500, "embedding": query, "reference": query})
                connection.rollback()
            assert connection.scalar(select(func.count()).select_from(vectors).where(vectors.c.id == 500)) == 0
        for invalid in ("'[1,2]'::halfvec(2048)", "'[NaN]'::halfvec(1)", "'[Infinity]'::halfvec(1)"):
            with migration_engine.connect() as connection:
                with pytest.raises(DBAPIError):
                    connection.exec_driver_sql("SELECT " + invalid)
                connection.rollback()
    finally:
        with migration_engine.begin() as connection:
            vectors.drop(connection, checkfirst=True)


def test_migrated_schema_matches_orm_including_constraints(app, migration_engine):
    from alembic.autogenerate import compare_metadata
    from alembic.migration import MigrationContext
    with migration_engine.connect() as connection:
        context = MigrationContext.configure(connection, opts={"compare_type": True,
            "include_object": lambda obj, name, kind, reflected, compare_to: not (kind == "index" and
                name.startswith(("ix_embedding_profile_", "ix_video_frame_profile_")) and name.endswith("_hnsw"))})
        assert compare_metadata(context, db.metadata) == []


def test_concurrent_registration_preserves_email_and_rolls_back_loser(app, monkeypatch):
    from app.cli import initialize
    from app.entities import Namespace, Gallery
    from app.services.user import UserService
    with app.app_context():
        initialize(app)
    barrier = Barrier(2)
    original = UserService.by_email
    def synchronized_lookup(self, email):
        result = original(self, email)
        if str(email).lower() == "case@test.local":
            barrier.wait(timeout=10)
        return result
    # Both requests pass both pre-insert email checks before either can commit.
    with monkeypatch.context() as patch:
        patch.setattr(UserService, "by_email", synchronized_lookup)
        def register(email):
            with app.test_client() as client:
                response = client.post("/user/signUp", json={"email": email, "password": "secret", "userName": "Case"})
                assert_documented_response("POST", "/user/signUp", response)
                return email, response.json["msgCode"]
        with ThreadPoolExecutor(max_workers=2) as pool:
            results = list(pool.map(register, ["Case@Test.local", "case@test.local"]))
    assert sorted(code for _, code in results) == [200, 400]
    winner = next(email for email, code in results if code == 200)
    with app.app_context():
        user = db.session.scalar(select(User).where(func.lower(User.email) == "case@test.local"))
        assert user.email == winner
        for model in (User, Namespace, Gallery):
            assert db.session.scalar(select(func.count()).select_from(model)) == 2
    with app.test_client() as client:
        assert client.post("/user/signIn", json={"email": "CASE@TEST.LOCAL", "password": "secret"}).json["msgCode"] == 200
