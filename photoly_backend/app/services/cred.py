from uuid import uuid4
from sqlalchemy import select
from app.entities import Cred, User
from app.services.base import BaseService, BusinessError


class CredService(BaseService):
    def authorization(self, auth):
        if not isinstance(auth, str):
            raise BusinessError(-1)
        result = "".join(char for char in "CRD" if char in auth.upper())
        if not result:
            raise BusinessError(-1)
        return result

    def create(self, user_id, data):
        with self.atomic(user_id):
            self.session.add(Cred(user_id=user_id, token=str(uuid4()), authorization=self.authorization(data.get("authorization"))))
        return 1

    def query(self, user_id):
        return self.session.scalars(select(Cred).where(Cred.user_id == user_id).order_by(Cred.cred_id)).all()

    def update(self, user_id, data):
        with self.atomic(user_id):
            item = self.owned(Cred, data.get("credId"), user_id)
            item.authorization = self.authorization(data.get("authorization"))
        return 1

    def delete(self, user_id, cred_id):
        with self.atomic(user_id):
            self.session.delete(self.owned(Cred, cred_id, user_id))
        return 1

    def identify(self, token):
        return self.session.scalar(select(Cred).join(User, Cred.user_id == User.user_id).where(Cred.token == token))
