-- name: ListMediaNotes :many
SELECT n.* FROM media_note n JOIN photo p ON p.photo_id = n.photo_id
WHERE n.photo_id = @photo_id::int AND n.user_id = @user_id::int AND p.user_id = @user_id::int ORDER BY n.note_id;

-- name: ListCategoryCounts :many
SELECT c.category,count(*)::bigint AS count FROM effective_media_category c
JOIN photo p ON p.photo_id = c.photo_id JOIN storage_source s ON s.id = p.source_id
WHERE c.user_id = @user_id::int AND s.enabled GROUP BY c.category;

-- name: ListCategoryPreviews :many
SELECT k.category::text AS category,p.photo_id,p.photo_uuid::text AS uuid,coalesce(p.photo_name,'')::text AS photo_name,coalesce(p.format,'')::text AS format,p.content_version
FROM (SELECT unnest(ARRAY['video','panorama','live','animated','screenshot','portrait']::text[]) AS category) k
CROSS JOIN LATERAL (
 SELECT p.photo_id,p.photo_uuid,p.photo_name,p.format,p.content_version FROM effective_media_category c
 JOIN photo p ON p.photo_id=c.photo_id JOIN storage_source s ON s.id=p.source_id
 WHERE c.user_id= @user_id::int AND s.enabled AND c.category=k.category
 ORDER BY p.photo_id DESC LIMIT 3
) p;

-- name: ClaimVideoThumbnail :one
WITH candidate AS (
 SELECT j.photo_id FROM video_thumbnail_job j JOIN photo p USING(photo_id) JOIN storage_source s ON s.id=p.source_id
 WHERE j.state IN ('pending','processing') AND j.next_attempt<=now() AND s.enabled
 ORDER BY j.next_attempt,j.photo_id FOR UPDATE OF j SKIP LOCKED LIMIT 1
)
UPDATE video_thumbnail_job j SET state='processing',lease_token= @lease_token::text,next_attempt=now()+interval '3 minutes'
FROM candidate k,photo p WHERE j.photo_id=k.photo_id AND p.photo_id=j.photo_id
RETURNING j.photo_id,j.content_version,coalesce(p.user_id,0)::int AS user_id;
