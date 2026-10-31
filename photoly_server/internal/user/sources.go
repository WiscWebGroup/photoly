package user

import (
	"net/http"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
)

func (s *svc) registerSources() {
	s.registerWebDAV()
	s.a.Mux.HandleFunc("GET /user/storage/sources/{sourceId}/failures", legacy.Opt{Auth: "header"}.Wrap(s.a, s.scanFailures))
	opt := legacy.Opt{Auth: "header"}
	s.a.Mux.HandleFunc("GET /user/storage/sources", opt.Wrap(s.a, s.sources))
	s.a.Mux.HandleFunc("GET /user/storage/s3", opt.Wrap(s.a, s.s3Settings))
	s.a.Mux.HandleFunc("POST /user/storage/s3", opt.Wrap(s.a, s.saveS3))
	s.a.Mux.HandleFunc("PATCH /user/storage/s3/{sourceId}", opt.Wrap(s.a, s.saveS3))
	s.a.Mux.HandleFunc("DELETE /user/storage/s3/{sourceId}", opt.Wrap(s.a, s.deleteS3))
	s.a.Mux.HandleFunc("PUT /user/storage/s3", opt.Wrap(s.a, s.saveS3))
	s.a.Mux.HandleFunc("POST /user/storage/s3/test", opt.Wrap(s.a, s.testS3))
}
func (s *svc) sources(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	rows, err := s.a.Pool.Query(r.Context(), `SELECT id,kind,enabled,revision,scan_status,scan_error,scanned_at,display_name,managed_by,scan_interval,scan_completed,scan_total,scan_retries,scan_phase,scan_discovered,scan_waiting,scan_unchanged,EXISTS(SELECT 1 FROM storage_scan_job j WHERE j.source_id=storage_source.id AND j.state='queued'),(SELECT id FROM storage_scan_job j WHERE j.source_id=storage_source.id ORDER BY id DESC LIMIT 1)  ,(SELECT count(*) FROM storage_scan_failure f WHERE f.source_id=storage_source.id AND f.retryable),(SELECT count(*) FROM storage_scan_failure f WHERE f.source_id=storage_source.id AND NOT f.retryable) FROM storage_source WHERE user_id=$1 ORDER BY storage_source.id`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, rev int64
		var kind, status, name, managed string
		var interval int
		var completed, total, retries, automatic, blocked int64
		var enabled, queued bool
		var phase string
		var discovered, waiting, unchanged int64
		var jobID *int64
		var failure *string
		var at *time.Time
		if err = rows.Scan(&id, &kind, &enabled, &rev, &status, &failure, &at, &name, &managed, &interval, &completed, &total, &retries, &phase, &discovered, &waiting, &unchanged, &queued, &jobID, &automatic, &blocked); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"sourceId": id, "sourceType": kind, "enabled": enabled, "revision": rev, "scanStatus": status, "scanError": failure, "scannedAt": at, "name": name, "managedBy": managed, "scanInterval": interval, "scanCompleted": completed, "scanTotal": total, "scanRetries": retries, "scanAutomaticRetries": automatic, "scanBlocked": blocked, "scanPhase": phase, "scanDiscovered": discovered, "scanWaiting": waiting, "scanUnchanged": unchanged, "scanQueued": queued, "scanJobId": jobID, "capabilities": map[string]bool{"write": enabled, "scan": kind != "s3", "detach": managed != "system"}})
	}
	return legacy.Res{V: out, Code: 200}, rows.Err()
}
