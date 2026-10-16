"""Short-lived, session-bound query vectors. No durable search history."""
import hashlib
import json
import math
import re
import time
from uuid import uuid4
from .types import AIError

PREFIX = "photoly:similar:"
TTL = 900


class SimilarCursor:
    def __init__(self, redis, uid, token, photo_id):
        self.redis, self.uid, self.photo_id = redis, uid, photo_id
        self.owner = hashlib.sha256(token.encode()).hexdigest()

    def load(self, cursor):
        if not isinstance(cursor, str) or not re.fullmatch(r"[0-9a-f]{32}", cursor):
            raise AIError("invalid_cursor")
        raw = self.redis.get(PREFIX + "page:" + cursor)
        if not raw:
            raise AIError("cursor_expired", 409)
        page = json.loads(raw)
        raw = self.redis.get(PREFIX + "query:" + page["query"])
        if not raw:
            raise AIError("cursor_expired", 409)
        context = json.loads(raw)
        if context["uid"] != self.uid or context["owner"] != self.owner or context["photo"] != self.photo_id:
            raise AIError("invalid_cursor")
        if context["expires"] <= time.time():
            raise AIError("cursor_expired", 409)
        return context, page["after"]

    def create(self, *, pid, version, vector, page_size, threshold=None):
        context = {"id":uuid4().hex,"uid":self.uid,"owner":self.owner,"photo":self.photo_id,
                   "profile":pid,"version":version,"vector":list(map(float,vector)),
                   "size":page_size,"threshold":threshold,"expires":time.time()+TTL}
        self.redis.setex(PREFIX + "query:" + context["id"], TTL, json.dumps(context, allow_nan=False))
        return context

    def next(self, context, after):
        ttl = math.ceil(context["expires"]-time.time())
        if ttl <= 0:
            raise AIError("cursor_expired", 409)
        # A retry of the same page gets the same continuation key, without extending expiry.
        cursor = hashlib.sha256((context["id"]+json.dumps(after)).encode()).hexdigest()[:32]
        self.redis.setex(PREFIX + "page:" + cursor, ttl, json.dumps({"query":context["id"],"after":after}))
        return cursor
