-- name: ListNamespacePins :many
WITH RECURSIVE paths AS (
  SELECT n.ns_id AS origin, n.ns_parent_id, coalesce(n.ns_name, '')::text AS path, ARRAY[n.ns_id] AS visited
  FROM namespace_pin p JOIN namespace n USING(ns_id)
  WHERE p.user_id = @user_id::int AND n.user_id = @user_id::int
  UNION ALL
  SELECT p.origin, n.ns_parent_id, coalesce(n.ns_name, '') || ' / ' || p.path, p.visited || n.ns_id
  FROM paths p JOIN namespace n ON n.ns_id = p.ns_parent_id
  WHERE n.user_id = @user_id::int AND NOT n.ns_id = ANY(p.visited)
)
SELECT sqlc.embed(n), p.is_default,
  (SELECT path FROM paths WHERE origin = p.ns_id ORDER BY cardinality(visited) DESC LIMIT 1)::text AS path
FROM namespace_pin p JOIN namespace n USING(ns_id)
WHERE p.user_id = @user_id::int AND n.user_id = @user_id::int
ORDER BY p.position;

-- name: PinNamespace :exec
INSERT INTO namespace_pin(user_id, ns_id) VALUES (@user_id::int, @ns_id::int)
ON CONFLICT (user_id, ns_id) DO NOTHING;

-- name: UnpinNamespace :exec
DELETE FROM namespace_pin WHERE user_id = @user_id::int AND ns_id = @ns_id::int;

-- name: ClearDefaultNamespace :exec
UPDATE namespace_pin SET is_default = false WHERE user_id = @user_id::int AND is_default;

-- name: SetDefaultNamespace :execrows
UPDATE namespace_pin SET is_default = true WHERE user_id = @user_id::int AND ns_id = @ns_id::int;
