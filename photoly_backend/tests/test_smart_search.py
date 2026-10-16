import pytest
from app.extensions import db
from app.entities import Photo, Tag, TagPhoto, Gallery, GalleryPhoto
from tests.openapi_assertions import assert_documented_response


def search(world, query, headers=None):
    response = world.client.get('/photo/searchSmart', headers=headers or world.headers,
                                query_string={'query': query})
    assert_documented_response('GET', '/photo/searchSmart', response)
    return response.json


@pytest.mark.parametrize('query', [' SUNSET ', 'beach', 'Travel', 'sun beach travel'])
def test_matches_name_tag_and_gallery(world, query):
    assert [x['photoId'] for x in search(world, query)['t']] == [1]
    assert search(world, query, world.other)['t'] == []


def test_score_weights_maximum_association_and_stable_ties(world):
    with world.app.app_context():
        db.session.add_all([
            Photo(photo_id=3, photo_name='beach', user_id=2, ns_id=3, format='jpg'),
            Photo(photo_id=4, photo_name='BEACH', user_id=2, ns_id=3, format='jpg'),
            Photo(photo_id=5, photo_name='beach view', user_id=2, ns_id=3, format='jpg'),
            Photo(photo_id=6, photo_name='another', user_id=3, ns_id=4, format='jpg'),
            Tag(tag_id=3, user_id=2, tag_name='beach'),
            Tag(tag_id=4, user_id=3, tag_name='beach'),
            Gallery(ga_id=3, user_id=3, ga_name='beach'),
        ])
        db.session.flush()
        db.session.add_all([TagPhoto(tag_id=3, photo_id=1, user_id=2),
                            TagPhoto(tag_id=4, photo_id=2, user_id=3),
                            GalleryPhoto(ga_id=3, photo_id=2)])
        db.session.commit()
    # Two matching tags still contribute only 0.9, below the exact name's 1.2.
    assert [x['photoId'] for x in search(world, 'beach')['t']] == [3, 4, 1, 5]


def test_chinese_keywords_and_literal_wildcards(world):
    with world.app.app_context():
        db.session.get(Photo, 1).photo_name = '旅行 海边 100%_完成'
        db.session.commit()
    for query in ['旅行 海边', '%_', '100%', '_完成']:
        assert [x['photoId'] for x in search(world, query)['t']] == [1]
    assert search(world, '%missing')['t'] == []
    assert search(world, '   ') == {'msgCode': 200, 't': []}
    assert search(world, 'x' * 513)['msgCode'] == 400
    assert world.client.get('/photo/searchSmart?query=sunset').json['msgCode'] == 403
    assert world.client.get('/photo/searchSmart', headers=world.headers).json['msgCode'] == 400


def test_bounded_results(world):
    with world.app.app_context():
        db.session.add_all([Photo(photo_name='match', user_id=2, ns_id=3, format='jpg') for _ in range(105)])
        db.session.commit()
    photos = search(world, 'match')['t']
    assert len(photos) == 100
    assert [p['photoId'] for p in photos] == sorted(p['photoId'] for p in photos)
