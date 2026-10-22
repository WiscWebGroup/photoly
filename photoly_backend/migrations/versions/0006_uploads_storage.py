"""Durable imports, groups and per-account storage limits."""
from alembic import op
import sqlalchemy as sa

revision = '0006_uploads_storage'
down_revision = '0005_smart_organization'
branch_labels = depends_on = None


def upgrade():
    op.create_table('user_group',
        sa.Column('id', sa.Integer, sa.Identity(), primary_key=True),
        sa.Column('name', sa.String(255), nullable=False, unique=True),
        sa.Column('max_storage_bytes', sa.BigInteger),
        sa.Column('is_default', sa.Boolean, nullable=False, server_default=sa.false()),
        sa.CheckConstraint('max_storage_bytes IS NULL OR max_storage_bytes >= 0', name='ck_group_storage'))
    op.create_index('uq_default_group', 'user_group', ['is_default'], unique=True, postgresql_where=sa.text('is_default'))
    op.execute("INSERT INTO user_group (name,max_storage_bytes,is_default) VALUES ('Default',50000000000,true)")
    op.add_column('user', sa.Column('group_id', sa.Integer, sa.ForeignKey('user_group.id'), nullable=False, server_default='1'))
    op.add_column('user', sa.Column('storage_override', sa.Boolean, nullable=False, server_default=sa.false()))
    op.add_column('user', sa.Column('max_storage_bytes', sa.BigInteger))
    op.add_column('user', sa.Column('used_bytes', sa.BigInteger))
    op.create_check_constraint('ck_user_storage', 'user', '(max_storage_bytes IS NULL OR max_storage_bytes >= 0) AND (used_bytes IS NULL OR used_bytes >= 0)')
    op.create_index('ix_user_group_id', 'user', ['group_id'])
    op.create_table('upload_batch',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('user_id', sa.Integer, sa.ForeignKey('user.user_id', ondelete='CASCADE'), nullable=False),
        sa.Column('ns_id', sa.Integer, sa.ForeignKey('namespace.ns_id', ondelete='SET NULL')),
        sa.Column('name', sa.String(255), nullable=False),
        sa.Column('sealed', sa.Boolean, nullable=False, server_default=sa.false()),
        sa.Column('cancelled', sa.Boolean, nullable=False, server_default=sa.false()),
        sa.Column('created_at', sa.DateTime(timezone=True), nullable=False),
        sa.Column('updated_at', sa.DateTime(timezone=True), nullable=False),
        sa.Column('folders', sa.JSON, nullable=False))
    op.create_index('ix_upload_batch_user_id', 'upload_batch', ['user_id'])
    op.create_index('ix_upload_batch_updated_at', 'upload_batch', ['updated_at'])
    op.create_table('upload_item',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('batch_id', sa.String(36), sa.ForeignKey('upload_batch.id', ondelete='CASCADE'), nullable=False),
        sa.Column('path', sa.Text, nullable=False), sa.Column('size', sa.BigInteger, nullable=False),
        sa.Column('state', sa.String(20), nullable=False), sa.Column('error', sa.String(255)),
        sa.Column('content_sha256', sa.String(64)),
        sa.Column('photo_id', sa.Integer, sa.ForeignKey('photo.photo_id', ondelete='SET NULL')),
        sa.Column('ns_id', sa.Integer, sa.ForeignKey('namespace.ns_id', ondelete='SET NULL')),
        sa.Column('source_id', sa.String(36)), sa.Column('entry_index', sa.Integer),
        sa.Column('updated_at', sa.DateTime(timezone=True), nullable=False))
    for field in ('batch_id','state','source_id'):
        op.create_index('ix_upload_item_' + field, 'upload_item', [field])


def downgrade():
    op.drop_table('upload_item')
    op.drop_table('upload_batch')
    op.drop_constraint('ck_user_storage', 'user', type_='check')
    for name in ('group_id','storage_override','max_storage_bytes','used_bytes'):
        op.drop_column('user', name)
    op.drop_table('user_group')
