from urllib.parse import urlsplit
from sqlalchemy import select, or_
from sqlalchemy.engine import make_url
from werkzeug.security import generate_password_hash
from app.entities import User, Setting
from app.services.base import BaseService, BusinessError
from app.services.user import UserService


class AdminService(BaseService):
    def ping_mysql(self, user_id):
        self.require_admin(user_id)
        return 1 if self.setting("ping") == "pong" else -1

    def ping_redis(self, user_id):
        self.require_admin(user_id)
        return 1 if self.infra.sessions.client.ping() else -1

    def addresses(self, user_id):
        self.require_admin(user_id)
        mysql = make_url(self.infra.config["SQLALCHEMY_DATABASE_URI"])
        redis = urlsplit(self.infra.config["REDIS_URL"])
        return {"mysql": f"{mysql.host}:{mysql.port or 3306}", "redis": f"{redis.hostname}:{redis.port or 6379}",
                "upload": str(self.storage.root), "config": "MySQL: <setting> table"}

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
