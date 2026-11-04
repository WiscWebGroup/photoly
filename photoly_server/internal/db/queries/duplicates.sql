-- name: ListDuplicateGroups :one
WITH filtered AS (
 SELECT g.*,coalesce(i.version=g.version,false) AS ignored FROM duplicate_group g
 LEFT JOIN duplicate_ignore i ON i.user_id=g.user_id AND i.group_key=g.group_key
 WHERE g.user_id=sqlc.arg(user_id)::int AND coalesce(i.version=g.version,false)=sqlc.arg(ignored)::boolean
), page AS (SELECT * FROM filtered ORDER BY group_key LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int)
SELECT jsonb_build_object('items',coalesce((SELECT jsonb_agg(jsonb_build_object('key',group_key,'version',version,'count',count,'keptCount',kept_count,'ignored',ignored) ORDER BY group_key) FROM page),'[]'::jsonb),'total',(SELECT count(*) FROM filtered));

-- name: GetDuplicateGroup :one
SELECT group_key::text,version::text,count,kept_count FROM duplicate_group WHERE user_id=sqlc.arg(user_id)::int AND group_key=sqlc.arg(group_key)::text;

-- name: ListDuplicateMembers :one
WITH RECURSIVE folders AS (
 SELECT n.ns_id,n.ns_name::text AS path FROM namespace n WHERE n.user_id=sqlc.arg(user_id)::int AND n.ns_parent_id=-1
 UNION ALL SELECT n.ns_id,rtrim(f.path,'/') || '/' || n.ns_name FROM namespace n JOIN folders f ON n.ns_parent_id=f.ns_id WHERE n.user_id=sqlc.arg(user_id)::int
), page AS (
 SELECT c.*,coalesce(k.identity=c.identity,false) AS kept,coalesce(f.path,'/') AS folder,s.display_name AS source_name,s.kind AS source_type
 FROM duplicate_candidate c JOIN storage_source s ON s.id=c.source_id LEFT JOIN folders f ON f.ns_id=c.ns_id
 LEFT JOIN duplicate_keep k ON k.photo_id=c.photo_id AND k.user_id=c.user_id
 WHERE c.user_id=sqlc.arg(user_id)::int AND c.group_key=sqlc.arg(group_key)::text
 ORDER BY c.photo_id LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int
)
SELECT coalesce(jsonb_agg(jsonb_build_object('photoId',photo_id,'photoName',coalesce(photo_name,''),'fileName',file_name,'format',format,'contentVersion',content_version,'nsId',ns_id,'sourceId',source_id,'bytes',bytes,'kept',kept,'folder',folder,'sourceName',source_name,'sourceType',source_type) ORDER BY photo_id),'[]'::jsonb)::jsonb AS items FROM page;

-- name: DuplicateSources :many
SELECT DISTINCT source_id FROM duplicate_candidate WHERE user_id=sqlc.arg(user_id)::int AND group_key=sqlc.arg(group_key)::text ORDER BY source_id;

-- name: LockDuplicatePhotos :many
SELECT p.photo_id FROM photo p JOIN duplicate_candidate c USING(photo_id) WHERE c.user_id=sqlc.arg(user_id)::int AND c.group_key=sqlc.arg(group_key)::text ORDER BY p.photo_id FOR UPDATE OF p;

-- name: DuplicateIdentities :many
SELECT c.photo_id,c.identity::text AS identity,c.still_hash::text AS still_hash,c.live_hash::text AS live_hash,coalesce(k.identity=c.identity,false)::boolean AS kept
FROM duplicate_candidate c LEFT JOIN duplicate_keep k ON k.photo_id=c.photo_id AND k.user_id=c.user_id
WHERE c.user_id=sqlc.arg(user_id)::int AND c.group_key=sqlc.arg(group_key)::text ORDER BY c.photo_id;

-- name: SetDuplicateKeep :exec
INSERT INTO duplicate_keep(photo_id,user_id,identity) VALUES(sqlc.arg(photo_id)::int,sqlc.arg(user_id)::int,sqlc.arg(identity)::text)
ON CONFLICT(photo_id) DO UPDATE SET identity=excluded.identity WHERE duplicate_keep.user_id=excluded.user_id;

-- name: ClearDuplicateKeep :exec
DELETE FROM duplicate_keep WHERE photo_id=sqlc.arg(photo_id)::int AND user_id=sqlc.arg(user_id)::int;

-- name: SetDuplicateIgnore :exec
INSERT INTO duplicate_ignore(user_id,group_key,version) VALUES(sqlc.arg(user_id)::int,sqlc.arg(group_key)::text,sqlc.arg(version)::text)
ON CONFLICT(user_id,group_key) DO UPDATE SET version=excluded.version;

-- name: ClearDuplicateIgnore :exec
DELETE FROM duplicate_ignore WHERE user_id=sqlc.arg(user_id)::int AND group_key=sqlc.arg(group_key)::text;
