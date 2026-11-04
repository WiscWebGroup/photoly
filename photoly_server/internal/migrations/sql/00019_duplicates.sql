-- +goose Up
CREATE TABLE duplicate_keep (
 photo_id integer PRIMARY KEY REFERENCES photo ON DELETE CASCADE,
 user_id integer NOT NULL REFERENCES "user" ON DELETE CASCADE,
 identity text NOT NULL
);
CREATE TABLE duplicate_ignore (
 user_id integer NOT NULL REFERENCES "user" ON DELETE CASCADE,
 group_key text NOT NULL,
 version text NOT NULL,
 PRIMARY KEY(user_id,group_key)
);
CREATE INDEX photo_duplicate_hash ON photo(user_id,content_sha256) WHERE content_sha256 <> '' AND missing_scans=0;

-- Pair identity deliberately distinguishes Live Photos from ordinary stills.
CREATE VIEW duplicate_candidate AS
SELECT p.photo_id,p.user_id,p.source_id,p.ns_id,p.file_name,p.photo_name,p.format,p.content_version,
 p.fs_size + CASE WHEN coalesce(m.live,false) THEN a.fs_size ELSE 0 END AS bytes,
 p.content_sha256::text AS still_hash,
 CASE WHEN coalesce(m.live,false) THEN a.content_sha256 ELSE '' END AS live_hash,
 p.content_sha256 || ':' || CASE WHEN coalesce(m.live,false) THEN a.content_sha256 ELSE 'still' END AS group_key,
 p.content_sha256 || ':' || p.content_version::text || ':' || CASE WHEN coalesce(m.live,false) THEN a.content_sha256 || ':' || a.content_version::text ELSE 'still' END AS identity
FROM photo p
LEFT JOIN photo_media m USING(photo_id)
LEFT JOIN media_asset a ON a.photo_id=p.photo_id AND a.role='live_original'
JOIN storage_source s ON s.id=p.source_id
WHERE p.content_sha256 ~ '^[0-9a-f]{64}$' AND p.missing_scans=0 AND s.enabled
 AND photo_visible(p.storage_id,p.user_id)
 AND lower(p.format) IN ('jpg','jpeg','gif','png','webp','ico','bmp','tif','tiff','svg','psd','raw','arw','pcd','jfif','heic','heif')
 AND (NOT coalesce(m.live,false) OR (a.content_sha256 ~ '^[0-9a-f]{64}$' AND a.missing_scans=0));
CREATE VIEW duplicate_group AS
SELECT c.user_id,c.group_key,count(*) AS count,
 md5(string_agg(c.photo_id::text || ':' || c.identity, ',' ORDER BY c.photo_id)) AS version,
 count(k.photo_id) AS kept_count
FROM duplicate_candidate c LEFT JOIN duplicate_keep k ON k.photo_id=c.photo_id AND k.user_id=c.user_id AND k.identity=c.identity
GROUP BY c.user_id,c.group_key HAVING count(*)>1;

-- +goose Down
DROP VIEW duplicate_group;
DROP VIEW duplicate_candidate;
DROP INDEX photo_duplicate_hash;
DROP TABLE duplicate_ignore;
DROP TABLE duplicate_keep;
