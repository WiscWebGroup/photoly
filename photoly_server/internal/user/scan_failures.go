package user

import (
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	"net/http"
	"strconv"
)

func (s *svc) scanFailures(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	id, err := strconv.ParseInt(r.PathValue("sourceId"), 10, 64)
	if err != nil {
		return nil, legacy.Err("invalid_source", 400)
	}
	var owned bool
	if err = s.a.Pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM storage_source WHERE id=$1 AND user_id=$2)`, id, uid).Scan(&owned); err != nil {
		return nil, err
	}
	if !owned {
		return nil, legacy.Err("source_not_found", 404)
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	var total int64
	if err = s.a.Pool.QueryRow(r.Context(), `SELECT count(*) FROM storage_scan_failure WHERE source_id=$1`, id).Scan(&total); err != nil {
		return nil, err
	}
	rows, err := s.a.Pool.Query(r.Context(), `SELECT path,error,retryable,attempted_at FROM storage_scan_failure WHERE source_id=$1 ORDER BY path LIMIT $2 OFFSET $3`, id, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var path, code string
		var retryable bool
		var at any
		if err = rows.Scan(&path, &code, &retryable, &at); err != nil {
			return nil, err
		}
		// Raw driver errors can contain deployment paths or connection details.
		message := map[string]string{"invalid_image": "This file is not a readable image.", "image_too_many_pixels": "The image exceeds the supported pixel limit.", "large_image_unsupported": "This large image format is not supported.", "large_image_processing_failed": "The large image preview could not be generated within resource limits.", "large_image_processor_unavailable": "The server's large image processor is unavailable.", "file_changed_during_scan": "The file changed while being read.", "file_changed_during_processing": "The file changed while being processed.", "live_preview_processing_failed": "The Live Photo playback preview could not be generated."}[code]
		if message == "" {
			code = "media_processing_failed"
			message = "Could not read or process this file. It will be retried."
		}
		items = append(items, map[string]any{"path": path, "code": code, "message": message, "retryable": retryable, "attemptedAt": at})
	}
	return legacy.Res{V: map[string]any{"items": items, "total": total}, Code: 200}, rows.Err()
}
