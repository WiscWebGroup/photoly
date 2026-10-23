from app.services.smart import manual_member, decorate
import re
from uuid import uuid4
from urllib.parse import urlencode
from sqlalchemy import select, func, or_, and_
from app.entities import Photo, PhotoExtra, Namespace, Gallery, GalleryPhoto, Tag, TagPhoto, User, PhotoMedia
from app.services.base import BaseService, BusinessError, valid_name

IMAGE_FORMATS = {"jpg", "jpeg", "gif", "png", "webp", "ico", "bmp", "tif", "tiff", "svg", "psd", "raw", "arw", "pcd", "jfif", "heic", "heif"}
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
                objects.append(self.install_one(user, upload, data, files))
        return objects

    def prepare(self, path, work):
        """Build previews and inspect metadata before acquiring the user's write lock."""
        from app.infrastructure.media import DECODABLE, image_info, encode_image
        from PIL import Image, ImageOps
        prepared = {'metadata': self.infra.metadata.read(path)}
        if path.suffix[1:].lower() in DECODABLE:
            prepared['info'] = image_info(path)
            with Image.open(path) as image:
                image = ImageOps.exif_transpose(image).convert('RGB')
                image.thumbnail((400, 400))
                thumbnail = work / 'thumbnail.jpg'
                image.save(thumbnail, 'JPEG')
                prepared['thumbnail'] = thumbnail
            if path.suffix.lower() in {'.heic', '.heif'}:
                preview = work / 'preview.jpg'
                encode_image(path, preview, 'jpeg')
                prepared['preview'] = preview
        return prepared

    def install_one(self, user, upload, data, files, prepared=None):
        user_id = user.user_id
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
        photo.content_sha256 = files.install(upload.stream, path)
        photo.content_version = 1
        if prepared is None:
            try:
                prepared = self.prepare(path, files.work)
            except BusinessError as error:
                # Preserve the legacy synchronous upload failure envelope.
                if error.code == 415:
                    raise BusinessError(-1) from error
                raise
        if prepared.get('thumbnail'):
            with prepared['thumbnail'].open('rb') as stream:
                files.install(stream, self.storage.media(user.uuid, photo, True))
        self.session.add(photo)
        self.session.flush()
        from app.infrastructure.media import DECODABLE, asset_path
        if suffix.lower() in DECODABLE:
            info = prepared['info']
            media = PhotoMedia(photo_id=photo.photo_id, animated=info['animated'])
            if suffix.lower() in {'heic', 'heif'}:
                preview = prepared['preview']
                with preview.open('rb') as stream:
                    files.install(stream, asset_path(self.storage, user.uuid, photo, 'preview'))
                media.has_preview = True
            self.session.add(media)
        metadata = prepared['metadata']
        self.session.add(self.infra.metadata.extra(photo.photo_id, metadata))
        from app.ai.hooks import photo_changed
        photo_changed(self.session, photo)
        return photo

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
            media = self.session.get(PhotoMedia, photo.photo_id)
            if media and media.has_preview:
                from app.infrastructure.media import asset_path
                files.remove(asset_path(self.storage, user.uuid, photo, 'preview'))
                media.has_preview = False
            files.remove(self.storage.media(user.uuid, photo, True))
            files.remove(self.storage.media(user.uuid, photo))
            photo.format = suffix
            path = self.storage.media(user.uuid, photo)
            photo.content_sha256 = files.install(upload.stream, path)
            photo.content_version += 1
            self.session.flush()
            from app.ai.hooks import photo_changed
            photo_changed(self.session, photo)
            self.storage.thumbnail(path, self.storage.media(user.uuid, photo, True), files)
            from app.infrastructure.media import image_info
            if media is None:
                media = PhotoMedia(photo_id=photo.photo_id)
                self.session.add(media)
            media.animated = image_info(path)['animated']
        return 1

    def all_for_user(self, user_id):
        return self.session.scalars(select(Photo).where(Photo.user_id == user_id).order_by(Photo.photo_id)).all()

    def date_summary(self, user_id):
        """Aggregate in SQL, never materialize the library to count calendar days."""
        day = func.date(func.coalesce(PhotoExtra.extra_photo_taken_date, Photo.upload_date))
        rows = self.session.execute(select(day, func.count(Photo.photo_id)).outerjoin(
            PhotoExtra, PhotoExtra.f_photo_id == Photo.photo_id).where(
                Photo.user_id == user_id, func.lower(Photo.format).in_(IMAGE_FORMATS)
            ).group_by(day).order_by(day)).all()
        return {"totalCount": sum(count for _, count in rows),
                "undatedCount": next((count for stamp, count in rows if stamp is None), 0),
                "days": [{"date": stamp.isoformat(), "count": count}
                         for stamp, count in rows if stamp is not None]}

    def month_preview(self, user_id, month, order='latest'):
        from datetime import datetime
        try:
            if not isinstance(month, str) or not re.fullmatch(r'[0-9]{4}-[0-9]{2}', month):
                raise ValueError()
            start = datetime.strptime(month, '%Y-%m')
            end = (datetime(start.year + 1, 1, 1) if start.month == 12
                   else datetime(start.year, start.month + 1, 1)) if month != '9999-12' else None
            if order not in ('latest', 'oldest'):
                raise ValueError()
        except ValueError as error:
            raise BusinessError(-1) from error
        date = func.coalesce(PhotoExtra.extra_photo_taken_date, Photo.upload_date)
        query = select(Photo.photo_id).outerjoin(PhotoExtra, PhotoExtra.f_photo_id == Photo.photo_id).where(
            Photo.user_id == user_id, func.lower(Photo.format).in_(IMAGE_FORMATS), date >= start)
        if end is not None:
            query = query.where(date < end)
        query = query.order_by(date.desc() if order == 'latest' else date.asc(),
                              Photo.photo_id.desc() if order == 'latest' else Photo.photo_id.asc()).limit(10)
        return list(self.session.scalars(query))

    def by_date(self, user_id, order="latest", limit=100, cursor=None, day=None):
        """Owner-scoped keyset pagination; undated images always follow dated images."""
        import base64
        import json
        from datetime import datetime, timedelta
        from app.serialization import serialize
        if order not in ("latest", "oldest") or not 1 <= limit <= 200:
            raise BusinessError(-1)
        date = func.coalesce(PhotoExtra.extra_photo_taken_date, Photo.upload_date)
        statement = select(Photo, PhotoExtra.extra_photo_taken_date, date).outerjoin(
            PhotoExtra, PhotoExtra.f_photo_id == Photo.photo_id).where(
                Photo.user_id == user_id, func.lower(Photo.format).in_(IMAGE_FORMATS))
        if day is not None:
            if day == "undated":
                statement = statement.where(date.is_(None))
            else:
                try:
                    if not re.fullmatch(r"[0-9]{4}-[0-9]{2}-[0-9]{2}", day):
                        raise ValueError()
                    start = datetime.strptime(day, "%Y-%m-%d")
                    end = start + timedelta(days=1) if day != "9999-12-31" else None
                except (ValueError, TypeError) as error:
                    raise BusinessError(-1) from error
                statement = statement.where(date >= start)
                if end is not None:
                    statement = statement.where(date < end)
        if cursor:
            try:
                if len(cursor) > 512:
                    raise ValueError()
                value = json.loads(base64.b64decode(cursor, altchars=b"-_", validate=True))
                if not isinstance(value, dict) or value.get("order") != order or value.get("user") != user_id or value.get("day") != day:
                    raise ValueError()
                identifier = value["id"]
                if type(identifier) is not int or not 0 < identifier <= 2147483647:
                    raise ValueError()
                stamp = datetime.fromisoformat(value["date"]) if value["date"] is not None else None
                if stamp and stamp.tzinfo is not None:
                    raise ValueError()
            except (ValueError, TypeError, KeyError, UnicodeError) as error:
                raise BusinessError(-1) from error
            after_id = Photo.photo_id < identifier if order == "latest" else Photo.photo_id > identifier
            if stamp is None:
                statement = statement.where(date.is_(None), after_id)
            else:
                after_date = date < stamp if order == "latest" else date > stamp
                statement = statement.where(or_(date.is_(None), after_date, and_(date == stamp, after_id)))
        direction = date.desc() if order == "latest" else date.asc()
        rows = self.session.execute(statement.order_by(direction.nulls_last(),
            Photo.photo_id.desc() if order == "latest" else Photo.photo_id.asc()).limit(limit + 1)).all()
        items = [{**serialize(photo), "timelineDate": serialize(stamp),
                  "dateSource": "taken" if taken is not None else "upload" if stamp is not None else None}
                 for photo, taken, stamp in rows[:limit]]
        next_cursor = None
        if len(rows) > limit:
            last = items[-1]
            next_cursor = base64.urlsafe_b64encode(json.dumps({"order": order, "user": user_id,
                "id": last["photoId"], "date": last["timelineDate"], "day": day}).encode()).decode()
        return {"items": items, "nextCursor": next_cursor}

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

    def remove_assets(self, user_uuid, photo, files):
        from app.infrastructure.media import ASSET_SUFFIXES, asset_path
        count = sum(files.remove(asset_path(self.storage, user_uuid, photo, role)) for role in ASSET_SUFFIXES)
        media = self.session.get(PhotoMedia, photo.photo_id)
        if media:
            self.session.delete(media)
            self.session.flush()
        return count

    def preview_path(self, user_id, photo_id, thumbnail=False):
        photo = self.owned(Photo, photo_id, user_id, None)
        media = self.session.get(PhotoMedia, photo_id)
        if not thumbnail and media and media.has_preview:
            from app.infrastructure.media import asset_path
            path = asset_path(self.storage, self.session.get(User, user_id).uuid, photo, 'preview')
            if not path.is_file():
                raise BusinessError(None, 404)
            return path
        return self.path(user_id, photo_id, thumbnail)

    def delete_record(self, user_id, photo, files):
        user = self.session.get(User, user_id)
        count = self.remove_assets(user.uuid, photo, files)
        count += files.remove(self.storage.media(user.uuid, photo))
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
                manual_member(self.session, Tag, key, photo_id, True)
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
                manual_member(self.session, Tag, key, photo_id, False)
                item = self.session.scalar(select(TagPhoto).where(TagPhoto.user_id == user_id, TagPhoto.photo_id == photo_id, TagPhoto.tag_id == key))
                if item:
                    self.session.delete(item)
                    count += 1
        return count

    def gallery_link(self, user_id, photo_id, ga_id, remove=False):
        with self.atomic(user_id):
            self.owned(Photo, photo_id, user_id)
            self.owned(Gallery, ga_id, user_id)
            manual_member(self.session, Gallery, ga_id, photo_id, not remove)
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

    def tags(self, user_id, photo_id, include_smart=False):
        self.owned(Photo, photo_id, user_id, None)
        query = select(Tag).join(TagPhoto, TagPhoto.tag_id == Tag.tag_id).where(TagPhoto.photo_id == photo_id, Tag.user_id == user_id).order_by(Tag.tag_id)
        rows = [{"photo_id": photo_id, "tag_id": tag.tag_id, "tag_name": tag.tag_name} for tag in self.session.scalars(query)]
        return decorate(self.session, Tag, rows, True) if include_smart else rows

    def galleries(self, user_id, photo_id, include_smart=False):
        self.owned(Photo, photo_id, user_id, None)
        query = select(Gallery).join(GalleryPhoto, GalleryPhoto.ga_id == Gallery.ga_id).where(GalleryPhoto.photo_id == photo_id, Gallery.user_id == user_id).order_by(Gallery.ga_id)
        rows = [{"photo_id": photo_id, "ga_id": ga.ga_id, "ga_name": ga.ga_name} for ga in self.session.scalars(query)]
        return decorate(self.session, Gallery, rows, True) if include_smart else rows

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
        media = self.session.get(PhotoMedia, photo.photo_id)
        if not thumbnail and media and media.has_preview:
            from app.infrastructure.media import asset_path
            return asset_path(self.storage, user.uuid, photo, 'preview')
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

    def covers(self, user_id, source, keys):
        """Item count and the newest three still images for each owned folder, gallery or tag."""
        models = {"folder": (Namespace, None), "gallery": (Gallery, GalleryPhoto), "tag": (Tag, TagPhoto)}
        if source not in models or len(set(keys)) > 500:
            raise BusinessError(-1)
        model, link = models[source]
        keys = sorted(set(keys))
        for key in keys:
            self.owned(model, key, user_id)
        if not keys:
            return []
        if link is None:
            group, statement = Photo.ns_id, select(Photo.ns_id, Photo.photo_id, Photo.format)
        else:
            group = link.ga_id if link is GalleryPhoto else link.tag_id
            statement = select(group, Photo.photo_id, Photo.format).join(link, link.photo_id == Photo.photo_id)
        rows = self.session.execute(statement.where(Photo.user_id == user_id, group.in_(keys)).order_by(Photo.photo_id.desc())).all()
        found = {key: {"id": key, "count": 0, "coverIds": []} for key in keys}
        for key, photo_id, suffix in rows:
            item = found[key]
            item["count"] += 1
            if len(item["coverIds"]) < 3 and (suffix or "").lower() in IMAGE_FORMATS:
                item["coverIds"].append(photo_id)
        return list(found.values())

    def map_places(self, user_id):
        """The nearest town for each geotagged item, from the bundled GeoNames list."""
        from app.places import index
        rows = self.session.execute(select(Photo.photo_id, PhotoExtra.extra_photo_gps_lat, PhotoExtra.extra_photo_gps_lon)
            .join(PhotoExtra, PhotoExtra.f_photo_id == Photo.photo_id)
            .where(Photo.user_id == user_id, PhotoExtra.extra_photo_gps_lat.is_not(None), PhotoExtra.extra_photo_gps_lon.is_not(None))
            .order_by(Photo.photo_id)).all()
        places = index() if rows else None
        result = []
        for photo_id, lat, lon in rows:
            found = places.nearest(lat, lon)
            if found:
                result.append({"photoId": photo_id, **found})
        return result

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
