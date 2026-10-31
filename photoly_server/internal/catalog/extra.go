package catalog

import (
	"net/http"

	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
)

func (s *svc) mapPhotos(byGallery bool) legacy.Handler {
	return func(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
		ctx := r.Context()
		var gallery *int64
		if byGallery {
			ga, _, err := legacy.ArgInt(r, "gaId", true)
			if err != nil {
				return nil, err
			}
			if err := legacy.Owned(ctx, s.a.Pool, "gallery", ga, true, uid, nil, 400); err != nil {
				return nil, err
			}
			gallery = &ga
		}
		rows, err := dbgen.New(s.a.Pool).ListMapPhotos(ctx, dbgen.ListMapPhotosParams{UserID: uid, GaID: gallery})
		if err != nil {
			return nil, err
		}
		out := []map[string]any{}
		for _, p := range rows {
			out = append(out, map[string]any{"photo_id": p.PhotoID, "photo_name": p.PhotoName, "format": p.Format, "ns_id": p.NsID, "source_id": p.SourceID, "visibility": p.Visibility, "token": p.Token,
				"photo_uuid": p.PhotoUuid, "upload_date": legacy.Day(p.UploadDate), "extra_photo_gps_lat": p.ExtraPhotoGpsLat,
				"extra_photo_gps_lon": p.ExtraPhotoGpsLon, "extra_photo_taken_date": legacy.Day(p.ExtraPhotoTakenDate)})
		}
		return legacy.Res{V: out, Code: 200}, nil
	}
}

func (s *svc) mapPlaces(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	rows, err := dbgen.New(s.a.Pool).ListMapPlaces(r.Context(), uid)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, p := range rows {
		if pl := places().nearest(&p.Lat, &p.Lon); pl != nil {
			out = append(out, map[string]any{"photoId": p.PhotoID, "place": pl.Place, "country": pl.Country})
		}
	}
	return legacy.Res{V: out, Code: 200}, nil
}
