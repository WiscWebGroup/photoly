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


class PostgresSessionStore(SessionStore):
    """Sessions in the photoly_session table shared with the Go binary (SESSION_BACKEND=postgres)."""

    def __init__(self, engine):
        self.engine = engine
        self.client = self  # health checks call sessions.client.ping()

    def ping(self):
        with self.engine.connect() as conn:
            conn.exec_driver_sql("SELECT 1")

    def create(self, user_id, days):
        token = str(uuid4())
        if days > 0:
            with self.engine.begin() as conn:
                conn.exec_driver_sql("INSERT INTO photoly_session VALUES (%s,%s, now() + make_interval(days => %s))",
                                     (token, user_id, days))
        return token

    def identify(self, token):
        if not token:
            return None
        with self.engine.connect() as conn:
            return conn.exec_driver_sql("SELECT user_id FROM photoly_session WHERE token=%s AND expires_at>now()",
                                        (token,)).scalar()

    def delete(self, token):
        if token:
            with self.engine.begin() as conn:
                conn.exec_driver_sql("DELETE FROM photoly_session WHERE token=%s", (token,))

    def revoke_user(self, user_id):
        with self.engine.begin() as conn:
            conn.exec_driver_sql("DELETE FROM photoly_session WHERE user_id=%s", (user_id,))
