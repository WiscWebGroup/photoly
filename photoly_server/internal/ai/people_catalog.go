package ai

import (
	"context"
	"strconv"

	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/jackc/pgx/v5"
)

type Person struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Hidden      bool   `json:"hidden"`
	Count       int64  `json:"count"`
	CoverFaceID *int64 `json:"coverFaceId"`
	MergedInto  *int64 `json:"mergedInto,omitempty"`
}
type PersonFace struct {
	ID        int64     `json:"id"`
	PhotoID   int64     `json:"photoId"`
	PersonID  int64     `json:"personId"`
	Name      string    `json:"name"`
	Box       []float32 `json:"box"`
	Dismissed bool      `json:"dismissed"`
}

func peoplePage(q map[string]string) (int64, int) {
	after, _ := strconv.ParseInt(q["after"], 10, 64)
	limit, _ := strconv.Atoi(q["limit"])
	if limit < 1 {
		limit = 50
	}
	return max(0, after), min(limit, 100)
}
func (p *People) canonicalPerson(ctx context.Context, q Q, uid, id int64) (int64, error) {
	for i := 0; i < 100; i++ {
		var next *int64
		if err := q.QueryRow(ctx, `SELECT merged_into FROM people_person WHERE person_id=$1 AND user_id=$2`, id, uid).Scan(&next); err != nil {
			return 0, peopleNotFound(err)
		}
		if next == nil {
			return id, nil
		}
		id = *next
	}
	return 0, fail("invalid_person")
}
func (p *People) list(ctx context.Context, uid int64, query map[string]string) (any, error) {
	if err := p.readable(ctx, p.pool, uid); err != nil {
		return nil, err
	}
	after, limit := peoplePage(query)
	mode := query["mode"]
	if mode == "" {
		mode = "people"
	}
	if mode != "people" && mode != "pending" && mode != "hidden" && mode != "all" {
		return nil, fail("invalid_mode")
	}
	gallery, _ := strconv.ParseInt(query["galleryId"], 10, 64)
	rows, err := p.pool.Query(ctx, `SELECT p.person_id,p.name,p.hidden,count(DISTINCT f.photo_id),
 COALESCE(min(f.face_id) FILTER(WHERE f.face_id=p.cover_face_id),min(f.face_id))
 FROM people_person p LEFT JOIN people_face f ON f.person_id=p.person_id AND NOT f.dismissed AND EXISTS(SELECT 1 FROM photo ph WHERE ph.photo_id=f.photo_id AND photo_visible(ph.storage_id,ph.user_id))
 WHERE p.user_id=$1 AND p.person_id>$2 AND p.merged_into IS NULL AND p.hidden=($3='hidden') AND ($4='' OR p.name ILIKE '%'||$4||'%')
 AND ($5::bigint=0 OR EXISTS(SELECT 1 FROM people_face gf JOIN gallery_photo gp ON gp.photo_id=gf.photo_id JOIN gallery g ON g.ga_id=gp.ga_id JOIN photo ph ON ph.photo_id=gf.photo_id WHERE gf.person_id=p.person_id AND NOT gf.dismissed AND g.ga_id=$5 AND g.user_id=$1 AND photo_visible(ph.storage_id,ph.user_id)))
 GROUP BY p.person_id HAVING ($3 IN ('all','hidden') OR ($3='pending' AND p.name='' AND count(DISTINCT f.photo_id)=1) OR ($3='people' AND (p.name<>'' OR count(DISTINCT f.photo_id)>=2)))
 ORDER BY p.person_id LIMIT $6`, uid, after, mode, query["q"], gallery, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Person{}
	for rows.Next() {
		var person Person
		if err = rows.Scan(&person.ID, &person.Name, &person.Hidden, &person.Count, &person.CoverFaceID); err != nil {
			return nil, err
		}
		items = append(items, person)
	}
	var next *int64
	if len(items) > limit {
		items = items[:limit]
		id := items[len(items)-1].ID
		next = &id
	}
	return map[string]any{"items": items, "nextAfter": next}, rows.Err()
}
func (p *People) detail(ctx context.Context, uid, id int64) (any, error) {
	if err := p.readable(ctx, p.pool, uid); err != nil {
		return nil, err
	}
	id, err := p.canonicalPerson(ctx, p.pool, uid, id)
	if err != nil {
		return nil, err
	}
	var person Person
	err = p.pool.QueryRow(ctx, `SELECT p.person_id,p.name,p.hidden,count(DISTINCT f.photo_id),COALESCE(min(f.face_id) FILTER(WHERE f.face_id=p.cover_face_id),min(f.face_id)) FROM people_person p LEFT JOIN people_face f ON f.person_id=p.person_id AND NOT f.dismissed AND EXISTS(SELECT 1 FROM photo ph WHERE ph.photo_id=f.photo_id AND photo_visible(ph.storage_id,ph.user_id)) WHERE p.person_id=$1 AND p.user_id=$2 GROUP BY p.person_id`, id, uid).Scan(&person.ID, &person.Name, &person.Hidden, &person.Count, &person.CoverFaceID)
	return person, peopleNotFound(err)
}
func (p *People) photos(ctx context.Context, uid, id int64, query map[string]string) (any, error) {
	if err := p.readable(ctx, p.pool, uid); err != nil {
		return nil, err
	}
	id, err := p.canonicalPerson(ctx, p.pool, uid, id)
	if err != nil {
		return nil, err
	}
	after, limit := peoplePage(query)
	rows, err := dbgen.New(p.pool).ListPeoplePhotos(ctx, dbgen.ListPeoplePhotosParams{UserID: uid, PersonID: id, AfterID: after, PageSize: int64(limit + 1)})
	if err != nil {
		return nil, err
	}
	var next *int64
	if len(rows) > limit {
		rows = rows[:limit]
		pid := rows[limit-1].PhotoID
		next = &pid
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"photoId": row.PhotoID, "photoName": row.PhotoName, "format": row.Format, "nsId": row.NsID, "sourceId": row.SourceID})
	}
	return map[string]any{"items": items, "nextAfter": next, "personId": id}, nil
}
func (p *People) faces(ctx context.Context, uid, personID, photoID int64, query map[string]string) (any, error) {
	if err := p.readable(ctx, p.pool, uid); err != nil {
		return nil, err
	}
	after, limit := peoplePage(query)
	state := "ready"
	var message *string
	if photoID > 0 {
		var exists bool
		if err := p.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM photo WHERE photo_id=$1 AND user_id=$2 AND photo_visible(storage_id,user_id))`, photoID, uid).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, fail("not_found", 404)
		}
		err := p.pool.QueryRow(ctx, `SELECT status,error FROM people_media WHERE photo_id=$1 AND user_id=$2`, photoID, uid).Scan(&state, &message)
		if err == pgx.ErrNoRows {
			state = "pending"
		} else if err != nil {
			return nil, err
		}
	}
	if personID > 0 {
		var err error
		personID, err = p.canonicalPerson(ctx, p.pool, uid, personID)
		if err != nil {
			return nil, err
		}
	}
	rows, err := p.pool.Query(ctx, `SELECT f.face_id,f.photo_id,f.person_id,p.name,f.box,f.dismissed FROM people_face f JOIN people_person p ON p.person_id=f.person_id JOIN photo ph ON ph.photo_id=f.photo_id WHERE f.user_id=$1 AND f.face_id>$2 AND ($3::bigint=0 OR f.person_id=$3) AND ($4::bigint=0 OR f.photo_id=$4) AND ($3::bigint>0 OR NOT p.hidden) AND NOT f.dismissed AND photo_visible(ph.storage_id,ph.user_id) ORDER BY f.face_id LIMIT $5`, uid, after, personID, photoID, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []PersonFace{}
	for rows.Next() {
		var f PersonFace
		if err = rows.Scan(&f.ID, &f.PhotoID, &f.PersonID, &f.Name, &f.Box, &f.Dismissed); err != nil {
			return nil, err
		}
		items = append(items, f)
	}
	var next *int64
	if len(items) > limit {
		items = items[:limit]
		id := items[len(items)-1].ID
		next = &id
	}
	return map[string]any{"items": items, "nextAfter": next, "status": state, "error": message}, rows.Err()
}
func (p *People) editPerson(ctx context.Context, uid, id int64, d map[string]any) (any, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = p.mutable(ctx, tx, uid); err != nil {
		return nil, err
	}
	id, err = p.canonicalPerson(ctx, tx, uid, id)
	if err != nil {
		return nil, err
	}
	for key, v := range d {
		switch key {
		case "name":
			name, e := cleanPersonName(v)
			if e != nil {
				return nil, e
			}
			_, err = tx.Exec(ctx, `UPDATE people_person SET name=$2,updated_at=now() WHERE person_id=$1`, id, name)
		case "hidden":
			if !isBool(v) {
				return nil, fail("invalid_settings")
			}
			_, err = tx.Exec(ctx, `UPDATE people_person SET hidden=$2,updated_at=now() WHERE person_id=$1`, id, v)
		case "coverFaceId":
			faceID, ok := toInt(v)
			if !ok {
				return nil, fail("invalid_face")
			}
			var exists bool
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM people_face f JOIN photo ph ON ph.photo_id=f.photo_id WHERE f.face_id=$1 AND f.person_id=$2 AND f.user_id=$3 AND NOT f.dismissed AND photo_visible(ph.storage_id,ph.user_id))`, faceID, id, uid).Scan(&exists)
			if err != nil {
				return nil, err
			}
			if !exists {
				return nil, fail("not_found", 404)
			}
			_, err = tx.Exec(ctx, `UPDATE people_person SET cover_face_id=$2 WHERE person_id=$1`, id, faceID)
		default:
			return nil, fail("invalid_settings")
		}
		if err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return p.detail(ctx, uid, id)
}
func (p *People) merge(ctx context.Context, uid, target int64, ids []int64) (any, error) {
	if len(ids) < 1 || len(ids) > 100 {
		return nil, fail("invalid_merge")
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = p.mutable(ctx, tx, uid); err != nil {
		return nil, err
	}
	target, err = p.canonicalPerson(ctx, tx, uid, target)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		id, err = p.canonicalPerson(ctx, tx, uid, id)
		if err != nil {
			return nil, err
		}
		if id == target {
			continue
		}
		if _, err = tx.Exec(ctx, `INSERT INTO people_exclusion(face_id,person_id) SELECT face_id,$2 FROM people_exclusion WHERE person_id=$1 ON CONFLICT DO NOTHING`, id, target); err != nil {
			return nil, err
		}
		if _, err = tx.Exec(ctx, `UPDATE people_face SET person_id=$2,manual=true WHERE person_id=$1 AND user_id=$3`, id, target, uid); err != nil {
			return nil, err
		}
		if _, err = tx.Exec(ctx, `UPDATE people_person SET merged_into=$2,updated_at=now() WHERE user_id=$3 AND (person_id=$1 OR merged_into=$1)`, id, target, uid); err != nil {
			return nil, err
		}
	}
	// The explicit merge overrides exclusions for faces the user just put in the target.
	if _, err = tx.Exec(ctx, `DELETE FROM people_exclusion x USING people_face f WHERE x.face_id=f.face_id AND x.person_id=f.person_id AND f.person_id=$1 AND f.user_id=$2`, target, uid); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return p.detail(ctx, uid, target)
}
func (p *People) editFace(ctx context.Context, uid, id int64, d map[string]any) (any, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = p.mutable(ctx, tx, uid); err != nil {
		return nil, err
	}
	var old int64
	err = tx.QueryRow(ctx, `SELECT f.person_id FROM people_face f JOIN photo ph ON ph.photo_id=f.photo_id WHERE f.face_id=$1 AND f.user_id=$2 AND photo_visible(ph.storage_id,ph.user_id) FOR UPDATE OF f`, id, uid).Scan(&old)
	if err != nil {
		return nil, peopleNotFound(err)
	}
	target := old
	for k := range d {
		if k != "personId" && k != "name" && k != "dismissed" {
			return nil, fail("invalid_face_edit")
		}
	}
	if v, ok := d["dismissed"]; ok {
		if !isBool(v) {
			return nil, fail("invalid_face_edit")
		}
		_, err = tx.Exec(ctx, `UPDATE people_face SET dismissed=$2,manual=true WHERE face_id=$1`, id, v)
		if err != nil {
			return nil, err
		}
	}
	if v, ok := d["personId"]; ok {
		target, ok = toInt(v)
		if !ok || target < 0 {
			return nil, fail("invalid_person")
		}
		if target == 0 {
			name := ""
			if v, ok := d["name"]; ok {
				name, err = cleanPersonName(v)
				if err != nil {
					return nil, err
				}
			}
			err = tx.QueryRow(ctx, `INSERT INTO people_person(user_id,name) VALUES($1,$2) RETURNING person_id`, uid, name).Scan(&target)
		} else {
			target, err = p.canonicalPerson(ctx, tx, uid, target)
		}
		if err != nil {
			return nil, err
		}
		if target != old {
			_, err = tx.Exec(ctx, `INSERT INTO people_exclusion(face_id,person_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, id, old)
			if err != nil {
				return nil, err
			}
		}
		if _, err = tx.Exec(ctx, `DELETE FROM people_exclusion WHERE face_id=$1 AND person_id=$2`, id, target); err != nil {
			return nil, err
		}
		if _, err = tx.Exec(ctx, `UPDATE people_face SET person_id=$2,manual=true,dismissed=false WHERE face_id=$1`, id, target); err != nil {
			return nil, err
		}
	}
	return map[string]int64{"personId": target}, tx.Commit(ctx)
}
