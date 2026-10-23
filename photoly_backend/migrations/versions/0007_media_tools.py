"""Media derivatives, Live Photo companions and server-side import tasks."""
from alembic import op
import sqlalchemy as sa

revision = '0007_media_tools'
down_revision = '0006_uploads_storage'
branch_labels = depends_on = None


def upgrade():
    op.create_table('photo_media',
        sa.Column('photo_id', sa.Integer, sa.ForeignKey('photo.photo_id', ondelete='CASCADE'), primary_key=True),
        sa.Column('animated', sa.Boolean, nullable=False, server_default=sa.false()),
        sa.Column('has_preview', sa.Boolean, nullable=False, server_default=sa.false()),
        sa.Column('live', sa.Boolean, nullable=False, server_default=sa.false()),
        sa.Column('live_status', sa.String(20)),
        sa.Column('live_error', sa.String(255)))
    op.add_column('upload_item', sa.Column('kind', sa.String(20), nullable=False, server_default='upload'))
    op.add_column('upload_item', sa.Column('options', sa.JSON))
    op.add_column('upload_item', sa.Column('media_metadata', sa.JSON))
    op.add_column('upload_item', sa.Column('pair_id', sa.String(36)))
    op.execute("INSERT INTO setting (setting_name,setting_value,note) VALUES ('OnlineDownload','0','Allow public image URL imports') ON CONFLICT (setting_name) DO NOTHING")


def downgrade():
    op.execute("DELETE FROM setting WHERE setting_name='OnlineDownload'")
    for column in ('pair_id', 'media_metadata', 'options', 'kind'):
        op.drop_column('upload_item', column)
    op.drop_table('photo_media')
