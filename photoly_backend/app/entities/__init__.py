"""The eleven existing database tables. HTTP field naming lives in serialization.py."""
from sqlalchemy import func
from app.extensions import db


class User(db.Model):
    __tablename__ = "user"
    user_id = db.Column(db.Integer, db.Identity(), primary_key=True)
    user_name = db.Column(db.String(255))
    email = db.Column(db.String(255), unique=True)
    create_date = db.Column(db.DateTime)
    role = db.Column(db.String(255))
    password = db.Column(db.String(255))
    uuid = db.Column(db.String(255), unique=True)
    ai_search_enabled = db.Column(db.Boolean, nullable=False, server_default=db.false())
    __table_args__ = (db.Index("uq_user_email_lower", func.lower(email), unique=True),)


class Namespace(db.Model):
    __tablename__ = "namespace"
    ns_id = db.Column(db.Integer, db.Identity(), primary_key=True)
    ns_name = db.Column(db.String(255))
    ns_parent_id = db.Column(db.Integer, index=True)
    user_id = db.Column(db.Integer, db.ForeignKey("user.user_id", ondelete="CASCADE"), index=True)


class Photo(db.Model):
    __tablename__ = "photo"
    photo_id = db.Column(db.Integer, db.Identity(), primary_key=True)
    photo_name = db.Column(db.String(255))
    upload_date = db.Column(db.DateTime)
    format = db.Column(db.String(255))
    ns_id = db.Column(db.Integer, db.ForeignKey("namespace.ns_id"), index=True)
    visibility = db.Column(db.Integer)
    token = db.Column(db.String(255), unique=True)
    photo_uuid = db.Column(db.String(255), unique=True)
    user_id = db.Column(db.Integer, db.ForeignKey("user.user_id", ondelete="CASCADE"), index=True)

    content_version = db.Column(db.Integer, nullable=False, server_default="1")
    content_sha256 = db.Column(db.String(64))
    __table_args__ = (db.CheckConstraint("content_version > 0", name="ck_photo_content_version"),)


class Gallery(db.Model):
    __tablename__ = "gallery"
    ga_id = db.Column(db.Integer, db.Identity(), primary_key=True)
    ga_name = db.Column(db.String(255))
    user_id = db.Column(db.Integer, db.ForeignKey("user.user_id", ondelete="CASCADE"), index=True)
    create_date = db.Column(db.DateTime)
    cover_id = db.Column(db.Integer)
    cover_color = db.Column(db.String(255))


class Tag(db.Model):
    __tablename__ = "tag"
    tag_id = db.Column(db.Integer, db.Identity(), primary_key=True)
    user_id = db.Column(db.Integer, db.ForeignKey("user.user_id", ondelete="CASCADE"), nullable=False, index=True)
    tag_name = db.Column(db.String(255), nullable=False)


class GalleryPhoto(db.Model):
    __tablename__ = "gallery_photo"
    gp_id = db.Column(db.Integer, db.Identity(), primary_key=True)
    ga_id = db.Column(db.Integer, db.ForeignKey("gallery.ga_id", ondelete="CASCADE"), index=True)
    photo_id = db.Column(db.Integer, db.ForeignKey("photo.photo_id", ondelete="CASCADE"), index=True)
    __table_args__ = (db.UniqueConstraint("ga_id", "photo_id", name="uq_gallery_photo_ga_id_photo_id"),)


class TagPhoto(db.Model):
    __tablename__ = "tag_photo"
    tp_id = db.Column(db.Integer, db.Identity(), primary_key=True)
    user_id = db.Column(db.Integer, db.ForeignKey("user.user_id", ondelete="CASCADE"), index=True)
    tag_id = db.Column(db.Integer, db.ForeignKey("tag.tag_id", ondelete="CASCADE"), index=True)
    photo_id = db.Column(db.Integer, db.ForeignKey("photo.photo_id", ondelete="CASCADE"), index=True)
    __table_args__ = (db.UniqueConstraint("tag_id", "photo_id", name="uq_tag_photo_tag_id_photo_id"),)


class NamespacePhoto(db.Model):
    """Legacy table retained; current folder membership is Photo.ns_id."""
    __tablename__ = "namespace_photo"
    np_id = db.Column(db.Integer, db.Identity(), primary_key=True)
    ns_id = db.Column(db.Integer, db.ForeignKey("namespace.ns_id", ondelete="CASCADE"), index=True)
    photo_id = db.Column(db.Integer, db.ForeignKey("photo.photo_id", ondelete="CASCADE"), index=True)


class PhotoExtra(db.Model):
    __tablename__ = "photo_extra"
    extra_photo_id = db.Column(db.Integer, db.Identity(), primary_key=True)
    f_photo_id = db.Column(db.Integer, db.ForeignKey("photo.photo_id", ondelete="CASCADE"), nullable=False, unique=True)
    extra_photo_gps_lat = db.Column(db.Double)
    extra_photo_gps_lon = db.Column(db.Double)
    extra_photo_camera = db.Column(db.String(255))
    extra_photo_lens = db.Column(db.String(255))
    extra_photo_aperture = db.Column(db.Double)
    extra_photo_focal = db.Column(db.Integer)
    extra_photo_iso = db.Column(db.Integer)
    extra_photo_shutter_sec = db.Column(db.Integer)
    extra_photo_taken_date = db.Column(db.DateTime)
    extra_photo_note = db.Column(db.String(255))


class Cred(db.Model):
    __tablename__ = "cred"
    cred_id = db.Column(db.Integer, db.Identity(), primary_key=True)
    user_id = db.Column(db.Integer, db.ForeignKey("user.user_id", ondelete="CASCADE"), index=True)
    token = db.Column(db.String(255), unique=True)
    authorization = db.Column(db.String(255))


class Setting(db.Model):
    __tablename__ = "setting"
    setting_id = db.Column(db.Integer, db.Identity(), primary_key=True)
    setting_name = db.Column(db.String(255), unique=True)
    setting_value = db.Column(db.String(255))
    note = db.Column(db.String(255))


from .ai import EmbeddingProfile, PhotoEmbeddingState, VideoFrameEmbedding, AIJob, AIControlState


class PhotoOCR(db.Model):
    __tablename__ = 'photo_ocr'
    photo_id = db.Column(db.Integer, db.ForeignKey('photo.photo_id', ondelete='CASCADE'), primary_key=True, autoincrement=False)
    content_version = db.Column(db.Integer, nullable=False)
    text = db.Column(db.Text, nullable=False)
    languages = db.Column(db.String(100), nullable=False)
    engine_version = db.Column(db.String(255))
    revision = db.Column(db.Integer, nullable=False)
    updated_at = db.Column(db.DateTime(timezone=True), nullable=False)
    __table_args__ = (
        db.CheckConstraint('content_version > 0 AND revision > 0', name='ck_ocr_versions'),
        db.CheckConstraint('length(text) <= 100000', name='ck_ocr_text_length'),
    )

from .smart import SmartPreference, SmartMedia, SmartItem, SmartMember, SmartLineage
