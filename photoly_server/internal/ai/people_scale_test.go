package ai

import (
	"os"
	"runtime"
	"testing"
	"time"
)

// Synthetic database scale, deliberately separate from real image inference.
// Use an isolated disposable PostgreSQL database; the fixture removes its rows.
func TestPeopleScale(t *testing.T) {
	if os.Getenv("PHOTOLY_PEOPLE_SCALE") == "" {
		t.Skip("PHOTOLY_PEOPLE_SCALE=1 enables 100k-photo/200k-face database test")
	}
	f := newPeopleFixture(t)
	started := time.Now()
	f.exec(`INSERT INTO photo(photo_name,photo_uuid,user_id,ns_id,format) SELECT 'scale-'||n,'people-scale-'||$1::int::text||'-'||n,$1,$2,'jpg' FROM generate_series(1,100000)n`, f.uid, f.root)
	f.exec(`INSERT INTO people_media(photo_id,user_id,content_version,fingerprint,generation,status,face_count) SELECT photo_id,user_id,content_version,$2,1,'ready',2 FROM photo WHERE user_id=$1`, f.uid, faceFingerprint)
	f.exec(`INSERT INTO people_person(user_id,name) SELECT $1,'Person '||n FROM generate_series(0,999)n`, f.uid)
	f.exec(`INSERT INTO people_face(user_id,photo_id,content_version,detector_version,box,landmarks,score,quality,person_id) SELECT $1,ph.photo_id,ph.content_version,$2,ARRAY[.1,.1,.2,.2]::real[],ARRAY[.12,.14,.25,.14,.2,.2,.15,.25,.23,.25]::real[],.99,true,p.person_id FROM photo ph CROSS JOIN generate_series(0,1)n JOIN people_person p ON p.user_id=$1 AND p.name='Person '||((ph.photo_id+n)%1000) WHERE ph.user_id=$1`, f.uid, faceFingerprint)
	f.exec(`INSERT INTO people_embedding(face_id,user_id,fingerprint,embedding,source) SELECT face_id,$1,$2,ARRAY(SELECT sin(f.face_id*.01+i)::real FROM generate_series(1,128)i)::vector,'server' FROM people_face f WHERE user_id=$1`, f.uid, faceFingerprint)
	t.Logf("seed 100000 photos / 200000 faces: %s", time.Since(started))
	f.exec(`ANALYZE photo`)
	f.exec(`ANALYZE people_face`)
	f.exec(`ANALYZE people_embedding`)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	started = time.Now()
	persons := f.list("people")
	if len(persons) != 50 {
		t.Fatalf("page length %d", len(persons))
	}
	t.Logf("person page: %s", time.Since(started))
	started = time.Now()
	if _, err := f.p.photos(f.ctx, f.uid, persons[0].ID, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	t.Logf("photo page: %s", time.Since(started))
	started = time.Now()
	tx, err := f.p.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.p.matchPerson(f.ctx, tx, f.uid, testFace(0).Vector); err != nil {
		tx.Rollback(f.ctx)
		t.Fatal(err)
	}
	tx.Rollback(f.ctx)
	t.Logf("vector candidate retrieval: %s", time.Since(started))
	started = time.Now()
	f.photo()
	_ = f.job()
	t.Logf("incremental discovery and claim: %s", time.Since(started))
	runtime.ReadMemStats(&after)
	t.Logf("Go query allocation: %.2f MiB, live heap: %.2f MiB (not model inference throughput)", float64(after.TotalAlloc-before.TotalAlloc)/(1<<20), float64(after.HeapAlloc)/(1<<20))
}
