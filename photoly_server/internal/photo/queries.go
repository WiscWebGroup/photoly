package photo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
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

func photosJSON(rows []dbgen.Photo, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, photoFromRow(r).JSON())
	}
	return out, nil
}

// allJSON runs a hand-built photoCols query (ByDate-style dynamic SQL).
func (s *Service) allJSON(ctx context.Context, sql string, args ...any) (any, error) {
	rows, err := s.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return collect(rows)
}

// DateSummary is PhotoService.date_summary.
func (s *Service) DateSummary(ctx context.Context, uid int64) (any, error) {
	rows, err := dbgen.New(s.Pool).ListDateSummary(ctx, dbgen.ListDateSummaryParams{UserID: uid, Formats: imageFormatList()})
	if err != nil {
		return nil, err
	}
	var total, undated int64
	days := make([]map[string]any, 0)
	for _, r := range rows {
		total += r.N
		if r.Day == "" {
			undated = r.N
		} else {
			days = append(days, map[string]any{"date": r.Day, "count": r.N})
		}
	}
	return map[string]any{"totalCount": total, "undatedCount": undated, "days": days}, nil
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
	arg := dbgen.ListMonthPreviewLatestParams{UserID: uid, Formats: imageFormatList(), Since: start}
	if month != "9999-12" {
		until := start.AddDate(0, 1, 0)
		arg.Until = &until
	}
	q := dbgen.New(s.Pool)
	var ids []int64
	if order == "oldest" {
		ids, err = q.ListMonthPreviewOldest(ctx, dbgen.ListMonthPreviewOldestParams(arg))
	} else {
		ids, err = q.ListMonthPreviewLatest(ctx, arg)
	}
	if ids == nil {
		ids = []int64{}
	}
	return ids, err
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
// Raw SQL on purpose: the ORDER BY direction and keyset predicate are built dynamically, which sqlc can't express.
func (s *Service) ByDate(ctx context.Context, uid int64, order string, limit int64, cursor string, day *string) (any, error) {
	if (order != "latest" && order != "oldest") || limit < 1 || limit > 200 {
		return nil, Bad()
	}
	args := []any{uid, imageFormatList()}
	where := `p.user_id=$1 AND photo_visible(p.storage_id, p.user_id) AND lower(p.format)=ANY($2)`
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
	return photosJSON(dbgen.New(s.Pool).ListPhotosByNamespace(ctx, dbgen.ListPhotosByNamespaceParams{UserID: uid, NsID: ns}))
}

// ByGallery is by_gallery.
func (s *Service) ByGallery(ctx context.Context, uid, ga int64) (any, error) {
	if err := s.Owned(ctx, s.Pool, "gallery", ga, uid, nil, 400); err != nil {
		return nil, err
	}
	return photosJSON(dbgen.New(s.Pool).ListPhotosByGallery(ctx, dbgen.ListPhotosByGalleryParams{GaID: ga, UserID: uid}))
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
	return photosJSON(dbgen.New(s.Pool).ListPhotosByTags(ctx, dbgen.ListPhotosByTagsParams{UserID: uid, TagIds: set, TagCount: int64(len(set))}))
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
	type coverRow struct {
		key, photoID int64
		format       *string
	}
	var rows []coverRow
	q := dbgen.New(s.Pool)
	switch source {
	case "folder":
		rs, err := q.ListFolderCovers(ctx, dbgen.ListFolderCoversParams{UserID: uid, Keys: keys})
		if err != nil {
			return nil, err
		}
		for _, r := range rs {
			rows = append(rows, coverRow{r.Key, r.PhotoID, r.Format})
		}
	case "gallery":
		rs, err := q.ListGalleryCovers(ctx, dbgen.ListGalleryCoversParams{UserID: uid, Keys: keys})
		if err != nil {
			return nil, err
		}
		for _, r := range rs {
			rows = append(rows, coverRow{r.Key, r.PhotoID, r.Format})
		}
	default:
		rs, err := q.ListTagCovers(ctx, dbgen.ListTagCoversParams{UserID: uid, Keys: keys})
		if err != nil {
			return nil, err
		}
		for _, r := range rs {
			rows = append(rows, coverRow{r.Key, r.PhotoID, r.Format})
		}
	}
	type cover struct {
		id    int64
		count int64
		ids   []int64
	}
	found := map[int64]*cover{}
	for _, k := range keys {
		found[k] = &cover{id: k, ids: []int64{}}
	}
	for _, r := range rows {
		c := found[r.key]
		c.count++
		if len(c.ids) < 3 && r.format != nil && ImageFormats[strings.ToLower(*r.format)] {
			c.ids = append(c.ids, r.photoID)
		}
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
	q := dbgen.New(s.Pool)
	switch option {
	case "photoName":
		return photosJSON(q.SearchPhotosByName(ctx, dbgen.SearchPhotosByNameParams{UserID: uid, Pattern: pattern}))
	case "namespace":
		return photosJSON(q.SearchPhotosByNamespace(ctx, dbgen.SearchPhotosByNamespaceParams{UserID: uid, Pattern: pattern}))
	case "gallery":
		return photosJSON(q.SearchPhotosByGallery(ctx, dbgen.SearchPhotosByGalleryParams{UserID: uid, Pattern: pattern}))
	case "tag":
		return photosJSON(q.SearchPhotosByTag(ctx, dbgen.SearchPhotosByTagParams{UserID: uid, Pattern: pattern}))
	}
	return []any{}, nil
}

// SearchSmart is smart_search.search_smart: bounded lexical candidates.
// Raw SQL on purpose: the predicate is assembled from the tokenized query.
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
		WHERE p.user_id=$1 AND photo_visible(p.storage_id, p.user_id) AND ` + score + ` > 0 ORDER BY ` + score + ` DESC, p.photo_id LIMIT 100`
	return s.allJSON(ctx, sql, args...)
}

// ---- links and smart-catalog membership ----

func (s *Service) manualMember(ctx context.Context, tx pgx.Tx, kind string, key, photoID int64, included bool) error {
	q := dbgen.New(tx)
	arg := dbgen.GetSmartItemForParams{TagID: &key}
	if kind == "gallery" {
		arg = dbgen.GetSmartItemForParams{GaID: &key}
	}
	item, err := q.GetSmartItemFor(ctx, arg)
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		return err
	}
	manual := int64(-1)
	if included {
		manual = 1
	}
	if err = q.UpsertManualSmartMember(ctx, dbgen.UpsertManualSmartMemberParams{ItemID: item.ItemID, PhotoID: photoID, Manual: manual}); err != nil {
		return err
	}
	return q.BumpSmartRevision(ctx, item.UserID)
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
			n, err := dbgen.New(tx).InsertTagPhoto(ctx, dbgen.InsertTagPhotoParams{PhotoID: photoID, TagID: k, UserID: uid})
			if err != nil {
				return err
			}
			count += int(n)
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
			n, err := dbgen.New(tx).DeleteTagPhoto(ctx, dbgen.DeleteTagPhotoParams{UserID: uid, PhotoID: photoID, TagID: k})
			if err != nil {
				return err
			}
			count += int(n)
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
		q := dbgen.New(tx)
		link := dbgen.GalleryPhotoExistsParams{PhotoID: photoID, GaID: ga}
		exists, err := q.GalleryPhotoExists(ctx, link)
		if err != nil {
			return err
		}
		if remove {
			if !exists {
				result = 0
				return nil
			}
			return q.DeleteGalleryPhoto(ctx, dbgen.DeleteGalleryPhotoParams(link))
		}
		if exists {
			result = -1
			return nil
		}
		return q.InsertGalleryPhoto(ctx, dbgen.InsertGalleryPhotoParams(link))
	})
	return result, err
}

func (s *Service) decorate(ctx context.Context, kind string, rows []map[string]any, keyField string) ([]map[string]any, error) {
	if len(rows) == 0 {
		return []map[string]any{}, nil
	}
	keys := make([]int64, len(rows))
	for i, r := range rows {
		keys[i] = r[keyField].(int64)
	}
	arg := dbgen.ListSmartItemsForParams{TagIds: keys}
	if kind == "gallery" {
		arg = dbgen.ListSmartItemsForParams{GaIds: keys}
	}
	rs, err := dbgen.New(s.Pool).ListSmartItemsFor(ctx, arg)
	if err != nil {
		return nil, err
	}
	items := map[int64]map[string]any{}
	for _, d := range rs {
		k := deref(d.TagID)
		if kind == "gallery" {
			k = deref(d.GaID)
		}
		name := d.Name
		if d.CustomName != nil && *d.CustomName != "" {
			name = *d.CustomName
		}
		var basis any
		_ = json.Unmarshal(d.Basis, &basis)
		items[k] = map[string]any{"id": d.ItemID, "source": d.Source, "name": name, "suppressed": d.Suppressed, "basis": basis}
	}
	for i, r := range rows {
		if it, ok := items[keys[i]]; ok {
			r["smart"] = it
		} else {
			r["smart"] = nil
		}
	}
	return rows, nil
}

// Tags is PhotoService.tags (snake_case rows, optional smart decoration).
func (s *Service) Tags(ctx context.Context, uid, photoID int64, smart bool) (any, error) {
	if _, err := s.OwnedPhoto(ctx, s.Pool, photoID, uid, nil, 400); err != nil {
		return nil, err
	}
	tags, err := dbgen.New(s.Pool).ListPhotoTags(ctx, dbgen.ListPhotoTagsParams{PhotoID: photoID, UserID: uid})
	if err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0)
	for _, t := range tags {
		rows = append(rows, map[string]any{"photo_id": photoID, "tag_id": t.TagID, "tag_name": t.TagName})
	}
	if smart {
		return s.decorate(ctx, "tag", rows, "tag_id")
	}
	return rows, nil
}

// Galleries is PhotoService.galleries.
func (s *Service) Galleries(ctx context.Context, uid, photoID int64, smart bool) (any, error) {
	if _, err := s.OwnedPhoto(ctx, s.Pool, photoID, uid, nil, 400); err != nil {
		return nil, err
	}
	galleries, err := dbgen.New(s.Pool).ListPhotoGalleries(ctx, dbgen.ListPhotoGalleriesParams{PhotoID: photoID, UserID: uid})
	if err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0)
	for _, g := range galleries {
		rows = append(rows, map[string]any{"photo_id": photoID, "ga_id": g.GaID, "ga_name": nilStr(g.GaName)})
	}
	if smart {
		return s.decorate(ctx, "gallery", rows, "ga_id")
	}
	return rows, nil
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
			if err = dbgen.New(tx).RenamePhoto(ctx, dbgen.RenamePhotoParams{Name: name, PhotoID: id}); err != nil {
				return err
			}
		}
		if v, ok := data["visibility"]; ok && v != nil {
			n, ok := toInt(v)
			if !ok || n < 0 || n > 2 {
				return Bad()
			}
			if err := dbgen.New(tx).SetPhotoVisibility(ctx, dbgen.SetPhotoVisibilityParams{Visibility: n, PhotoID: id}); err != nil {
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
		return dbgen.New(tx).MovePhoto(ctx, dbgen.MovePhotoParams{NsID: ns, PhotoID: photoID})
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
	u, err := dbgen.New(q).GetUserUUID(ctx, uid)
	return deref(u), err
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

func isFile(p string) bool { return storage.Fetch(p) && (fileSize(p) > 0 || regular(p)) }

// PreviewPath is PhotoService.preview_path (HEIC covers use the generated JPEG).
func (s *Service) PreviewPath(ctx context.Context, uid, photoID int64, thumbnail bool) (string, error) {
	p, err := s.OwnedPhoto(ctx, s.Pool, photoID, uid, nil, 400)
	if err != nil {
		return "", err
	}
	if !thumbnail {
		if has, err := dbgen.New(s.Pool).GetPhotoHasPreview(ctx, photoID); err == nil && has {
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
	row, err := dbgen.New(s.Pool).GetPhotoByUUID(ctx, photoUUID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", Err(nil, 403)
		}
		return "", err
	}
	p := photoFromRow(row)
	vis := int64(-1)
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
		if has, err := dbgen.New(s.Pool).GetPhotoHasPreview(ctx, p.ID); err == nil && has {
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
