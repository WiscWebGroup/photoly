"""Smart organization provenance; public catalogs keep their existing schema."""
from sqlalchemy.dialects.postgresql import JSONB
from app.extensions import db


class SmartPreference(db.Model):
    __tablename__ = 'smart_preference'
    user_id = db.Column(db.Integer, db.ForeignKey('user.user_id', ondelete='CASCADE'), primary_key=True, autoincrement=False)
    enabled = db.Column(db.Boolean, nullable=False, server_default=db.false())
    revision = db.Column(db.BigInteger, nullable=False, server_default='1')
    completed_revision = db.Column(db.BigInteger, nullable=False, server_default='0')
    profile_id = db.Column(db.Integer)
    checkpoint = db.Column(JSONB, nullable=False, server_default='{}')
    reason = db.Column(db.String(100))
    updated_at = db.Column(db.DateTime(timezone=True))


class SmartMedia(db.Model):
    __tablename__ = 'smart_media'
    photo_id = db.Column(db.Integer, db.ForeignKey('photo.photo_id', ondelete='CASCADE'), primary_key=True, autoincrement=False)
    user_id = db.Column(db.Integer, db.ForeignKey('user.user_id', ondelete='CASCADE'), nullable=False)
    content_version = db.Column(db.Integer, nullable=False)
    captured_at = db.Column(db.DateTime(timezone=True))
    latitude = db.Column(db.Double)
    longitude = db.Column(db.Double)
    facts = db.Column(JSONB, nullable=False, server_default='{}')
    attempts = db.Column(db.Integer, nullable=False, server_default='0')
    error = db.Column(db.String(100))
    __table_args__ = (db.Index('ix_smart_media_user_time', 'user_id', 'captured_at', 'photo_id'),)


class SmartItem(db.Model):
    __tablename__ = 'smart_item'
    item_id = db.Column(db.Integer, db.Identity(), primary_key=True)
    user_id = db.Column(db.Integer, db.ForeignKey('user.user_id', ondelete='CASCADE'), nullable=False, index=True)
    stable_key = db.Column(db.String(100), nullable=False)
    source = db.Column(db.String(16), nullable=False)
    tag_id = db.Column(db.Integer, db.ForeignKey('tag.tag_id', ondelete='SET NULL'), unique=True)
    ga_id = db.Column(db.Integer, db.ForeignKey('gallery.ga_id', ondelete='SET NULL'), unique=True)
    name = db.Column(db.String(255), nullable=False)
    custom_name = db.Column(db.String(255))
    cover = db.Column(JSONB)
    suppressed = db.Column(db.Boolean, nullable=False, server_default=db.false())
    run = db.Column(db.String(36))
    basis = db.Column(JSONB, nullable=False, server_default='{}')
    __table_args__ = (db.UniqueConstraint('user_id', 'stable_key', name='uq_smart_identity'),
        db.CheckConstraint("source IN ('camera','lens','trip')", name='ck_smart_source'))


class SmartMember(db.Model):
    __tablename__ = 'smart_member'
    item_id = db.Column(db.Integer, db.ForeignKey('smart_item.item_id', ondelete='CASCADE'), primary_key=True)
    photo_id = db.Column(db.Integer, db.ForeignKey('photo.photo_id', ondelete='CASCADE'), primary_key=True)
    content_version = db.Column(db.Integer, nullable=False)
    automatic = db.Column(db.Boolean, nullable=False, server_default=db.false())
    manual = db.Column(db.SmallInteger, nullable=False, server_default='0')
    run = db.Column(db.String(36))
    anchor = db.Column(db.Boolean, nullable=False, server_default=db.false())
    __table_args__ = (db.CheckConstraint('manual IN (-1,0,1)', name='ck_smart_manual'),
        db.Index('ix_smart_member_photo', 'photo_id'), db.Index('ix_smart_member_run', 'run', 'item_id'))


class SmartLineage(db.Model):
    __tablename__ = 'smart_lineage'
    parent_id = db.Column(db.Integer, db.ForeignKey('smart_item.item_id', ondelete='CASCADE'), primary_key=True)
    child_id = db.Column(db.Integer, db.ForeignKey('smart_item.item_id', ondelete='CASCADE'), primary_key=True)
