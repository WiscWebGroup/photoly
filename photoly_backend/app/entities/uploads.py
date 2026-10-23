"""Private, durable import history. File paths are never supplied by clients."""
from app.extensions import db


class UploadBatch(db.Model):
    __tablename__ = 'upload_batch'
    id = db.Column(db.String(36), primary_key=True)
    user_id = db.Column(db.Integer, db.ForeignKey('user.user_id', ondelete='CASCADE'), nullable=False, index=True)
    ns_id = db.Column(db.Integer, db.ForeignKey('namespace.ns_id', ondelete='SET NULL'))
    name = db.Column(db.String(255), nullable=False)
    sealed = db.Column(db.Boolean, nullable=False, server_default=db.false())
    cancelled = db.Column(db.Boolean, nullable=False, server_default=db.false())
    created_at = db.Column(db.DateTime(timezone=True), nullable=False)
    updated_at = db.Column(db.DateTime(timezone=True), nullable=False, index=True)
    dismissed = db.Column(db.Boolean, nullable=False, server_default=db.false())
    failures_acknowledged_at = db.Column(db.DateTime(timezone=True))
    # Relative directory -> namespace ID, allocated only when a media item is imported.
    folders = db.Column(db.JSON, nullable=False, default=dict)


class UploadItem(db.Model):
    __tablename__ = 'upload_item'
    id = db.Column(db.String(36), primary_key=True)
    batch_id = db.Column(db.String(36), db.ForeignKey('upload_batch.id', ondelete='CASCADE'), nullable=False, index=True)
    path = db.Column(db.Text, nullable=False)
    size = db.Column(db.BigInteger, nullable=False)
    state = db.Column(db.String(20), nullable=False, default='pending', index=True)
    error = db.Column(db.String(255))
    content_sha256 = db.Column(db.String(64))
    photo_id = db.Column(db.Integer, db.ForeignKey('photo.photo_id', ondelete='SET NULL'))
    ns_id = db.Column(db.Integer, db.ForeignKey('namespace.ns_id', ondelete='SET NULL'))
    kind = db.Column(db.String(20), nullable=False, server_default='upload')
    options = db.Column(db.JSON)
    media_metadata = db.Column(db.JSON)
    pair_id = db.Column(db.String(36))
    # Archive children refer to the durable source, never duplicate its compressed bytes.
    source_id = db.Column(db.String(36), index=True)
    entry_index = db.Column(db.Integer)
    updated_at = db.Column(db.DateTime(timezone=True), nullable=False)
