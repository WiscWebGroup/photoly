from urllib.parse import urlsplit
from sqlalchemy import select, or_, text, func
from sqlalchemy.engine import make_url
from werkzeug.security import generate_password_hash
from app.entities import User, Setting, Photo
from app.serialization import serialize
from app.services.base import BaseService, BusinessError
from app.services.user import UserService


class AdminService(BaseService):
    def ping_database(self, user_id):
        self.require_admin(user_id)
        return self.session.scalar(text("SELECT 1"))

    def ping_mysql(self, user_id):
        """Legacy route alias; the active database is PostgreSQL."""
        return self.ping_database(user_id)

    def database_info(self, user_id):
        self.require_admin(user_id)
        url = make_url(self.infra.config["SQLALCHEMY_DATABASE_URI"])
        return {"type": "postgresql", "version": self.session.scalar(text("SHOW server_version")),
                "address": f"{url.host}:{url.port or 5432}",
                "pgvectorVersion": self.session.scalar(text("SELECT extversion FROM pg_extension WHERE extname='vector'")),
                "schemaRevision": self.session.scalar(text("SELECT version_num FROM alembic_version"))}

    def ping_redis(self, user_id):
        self.require_admin(user_id)
        return 1 if self.infra.sessions.client.ping() else -1

    def addresses(self, user_id):
        self.require_admin(user_id)
        database = make_url(self.infra.config["SQLALCHEMY_DATABASE_URI"])
        redis = urlsplit(self.infra.config["REDIS_URL"])
        return {"mysql": f"{database.host}:{database.port or 5432}", "redis": f"{redis.hostname}:{redis.port or 6379}",
                "upload": str(self.storage.root), "config": "PostgreSQL: <setting> table"}

    def settings(self, user_id):
        self.require_admin(user_id)
        return {item.setting_name: item.setting_value for item in self.session.scalars(select(Setting).order_by(Setting.setting_id))}

    def set_setting(self, user_id, name, value):
        self.require_admin(user_id)
        if name in ("TokenDuration", "SignUp", "SSafeUUID"):
            try:
                number = int(value)
            except (ValueError, TypeError):
                raise BusinessError(-1)
            if name == "TokenDuration" and not 0 <= number <= 365:
                raise BusinessError(-1, 403)
            if name != "TokenDuration" and number not in (0, 1):
                raise BusinessError(-1)
        with self.atomic(user_id):
            item = self.session.scalar(select(Setting).where(Setting.setting_name == name))
            if item is None:
                return 0
            item.setting_value = str(value)
        return 1

    def get_setting(self, user_id, name):
        self.require_admin(user_id)
        return self.setting(name)

    def set_general_setting(self, user_id, name, value):
        if not self.is_admin(user_id):
            return -1
        return self.set_setting(user_id, name, value)

    def users(self, user_id, page, rows, search=None):
        self.require_admin(user_id)
        query = select(User).order_by(User.user_id)
        if search is not None:
            pattern = "%" + search + "%"
            query = query.where(or_(User.user_name.ilike(pattern), User.email.ilike(pattern)))
        return self.page(query, page, rows)

    def user_list(self, user_id, page, rows, search=None, role=None):
        """One page of accounts with each one's media count, plus the total that match."""
        self.require_admin(user_id)
        if page is None or rows is None or page < 1 or not 1 <= rows <= 200 or role not in (None, "admin", "user"):
            raise BusinessError(-1)
        query = select(User)
        if search:
            pattern = "%" + search.replace("\\", "\\\\").replace("%", "\\%").replace("_", "\\_") + "%"
            query = query.where(or_(User.user_name.ilike(pattern, escape="\\"), User.email.ilike(pattern, escape="\\")))
        if role:
            query = query.where(User.role == role)
        total = self.session.scalar(select(func.count()).select_from(query.subquery()))
        users = self.session.scalars(query.order_by(User.user_id).offset((page - 1) * rows).limit(rows)).all()
        counts = dict(self.session.execute(select(Photo.user_id, func.count()).where(Photo.user_id.in_([u.user_id for u in users]))
                                           .group_by(Photo.user_id)).all()) if users else {}
        return {"users": [{**serialize(u), "mediaCount": counts.get(u.user_id, 0)} for u in users], "total": total}

    def add_user(self, user_id, data):
        self.require_admin(user_id)
        if data.get("userName") is None:
            raise BusinessError(-1)
        with self.atomic(user_id):
            role = data.get("role") if data.get("role") in ("user", "admin") else "user"
            UserService(self.session, self.infra).create(data, role)
        return 1

    def delete_user(self, user_id, target):
        self.require_admin(user_id)
        if target is None:
            return -2
        return UserService(self.session, self.infra).delete(user_id, target)

    def reset(self, user_id, data, field):
        self.require_admin(user_id)
        key = {"user_name": "userName", "email": "email", "password": "password", "role": "role"}[field]
        with self.atomic(user_id):
            user = self.session.get(User, data.get("userId")) if data.get("userId") is not None else None
            if user is None:
                return 0
            value = data.get(key)
            if not isinstance(value, str) or not value:
                raise BusinessError(-1)
            if field == "password":
                value = generate_password_hash(value)
            elif field == "role" and value not in ("user", "admin"):
                raise BusinessError(-1)
            elif field == "email":
                existing = UserService(self.session, self.infra).by_email(value)
                if existing and existing.user_id != user.user_id:
                    raise BusinessError(-1)
            setattr(user, field, value)
        return 1

    def stop(self, user_id):
        if not self.is_admin(user_id):
            return -2
        return self.infra.lifecycle.stop()
