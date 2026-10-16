"""PostgreSQL baseline: eleven existing business tables, no AI entities."""
from alembic import op
import sqlalchemy as sa

revision = "0001_postgresql"
down_revision = None
branch_labels = None
depends_on = None

def upgrade():
    # Extension installation belongs to the privileged database bootstrap.
    if op.get_bind().execute(sa.text("SELECT extversion FROM pg_extension WHERE extname = 'vector'")).scalar() != "0.8.7":
        raise RuntimeError("pgvector 0.8.7 must be installed by the database bootstrap")
    op.create_table('user',
        sa.Column('user_id', sa.Integer, sa.Identity(), primary_key=True),
        sa.Column('user_name', sa.String(255)),
        sa.Column('email', sa.String(255)),
        sa.Column('create_date', sa.DateTime),
        sa.Column('role', sa.String(255)),
        sa.Column('password', sa.String(255)),
        sa.Column('uuid', sa.String(255)),
        sa.UniqueConstraint('email', name="uq_user_email"),
        sa.UniqueConstraint('uuid', name="uq_user_uuid")
    )

    op.create_table('setting',
        sa.Column('setting_id', sa.Integer, sa.Identity(), primary_key=True),
        sa.Column('setting_name', sa.String(255)),
        sa.Column('setting_value', sa.String(255)),
        sa.Column('note', sa.String(255)),
        sa.UniqueConstraint('setting_name', name="uq_setting_setting_name")
    )

    op.create_table('namespace',
        sa.Column('ns_id', sa.Integer, sa.Identity(), primary_key=True),
        sa.Column('ns_name', sa.String(255)),
        sa.Column('ns_parent_id', sa.Integer),
        sa.Column('user_id', sa.Integer, sa.ForeignKey('user.user_id', ondelete='CASCADE'))
    )
    op.create_index("ix_namespace_ns_parent_id", 'namespace', ['ns_parent_id'])
    op.create_index("ix_namespace_user_id", 'namespace', ['user_id'])
    op.create_table('gallery',
        sa.Column('ga_id', sa.Integer, sa.Identity(), primary_key=True),
        sa.Column('ga_name', sa.String(255)),
        sa.Column('user_id', sa.Integer, sa.ForeignKey('user.user_id', ondelete='CASCADE')),
        sa.Column('create_date', sa.DateTime),
        sa.Column('cover_id', sa.Integer),
        sa.Column('cover_color', sa.String(255))
    )
    op.create_index("ix_gallery_user_id", 'gallery', ['user_id'])
    op.create_table('tag',
        sa.Column('tag_id', sa.Integer, sa.Identity(), primary_key=True),
        sa.Column('user_id', sa.Integer, sa.ForeignKey('user.user_id', ondelete='CASCADE'), nullable=False),
        sa.Column('tag_name', sa.String(255), nullable=False)
    )
    op.create_index("ix_tag_user_id", 'tag', ['user_id'])
    op.create_table('photo',
        sa.Column('photo_id', sa.Integer, sa.Identity(), primary_key=True),
        sa.Column('photo_name', sa.String(255)),
        sa.Column('upload_date', sa.DateTime),
        sa.Column('format', sa.String(255)),
        sa.Column('ns_id', sa.Integer, sa.ForeignKey('namespace.ns_id')),
        sa.Column('visibility', sa.Integer),
        sa.Column('token', sa.String(255)),
        sa.Column('photo_uuid', sa.String(255)),
        sa.Column('user_id', sa.Integer, sa.ForeignKey('user.user_id', ondelete='CASCADE')),
        sa.UniqueConstraint('token', name="uq_photo_token"),
        sa.UniqueConstraint('photo_uuid', name="uq_photo_photo_uuid")
    )
    op.create_index("ix_photo_ns_id", 'photo', ['ns_id'])
    op.create_index("ix_photo_user_id", 'photo', ['user_id'])
    op.create_table('cred',
        sa.Column('cred_id', sa.Integer, sa.Identity(), primary_key=True),
        sa.Column('user_id', sa.Integer, sa.ForeignKey('user.user_id', ondelete='CASCADE')),
        sa.Column('token', sa.String(255)),
        sa.Column('authorization', sa.String(255)),
        sa.UniqueConstraint('token', name="uq_cred_token")
    )
    op.create_index("ix_cred_user_id", 'cred', ['user_id'])
    op.create_table('gallery_photo',
        sa.Column('gp_id', sa.Integer, sa.Identity(), primary_key=True),
        sa.Column('ga_id', sa.Integer, sa.ForeignKey('gallery.ga_id', ondelete='CASCADE')),
        sa.Column('photo_id', sa.Integer, sa.ForeignKey('photo.photo_id', ondelete='CASCADE')),
        sa.UniqueConstraint('ga_id', 'photo_id', name="uq_gallery_photo_ga_id_photo_id")
    )
    op.create_index("ix_gallery_photo_ga_id", 'gallery_photo', ['ga_id'])
    op.create_index("ix_gallery_photo_photo_id", 'gallery_photo', ['photo_id'])
    op.create_table('tag_photo',
        sa.Column('tp_id', sa.Integer, sa.Identity(), primary_key=True),
        sa.Column('user_id', sa.Integer, sa.ForeignKey('user.user_id', ondelete='CASCADE')),
        sa.Column('tag_id', sa.Integer, sa.ForeignKey('tag.tag_id', ondelete='CASCADE')),
        sa.Column('photo_id', sa.Integer, sa.ForeignKey('photo.photo_id', ondelete='CASCADE')),
        sa.UniqueConstraint('tag_id', 'photo_id', name="uq_tag_photo_tag_id_photo_id")
    )
    op.create_index("ix_tag_photo_user_id", 'tag_photo', ['user_id'])
    op.create_index("ix_tag_photo_tag_id", 'tag_photo', ['tag_id'])
    op.create_index("ix_tag_photo_photo_id", 'tag_photo', ['photo_id'])
    op.create_table('namespace_photo',
        sa.Column('np_id', sa.Integer, sa.Identity(), primary_key=True),
        sa.Column('ns_id', sa.Integer, sa.ForeignKey('namespace.ns_id', ondelete='CASCADE')),
        sa.Column('photo_id', sa.Integer, sa.ForeignKey('photo.photo_id', ondelete='CASCADE'))
    )
    op.create_index("ix_namespace_photo_ns_id", 'namespace_photo', ['ns_id'])
    op.create_index("ix_namespace_photo_photo_id", 'namespace_photo', ['photo_id'])
    op.create_table('photo_extra',
        sa.Column('extra_photo_id', sa.Integer, sa.Identity(), primary_key=True),
        sa.Column('f_photo_id', sa.Integer, sa.ForeignKey('photo.photo_id', ondelete='CASCADE'), nullable=False),
        sa.Column('extra_photo_gps_lat', sa.Double),
        sa.Column('extra_photo_gps_lon', sa.Double),
        sa.Column('extra_photo_camera', sa.String(255)),
        sa.Column('extra_photo_lens', sa.String(255)),
        sa.Column('extra_photo_aperture', sa.Double),
        sa.Column('extra_photo_focal', sa.Integer),
        sa.Column('extra_photo_iso', sa.Integer),
        sa.Column('extra_photo_shutter_sec', sa.Integer),
        sa.Column('extra_photo_taken_date', sa.DateTime),
        sa.Column('extra_photo_note', sa.String(255)),
        sa.UniqueConstraint('f_photo_id', name="uq_photo_extra_f_photo_id")
    )

    op.create_index("uq_user_email_lower", "user", [sa.text("lower(email)")], unique=True)

def downgrade():
    op.drop_table('photo_extra')
    op.drop_table('namespace_photo')
    op.drop_table('tag_photo')
    op.drop_table('gallery_photo')
    op.drop_table('cred')
    op.drop_table('photo')
    op.drop_table('tag')
    op.drop_table('gallery')
    op.drop_table('namespace')
    op.drop_table('setting')
    op.drop_table('user')
