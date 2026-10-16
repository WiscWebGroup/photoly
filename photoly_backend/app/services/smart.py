"""Preferences and manual edits are independent of automatic processing eligibility."""
from sqlalchemy import select, update, delete, func, text
from sqlalchemy.dialects.postgresql import insert
from app.entities import User, Photo, Tag, Gallery, TagPhoto, GalleryPhoto
from app.entities.smart import SmartPreference, SmartMedia, SmartItem, SmartMember
from app.ai.state import settings, utcnow
from app.ai.types import AIError
from app.serialization import serialize


def preference(session, uid):
    session.execute(insert(SmartPreference).values(user_id=uid).on_conflict_do_nothing())
    return session.get(SmartPreference, uid)


def eligibility(session, uid, scheduling=False):
    cfg = settings(session)
    user, pref = session.get(User, uid), session.get(SmartPreference, uid)
    if not cfg['enabled']: return 'ai_disabled'
    if not cfg['smart_tag_enabled']: return 'smart_tag_disabled'
    if not user or not user.ai_search_enabled: return 'ai_permission_required'
    if not pref or not pref.enabled: return 'user_disabled'
    if scheduling:
        if cfg['maintenance']: return 'maintenance'
        if cfg['scheduler'] != 'running': return 'scheduler_' + cfg['scheduler']
    return None


def dirty(session, uid):
    session.execute(update(SmartPreference).where(SmartPreference.user_id == uid).values(revision=SmartPreference.revision+1))


def source_for(session, model, key):
    column = SmartItem.ga_id if model is Gallery else SmartItem.tag_id
    return session.scalar(select(SmartItem).where(column == key))


def dto(item):
    return {'id': item.item_id, 'source': item.source, 'name': item.custom_name or item.name,
            'suppressed': item.suppressed, 'basis': item.basis}


def decorate(session, model, rows, snake=False):
    """Explicit opt-in extension; legacy serialization remains byte-for-byte compatible."""
    if not rows:
        return []
    column, field = (SmartItem.ga_id, 'ga_id') if model is Gallery else (SmartItem.tag_id, 'tag_id')
    keys = [row[field] if isinstance(row, dict) else getattr(row, field) for row in rows]
    items = {getattr(item, field): item for item in session.scalars(select(SmartItem).where(column.in_(keys)))}
    return [{**serialize(row), 'smart': dto(items[key]) if key in items else None} for row, key in zip(rows, keys)]


def rename(session, model, obj):
    item = source_for(session, model, obj.ga_id if model is Gallery else obj.tag_id)
    if item:
        item.custom_name = obj.ga_name if model is Gallery else obj.tag_name
        if model is Gallery:
            item.cover = {'cover_id': obj.cover_id, 'cover_color': obj.cover_color}


def dismiss(session, model, obj):
    item = source_for(session, model, obj.ga_id if model is Gallery else obj.tag_id)
    if item:
        rename(session, model, obj)
        item.suppressed = True
        item.ga_id = item.tag_id = None
        dirty(session, item.user_id)


def manual_member(session, model, key, photo_id, included):
    item = source_for(session, model, key)
    if not item:
        return
    photo = session.get(Photo, photo_id)
    session.execute(insert(SmartMember).values(item_id=item.item_id, photo_id=photo_id,
        content_version=photo.content_version, manual=1 if included else -1).on_conflict_do_update(
        index_elements=['item_id', 'photo_id'], set_={'manual': 1 if included else -1}))
    dirty(session, item.user_id)


def sync_link(session, item, member):
    model, column, key = (GalleryPhoto, GalleryPhoto.ga_id, item.ga_id) if item.source == 'trip' else (TagPhoto, TagPhoto.tag_id, item.tag_id)
    if key is None:
        return
    photo = session.get(Photo, member.photo_id)
    include = member.manual == 1 or (member.manual == 0 and member.automatic and photo and member.content_version == photo.content_version)
    if include and not item.suppressed:
        values = {'photo_id': member.photo_id, column.key: key}
        if model is TagPhoto: values['user_id'] = item.user_id
        session.execute(insert(model).values(**values).on_conflict_do_nothing())
    else:
        session.execute(delete(model).where(column == key, model.photo_id == member.photo_id))


def ensure_catalog(session, item, visible=True):
    model = Gallery if item.source == 'trip' else Tag
    key = item.ga_id if model is Gallery else item.tag_id
    if item.suppressed or not visible:
        if key is not None:
            session.execute(delete(model).where(model.__mapper__.primary_key[0] == key))
            item.ga_id = item.tag_id = None
        return
    obj = session.get(model, key) if key is not None else None
    if obj is None:
        obj = Gallery(user_id=item.user_id, ga_name=item.custom_name or item.name,
            create_date=utcnow().replace(tzinfo=None), **(item.cover or {'cover_id': 3, 'cover_color': '#0d9488'})) if model is Gallery else Tag(user_id=item.user_id, tag_name=item.custom_name or item.name)
        session.add(obj)
        session.flush()
        if model is Gallery: item.ga_id = obj.ga_id
        else: item.tag_id = obj.tag_id
    elif not item.custom_name:
        if model is Gallery: obj.ga_name = item.name
        else: obj.tag_name = item.name


class SmartService:
    def __init__(self, session):
        self.session = session

    def get_settings(self, uid):
        pref = self.session.get(SmartPreference, uid)
        cfg = settings(self.session)
        return {'enabled': bool(pref and pref.enabled), 'available': bool(cfg['enabled'] and cfg['smart_tag_enabled'] and self.session.get(User, uid).ai_search_enabled),
                'reason': eligibility(self.session, uid), 'intervalSeconds': 60}

    def patch_settings(self, uid, data):
        if set(data) != {'enabled'} or type(data['enabled']) is not bool:
            raise AIError('invalid_settings')
        self.session.scalar(select(User).where(User.user_id == uid).with_for_update())
        pref = preference(self.session, uid)
        if pref.enabled != data['enabled']:
            pref.enabled = data['enabled']
            pref.revision += 1
        return self.get_settings(uid)

    def status(self, uid=None):
        media = select(func.count()).select_from(SmartMedia)
        photos = select(func.count()).select_from(Photo)
        failures = media.where(SmartMedia.error.is_not(None), SmartMedia.attempts >= 3)
        if uid is not None:
            media, failures, photos = media.where(SmartMedia.user_id == uid), failures.where(SmartMedia.user_id == uid), photos.where(Photo.user_id == uid)
        failed_jobs = select(func.count()).select_from(SmartPreference).where(SmartPreference.reason == 'smart_processing_failed')
        if uid is not None: failed_jobs = failed_jobs.where(SmartPreference.user_id == uid)
        result = {'processed': self.session.scalar(media), 'total': self.session.scalar(photos), 'failed': self.session.scalar(failures) + self.session.scalar(failed_jobs)}
        if uid is not None:
            pref = self.session.get(SmartPreference, uid)
            from app.entities.ai import AIControlState
            observed = self.session.get(AIControlState, 'smart')
            result.update(reason=eligibility(self.session, uid, True) or ((observed.observed.get('reason') if observed else None)) or (pref.reason if pref else None),
                phase=(pref.checkpoint.get('phase') if pref else None), pending=bool(pref and pref.revision > pref.completed_revision),
                updatedAt=pref.updated_at if pref else None)
        else:
            from app.ai.state import control
            result.update(control(self.session, 'smart').observed or {})
            if not result.get('reason') and self.session.scalar(select(SmartPreference.user_id).join(User).where(
                SmartPreference.enabled.is_(True), User.ai_search_enabled.is_(True), SmartPreference.reason == 'waiting_for_vectors').limit(1)):
                result['reason'] = 'waiting_for_vectors'
            cfg = settings(self.session)
            if not cfg['enabled']: result['reason'] = 'ai_disabled'
            elif not cfg['smart_tag_enabled']: result['reason'] = 'smart_tag_disabled'
            elif cfg['maintenance']: result['reason'] = 'maintenance'
            elif cfg['scheduler'] != 'running': result['reason'] = 'scheduler_' + cfg['scheduler']
        return result

    def refresh(self, uid):
        self.session.scalar(select(User).where(User.user_id == uid).with_for_update())
        pref = preference(self.session, uid)
        pref.revision += 1
        self.session.execute(update(SmartMedia).where(SmartMedia.user_id == uid, SmartMedia.error.is_not(None)).values(attempts=0))
        return {'queued': True}

    def dismissed(self, uid, after=0):
        rows = self.session.scalars(select(SmartItem).where(SmartItem.user_id == uid, SmartItem.suppressed.is_(True), SmartItem.item_id > after).order_by(SmartItem.item_id).limit(50)).all()
        return {'items': [dto(row) for row in rows], 'nextAfter': rows[-1].item_id if len(rows) == 50 else None}

    def restore(self, uid, key):
        self.session.scalar(select(User).where(User.user_id == uid).with_for_update())
        item = self.session.get(SmartItem, key)
        if not item or item.user_id != uid:
            raise AIError('smart_item_not_found', 404)
        # A dismissal inherited through a split/merge is restored as one lineage.
        ids = self.session.scalars(text('''WITH RECURSIVE family(id) AS (
            SELECT CAST(:id AS integer) UNION SELECT CASE WHEN l.parent_id=f.id THEN l.child_id ELSE l.parent_id END
            FROM smart_lineage l JOIN family f ON l.parent_id=f.id OR l.child_id=f.id)
            SELECT id FROM family'''), {'id': key}).all()
        self.session.execute(update(SmartItem).where(SmartItem.user_id == uid, SmartItem.item_id.in_(ids)).values(suppressed=False))
        self.session.flush()
        for restored in self.session.scalars(select(SmartItem).where(SmartItem.user_id == uid, SmartItem.item_id.in_(ids))):
            ensure_catalog(self.session, restored, restored.source != 'trip' or bool(restored.basis.get('eligible')))
            model, column, target = (GalleryPhoto, GalleryPhoto.ga_id, restored.ga_id) if restored.source == 'trip' else (TagPhoto, TagPhoto.tag_id, restored.tag_id)
            if target is not None:
                columns = ['photo_id', column.key] + (['user_id'] if model is TagPhoto else [])
                values = [SmartMember.photo_id, func.cast(target, column.type)] + ([func.cast(uid, TagPhoto.user_id.type)] if model is TagPhoto else [])
                self.session.execute(insert(model).from_select(columns, select(*values).join(Photo, Photo.photo_id == SmartMember.photo_id).where(
                    SmartMember.item_id == restored.item_id, Photo.user_id == uid,
                    (SmartMember.manual == 1) | ((SmartMember.manual == 0) & SmartMember.automatic.is_(True) & (SmartMember.content_version == Photo.content_version)))).on_conflict_do_nothing())
        self.refresh(uid)
        return {'restored': True}
