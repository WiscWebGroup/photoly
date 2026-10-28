-- name: GetUploadBatch :one
SELECT * FROM upload_batch WHERE id = @id::text;

-- name: GetUploadBatchOwner :one
SELECT user_id FROM upload_batch WHERE id = @id::text;

-- name: GetUploadItem :one
SELECT * FROM upload_item WHERE id = @id::text;

-- name: InsertUploadBatch :exec
INSERT INTO upload_batch (id, user_id, ns_id, name, created_at, updated_at, folders)
VALUES (@id::text, @user_id::int, @ns_id::int, @name::text, @now::timestamptz, @now::timestamptz, '{}');

-- A sealed batch holding the single item of a convert / online download / live preview job.
-- name: InsertSealedUploadBatch :exec
INSERT INTO upload_batch (id, user_id, ns_id, name, sealed, cancelled, created_at, updated_at, folders, dismissed)
VALUES (@id::text, @user_id::int, @ns_id::int, @name::text, true, false, @now::timestamptz, @now::timestamptz, '{}', false);

-- name: InsertUploadItem :exec
INSERT INTO upload_item (id, batch_id, path, size, state, error, updated_at)
VALUES (@id::text, @batch_id::text, @path::text, @size::bigint, @state::text, sqlc.narg(error)::text, @updated_at::timestamptz);

-- name: InsertExpandedUploadItem :exec
INSERT INTO upload_item (id, batch_id, path, size, state, error, source_id, entry_index, updated_at)
VALUES (@id::text, @batch_id::text, @path::text, @size::bigint, @state::text, sqlc.narg(error)::text, @source_id::text, @entry_index::int, @updated_at::timestamptz);

-- name: InsertMediaUploadItem :exec
INSERT INTO upload_item (id, batch_id, path, size, state, kind, options, updated_at)
VALUES (@id::text, @batch_id::text, @path::text, 0, 'ready', @kind::text, @options::jsonb, @updated_at::timestamptz);

-- name: GetFirstUploadItemOfBatch :one
SELECT kind, options FROM upload_item WHERE batch_id = @batch_id::text LIMIT 1;

-- name: GetUploadItemForProcess :one
SELECT i.batch_id, i.kind, i.options, b.user_id
FROM upload_item i JOIN upload_batch b ON b.id = i.batch_id
WHERE i.id = @id::text;

-- name: GetUploadItemStateForUser :one
SELECT i.state, b.cancelled, b.ns_id
FROM upload_item i JOIN upload_batch b ON b.id = i.batch_id
WHERE i.id = @id::text AND b.user_id = @user_id::int;

-- name: TouchUploadBatch :exec
UPDATE upload_batch SET updated_at = @updated_at::timestamptz WHERE id = @id::text;

-- name: SealUploadBatch :exec
UPDATE upload_batch SET sealed = true, updated_at = @updated_at::timestamptz WHERE id = @id::text;

-- name: CancelUploadBatch :exec
UPDATE upload_batch SET cancelled = true, sealed = true, updated_at = @updated_at::timestamptz WHERE id = @id::text;

-- name: ReopenUploadBatch :exec
UPDATE upload_batch SET cancelled = false, dismissed = false, updated_at = @updated_at::timestamptz WHERE id = @id::text;

-- name: SetUploadBatchFolders :exec
UPDATE upload_batch SET folders = @folders::json WHERE id = @id::text;

-- name: AcknowledgeUploadFailures :exec
UPDATE upload_batch SET failures_acknowledged_at = @at::timestamptz WHERE user_id = @user_id::int AND NOT dismissed;

-- name: DismissUploadBatches :exec
UPDATE upload_batch SET dismissed = true WHERE id = ANY(@ids::text[]);

-- Sealed batches whose items are all settled; one batch when only_id is given.
-- name: ListFinishedUploadBatchIDs :many
SELECT b.id FROM upload_batch b
WHERE b.user_id = @user_id::int AND NOT b.dismissed AND b.sealed
  AND NOT EXISTS (SELECT 1 FROM upload_item i WHERE i.batch_id = b.id AND i.state NOT IN ('done','failed','skipped','cancelled','expanded'))
  AND (sqlc.narg(only_id)::text IS NULL OR b.id = sqlc.narg(only_id)::text);

-- name: CountUploadBatches :one
SELECT count(*) FROM upload_batch WHERE user_id = @user_id::int AND NOT dismissed;

-- name: ListUploadBatches :many
SELECT * FROM upload_batch WHERE user_id = @user_id::int AND NOT dismissed
ORDER BY created_at DESC, id OFFSET @skip::bigint LIMIT 20;

-- name: CountUploadItems :one
SELECT count(*) FROM upload_item WHERE batch_id = @batch_id::text;

-- name: ListUploadItems :many
SELECT * FROM upload_item WHERE batch_id = @batch_id::text ORDER BY path, id OFFSET @skip::bigint LIMIT 50;

-- sum(bigint) is numeric: Flask emits Decimal as a JSON string, so bytes stays text.
-- name: CountUploadItemsByState :many
SELECT batch_id, state, count(*) AS n, coalesce(sum(size), 0)::text AS bytes
FROM upload_item WHERE batch_id = ANY(@batch_ids::text[]) GROUP BY batch_id, state;

-- name: CountActiveUploadItems :one
SELECT count(*) FROM upload_item i JOIN upload_batch b ON b.id = i.batch_id
WHERE b.user_id = @user_id::int AND i.state IN ('pending','ready','processing','waiting');

-- name: CountUnseenFailedUploadItems :one
SELECT count(*) FROM upload_item i JOIN upload_batch b ON b.id = i.batch_id
WHERE b.user_id = @user_id::int AND NOT b.dismissed AND i.state = 'failed'
  AND (b.failures_acknowledged_at IS NULL OR i.updated_at > b.failures_acknowledged_at);

-- name: SetUploadItemState :exec
UPDATE upload_item SET state = @state::text, error = sqlc.narg(error)::text, updated_at = @updated_at::timestamptz WHERE id = @id::text;

-- name: FailProcessingUploadItem :exec
UPDATE upload_item SET state = 'failed', error = @error::text, updated_at = @updated_at::timestamptz
WHERE id = @id::text AND state = 'processing';

-- name: MarkUploadItemReceived :exec
UPDATE upload_item SET content_sha256 = @sha::text, state = 'ready', error = NULL, updated_at = @updated_at::timestamptz WHERE id = @id::text;

-- name: StartUploadItem :exec
UPDATE upload_item SET state = 'processing', updated_at = @updated_at::timestamptz WHERE id = @id::text;

-- name: TouchUploadItem :exec
UPDATE upload_item SET updated_at = @updated_at::timestamptz WHERE id = @id::text;

-- name: WaitUploadItem :exec
UPDATE upload_item SET state = 'waiting', updated_at = @updated_at::timestamptz WHERE id = @id::text AND state = 'processing';

-- name: MarkUploadItemExpanded :exec
UPDATE upload_item SET state = 'expanded', error = NULL WHERE id = @id::text;

-- name: MarkUploadItemDone :exec
UPDATE upload_item SET photo_id = @photo_id::int, ns_id = @ns_id::int, state = 'done', error = NULL, updated_at = @updated_at::timestamptz
WHERE id = @id::text;

-- name: MarkMediaUploadItemDone :exec
UPDATE upload_item SET photo_id = @photo_id::int, ns_id = @ns_id::int, size = @size::bigint, state = 'done', error = NULL, updated_at = @updated_at::timestamptz
WHERE id = @id::text;

-- name: SetUploadItemMediaMetadata :exec
UPDATE upload_item SET media_metadata = @media_metadata::jsonb WHERE id = @id::text;

-- name: PairUploadItem :exec
UPDATE upload_item SET pair_id = @pair_id::text, state = 'processing', updated_at = @updated_at::timestamptz WHERE id = @id::text;

-- name: SetUploadItemPair :exec
UPDATE upload_item SET pair_id = @pair_id::text WHERE id = @id::text;

-- name: MarkPairedUploadItemDone :exec
UPDATE upload_item SET pair_id = @pair_id::text, photo_id = @photo_id::int, ns_id = @ns_id::int, state = 'done', error = NULL, updated_at = @updated_at::timestamptz
WHERE id = @id::text;

-- name: CancelOpenUploadItems :exec
UPDATE upload_item SET state = 'cancelled', updated_at = @updated_at::timestamptz
WHERE batch_id = @batch_id::text AND state IN ('pending','ready','processing','waiting');

-- name: ListRetryableUploadItems :many
SELECT * FROM upload_item WHERE batch_id = @batch_id::text AND state IN ('failed','cancelled');

-- name: ListUploadSiblings :many
SELECT * FROM upload_item WHERE batch_id = @batch_id::text AND kind = 'upload';

-- name: ListRunnableUploadItems :many
SELECT id, batch_id FROM upload_item WHERE state IN ('ready','processing','waiting')
ORDER BY (state = 'waiting'), updated_at, id LIMIT 100;

-- name: ListStaleUploadBatches :many
SELECT id, user_id FROM upload_batch WHERE updated_at < @cutoff::timestamptz;

-- name: CountInflightUploadItems :one
SELECT count(*) FROM upload_item WHERE batch_id = @batch_id::text AND state IN ('ready','processing');

-- name: ExpireUntransmittedUploadItems :exec
UPDATE upload_item SET state = 'failed', error = 'transmission_expired', updated_at = @updated_at::timestamptz
WHERE batch_id = @batch_id::text AND state = 'pending';

-- name: SealUploadBatchQuietly :exec
UPDATE upload_batch SET sealed = true WHERE id = @id::text;

-- name: ResumeWaitingUploadItems :execrows
UPDATE upload_item SET state = 'ready', updated_at = @updated_at::timestamptz WHERE batch_id = @batch_id::text AND state = 'waiting';

-- name: ListUploadSourceIDs :many
SELECT id FROM upload_item WHERE batch_id = @batch_id::text AND source_id IS NULL;

-- name: DeleteBatchUploadItems :exec
DELETE FROM upload_item WHERE batch_id = @batch_id::text;

-- name: DeleteUploadBatch :exec
DELETE FROM upload_batch WHERE id = @id::text;

-- name: UploadItemExists :one
SELECT EXISTS (SELECT 1 FROM upload_item WHERE id = @id::text)::boolean;

-- name: NamespaceNameTaken :one
SELECT EXISTS (SELECT 1 FROM namespace WHERE user_id = @user_id::int AND ns_parent_id = @parent_id::int AND ns_name = @ns_name::text)::boolean;

-- name: CreateFolder :one
INSERT INTO namespace (ns_name, ns_parent_id, user_id) VALUES (@ns_name::text, @parent_id::int, @user_id::int) RETURNING ns_id;

-- name: TryBatchLock :one
SELECT pg_try_advisory_lock(hashtextextended(@batch::text, @session_lock::bigint))::boolean;

-- name: BatchUnlock :exec
SELECT pg_advisory_unlock(hashtextextended(@batch::text, @session_lock::bigint));
