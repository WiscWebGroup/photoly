"""Read-only summaries for the web app: storage, card covers, map place names and the admin user table."""
import pytest
from app.places import index


def get(world, path, headers=None, **params):
    return world.client.get(path, headers=world.headers if headers is None else headers, query_string=params).json


def test_storage_counts_only_the_users_own_files(world):
    alice = get(world, "/user/getStorage")
    assert alice["msgCode"] == 200
    assert alice["t"]["usedBytes"] > len(world.jpeg) + len(world.video)  # Originals plus the thumbnail.
    assert alice["t"]["capacityBytes"] > alice["t"]["usedBytes"]
    assert get(world, "/user/getStorage", world.other)["t"]["usedBytes"] == 0
    assert get(world, "/user/getStorage", {})["msgCode"] == 403


def test_covers_count_items_and_pick_newest_still_images(world):
    galleries = get(world, "/photo/getCovers", source="gallery", ids="2,1")
    assert galleries == {"t": [{"id": 1, "count": 0, "coverIds": []}, {"id": 2, "count": 1, "coverIds": [1]}], "msgCode": 200}
    # Folders count only what is directly inside; videos count but are never covers.
    folders = world.client.get("/photo/getCovers?source=folder&ids=2&ids=3", headers=world.headers).json["t"]
    assert folders == [{"id": 2, "count": 1, "coverIds": []}, {"id": 3, "count": 1, "coverIds": [1]}]
    assert get(world, "/photo/getCovers", source="tag", ids="1,2")["t"] == [
        {"id": 1, "count": 1, "coverIds": [1]}, {"id": 2, "count": 0, "coverIds": []}]
    assert get(world, "/photo/getCovers", source="tag", ids="")["t"] == []


@pytest.mark.parametrize("params", [{"source": "folder", "ids": "4"}, {"source": "gallery", "ids": "1,99"},
                                    {"source": "photo", "ids": "1"}, {"ids": "1"}, {"source": "tag", "ids": "x"}])
def test_covers_reject_foreign_ids_and_bad_sources(world, params):
    assert get(world, "/photo/getCovers", **params)["msgCode"] == 400


def test_map_places_name_each_geotagged_item(world):
    assert get(world, "/photo-extra/getMapPlaces") == {
        "t": [{"photoId": 1, "place": "New York City", "country": "United States"}], "msgCode": 200}
    assert get(world, "/photo-extra/getMapPlaces", world.other)["t"] == []


def test_place_index_prefers_towns_and_skips_empty_areas():
    places = index()
    assert places.nearest(64.1466, -21.9426) == {"place": "Reykjavík", "country": "Iceland"}
    assert places.nearest(51.5, -0.12) == {"place": "London", "country": "United Kingdom"}
    assert places.nearest(0, -30) is None  # Mid-Atlantic.
    assert places.nearest(None, 10) is None
    assert places.nearest(91, 0) is None


def test_admin_user_list_has_media_counts_role_filter_and_total(world):
    everyone = get(world, "/admin/getUsers", world.admin, page=1, rowsPerPage=2)
    assert everyone["msgCode"] == 200 and everyone["t"]["total"] == 3
    assert [(u["userName"], u["mediaCount"]) for u in everyone["t"]["users"]] == [("admin", 0), ("alice", 2)]
    assert everyone["t"]["users"][0]["password"] is None
    admins = get(world, "/admin/getUsers", world.admin, page=1, rowsPerPage=8, role="admin")["t"]
    assert admins["total"] == 1 and [u["userName"] for u in admins["users"]] == ["admin"]
    found = get(world, "/admin/getUsers", world.admin, page=1, rowsPerPage=8, search="BOB", role="user")["t"]
    assert found["total"] == 1 and found["users"][0]["email"] == "bob@test.local"
    # Search text is literal: "_" is not a wildcard.
    assert get(world, "/admin/getUsers", world.admin, page=1, rowsPerPage=8, search="_")["t"]["total"] == 0
    assert get(world, "/admin/getUsers", world.admin, page=9, rowsPerPage=8)["t"] == {"users": [], "total": 3}


@pytest.mark.parametrize("params", [{"page": 0, "rowsPerPage": 8}, {"page": 1, "rowsPerPage": 201},
                                    {"page": 1}, {"page": 1, "rowsPerPage": 8, "role": "owner"}])
def test_admin_user_list_rejects_bad_paging_and_roles(world, params):
    assert get(world, "/admin/getUsers", world.admin, **params)["msgCode"] == 400


def test_admin_user_list_is_for_administrators_only(world):
    assert get(world, "/admin/getUsers", page=1, rowsPerPage=8) == {"t": -2, "msgCode": 400}
    assert get(world, "/admin/getUsers", {}, page=1, rowsPerPage=8) == {"t": -2, "msgCode": 403}
