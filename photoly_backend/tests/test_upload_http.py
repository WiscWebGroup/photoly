"""Real Gunicorn + independent upload worker acceptance in disposable Compose only."""
import io
import time
from concurrent.futures import ThreadPoolExecutor
from uuid import uuid4
from zipfile import ZipFile
import pytest
from tests.test_http import http, register

pytestmark = pytest.mark.http


def test_background_import_and_quota_retry_over_http(http, jpeg):
    headers, token = register(http)
    root = http('GET','/namespace/getRoot',headers=headers).json()['t']
    bid = str(uuid4())
    output = io.BytesIO()
    with ZipFile(output,'w',allowZip64=True) as archive:
        with archive.open('旅行/nested/photo.jpg','w',force_zip64=True) as stream:
            stream.write(jpeg)
        archive.writestr('notes.txt','skip me')
    archive_bytes = output.getvalue()
    entries = [{'id':str(uuid4()),'path':f'Directory/{n}.jpg','size':len(jpeg)} for n in range(18)]
    entries.append({'id':str(uuid4()),'path':'Archive.zip','size':len(archive_bytes)})
    assert http('POST','/uploads',headers=headers,json={'id':bid,'nsId':root['nsId'],'name':'HTTP import'}).json()['msgCode']==200
    assert http('POST',f'/uploads/{bid}/items',headers=headers,json={'items':entries}).json()['msgCode']==200
    assert http('POST',f'/uploads/{bid}/finish',headers=headers).json()['msgCode']==200
    def send(entry):
        return http('PUT',f'/uploads/{bid}/items/{entry["id"]}/content',headers={**headers,'Content-Type':'application/octet-stream'},
                    data=archive_bytes if entry['path'].endswith('.zip') else jpeg).json()['msgCode']
    with ThreadPoolExecutor(2) as pool:
        assert set(pool.map(send,entries))=={200}
    # This is a different session: work/history are tied to the account, not a page.
    info=http('GET','/user/getInfo',headers=headers).json()['t']
    http('POST','/user/signOut',headers=headers)
    new_token=http('POST','/user/signIn',json={'email':info['email'],'password':'smoke-password'}).json()['token']
    headers={'HRD-Token':new_token}
    deadline=time.monotonic()+45
    while True:
        rows=http('GET',f'/uploads/{bid}/items',headers=headers).json()['t']['items']
        if all(r['state'] in ('done','expanded','skipped','failed') for r in rows):break
        assert time.monotonic()<deadline,rows
        time.sleep(.2)
    assert sum(r['state']=='done' for r in rows)==19
    assert sum(r['state']=='skipped' for r in rows)==1
    assert http('GET','/user/getStorage',headers=headers).json()['t']['capacityBytes']==50_000_000_000
    # Admin API changes apply to a subsequent legacy upload as well.
    admin_token=http('POST','/user/signIn',json={'email':'admin@test.local','password':'test-password-only'}).json()['token']
    admin={'HRD-Token':admin_token}
    settings={'groupId':1,'storageOverride':True,'maxStorageBytes':0}
    changed=http('PUT',f'/admin/users/{info["userId"]}/storage',headers=admin,json=settings).json()
    assert changed['msgCode']==200 and changed['t']['overQuota']
    assert http('GET','/photo/render/'+new_token,headers=headers,params={'photoId':next(r['photoId'] for r in rows if r['state']=='done')}).content==jpeg
    assert http('GET','/user/deleteUser',headers=headers).json()['msgCode']==200
