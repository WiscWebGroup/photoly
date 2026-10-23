"""Bulk import and quota contracts. New responses use explicit projections."""
from ..builders import operation as op, parameter as p, json_body
from ..schemas import INTEGER, STRING, BOOLEAN, obj, array, nullable

LIMIT = nullable({'type':'integer','minimum':0,'maximum':9007199254740991})
STORAGE_FIELDS = {'usedBytes':INTEGER,'capacityBytes':LIMIT,'groupId':INTEGER,'groupName':STRING,
                  'storageOverride':BOOLEAN,'maxStorageBytes':LIMIT,'quotaSource':{'enum':['user','group']},'overQuota':BOOLEAN}
STORAGE_EXAMPLE = {'usedBytes':1200,'capacityBytes':50000000000,'groupId':1,'groupName':'Default',
                   'storageOverride':False,'maxStorageBytes':None,'quotaSource':'group','overQuota':False}
GROUP = obj({'id':INTEGER,'name':STRING,'maxStorageBytes':LIMIT,'isDefault':BOOLEAN,'memberCount':INTEGER})
GROUP_EXAMPLE = {'id':1,'name':'Default','maxStorageBytes':50000000000,'isDefault':True,'memberCount':3}
ITEM = obj({'id':STRING,'batchId':STRING,'path':STRING,'size':INTEGER,'state':{'enum':['pending','ready','processing','waiting','done','failed','skipped','cancelled','expanded']},
            'error':nullable(STRING),'photoId':nullable(INTEGER),'nsId':nullable(INTEGER),'sourceId':nullable(STRING)})
KEY = '00000000-0000-4000-8000-000000000001'
ITEM_EXAMPLE = {'id':KEY,'batchId':KEY,'path':'Trip/photo.jpg','size':1200,'state':'pending','error':None,'photoId':None,'nsId':None,'sourceId':None}
BATCH = obj({'id':STRING,'name':STRING,'nsId':nullable(INTEGER),'sealed':BOOLEAN,'cancelled':BOOLEAN,'createdAt':STRING,
             'counts':{'type':'object','additionalProperties':obj({'count':INTEGER,'bytes':INTEGER})}})
BATCH_PATH = [p('batch_id',STRING,'Owned upload batch UUID.',where='path')]
GROUP_PATH = [p('group_id',INTEGER,'Group ID.',where='path')]
PAGE = [p('page',{'type':'integer','minimum':1},'One-based page.',False)]
GROUP_BODY = json_body(obj({'name':STRING,'maxStorageBytes':LIMIT}),{'name':'Family','maxStorageBytes':50000000000})
OPERATIONS = [
    op('Uploads','create_upload','POST','/uploads','Create an idempotent upload batch',STRING,KEY,
       body=json_body(obj({'id':STRING,'nsId':INTEGER,'name':STRING}),{'id':KEY,'nsId':1,'name':'Trip'}),
       description='Batch UUID is supplied by the client. Destination must be owned. There is no batch file-count or total-byte limit. Apple Live Photo components must share one sealed batch and relative directory. The waiting state means metadata or another candidate file is still pending. Pairing uses ContentIdentifier, never filenames.'),
    op('Uploads','list_uploads','GET','/uploads','List recent private uploads',
       obj({'batches':array(BATCH),'total':INTEGER,'maxFileBytes':INTEGER,'activeCount':INTEGER,'failedCount':INTEGER}),{'batches':[],'total':0,'maxFileBytes':629145600,'activeCount':0,'failedCount':0},params=PAGE,
       description='20 batches per page; default retention 30 days. Counts separate transmission, processing, failures and skipped files. Cleared batches are excluded. failedCount counts only unacknowledged failures; per-batch counts still include all failures.'),
    op('Uploads','register_upload_items','POST','/uploads/{batch_id}/items','Register a page of upload entries',params=BATCH_PATH,
       body=json_body(obj({'items':{'type':'array','minItems':1,'maxItems':500,'items':obj({'id':STRING,'path':STRING,'size':INTEGER})}}),
                      {'items':[{'id':KEY,'path':'Trip/photo.jpg','size':1200}]}),
       description='Send as many manifest pages as needed before finish. Paths are relative; unsupported extensions are recorded as skipped. Entry UUIDs are idempotent.'),
    op('Uploads','list_upload_items','GET','/uploads/{batch_id}/items','Read upload results',
       obj({'items':array(ITEM),'total':INTEGER}),{'items':[ITEM_EXAMPLE],'total':1},params=BATCH_PATH+PAGE,
       description='50 items per page. Results include the imported photo/folder IDs. Deleted results have null IDs.'),
    op('Uploads','receive_upload_item','PUT','/uploads/{batch_id}/items/{item_id}/content','Send one file or ZIP',ITEM,ITEM_EXAMPLE,
       params=BATCH_PATH+[p('item_id',STRING,'Registered entry UUID.',where='path')],
       body={'required':True,'content':{'application/octet-stream':{'schema':{'type':'string','format':'binary'}}}},
       description='Raw file bytes, matching registered size; default maximum 600 MiB. A successful reply means durable receipt, not completed import. Repeating an already received item does not create another photo.'),
    op('Uploads','finish_upload','POST','/uploads/{batch_id}/finish','Seal the batch manifest',params=BATCH_PATH,
       description='Prevents further manifest entries. Registered pending content can still be sent. Received files process independently.'),
    op('Uploads','cancel_upload','POST','/uploads/{batch_id}/cancel','Cancel remaining upload work',params=BATCH_PATH,
       description='Already committed photos remain. Cancels pending work; an in-flight media transaction may finish first.'),
    op('Uploads','retry_upload','POST','/uploads/{batch_id}/retry','Retry unsuccessful items',params=BATCH_PATH,
       description='Received sources can be processed again; pending files require client retransmission. Successful photos are never duplicated. Temporary sources expire after 24 hours without activity.'),
    op('Admin','list_user_groups','GET','/admin/user-groups','List storage groups',array(GROUP),[GROUP_EXAMPLE]),
    op('Admin','create_user_group','POST','/admin/user-groups','Create a storage group',body=GROUP_BODY),
    op('Admin','edit_user_group','PUT','/admin/user-groups/{group_id}','Update a group limit',params=GROUP_PATH,body=GROUP_BODY,
       description='Each member inherits this limit unless overridden. Null means unlimited. Lower limits keep existing files.'),
    op('Admin','delete_user_group','DELETE','/admin/user-groups/{group_id}','Delete a group and reassign its members',params=GROUP_PATH,
       body=json_body(obj({'reassignTo':INTEGER}),{'reassignTo':1}),description='Default group cannot be deleted. An existing replacement group is required; personal overrides remain.'),
    op('Admin','set_user_storage','PUT','/admin/users/{user_id}/storage','Set membership and personal storage override',obj(STORAGE_FIELDS),STORAGE_EXAMPLE,
       params=[p('user_id',INTEGER,'Target user ID.',where='path')],
       body=json_body(obj({'groupId':INTEGER,'storageOverride':BOOLEAN,'maxStorageBytes':LIMIT}),{'groupId':1,'storageOverride':False,'maxStorageBytes':None}),
       description='storageOverride=false inherits the group. With override=true, a null limit is unlimited. Originals, thumbnails and avatars count toward usage.'),
]

OPERATIONS += [
    op('Uploads', 'clear_upload_activity', 'DELETE', '/uploads', 'Clear finished upload activity', array(STRING), [KEY],
       description='Hides sealed batches without pending work, including failed and cancelled imports, and returns cleared IDs. Photos, idempotency records and normal temporary-file expiry are preserved. Active uploads stay visible.'),
    op('Uploads', 'clear_upload_batch', 'DELETE', '/uploads/{batch_id}', 'Clear one finished upload activity', array(STRING), [KEY], params=BATCH_PATH,
       description='Owner-only and idempotent. Active or unsealed batches return msgCode 409. Does not delete photos.'),
    op('Uploads', 'acknowledge_upload_failures', 'POST', '/uploads/acknowledge-failures', 'Dismiss existing failure alerts',
       description='Acknowledges current failures across all pages for this user. Keeps activity and retries available. Later failures raise the alert again.'),
]
