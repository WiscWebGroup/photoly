package photo

import (
	"os"
	"path/filepath"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func imageFormatList() []string {
	out := make([]string, 0, len(ImageFormats))
	for k := range ImageFormats {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func collect(rows pgx.Rows) ([]map[string]any, error) {
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		p, err := scanPhoto(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p.JSON())
	}
	return out, rows.Err()
}

func (s *Service) allJSON(ctx context.Context, sql string, args ...any) (any, error) {
	rows, err := s.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return collect(rows)
}

// DateSummary is PhotoService.date_summary.
func (s *Service) DateSummary(ctx context.Context, uid int64) (any, error) {
	rows, err := s.Pool.Query(ctx, `SELECT date(coalesce(e.extra_photo_taken_date, p.upload_date)) AS d, count(p.photo_id)
		FROM photo p LEFT JOIN photo_extra e ON e.f_photo_id=p.photo_id
		WHERE p.user_id=$1 AND lower(p.format)=ANY($2) GROUP BY d ORDER BY d`, uid, imageFormatList())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var total, undated int64
	days := make([]map[string]any, 0)
	for rows.Next() {
		var d *time.Time
		var n int64
		if err := rows.Scan(&d, &n); err != nil {
			return nil, err
		}
		total += n
		if d == nil {
			undated = n
		} else {
			days = append(days, map[string]any{"date": d.Format("2006-01-02"), "count": n})
		}
	}
	return map[string]any{"totalCount": total, "undatedCount": undated, "days": days}, rows.Err()
}

var (
	monthRE = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}$`)
	dayRE   = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
)

const dateExpr = `coalesce(e.extra_photo_taken_date, p.upload_date)`

// MonthPreview is PhotoService.month_preview.
func (s *Service) MonthPreview(ctx context.Context, uid int64, month string, hasMonth bool, order string) (any, error) {
	if !hasMonth || !monthRE.MatchString(month) || (order != "latest" && order != "oldest") {
		return nil, Bad()
	}
	start, err := time.Parse("2006-01", month)
	if err != nil || start.Year() < 1 {
		return nil, Bad()
	}
	q := `SELECT p.photo_id FROM photo p LEFT JOIN photo_extra e ON e.f_photo_id=p.photo_id
		WHERE p.user_id=$1 AND lower(p.format)=ANY($2) AND ` + dateExpr + ` >= $3`
	args := []any{uid, imageFormatList(), start}
	if month != "9999-12" {
		q += ` AND ` + dateExpr + ` < $4`
		args = append(args, start.AddDate(0, 1, 0))
	}
	dir := "DESC"
	if order == "oldest" {
		dir = "ASC"
	}
	rows, err := s.Pool.Query(ctx, q+fmt.Sprintf(` ORDER BY %s %s, p.photo_id %s LIMIT 10`, dateExpr, dir, dir), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func pyIso(t time.Time) string { return isoNaive(t) }

func parseISO(v string) (time.Time, bool) {
	for _, l := range []string{"2006-01-02T15:04:05.999999", "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(l, v); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// ByDate is PhotoService.by_date: owner-scoped keyset pagination, undated images last.
func (s *Service) ByDate(ctx context.Context, uid int64, order string, limit int64, cursor string, day *string) (any, error) {
	if (order != "latest" && order != "oldest") || limit < 1 || limit > 200 {
		return nil, Bad()
	}
	args := []any{uid, imageFormatList()}
	where := `p.user_id=$1 AND lower(p.format)=ANY($2)`
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	if day != nil {
		if *day == "undated" {
			where += ` AND ` + dateExpr + ` IS NULL`
		} else {
			if !dayRE.MatchString(*day) {
				return nil, Bad()
			}
			start, err := time.Parse("2006-01-02", *day)
			if err != nil || start.Year() < 1 {
				return nil, Bad()
			}
			where += ` AND ` + dateExpr + ` >= ` + arg(start)
			if *day != "9999-12-31" {
				where += ` AND ` + dateExpr + ` < ` + arg(start.AddDate(0, 0, 1))
			}
		}
	}
	latest := order == "latest"
	if cursor != "" {
		if len(cursor) > 512 {
			return nil, Bad()
		}
		raw, err := base64.URLEncoding.DecodeString(cursor)
		if err != nil {
			return nil, Bad()
		}
		var c map[string]any
		d := json.NewDecoder(strings.NewReader(string(raw)))
		d.UseNumber()
		if d.Decode(&c) != nil || c == nil {
			return nil, Bad()
		}
		cday, _ := c["day"].(string)
		if c["order"] != order || fmt.Sprint(c["user"]) != fmt.Sprint(uid) || (c["day"] == nil) != (day == nil) || (day != nil && cday != *day) {
			return nil, Bad()
		}
		num, ok := c["id"].(json.Number)
		id, err := num.Int64()
		if !ok || err != nil || id <= 0 || id > 2147483647 || strings.ContainsAny(num.String(), ".eE") {
			return nil, Bad()
		}
		var stamp *time.Time
		if c["date"] != nil {
			sv, ok := c["date"].(string)
			t, ok2 := parseISO(sv)
			if !ok || !ok2 {
				return nil, Bad()
			}
			stamp = &t
		}
		idCmp := " > "
		if latest {
			idCmp = " < "
		}
		afterID := `p.photo_id` + idCmp + arg(id)
		if stamp == nil {
			where += ` AND ` + dateExpr + ` IS NULL AND ` + afterID
		} else {
			dcmp := " > "
			if latest {
				dcmp = " < "
			}
			sp := arg(*stamp)
			where += ` AND (` + dateExpr + ` IS NULL OR ` + dateExpr + dcmp + sp + ` OR (` + dateExpr + ` = ` + sp + ` AND ` + afterID + `))`
		}
	}
	dir := "ASC"
	if latest {
		dir = "DESC"
	}
	lim := arg(limit + 1)
	rows, err := s.Pool.Query(ctx, `SELECT `+photoCols+`, e.extra_photo_taken_date, `+dateExpr+` FROM photo p
		LEFT JOIN photo_extra e ON e.f_photo_id=p.photo_id WHERE `+where+
		` ORDER BY `+dateExpr+` `+dir+` NULLS LAST, p.photo_id `+dir+` LIMIT `+lim, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	var extraRow bool
	for rows.Next() {
		var p Photo
		var taken, stamp *time.Time
		if err := rows.Scan(&p.ID, &p.Name, &p.UploadDate, &p.Format, &p.NsID, &p.Visibility, &p.Token, &p.UUID, &p.UserID, &p.ContentVersion, &taken, &stamp); err != nil {
			return nil, err
		}
		if int64(len(items)) == limit {
			extraRow = true
			break
		}
		j := p.JSON()
		j["timelineDate"] = optTime(stamp)
		switch {
		case taken != nil:
			j["dateSource"] = "taken"
		case stamp != nil:
			j["dateSource"] = "upload"
		default:
			j["dateSource"] = nil
		}
		items = append(items, j)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var next any
	if extraRow {
		last := items[len(items)-1]
		// json.dumps default separators and key order, urlsafe base64 with padding.
		dv := "null"
		if last["timelineDate"] != nil {
			dv = jsonStr(last["timelineDate"].(string))
		}
		dayv := "null"
		if day != nil {
			dayv = jsonStr(*day)
		}
		payload := fmt.Sprintf(`{"order": %s, "user": %d, "id": %d, "date": %s, "day": %s}`, jsonStr(order), uid, last["photoId"], dv, dayv)
		next = base64.URLEncoding.EncodeToString([]byte(payload))
	}
	return map[string]any{"items": items, "nextCursor": next}, nil
}

func jsonStr(s string) string {
	// ensure_ascii like Python's json.dumps
	b, _ := json.Marshal(s)
	var out strings.Builder
	for _, r := range string(b) {
		if r > 126 {
			if r > 0xFFFF {
				r -= 0x10000
				fmt.Fprintf(&out, `\u%04x\u%04x`, 0xD800+(r>>10), 0xDC00+(r&0x3FF))
			} else {
				fmt.Fprintf(&out, `\u%04x`, r)
			}
		} else {
			out.WriteRune(r)
		}
	}
	return out.String()
}

// ByNamespace is by_namespace.
func (s *Service) ByNamespace(ctx context.Context, uid int64, ns *int64) (any, error) {
	return s.allJSON(ctx, `SELECT `+photoCols+` FROM photo p WHERE p.user_id=$1 AND p.ns_id IS NOT DISTINCT FROM $2 ORDER BY p.photo_id`, uid, ns)
}

// ByGallery is by_gallery.
func (s *Service) ByGallery(ctx context.Context, uid, ga int64) (any, error) {
	if err := s.Owned(ctx, s.Pool, "gallery", ga, uid, nil, 400); err != nil {
		return nil, err
	}
	return s.allJSON(ctx, `SELECT `+photoCols+` FROM photo p JOIN gallery_photo g ON g.photo_id=p.photo_id
		WHERE g.ga_id=$1 AND p.user_id=$2 ORDER BY p.photo_id`, ga, uid)
}

// ByTags is by_tags (photos carrying every listed tag).
func (s *Service) ByTags(ctx context.Context, uid int64, tagIDs []int64) (any, error) {
	set := uniqueSorted(tagIDs)
	if len(set) == 0 {
		return []any{}, nil
	}
	for _, id := range set {
		if err := s.Owned(ctx, s.Pool, "tag", id, uid, nil, 400); err != nil {
			return nil, err
		}
	}
	return s.allJSON(ctx, `SELECT `+photoCols+` FROM photo p WHERE p.user_id=$1 AND p.photo_id IN (
		SELECT photo_id FROM tag_photo WHERE user_id=$1 AND tag_id=ANY($2) GROUP BY photo_id HAVING count(DISTINCT tag_id)=$3)
		ORDER BY p.photo_id`, uid, set, len(set))
}

func uniqueSorted(v []int64) []int64 {
	m := map[int64]bool{}
	out := []int64{}
	for _, x := range v {
		if !m[x] {
			m[x] = true
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Covers is PhotoService.covers.
func (s *Service) Covers(ctx context.Context, uid int64, source string, hasSource bool, ids []int64) (any, error) {
	if !hasSource {
		return nil, Bad()
	}
	kinds := map[string]string{"folder": "namespace", "gallery": "gallery", "tag": "tag"}
	kind, ok := kinds[source]
	keys := uniqueSorted(ids)
	if !ok || len(keys) > 500 {
		return nil, Bad()
	}
	for _, k := range keys {
		if err := s.Owned(ctx, s.Pool, kind, k, uid, -1, 400); err != nil {
			return nil, err
		}
	}
	if len(keys) == 0 {
		return []any{}, nil
	}
	var q string
	switch source {
	case "folder":
		q = `SELECT p.ns_id, p.photo_id, p.format FROM photo p WHERE p.user_id=$1 AND p.ns_id=ANY($2) ORDER BY p.photo_id DESC`
	case "gallery":
		q = `SELECT l.ga_id, p.photo_id, p.format FROM photo p JOIN gallery_photo l ON l.photo_id=p.photo_id WHERE p.user_id=$1 AND l.ga_id=ANY($2) ORDER BY p.photo_id DESC`
	default:
		q = `SELECT l.tag_id, p.photo_id, p.format FROM photo p JOIN tag_photo l ON l.photo_id=p.photo_id WHERE p.user_id=$1 AND l.tag_id=ANY($2) ORDER BY p.photo_id DESC`
	}
	rows, err := s.Pool.Query(ctx, q, uid, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type cover struct {
		id    int64
		count int64
		ids   []int64
	}
	found := map[int64]*cover{}
	for _, k := range keys {
		found[k] = &cover{id: k, ids: []int64{}}
	}
	for rows.Next() {
		var k, pid int64
		var f *string
		if err := rows.Scan(&k, &pid, &f); err != nil {
			return nil, err
		}
		c := found[k]
		c.count++
		if len(c.ids) < 3 && f != nil && ImageFormats[strings.ToLower(*f)] {
			c.ids = append(c.ids, pid)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		c := found[k]
		out = append(out, map[string]any{"id": c.id, "count": c.count, "coverIds": c.ids})
	}
	return out, nil
}

// SearchPhoto is PhotoService.search.
func (s *Service) SearchPhoto(ctx context.Context, uid int64, option, query string) (any, error) {
	pattern := "%" + query + "%"
	var q string
	switch option {
	case "photoName":
		q = `p.photo_name ILIKE $2`
	case "namespace":
		q = `p.ns_id IN (SELECT ns_id FROM namespace WHERE user_id=$1 AND ns_name ILIKE $2)`
	case "gallery":
		q = `p.photo_id IN (SELECT gp.photo_id FROM gallery_photo gp JOIN gallery g ON g.ga_id=gp.ga_id WHERE g.user_id=$1 AND g.ga_name ILIKE $2)`
	case "tag":
		q = `p.photo_id IN (SELECT tp.photo_id FROM tag_photo tp JOIN tag t ON t.tag_id=tp.tag_id WHERE t.user_id=$1 AND t.tag_name ILIKE $2)`
	default:
		return []any{}, nil
	}
	return s.allJSON(ctx, `SELECT `+photoCols+` FROM photo p WHERE p.user_id=$1 AND `+q+` ORDER BY p.photo_id`, uid, pattern)
}

// SearchSmart is smart_search.search_smart: bounded lexical candidates.
func (s *Service) SearchSmart(ctx context.Context, uid int64, query string, ok bool) (any, error) {
	if !ok || len([]rune(query)) > 512 {
		return nil, Bad()
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return []any{}, nil
	}
	words, seen := []string{}, map[string]bool{}
	for _, w := range strings.Fields(query) {
		if !seen[w] {
			seen[w] = true
			words = append(words, w)
		}
	}
	// match(col): exact 3, contains whole query 2, else fraction of words contained.
	match := func(col string) string {
		name := `lower(coalesce(` + col + `,''))`
		var parts []string
		for i := range words {
			parts = append(parts, fmt.Sprintf(`(CASE WHEN position($%d in %s)>0 THEN 1 ELSE 0 END)`, i+3, name))
		}
		return fmt.Sprintf(`(CASE WHEN %s = $2 THEN 3.0 WHEN position($2 in %s)>0 THEN 2.0 ELSE (%s)::float8/%d.0 END)`, name, name, strings.Join(parts, "+"), len(words))
	}
	args := []any{uid, query}
	for _, w := range words {
		args = append(args, w)
	}
	score := `(0.4*` + match("p.photo_name") + ` + 0.3*coalesce(t.score,0) + 0.3*coalesce(g.score,0))`
	sql := `SELECT ` + photoCols + ` FROM photo p
		LEFT JOIN (SELECT tp.photo_id, max(` + match("t.tag_name") + `) AS score FROM tag_photo tp JOIN tag t ON t.tag_id=tp.tag_id
			WHERE t.user_id=$1 AND tp.user_id=$1 GROUP BY tp.photo_id) t ON t.photo_id=p.photo_id
		LEFT JOIN (SELECT gp.photo_id, max(` + match("g.ga_name") + `) AS score FROM gallery_photo gp JOIN gallery g ON g.ga_id=gp.ga_id
			WHERE g.user_id=$1 GROUP BY gp.photo_id) g ON g.photo_id=p.photo_id
		WHERE p.user_id=$1 AND ` + score + ` > 0 ORDER BY ` + score + ` DESC, p.photo_id LIMIT 100`
	return s.allJSON(ctx, sql, args...)
}

// ---- links and smart-catalog membership ----

func (s *Service) manualMember(ctx context.Context, tx pgx.Tx, kind string, key, photoID int64, included bool) error {
	col := "tag_id"
	if kind == "gallery" {
		col = "ga_id"
	}
	var item, owner int64
	err := tx.QueryRow(ctx, `SELECT item_id, user_id FROM smart_item WHERE `+col+`=$1`, key).Scan(&item, &owner)
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		return err
	}
	manual := -1
	if included {
		manual = 1
	}
	if _, err = tx.Exec(ctx, `INSERT INTO smart_member (item_id, photo_id, content_version, manual)
		SELECT $1, photo_id, content_version, $3 FROM photo WHERE photo_id=$2
		ON CONFLICT (item_id, photo_id) DO UPDATE SET manual=EXCLUDED.manual`, item, photoID, manual); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE smart_preference SET revision=revision+1 WHERE user_id=$1`, owner)
	return err
}

// AddTags is PhotoService.add_tags.
func (s *Service) AddTags(ctx context.Context, uid, photoID int64, tagIDs []int64) (int, error) {
	count := 0
	err := s.Atomic(ctx, uid, func(tx pgx.Tx, u UserRow, f *Files) error {
		if _, err := s.OwnedPhoto(ctx, tx, photoID, uid, -1, 400); err != nil {
			return err
		}
		for _, k := range tagIDs {
			if err := s.Owned(ctx, tx, "tag", k, uid, -1, 400); err != nil {
				return err
			}
		}
		for _, k := range uniqueSorted(tagIDs) {
			if err := s.manualMember(ctx, tx, "tag", k, photoID, true); err != nil {
				return err
			}
			ct, err := tx.Exec(ctx, `INSERT INTO tag_photo (photo_id, tag_id, user_id)
				SELECT $1,$2,$3 WHERE NOT EXISTS (SELECT 1 FROM tag_photo WHERE photo_id=$1 AND tag_id=$2)`, photoID, k, uid)
			if err != nil {
				return err
			}
			count += int(ct.RowsAffected())
		}
		return nil
	})
	return count, err
}

// DeleteTags is PhotoService.delete_tags.
func (s *Service) DeleteTags(ctx context.Context, uid, photoID int64, tagIDs []int64) (int, error) {
	count := 0
	err := s.Atomic(ctx, uid, func(tx pgx.Tx, u UserRow, f *Files) error {
		if _, err := s.OwnedPhoto(ctx, tx, photoID, uid, -1, 400); err != nil {
			return err
		}
		for _, k := range tagIDs {
			if err := s.Owned(ctx, tx, "tag", k, uid, -1, 400); err != nil {
				return err
			}
		}
		for _, k := range uniqueSorted(tagIDs) {
			if err := s.manualMember(ctx, tx, "tag", k, photoID, false); err != nil {
				return err
			}
			ct, err := tx.Exec(ctx, `DELETE FROM tag_photo WHERE user_id=$1 AND photo_id=$2 AND tag_id=$3`, uid, photoID, k)
			if err != nil {
				return err
			}
			count += int(ct.RowsAffected())
		}
		return nil
	})
	return count, err
}

// GalleryLink is PhotoService.gallery_link (1 linked/unlinked, 0 not linked, -1 already linked).
func (s *Service) GalleryLink(ctx context.Context, uid, photoID, ga int64, remove bool) (int, error) {
	result := 1
	err := s.Atomic(ctx, uid, func(tx pgx.Tx, u UserRow, f *Files) error {
		if _, err := s.OwnedPhoto(ctx, tx, photoID, uid, -1, 400); err != nil {
			return err
		}
		if err := s.Owned(ctx, tx, "gallery", ga, uid, -1, 400); err != nil {
			return err
		}
		if err := s.manualMember(ctx, tx, "gallery", ga, photoID, !remove); err != nil {
			return err
		}
		var one int
		err := tx.QueryRow(ctx, `SELECT 1 FROM gallery_photo WHERE photo_id=$1 AND ga_id=$2`, photoID, ga).Scan(&one)
		exists := err == nil
		if err != nil && err != pgx.ErrNoRows {
			return err
		}
		if remove {
			if !exists {
				result = 0
				return nil
			}
			_, err = tx.Exec(ctx, `DELETE FROM gallery_photo WHERE photo_id=$1 AND ga_id=$2`, photoID, ga)
			return err
		}
		if exists {
			result = -1
			return nil
		}
		_, err = tx.Exec(ctx, `INSERT INTO gallery_photo (photo_id, ga_id) VALUES ($1,$2)`, photoID, ga)
		return err
	})
	return result, err
}

type smartDTO struct {
	ID         int64
	Source     string
	Name       string
	CustomName *string
	Suppressed bool
	Basis      json.RawMessage
}

func (s *Service) decorate(ctx context.Context, kind string, rows []map[string]any, keyField string) ([]map[string]any, error) {
	if len(rows) == 0 {
		return []map[string]any{}, nil
	}
	col := "tag_id"
	if kind == "gallery" {
		col = "ga_id"
	}
	keys := make([]int64, len(rows))
	for i, r := range rows {
		keys[i] = r[keyField].(int64)
	}
	rs, err := s.Pool.Query(ctx, `SELECT `+col+`, item_id, source, name, custom_name, suppressed, basis FROM smart_item WHERE `+col+`=ANY($1)`, keys)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	items := map[int64]map[string]any{}
	for rs.Next() {
		var k int64
		var d smartDTO
		if err := rs.Scan(&k, &d.ID, &d.Source, &d.Name, &d.CustomName, &d.Suppressed, &d.Basis); err != nil {
			return nil, err
		}
		name := d.Name
		if d.CustomName != nil && *d.CustomName != "" {
			name = *d.CustomName
		}
		var basis any
		_ = json.Unmarshal(d.Basis, &basis)
		items[k] = map[string]any{"id": d.ID, "source": d.Source, "name": name, "suppressed": d.Suppressed, "basis": basis}
	}
	for i, r := range rows {
		if it, ok := items[keys[i]]; ok {
			r["smart"] = it
		} else {
			r["smart"] = nil
		}
	}
	return rows, rs.Err()
}

// Tags is PhotoService.tags (snake_case rows, optional smart decoration).
func (s *Service) Tags(ctx context.Context, uid, photoID int64, smart bool) (any, error) {
	if _, err := s.OwnedPhoto(ctx, s.Pool, photoID, uid, nil, 400); err != nil {
		return nil, err
	}
	rs, err := s.Pool.Query(ctx, `SELECT t.tag_id, t.tag_name FROM tag t JOIN tag_photo tp ON tp.tag_id=t.tag_id
		WHERE tp.photo_id=$1 AND t.user_id=$2 ORDER BY t.tag_id`, photoID, uid)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	rows := make([]map[string]any, 0)
	for rs.Next() {
		var id int64
		var name *string
		if err := rs.Scan(&id, &name); err != nil {
			return nil, err
		}
		rows = append(rows, map[string]any{"photo_id": photoID, "tag_id": id, "tag_name": nilStr(name)})
	}
	if smart {
		return s.decorate(ctx, "tag", rows, "tag_id")
	}
	return rows, rs.Err()
}

// Galleries is PhotoService.galleries.
func (s *Service) Galleries(ctx context.Context, uid, photoID int64, smart bool) (any, error) {
	if _, err := s.OwnedPhoto(ctx, s.Pool, photoID, uid, nil, 400); err != nil {
		return nil, err
	}
	rs, err := s.Pool.Query(ctx, `SELECT g.ga_id, g.ga_name FROM gallery g JOIN gallery_photo gp ON gp.ga_id=g.ga_id
		WHERE gp.photo_id=$1 AND g.user_id=$2 ORDER BY g.ga_id`, photoID, uid)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	rows := make([]map[string]any, 0)
	for rs.Next() {
		var id int64
		var name *string
		if err := rs.Scan(&id, &name); err != nil {
			return nil, err
		}
		rows = append(rows, map[string]any{"photo_id": photoID, "ga_id": id, "ga_name": nilStr(name)})
	}
	if smart {
		return s.decorate(ctx, "gallery", rows, "ga_id")
	}
	return rows, rs.Err()
}

// Update is PhotoService.update.
func (s *Service) Update(ctx context.Context, uid int64, data map[string]any) (int, error) {
	err := s.Atomic(ctx, uid, func(tx pgx.Tx, u UserRow, f *Files) error {
		id, _ := toInt(data["photoId"])
		if _, err := s.OwnedPhoto(ctx, tx, id, uid, -1, 400); err != nil {
			return err
		}
		if v, ok := data["photoName"]; ok && v != nil {
			name, err := ValidName(v, false)
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE photo SET photo_name=$1 WHERE photo_id=$2`, name, id); err != nil {
				return err
			}
		}
		if v, ok := data["visibility"]; ok && v != nil {
			n, ok := toInt(v)
			if !ok || n < 0 || n > 2 {
				return Bad()
			}
			if _, err := tx.Exec(ctx, `UPDATE photo SET visibility=$1 WHERE photo_id=$2`, n, id); err != nil {
				return err
			}
		}
		return nil
	})
	return 1, err
}

// toInt converts a decoded JSON value to int64 (whole numbers only, no booleans).
func toInt(v any) (int64, bool) {
	switch n := v.(type) {
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	case float64:
		if n == math.Trunc(n) {
			return int64(n), true
		}
	case int64:
		return n, true
	}
	return 0, false
}

// Move is PhotoService.move (changeNamespace).
func (s *Service) Move(ctx context.Context, uid, photoID, ns int64) (int, error) {
	err := s.Atomic(ctx, uid, func(tx pgx.Tx, u UserRow, f *Files) error {
		if _, err := s.OwnedPhoto(ctx, tx, photoID, uid, -1, 400); err != nil {
			return err
		}
		if err := s.Owned(ctx, tx, "namespace", ns, uid, -1, 400); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE photo SET ns_id=$1 WHERE photo_id=$2`, ns, photoID)
		return err
	})
	return 1, err
}

// Delete is PhotoService.delete.
func (s *Service) Delete(ctx context.Context, uid, photoID int64) (int, error) {
	err := s.Atomic(ctx, uid, func(tx pgx.Tx, u UserRow, f *Files) error {
		p, err := s.OwnedPhoto(ctx, tx, photoID, uid, -1, 400)
		if err != nil {
			return err
		}
		return s.DeleteRecord(ctx, tx, f, u, p)
	})
	return 1, err
}

// ---- file lookup ----

func (s *Service) userUUID(ctx context.Context, q DB, uid int64) (string, error) {
	var u string
	err := q.QueryRow(ctx, `SELECT uuid FROM "user" WHERE user_id=$1`, uid).Scan(&u)
	return u, err
}

// PhotoPath is PhotoService.path.
func (s *Service) PhotoPath(ctx context.Context, uid, photoID int64, thumbnail bool) (string, error) {
	p, err := s.OwnedPhoto(ctx, s.Pool, photoID, uid, nil, 400)
	if err != nil {
		return "", err
	}
	uu, err := s.userUUID(ctx, s.Pool, uid)
	if err != nil {
		return "", err
	}
	path, err := s.MediaPath(uu, p.UUID, p.Format, thumbnail)
	if err != nil {
		return "", err
	}
	if thumbnail && !isFile(path) {
		if path, err = s.MediaPath(uu, p.UUID, p.Format, false); err != nil {
			return "", err
		}
	}
	if !isFile(path) {
		return "", Err(nil, 404)
	}
	return path, nil
}

func isFile(p string) bool { return fileSize(p) > 0 || regular(p) }

// PreviewPath is PhotoService.preview_path (HEIC covers use the generated JPEG).
func (s *Service) PreviewPath(ctx context.Context, uid, photoID int64, thumbnail bool) (string, error) {
	p, err := s.OwnedPhoto(ctx, s.Pool, photoID, uid, nil, 400)
	if err != nil {
		return "", err
	}
	if !thumbnail {
		var has bool
		if err := s.Pool.QueryRow(ctx, `SELECT has_preview FROM photo_media WHERE photo_id=$1`, photoID).Scan(&has); err == nil && has {
			uu, err := s.userUUID(ctx, s.Pool, uid)
			if err != nil {
				return "", err
			}
			ap, err := s.AssetPath(uu, p.UUID, "preview")
			if err != nil {
				return "", err
			}
			if !isFile(ap) {
				return "", Err(nil, 404)
			}
			return ap, nil
		}
	}
	return s.PhotoPath(ctx, uid, photoID, thumbnail)
}

// SharedPath is PhotoService.shared_path: resolve from the database, never a client path.
func (s *Service) SharedPath(ctx context.Context, path, token string, hasPath, hasToken, thumbnail bool) (string, error) {
	parts := strings.Split(path, "/")
	if !hasPath || len(parts) != 2 {
		return "", Err(nil, 403)
	}
	userUUID, filename := parts[0], parts[1]
	i := strings.LastIndex(filename, ".")
	if i < 0 {
		return "", Err(nil, 403)
	}
	photoUUID := filename[:i]
	if thumbnail {
		if !strings.HasSuffix(photoUUID, "_thumbnail") {
			return "", Err(nil, 403)
		}
		photoUUID = strings.TrimSuffix(photoUUID, "_thumbnail")
	}
	var p *Photo
	p, err := scanPhoto(s.Pool.QueryRow(ctx, `SELECT `+photoCols+` FROM photo p WHERE p.photo_uuid=$1`, photoUUID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", Err(nil, 403)
		}
		return "", err
	}
	vis := int32(-1)
	if p.Visibility != nil {
		vis = *p.Visibility
	}
	if vis != 1 && vis != 2 || (vis == 1 && (!hasToken || p.Token != token)) {
		return "", Err(nil, 403)
	}
	uu, err := s.userUUID(ctx, s.Pool, p.UserID)
	if err != nil {
		return "", err
	}
	canonical, err := s.MediaPath(uu, p.UUID, p.Format, thumbnail)
	if err != nil {
		return "", err
	}
	if userUUID != uu || filename != baseName(canonical) {
		return "", Err(nil, 403)
	}
	if !isFile(canonical) {
		return "", Err(nil, 404)
	}
	if !thumbnail {
		var has bool
		if err := s.Pool.QueryRow(ctx, `SELECT has_preview FROM photo_media WHERE photo_id=$1`, p.ID).Scan(&has); err == nil && has {
			return s.AssetPath(uu, p.UUID, "preview")
		}
	}
	return canonical, nil
}

// ShareURL is PhotoService.share_url.
func (s *Service) ShareURL(ctx context.Context, uid, photoID int64) (string, error) {
	p, err := s.OwnedPhoto(ctx, s.Pool, photoID, uid, nil, 403)
	if err != nil {
		return "", err
	}
	suffix := strings.ToLower(p.Format)
	var endpoint string
	switch {
	case ImageFormats[suffix]:
		endpoint = "/photo/renderToken"
	case VideoFormats[suffix]:
		endpoint = "/photo/renderVToken"
	default:
		return "", Err(nil, 402)
	}
	uu, err := s.userUUID(ctx, s.Pool, uid)
	if err != nil {
		return "", err
	}
	path := uu + "/" + p.UUID + "." + p.Format
	return s.Cfg.PublicBaseURL + endpoint + "?" + urlencode(path, p.Token), nil
}

// urlencode is urlencode({"path": path, "token": token}, safe="/").
func urlencode(path, token string) string {
	enc := func(v string) string { return strings.ReplaceAll(url.QueryEscape(v), "%2F", "/") }
	return "path=" + enc(path) + "&token=" + enc(token)
}

// ExifOf is PhotoService.exif.
func (s *Service) ExifOf(ctx context.Context, uid, photoID int64) (any, error) {
	if _, err := s.OwnedPhoto(ctx, s.Pool, photoID, uid, nil, 404); err != nil {
		return nil, err
	}
	path, err := s.PhotoPath(ctx, uid, photoID, false)
	if err != nil {
		return nil, err
	}
	return PublicEXIF(s.ReadMetadata(ctx, path), path), nil
}

func regular(p string) bool { fi, err := os.Stat(p); return err == nil && fi.Mode().IsRegular() }

func baseName(p string) string { return filepath.Base(p) }
