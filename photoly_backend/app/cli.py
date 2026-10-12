import time
import click
from sqlalchemy import select, text
from app.extensions import db
from app.entities import Setting, User, Namespace, Gallery
from app.services.user import UserService

DEFAULT_SETTINGS = {
    "ping": ("pong", "use for testing connection"),
    "SignUp": ("1", "use for determine if new user could sign up"),
    "SSafeUUID": ("1", "use for determine if SSafe is enabled"),
    "TokenDuration": ("30", "use for determine the duration of token"),
}


def initialize(app):
    password = app.config["ADMIN_PASSWORD"]
    if not password:
        raise click.ClickException("Set ADMIN_PASSWORD before initializing a fresh deployment.")
    db.create_all()
    users = UserService(db.session, app.extensions["photoly"])
    with users.atomic():
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
    @app.cli.command("init-db")
    def init_db():
        """Create missing tables, defaults and administrator without dropping or overwriting data."""
        initialize(app)
        click.echo("Photoly database initialized.")

    @app.cli.command("wait-ready")
    @click.option("--timeout", default=90, type=int)
    def wait_ready(timeout):
        """Wait for MySQL and Redis before the single initialization process starts."""
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
                    raise click.ClickException("MySQL or Redis was not ready before timeout") from error
                time.sleep(1)
