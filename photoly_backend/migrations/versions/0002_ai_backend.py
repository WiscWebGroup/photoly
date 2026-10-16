"""AI profiles, jobs and state; frozen schema independent of application models."""
from alembic import op
import sqlalchemy as sa
from sqlalchemy import text
from sqlalchemy.dialects.postgresql import JSONB
from pgvector.sqlalchemy import HALFVEC
revision = "0002_ai_backend"
down_revision = "0001_postgresql"
branch_labels = depends_on = None
metadata = sa.MetaData(naming_convention={"ix": "ix_%(table_name)s_%(column_0_name)s", "uq": "uq_%(table_name)s_%(column_0_name)s", "fk": "fk_%(table_name)s_%(column_0_name)s_%(referred_table_name)s", "pk": "pk_%(table_name)s"})
# Foreign-key targets are defined for SQLAlchemy resolution only, never created here.
sa.Table("photo", metadata, sa.Column("photo_id", sa.Integer, primary_key=True))
embedding_profile = sa.Table('embedding_profile', metadata,
    sa.Column('profile_id', sa.Integer, sa.Identity(), primary_key=True),
    sa.Column('provider', sa.String(32), nullable=False),
    sa.Column('dimension', sa.Integer, nullable=False),
    sa.Column('fingerprint', sa.String(64), nullable=False, unique=True),
    sa.Column('spec', JSONB, nullable=False),
    sa.Column('index_status', sa.String(16), nullable=False, server_default='pending'),
    sa.Column('created_at', sa.DateTime(timezone=True), nullable=False, server_default=sa.func.now()),
    sa.CheckConstraint('dimension IN (768, 2048)', name='ck_profile_dimension')
)

photo_embedding_state = sa.Table('photo_embedding_state', metadata,
    sa.Column('state_id', sa.BigInteger, sa.Identity(), primary_key=True),
    sa.Column('photo_id', sa.Integer, sa.ForeignKey('photo.photo_id', ondelete='CASCADE'), nullable=False, index=True),
    sa.Column('profile_id', sa.Integer, sa.ForeignKey('embedding_profile.profile_id', ondelete='CASCADE'), nullable=False),
    sa.Column('content_version', sa.Integer, nullable=False),
    sa.Column('content_sha256', sa.String(64)),
    sa.Column('status', sa.String(16), nullable=False),
    sa.Column('embedding', HALFVEC()),
    sa.Column('computed_at', sa.DateTime(timezone=True)),
    sa.Column('duration_ms', sa.Integer),
    sa.Column('error_code', sa.String(80)),
    sa.UniqueConstraint('profile_id', 'photo_id', name='uq_embedding_profile_photo'),
    sa.CheckConstraint("status IN ('pending','ready','failed','unsupported')", name='ck_embedding_status'),
    sa.CheckConstraint('content_version > 0', name='ck_embedding_version')
)

ai_job = sa.Table('ai_job', metadata,
    sa.Column('job_id', sa.BigInteger, sa.Identity(), primary_key=True),
    sa.Column('kind', sa.String(32), nullable=False),
    sa.Column('photo_id', sa.Integer, sa.ForeignKey('photo.photo_id', ondelete='CASCADE')),
    sa.Column('profile_id', sa.Integer, sa.ForeignKey('embedding_profile.profile_id', ondelete='SET NULL')),
    sa.Column('content_version', sa.Integer),
    sa.Column('payload', JSONB, nullable=False, server_default=text("'{}'::jsonb")),
    sa.Column('result', JSONB, nullable=False, server_default=text("'{}'::jsonb")),
    sa.Column('status', sa.String(16), nullable=False, server_default='queued'),
    sa.Column('priority', sa.Integer, nullable=False, server_default='0'),
    sa.Column('attempts', sa.Integer, nullable=False, server_default='0'),
    sa.Column('next_run_at', sa.DateTime(timezone=True), nullable=False, server_default=sa.func.now()),
    sa.Column('lease_owner', sa.String(80)),
    sa.Column('lease_token', sa.String(36)),
    sa.Column('lease_expires_at', sa.DateTime(timezone=True)),
    sa.Column('heartbeat_at', sa.DateTime(timezone=True)),
    sa.Column('dedup_key', sa.String(160), nullable=False),
    sa.Column('reserved_bytes', sa.BigInteger, nullable=False, server_default='0'),
    sa.Column('created_at', sa.DateTime(timezone=True), nullable=False, server_default=sa.func.now()),
    sa.Column('finished_at', sa.DateTime(timezone=True)),
    sa.CheckConstraint("status IN ('queued','running','succeeded','failed','cancelled')", name='ck_ai_job_status')
)
sa.Index('uq_ai_job_active', ai_job.c.dedup_key, unique=True, postgresql_where=text("status IN ('queued','running')"))
sa.Index('ix_ai_job_claim', ai_job.c.kind, ai_job.c.status, ai_job.c.next_run_at, ai_job.c.priority)
sa.Index('ix_ai_job_expiry', ai_job.c.lease_expires_at, postgresql_where=text("status = 'running'"))
sa.Index('uq_ai_operation_key', ai_job.c.dedup_key, unique=True, postgresql_where=text("kind = 'operation'"))
ai_control_state = sa.Table('ai_control_state', metadata,
    sa.Column('name', sa.String(32), primary_key=True),
    sa.Column('generation', sa.BigInteger, nullable=False, server_default='0'),
    sa.Column('desired', JSONB, nullable=False, server_default=text("'{}'::jsonb")),
    sa.Column('observed', JSONB, nullable=False, server_default=text("'{}'::jsonb")),
    sa.Column('lease_owner', sa.String(80)),
    sa.Column('lease_token', sa.String(36)),
    sa.Column('lease_expires_at', sa.DateTime(timezone=True)),
    sa.Column('heartbeat_at', sa.DateTime(timezone=True))
)


def upgrade():
    op.add_column("user", sa.Column("ai_search_enabled", sa.Boolean, nullable=False, server_default=sa.false()))
    op.add_column("photo", sa.Column("content_version", sa.Integer, nullable=False, server_default="1"))
    op.add_column("photo", sa.Column("content_sha256", sa.String(64)))
    op.create_check_constraint("ck_photo_content_version", "photo", "content_version > 0")
    embedding_profile.create(op.get_bind())
    photo_embedding_state.create(op.get_bind())
    ai_job.create(op.get_bind())
    ai_control_state.create(op.get_bind())
    op.execute("""
CREATE FUNCTION photoly_validate_embedding() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE dim integer;
BEGIN
  SELECT dimension INTO dim FROM embedding_profile WHERE profile_id=NEW.profile_id;
  IF NEW.status='ready' THEN
    IF NEW.embedding IS NULL OR vector_dims(NEW.embedding) <> dim OR l2_norm(NEW.embedding) <= 0 THEN
      RAISE EXCEPTION 'invalid embedding' USING ERRCODE='23514';
    END IF;
  ELSIF NEW.embedding IS NOT NULL THEN
    RAISE EXCEPTION 'nonready embedding' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER validate_embedding BEFORE INSERT OR UPDATE ON photo_embedding_state
FOR EACH ROW EXECUTE FUNCTION photoly_validate_embedding();
CREATE FUNCTION photoly_immutable_profile() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF (NEW.provider,NEW.dimension,NEW.fingerprint,NEW.spec) IS DISTINCT FROM (OLD.provider,OLD.dimension,OLD.fingerprint,OLD.spec) THEN
   RAISE EXCEPTION 'immutable profile' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER immutable_profile BEFORE UPDATE ON embedding_profile FOR EACH ROW EXECUTE FUNCTION photoly_immutable_profile();
""")


def downgrade():
    op.execute("DROP TRIGGER validate_embedding ON photo_embedding_state; DROP FUNCTION photoly_validate_embedding(); DROP TRIGGER immutable_profile ON embedding_profile; DROP FUNCTION photoly_immutable_profile();")
    op.drop_table('ai_control_state')
    op.drop_table('ai_job')
    op.drop_table('photo_embedding_state')
    op.drop_table('embedding_profile')
    op.drop_constraint("ck_photo_content_version", "photo", type_="check")
    op.drop_column("photo", "content_sha256")
    op.drop_column("photo", "content_version")
    op.drop_column("user", "ai_search_enabled")
