from alembic import context
from sqlalchemy import create_engine, pool
from app.config import database_url
from app.extensions import db
from app import entities  # Register metadata without creating the Flask app.

config = context.config

def migrate(connection):
    context.configure(connection=connection, target_metadata=db.metadata, compare_type=True)
    with context.begin_transaction():
        context.run_migrations()
        if connection.exec_driver_sql("SELECT to_regclass('public.alembic_version')").scalar():
            connection.exec_driver_sql("REVOKE INSERT, UPDATE, DELETE ON alembic_version FROM photoly")

if context.is_offline_mode():
    raise RuntimeError("This migration requires an online PostgreSQL connection")
elif config.attributes.get("connection") is not None:
    migrate(config.attributes["connection"])
else:
    engine = create_engine(database_url(migration=True), poolclass=pool.NullPool)
    with engine.connect() as connection:
        # Serialize migration runners even if two deployments accidentally overlap.
        connection.exec_driver_sql("SELECT pg_advisory_lock(74831001)")
        connection.commit()
        try:
            migrate(connection)
        finally:
            connection.rollback()
            connection.exec_driver_sql("SELECT pg_advisory_unlock(74831001)")
            connection.commit()
    engine.dispose()
