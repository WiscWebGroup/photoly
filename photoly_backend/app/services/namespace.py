from sqlalchemy import select, func
from app.entities import Namespace, Photo, PhotoExtra, User, PhotoMedia
from app.infrastructure.media import asset_path, ASSET_SUFFIXES
from app.services.base import BaseService, BusinessError, valid_name


class NamespaceService(BaseService):
    def root(self, user_id):
        return self.session.scalar(select(Namespace).where(Namespace.user_id == user_id, Namespace.ns_parent_id == -1))

    def children(self, user_id, parent_id):
        return self.session.scalars(select(Namespace).where(Namespace.user_id == user_id, Namespace.ns_parent_id == parent_id).order_by(Namespace.ns_id)).all()

    def name(self, user_id, ns_id):
        return self.owned(Namespace, ns_id, user_id).ns_name

    def details(self, user_id, ns_id):
        from collections import Counter
        from app.services.photo import IMAGE_FORMATS, VIDEO_FORMATS
        from app.places import index
        folder = self.owned(Namespace, ns_id, user_id)
        user = self.session.get(User, user_id)
        # UNION deduplicates IDs, so even malformed legacy cycles terminate.
        descendants = select(Namespace.ns_id).where(Namespace.ns_id == folder.ns_id,
            Namespace.user_id == user_id).cte("descendants", recursive=True)
        descendants = descendants.union(select(Namespace.ns_id).join(descendants,
            Namespace.ns_parent_id == descendants.c.ns_id).where(Namespace.user_id == user_id))
        folder_count = self.session.scalar(select(func.count()).select_from(descendants)) - 1
        rows = self.session.execute(select(Photo, PhotoExtra).outerjoin(PhotoExtra,
            PhotoExtra.f_photo_id == Photo.photo_id).where(Photo.user_id == user_id,
                Photo.ns_id.in_(select(descendants.c.ns_id))).order_by(Photo.photo_id).execution_options(yield_per=500))
        photos = videos = original_bytes = thumbnail_bytes = missing = 0
        cameras, lenses, places = Counter(), Counter(), Counter()
        labels = {}
        def device(counter, value):
            name = " ".join((value or "").split())
            if name:
                key = name.casefold()
                labels.setdefault(key, name)
                counter[key] += 1
        for photo, extra in rows:
            suffix = (photo.format or "").lower()
            photos += suffix in IMAGE_FORMATS
            videos += suffix in VIDEO_FORMATS
            try:
                original_bytes += self.storage.media(user.uuid, photo).stat().st_size
            except OSError:
                missing += 1
            try:
                thumbnail_bytes += self.storage.media(user.uuid, photo, True).stat().st_size
            except FileNotFoundError:
                # Thumbnails are only generated for formats supported by Pillow.
                if suffix in {"jpg", "jpeg", "png", "gif", "jfif", "ico", "bmp", "tif", "tiff", "webp"}:
                    missing += 1
            except OSError:
                missing += 1
            media = self.session.get(PhotoMedia, photo.photo_id)
            if media:
                for role in ASSET_SUFFIXES:
                    resource = asset_path(self.storage, user.uuid, photo, role)
                    if resource.is_file():
                        original_bytes += resource.stat().st_size
            if extra is not None and suffix in IMAGE_FORMATS:
                device(cameras, extra.extra_photo_camera)
                device(lenses, extra.extra_photo_lens)
                place = index().nearest(extra.extra_photo_gps_lat, extra.extra_photo_gps_lon)
                if place:
                    places[(place["place"], place["country"])] += 1
        def devices(counter):
            return [{"name": labels[key], "count": count} for key, count in
                    sorted(counter.items(), key=lambda entry: (-entry[1], entry[0]))]
        return {"nsId": folder.ns_id, "nsName": folder.ns_name, "photoCount": photos,
                "videoCount": videos, "folderCount": folder_count, "originalBytes": original_bytes,
                "thumbnailBytes": thumbnail_bytes, "totalBytes": original_bytes + thumbnail_bytes,
                "missingFileCount": missing, "complete": missing == 0,
                "cameras": devices(cameras), "lenses": devices(lenses),
                "places": [{"name": name + ", " + country, "count": count}
                           for (name, country), count in sorted(places.items(), key=lambda entry: (-entry[1], entry[0]))]}

    def insert(self, user_id, data):
        with self.atomic(user_id):
            name = valid_name(data.get("nsName"))
            parent = self.owned(Namespace, data.get("nsParentId"), user_id)
            self.session.add(Namespace(ns_name=name, ns_parent_id=parent.ns_id, user_id=user_id))
        return 1

    def rename(self, user_id, data):
        with self.atomic(user_id):
            item = self.owned(Namespace, data.get("nsId"), user_id)
            if item.ns_parent_id == -1:
                raise BusinessError(-1)
            item.ns_name = valid_name(data.get("nsName"))
        return 1

    def move(self, user_id, data):
        with self.atomic(user_id):
            item = self.owned(Namespace, data.get("nsId"), user_id)
            parent = self.owned(Namespace, data.get("nsParentId"), user_id)
            if item.ns_parent_id == -1:
                raise BusinessError(-1)
            seen = {item.ns_id}
            cursor = parent
            while cursor is not None:
                if cursor.ns_id in seen:
                    raise BusinessError(-1)
                seen.add(cursor.ns_id)
                if cursor.ns_parent_id == -1:
                    break
                cursor = self.owned(Namespace, cursor.ns_parent_id, user_id)
            item.ns_parent_id = parent.ns_id
        return 1

    def trace(self, user_id, ns_id):
        item = self.owned(Namespace, ns_id, user_id, None)
        result, seen = [], set()
        while item is not None:
            if item.ns_id in seen:
                raise BusinessError(None)
            seen.add(item.ns_id)
            result.append({"nsId": item.ns_id, "nsName": item.ns_name})
            if item.ns_parent_id == -1:
                break
            item = self.owned(Namespace, item.ns_parent_id, user_id, None)
        return list(reversed(result))

    def tree(self, user_id, ns_id):
        item = self.owned(Namespace, ns_id, user_id)
        result, stack, seen = [], [(item, "root" if item.ns_name == "/" else valid_name(item.ns_name))], set()
        while stack:
            item, path = stack.pop()
            if item.ns_id in seen:
                raise BusinessError(-1)
            seen.add(item.ns_id)
            result.append((item, path))
            children = self.children(user_id, item.ns_id)
            names = set()
            for child in children:
                name = valid_name(child.ns_name)
                if name in names:
                    name += f" ({child.ns_id})"
                names.add(name)
                stack.append((child, path + "/" + name))
        return result

    def delete(self, user_id, ns_id):
        from app.services.photo import PhotoService
        with self.atomic(user_id) as files:
            item = self.owned(Namespace, ns_id, user_id)
            if item.ns_parent_id == -1:
                raise BusinessError(-1)
            tree = self.tree(user_id, ns_id)
            photos = PhotoService(self.session, self.infra)
            count = 0
            for ns, _ in reversed(tree):
                for photo in photos.by_namespace(user_id, ns.ns_id):
                    count += photos.delete_record(user_id, photo, files)
                self.session.flush()
                self.session.delete(ns)
                count += 1
        return count

    def download(self, user_id, ns_id):
        user = self.session.get(User, user_id)
        entries = []
        for ns, path in self.tree(user_id, ns_id):
            photos = self.session.scalars(select(Photo).where(Photo.user_id == user_id, Photo.ns_id == ns.ns_id))
            for photo in photos:
                name = valid_name(photo.photo_name) + "." + photo.format
                entries.append((self.storage.media(user.uuid, photo), path + "/" + name, photo.photo_id))
                media = self.session.get(PhotoMedia, photo.photo_id)
                if media and media.live:
                    entries.append((asset_path(self.storage, user.uuid, photo, 'live_original'), path + '/' + photo.photo_name + '.mov', photo.photo_id))
        return self.storage.archive(entries)
