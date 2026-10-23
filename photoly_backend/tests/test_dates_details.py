"""Owner-scoped date pagination and recursive folder metadata summaries."""
from datetime import datetime
import base64
import json
import pytest
from sqlalchemy import select
from app.entities import Namespace, Photo, PhotoExtra
from app.extensions import db
from tests.openapi_assertions import assert_documented_response


def get(world, path, headers=None, **query):
    response = world.client.get(path, headers=world.headers if headers is None else headers, query_string=query)
    assert_documented_response('GET', path, response)
    return response.json


def seed(world, count=5):
    with world.app.app_context():
        for i in range(count):
            photo = Photo(photo_name=f'image-{i}', format='JPG', user_id=2, ns_id=2,
                          photo_uuid=f'image-{i}', upload_date=datetime(2025, 1, 1) if i != 2 else None)
            db.session.add(photo)
            db.session.flush()
            if i == 0:
                db.session.add(PhotoExtra(f_photo_id=photo.photo_id, extra_photo_taken_date=datetime(2020, 1, 1)))
        db.session.commit()


@pytest.mark.parametrize('order', ['latest', 'oldest'])
def test_dates_keyset_capture_fallback_undated_and_isolation(world, order):
    seed(world)
    cursor, records = None, []
    while True:
        data = get(world, '/photo/getByDate', order=order, limit=2, **({'cursor': cursor} if cursor else {}))['t']
        records.extend(data['items'])
        cursor = data['nextCursor']
        if not cursor:
            break
    assert len(records) == len({p['photoId'] for p in records}) == 6
    assert records[-1]['photoName'] == 'image-2' and records[-1]['dateSource'] is None
    dates = [(p['timelineDate'], p['photoId']) for p in records[:-1]]
    assert dates == sorted(dates, reverse=order == 'latest')
    capture = next(p for p in records if p['photoName'] == 'image-0')
    assert capture['timelineDate'] == '2020-01-01T00:00:00' and capture['dateSource'] == 'taken'
    assert capture['uploadDate'] == '2025-01-01T00:00:00'
    assert next(p for p in records if p['photoName'] == 'image-1')['dateSource'] == 'upload'
    assert get(world, '/photo/getByDate', world.other)['t']['items'] == []
    assert get(world, '/photo/getByDate', {})['msgCode'] == 403


def test_dates_large_library_defaults_and_ties(world):
    seed(world, 1000)
    page = get(world, '/photo/getByDate')['t']
    assert len(page['items']) == 100 and page['nextCursor']
    ids = []
    cursor = None
    while True:
        page = get(world, '/photo/getByDate', limit=200, **({'cursor': cursor} if cursor else {}))['t']
        ids += [p['photoId'] for p in page['items']]
        cursor = page['nextCursor']
        if cursor is None:
            break
    assert len(ids) == len(set(ids)) == 1001


@pytest.mark.parametrize('params', [{'order': 'bad'}, {'limit': 0}, {'limit': 201}, {'limit': 'bad'},
    {'cursor': 'garbage'}, {'cursor': base64.urlsafe_b64encode(b'[]').decode()},
    {'cursor': base64.urlsafe_b64encode(json.dumps({'order':'latest','user':2,'id':True,'date':None}).encode()).decode()}])
def test_dates_invalid_parameters(world, params):
    assert get(world, '/photo/getByDate', **params)['msgCode'] == 400


def test_cursor_is_bound_to_account_and_sort(world):
    seed(world)
    cursor = get(world, '/photo/getByDate', limit=1)['t']['nextCursor']
    assert get(world, '/photo/getByDate', world.other, cursor=cursor)['msgCode'] == 400
    assert get(world, '/photo/getByDate', order='oldest', cursor=cursor)['msgCode'] == 400


def test_details_recursive_sizes_devices_and_missing_files(world):
    with world.app.app_context():
        db.session.add_all([Namespace(ns_id=5, ns_name='Nested', ns_parent_id=3, user_id=2),
                            Namespace(ns_id=6, ns_name='Empty', ns_parent_id=5, user_id=2)])
        extra = db.session.query(PhotoExtra).filter_by(f_photo_id=1).one()
        extra.extra_photo_camera = '  Canon   EOS '
        extra.extra_photo_lens = ' RF  50mm '
        db.session.commit()
    original = get(world, '/namespace/getDetails', nsId=2)['t']
    assert (original['photoCount'], original['videoCount'], original['folderCount']) == (1, 1, 3)
    assert original['originalBytes'] == len(world.jpeg) + len(world.video)
    assert original['thumbnailBytes'] > 0
    assert original['totalBytes'] == original['originalBytes'] + original['thumbnailBytes']
    assert original['complete'] and original['missingFileCount'] == 0
    assert original['places'] == [{'name': 'New York City, United States', 'count': 1}]
    assert original['cameras'] == [{'name': 'Canon EOS', 'count': 1}]
    assert original['lenses'] == [{'name': 'RF 50mm', 'count': 1}]
    seed(world, 1)
    with world.app.app_context():
        added = db.session.query(Photo).filter_by(photo_name='image-0').one()
        added.ns_id = 5
        extra = db.session.query(PhotoExtra).filter_by(f_photo_id=added.photo_id).one()
        extra.extra_photo_camera = 'canon eos'
        extra.extra_photo_lens = 'RF 50mm'
        db.session.commit()
    partial = get(world, '/namespace/getDetails', nsId=3)['t']
    assert partial['photoCount'] == 2 and partial['folderCount'] == 2
    assert partial['cameras'] == [{'name': 'Canon EOS', 'count': 2}]
    assert not partial['complete'] and partial['missingFileCount'] == 2
    assert partial['originalBytes'] == len(world.jpeg)
    # Moving a descendant changes the recursive scope immediately.
    world.client.post('/namespace/updateParent', headers=world.headers, json={'nsId': 5, 'nsParentId': 2})
    assert get(world, '/namespace/getDetails', nsId=3)['t']['photoCount'] == 1
    world.client.post('/namespace/delete', headers=world.headers, query_string={'nsId': 5})
    assert get(world, '/namespace/getDetails', nsId=5)['msgCode'] == 400


def test_details_empty_and_ownership(world):
    empty = get(world, '/namespace/getDetails', world.other, nsId=4)['t']
    assert empty['totalBytes'] == empty['folderCount'] == empty['photoCount'] == empty['videoCount'] == 0
    assert empty['places'] == empty['cameras'] == empty['lenses'] == []
    assert empty['complete']
    for identifier in (4, -1, 999):
        assert get(world, '/namespace/getDetails', nsId=identifier)['msgCode'] == 400
    assert get(world, '/namespace/getDetails', {}, nsId=2)['msgCode'] == 403


def test_details_survive_legacy_cycles_without_duplicate_counts(world):
    with world.app.app_context():
        db.session.get(Namespace, 2).ns_parent_id = 3
        db.session.commit()
    data = get(world, '/namespace/getDetails', nsId=3)['t']
    assert data['folderCount'] == 1 and data['photoCount'] == 1 and data['videoCount'] == 1


def test_date_summary_matches_feed_and_owner_scope(world):
    seed(world)
    summary = get(world, '/photo/getDateSummary')['t']
    items = get(world, '/photo/getByDate')['t']['items']
    assert summary['totalCount'] == len(items) == 6
    assert summary['undatedCount'] == 1
    expected = {}
    for photo in items:
        if photo['timelineDate']:
            key = photo['timelineDate'][:10]
            expected[key] = expected.get(key, 0) + 1
    assert summary['days'] == [{'date': key, 'count': expected[key]} for key in sorted(expected)]
    assert get(world, '/photo/getDateSummary', world.other)['t'] == {
        'totalCount': 0, 'undatedCount': 0, 'days': []}
    assert get(world, '/photo/getDateSummary', {})['msgCode'] == 403


@pytest.mark.parametrize('order', ['latest', 'oldest'])
def test_dates_filtered_pagination_and_cursor_scope(world, order):
    seed(world, 205)
    cursor, items = None, []
    while True:
        page = get(world, '/photo/getByDate', day='2025-01-01', order=order, limit=100,
                   **({'cursor': cursor} if cursor else {}))['t']
        items += page['items']
        cursor = page['nextCursor']
        if not cursor:
            break
        for other_day in ('2020-01-01', 'undated', None):
            query = {'day': other_day} if other_day else {}
            assert get(world, '/photo/getByDate', order=order, cursor=cursor, **query)['msgCode'] == 400
    assert len(items) == len({p['photoId'] for p in items}) == 203
    assert all(p['timelineDate'].startswith('2025-01-01') for p in items)
    assert get(world, '/photo/getByDate', day='undated')['t']['items'][0]['photoName'] == 'image-2'
    unfiltered = get(world, '/photo/getByDate', limit=1)['t']['nextCursor']
    assert get(world, '/photo/getByDate', day='2025-01-01', cursor=unfiltered)['msgCode'] == 400


@pytest.mark.parametrize('day', ['', '2024-2-01', '2023-02-29', '2024-02-30', '2024-13-01',
                                 '0000-01-01', '2024-01-01T00:00:00', 'garbage'])
def test_dates_reject_invalid_day(world, day):
    assert get(world, '/photo/getByDate', day=day)['msgCode'] == 400


def test_dates_leap_day_boundaries_and_capture_precedence(world):
    with world.app.app_context():
        for index, stamp in enumerate((datetime(2024, 2, 28, 23, 59, 59), datetime(2024, 2, 29),
                                       datetime(2024, 2, 29, 23, 59, 59), datetime(2024, 3, 1))):
            photo = Photo(photo_name=f'boundary-{index}', format='JPG', user_id=2, ns_id=2,
                          photo_uuid=f'boundary-{index}', upload_date=datetime(2025, 1, 1))
            db.session.add(photo)
            db.session.flush()
            db.session.add(PhotoExtra(f_photo_id=photo.photo_id, extra_photo_taken_date=stamp))
        db.session.commit()
    items = get(world, '/photo/getByDate', day='2024-02-29')['t']['items']
    assert {p['photoName'] for p in items} == {'boundary-1', 'boundary-2'}
    summary = get(world, '/photo/getDateSummary')['t']
    assert {'date': '2024-02-29', 'count': 2} in summary['days']
    assert get(world, '/photo/getByDate', day='2024-04-01')['t']['items'] == []


def test_unfiltered_legacy_cursor_remains_compatible(world):
    seed(world)
    page = get(world, '/photo/getByDate', limit=1)['t']
    value = json.loads(base64.urlsafe_b64decode(page['nextCursor']))
    value.pop('day')
    legacy_cursor = base64.urlsafe_b64encode(json.dumps(value).encode()).decode()
    next_page = get(world, '/photo/getByDate', limit=1, cursor=legacy_cursor)['t']
    assert next_page['items'][0]['photoId'] != page['items'][0]['photoId']
    assert get(world, '/photo/getByDate', day='2025-01-01', cursor=legacy_cursor)['msgCode'] == 400


def test_month_preview_is_bounded_ordered_and_private(world):
    seed(world, 25)
    latest = get(world, '/photo/getMonthPreview', month='2025-01')['t']
    oldest = get(world, '/photo/getMonthPreview', month='2025-01', order='oldest')['t']
    assert len(latest) == len(oldest) == 10
    assert latest == sorted(latest, reverse=True)
    assert oldest == sorted(oldest)
    assert latest != oldest
    assert get(world, '/photo/getMonthPreview', world.other, month='2025-01')['t'] == []
    assert get(world, '/photo/getMonthPreview', {}, month='2025-01')['msgCode'] == 403
    assert len(get(world, '/photo/getMonthPreview', month='2020-01')['t']) == 1
    assert get(world, '/photo/getMonthPreview', month='2020-02')['t'] == []


@pytest.mark.parametrize('params', [{}, {'month':'2025-13'}, {'month':'2025-1'},
    {'month':'0000-01'}, {'month':'2025-01-01'}, {'month':'2025-01','order':'bad'}])
def test_month_preview_invalid_input(world, params):
    assert get(world, '/photo/getMonthPreview', **params)['msgCode'] == 400


def test_month_preview_leap_day_boundary_and_video_exclusion(world):
    with world.app.app_context():
        db.session.add_all([
            Photo(photo_name='leap', format='png', user_id=2, ns_id=2, photo_uuid='leap', upload_date=datetime(2024,2,29,23,59,59)),
            Photo(photo_name='march', format='png', user_id=2, ns_id=2, photo_uuid='march', upload_date=datetime(2024,3,1)),
            Photo(photo_name='video', format='mov', user_id=2, ns_id=2, photo_uuid='video', upload_date=datetime(2024,2,29)),
        ])
        db.session.commit()
        leap = db.session.scalar(select(Photo.photo_id).where(Photo.photo_name=='leap'))
    assert get(world, '/photo/getMonthPreview', month='2024-02')['t'] == [leap]
