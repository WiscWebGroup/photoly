import re
from uuid import uuid4
from sqlalchemy import select, func
from werkzeug.security import generate_password_hash, check_password_hash
from app.entities import User, Namespace, Gallery, Tag, Cred
from app.serialization import token_result
from app.services.base import BaseService, BusinessError


class UserService(BaseService):
    def by_email(self, email):
        return self.session.scalar(select(User).where(func.lower(User.email) == str(email).lower()))

    def create(self, data, role="user"):
        email, password = data.get("email"), data.get("password")
        if not isinstance(email, str) or not re.fullmatch(r".+@.+\..+", email) or not isinstance(password, str) or not password:
            raise BusinessError(-1)
        if self.by_email(email):
            raise BusinessError(-1)
        user = User(email=email, password=generate_password_hash(password), user_name=data.get("userName"),
                    role=role, uuid=str(uuid4()), create_date=self.now())
        from app.ai.state import settings
        user.ai_search_enabled = settings(self.session)["new_user_default"]
        self.session.add(user)
        self.session.flush()
        self.session.add(Namespace(ns_name="/", ns_parent_id=-1, user_id=user.user_id))
        self.session.add(Gallery(ga_name="My Favorite", user_id=user.user_id, create_date=self.now(), cover_id=2, cover_color="#FFFFFF"))
        return user

    def sign_up(self, data):
        if self.setting("SignUp", "0") != "1":
            return token_result("Register is closed by Admin", code=400)
        email = data.get("email")
        if email is None:
            return token_result("Email is null", code=400)
        if not isinstance(email, str) or not re.fullmatch(r".+@.+\..+", email):
            return token_result("Email is incorrect", code=400)
        if self.by_email(email):
            return token_result("Repeated Email Address", code=401)
        with self.atomic():
            user = self.create(data)
        return self.login_result(user)

    def login_result(self, user):
        token = self.infra.sessions.create(user.user_id, int(self.setting("TokenDuration", "30")))
        return token_result("Succeed", token)

    def sign_in(self, data):
        user = self.by_email(data.get("email"))
        password = data.get("password")
        if user is None or not isinstance(password, str) or not user.password or not check_password_hash(user.password, password):
            return token_result("Invalid Email or Password", code=400)
        return self.login_result(user)

    def sign_out(self, token):
        self.infra.sessions.delete(token)
        return 1

    def identify(self, token):
        user_id = self.infra.sessions.identify(token)
        if user_id is not None and self.session.get(User, user_id) is not None:
            return user_id
        return None

    def info(self, user_id):
        return self.session.get(User, user_id)

    def update(self, user_id, field, value, old_password=None):
        with self.atomic(user_id):
            user = self.session.get(User, user_id)
            if field == "email":
                if value is None:
                    raise BusinessError(-1)
                if not isinstance(value, str) or not re.fullmatch(r".+@.+\..+", value):
                    raise BusinessError(-2)
                if self.by_email(value):
                    raise BusinessError(-2)
            elif field == "password":
                if not old_password or not check_password_hash(user.password, old_password):
                    raise BusinessError(-2)
                if not isinstance(value, str) or not value:
                    raise BusinessError(-1)
                value = generate_password_hash(value)
            elif not isinstance(value, str) or not value:
                raise BusinessError(-1)
            setattr(user, field, value)
        return 1

    def update_avatar(self, user_id, upload):
        if upload is None:
            raise BusinessError(-1)
        from app.services.avatar import avatar_jpeg
        image = avatar_jpeg(upload.stream)
        with self.atomic(user_id) as files:
            user = self.info(user_id)
            files.install(image, self.storage.path(user.uuid, user.uuid + ".jpg"))
        return 1

    def avatar(self, user_id):
        from pathlib import Path
        user = self.info(user_id)
        path = self.storage.path(user.uuid, user.uuid + ".jpg")
        return path if path.is_file() else Path(__file__).resolve().parents[1] / "assets/defaultAvatar.jpg"

    def delete(self, operator_id, user_id):
        user = self.session.get(User, user_id) if user_id is not None else None
        if user is None:
            return -5
        if operator_id != user_id and not self.is_admin(operator_id):
            return -2
        from app.services.photo import PhotoService
        with self.atomic(user_id) as files:
            count = 0
            namespaces = self.session.scalars(select(Namespace).where(Namespace.user_id == user_id)).all()
            # Include orphaned media as well as every folder, not only the root subtree.
            photos = PhotoService(self.session, self.infra)
            for photo in photos.all_for_user(user_id):
                count += photos.delete_record(user_id, photo, files)
            self.session.flush()
            for ns in namespaces:
                self.session.delete(ns)
                count += 1
            for model in (Tag, Gallery):
                objects = self.session.scalars(select(model).where(model.user_id == user_id)).all()
                count += len(objects)
                for obj in objects:
                    self.session.delete(obj)
            for cred in self.session.scalars(select(Cred).where(Cred.user_id == user_id)):
                self.session.delete(cred)
            files.remove(self.storage.path(user.uuid, user.uuid + ".jpg"))
            self.session.flush()
            self.session.delete(user)
            count += 1
        self.infra.sessions.revoke_user(user_id)
        return count
