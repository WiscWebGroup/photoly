from sqlalchemy import select
from app.entities import Gallery, Tag, Photo, GalleryPhoto, User
from app.services.base import BaseService, BusinessError, valid_name


class CatalogService(BaseService):
    def query(self, model, user_id):
        return select(model).where(model.user_id == user_id).order_by(model.__mapper__.primary_key[0])

    def all(self, model, user_id):
        return self.session.scalars(self.query(model, user_id)).all()

    def paged(self, model, user_id, page, rows):
        return self.page(self.query(model, user_id), page, rows)

    def insert(self, model, user_id, data):
        with self.atomic(user_id):
            if model is Gallery:
                obj = Gallery(user_id=user_id, ga_name=valid_name(data.get("gaName")), create_date=self.now(),
                              cover_id=data.get("coverId"), cover_color=data.get("coverColor"))
            else:
                obj = Tag(user_id=user_id, tag_name=valid_name(data.get("tagName")))
            self.session.add(obj)
        return True if model is Gallery else 1

    def update(self, model, user_id, data):
        key = "gaId" if model is Gallery else "tagId"
        with self.atomic(user_id):
            obj = self.owned(model, data.get(key), user_id)
            if model is Gallery:
                obj.ga_name = valid_name(data.get("gaName"))
                obj.cover_color = data.get("coverColor")
                obj.cover_id = data.get("coverId")
            else:
                obj.tag_name = valid_name(data.get("tagName"))
        return 1

    def delete(self, model, user_id, key):
        with self.atomic(user_id):
            self.session.delete(self.owned(model, key, user_id))
        return 1

    def download_gallery(self, user_id, ga_id):
        gallery = self.owned(Gallery, ga_id, user_id)
        user = self.session.get(User, user_id)
        photos = self.session.scalars(select(Photo).join(GalleryPhoto, Photo.photo_id == GalleryPhoto.photo_id)
                                      .where(GalleryPhoto.ga_id == ga_id, Photo.user_id == user_id)).all()
        name = valid_name(gallery.ga_name)
        return self.storage.archive([(self.storage.media(user.uuid, p), name + "/" + valid_name(p.photo_name) + "." + p.format, p.photo_id) for p in photos])
