"""Bounded organization steps. Only document vectors from the active profile are read."""
import hashlib
from datetime import datetime, timedelta
from uuid import uuid4
import numpy as np
from sqlalchemy import select, delete, func, case, tuple_
from sqlalchemy.dialects.postgresql import insert
from app.entities import Photo
from app.entities.ai import PhotoEmbeddingState, VideoFrameEmbedding
from app.entities.smart import SmartItem, SmartMedia, SmartMember, SmartLineage
from app.services.smart import ensure_catalog, sync_link

GAP = timedelta(hours=36)
SPAN = timedelta(days=14)
NEAR = timedelta(hours=6)
MIN_SCORE = .80
MIN_MARGIN = .05
MAX_ANCHORS = 20


def devices(session, media):
    desired = set()
    for source in ('camera', 'lens'):
        name = media.facts.get(source)
        if not name:
            continue
        stable = source + ':' + hashlib.sha256(name.casefold().encode()).hexdigest()
        item = session.scalar(select(SmartItem).where(SmartItem.user_id == media.user_id, SmartItem.stable_key == stable))
        if not item:
            item = SmartItem(user_id=media.user_id, source=source, stable_key=stable, name=name,
                basis={'kind': source.title(), 'method': 'EXIF device model'})
            session.add(item)
            session.flush()
        desired.add(item.item_id)
        ensure_catalog(session, item)
        session.execute(insert(SmartMember).values(item_id=item.item_id, photo_id=media.photo_id, content_version=media.content_version,
            automatic=True).on_conflict_do_update(index_elements=['item_id', 'photo_id'], set_={'automatic': True, 'content_version': media.content_version}))
        sync_link(session, item, session.get(SmartMember, (item.item_id, media.photo_id), populate_existing=True))
    old = session.scalars(select(SmartMember).join(SmartItem).where(SmartMember.photo_id == media.photo_id,
        SmartItem.source != 'trip', SmartMember.item_id.not_in(desired))).all()
    for member in old:
        member.automatic = False
        sync_link(session, session.get(SmartItem, member.item_id), member)


def inherit(session, parent, child):
    if parent.item_id == child.item_id:
        return
    session.execute(insert(SmartLineage).values(parent_id=parent.item_id, child_id=child.item_id).on_conflict_do_nothing())
    child.suppressed = child.suppressed or parent.suppressed
    child.custom_name = child.custom_name or parent.custom_name
    child.cover = child.cover or parent.cover
    # Copy explicit choices, not old automatic membership. Exclusions win conflicts.
    sql = insert(SmartMember).from_select(['item_id', 'photo_id', 'content_version', 'manual'],
        select(func.cast(child.item_id, SmartMember.item_id.type), SmartMember.photo_id, SmartMember.content_version, SmartMember.manual)
        .where(SmartMember.item_id == parent.item_id, SmartMember.manual != 0))
    session.execute(sql.on_conflict_do_update(index_elements=['item_id', 'photo_id'],
        set_={'manual': case((SmartMember.manual == 0, sql.excluded.manual), else_=func.least(SmartMember.manual, sql.excluded.manual))}))


def anchor_step(session, pref, cp):
    query = select(SmartMedia).join(Photo, Photo.photo_id == SmartMedia.photo_id).where(SmartMedia.user_id == pref.user_id,
        SmartMedia.content_version == Photo.content_version, SmartMedia.error.is_(None),
        SmartMedia.captured_at.is_not(None), SmartMedia.latitude.is_not(None))
    if cp.get('time'):
        query = query.where(tuple_(SmartMedia.captured_at, SmartMedia.photo_id) > tuple_(datetime.fromisoformat(cp['time']), cp['id']))
    media = session.scalar(query.order_by(SmartMedia.captured_at, SmartMedia.photo_id).limit(1))
    if not media:
        cp.update(phase='missing', cursor=0)
        return
    old = session.scalars(select(SmartItem).join(SmartMember).where(SmartMember.photo_id == media.photo_id, SmartItem.source == 'trip',
        SmartItem.user_id == pref.user_id).order_by(SmartItem.suppressed.desc(), SmartItem.ga_id.is_not(None).desc(), SmartItem.item_id)).all()
    start = datetime.fromisoformat(cp['start']) if cp.get('start') else None
    last = datetime.fromisoformat(cp['time']) if cp.get('time') else None
    new_group = last is None or media.captured_at - last > GAP or media.captured_at - start > SPAN
    if new_group:
        # A late upload just before an existing trip should retain that trip's identity.
        nearby = session.scalars(select(SmartItem).join(SmartMember).join(SmartMedia, SmartMedia.photo_id == SmartMember.photo_id)
            .where(SmartItem.user_id == pref.user_id, SmartItem.source == 'trip', SmartMedia.latitude.is_not(None),
                SmartMedia.captured_at.between(media.captured_at, media.captured_at + GAP))
            .order_by(SmartMedia.captured_at, SmartItem.item_id).limit(20)).all()
        old = list({row.item_id: row for row in [*old, *nearby]}.values())
        item = next((item for item in old if item.run != cp['run']), None)
        if item is None:
            item = SmartItem(user_id=pref.user_id, stable_key='trip:' + str(uuid4()), source='trip', name='Travel moments')
            session.add(item)
            session.flush()
        item.run = cp['run']
        item.basis = {'method': 'Capture time, nearby locations and visual similarity',
            'start': media.captured_at.isoformat(), 'end': media.captured_at.isoformat(), 'places': {},
            'startDate': media.facts.get('localDate'), 'endDate': media.facts.get('localDate'),
            'profileId': pref.profile_id, 'similarity': MIN_SCORE, 'margin': MIN_MARGIN, 'timezoneSources': []}
        cp.update(item=item.item_id, start=media.captured_at.isoformat())
    else:
        item = session.get(SmartItem, cp['item'])
    for parent in old:
        inherit(session, parent, item)
    basis = dict(item.basis)
    city = media.facts.get('city')
    places = dict(basis.get('places', {}))
    if city:
        places[city['name']] = places.get(city['name'], 0) + 1
    basis.update(end=media.captured_at.isoformat(), endDate=media.facts.get('localDate'), places=places,
        timezoneSources=sorted(set(basis.get('timezoneSources', []) + [media.facts.get('timezoneSource', 'unknown')])))
    item.basis = basis
    session.execute(insert(SmartMember).values(item_id=item.item_id, photo_id=media.photo_id, content_version=media.content_version,
        automatic=True, anchor=True, run=cp['run']).on_conflict_do_update(index_elements=['item_id', 'photo_id'],
        set_={'automatic': True, 'anchor': True, 'run': cp['run'], 'content_version': media.content_version}))
    cp.update(time=media.captured_at.isoformat(), id=media.photo_id)


def vectors(session, photo_id, profile_id):
    if not profile_id:
        return None
    state = session.scalar(select(PhotoEmbeddingState).join(Photo, Photo.photo_id == PhotoEmbeddingState.photo_id).where(
        PhotoEmbeddingState.photo_id == photo_id, PhotoEmbeddingState.profile_id == profile_id,
        PhotoEmbeddingState.status == 'ready', PhotoEmbeddingState.content_version == Photo.content_version))
    if not state:
        return None
    raw = [state.embedding] if state.embedding is not None else session.scalars(select(VideoFrameEmbedding.embedding).where(
        VideoFrameEmbedding.state_id == state.state_id, VideoFrameEmbedding.profile_id == profile_id).order_by(VideoFrameEmbedding.frame_index).limit(5)).all()
    if not raw:
        return None
    values = np.array([v.to_list() if hasattr(v, 'to_list') else v for v in raw], dtype=np.float32)
    norms = np.linalg.norm(values, axis=1, keepdims=True)
    if not np.isfinite(values).all() or np.any(norms <= 0):
        return None
    return values / norms


def missing_step(session, pref, cp):
    media = session.scalar(select(SmartMedia).join(Photo, Photo.photo_id == SmartMedia.photo_id).where(SmartMedia.user_id == pref.user_id,
        SmartMedia.photo_id > cp['cursor'], SmartMedia.captured_at.is_not(None), SmartMedia.latitude.is_(None),
        SmartMedia.content_version == Photo.content_version, SmartMedia.error.is_(None)).order_by(SmartMedia.photo_id).limit(1))
    if not media:
        cp.update(phase='items', cursor=0)
        return
    cp['cursor'] = media.photo_id
    source = vectors(session, media.photo_id, pref.profile_id)
    anchors = session.execute(select(SmartMember.item_id, SmartMedia.photo_id).join(SmartMedia, SmartMedia.photo_id == SmartMember.photo_id)
        .where(SmartMedia.user_id == pref.user_id, SmartMember.run == cp['run'], SmartMember.anchor.is_(True),
            SmartMedia.captured_at.between(media.captured_at - NEAR, media.captured_at + NEAR))
        .order_by(func.abs(func.extract('epoch', SmartMedia.captured_at - media.captured_at)), SmartMedia.photo_id).limit(MAX_ANCHORS)).all()
    scores = {}
    incomplete = False
    if source is not None:
        for item_id, photo_id in anchors:
            group = session.get(SmartItem, item_id)
            start, end = datetime.fromisoformat(group.basis['start']), datetime.fromisoformat(group.basis['end'])
            if media.captured_at - start > SPAN or end - media.captured_at > SPAN:
                continue
            target = vectors(session, photo_id, pref.profile_id)
            if target is None:
                cp['waiting'] = True
                incomplete = True
                continue
            scores[item_id] = max(scores.get(item_id, -1), float(np.max(source @ target.T)))
    elif anchors:
        cp['waiting'] = True
    if incomplete:
        return
    ranking = sorted(scores.items(), key=lambda row: (-row[1], row[0]))
    if ranking and ranking[0][1] >= MIN_SCORE and (len(ranking) == 1 or ranking[0][1] - ranking[1][1] >= MIN_MARGIN):
        item = session.get(SmartItem, ranking[0][0])
        basis = dict(item.basis)
        if media.captured_at < datetime.fromisoformat(basis['start']):
            basis.update(start=media.captured_at.isoformat(), startDate=media.facts.get('localDate'))
        if media.captured_at > datetime.fromisoformat(basis['end']):
            basis.update(end=media.captured_at.isoformat(), endDate=media.facts.get('localDate'))
        item.basis = basis
        session.execute(insert(SmartMember).values(item_id=ranking[0][0], photo_id=media.photo_id, content_version=media.content_version,
            automatic=True, anchor=False, run=cp['run']).on_conflict_do_update(index_elements=['item_id', 'photo_id'],
            set_={'automatic': True, 'anchor': False, 'run': cp['run'], 'content_version': media.content_version}))


def item_step(session, pref, cp):
    item = session.scalar(select(SmartItem).where(SmartItem.user_id == pref.user_id, SmartItem.item_id > cp['cursor'],
        SmartItem.source == 'trip').order_by(SmartItem.item_id).limit(1))
    if not item:
        cp.update(phase='members', cursor=0, member=0)
        return
    cp['cursor'] = item.item_id
    count = session.scalar(select(func.count()).select_from(SmartMember).where(SmartMember.item_id == item.item_id, SmartMember.run == cp['run']))
    anchors = session.scalar(select(func.count()).select_from(SmartMember).where(SmartMember.item_id == item.item_id, SmartMember.run == cp['run'], SmartMember.anchor.is_(True)))
    visible = count >= 5 and anchors >= 2
    if not visible and item.basis.get('eligible'):
        has_pins = session.scalar(select(SmartMember.photo_id).where(SmartMember.item_id == item.item_id, SmartMember.manual == 1).limit(1))
        merged = session.scalar(select(SmartLineage.child_id).join(SmartItem, SmartItem.item_id == SmartLineage.child_id).where(
            SmartLineage.parent_id == item.item_id, SmartItem.run == cp['run']).limit(1))
        visible = bool(has_pins and not merged)
    item.basis = {**item.basis, 'eligible': visible}
    if item.run == cp['run']:
        basis = dict(item.basis)
        places = sorted(basis.get('places', {}), key=lambda name: (-basis['places'][name], name))
        title = 'Times in ' + ' and '.join(places[:2]) if places else 'Travel moments'
        if len(places) > 2: title += f' + {len(places) - 2} places'
        dates = f"{basis['startDate']} – {basis['endDate']}"
        item.name = (title[:210] + ' · ' + dates)[:255]
        basis.update(mediaCount=count, gpsCount=anchors, eligible=visible)
        item.basis = basis
    ensure_catalog(session, item, visible)


def member_step(session, pref, cp):
    member = session.scalar(select(SmartMember).join(SmartItem).where(SmartItem.user_id == pref.user_id,
        SmartItem.source == 'trip', tuple_(SmartMember.item_id, SmartMember.photo_id) > tuple_(cp['cursor'], cp['member']))
        .order_by(SmartMember.item_id, SmartMember.photo_id).limit(1))
    if not member:
        pref.completed_revision = cp['revision']
        pref.reason = 'waiting_for_vectors' if cp.get('waiting') else None
        cp.clear()
        return
    cp.update(cursor=member.item_id, member=member.photo_id)
    member.automatic = member.run == cp['run']
    sync_link(session, session.get(SmartItem, member.item_id), member)
