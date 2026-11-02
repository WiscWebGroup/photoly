package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

type peopleFixture struct {
	p         *People
	ctx       context.Context
	uid, root int64
	t         *testing.T
}

func newPeopleFixture(t *testing.T) *peopleFixture {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	if err := migrations.Run(ctx, url); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	f := &peopleFixture{p: &People{pool: pool, root: t.TempDir()}, ctx: ctx, t: t}
	var saved []byte
	err = pool.QueryRow(ctx, `SELECT desired FROM ai_control_state WHERE name='global'`).Scan(&saved)
	if err != nil {
		saved = []byte(`{}`)
	}
	cfg := defaults()
	cfg["enabled"] = true
	cfg["people_enabled"] = true
	cfg["scheduler"] = "running"
	cfg["cpu_pause_percent"] = 100
	cfg["cpu_resume_percent"] = 99
	cfg["ram_max_percent"] = 100
	cfg["memory_pause_free_mib"] = 0
	cfg["disk_pause_free_mib"] = 0
	f.exec(`INSERT INTO ai_control_state(name,desired,observed) VALUES('global',$1,'{}') ON CONFLICT(name) DO UPDATE SET desired=excluded.desired`, map[string]any(cfg))
	id := newUUID()
	err = pool.QueryRow(ctx, `INSERT INTO "user"(uuid,email,user_name,role,ai_search_enabled) VALUES($1,$2,'People test','user',true) RETURNING user_id`, id, id+"@test.invalid").Scan(&f.uid)
	if err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, `INSERT INTO namespace(ns_name,ns_parent_id,user_id) VALUES('/',-1,$1) RETURNING ns_id`, f.uid).Scan(&f.root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM ai_job WHERE kind='people' AND payload->>'user_id'=$1`, fmt.Sprint(f.uid))
		pool.Exec(ctx, `DELETE FROM photo WHERE user_id=$1`, f.uid)
		pool.Exec(ctx, `DELETE FROM "user" WHERE user_id=$1`, f.uid)
		pool.Exec(ctx, `UPDATE ai_control_state SET desired=$1 WHERE name='global'`, saved)
		pool.Close()
	})
	if _, err = f.p.changeSettings(ctx, f.uid, map[string]any{"enabled": true}); err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *peopleFixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.p.pool.Exec(f.ctx, sql, args...); err != nil {
		f.t.Fatal(err)
	}
}
func (f *peopleFixture) photo() int64 {
	f.t.Helper()
	var id int64
	err := f.p.pool.QueryRow(f.ctx, `INSERT INTO photo(photo_name,photo_uuid,user_id,ns_id,format) VALUES($1,$1,$2,$3,'jpg') RETURNING photo_id`, newUUID(), f.uid, f.root).Scan(&id)
	if err != nil {
		f.t.Fatal(err)
	}
	return id
}
func (f *peopleFixture) job() PeopleJob {
	f.t.Helper()
	for i := 0; i < 5; i++ {
		job, err := f.p.claim(f.ctx, f.uid, "browser")
		if err != nil {
			f.t.Fatal(err)
		}
		if job != nil {
			return *job
		}
		time.Sleep(20 * time.Millisecond)
	}
	f.t.Fatal("no job")
	return PeopleJob{}
}
func testFace(axis int) FaceDetection {
	v := make([]float32, 128)
	v[axis] = 1
	return FaceDetection{Box: []float32{.1, .1, .3, .3}, Landmarks: []float32{.15, .2, .3, .2, .22, .25, .17, .32, .3, .32}, Score: .99, Vector: v}
}
func (f *peopleFixture) store(job PeopleJob, faces ...FaceDetection) {
	f.t.Helper()
	if _, err := f.p.store(f.ctx, f.uid, job, faces); err != nil {
		f.t.Fatal(err)
	}
}
func (f *peopleFixture) list(mode string) []Person {
	f.t.Helper()
	out, err := f.p.list(f.ctx, f.uid, map[string]string{"mode": mode})
	if err != nil {
		f.t.Fatal(err)
	}
	return out.(map[string]any)["items"].([]Person)
}
func TestPeopleGates(t *testing.T) {
	f := newPeopleFixture(t)
	for mask := 0; mask < 16; mask++ {
		f.exec(`UPDATE ai_control_state SET desired=desired||jsonb_build_object('enabled',$1::bool,'people_enabled',$2::bool) WHERE name='global'`, mask&1 != 0, mask&2 != 0)
		f.exec(`UPDATE "user" SET ai_search_enabled=$2 WHERE user_id=$1`, f.uid, mask&4 != 0)
		f.exec(`UPDATE people_preference SET enabled=$2 WHERE user_id=$1`, f.uid, mask&8 != 0)
		pref, _, err := f.p.preference(f.ctx, f.p.pool, f.uid, false)
		if err != nil {
			t.Fatal(err)
		}
		if pref.Visible != (mask == 15) {
			t.Fatalf("mask %d visible %v", mask, pref.Visible)
		}
		if mask != 15 {
			if _, err = f.p.list(f.ctx, f.uid, map[string]string{}); err == nil {
				t.Fatalf("mask %d readable", mask)
			}
		}
	}
}
func TestPeopleGroupingAndEdits(t *testing.T) {
	f := newPeopleFixture(t)
	first := f.photo()
	j := f.job()
	f.store(j, testFace(0))
	if len(f.list("pending")) != 1 || len(f.list("people")) != 0 {
		t.Fatal("singleton not pending")
	}
	second := f.photo()
	j2 := f.job()
	f.store(j2, testFace(0))
	people := f.list("people")
	if len(people) != 1 || people[0].Count != 2 {
		t.Fatalf("grouped %+v", people)
	}
	id := people[0].ID
	if _, err := f.p.editPerson(f.ctx, f.uid, id, map[string]any{"name": "Alice"}); err != nil {
		t.Fatal(err)
	}
	f.store(j2, testFace(1))
	if f.list("people")[0].Name != "Alice" {
		t.Fatal("idempotent retry overwrote edits")
	}
	if _, err := f.p.photos(f.ctx, f.uid, id, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	fs, err := f.p.faces(f.ctx, f.uid, id, 0, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	face := fs.(map[string]any)["items"].([]PersonFace)[0]
	moved, err := f.p.editFace(f.ctx, f.uid, face.ID, map[string]any{"personId": json.Number("0"), "name": "Bob"})
	if err != nil {
		t.Fatal(err)
	}
	bob := moved.(map[string]int64)["personId"]
	f.store(j, testFace(0))
	if len(f.list("people")) != 2 {
		t.Fatal("manual split lost")
	}
	if _, err = f.p.merge(f.ctx, f.uid, id, []int64{bob}); err != nil {
		t.Fatal(err)
	}
	if len(f.list("people")) != 1 || f.list("people")[0].Count != 2 {
		t.Fatal("merge failed")
	}
	canonical, err := f.p.canonicalPerson(f.ctx, f.p.pool, f.uid, bob)
	if err != nil || canonical != id {
		t.Fatal("merge alias lost")
	}
	if _, err = f.p.editPerson(f.ctx, f.uid, id, map[string]any{"hidden": true}); err != nil {
		t.Fatal(err)
	}
	if len(f.list("people")) != 0 || len(f.list("hidden")) != 1 {
		t.Fatal("hidden person visible")
	}
	faces, err := f.p.faces(f.ctx, f.uid, 0, first, map[string]string{})
	if err != nil || len(faces.(map[string]any)["items"].([]PersonFace)) != 0 {
		t.Fatal("hidden face visible")
	}
	if _, err = f.p.editPerson(f.ctx, f.uid, id, map[string]any{"hidden": false}); err != nil {
		t.Fatal(err)
	}
	f.exec(`UPDATE photo SET content_version=content_version+1 WHERE photo_id=$1`, first)
	if f.list("people")[0].Count != 1 {
		t.Fatal("replaced content still linked")
	}
	if _, err = f.p.store(f.ctx, f.uid, j, []FaceDetection{testFace(0)}); err == nil {
		t.Fatal("stale content accepted")
	}
	f.exec(`DELETE FROM photo WHERE photo_id=$1`, second)
	if f.list("people")[0].Name != "Alice" || f.list("people")[0].Count != 0 {
		t.Fatal("name lost on deletion")
	}
}
func TestPeopleLeaseResetIsolation(t *testing.T) {
	f := newPeopleFixture(t)
	f.photo()
	job := f.job()
	if other, err := f.p.claim(f.ctx, f.uid, "browser"); err != nil || other != nil {
		t.Fatal("duplicate lease")
	}
	if _, err := f.p.renewJob(f.ctx, f.uid, job); err != nil {
		t.Fatal(err)
	}
	if _, err := f.p.store(f.ctx, f.uid+100000, job, []FaceDetection{testFace(0)}); err == nil {
		t.Fatal("cross user store accepted")
	}
	bad := testFace(0)
	bad.Vector[0] = float32(math.NaN())
	if _, err := f.p.store(f.ctx, f.uid, job, []FaceDetection{bad}); err == nil {
		t.Fatal("NaN accepted")
	}
	if _, err := f.p.changeSettings(f.ctx, f.uid, map[string]any{"paused": true}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.p.store(f.ctx, f.uid, job, []FaceDetection{}); err == nil {
		t.Fatal("paused commit accepted")
	}
	if _, err := f.p.changeSettings(f.ctx, f.uid, map[string]any{"paused": false}); err != nil {
		t.Fatal(err)
	}
	fresh := f.job()
	if fresh.Token == job.Token {
		t.Fatal("lease token reused")
	}
	f.store(fresh)
	f.photo()
	late := f.job()
	if _, err := f.p.clear(f.ctx, f.uid); err != nil {
		t.Fatal(err)
	}
	if _, err := f.p.store(f.ctx, f.uid, late, []FaceDetection{}); err == nil {
		t.Fatal("commit after reset")
	}
	pref, _, err := f.p.preference(f.ctx, f.p.pool, f.uid, false)
	if err != nil || pref.Enabled || pref.Generation != 2 {
		t.Fatalf("reset %+v %v", pref, err)
	}
}

func TestPeopleRevocationRetriesAndPlacement(t *testing.T) {
	f := newPeopleFixture(t)
	f.photo()
	j := f.job()
	f.exec(`UPDATE "user" SET ai_search_enabled=false WHERE user_id=$1`, f.uid)
	if _, err := f.p.store(f.ctx, f.uid, j, nil); err == nil {
		t.Fatal("revoked account submitted")
	}
	f.exec(`UPDATE "user" SET ai_search_enabled=true WHERE user_id=$1`, f.uid)
	f.exec(`UPDATE ai_control_state SET desired=desired||'{"placement_people":"server"}' WHERE name='global'`)
	if _, err := f.p.store(f.ctx, f.uid, j, nil); err == nil {
		t.Fatal("old placement submitted")
	}
	if job, err := f.p.claim(f.ctx, f.uid, "browser"); err != nil || job != nil {
		t.Fatal("browser claimed server job")
	}
	f.exec(`UPDATE ai_control_state SET desired=desired||'{"placement_people":"user"}' WHERE name='global'`)
	for attempt := 1; attempt <= 3; attempt++ {
		if _, err := f.p.failJob(f.ctx, f.uid, j, "test failure"); err != nil {
			t.Fatal(err)
		}
		f.exec(`UPDATE ai_job SET next_run_at=now() WHERE job_id=$1`, j.ID)
		if attempt < 3 {
			j = f.job()
		}
	}
	if job, err := f.p.claim(f.ctx, f.uid, "browser"); err != nil || job != nil {
		t.Fatal("failed job restarted automatically")
	}
	if _, err := f.p.retry(f.ctx, f.uid); err != nil {
		t.Fatal(err)
	}
	j = f.job()
	f.exec(`UPDATE ai_job SET lease_expires_at=now()-interval '1 second' WHERE job_id=$1`, j.ID)
	fresh := f.job()
	if fresh.Token == j.Token {
		t.Fatal("expired token reused")
	}
	if _, err := f.p.store(f.ctx, f.uid, j, nil); err == nil {
		t.Fatal("expired worker submitted")
	}
	f.store(fresh)
}

func TestPeopleConcurrentClaimsAndReset(t *testing.T) {
	f := newPeopleFixture(t)
	for range 4 {
		f.photo()
	}
	var wg sync.WaitGroup
	jobs := make(chan *PeopleJob, 4)
	for range 4 {
		wg.Go(func() {
			j, err := f.p.claim(f.ctx, f.uid, "browser")
			if err != nil {
				t.Error(err)
			}
			jobs <- j
		})
	}
	wg.Wait()
	close(jobs)
	seen := map[int64]bool{}
	for j := range jobs {
		if j == nil {
			t.Fatal("missing concurrent lease")
		}
		if seen[j.ID] {
			t.Fatal("duplicate concurrent lease")
		}
		seen[j.ID] = true
		wg.Go(func() { _, _ = f.p.store(f.ctx, f.uid, *j, []FaceDetection{testFace(0)}) })
	}
	wg.Go(func() {
		if _, err := f.p.clear(f.ctx, f.uid); err != nil {
			t.Error(err)
		}
	})
	wg.Wait()
	var count int
	if err := f.p.pool.QueryRow(f.ctx, `SELECT count(*) FROM people_face WHERE user_id=$1`, f.uid).Scan(&count); err != nil || count != 0 {
		t.Fatal("reset raced with commit", count, err)
	}
}

func TestPeopleAmbiguousAndInvalidResults(t *testing.T) {
	f := newPeopleFixture(t)
	f.photo()
	f.store(f.job(), testFace(0), testFace(1))
	f.photo()
	j := f.job()
	for _, bad := range []FaceDetection{
		{Box: []float32{-1, 0, 1, 1}, Landmarks: testFace(0).Landmarks, Score: 1, Vector: testFace(0).Vector},
		{Box: testFace(0).Box, Landmarks: testFace(0).Landmarks, Score: 1, Vector: []float32{1}},
	} {
		if _, err := f.p.store(f.ctx, f.uid, j, []FaceDetection{bad}); err == nil {
			t.Fatal("invalid face accepted")
		}
	}
	ambiguous := testFace(0)
	ambiguous.Vector[0] = float32(math.Sqrt(.5))
	ambiguous.Vector[1] = float32(math.Sqrt(.5))
	f.store(j, ambiguous)
	if len(f.list("pending")) != 3 {
		t.Fatal("ambiguous face automatically assigned")
	}
	f.photo()
	low := testFace(5)
	low.Score = .85
	f.store(f.job(), low)
	f.photo()
	f.store(f.job(), testFace(5))
	if len(f.list("pending")) != 5 {
		t.Fatal("low quality face expanded a person")
	}
}

func TestPeopleExpiredAttemptsReachFailure(t *testing.T) {
	f := newPeopleFixture(t)
	f.photo()
	j := f.job()
	for attempt := 1; attempt <= 3; attempt++ {
		f.exec(`UPDATE ai_job SET lease_expires_at=now()-interval '1 second' WHERE job_id=$1`, j.ID)
		if attempt < 3 {
			j = f.job()
		} else {
			next, err := f.p.claim(f.ctx, f.uid, "browser")
			if err != nil || next != nil {
				t.Fatal("expired job not exhausted", err)
			}
		}
	}
	var state string
	if err := f.p.pool.QueryRow(f.ctx, `SELECT status FROM ai_job WHERE job_id=$1`, j.ID).Scan(&state); err != nil || state != "failed" {
		t.Fatal("failure recovery rolled back", state, err)
	}
}

func TestPeopleAdminPermissionRestoration(t *testing.T) {
	f := newPeopleFixture(t)
	f.photo()
	old := f.job()
	f.photo()
	if err := f.p.discover(f.ctx, f.uid); err != nil {
		t.Fatal(err)
	}
	admin := &Admin{pool: f.p.pool}
	if _, err := admin.SetUser(f.ctx, f.uid, false); err != nil {
		t.Fatal(err)
	}
	if _, err := f.p.store(f.ctx, f.uid, old, nil); err == nil {
		t.Fatal("revoked token accepted")
	}
	if _, err := admin.SetUser(f.ctx, f.uid, true); err != nil {
		t.Fatal(err)
	}
	f.store(f.job())
	f.store(f.job())
	var ready int
	if err := f.p.pool.QueryRow(f.ctx, `SELECT count(*) FROM people_media WHERE user_id=$1 AND status='ready'`, f.uid).Scan(&ready); err != nil || ready != 2 {
		t.Fatal("re-authorized jobs lost", ready, err)
	}
}

func TestPeopleDatabaseBudget(t *testing.T) {
	f := newPeopleFixture(t)
	f.photo()
	f.exec(`UPDATE ai_control_state SET desired=desired||'{"quota_bytes":1}' WHERE name='global'`)
	if job, err := f.p.claim(f.ctx, f.uid, "browser"); err != nil || job != nil {
		t.Fatal("quota admission", job, err)
	}
	f.exec(`UPDATE ai_control_state SET desired=desired||'{"quota_bytes":10737418240}' WHERE name='global'`)
	j := f.job()
	before, err := usage(f.ctx, f.p.pool)
	if err != nil {
		t.Fatal(err)
	}
	if before.Reserved < int64(faceMaxCount)*faceStorageCharge {
		t.Fatal("missing reservation")
	}
	f.store(j, testFace(0))
	after, err := usage(f.ctx, f.p.pool)
	if err != nil {
		t.Fatal(err)
	}
	if after.Reserved != before.Reserved-int64(faceMaxCount)*faceStorageCharge || after.Estimated-before.Estimated != faceStorageCharge {
		t.Fatal("storage accounting", before, after)
	}
}
