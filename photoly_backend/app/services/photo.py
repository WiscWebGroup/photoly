import re
from uuid import uuid4
from urllib.parse import urlencode
from sqlalchemy import select, func
from app.entities import Photo, PhotoExtra, Namespace, Gallery, GalleryPhoto, Tag, TagPhoto, User
from app.services.base import BaseService, BusinessError, valid_name

IMAGE_FORMATS = {"jpg", "jpeg", "gif", "png", "webp", "ico", "bmp", "tif", "tiff", "svg", "psd", "raw", "arw", "pcd", "jfif"}
EDITABLE_FORMATS = {"jpg", "jpeg", "png", "webp"}
VIDEO_FORMATS = {"mp4", "avi", "wmv", "mpeg", "m4v", "mov", "flv", "asf", "f4v", "rmvb", "vob", "rm"}


class PhotoService(BaseService):
    def upload(self, user_id, uploads, descriptions):
        if len(uploads) != len(descriptions) or not uploads:
            raise BusinessError(-1)
        objects = []
        with self.atomic(user_id) as files:
            user = self.session.get(User, user_id)
            # Validate every namespace before writing any file in a batch.
            for data in descriptions:
                if not isinstance(data, dict):
                    raise BusinessError(-1)
                self.owned(Namespace, data.get("nsId"), user_id, -2)
            for upload, data in zip(uploads, descriptions):
                filename = upload.filename
                if not filename or "." not in filename:
                    raise BusinessError(-1)
                stem, suffix = filename.rsplit(".", 1)
                valid_name(stem)
                if not re.fullmatch(r"[a-zA-Z0-9]{1,20}", suffix):
                    raise BusinessError(-1)
                visibility = data.get("visibility", 0)
                if visibility not in (0, 1, 2):
                    visibility = 0
                photo = Photo(photo_name=stem, format=suffix, upload_date=self.now(), ns_id=data["nsId"],
                              visibility=visibility, token=str(uuid4()), photo_uuid=str(uuid4()), user_id=user_id)
                path = self.storage.media(user.uuid, photo)
                files.install(upload.stream, path)
                self.storage.thumbnail(path, self.storage.media(user.uuid, photo, True), files)
                self.session.add(photo)
                self.session.flush()
                metadata = self.infra.metadata.read(path)
                self.session.add(self.infra.metadata.extra(photo.photo_id, metadata))
                objects.append(photo)
        return objects

    def replace(self, user_id, photo_id, upload):
        """Swap an owned image's content in place; id, UUID, share token, tags and galleries are kept."""
        filename = upload.filename or ""
        suffix = filename.rsplit(".", 1)[-1].lower() if "." in filename else ""
        if suffix not in EDITABLE_FORMATS:
            raise BusinessError(-1)
        with self.atomic(user_id) as files:
            photo = self.owned(Photo, photo_id, user_id)
            if photo.format.lower() not in EDITABLE_FORMATS:
                raise BusinessError(-1)
            user = self.session.get(User, user_id)
            files.remove(self.storage.media(user.uuid, photo, True))
            files.remove(self.storage.media(user.uuid, photo))
            photo.format = suffix
            path = self.storage.media(user.uuid, photo)
            files.install(upload.stream, path)
            self.storage.thumbnail(path, self.storage.media(user.uuid, photo, True), files)
        return 1

    def all_for_user(self, user_id):
        return self.session.scalars(select(Photo).where(Photo.user_id == user_id).order_by(Photo.photo_id)).all()

    def by_namespace(self, user_id, ns_id):
        return self.session.scalars(select(Photo).where(Photo.user_id == user_id, Photo.ns_id == ns_id).order_by(Photo.photo_id)).all()

    def by_uuid(self, user_id, uuid):
        item = self.session.scalar(select(Photo).where(Photo.user_id == user_id, Photo.photo_uuid == uuid))
        if item is None:
            raise BusinessError(-1)
        return item

    def by_gallery(self, user_id, ga_id):
        self.owned(Gallery, ga_id, user_id, None)
        return self.session.scalars(select(Photo).join(GalleryPhoto, GalleryPhoto.photo_id == Photo.photo_id)
            .where(GalleryPhoto.ga_id == ga_id, Photo.user_id == user_id).order_by(Photo.photo_id)).all()

    def by_tags(self, user_id, tag_ids):
        ids = set(tag_ids)
        if not ids:
            return []
        for key in ids:
            self.owned(Tag, key, user_id, None)
        subquery = select(TagPhoto.photo_id).where(TagPhoto.user_id == user_id, TagPhoto.tag_id.in_(ids))\
            .group_by(TagPhoto.photo_id).having(func.count(func.distinct(TagPhoto.tag_id)) == len(ids))
        return self.session.scalars(select(Photo).where(Photo.user_id == user_id, Photo.photo_id.in_(subquery)).order_by(Photo.photo_id)).all()

    def delete_record(self, user_id, photo, files):
        user = self.session.get(User, user_id)
        count = files.remove(self.storage.media(user.uuid, photo))
        count += files.remove(self.storage.media(user.uuid, photo, True))
        self.session.delete(photo)
        return count

    def delete(self, user_id, photo_id):
        with self.atomic(user_id) as files:
            self.delete_record(user_id, self.owned(Photo, photo_id, user_id), files)
        return 1

    def update(self, user_id, data):
        with self.atomic(user_id):
            item = self.owned(Photo, data.get("photoId"), user_id)
            if data.get("photoName") is not None:
                item.photo_name = valid_name(data["photoName"])
            if data.get("visibility") is not None:
                if data["visibility"] not in (0, 1, 2):
                    raise BusinessError(-1)
                item.visibility = data["visibility"]
        return 1

    def move(self, user_id, photo_id, ns_id):
        with self.atomic(user_id):
            photo = self.owned(Photo, photo_id, user_id)
            self.owned(Namespace, ns_id, user_id)
            photo.ns_id = ns_id
        return 1

    def add_tags(self, user_id, photo_id, tag_ids):
        with self.atomic(user_id):
            self.owned(Photo, photo_id, user_id)
            for key in tag_ids:
                self.owned(Tag, key, user_id)
            count = 0
            for key in set(tag_ids):
                existing = self.session.scalar(select(TagPhoto).where(TagPhoto.photo_id == photo_id, TagPhoto.tag_id == key))
                if existing is None:
                    self.session.add(TagPhoto(photo_id=photo_id, tag_id=key, user_id=user_id))
                    count += 1
        return count

    def delete_tags(self, user_id, photo_id, tag_ids):
        with self.atomic(user_id):
            self.owned(Photo, photo_id, user_id)
            for key in tag_ids:
                self.owned(Tag, key, user_id)
            count = 0
            for key in set(tag_ids):
                item = self.session.scalar(select(TagPhoto).where(TagPhoto.user_id == user_id, TagPhoto.photo_id == photo_id, TagPhoto.tag_id == key))
                if item:
                    self.session.delete(item)
                    count += 1
        return count

    def gallery_link(self, user_id, photo_id, ga_id, remove=False):
        with self.atomic(user_id):
            self.owned(Photo, photo_id, user_id)
            self.owned(Gallery, ga_id, user_id)
            item = self.session.scalar(select(GalleryPhoto).where(GalleryPhoto.photo_id == photo_id, GalleryPhoto.ga_id == ga_id))
            if remove:
                if item is None:
                    return 0
                self.session.delete(item)
            else:
                if item is not None:
                    return -1
                self.session.add(GalleryPhoto(photo_id=photo_id, ga_id=ga_id))
        return 1

    def tags(self, user_id, photo_id):
        self.owned(Photo, photo_id, user_id, None)
        query = select(Tag).join(TagPhoto, TagPhoto.tag_id == Tag.tag_id).where(TagPhoto.photo_id == photo_id, Tag.user_id == user_id).order_by(Tag.tag_id)
        return [{"photo_id": photo_id, "tag_id": tag.tag_id, "tag_name": tag.tag_name} for tag in self.session.scalars(query)]

    def galleries(self, user_id, photo_id):
        self.owned(Photo, photo_id, user_id, None)
        query = select(Gallery).join(GalleryPhoto, GalleryPhoto.ga_id == Gallery.ga_id).where(GalleryPhoto.photo_id == photo_id, Gallery.user_id == user_id).order_by(Gallery.ga_id)
        return [{"photo_id": photo_id, "ga_id": ga.ga_id, "ga_name": ga.ga_name} for ga in self.session.scalars(query)]

    def path(self, user_id, photo_id, thumbnail=False):
        photo = self.owned(Photo, photo_id, user_id, None)
        user = self.session.get(User, user_id)
        path = self.storage.media(user.uuid, photo, thumbnail)
        if thumbnail and not path.is_file():
            path = self.storage.media(user.uuid, photo)
        if not path.is_file():
            raise BusinessError(None, 404)
        return path

    def shared_path(self, path, token, thumbnail=False):
        # Resolve from the database, not from a client-provided filesystem path.
        if not isinstance(path, str) or len(path.split("/")) != 2:
            raise BusinessError(None, 403)
        user_uuid, filename = path.split("/")
        if "." not in filename:
            raise BusinessError(None, 403)
        photo_uuid = filename.rsplit(".", 1)[0]
        if thumbnail:
            if not photo_uuid.endswith("_thumbnail"):
                raise BusinessError(None, 403)
            photo_uuid = photo_uuid[:-10]
        photo = self.session.scalar(select(Photo).where(Photo.photo_uuid == photo_uuid))
        if photo is None or photo.visibility not in (1, 2) or (photo.visibility == 1 and photo.token != token):
            raise BusinessError(None, 403)
        user = self.session.get(User, photo.user_id)
        canonical = self.storage.media(user.uuid, photo, thumbnail)
        if user_uuid != user.uuid or filename != canonical.name:
            raise BusinessError(None, 403)
        if not canonical.is_file():
            raise BusinessError(None, 404)
        return canonical

    def share_url(self, user_id, photo_id):
        photo = self.owned(Photo, photo_id, user_id, None, 403)
        suffix = photo.format.lower()
        if suffix in IMAGE_FORMATS:
            endpoint = "/photo/renderToken"
        elif suffix in VIDEO_FORMATS:
            endpoint = "/photo/renderVToken"
        else:
            raise BusinessError(None, 402)
        user = self.session.get(User, user_id)
        path = user.uuid + "/" + photo.photo_uuid + "." + photo.format
        return self.infra.config["PUBLIC_BASE_URL"] + endpoint + "?" + urlencode({"path": path, "token": photo.token}, safe="/")

    def exif(self, user_id, photo_id):
        self.owned(Photo, photo_id, user_id, None, 404)
        path = self.path(user_id, photo_id)
        return self.infra.metadata.public(self.infra.metadata.read(path), path)

    def qr(self, user_id, photo_id):
        return self.infra.qr.encode(self.share_url(user_id, photo_id))

    def search(self, user_id, option, query):
        statement = select(Photo).where(Photo.user_id == user_id)
        pattern = "%" + (query or "") + "%"
        if option == "photoName":
            statement = statement.where(Photo.photo_name.ilike(pattern))
        elif option == "namespace":
            statement = statement.join(Namespace, Photo.ns_id == Namespace.ns_id).where(Namespace.user_id == user_id, Namespace.ns_name.ilike(pattern))
        elif option == "gallery":
            statement = statement.join(GalleryPhoto, Photo.photo_id == GalleryPhoto.photo_id).join(Gallery, GalleryPhoto.ga_id == Gallery.ga_id).where(Gallery.user_id == user_id, Gallery.ga_name.ilike(pattern))
        elif option == "tag":
            statement = statement.join(TagPhoto, Photo.photo_id == TagPhoto.photo_id).join(Tag, TagPhoto.tag_id == Tag.tag_id).where(Tag.user_id == user_id, Tag.tag_name.ilike(pattern))
        else:
            return []
        return self.session.scalars(statement.distinct().order_by(Photo.photo_id)).all()

    def map_photos(self, user_id, ga_id=None):
        statement = select(Photo, PhotoExtra).join(PhotoExtra, PhotoExtra.f_photo_id == Photo.photo_id).where(Photo.user_id == user_id)
        if ga_id is not None:
            self.owned(Gallery, ga_id, user_id, None)
            statement = statement.join(GalleryPhoto, GalleryPhoto.photo_id == Photo.photo_id).where(GalleryPhoto.ga_id == ga_id)
        result = []
        for photo, extra in self.session.execute(statement.order_by(Photo.photo_id)):
            # Mapper XML returned JDBC timestamps, serialized by Spring's yyyy-MM-dd setting.
            row = {key: getattr(photo, key) for key in ("photo_id", "photo_name", "format", "ns_id", "token", "photo_uuid")}
            row["upload_date"] = photo.upload_date.strftime("%Y-%m-%d") if photo.upload_date else None
            row.update(extra_photo_gps_lat=extra.extra_photo_gps_lat, extra_photo_gps_lon=extra.extra_photo_gps_lon,
                       extra_photo_taken_date=extra.extra_photo_taken_date.strftime("%Y-%m-%d") if extra.extra_photo_taken_date else None)
            result.append(row)
        return result
