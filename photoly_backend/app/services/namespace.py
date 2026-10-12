from sqlalchemy import select
from app.entities import Namespace, Photo, User
from app.services.base import BaseService, BusinessError, valid_name


class NamespaceService(BaseService):
    def root(self, user_id):
        return self.session.scalar(select(Namespace).where(Namespace.user_id == user_id, Namespace.ns_parent_id == -1))

    def children(self, user_id, parent_id):
        return self.session.scalars(select(Namespace).where(Namespace.user_id == user_id, Namespace.ns_parent_id == parent_id).order_by(Namespace.ns_id)).all()

    def name(self, user_id, ns_id):
        return self.owned(Namespace, ns_id, user_id).ns_name

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
        return self.storage.archive(entries)
