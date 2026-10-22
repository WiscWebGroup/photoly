from sqlalchemy import select, func
from app.entities import User, UserGroup
from app.services.base import BaseService, BusinessError, valid_name


def effective_limit(session, user):
    return user.max_storage_bytes if user.storage_override else session.get(UserGroup, user.group_id).max_storage_bytes


def storage_projection(session, user):
    group = session.get(UserGroup, user.group_id)
    limit = effective_limit(session, user)
    return {'usedBytes': user.used_bytes or 0, 'capacityBytes': limit,
            'groupId': group.id, 'groupName': group.name,
            'storageOverride': user.storage_override, 'maxStorageBytes': user.max_storage_bytes,
            'quotaSource': 'user' if user.storage_override else 'group',
            'overQuota': limit is not None and (user.used_bytes or 0) > limit}


def storage_limit(value):
    if value is not None and (type(value) is not int or not 0 <= value <= 9007199254740991):
        raise BusinessError('invalid_storage_limit')
    return value


class QuotaService(BaseService):
    def groups(self, uid):
        self.require_admin(uid)
        counts = dict(self.session.execute(select(User.group_id, func.count()).group_by(User.group_id)).all())
        return [{'id': g.id, 'name': g.name, 'maxStorageBytes': g.max_storage_bytes,
                 'isDefault': g.is_default, 'memberCount': counts.get(g.id, 0)}
                for g in self.session.scalars(select(UserGroup).order_by(UserGroup.id))]

    def save_group(self, uid, data, group_id=None):
        self.require_admin(uid)
        with self.atomic(policy_write=True):
            group = self.session.get(UserGroup, group_id) if group_id else UserGroup()
            if group is None:
                raise BusinessError('group_not_found', 404)
            group.name = valid_name(data.get('name'))
            group.max_storage_bytes = storage_limit(data.get('maxStorageBytes'))
            self.session.add(group)
            self.session.flush()
            identifier = group.id
        return identifier

    def delete_group(self, uid, group_id, target):
        self.require_admin(uid)
        with self.atomic(policy_write=True):
            group = self.session.get(UserGroup, group_id)
            replacement = self.session.get(UserGroup, target) if type(target) is int else None
            if group is None or group.is_default or replacement is None or target == group_id:
                raise BusinessError('invalid_group_reassignment')
            for user in self.session.scalars(select(User).where(User.group_id == group_id).with_for_update()):
                user.group_id = target
            self.session.flush()
            self.session.delete(group)
        return 1

    def set_user(self, uid, target, data):
        self.require_admin(uid)
        with self.atomic(target, policy_write=True):
            user = self.session.get(User, target)
            group = self.session.get(UserGroup, data.get('groupId')) if type(data.get('groupId')) is int else None
            if group is None or type(data.get('storageOverride')) is not bool:
                raise BusinessError('invalid_storage_settings')
            user.group_id = group.id
            user.storage_override = data['storageOverride']
            user.max_storage_bytes = storage_limit(data.get('maxStorageBytes')) if user.storage_override else None
        return storage_projection(self.session, user)
