from datetime import date, datetime
from sqlalchemy import inspect
from app.entities import User


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
        return {camel(attr.key): None if isinstance(value, User) and attr.key == "password"
                else serialize(getattr(value, attr.key)) for attr in mapper.column_attrs}
    return value


def result(value=None, code=200):
    return {"t": serialize(value), "msgCode": code}


def token_result(value, token=None, code=200):
    return {"t": value, "token": token, "msgCode": code}


def page_result(value=None, pages=0, code=200):
    return {"t": serialize(value), "pageNum": pages, "msgCode": code}
