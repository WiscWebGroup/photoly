from datetime import date, datetime
from sqlalchemy import inspect
from app.entities import User


LEGACY_FIELDS = {'Photo': ['photo_id', 'photo_name', 'upload_date', 'format', 'ns_id', 'visibility', 'token', 'photo_uuid', 'user_id'], 'Tag': ['tag_id', 'user_id', 'tag_name'], 'Namespace': ['ns_id', 'ns_name', 'ns_parent_id', 'user_id'], 'Cred': ['cred_id', 'user_id', 'token', 'authorization'], 'User': ['user_id', 'user_name', 'email', 'create_date', 'role', 'password', 'uuid'], 'NamespacePhoto': ['np_id', 'ns_id', 'photo_id'], 'Gallery': ['ga_id', 'ga_name', 'user_id', 'create_date', 'cover_id', 'cover_color'], 'TagPhoto': ['tp_id', 'user_id', 'tag_id', 'photo_id'], 'PhotoExtra': ['extra_photo_id', 'f_photo_id', 'extra_photo_gps_lat', 'extra_photo_gps_lon', 'extra_photo_camera', 'extra_photo_lens', 'extra_photo_aperture', 'extra_photo_focal', 'extra_photo_iso', 'extra_photo_shutter_sec', 'extra_photo_taken_date', 'extra_photo_note'], 'Setting': ['setting_id', 'setting_name', 'setting_value', 'note'], 'GalleryPhoto': ['gp_id', 'ga_id', 'photo_id']}

def camel(name):
    first, *parts = name.split("_")
    return first + "".join(part.capitalize() for part in parts)


def serialize(value):
    if isinstance(value, datetime):
        # Spring's LocalDateTime uses ISO-8601, without an offset.
        return value.isoformat(timespec="seconds" if not value.microsecond else "microseconds")
    if isinstance(value, date):
        return value.isoformat()
    if isinstance(value, (list, tuple)):
        return [serialize(item) for item in value]
    if isinstance(value, dict):
        return {key: serialize(item) for key, item in value.items()}
    mapper = inspect(type(value), raiseerr=False)
    if mapper is not None and hasattr(mapper, "column_attrs"):
        fields = LEGACY_FIELDS.get(type(value).__name__)
        if fields is None:
            raise TypeError("Internal models require an explicit response projection")
        return {camel(key): None if isinstance(value, User) and key == "password"
                else serialize(getattr(value, key)) for key in fields}
    return value


def result(value=None, code=200):
    return {"t": serialize(value), "msgCode": code}


def token_result(value, token=None, code=200):
    return {"t": value, "token": token, "msgCode": code}


def page_result(value=None, pages=0, code=200):
    return {"t": serialize(value), "pageNum": pages, "msgCode": code}
