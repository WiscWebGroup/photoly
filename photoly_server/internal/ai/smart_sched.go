package ai

// Smart Tag scheduler and bounded organization steps (scheduler/smart_tag_scheduler.py, smart/organize.py).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	smartBatch  = 25
	smartBudget = 5 * time.Second
	tripGap     = 36 * time.Hour
	tripSpan    = 14 * 24 * time.Hour
	tripNear    = 6 * time.Hour
	minScore    = .80
	minMargin   = .05
	maxAnchors  = 20
)

const pyTime = "2006-01-02T15:04:05.999999-07:00"

func pyIso(t time.Time) string { return t.UTC().Format(pyTime) }

func parseTime(v any) time.Time {
	s, _ := v.(string)
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

// SmartSched claims 'smart' jobs and advances each user's checkpoint a little at a time.
type SmartSched struct {
	pool       *pgxpool.Pool
	root       string
	owner      string
	policy     Policy
	gaz        *gazetteer
	appZone    string
	nextCheck  time.Time
	lastUser   int64
	userCursor int64
}

func (s *SmartSched) reconcile(ctx context.Context, q Q, cfg Settings) error {
	if !cfg.Bool("enabled") || !cfg.Bool("smart_tag_enabled") {
		return nil
	}
	read := func(after int64) ([]*smartPref, error) {
		rows, err := q.Query(ctx, `SELECT p.`+strings.ReplaceAll(prefCols, ", ", ", p.")+` FROM smart_preference p JOIN "user" u ON u.user_id=p.user_id
			WHERE p.enabled AND u.ai_search_enabled AND p.user_id>$1 ORDER BY p.user_id LIMIT 25`, after)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []*smartPref
		for rows.Next() {
			p := &smartPref{}
			if err := p.scan(rows); err != nil {
				return nil, err
			}
			out = append(out, p)
		}
		return out, rows.Err()
	}
	prefs, err := read(s.userCursor)
	if err != nil {
		return err
	}
	if len(prefs) == 0 && s.userCursor != 0 {
		if prefs, err = read(0); err != nil {
			return err
		}
	}
	s.userCursor = 0
	if len(prefs) > 0 {
		s.userCursor = prefs[len(prefs)-1].UserID
	}
	cur := cfg.PID("search_profile_id")
	for _, p := range prefs {
		same := (p.ProfileID == nil && cur == nil) || (p.ProfileID != nil && cur != nil && *p.ProfileID == *cur)
		if !same || p.Revision > p.Completed || len(p.Checkpoint) > 0 {
			if _, err := q.Exec(ctx, `INSERT INTO ai_job (kind, dedup_key, payload) VALUES ('smart',$1,$2) ON CONFLICT DO NOTHING`,
				fmt.Sprintf("smart:%d", p.UserID), map[string]any{"user_id": p.UserID}); err != nil {
				return err
			}
		}
	}
	return nil
}

// Tick runs at most one smart batch; it reports whether it did any work.
func (s *SmartSched) Tick(ctx context.Context) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	cfg, err := loadSettings(ctx, tx)
	if err != nil {
		return false, err
	}
	if time.Now().After(s.nextCheck) {
		if err := s.reconcile(ctx, tx, cfg); err != nil {
			return false, err
		}
		s.nextCheck = time.Now().Add(time.Minute)
	}
	reason := ""
	switch {
	case !cfg.Bool("enabled"):
		reason = "ai_disabled"
	case !cfg.Bool("smart_tag_enabled"):
		reason = "smart_tag_disabled"
	case cfg.Bool("maintenance"):
		reason = "maintenance"
	case cfg.Str("scheduler") != "running":
		reason = "scheduler_" + cfg.Str("scheduler")
	default:
		if d := s.policy.Decide(SystemUsage(s.root).toMap(), cfg); !d.Allowed {
			reason = strings.Join(d.Reasons, ", ")
		}
	}
	var r any
	if reason != "" {
		r = reason
	}
	if err := saveObserved(ctx, tx, "smart", map[string]any{"reason": r, "batchSize": smartBatch, "budgetSeconds": int(smartBudget.Seconds())}); err != nil {
		return false, err
	}
	if reason != "" {
		return false, tx.Commit(ctx)
	}
	lu := s.lastUser
	task, err := claim(ctx, tx, s.owner, ClaimOpts{Kinds: []string{"smart"}, LastUser: &lu})
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil || task == nil {
		return false, err
	}
	uid, _ := toInt(task.Payload["user_id"])
	s.lastUser = uid
	if err := s.batch(ctx, task, uid); err != nil {
		tx, e2 := s.pool.Begin(ctx)
		if e2 == nil {
			defer tx.Rollback(ctx)
			if j, _ := currentJob(ctx, tx, task); j != nil {
				status := "queued"
				if j.Attempts+1 >= 3 {
					status = "failed"
				}
				_, _ = tx.Exec(ctx, `UPDATE ai_job SET attempts=attempts+1, status=$2, result='{"reason":"smart_processing_failed"}' WHERE job_id=$1`, task.ID, status)
				if status == "failed" {
					_, _ = tx.Exec(ctx, `UPDATE smart_preference SET reason='smart_processing_failed', completed_revision=revision, checkpoint='{}' WHERE user_id=$1`, uid)
				}
			}
			_ = tx.Commit(ctx)
		}
		return true, err
	}
	return true, nil
}

// fence locks the user and the control row and re-validates the job; nil pref means stop.
func (s *SmartSched) fence(ctx context.Context, tx pgx.Tx, t *Task, uid int64) (*smartPref, error) {
	var x int64
	if err := tx.QueryRow(ctx, `SELECT user_id FROM "user" WHERE user_id=$1 FOR UPDATE`, uid).Scan(&x); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if _, err := control(ctx, tx, "global", true); err != nil {
		return nil, err
	}
	job, err := currentJob(ctx, tx, t)
	if err != nil || job == nil {
		return nil, err
	}
	pref, err := preference(ctx, tx, uid)
	if err != nil {
		return nil, err
	}
	if why, err := eligibility(ctx, tx, uid, true); err != nil || why != "" {
		if err == nil {
			_, err = finish(ctx, tx, t, "cancelled", map[string]any{"reason": "eligibility_changed"})
		}
		return nil, err
	}
	cfg, err := loadSettings(ctx, tx)
	if err != nil {
		return nil, err
	}
	cur := cfg.PID("search_profile_id")
	if (pref.ProfileID == nil) != (cur == nil) || (cur != nil && *pref.ProfileID != *cur) {
		pref.ProfileID = cur
		pref.Revision++
		pref.Checkpoint = map[string]any{}
	}
	return pref, nil
}

func (s *SmartSched) batch(ctx context.Context, t *Task, uid int64) error {
	deadline := time.Now().Add(smartBudget)
	for i := 0; i < smartBatch && time.Now().Before(deadline); i++ {
		type todo struct {
			photoID, userID int64
			version         int32
			path            string
		}
		var meta *todo
		done, err := func() (bool, error) {
			tx, err := s.pool.Begin(ctx)
			if err != nil {
				return false, err
			}
			defer tx.Rollback(ctx)
			pref, err := s.fence(ctx, tx, t, uid)
			if err != nil || pref == nil {
				return true, err
			}
			cp := pref.Checkpoint
			if len(cp) == 0 {
				cp = map[string]any{"phase": "metadata", "run": newUUID(), "revision": pref.Revision}
			}
			switch cp["phase"] {
			case "metadata":
				var pid int64
				var ver int32
				var puid, format, owner string
				err := tx.QueryRow(ctx, `SELECT p.photo_id, p.content_version, COALESCE(p.photo_uuid,''), COALESCE(p.format,''), u.uuid
					FROM photo p JOIN "user" u ON u.user_id=p.user_id LEFT JOIN smart_media sm ON sm.photo_id=p.photo_id
					WHERE p.user_id=$1 AND (sm.photo_id IS NULL OR sm.content_version<>p.content_version OR (sm.error IS NOT NULL AND sm.attempts<3))
					ORDER BY p.photo_id LIMIT 1`, uid).Scan(&pid, &ver, &puid, &format, &owner)
				switch {
				case err == nil:
					path, perr := mediaPath(s.root, owner, puid, format)
					if perr != nil {
						path = ""
					}
					meta = &todo{pid, uid, ver, path}
				case errors.Is(err, pgx.ErrNoRows):
					cp["phase"] = "anchors"
				default:
					return false, err
				}
			case "anchors":
				err = anchorStep(ctx, tx, pref, cp)
			case "missing":
				err = missingStep(ctx, tx, pref, cp)
			case "items":
				err = itemStep(ctx, tx, pref, cp)
			case "members":
				err = memberStep(ctx, tx, pref, cp)
			default:
				cp = map[string]any{}
			}
			if err != nil {
				return false, err
			}
			pref.Checkpoint = cp
			now := time.Now()
			pref.UpdatedAt = &now
			if err := pref.save(ctx, tx); err != nil {
				return false, err
			}
			if len(cp) == 0 {
				_, err = finish(ctx, tx, t, "", nil)
				if err == nil {
					err = tx.Commit(ctx)
				}
				return true, err
			}
			return false, tx.Commit(ctx)
		}()
		if err != nil || done {
			return err
		}
		if meta == nil {
			continue
		}
		var facts mediaFacts
		var code string
		if meta.path == "" {
			code = "metadata_dependency_or_file_missing"
		} else {
			facts, code = readMedia(ctx, meta.path, s.gaz, s.appZone)
		}
		stop, err := func() (bool, error) {
			tx, err := s.pool.Begin(ctx)
			if err != nil {
				return false, err
			}
			defer tx.Rollback(ctx)
			pref, err := s.fence(ctx, tx, t, uid)
			if err != nil || pref == nil {
				return true, err
			}
			var cv int32
			if err := tx.QueryRow(ctx, `SELECT content_version FROM photo WHERE photo_id=$1`, meta.photoID).Scan(&cv); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return false, tx.Commit(ctx)
				}
				return false, err
			}
			if cv != meta.version {
				return false, tx.Commit(ctx)
			}
			var e any
			attempts := 0
			if code != "" {
				e, attempts = code, 1
			}
			if _, err := tx.Exec(ctx, `INSERT INTO smart_media (photo_id,user_id,content_version,captured_at,latitude,longitude,facts,attempts,error)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
				ON CONFLICT (photo_id) DO UPDATE SET content_version=EXCLUDED.content_version, error=EXCLUDED.error,
					attempts=CASE WHEN $9::text IS NULL THEN 0 WHEN smart_media.content_version=$3 THEN smart_media.attempts+1 ELSE 1 END,
					captured_at=CASE WHEN $9::text IS NULL THEN EXCLUDED.captured_at ELSE smart_media.captured_at END,
					latitude=CASE WHEN $9::text IS NULL THEN EXCLUDED.latitude ELSE smart_media.latitude END,
					longitude=CASE WHEN $9::text IS NULL THEN EXCLUDED.longitude ELSE smart_media.longitude END,
					facts=CASE WHEN $9::text IS NULL THEN EXCLUDED.facts ELSE smart_media.facts END`,
				meta.photoID, uid, meta.version, facts.CapturedAt, facts.Lat, facts.Lon, nonNilMap(facts.Facts), attempts, e); err != nil {
				return false, err
			}
			if code == "" {
				if err := devicesStep(ctx, tx, uid, meta.photoID, meta.version, facts.Facts); err != nil {
					return false, err
				}
			}
			return false, tx.Commit(ctx)
		}()
		if err != nil || stop {
			return err
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	pref, err := s.fence(ctx, tx, t, uid)
	if err != nil || pref == nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE ai_job SET status='queued', result=$2 WHERE job_id=$1 AND status='running' AND lease_token=$3`, t.ID,
		map[string]any{"phase": pref.Checkpoint["phase"], "reason": pref.Reason}, t.Token)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func nonNilMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

// ---- organize steps ----

func findItem(ctx context.Context, q Q, uid int64, key string) (*smartItem, error) {
	i := &smartItem{}
	err := i.scan(q.QueryRow(ctx, `SELECT `+itemCols+` FROM smart_item WHERE user_id=$1 AND stable_key=$2`, uid, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if i.Basis == nil {
		i.Basis = map[string]any{}
	}
	return i, err
}

func newItem(ctx context.Context, q Q, uid int64, key, source, name string, basis map[string]any) (*smartItem, error) {
	var id int64
	if err := q.QueryRow(ctx, `INSERT INTO smart_item (user_id, stable_key, source, name, basis) VALUES ($1,$2,$3,$4,$5) RETURNING item_id`,
		uid, key, source, name, basis).Scan(&id); err != nil {
		return nil, err
	}
	return loadItem(ctx, q, id)
}

func devicesStep(ctx context.Context, q Q, uid, photoID int64, version int32, facts map[string]any) error {
	desired := []int64{}
	for _, source := range []string{"camera", "lens"} {
		name, _ := facts[source].(string)
		if name == "" {
			continue
		}
		sum := sha256.Sum256([]byte(strings.ToLower(name)))
		key := source + ":" + hex.EncodeToString(sum[:])
		it, err := findItem(ctx, q, uid, key)
		if err != nil {
			return err
		}
		if it == nil {
			it, err = newItem(ctx, q, uid, key, source, name, map[string]any{"kind": strings.Title(source), "method": "EXIF device model"})
			if err != nil {
				return err
			}
		}
		desired = append(desired, it.ID)
		if err := ensureCatalog(ctx, q, it, true); err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `INSERT INTO smart_member (item_id, photo_id, content_version, automatic) VALUES ($1,$2,$3,true)
			ON CONFLICT (item_id, photo_id) DO UPDATE SET automatic=true, content_version=$3`, it.ID, photoID, version); err != nil {
			return err
		}
		m, err := loadMember(ctx, q, it.ID, photoID)
		if err != nil {
			return err
		}
		if err := syncLink(ctx, q, it, m); err != nil {
			return err
		}
	}
	rows, err := q.Query(ctx, `SELECT m.item_id FROM smart_member m JOIN smart_item i ON i.item_id=m.item_id
		WHERE m.photo_id=$1 AND i.source<>'trip' AND NOT (m.item_id = ANY($2))`, photoID, desired)
	if err != nil {
		return err
	}
	var old []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		old = append(old, id)
	}
	rows.Close()
	for _, id := range old {
		if _, err := q.Exec(ctx, `UPDATE smart_member SET automatic=false WHERE item_id=$1 AND photo_id=$2`, id, photoID); err != nil {
			return err
		}
		it, err := loadItem(ctx, q, id)
		if err != nil || it == nil {
			return err
		}
		m, err := loadMember(ctx, q, id, photoID)
		if err != nil {
			return err
		}
		if err := syncLink(ctx, q, it, m); err != nil {
			return err
		}
	}
	return nil
}

func inherit(ctx context.Context, q Q, parent, child *smartItem) error {
	if parent.ID == child.ID {
		return nil
	}
	if _, err := q.Exec(ctx, `INSERT INTO smart_lineage (parent_id, child_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, parent.ID, child.ID); err != nil {
		return err
	}
	child.Suppressed = child.Suppressed || parent.Suppressed
	if child.CustomName == nil || *child.CustomName == "" {
		child.CustomName = parent.CustomName
	}
	if child.Cover == nil {
		child.Cover = parent.Cover
	}
	// Copy explicit choices, not old automatic membership. Exclusions win conflicts.
	_, err := q.Exec(ctx, `INSERT INTO smart_member (item_id, photo_id, content_version, manual)
		SELECT $1, photo_id, content_version, manual FROM smart_member WHERE item_id=$2 AND manual<>0
		ON CONFLICT (item_id, photo_id) DO UPDATE SET manual=CASE WHEN smart_member.manual=0 THEN EXCLUDED.manual ELSE LEAST(smart_member.manual, EXCLUDED.manual) END`,
		child.ID, parent.ID)
	return err
}

type mediaRow struct {
	PhotoID, UserID int64
	Version         int32
	At              time.Time
	Facts           map[string]any
}

func localDate(m *mediaRow) any { return m.Facts["localDate"] }

func anchorStep(ctx context.Context, q Q, pref *smartPref, cp map[string]any) error {
	sql := `SELECT s.photo_id, s.user_id, s.content_version, s.captured_at, s.facts FROM smart_media s JOIN photo p ON p.photo_id=s.photo_id
		WHERE s.user_id=$1 AND s.content_version=p.content_version AND s.error IS NULL AND s.captured_at IS NOT NULL AND s.latitude IS NOT NULL`
	args := []any{pref.UserID}
	last := time.Time{}
	if cp["time"] != nil {
		last = parseTime(cp["time"])
		id, _ := toInt(cp["id"])
		sql += ` AND (s.captured_at, s.photo_id) > ($2, $3)`
		args = append(args, last, id)
	}
	m := &mediaRow{}
	err := q.QueryRow(ctx, sql+` ORDER BY s.captured_at, s.photo_id LIMIT 1`, args...).Scan(&m.PhotoID, &m.UserID, &m.Version, &m.At, &m.Facts)
	if errors.Is(err, pgx.ErrNoRows) {
		cp["phase"], cp["cursor"] = "missing", 0
		return nil
	}
	if err != nil {
		return err
	}
	run, _ := cp["run"].(string)
	oldItems := func(rows pgx.Rows, e error) ([]*smartItem, error) {
		if e != nil {
			return nil, e
		}
		defer rows.Close()
		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		rows.Close()
		var out []*smartItem
		for _, id := range ids {
			it, err := loadItem(ctx, q, id)
			if err != nil {
				return nil, err
			}
			out = append(out, it)
		}
		return out, nil
	}
	old, err := oldItems(q.Query(ctx, `SELECT i.item_id FROM smart_item i JOIN smart_member m ON m.item_id=i.item_id
		WHERE m.photo_id=$1 AND i.source='trip' AND i.user_id=$2 ORDER BY i.suppressed DESC, (i.ga_id IS NOT NULL) DESC, i.item_id`, m.PhotoID, pref.UserID))
	if err != nil {
		return err
	}
	var start time.Time
	if cp["start"] != nil {
		start = parseTime(cp["start"])
	}
	var item *smartItem
	if last.IsZero() || m.At.Sub(last) > tripGap || m.At.Sub(start) > tripSpan {
		// A late upload just before an existing trip should retain that trip's identity.
		nearby, err := oldItems(q.Query(ctx, `SELECT i.item_id FROM smart_item i JOIN smart_member mm ON mm.item_id=i.item_id JOIN smart_media s ON s.photo_id=mm.photo_id
			WHERE i.user_id=$1 AND i.source='trip' AND s.latitude IS NOT NULL AND s.captured_at BETWEEN $2 AND $3
			ORDER BY s.captured_at, i.item_id LIMIT 20`, pref.UserID, m.At, m.At.Add(tripGap)))
		if err != nil {
			return err
		}
		seen := map[int64]bool{}
		merged := old[:0:0]
		for _, it := range append(append([]*smartItem{}, old...), nearby...) {
			if !seen[it.ID] {
				seen[it.ID] = true
				merged = append(merged, it)
			}
		}
		old = merged
		for _, it := range old {
			if it.Run == nil || *it.Run != run {
				item = it
				break
			}
		}
		if item == nil {
			item, err = newItem(ctx, q, pref.UserID, "trip:"+newUUID(), "trip", "Travel moments", map[string]any{})
			if err != nil {
				return err
			}
		}
		item.Run = &run
		item.Basis = map[string]any{"method": "Capture time, nearby locations and visual similarity", "start": pyIso(m.At), "end": pyIso(m.At), "places": map[string]any{},
			"startDate": localDate(m), "endDate": localDate(m), "profileId": pref.ProfileID, "similarity": minScore, "margin": minMargin, "timezoneSources": []any{}}
		cp["item"], cp["start"] = item.ID, pyIso(m.At)
	} else {
		id, _ := toInt(cp["item"])
		if item, err = loadItem(ctx, q, id); err != nil || item == nil {
			return errors.Join(err, errors.New("smart_item_missing"))
		}
	}
	for _, parent := range old {
		if err := inherit(ctx, q, parent, item); err != nil {
			return err
		}
	}
	places, _ := item.Basis["places"].(map[string]any)
	if places == nil {
		places = map[string]any{}
	}
	if c, ok := m.Facts["city"].(map[string]any); ok {
		name, _ := c["name"].(string)
		n, _ := toInt(places[name])
		places[name] = n + 1
	}
	srcs := map[string]bool{}
	if arr, ok := item.Basis["timezoneSources"].([]any); ok {
		for _, v := range arr {
			if s, ok := v.(string); ok {
				srcs[s] = true
			}
		}
	}
	src, _ := m.Facts["timezoneSource"].(string)
	if src == "" {
		src = "unknown"
	}
	srcs[src] = true
	list := make([]string, 0, len(srcs))
	for s := range srcs {
		list = append(list, s)
	}
	sort.Strings(list)
	sl := make([]any, len(list))
	for i, s := range list {
		sl[i] = s
	}
	item.Basis["end"], item.Basis["endDate"], item.Basis["places"], item.Basis["timezoneSources"] = pyIso(m.At), localDate(m), places, sl
	if err := item.save(ctx, q); err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `INSERT INTO smart_member (item_id, photo_id, content_version, automatic, anchor, run) VALUES ($1,$2,$3,true,true,$4)
		ON CONFLICT (item_id, photo_id) DO UPDATE SET automatic=true, anchor=true, run=$4, content_version=$3`, item.ID, m.PhotoID, m.Version, run); err != nil {
		return err
	}
	cp["time"], cp["id"] = pyIso(m.At), m.PhotoID
	return nil
}

// docVectors returns the unit-normalised vectors of a photo (or its first five video frames) for the profile.
func docVectors(ctx context.Context, q Q, photoID int64, profileID *int64) ([][]float32, error) {
	if profileID == nil {
		return nil, nil
	}
	var state int64
	var emb []float32
	err := q.QueryRow(ctx, `SELECT s.state_id, s.embedding::real[] FROM photo_embedding_state s JOIN photo p ON p.photo_id=s.photo_id
		WHERE s.photo_id=$1 AND s.profile_id=$2 AND s.status='ready' AND s.content_version=p.content_version`, photoID, *profileID).Scan(&state, &emb)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	raw := [][]float32{}
	if emb != nil {
		raw = append(raw, emb)
	} else {
		rows, err := q.Query(ctx, `SELECT embedding::real[] FROM video_frame_embedding WHERE state_id=$1 AND profile_id=$2 ORDER BY frame_index LIMIT 5`, state, *profileID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var v []float32
			if err := rows.Scan(&v); err != nil {
				return nil, err
			}
			raw = append(raw, v)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	if len(raw) == 0 {
		return nil, nil
	}
	for _, v := range raw {
		var n float64
		for _, x := range v {
			if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
				return nil, nil
			}
			n += float64(x) * float64(x)
		}
		if n <= 0 || len(v) != len(raw[0]) {
			return nil, nil
		}
		n = math.Sqrt(n)
		for i := range v {
			v[i] = float32(float64(v[i]) / n)
		}
	}
	return raw, nil
}

func maxDot(a, b [][]float32) float64 {
	best := math.Inf(-1)
	for _, x := range a {
		for _, y := range b {
			var d float64
			for i := range x {
				d += float64(x[i]) * float64(y[i])
			}
			best = math.Max(best, d)
		}
	}
	return best
}

func missingStep(ctx context.Context, q Q, pref *smartPref, cp map[string]any) error {
	cursor, _ := toInt(cp["cursor"])
	m := &mediaRow{}
	err := q.QueryRow(ctx, `SELECT s.photo_id, s.user_id, s.content_version, s.captured_at, s.facts FROM smart_media s JOIN photo p ON p.photo_id=s.photo_id
		WHERE s.user_id=$1 AND s.photo_id>$2 AND s.captured_at IS NOT NULL AND s.latitude IS NULL AND s.content_version=p.content_version AND s.error IS NULL
		ORDER BY s.photo_id LIMIT 1`, pref.UserID, cursor).Scan(&m.PhotoID, &m.UserID, &m.Version, &m.At, &m.Facts)
	if errors.Is(err, pgx.ErrNoRows) {
		cp["phase"], cp["cursor"] = "items", 0
		return nil
	}
	if err != nil {
		return err
	}
	cp["cursor"] = m.PhotoID
	run, _ := cp["run"].(string)
	source, err := docVectors(ctx, q, m.PhotoID, pref.ProfileID)
	if err != nil {
		return err
	}
	rows, err := q.Query(ctx, `SELECT mm.item_id, s.photo_id FROM smart_member mm JOIN smart_media s ON s.photo_id=mm.photo_id
		WHERE s.user_id=$1 AND mm.run=$2 AND mm.anchor AND s.captured_at BETWEEN $3 AND $4
		ORDER BY abs(extract(epoch FROM s.captured_at - $5::timestamptz)), s.photo_id LIMIT $6`,
		pref.UserID, run, m.At.Add(-tripNear), m.At.Add(tripNear), m.At, maxAnchors)
	if err != nil {
		return err
	}
	type anchor struct{ item, photo int64 }
	var anchors []anchor
	for rows.Next() {
		var a anchor
		if err := rows.Scan(&a.item, &a.photo); err != nil {
			rows.Close()
			return err
		}
		anchors = append(anchors, a)
	}
	rows.Close()
	scores := map[int64]float64{}
	incomplete := false
	if source != nil {
		for _, a := range anchors {
			group, err := loadItem(ctx, q, a.item)
			if err != nil || group == nil {
				return err
			}
			st, en := parseTime(group.Basis["start"]), parseTime(group.Basis["end"])
			if m.At.Sub(st) > tripSpan || en.Sub(m.At) > tripSpan {
				continue
			}
			target, err := docVectors(ctx, q, a.photo, pref.ProfileID)
			if err != nil {
				return err
			}
			if target == nil {
				cp["waiting"], incomplete = true, true
				continue
			}
			d := maxDot(source, target)
			if cur, ok := scores[a.item]; !ok || d > cur {
				scores[a.item] = d
			}
		}
	} else if len(anchors) > 0 {
		cp["waiting"] = true
	}
	if incomplete {
		return nil
	}
	type rank struct {
		id    int64
		score float64
	}
	var ranking []rank
	for id, sc := range scores {
		ranking = append(ranking, rank{id, sc})
	}
	sort.Slice(ranking, func(i, j int) bool {
		if ranking[i].score != ranking[j].score {
			return ranking[i].score > ranking[j].score
		}
		return ranking[i].id < ranking[j].id
	})
	if len(ranking) > 0 && ranking[0].score >= minScore && (len(ranking) == 1 || ranking[0].score-ranking[1].score >= minMargin) {
		it, err := loadItem(ctx, q, ranking[0].id)
		if err != nil || it == nil {
			return err
		}
		if m.At.Before(parseTime(it.Basis["start"])) {
			it.Basis["start"], it.Basis["startDate"] = pyIso(m.At), localDate(m)
		}
		if m.At.After(parseTime(it.Basis["end"])) {
			it.Basis["end"], it.Basis["endDate"] = pyIso(m.At), localDate(m)
		}
		if err := it.save(ctx, q); err != nil {
			return err
		}
		_, err = q.Exec(ctx, `INSERT INTO smart_member (item_id, photo_id, content_version, automatic, anchor, run) VALUES ($1,$2,$3,true,false,$4)
			ON CONFLICT (item_id, photo_id) DO UPDATE SET automatic=true, anchor=false, run=$4, content_version=$3`, it.ID, m.PhotoID, m.Version, run)
		return err
	}
	return nil
}

func pyStr(v any) string {
	if v == nil {
		return "None"
	}
	return fmt.Sprint(v)
}

func itemStep(ctx context.Context, q Q, pref *smartPref, cp map[string]any) error {
	cursor, _ := toInt(cp["cursor"])
	var id int64
	err := q.QueryRow(ctx, `SELECT item_id FROM smart_item WHERE user_id=$1 AND item_id>$2 AND source='trip' ORDER BY item_id LIMIT 1`, pref.UserID, cursor).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		cp["phase"], cp["cursor"], cp["member"] = "members", 0, 0
		return nil
	}
	if err != nil {
		return err
	}
	cp["cursor"] = id
	run, _ := cp["run"].(string)
	it, err := loadItem(ctx, q, id)
	if err != nil {
		return err
	}
	var count, anchors int64
	if err := q.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE anchor) FROM smart_member WHERE item_id=$1 AND run=$2`, id, run).Scan(&count, &anchors); err != nil {
		return err
	}
	visible := count >= 5 && anchors >= 2
	if !visible && it.Basis["eligible"] == true {
		var pins, merged bool
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM smart_member WHERE item_id=$1 AND manual=1),
			EXISTS (SELECT 1 FROM smart_lineage l JOIN smart_item c ON c.item_id=l.child_id WHERE l.parent_id=$1 AND c.run=$2)`, id, run).Scan(&pins, &merged); err != nil {
			return err
		}
		visible = pins && !merged
	}
	it.Basis["eligible"] = visible
	if it.Run != nil && *it.Run == run {
		places, _ := it.Basis["places"].(map[string]any)
		names := make([]string, 0, len(places))
		for n := range places {
			names = append(names, n)
		}
		sort.Slice(names, func(i, j int) bool {
			a, _ := toInt(places[names[i]])
			b, _ := toInt(places[names[j]])
			if a != b {
				return a > b
			}
			return names[i] < names[j]
		})
		title := "Travel moments"
		if len(names) > 0 {
			title = "Times in " + strings.Join(names[:min(2, len(names))], " and ")
		}
		if len(names) > 2 {
			title += fmt.Sprintf(" + %d places", len(names)-2)
		}
		it.Name = trunc(trunc(title, 210)+" · "+pyStr(it.Basis["startDate"])+" – "+pyStr(it.Basis["endDate"]), 255)
		it.Basis["mediaCount"], it.Basis["gpsCount"] = count, anchors
	}
	if err := it.save(ctx, q); err != nil {
		return err
	}
	return ensureCatalog(ctx, q, it, visible)
}

func memberStep(ctx context.Context, q Q, pref *smartPref, cp map[string]any) error {
	cursor, _ := toInt(cp["cursor"])
	member, _ := toInt(cp["member"])
	var item, photo int64
	var mrun *string
	err := q.QueryRow(ctx, `SELECT m.item_id, m.photo_id, m.run FROM smart_member m JOIN smart_item i ON i.item_id=m.item_id
		WHERE i.user_id=$1 AND i.source='trip' AND (m.item_id, m.photo_id) > ($2, $3) ORDER BY m.item_id, m.photo_id LIMIT 1`,
		pref.UserID, cursor, member).Scan(&item, &photo, &mrun)
	if errors.Is(err, pgx.ErrNoRows) {
		rev, _ := toInt(cp["revision"])
		pref.Completed = rev
		pref.Reason = nil
		if cp["waiting"] == true {
			r := "waiting_for_vectors"
			pref.Reason = &r
		}
		clear(cp)
		return nil
	}
	if err != nil {
		return err
	}
	cp["cursor"], cp["member"] = item, photo
	run, _ := cp["run"].(string)
	auto := mrun != nil && *mrun == run
	if _, err := q.Exec(ctx, `UPDATE smart_member SET automatic=$3 WHERE item_id=$1 AND photo_id=$2`, item, photo, auto); err != nil {
		return err
	}
	it, err := loadItem(ctx, q, item)
	if err != nil || it == nil {
		return err
	}
	m, err := loadMember(ctx, q, item, photo)
	if err != nil {
		return err
	}
	return syncLink(ctx, q, it, m)
}
