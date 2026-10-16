"""Video frame vectors; preserve existing image profiles and embeddings."""
from alembic import op
import sqlalchemy as sa
from pgvector.sqlalchemy import HALFVEC

revision = '0003_video_vectors'
down_revision = '0002_ai_backend'
branch_labels = depends_on = None


def upgrade():
    op.add_column('photo_embedding_state', sa.Column('sampling_version', sa.Integer()))
    op.add_column('photo_embedding_state', sa.Column('frame_count', sa.Integer(), nullable=False, server_default='0'))
    op.create_unique_constraint('uq_embedding_state_profile', 'photo_embedding_state', ['state_id', 'profile_id'])
    op.create_check_constraint('ck_embedding_frame_count', 'photo_embedding_state', 'frame_count BETWEEN 0 AND 5')
    op.create_table('video_frame_embedding',
        sa.Column('frame_id', sa.BigInteger(), sa.Identity(), primary_key=True),
        sa.Column('state_id', sa.BigInteger(), nullable=False),
        sa.Column('profile_id', sa.Integer(), nullable=False),
        sa.Column('frame_index', sa.Integer(), nullable=False),
        sa.Column('timestamp_ms', sa.BigInteger(), nullable=False),
        sa.Column('embedding', HALFVEC(), nullable=False),
        sa.ForeignKeyConstraint(['state_id', 'profile_id'], ['photo_embedding_state.state_id', 'photo_embedding_state.profile_id'], ondelete='CASCADE'),
        sa.UniqueConstraint('state_id', 'frame_index', name='uq_video_frame_position'),
        sa.CheckConstraint('frame_index BETWEEN 0 AND 4', name='ck_video_frame_index'),
        sa.CheckConstraint('timestamp_ms >= 0', name='ck_video_frame_timestamp'))
    op.create_index('ix_video_frame_embedding_profile_id', 'video_frame_embedding', ['profile_id'])
    op.execute("""
CREATE OR REPLACE FUNCTION photoly_validate_embedding() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE dim integer;
BEGIN
  SELECT dimension INTO dim FROM embedding_profile WHERE profile_id=NEW.profile_id;
  IF NEW.sampling_version IS NOT NULL THEN
    IF NEW.sampling_version <= 0 OR NEW.embedding IS NOT NULL OR
       (NEW.status='ready' AND NEW.frame_count NOT BETWEEN 1 AND 5) OR
       (NEW.status<>'ready' AND NEW.frame_count<>0) THEN
      RAISE EXCEPTION 'invalid video embedding state' USING ERRCODE='23514';
    END IF;
  ELSE
    IF NEW.frame_count<>0 THEN RAISE EXCEPTION 'image frame count' USING ERRCODE='23514'; END IF;
    IF NEW.status='ready' THEN
      IF NEW.embedding IS NULL OR vector_dims(NEW.embedding) <> dim OR l2_norm(NEW.embedding) <= 0 THEN
        RAISE EXCEPTION 'invalid embedding' USING ERRCODE='23514';
      END IF;
    ELSIF NEW.embedding IS NOT NULL THEN
      RAISE EXCEPTION 'nonready embedding' USING ERRCODE='23514';
    END IF;
  END IF;
  RETURN NEW;
END $$;
CREATE FUNCTION photoly_validate_video_frame() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE dim integer;
BEGIN
  IF TG_OP='UPDATE' AND (NEW.state_id<>OLD.state_id OR NEW.profile_id<>OLD.profile_id) THEN
    RAISE EXCEPTION 'frame ownership is immutable' USING ERRCODE='23514';
  END IF;
  SELECT dimension INTO dim FROM embedding_profile WHERE profile_id=NEW.profile_id;
  IF dim IS NULL OR vector_dims(NEW.embedding)<>dim OR l2_norm(NEW.embedding)<=0 THEN
    RAISE EXCEPTION 'invalid frame embedding' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER validate_video_frame BEFORE INSERT OR UPDATE ON video_frame_embedding
FOR EACH ROW EXECUTE FUNCTION photoly_validate_video_frame();
CREATE FUNCTION photoly_validate_frame_count() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE expected integer; actual integer; sid bigint;
BEGIN
  sid=COALESCE(NEW.state_id,OLD.state_id);
  SELECT frame_count INTO expected FROM photo_embedding_state WHERE state_id=sid;
  IF FOUND THEN
    SELECT count(*) INTO actual FROM video_frame_embedding WHERE state_id=sid;
    IF actual<>expected THEN RAISE EXCEPTION 'incomplete video frames' USING ERRCODE='23514'; END IF;
  END IF;
  RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER complete_video_state AFTER INSERT OR UPDATE ON photo_embedding_state
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION photoly_validate_frame_count();
CREATE CONSTRAINT TRIGGER complete_video_frames AFTER INSERT OR UPDATE OR DELETE ON video_frame_embedding
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION photoly_validate_frame_count();
INSERT INTO ai_job(kind,profile_id,dedup_key)
SELECT 'index',p.profile_id,'index:'||p.profile_id FROM embedding_profile p
WHERE NOT EXISTS (SELECT 1 FROM ai_job j WHERE j.kind='index' AND j.profile_id=p.profile_id AND j.status IN ('queued','running'));
""")


def downgrade():
    # A downgraded worker must never try to use a ready video with no image embedding.
    op.execute("""
DROP TRIGGER complete_video_state ON photo_embedding_state;
DROP TRIGGER complete_video_frames ON video_frame_embedding;
DROP FUNCTION photoly_validate_frame_count();
DROP TRIGGER validate_video_frame ON video_frame_embedding;
DROP FUNCTION photoly_validate_video_frame();
UPDATE photo_embedding_state SET status='unsupported',frame_count=0,error_code=NULL WHERE sampling_version IS NOT NULL;
CREATE OR REPLACE FUNCTION photoly_validate_embedding() RETURNS trigger LANGUAGE plpgsql AS $$
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
""")
    op.drop_table('video_frame_embedding')
    op.drop_constraint('uq_embedding_state_profile', 'photo_embedding_state', type_='unique')
    op.drop_constraint('ck_embedding_frame_count', 'photo_embedding_state', type_='check')
    op.drop_column('photo_embedding_state', 'frame_count')
    op.drop_column('photo_embedding_state', 'sampling_version')
