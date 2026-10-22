"""Account and login-session contracts."""
from ..builders import operation as op, parameter as p, token_parameter, json_body, upload_body
from ..schemas import INTEGER, STRING, PASSWORD, EMAIL, obj, ref, EXAMPLES

OPERATIONS = [
    op('User', 'get_storage', 'GET', '/user/getStorage', 'Read storage used by the current account',
       obj({'usedBytes': INTEGER, 'capacityBytes': INTEGER}), {'usedBytes': 51754511974, 'capacityBytes': 214748364800},
       failure=None, description='usedBytes counts the account\'s originals, thumbnails and avatar. capacityBytes is the size of the disk that holds all media; there are no per-account quotas.'),
    op('User', 'sign_up', 'POST', '/user/signUp', 'Register an account', STRING, 'Succeed', auth=None, shape='token',
       body=json_body(ref('SignUp'), {'email': 'reader@example.com', 'password': 'example-password', 'userName': 'Reader'}),
       description='Creates a root namespace and favorite gallery. Registration must be enabled by an administrator.',
       errors={'businessFailure': {'t': 'Error Create User', 'token': None, 'msgCode': 400},
               'registrationClosed': {'t': 'Register is closed by Admin', 'token': None, 'msgCode': 400},
               'duplicateEmail': {'t': 'Repeated Email Address', 'token': None, 'msgCode': 401}}),
    op('User', 'sign_in', 'POST', '/user/signIn', 'Create a login session', STRING, 'Succeed', auth=None, shape='token',
       body=json_body(ref('SignIn'), {'email': 'reader@example.com', 'password': 'example-password'}),
       description='Use the returned token in Authorize (HRD-Token). Session duration is controlled by the TokenDuration setting.'),
    op('User', 'sign_out', 'POST', '/user/signOut', 'Invalidate a login session', auth=None,
       params=[p('HRD-Token', STRING, 'Optional login session to invalidate. An absent or expired token is also successful.', False, 'header')]),
    op('User', 'get_info', 'GET', '/user/getInfo', 'Get the current account', ref('User'), EXAMPLES['User'], failure=None),
    op('User', 'update_username', 'POST', '/user/updateUsername', 'Change the display name',
       params=[p('username', STRING, 'New display name.', example='Reader')], form=True),
    op('User', 'update_email', 'POST', '/user/updateEmail', 'Change the email address',
       params=[p('email', EMAIL, 'New unique email address.', example='reader@example.com')], form=True),
    op('User', 'update_password', 'POST', '/user/updatePassword', 'Change the password',
       params=[p('oldPass', PASSWORD, 'Current password.'), p('newPass', PASSWORD, 'New password.')], form=True),
    op('User', 'update_avatar', 'POST', '/user/updateAvatar', 'Replace the account avatar', body=upload_body(avatar=True), description='Validates an image up to 20 MiB and 40 MP, applies orientation, center-crops and resizes to 512 x 512 JPEG. Transparent pixels become white. Invalid images are rejected without replacing the current avatar.'),
    op('User', 'get_avatar', 'GET', '/user/getAvatar/{token}', 'Read the account avatar', shape='binary', auth=None,
       params=[token_parameter()], mime='image/jpeg', description='Returns the default avatar when no custom avatar exists.'),
    op('User', 'delete_user', 'GET', '/user/deleteUser', 'Permanently delete the current account',
       description='DESTRUCTIVE: deletes the account, media files, folders, galleries, tags, API credentials and login sessions. This legacy endpoint uses GET.'),
    op('User', 'ping_token', 'POST', '/user/pingToken', 'Check a login session'),
]
