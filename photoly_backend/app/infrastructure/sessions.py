from uuid import uuid4


class SessionStore:
    def __init__(self, client):
        self.client = client

    def create(self, user_id, days):
        token = str(uuid4())
        # A duration of zero remains accepted, producing an immediately expired session.
        if days > 0:
            self.client.setex("photoly:session:" + token, days * 86400, user_id)
        return token

    def identify(self, token):
        if not token:
            return None
        value = self.client.get("photoly:session:" + token)
        try:
            return int(value) if value is not None else None
        except (TypeError, ValueError):
            return None

    def delete(self, token):
        if token:
            self.client.delete("photoly:session:" + token)

    def revoke_user(self, user_id):
        for key in self.client.scan_iter(match="photoly:session:*"):
            value = self.client.get(key)
            if value is not None and int(value) == user_id:
                self.client.delete(key)
