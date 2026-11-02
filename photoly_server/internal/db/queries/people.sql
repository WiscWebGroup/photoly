-- name: PeopleProgress :one
SELECT count(*)::bigint AS total,
 count(*) FILTER(WHERE m.status='ready')::bigint AS ready,
 count(*) FILTER(WHERE m.status='failed')::bigint AS failed,
 count(*) FILTER(WHERE m.status='processing')::bigint AS processing
FROM photo ph LEFT JOIN people_media m ON m.photo_id=ph.photo_id AND m.content_version=ph.content_version AND m.fingerprint= @fingerprint::text
WHERE ph.user_id= @user_id::int AND photo_visible(ph.storage_id,ph.user_id) AND lower(ph.format)=ANY(@formats::text[]);

-- name: ListPeoplePhotos :many
SELECT ph.photo_id,coalesce(ph.photo_name,'')::text AS photo_name,coalesce(ph.format,'')::text AS format,
 coalesce(ph.ns_id,0)::bigint AS ns_id,ph.source_id
FROM photo ph
WHERE ph.user_id= @user_id::int AND ph.photo_id> @after_id::int AND photo_visible(ph.storage_id,ph.user_id)
AND EXISTS(SELECT 1 FROM people_face f WHERE f.photo_id=ph.photo_id AND f.person_id= @person_id::bigint AND NOT f.dismissed)
ORDER BY ph.photo_id LIMIT @page_size::int;
