package ai

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

type PeopleJob struct {
	ID          int64  `json:"id"`
	Token       string `json:"token"`
	PhotoID     int64  `json:"photoId"`
	Version     int32  `json:"version"`
	Fingerprint string `json:"fingerprint"`
	Generation  int64  `json:"generation"`
	Placement   string `json:"placement"`
	Status      string `json:"status"`
}

func (p *People) discover(ctx context.Context, uid int64) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	pref, _, err := p.preference(ctx, tx, uid, true)
	if err != nil {
		return err
	}
	if pref.Reason != "" {
		return nil
	}
	var after int64
	if err = tx.QueryRow(ctx, `SELECT discovery_after FROM people_preference WHERE user_id=$1`, uid).Scan(&after); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT photo_id,content_version FROM photo WHERE user_id=$1 AND photo_id>$2 AND photo_visible(storage_id,user_id) AND lower(format)=ANY($3) AND NOT EXISTS(SELECT 1 FROM people_media m WHERE m.photo_id=photo.photo_id) ORDER BY photo_id LIMIT 100`, uid, after, formats(false))
	if err != nil {
		return err
	}
	type item struct {
		id int64
		v  int32
	}
	items := []item{}
	for rows.Next() {
		var it item
		if err = rows.Scan(&it.id, &it.v); err != nil {
			rows.Close()
			return err
		}
		items = append(items, it)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, it := range items {
		_, err = tx.Exec(ctx, `INSERT INTO people_media(photo_id,user_id,content_version,fingerprint,generation) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, it.id, uid, it.v, faceFingerprint, pref.Generation)
		if err != nil {
			return err
		}
		key := fmt.Sprintf("people:%d:%d:%s:%d", it.id, it.v, faceFingerprint, pref.Generation)
		_, err = tx.Exec(ctx, `INSERT INTO ai_job(kind,photo_id,content_version,payload,dedup_key) SELECT 'people',$1,$2,jsonb_build_object('user_id',$3::bigint,'generation',$4::bigint,'fingerprint',$5::text),$6 FROM people_media WHERE photo_id=$1 AND status='pending' ON CONFLICT(dedup_key) WHERE status IN ('queued','running') DO NOTHING`, it.id, it.v, uid, pref.Generation, faceFingerprint, key)
		if err != nil {
			return err
		}
		after = it.id
	}
	if len(items) < 100 {
		after = 0
	}
	_, err = tx.Exec(ctx, `UPDATE people_preference SET discovery_after=$2 WHERE user_id=$1`, uid, after)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p *People) claim(ctx context.Context, uid int64, placement string) (*PeopleJob, error) {
	if err := p.discover(ctx, uid); err != nil {
		return nil, err
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	// Share the durable storage budget with image indexing; serialize admissions.
	if _, err = control(ctx, tx, "global", true); err != nil {
		return nil, err
	}
	pref, cfg, err := p.preference(ctx, tx, uid, true)
	if err != nil {
		return nil, err
	}
	if pref.Reason != "" || pref.Placement != placement {
		return nil, nil
	}
	if !p.policy.Decide(SystemUsage(p.root).toMap(), cfg).Allowed {
		return nil, nil
	}
	_, err = tx.Exec(ctx, `UPDATE ai_job SET status=CASE WHEN attempts>=3 THEN 'failed' ELSE 'queued' END,lease_token=NULL,reserved_bytes=0 WHERE kind='people' AND payload->>'user_id'=$1 AND status='running' AND lease_expires_at<now()`, fmt.Sprint(uid))
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE people_media m SET status='failed',error='Repeated worker interruption' FROM ai_job j WHERE m.photo_id=j.photo_id AND m.user_id=$1 AND j.kind='people' AND j.status='failed' AND m.status='processing'`, uid)
	if err != nil {
		return nil, err
	}
	job := &PeopleJob{Fingerprint: faceFingerprint, Generation: pref.Generation, Placement: placement, Token: newUUID(), Status: "running"}
	err = tx.QueryRow(ctx, `SELECT j.job_id,j.photo_id,j.content_version FROM ai_job j JOIN photo ph ON ph.photo_id=j.photo_id WHERE j.kind='people' AND j.payload->>'user_id'=$1 AND j.status='queued' AND j.next_run_at<=now() AND ph.content_version=j.content_version AND photo_visible(ph.storage_id,ph.user_id) AND j.payload->>'fingerprint'=$2 AND (j.payload->>'generation')::bigint=$3 ORDER BY j.job_id FOR UPDATE OF j SKIP LOCKED LIMIT 1`, fmt.Sprint(uid), faceFingerprint, pref.Generation).Scan(&job.ID, &job.PhotoID, &job.Version)
	if err == pgx.ErrNoRows {
		return nil, tx.Commit(ctx) // persist expired leases, including exhausted failures
	}
	if err != nil {
		return nil, err
	}
	reserved := int64(faceMaxCount) * faceStorageCharge
	used, e := usage(ctx, tx)
	if e != nil {
		return nil, e
	}
	if max(used.Estimated+used.Reserved+reserved, used.Physical) > cfg.Int("quota_bytes") {
		return nil, tx.Commit(ctx)
	}
	if placement == "server" {
		if _, err = control(ctx, tx, "background_throttle", true); err != nil {
			return nil, err
		}
		allowed, e := admit(ctx, tx, cfg)
		if e != nil || !allowed {
			return nil, e
		}
	}
	_, err = tx.Exec(ctx, `UPDATE ai_job SET status='running',attempts=attempts+1,lease_token=$2,lease_owner=$3::text,lease_expires_at=now()+interval '120 seconds',heartbeat_at=now(),reserved_bytes=$4,payload=payload||jsonb_build_object('placement',$3::text) WHERE job_id=$1`, job.ID, job.Token, placement, reserved)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE people_media SET status='processing',error=NULL,updated_at=now() WHERE photo_id=$1`, job.PhotoID)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return job, nil
}
func (p *People) checkJob(ctx context.Context, q Q, uid int64, job PeopleJob, completed bool) (PeopleJob, error) {
	var current PeopleJob
	var alive bool
	err := q.QueryRow(ctx, `SELECT j.job_id,j.photo_id,j.content_version,COALESCE(j.lease_token,''),j.status,j.payload->>'fingerprint',(j.payload->>'generation')::bigint,j.payload->>'placement',COALESCE(j.lease_expires_at>now(),false)
 FROM ai_job j JOIN photo ph ON ph.photo_id=j.photo_id JOIN people_preference pref ON pref.user_id=ph.user_id
 WHERE j.job_id=$1 AND ph.user_id=$2 AND j.kind='people' AND ph.content_version=j.content_version AND photo_visible(ph.storage_id,ph.user_id) AND (j.payload->>'generation')::bigint=pref.generation FOR UPDATE OF j FOR SHARE OF ph`, job.ID, uid).Scan(&current.ID, &current.PhotoID, &current.Version, &current.Token, &current.Status, &current.Fingerprint, &current.Generation, &current.Placement, &alive)
	if err != nil {
		return current, peopleNotFound(err)
	}
	if current.Token != job.Token || current.PhotoID != job.PhotoID || current.Version != job.Version || current.Fingerprint != faceFingerprint || job.Fingerprint != faceFingerprint || current.Generation != job.Generation || current.Placement != job.Placement {
		return current, fail("stale_face_job", 409)
	}
	if completed && current.Status == "succeeded" {
		return current, nil
	}
	if current.Status != "running" || !alive {
		return current, fail("stale_face_job", 409)
	}
	// Admin placement can change without touching this user's preference row.
	cfg, err := loadSettings(ctx, q)
	if err != nil {
		return current, err
	}
	var choice string
	if err = q.QueryRow(ctx, `SELECT placement FROM people_preference WHERE user_id=$1`, uid).Scan(&choice); err != nil {
		return current, err
	}
	if effectivePlacement(cfg.Str("placement_people"), choice) != current.Placement {
		return current, fail("placement_changed", 409)
	}
	return current, nil
}
func (p *People) renewJob(ctx context.Context, uid int64, job PeopleJob) (any, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	pref, _, err := p.preference(ctx, tx, uid, true)
	if err != nil {
		return nil, err
	}
	if pref.Reason != "" {
		return nil, fail(pref.Reason, 409)
	}
	if _, err = p.checkJob(ctx, tx, uid, job, false); err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE ai_job SET lease_expires_at=now()+interval '120 seconds',heartbeat_at=now() WHERE job_id=$1`, job.ID)
	if err != nil {
		return nil, err
	}
	return map[string]bool{"renewed": true}, tx.Commit(ctx)
}
func (p *People) failJob(ctx context.Context, uid int64, job PeopleJob, message string) (any, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	pref, _, err := p.preference(ctx, tx, uid, true)
	if err != nil {
		return nil, err
	}
	if pref.Reason != "" {
		return nil, fail(pref.Reason, 409)
	}
	if _, err = p.checkJob(ctx, tx, uid, job, false); err != nil {
		return nil, err
	}
	if len(message) > 240 {
		message = message[:240]
	}
	var state string
	err = tx.QueryRow(ctx, `UPDATE ai_job SET status=CASE WHEN attempts>=3 THEN 'failed' ELSE 'queued' END,next_run_at=now()+interval '30 seconds',result=jsonb_build_object('error',$2::text),reserved_bytes=0,lease_token=NULL WHERE job_id=$1 RETURNING status`, job.ID, message).Scan(&state)
	if err != nil {
		return nil, err
	}
	if state == "queued" {
		state = "pending"
	}
	_, err = tx.Exec(ctx, `UPDATE people_media SET status=$2,error=$3,updated_at=now() WHERE photo_id=$1`, job.PhotoID, state, message)
	if err != nil {
		return nil, err
	}
	return map[string]string{"status": state}, tx.Commit(ctx)
}
func (p *People) image(ctx context.Context, uid int64, job PeopleJob) (FaceImage, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return FaceImage{}, err
	}
	defer tx.Rollback(ctx)
	pref, _, err := p.preference(ctx, tx, uid, true)
	if err != nil {
		return FaceImage{}, err
	}
	if pref.Reason != "" {
		return FaceImage{}, fail(pref.Reason, 409)
	}
	if _, err = p.checkJob(ctx, tx, uid, job, false); err != nil {
		return FaceImage{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return FaceImage{}, err
	}
	release, err := faceDecodeSlot(ctx)
	if err != nil {
		return FaceImage{}, err
	}
	defer release()
	snap, err := snapshotPhoto(ctx, p.pool, p.root, job.PhotoID, &uid, &job.Version)
	if err != nil {
		return FaceImage{}, err
	}
	return canonicalFaceImage(snap.Data)
}
func (p *People) runJob(ctx context.Context, uid int64, job PeopleJob) {
	work, stop := context.WithTimeout(ctx, 6*time.Minute)
	defer stop()
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(30 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-work.Done():
				return
			case <-tick.C:
				if _, err := p.renewJob(work, uid, job); err != nil {
					stop()
					return
				}
			}
		}
	}()
	im, err := p.image(work, uid, job)
	var faces []FaceDetection
	if err == nil {
		p.rt.ioMu.Lock()
		var w *Worker
		w, err = p.rt.worker()
		if err == nil {
			cfg, e := loadSettings(work, p.pool)
			if e != nil {
				err = e
			} else {
				w.Threads = clampInt(cfg.Int("cpu_threads"))
				w.Device = cfg.Str("device")
				faces, err = w.DetectFaces(work, im)
			}
		}
		p.rt.ioMu.Unlock()
	}
	if err == nil && work.Err() == nil {
		_, err = p.store(work, uid, job, faces)
	}
	if err != nil && work.Err() == nil {
		_, _ = p.failJob(work, uid, job, err.Error())
	}
	stop()
	<-done
}
func (p *People) start(ctx context.Context) func() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		var after int64
		tick := time.NewTicker(2 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
			cfg, err := loadSettings(ctx, p.pool)
			if err != nil {
				continue
			}
			if !cfg.Bool("enabled") || !cfg.Bool("people_enabled") {
				p.rt.UnloadModel("people")
				continue
			}
			rows, err := p.pool.Query(ctx, `SELECT pref.user_id FROM people_preference pref JOIN "user" u ON u.user_id=pref.user_id WHERE pref.enabled AND NOT pref.paused AND u.ai_search_enabled AND ($2='server' OR ($2='user' AND pref.placement='server')) AND pref.user_id>$1 ORDER BY pref.user_id LIMIT 25`, after, cfg.Str("placement_people"))
			if err != nil {
				continue
			}
			ids := []int64{}
			for rows.Next() {
				var id int64
				if rows.Scan(&id) == nil {
					ids = append(ids, id)
				}
			}
			rows.Close()
			if len(ids) == 0 {
				after = 0
				continue
			}
			for _, uid := range ids {
				after = uid
				job, err := p.claim(ctx, uid, "server")
				if err != nil {
					log.Printf("people: %v", err)
					continue
				}
				if job != nil {
					p.runJob(ctx, uid, *job)
				}
			}
		}
	}()
	return func() { <-done }
}
