from app.extensions import db


class PhotoMedia(db.Model):
    __tablename__ = 'photo_media'
    photo_id = db.Column(db.Integer, db.ForeignKey('photo.photo_id', ondelete='CASCADE'), primary_key=True)
    animated = db.Column(db.Boolean, nullable=False, server_default=db.false())
    has_preview = db.Column(db.Boolean, nullable=False, server_default=db.false())
    live = db.Column(db.Boolean, nullable=False, server_default=db.false())
    live_status = db.Column(db.String(20))
    live_error = db.Column(db.String(255))
