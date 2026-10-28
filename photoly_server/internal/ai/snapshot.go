package ai

// snapshot_photo (app/ai/snapshot.py): read one photo's bytes for embedding, pinned to a
// content version, without holding the owner's lock while decoding.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func mediaPath(root, userUUID, photoUUID, format string) (string, error) {
	name := photoUUID + "." + format
	for _, part := range []string{userUUID, name} {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "/\\\x00") {
			return "", fail("image_missing", 404)
		}
	}
	return filepath.Join(root, userUUID, name), nil
}

func readBounded(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxImageBytes {
		return nil, fail("image_too_large", 413)
	}
	if len(data) == 0 {
		return nil, fail("empty_image")
	}
	return data, nil
}

func sha256hex(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// snapshotPhoto: userID restricts to an owner (similar search); version pins the content.
func snapshotPhoto(ctx context.Context, pool *pgxpool.Pool, root string, photoID int64, userID *int64, version *int32) (*Snapshot, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var owner, cv int32
	var ownerUUID, photoUUID, format string
	var sha *string
	err = tx.QueryRow(ctx, `SELECT user_id, content_version, content_sha256, COALESCE(photo_uuid,''), COALESCE(format,'') FROM photo WHERE photo_id=$1`, photoID).
		Scan(&owner, &cv, &sha, &photoUUID, &format)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && userID != nil && int64(owner) != *userID) {
		return nil, fail("photo_not_found", 404)
	}
	if err != nil {
		return nil, err
	}
	var enabled bool
	if err = tx.QueryRow(ctx, `SELECT ai_search_enabled, uuid FROM "user" WHERE user_id=$1 FOR UPDATE`, owner).Scan(&enabled, &ownerUUID); err != nil {
		return nil, fail("stale_content", 409)
	}
	if err = tx.QueryRow(ctx, `SELECT content_version, content_sha256 FROM photo WHERE photo_id=$1`, photoID).Scan(&cv, &sha); err != nil {
		return nil, fail("stale_content", 409)
	}
	if !enabled || (version != nil && cv != *version) {
		return nil, fail("stale_content", 409)
	}
	path, err := mediaPath(root, ownerUUID, photoUUID, format)
	if err != nil {
		return nil, err
	}
	storage.Fetch(path)
	f, err := os.Open(path)
	if err != nil {
		return nil, fail("image_missing", 404)
	}
	defer f.Close()
	// Writers replace immutable inodes while holding the same user lock; the open
	// descriptor stays valid across a later rename or delete.
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	snap := &Snapshot{UserID: int64(owner), Version: cv}
	expected := ""
	if sha != nil {
		expected = *sha
	}
	if isVideo(format) {
		snap.Hash, snap.Frames, err = snapshotVideo(ctx, f, expected)
		return snap, err
	}
	data, err := readBounded(f)
	if err != nil {
		return nil, err
	}
	snap.Data, snap.Hash = data, sha256hex(data)
	if expected != "" && snap.Hash != expected {
		return nil, fail("content_hash_mismatch", 409)
	}
	return snap, nil
}
