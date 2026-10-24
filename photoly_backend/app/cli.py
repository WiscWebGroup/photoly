import time
import click
from sqlalchemy import select, text, func
from app.extensions import db
from app.entities import Setting, User, Namespace, Gallery
from app.services.user import UserService
from app.infrastructure.schema import require_schema

DEFAULT_SETTINGS = {
    "OnlineDownload": ("0", "Allow public image URL imports"),
    "ping": ("pong", "use for testing connection"),
    "SignUp": ("1", "use for determine if new user could sign up"),
    "SSafeUUID": ("1", "use for determine if SSafe is enabled"),
    "TokenDuration": ("30", "use for determine the duration of token"),
}


def initialize(app):
    password = app.config["ADMIN_PASSWORD"]
    if not password:
        raise click.ClickException("Set ADMIN_PASSWORD before initializing a fresh deployment.")
    require_schema(db.session)
    users = UserService(db.session, app.extensions["photoly"])
    with users.atomic():
        db.session.execute(text("SELECT pg_advisory_xact_lock(74831002)"))
        for name, (value, note) in DEFAULT_SETTINGS.items():
            if db.session.scalar(select(Setting).where(Setting.setting_name == name)) is None:
                db.session.add(Setting(setting_name=name, setting_value=value, note=note))
        admin = users.by_email(app.config["ADMIN_EMAIL"])
        if admin is None:
            users.create({"email": app.config["ADMIN_EMAIL"], "password": password, "userName": "admin"}, "admin")
        else:
            if db.session.scalar(select(Namespace).where(Namespace.user_id == admin.user_id, Namespace.ns_parent_id == -1)) is None:
                db.session.add(Namespace(user_id=admin.user_id, ns_name="/", ns_parent_id=-1))
            if db.session.scalar(select(Gallery).where(Gallery.user_id == admin.user_id, Gallery.ga_name == "My Favorite")) is None:
                db.session.add(Gallery(user_id=admin.user_id, ga_name="My Favorite", create_date=users.now(), cover_id=2, cover_color="#FFFFFF"))


def register_cli(app):
    @app.cli.command("seed-db")
    def seed_db():
        """Seed defaults and administrator into an already migrated database."""
        initialize(app)
        click.echo("Photoly database initialized.")

    @app.cli.command("reconcile-storage")
    @click.option('--all-users', is_flag=True, help='Recount initialized accounts too.')
    def reconcile_storage(all_users):
        """Initialize/reconcile counters while holding each account's mutation lock."""
        users = UserService(db.session, app.extensions['photoly'])
        query = select(User.user_id)
        if not all_users:
            query = query.where(User.used_bytes.is_(None))
        ids = list(db.session.scalars(query))
        for uid in ids:
            with users.atomic(uid):
                user = db.session.get(User, uid)
                user.used_bytes = users.storage.usage(user.uuid)[0]
        click.echo(f'Reconciled {len(ids)} accounts.')

    @app.cli.command("repair-thumbnail-orientation")
    @click.option('--apply', is_flag=True, help='Rebuild thumbnails; without this flag only audit.')
    def repair_thumbnail_orientation(apply):
        """Repair legacy thumbnails whose originals carry EXIF rotations or reflections."""
        from PIL import Image
        from app.entities import Photo
        from app.services.photo import PhotoService
        from app.infrastructure.media import DECODABLE
        photos = PhotoService(db.session, app.extensions['photoly'])
        after, candidates, repaired, unreadable = 0, 0, 0, 0
        while True:
            rows = list(db.session.execute(select(Photo.photo_id, Photo.user_id).where(
                Photo.photo_id > after, func.lower(Photo.format).in_(DECODABLE)
            ).order_by(Photo.photo_id).limit(100)))
            db.session.rollback()
            if not rows:
                break
            for pid, uid in rows:
                after = pid
                photo = db.session.get(Photo, pid)
                user = db.session.get(User, uid)
                if photo is None or user is None:
                    db.session.rollback()
                    continue
                original = photos.storage.media(user.uuid, photo)
                try:
                    with Image.open(original) as image:
                        orientation = image.getexif().get(274, 1)
                except (OSError, ValueError, Image.DecompressionBombError):
                    unreadable += 1
                    db.session.rollback()
                    continue
                db.session.rollback()
                if orientation not in range(2, 9):
                    continue
                candidates += 1
                if apply:
                    # Re-read under the owner lock to serialize with overwrite/delete.
                    with photos.atomic(uid) as files:
                        photo = db.session.get(Photo, pid, populate_existing=True)
                        if photo is None:
                            continue
                        user = db.session.get(User, uid)
                        original = photos.storage.media(user.uuid, photo)
                        with Image.open(original) as image:
                            if image.getexif().get(274, 1) not in range(2, 9):
                                continue
                        photos.storage.thumbnail(original, photos.storage.media(user.uuid, photo, True), files)
                    repaired += 1
        click.echo(f'Orientation candidates: {candidates}; rebuilt: {repaired}; unreadable: {unreadable}. Originals unchanged.')

    @app.cli.command("check-schema")
    def check_schema():
        """Refuse to serve an uninitialized or incompatible schema."""
        require_schema(db.session)
        click.echo("Photoly schema and pgvector verified.")

    @app.cli.command("init-db")
    def init_db_compat():
        """Compatibility alias for seed-db; never performs DDL."""
        initialize(app)
        click.echo("Photoly defaults initialized.")

    @app.cli.command("wait-ready")
    @click.option("--timeout", default=90, type=int)
    def wait_ready(timeout):
        """Wait for PostgreSQL and Redis before the single initialization process starts."""
        deadline = time.monotonic() + timeout
        while True:
            try:
                db.session.execute(text("SELECT 1"))
                app.extensions["photoly"].sessions.client.ping()
                db.session.rollback()
                return
            except Exception as error:
                db.session.rollback()
                if time.monotonic() >= deadline:
                    raise click.ClickException("PostgreSQL or Redis was not ready before timeout") from error
                time.sleep(1)
