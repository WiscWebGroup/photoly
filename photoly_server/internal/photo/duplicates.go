package photo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"

	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/jackc/pgx/v5"
)

var duplicateKey = regexp.MustCompile(`^[0-9a-f]{64}:(still|[0-9a-f]{64})$`)
var duplicateVersion = regexp.MustCompile(`^[0-9a-f]{32}$`)

func duplicatePagination(r *http.Request, defaultLimit int64) (int64, int64, error) {
	limit, offset := defaultLimit, int64(0)
	var err error
	if r.URL.Query().Has("limit") {
		limit, err = strconv.ParseInt(r.URL.Query().Get("limit"), 10, 32)
		if err != nil {
			return 0, 0, Bad()
		}
	}
	if r.URL.Query().Has("offset") {
		offset, err = strconv.ParseInt(r.URL.Query().Get("offset"), 10, 32)
		if err != nil {
			return 0, 0, Bad()
		}
	}
	if limit < 1 || limit > 100 || offset < 0 {
		return 0, 0, Bad()
	}
	return limit, offset, nil
}
func (s *Service) registerDuplicates() {
	s.app.Mux.HandleFunc("GET /photo/duplicates", s.privateJSONHandler(func(r *http.Request, uid int64) (any, error) {
		limit, offset, e := duplicatePagination(r, 10)
		if e != nil {
			return nil, e
		}
		ignored := r.URL.Query().Get("ignored")
		if ignored != "" && ignored != "true" && ignored != "false" {
			return nil, Bad()
		}
		data, e := dbgen.New(s.Pool).ListDuplicateGroups(r.Context(), dbgen.ListDuplicateGroupsParams{UserID: uid, Ignored: ignored == "true", PageLimit: limit, PageOffset: offset})
		return json.RawMessage(data), e
	}))
	s.app.Mux.HandleFunc("GET /photo/duplicates/members", s.privateJSONHandler(func(r *http.Request, uid int64) (any, error) {
		limit, offset, e := duplicatePagination(r, 24)
		if e != nil {
			return nil, e
		}
		key, version := r.URL.Query().Get("key"), r.URL.Query().Get("version")
		if !duplicateKey.MatchString(key) || !duplicateVersion.MatchString(version) {
			return nil, Bad()
		}
		// One repeatable snapshot for membership, version and paging.
		tx, e := s.Pool.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
		if e != nil {
			return nil, e
		}
		defer tx.Rollback(r.Context())
		q := dbgen.New(tx)
		g, e := q.GetDuplicateGroup(r.Context(), dbgen.GetDuplicateGroupParams{UserID: uid, GroupKey: key})
		if errors.Is(e, pgx.ErrNoRows) {
			return nil, Err("duplicate_changed", 409)
		}
		if e != nil {
			return nil, e
		}
		if g.Version != version {
			return nil, Err("duplicate_changed", 409)
		}
		items, e := q.ListDuplicateMembers(r.Context(), dbgen.ListDuplicateMembersParams{UserID: uid, GroupKey: key, PageLimit: limit, PageOffset: offset})
		if e != nil {
			return nil, e
		}
		return map[string]any{"items": json.RawMessage(items), "total": g.Count, "version": g.Version}, nil
	}))
	for _, action := range []string{"keep", "ignore", "delete"} {
		s.app.Mux.HandleFunc("POST /photo/duplicates/"+action, s.privateJSONHandler(func(r *http.Request, uid int64) (any, error) {
			r.Body = http.MaxBytesReader(nil, r.Body, 65536)
			d, e := body(r)
			if e != nil {
				return nil, e
			}
			key, _ := d["key"].(string)
			version, _ := d["version"].(string)
			if !duplicateKey.MatchString(key) || !duplicateVersion.MatchString(version) {
				return nil, Bad()
			}
			e = s.changeDuplicates(r.Context(), uid, action, key, version, d)
			if e != nil {
				return nil, e
			}
			return map[string]any{"ok": true}, nil
		}))
	}
}

func (s *Service) changeDuplicates(ctx context.Context, uid int64, action, key, version string, d map[string]any) error {
	return s.Atomic(ctx, uid, func(tx pgx.Tx, u UserRow, f *Files) error {
		q := dbgen.New(tx)
		// Match the storage writer/scanner lock order: sources, then photo rows.
		sources, e := q.DuplicateSources(ctx, dbgen.DuplicateSourcesParams{UserID: uid, GroupKey: key})
		if e != nil {
			return e
		}
		for _, id := range sources {
			source, e := storage.ReadSource(ctx, tx, id)
			if e != nil {
				return e
			}
			if e = storage.LockSource(ctx, tx, source); e != nil {
				return e
			}
		}
		if _, e = q.LockDuplicatePhotos(ctx, dbgen.LockDuplicatePhotosParams{UserID: uid, GroupKey: key}); e != nil {
			return e
		}
		g, e := q.GetDuplicateGroup(ctx, dbgen.GetDuplicateGroupParams{UserID: uid, GroupKey: key})
		if errors.Is(e, pgx.ErrNoRows) {
			return Err("duplicate_changed", 409)
		}
		if e != nil {
			return e
		}
		if g.Version != version {
			return Err("duplicate_changed", 409)
		}
		if action == "ignore" {
			value, ok := d["value"].(bool)
			if !ok {
				return Bad()
			}
			if value {
				return q.SetDuplicateIgnore(ctx, dbgen.SetDuplicateIgnoreParams{UserID: uid, GroupKey: key, Version: version})
			}
			return q.ClearDuplicateIgnore(ctx, dbgen.ClearDuplicateIgnoreParams{UserID: uid, GroupKey: key})
		}
		rows, e := q.DuplicateIdentities(ctx, dbgen.DuplicateIdentitiesParams{UserID: uid, GroupKey: key})
		if e != nil {
			return e
		}
		if action == "keep" {
			id, ok := intOf(d["photoId"])
			if !ok {
				return Bad()
			}
			value, ok := d["value"].(bool)
			if !ok {
				return Bad()
			}
			for _, row := range rows {
				if row.PhotoID == id {
					if value {
						return q.SetDuplicateKeep(ctx, dbgen.SetDuplicateKeepParams{PhotoID: id, UserID: uid, Identity: row.Identity})
					}
					return q.ClearDuplicateKeep(ctx, dbgen.ClearDuplicateKeepParams{PhotoID: id, UserID: uid})
				}
			}
			return Err("duplicate_changed", 409)
		}
		ids, ok := d["photoIds"].([]any)
		if !ok || len(ids) == 0 || len(ids) > 1000 {
			return Bad()
		}
		selected := map[int64]bool{}
		for _, v := range ids {
			id, ok := intOf(v)
			if !ok || id < 1 || selected[id] {
				return Bad()
			}
			selected[id] = true
		}
		if len(selected) >= len(rows) {
			return Err("duplicate_keep_required", 400)
		}
		found := 0
		for _, row := range rows {
			if selected[row.PhotoID] {
				found++
				if row.Kept {
					return Err("duplicate_keep_required", 400)
				}
			}
		}
		if found != len(selected) {
			return Err("duplicate_changed", 409)
		}
		// Re-read file bytes, not only the index, before deleting. Confirm a survivor
		// really exists as well; an offline source is not a safe remaining copy.
		var deletedPaths []string
		for _, row := range rows {
			if !selected[row.PhotoID] {
				continue
			}
			p, e := s.OwnedPhoto(ctx, tx, row.PhotoID, uid, nil, 404)
			if e != nil {
				return e
			}
			paths, e := s.verifyDuplicateFile(ctx, tx, u, p, row.StillHash, row.LiveHash)
			if e != nil {
				return e
			}
			deletedPaths = append(deletedPaths, paths...)
		}
		survivor := false
		for _, row := range rows {
			if selected[row.PhotoID] {
				continue
			}
			p, e := s.OwnedPhoto(ctx, tx, row.PhotoID, uid, nil, 404)
			if e != nil {
				return e
			}
			// Check every unselected member for physical aliases, including kept
			// records, even after finding a readable survivor.
			roles := []string{"original"}
			if row.LiveHash != "" {
				roles = append(roles, "live_original")
			}
			for _, role := range roles {
				path, e := storage.Resolve(ctx, tx, s.Root, u.UUID, p.UUID, role)
				if e != nil {
					continue
				}
				for _, deleted := range deletedPaths {
					if storage.SameObject(path, deleted) {
						return Err("duplicate_file_changed", 400)
					}
				}
			}
			if !survivor {
				if _, e := s.verifyDuplicateFile(ctx, tx, u, p, row.StillHash, row.LiveHash); e == nil {
					survivor = true
				}
			}
		}
		if !survivor {
			return Err("duplicate_file_changed", 400)
		}
		for _, row := range rows {
			if selected[row.PhotoID] {
				p, e := s.OwnedPhoto(ctx, tx, row.PhotoID, uid, nil, 404)
				if e != nil {
					return e
				}
				if e = s.DeleteRecord(ctx, tx, f, u, p); e != nil {
					return e
				}
			}
		}
		return nil
	})
}
func (s *Service) verifyDuplicateFile(ctx context.Context, tx pgx.Tx, u UserRow, p *Photo, still, live string) ([]string, error) {
	var paths []string
	for _, asset := range []struct{ role, hash string }{{"original", still}, {"live_original", live}} {
		if asset.hash == "" {
			continue
		}
		path, e := storage.Resolve(ctx, tx, s.Root, u.UUID, p.UUID, asset.role)
		if e != nil {
			return nil, Err("duplicate_file_changed", 400)
		}
		paths = append(paths, path)
		file, e := storage.OpenFresh(ctx, path)
		if e != nil {
			return nil, Err("duplicate_file_changed", 400)
		}
		hash := sha256.New()
		_, e = io.Copy(hash, file)
		file.Close()
		if e != nil || hex.EncodeToString(hash.Sum(nil)) != asset.hash {
			return nil, Err("duplicate_file_changed", 400)
		}
		if e = ctx.Err(); e != nil {
			return nil, e
		}
	}
	return paths, nil
}
