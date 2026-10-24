package catalog

import (
	"net/http"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
)

func (s *svc) mapPhotos(byGallery bool) legacy.Handler {
	return func(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
		ctx := r.Context()
		q := `SELECT p.photo_id, p.photo_name, p.format, p.ns_id, p.token, p.photo_uuid, p.upload_date,
			e.extra_photo_gps_lat, e.extra_photo_gps_lon, e.extra_photo_taken_date
			FROM photo p JOIN photo_extra e ON e.f_photo_id=p.photo_id`
		args := []any{uid}
		if byGallery {
			ga, _, err := legacy.ArgInt(r, "gaId", true)
			if err != nil {
				return nil, err
			}
			if err := legacy.Owned(ctx, s.a.Pool, "gallery", "ga_id", ga, true, uid, nil, 400); err != nil {
				return nil, err
			}
			q += ` JOIN gallery_photo gp ON gp.photo_id=p.photo_id WHERE p.user_id=$1 AND gp.ga_id=$2`
			args = append(args, ga)
		} else {
			q += ` WHERE p.user_id=$1`
		}
		rows, err := s.a.Pool.Query(ctx, q+` ORDER BY p.photo_id`, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id int64
			var name, format, token, puuid *string
			var ns *int64
			var up, taken *time.Time
			var lat, lon *float64
			if err := rows.Scan(&id, &name, &format, &ns, &token, &puuid, &up, &lat, &lon, &taken); err != nil {
				return nil, err
			}
			out = append(out, map[string]any{"photo_id": id, "photo_name": name, "format": format, "ns_id": ns, "token": token,
				"photo_uuid": puuid, "upload_date": legacy.Day(up), "extra_photo_gps_lat": lat, "extra_photo_gps_lon": lon,
				"extra_photo_taken_date": legacy.Day(taken)})
		}
		return legacy.Res{V: out, Code: 200}, rows.Err()
	}
}

func (s *svc) mapPlaces(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	rows, err := s.a.Pool.Query(r.Context(), `SELECT p.photo_id, e.extra_photo_gps_lat, e.extra_photo_gps_lon FROM photo p
		JOIN photo_extra e ON e.f_photo_id=p.photo_id WHERE p.user_id=$1 AND e.extra_photo_gps_lat IS NOT NULL
		AND e.extra_photo_gps_lon IS NOT NULL ORDER BY p.photo_id`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var lat, lon float64
		if err := rows.Scan(&id, &lat, &lon); err != nil {
			return nil, err
		}
		if pl := places().nearest(&lat, &lon); pl != nil {
			out = append(out, map[string]any{"photoId": id, "place": pl.Place, "country": pl.Country})
		}
	}
	return legacy.Res{V: out, Code: 200}, rows.Err()
}
