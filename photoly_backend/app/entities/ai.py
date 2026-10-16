from pgvector.sqlalchemy import HALFVEC
from sqlalchemy.dialects.postgresql import JSONB
from sqlalchemy import text
from app.extensions import db


class EmbeddingProfile(db.Model):
    __tablename__ = "embedding_profile"
    profile_id = db.Column(db.Integer, db.Identity(), primary_key=True)
    provider = db.Column(db.String(32), nullable=False)
    dimension = db.Column(db.Integer, nullable=False)
    fingerprint = db.Column(db.String(64), nullable=False, unique=True)
    spec = db.Column(JSONB, nullable=False)
    index_status = db.Column(db.String(16), nullable=False, server_default="pending")
    created_at = db.Column(db.DateTime(timezone=True), nullable=False, server_default=db.func.now())
    __table_args__ = (db.CheckConstraint("dimension IN (768, 2048)", name="ck_profile_dimension"),)


class PhotoEmbeddingState(db.Model):
    __tablename__ = "photo_embedding_state"
    state_id = db.Column(db.BigInteger, db.Identity(), primary_key=True)
    photo_id = db.Column(db.Integer, db.ForeignKey("photo.photo_id", ondelete="CASCADE"), nullable=False, index=True)
    profile_id = db.Column(db.Integer, db.ForeignKey("embedding_profile.profile_id", ondelete="CASCADE"), nullable=False)
    content_version = db.Column(db.Integer, nullable=False)
    content_sha256 = db.Column(db.String(64))
    status = db.Column(db.String(16), nullable=False)
    embedding = db.Column(HALFVEC())
    computed_at = db.Column(db.DateTime(timezone=True))
    duration_ms = db.Column(db.Integer)
    error_code = db.Column(db.String(80))
    __table_args__ = (db.UniqueConstraint("profile_id", "photo_id", name="uq_embedding_profile_photo"),
        db.CheckConstraint("status IN ('pending','ready','failed','unsupported')", name="ck_embedding_status"),
        db.CheckConstraint("content_version > 0", name="ck_embedding_version"),)


class AIJob(db.Model):
    __tablename__ = "ai_job"
    job_id = db.Column(db.BigInteger, db.Identity(), primary_key=True)
    kind = db.Column(db.String(32), nullable=False)
    photo_id = db.Column(db.Integer, db.ForeignKey("photo.photo_id", ondelete="CASCADE"))
    profile_id = db.Column(db.Integer, db.ForeignKey("embedding_profile.profile_id", ondelete="SET NULL"))
    content_version = db.Column(db.Integer)
    payload = db.Column(JSONB, nullable=False, server_default=text("'{}'::jsonb"))
    result = db.Column(JSONB, nullable=False, server_default=text("'{}'::jsonb"))
    status = db.Column(db.String(16), nullable=False, server_default="queued")
    priority = db.Column(db.Integer, nullable=False, server_default="0")
    attempts = db.Column(db.Integer, nullable=False, server_default="0")
    next_run_at = db.Column(db.DateTime(timezone=True), nullable=False, server_default=db.func.now())
    lease_owner = db.Column(db.String(80))
    lease_token = db.Column(db.String(36))
    lease_expires_at = db.Column(db.DateTime(timezone=True))
    heartbeat_at = db.Column(db.DateTime(timezone=True))
    dedup_key = db.Column(db.String(160), nullable=False)
    reserved_bytes = db.Column(db.BigInteger, nullable=False, server_default="0")
    created_at = db.Column(db.DateTime(timezone=True), nullable=False, server_default=db.func.now())
    finished_at = db.Column(db.DateTime(timezone=True))
    __table_args__ = (db.CheckConstraint("status IN ('queued','running','succeeded','failed','cancelled')", name="ck_ai_job_status"),
        db.Index("uq_ai_job_active", "dedup_key", unique=True, postgresql_where=text("status IN ('queued','running')")),
        db.Index("ix_ai_job_claim", "kind", "status", "next_run_at", "priority"),
        db.Index("ix_ai_job_expiry", "lease_expires_at", postgresql_where=text("status = 'running'")),
        db.Index("uq_ai_operation_key", "dedup_key", unique=True, postgresql_where=text("kind = 'operation'")),)


class AIControlState(db.Model):
    __tablename__ = "ai_control_state"
    name = db.Column(db.String(32), primary_key=True)
    generation = db.Column(db.BigInteger, nullable=False, server_default="0")
    desired = db.Column(JSONB, nullable=False, server_default=text("'{}'::jsonb"))
    observed = db.Column(JSONB, nullable=False, server_default=text("'{}'::jsonb"))
    lease_owner = db.Column(db.String(80))
    lease_token = db.Column(db.String(36))
    lease_expires_at = db.Column(db.DateTime(timezone=True))
    heartbeat_at = db.Column(db.DateTime(timezone=True))
