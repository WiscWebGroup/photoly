"""Persist editable OCR text; invalidate it whenever original contents change."""
from alembic import op
import sqlalchemy as sa

revision = '0004_photo_ocr'
down_revision = '0003_video_vectors'
branch_labels = depends_on = None


def upgrade():
    op.create_table('photo_ocr',
        sa.Column('photo_id', sa.Integer(), sa.ForeignKey('photo.photo_id', ondelete='CASCADE'), primary_key=True, autoincrement=False),
        sa.Column('content_version', sa.Integer(), nullable=False),
        sa.Column('text', sa.Text(), nullable=False),
        sa.Column('languages', sa.String(100), nullable=False),
        sa.Column('engine_version', sa.String(255)),
        sa.Column('revision', sa.Integer(), nullable=False),
        sa.Column('updated_at', sa.DateTime(timezone=True), nullable=False),
        sa.CheckConstraint('content_version > 0 AND revision > 0', name='ck_ocr_versions'),
        sa.CheckConstraint('length(text) <= 100000', name='ck_ocr_text_length'))
    op.execute('''
CREATE FUNCTION photoly_invalidate_ocr() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  DELETE FROM photo_ocr WHERE photo_id=NEW.photo_id;
  RETURN NEW;
END $$;
CREATE TRIGGER invalidate_photo_ocr AFTER UPDATE OF content_version ON photo
FOR EACH ROW WHEN (OLD.content_version IS DISTINCT FROM NEW.content_version)
EXECUTE FUNCTION photoly_invalidate_ocr();
''')


def downgrade():
    op.execute('DROP TRIGGER invalidate_photo_ocr ON photo; DROP FUNCTION photoly_invalidate_ocr()')
    op.drop_table('photo_ocr')
