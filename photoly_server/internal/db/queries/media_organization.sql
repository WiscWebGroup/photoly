-- name: ListMediaNotes :many
SELECT n.* FROM media_note n JOIN photo p ON p.photo_id = n.photo_id
WHERE n.photo_id = @photo_id::int AND n.user_id = @user_id::int AND p.user_id = @user_id::int ORDER BY n.note_id;

-- name: ListCategoryCounts :many
SELECT c.category,count(*)::bigint AS count FROM effective_media_category c
JOIN photo p ON p.photo_id = c.photo_id JOIN storage_source s ON s.id = p.source_id
WHERE c.user_id = @user_id::int AND s.enabled GROUP BY c.category;
