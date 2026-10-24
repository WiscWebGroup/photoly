import os
import pytest

pytestmark = pytest.mark.skipif(os.getenv("SESSION_BACKEND") != "postgres", reason="needs SESSION_BACKEND=postgres")


def test_postgres_session_store_lifecycle(world):
    store = world.app.extensions["photoly"].sessions
    token = store.create(2, 1)
    assert store.identify(token) == 2 and store.identify(None) is None
    assert store.identify(store.create(2, 0)) is None  # zero days: immediately expired
    store.revoke_user(2)
    assert store.identify(token) is None and store.identify("token-bob") == 3
    store.delete("token-bob")
    assert store.identify("token-bob") is None
