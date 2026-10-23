"""Allow owners to clear upload history and acknowledge failure notifications."""
from alembic import op
import sqlalchemy as sa

revision = '0008_upload_activity'
down_revision = '0007_media_tools'
branch_labels = depends_on = None


def upgrade():
    op.add_column('upload_batch', sa.Column('dismissed', sa.Boolean, nullable=False, server_default=sa.false()))
    op.add_column('upload_batch', sa.Column('failures_acknowledged_at', sa.DateTime(timezone=True)))


def downgrade():
    op.drop_column('upload_batch', 'failures_acknowledged_at')
    op.drop_column('upload_batch', 'dismissed')
