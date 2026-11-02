package photo

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/jackc/pgx/v5"
)

type mediaNote struct {
	ID              int64     `json:"id"`
	Body            string    `json:"body"`
	X               *float64  `json:"x"`
	Y               *float64  `json:"y"`
	ContentVersion  *int64    `json:"contentVersion"`
	Revision        int64     `json:"revision"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	NeedsReposition bool      `json:"needsReposition"`
}

func notePosition(d map[string]any) (*float64, *float64, error) {
	if d["x"] == nil && d["y"] == nil {
		return nil, nil, nil
	}
	parse := func(v any) (float64, error) { return strconv.ParseFloat(strings.TrimSpace(stringValue(v)), 64) }
	x, e := parse(d["x"])
	if e != nil {
		return nil, nil, Bad()
	}
	y, e := parse(d["y"])
	if e != nil || math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) || x < 0 || x > 1 || y < 0 || y > 1 {
		return nil, nil, Bad()
	}
	return &x, &y, nil
}
func stringValue(v any) string {
	switch n := v.(type) {
	case json.Number:
		return string(n)
	case string:
		return n
	}
	return ""
}
func (s *Service) registerNotes() {
	s.app.Mux.HandleFunc("GET /photo/notes", s.privateJSONHandler(func(r *http.Request, uid int64) (any, error) {
		id, e := qint(r, "photoId")
		if e != nil {
			return nil, e
		}
		p, e := s.OwnedPhoto(r.Context(), s.Pool, id, uid, nil, 404)
		if e != nil {
			return nil, e
		}
		rows, e := dbgen.New(s.Pool).ListMediaNotes(r.Context(), dbgen.ListMediaNotesParams{PhotoID: id, UserID: uid})
		if e != nil {
			return nil, e
		}
		notes := []mediaNote{}
		for _, row := range rows {
			n := mediaNote{ID: row.NoteID, Body: row.Body, X: row.X, Y: row.Y, ContentVersion: row.ContentVersion, Revision: row.Revision, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
			n.NeedsReposition = n.ContentVersion != nil && *n.ContentVersion != p.ContentVersion
			notes = append(notes, n)
		}
		return map[string]any{"items": notes, "contentVersion": p.ContentVersion}, nil
	}))
	for _, method := range []string{"POST", "PATCH", "DELETE"} {
		method := method
		pattern := method + " /photo/notes"
		if method != "POST" {
			pattern += "/{noteId}"
		}
		s.app.Mux.HandleFunc(pattern, s.privateJSONHandler(func(r *http.Request, uid int64) (any, error) {
			r.Body = http.MaxBytesReader(nil, r.Body, 500000)
			d, e := body(r)
			if e != nil {
				return nil, e
			}
			pid, ok := intOf(d["photoId"])
			if !ok {
				return nil, Bad()
			}
			tx, e := s.Pool.Begin(r.Context())
			if e != nil {
				return nil, e
			}
			defer tx.Rollback(r.Context())
			// Lock the media so replacement/deletion cannot race validation or anchor writes.
			var version int64
			var format string
			e = tx.QueryRow(r.Context(), `SELECT content_version,coalesce(format,'') FROM photo WHERE photo_id=$1 AND user_id=$2 FOR UPDATE`, pid, uid).Scan(&version, &format)
			if e == pgx.ErrNoRows {
				return nil, Err(nil, 404)
			}
			if e != nil {
				return nil, e
			}
			nid, _ := strconv.ParseInt(r.PathValue("noteId"), 10, 64)
			rev, _ := intOf(d["revision"])
			if method == "DELETE" {
				tag, e := tx.Exec(r.Context(), `DELETE FROM media_note WHERE note_id=$1 AND photo_id=$2 AND user_id=$3 AND revision=$4`, nid, pid, uid, rev)
				if e != nil {
					return nil, e
				}
				if tag.RowsAffected() == 0 {
					return nil, Err("note_conflict", 409)
				}
			} else {
				text, ok := d["body"].(string)
				if !ok || strings.TrimSpace(text) == "" || utf8.RuneCountInString(text) > 100000 {
					return nil, Bad()
				}
				x, y, e := notePosition(d)
				if e != nil {
					return nil, e
				}
				var anchor *int64
				if x != nil {
					if VideoFormats[strings.ToLower(format)] {
						return nil, Bad()
					}
					v, ok := intOf(d["contentVersion"])
					if !ok {
						return nil, Bad()
					}
					anchor = &v
					if v != version {
						// Editing a stale note's text preserves its old anchor, but cannot create/move one.
						var oldX, oldY float64
						var oldV int64
						if method != "PATCH" {
							return nil, Err("media_changed", 409)
						}
						e = tx.QueryRow(r.Context(), `SELECT x,y,content_version FROM media_note WHERE note_id=$1 AND photo_id=$2 AND user_id=$3`, nid, pid, uid).Scan(&oldX, &oldY, &oldV)
						if e != nil || oldV != v || oldX != *x || oldY != *y {
							return nil, Err("media_changed", 409)
						}
					}
				}
				if method == "POST" {
					e = tx.QueryRow(r.Context(), `INSERT INTO media_note(photo_id,user_id,body,x,y,content_version) VALUES($1,$2,$3,$4,$5,$6) RETURNING note_id`, pid, uid, text, x, y, anchor).Scan(&nid)
				} else {
					var tagRows int64
					tag, err := tx.Exec(r.Context(), `UPDATE media_note SET body=$5,x=$6,y=$7,content_version=$8,revision=revision+1,updated_at=now() WHERE note_id=$1 AND photo_id=$2 AND user_id=$3 AND revision=$4`, nid, pid, uid, rev, text, x, y, anchor)
					e = err
					tagRows = tag.RowsAffected()
					if e == nil && tagRows == 0 {
						return nil, Err("note_conflict", 409)
					}
				}
				if e != nil {
					return nil, e
				}
			}
			if e = tx.Commit(r.Context()); e != nil {
				return nil, e
			}
			return map[string]any{"id": nid}, nil
		}))
	}
}

func (s *Service) privateJSONHandler(fn func(*http.Request, int64) (any, error)) http.HandlerFunc {
	h := s.JSONHandler(fn)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		h(w, r)
	}
}
