package ai

// Smart Tag: opt-in organization of a user's library into camera / lens tags and trip
// galleries (services/smart.py). The scheduler half is in smart_sched.go.

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Smart struct {
	pool *pgxpool.Pool
	root string
	gaz  *gazetteer
}

type smartPref struct {
	UserID              int64
	Enabled             bool
	Revision, Completed int64
	ProfileID           *int64
	Checkpoint          map[string]any
	Reason              *string
	UpdatedAt           *time.Time
}

const prefCols = `user_id, enabled, revision, completed_revision, profile_id, checkpoint, reason, updated_at`

func (p *smartPref) scan(r pgx.Row) error {
	return r.Scan(&p.UserID, &p.Enabled, &p.Revision, &p.Completed, &p.ProfileID, &p.Checkpoint, &p.Reason, &p.UpdatedAt)
}

// preference returns (creating if needed) a user's smart_preference row.
func preference(ctx context.Context, q Q, uid int64) (*smartPref, error) {
	if _, err := q.Exec(ctx, `INSERT INTO smart_preference (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, uid); err != nil {
		return nil, err
	}
	p := &smartPref{}
	err := p.scan(q.QueryRow(ctx, `SELECT `+prefCols+` FROM smart_preference WHERE user_id=$1`, uid))
	if p.Checkpoint == nil {
		p.Checkpoint = map[string]any{}
	}
	return p, err
}

func (p *smartPref) save(ctx context.Context, q Q) error {
	_, err := q.Exec(ctx, `UPDATE smart_preference SET enabled=$2, revision=$3, completed_revision=$4, profile_id=$5, checkpoint=$6, reason=$7, updated_at=$8 WHERE user_id=$1`,
		p.UserID, p.Enabled, p.Revision, p.Completed, p.ProfileID, p.Checkpoint, p.Reason, p.UpdatedAt)
	return err
}

func dirty(ctx context.Context, q Q, uid int64) error {
	_, err := q.Exec(ctx, `UPDATE smart_preference SET revision=revision+1 WHERE user_id=$1`, uid)
	return err
}

// eligibility says why a user's smart organization cannot run ("" when it can).
func eligibility(ctx context.Context, q Q, uid int64, scheduling bool) (string, error) {
	cfg, err := loadSettings(ctx, q)
	if err != nil {
		return "", err
	}
	var enabled bool
	err = q.QueryRow(ctx, `SELECT ai_search_enabled FROM "user" WHERE user_id=$1`, uid).Scan(&enabled)
	userOK := err == nil && enabled
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	var prefOn bool
	err = q.QueryRow(ctx, `SELECT enabled FROM smart_preference WHERE user_id=$1`, uid).Scan(&prefOn)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	switch {
	case !cfg.Bool("enabled"):
		return "ai_disabled", nil
	case !cfg.Bool("smart_tag_enabled"):
		return "smart_tag_disabled", nil
	case !userOK:
		return "ai_permission_required", nil
	case !prefOn:
		return "user_disabled", nil
	}
	if scheduling {
		if cfg.Bool("maintenance") {
			return "maintenance", nil
		}
		if cfg.Str("scheduler") != "running" {
			return "scheduler_" + cfg.Str("scheduler"), nil
		}
	}
	return "", nil
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s *Smart) lockUser(ctx context.Context, q Q, uid int64) error {
	var id int64
	err := q.QueryRow(ctx, `SELECT user_id FROM "user" WHERE user_id=$1 FOR UPDATE`, uid).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

func (s *Smart) settings(ctx context.Context, q Q, uid int64) (map[string]any, error) {
	var on bool
	err := q.QueryRow(ctx, `SELECT enabled FROM smart_preference WHERE user_id=$1`, uid).Scan(&on)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	cfg, err := loadSettings(ctx, q)
	if err != nil {
		return nil, err
	}
	var userOn bool
	_ = q.QueryRow(ctx, `SELECT ai_search_enabled FROM "user" WHERE user_id=$1`, uid).Scan(&userOn)
	reason, err := eligibility(ctx, q, uid, false)
	if err != nil {
		return nil, err
	}
	return map[string]any{"enabled": on, "available": cfg.Bool("enabled") && cfg.Bool("smart_tag_enabled") && userOn,
		"reason": nilIfEmpty(reason), "intervalSeconds": 60}, nil
}

func (s *Smart) GetSettings(ctx context.Context, uid int64) (map[string]any, error) {
	return s.settings(ctx, s.pool, uid)
}

func (s *Smart) PatchSettings(ctx context.Context, uid int64, data map[string]any) (map[string]any, error) {
	on, ok := data["enabled"].(bool)
	if len(data) != 1 || !ok {
		return nil, fail("invalid_settings")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := s.lockUser(ctx, tx, uid); err != nil {
		return nil, err
	}
	pref, err := preference(ctx, tx, uid)
	if err != nil {
		return nil, err
	}
	if pref.Enabled != on {
		pref.Enabled = on
		pref.Revision++
		if err := pref.save(ctx, tx); err != nil {
			return nil, err
		}
	}
	out, err := s.settings(ctx, tx, uid)
	if err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

// Status is the per-user progress (uid != nil) or the admin summary (uid == nil).
func (s *Smart) status(ctx context.Context, uid *int64) (map[string]any, error) {
	where := func(col string) (string, []any) {
		if uid == nil {
			return "", nil
		}
		return " AND " + col + "=$1", []any{*uid}
	}
	count := func(sql, col string, extra string) (int64, error) {
		w, args := where(col)
		var n int64
		err := s.pool.QueryRow(ctx, sql+extra+w, args...).Scan(&n)
		return n, err
	}
	processed, err := count(`SELECT count(*) FROM smart_media WHERE true`, "user_id", "")
	if err != nil {
		return nil, err
	}
	total, err := count(`SELECT count(*) FROM photo WHERE true`, "user_id", "")
	if err != nil {
		return nil, err
	}
	failures, err := count(`SELECT count(*) FROM smart_media WHERE error IS NOT NULL AND attempts>=3`, "user_id", "")
	if err != nil {
		return nil, err
	}
	failedJobs, err := count(`SELECT count(*) FROM smart_preference WHERE reason='smart_processing_failed'`, "user_id", "")
	if err != nil {
		return nil, err
	}
	out := map[string]any{"processed": processed, "total": total, "failed": failures + failedJobs}
	if uid != nil {
		pref := &smartPref{}
		perr := pref.scan(s.pool.QueryRow(ctx, `SELECT `+prefCols+` FROM smart_preference WHERE user_id=$1`, *uid))
		if perr != nil && !errors.Is(perr, pgx.ErrNoRows) {
			return nil, perr
		}
		have := perr == nil
		reason, err := eligibility(ctx, s.pool, *uid, true)
		if err != nil {
			return nil, err
		}
		var r any = nilIfEmpty(reason)
		if r == nil {
			row, err := control(ctx, s.pool, "smart", false)
			if err != nil {
				return nil, err
			}
			r = row.Observed["reason"]
		}
		if r == nil && have && pref.Reason != nil {
			r = *pref.Reason
		}
		out["reason"], out["phase"], out["pending"], out["updatedAt"] = r, nil, have && pref.Revision > pref.Completed, nil
		if have {
			out["phase"] = pref.Checkpoint["phase"]
			out["updatedAt"] = isoPtr(pref.UpdatedAt)
		}
		return out, nil
	}
	row, err := control(ctx, s.pool, "smart", false)
	if err != nil {
		return nil, err
	}
	for k, v := range row.Observed {
		out[k] = v
	}
	if out["reason"] == nil {
		var waiting bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM smart_preference p JOIN "user" u ON u.user_id=p.user_id
			WHERE p.enabled AND u.ai_search_enabled AND p.reason='waiting_for_vectors')`).Scan(&waiting); err != nil {
			return nil, err
		}
		if waiting {
			out["reason"] = "waiting_for_vectors"
		}
	}
	cfg, err := loadSettings(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	switch {
	case !cfg.Bool("enabled"):
		out["reason"] = "ai_disabled"
	case !cfg.Bool("smart_tag_enabled"):
		out["reason"] = "smart_tag_disabled"
	case cfg.Bool("maintenance"):
		out["reason"] = "maintenance"
	case cfg.Str("scheduler") != "running":
		out["reason"] = "scheduler_" + cfg.Str("scheduler")
	}
	return out, nil
}

func (s *Smart) Status(ctx context.Context, uid int64) (map[string]any, error) {
	return s.status(ctx, &uid)
}
func (s *Smart) AdminStatus(ctx context.Context) (map[string]any, error) { return s.status(ctx, nil) }

func (s *Smart) refresh(ctx context.Context, q Q, uid int64) error {
	if err := s.lockUser(ctx, q, uid); err != nil {
		return err
	}
	if _, err := preference(ctx, q, uid); err != nil {
		return err
	}
	if err := dirty(ctx, q, uid); err != nil {
		return err
	}
	_, err := q.Exec(ctx, `UPDATE smart_media SET attempts=0 WHERE user_id=$1 AND error IS NOT NULL`, uid)
	return err
}

func (s *Smart) Refresh(ctx context.Context, uid int64) (map[string]any, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := s.refresh(ctx, tx, uid); err != nil {
		return nil, err
	}
	return map[string]any{"queued": true}, tx.Commit(ctx)
}

func itemDTO(id int64, source, name string, custom *string, suppressed bool, basis map[string]any) map[string]any {
	if custom != nil && *custom != "" {
		name = *custom
	}
	return map[string]any{"id": id, "source": source, "name": name, "suppressed": suppressed, "basis": basis}
}

func (s *Smart) Dismissed(ctx context.Context, uid, after int64) (map[string]any, error) {
	if after < 0 {
		return nil, fail("invalid_pagination")
	}
	rows, err := s.pool.Query(ctx, `SELECT item_id, source, name, custom_name, suppressed, basis FROM smart_item
		WHERE user_id=$1 AND suppressed AND item_id>$2 ORDER BY item_id LIMIT 50`, uid, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	var last int64
	for rows.Next() {
		var id int64
		var source, name string
		var custom *string
		var sup bool
		var basis map[string]any
		if err := rows.Scan(&id, &source, &name, &custom, &sup, &basis); err != nil {
			return nil, err
		}
		items = append(items, itemDTO(id, source, name, custom, sup, basis))
		last = id
	}
	var next any
	if len(items) == 50 {
		next = last
	}
	return map[string]any{"items": items, "nextAfter": next}, rows.Err()
}

func (s *Smart) Restore(ctx context.Context, uid, key int64) (map[string]any, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := s.lockUser(ctx, tx, uid); err != nil {
		return nil, err
	}
	var owner int64
	err = tx.QueryRow(ctx, `SELECT user_id FROM smart_item WHERE item_id=$1`, key).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && owner != uid) {
		return nil, fail("smart_item_not_found", 404)
	}
	if err != nil {
		return nil, err
	}
	// A dismissal inherited through a split/merge is restored as one lineage.
	rows, err := tx.Query(ctx, `WITH RECURSIVE family(id) AS (
		SELECT $1::integer UNION SELECT CASE WHEN l.parent_id=f.id THEN l.child_id ELSE l.parent_id END
		FROM smart_lineage l JOIN family f ON l.parent_id=f.id OR l.child_id=f.id) SELECT id FROM family`, key)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if _, err := tx.Exec(ctx, `UPDATE smart_item SET suppressed=false WHERE user_id=$1 AND item_id=ANY($2)`, uid, ids); err != nil {
		return nil, err
	}
	for _, id := range ids {
		it, err := loadItem(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		if it == nil || it.UserID != uid {
			continue
		}
		visible := it.Source != "trip" || it.Basis["eligible"] == true
		if err := ensureCatalog(ctx, tx, it, visible); err != nil {
			return nil, err
		}
		target := it.TagID
		link, col, withUser := "tag_photo", "tag_id", true
		if it.Source == "trip" {
			target, link, col, withUser = it.GaID, "gallery_photo", "ga_id", false
		}
		if target != nil {
			cols, sel := "photo_id, "+col, "m.photo_id, $2::integer"
			if withUser {
				cols, sel = cols+", user_id", sel+", $3::integer"
			}
			args := []any{id, *target, uid}
			if _, err := tx.Exec(ctx, `INSERT INTO `+link+` (`+cols+`) SELECT `+sel+` FROM smart_member m JOIN photo p ON p.photo_id=m.photo_id
				WHERE m.item_id=$1 AND p.user_id=$3 AND (m.manual=1 OR (m.manual=0 AND m.automatic AND m.content_version=p.content_version))
				ON CONFLICT DO NOTHING`, args...); err != nil {
				return nil, err
			}
		}
	}
	if err := s.refresh(ctx, tx, uid); err != nil {
		return nil, err
	}
	return map[string]any{"restored": true}, tx.Commit(ctx)
}

// ---- items and catalog links ----

type smartItem struct {
	ID, UserID              int64
	StableKey, Source, Name string
	CustomName              *string
	TagID, GaID             *int64
	Cover                   map[string]any
	Suppressed              bool
	Run                     *string
	Basis                   map[string]any
}

const itemCols = `item_id, user_id, stable_key, source, name, custom_name, tag_id, ga_id, cover, suppressed, run, basis`

func (i *smartItem) scan(r pgx.Row) error {
	return r.Scan(&i.ID, &i.UserID, &i.StableKey, &i.Source, &i.Name, &i.CustomName, &i.TagID, &i.GaID, &i.Cover, &i.Suppressed, &i.Run, &i.Basis)
}

func loadItem(ctx context.Context, q Q, id int64) (*smartItem, error) {
	i := &smartItem{}
	err := i.scan(q.QueryRow(ctx, `SELECT `+itemCols+` FROM smart_item WHERE item_id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if i.Basis == nil {
		i.Basis = map[string]any{}
	}
	return i, err
}

func (i *smartItem) save(ctx context.Context, q Q) error {
	_, err := q.Exec(ctx, `UPDATE smart_item SET name=$2, custom_name=$3, tag_id=$4, ga_id=$5, cover=$6, suppressed=$7, run=$8, basis=$9 WHERE item_id=$1`,
		i.ID, i.Name, i.CustomName, i.TagID, i.GaID, i.Cover, i.Suppressed, i.Run, i.Basis)
	return err
}

func (i *smartItem) shown() string {
	if i.CustomName != nil && *i.CustomName != "" {
		return *i.CustomName
	}
	return i.Name
}

// ensureCatalog creates, renames or deletes the user-visible tag / gallery behind an item.
func ensureCatalog(ctx context.Context, q Q, it *smartItem, visible bool) error {
	trip := it.Source == "trip"
	key := it.TagID
	table, idCol := "tag", "tag_id"
	if trip {
		key, table, idCol = it.GaID, "gallery", "ga_id"
	}
	if it.Suppressed || !visible {
		if key != nil {
			if _, err := q.Exec(ctx, `DELETE FROM `+table+` WHERE `+idCol+`=$1`, *key); err != nil {
				return err
			}
			it.GaID, it.TagID = nil, nil
			return it.save(ctx, q)
		}
		return nil
	}
	exists := false
	if key != nil {
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM `+table+` WHERE `+idCol+`=$1)`, *key).Scan(&exists); err != nil {
			return err
		}
	}
	if !exists {
		var id int64
		if trip {
			coverID, color := any(3), any("#0d9488")
			if it.Cover != nil {
				coverID, color = it.Cover["cover_id"], it.Cover["cover_color"]
			}
			if err := q.QueryRow(ctx, `INSERT INTO gallery (user_id, ga_name, create_date, cover_id, cover_color) VALUES ($1,$2,(now() at time zone 'utc'),$3,$4) RETURNING ga_id`,
				it.UserID, it.shown(), coverID, color).Scan(&id); err != nil {
				return err
			}
			it.GaID = &id
		} else {
			if err := q.QueryRow(ctx, `INSERT INTO tag (user_id, tag_name) VALUES ($1,$2) RETURNING tag_id`, it.UserID, it.shown()).Scan(&id); err != nil {
				return err
			}
			it.TagID = &id
		}
		return it.save(ctx, q)
	}
	if it.CustomName == nil || *it.CustomName == "" {
		col := "tag_name"
		if trip {
			col = "ga_name"
		}
		_, err := q.Exec(ctx, `UPDATE `+table+` SET `+col+`=$2 WHERE `+idCol+`=$1`, *key, it.Name)
		return err
	}
	return nil
}

type smartMember struct {
	ItemID, PhotoID int64
	Version         int32
	Automatic       bool
	Manual          int16
	Anchor          bool
	Run             *string
}

// syncLink makes the tag / gallery membership match a member's state.
func syncLink(ctx context.Context, q Q, it *smartItem, m *smartMember) error {
	table, col, key := "tag_photo", "tag_id", it.TagID
	if it.Source == "trip" {
		table, col, key = "gallery_photo", "ga_id", it.GaID
	}
	if key == nil {
		return nil
	}
	var cv int32
	found := true
	if err := q.QueryRow(ctx, `SELECT content_version FROM photo WHERE photo_id=$1`, m.PhotoID).Scan(&cv); errors.Is(err, pgx.ErrNoRows) {
		found = false
	} else if err != nil {
		return err
	}
	include := m.Manual == 1 || (m.Manual == 0 && m.Automatic && found && m.Version == cv)
	if include && !it.Suppressed {
		if table == "tag_photo" {
			_, err := q.Exec(ctx, `INSERT INTO tag_photo (photo_id, tag_id, user_id) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, m.PhotoID, *key, it.UserID)
			return err
		}
		_, err := q.Exec(ctx, `INSERT INTO gallery_photo (photo_id, ga_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, m.PhotoID, *key)
		return err
	}
	_, err := q.Exec(ctx, `DELETE FROM `+table+` WHERE `+col+`=$1 AND photo_id=$2`, *key, m.PhotoID)
	return err
}

func loadMember(ctx context.Context, q Q, item, photo int64) (*smartMember, error) {
	m := &smartMember{}
	err := q.QueryRow(ctx, `SELECT item_id, photo_id, content_version, automatic, manual, anchor, run FROM smart_member WHERE item_id=$1 AND photo_id=$2`, item, photo).
		Scan(&m.ItemID, &m.PhotoID, &m.Version, &m.Automatic, &m.Manual, &m.Anchor, &m.Run)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return m, err
}

// ---- helpers other packages (tags / galleries) call when a user edits a smart item ----

// SmartRename keeps an item's custom name in step with a renamed tag or gallery.
// SmartDismiss marks the item behind a deleted tag / gallery as dismissed.
// SmartManualMember records a user's explicit add (included) or removal.
func SmartDismiss(ctx context.Context, q Q, trip bool, key int64) error {
	col := "tag_id"
	if trip {
		col = "ga_id"
	}
	var id, uid int64
	err := q.QueryRow(ctx, `SELECT item_id, user_id FROM smart_item WHERE `+col+`=$1`, key).Scan(&id, &uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := smartRename(ctx, q, trip, key); err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `UPDATE smart_item SET suppressed=true, ga_id=NULL, tag_id=NULL WHERE item_id=$1`, id); err != nil {
		return err
	}
	return dirty(ctx, q, uid)
}

func SmartRename(ctx context.Context, q Q, trip bool, key int64) error {
	return smartRename(ctx, q, trip, key)
}

func smartRename(ctx context.Context, q Q, trip bool, key int64) error {
	if trip {
		_, err := q.Exec(ctx, `UPDATE smart_item i SET custom_name=g.ga_name, cover=jsonb_build_object('cover_id', g.cover_id, 'cover_color', g.cover_color)
			FROM gallery g WHERE i.ga_id=$1 AND g.ga_id=$1`, key)
		return err
	}
	_, err := q.Exec(ctx, `UPDATE smart_item i SET custom_name=t.tag_name FROM tag t WHERE i.tag_id=$1 AND t.tag_id=$1`, key)
	return err
}

func SmartManualMember(ctx context.Context, q Q, trip bool, key, photoID int64, included bool) error {
	col := "tag_id"
	if trip {
		col = "ga_id"
	}
	var id, uid int64
	err := q.QueryRow(ctx, `SELECT item_id, user_id FROM smart_item WHERE `+col+`=$1`, key).Scan(&id, &uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	manual := -1
	if included {
		manual = 1
	}
	if _, err := q.Exec(ctx, `INSERT INTO smart_member (item_id, photo_id, content_version, manual) SELECT $1,$2,content_version,$3 FROM photo WHERE photo_id=$2
		ON CONFLICT (item_id, photo_id) DO UPDATE SET manual=$3`, id, photoID, manual); err != nil {
		return err
	}
	return dirty(ctx, q, uid)
}
