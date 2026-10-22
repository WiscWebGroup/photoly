from contextlib import contextmanager
from datetime import datetime
from math import ceil
from zoneinfo import ZoneInfo
from sqlalchemy import select, func, text, inspect
from app.entities import User, Setting
from app.serialization import page_result


class BusinessError(Exception):
    def __init__(self, value=-1, code=400):
        self.value, self.code = value, code
        super().__init__(str(value))


class BaseService:
    def __init__(self, session, infrastructure):
        self.session = session
        self.infra = infrastructure
        self.storage = infrastructure.storage

    def now(self):
        return datetime.now(ZoneInfo(self.infra.config["TIMEZONE"])).replace(tzinfo=None, microsecond=0)

    def owned(self, model, key, user_id, failure=-1, code=400):
        obj = self.session.get(model, key) if key is not None else None
        if obj is None or obj.user_id != user_id:
            raise BusinessError(failure, code)
        return obj

    def is_admin(self, user_id):
        user = self.session.get(User, user_id)
        return user is not None and user.role == "admin"

    def require_admin(self, user_id):
        if not self.is_admin(user_id):
            raise BusinessError(-2)

    def setting(self, name, default=None):
        item = self.session.scalar(select(Setting).where(Setting.setting_name == name))
        return item.setting_value if item is not None else default

    def page(self, query, page, rows):
        if page is None or rows is None or page < 1 or rows < 1:
            raise BusinessError(-1)
        total = self.session.scalar(select(func.count()).select_from(query.subquery()))
        return page_result(self.session.scalars(query.offset((page - 1) * rows).limit(rows)).all(), ceil(total / rows))

    @contextmanager
    def atomic(self, user_id=None, policy_write=False):
        # User-row locks serialize mutations of each user's directory tree, including
        # concurrent move/delete/upload requests across Gunicorn workers.
        with self.storage.mutation() as files:
            try:
                # A shared policy lock permits concurrent users; quota/group edits take
                # its exclusive counterpart before any user row, avoiding lock inversion.
                lock = 'pg_advisory_xact_lock' if policy_write else 'pg_advisory_xact_lock_shared'
                self.session.execute(text(f'SELECT {lock}(74831003)'))
                user = None
                if user_id is not None:
                    user = self.session.scalar(select(User).where(User.user_id == user_id)
                        .with_for_update().execution_options(populate_existing=True))
                    if user is None:
                        raise BusinessError(-1)
                    if user.used_bytes is None:
                        user.used_bytes = self.storage.usage(user.uuid)[0]
                yield files
                if user is not None and not inspect(user).deleted and user not in self.session.deleted:
                    from app.services.quota import effective_limit
                    delta = files.delta(self.storage.root / user.uuid)
                    limit = effective_limit(self.session, user)
                    if delta > 0 and limit is not None and user.used_bytes + delta > limit:
                        raise BusinessError('storage_quota_exceeded', 413)
                    user.used_bytes = max(0, user.used_bytes + delta)
                self.session.commit()
            except Exception:
                self.session.rollback()
                raise


def valid_name(value, root=False):
    if not isinstance(value, str) or not value.strip() or len(value) > 255:
        raise BusinessError(-1)
    if value in (".", "..") or any(char in value for char in ("/", "\\", "\x00", "\r", "\n")):
        if not (root and value == "/"):
            raise BusinessError(-1)
    return value
