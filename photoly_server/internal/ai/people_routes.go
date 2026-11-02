package ai

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"net/http"
	"os"
	"strconv"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
)

func (p *People) register(a *app.App) {
	user := func(pattern string, fn func(*http.Request, app.User) (any, error)) {
		a.Mux.HandleFunc(pattern, Route(a, false, fn))
	}
	user("GET /ai/people/settings", func(r *http.Request, u app.User) (any, error) { return p.settings(r.Context(), u.ID) })
	user("PATCH /ai/people/settings", func(r *http.Request, u app.User) (any, error) {
		d, e := body(r)
		if e != nil {
			return nil, e
		}
		return p.changeSettings(r.Context(), u.ID, d)
	})
	user("GET /ai/people/status", func(r *http.Request, u app.User) (any, error) { return p.status(r.Context(), u.ID) })
	user("POST /ai/people/retry", func(r *http.Request, u app.User) (any, error) { return p.retry(r.Context(), u.ID) })
	user("DELETE /ai/people/data", func(r *http.Request, u app.User) (any, error) {
		d, e := body(r)
		if e != nil || d["confirm"] != true {
			return nil, fail("confirmation_required")
		}
		return p.clear(r.Context(), u.ID)
	})
	user("POST /ai/people/jobs/claim", func(r *http.Request, u app.User) (any, error) {
		job, e := p.claim(r.Context(), u.ID, "browser")
		return map[string]any{"job": job}, e
	})
	readJob := func(r *http.Request) (PeopleJob, error) {
		var job PeopleJob
		e := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 16384)).Decode(&job)
		if e != nil {
			return job, fail("invalid_face_job")
		}
		return job, nil
	}
	user("POST /ai/people/jobs/renew", func(r *http.Request, u app.User) (any, error) {
		j, e := readJob(r)
		if e != nil {
			return nil, e
		}
		return p.renewJob(r.Context(), u.ID, j)
	})
	user("POST /ai/people/jobs/result", func(r *http.Request, u app.User) (any, error) {
		j, f, e := decodeFaceBody(r)
		if e != nil {
			return nil, e
		}
		return p.store(r.Context(), u.ID, j, f)
	})
	user("POST /ai/people/jobs/fail", func(r *http.Request, u app.User) (any, error) {
		var d struct {
			Job   PeopleJob `json:"job"`
			Error string    `json:"error"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 16384)).Decode(&d); e != nil {
			return nil, fail("invalid_face_job")
		}
		return p.failJob(r.Context(), u.ID, d.Job, d.Error)
	})
	user("GET /ai/people/persons", func(r *http.Request, u app.User) (any, error) { return p.list(r.Context(), u.ID, queryMap(r)) })
	user("GET /ai/people/persons/{id}", func(r *http.Request, u app.User) (any, error) {
		id, e := idParam(r, "id")
		if e != nil {
			return nil, e
		}
		return p.detail(r.Context(), u.ID, id)
	})
	user("PATCH /ai/people/persons/{id}", func(r *http.Request, u app.User) (any, error) {
		id, e := idParam(r, "id")
		if e != nil {
			return nil, e
		}
		d, e := body(r)
		if e != nil {
			return nil, e
		}
		return p.editPerson(r.Context(), u.ID, id, d)
	})
	user("POST /ai/people/persons/{id}/merge", func(r *http.Request, u app.User) (any, error) {
		id, e := idParam(r, "id")
		if e != nil {
			return nil, e
		}
		var d struct {
			IDs []int64 `json:"ids"`
		}
		if e = json.NewDecoder(http.MaxBytesReader(nil, r.Body, 16384)).Decode(&d); e != nil {
			return nil, fail("invalid_merge")
		}
		return p.merge(r.Context(), u.ID, id, d.IDs)
	})
	user("GET /ai/people/persons/{id}/photos", func(r *http.Request, u app.User) (any, error) {
		id, e := idParam(r, "id")
		if e != nil {
			return nil, e
		}
		return p.photos(r.Context(), u.ID, id, queryMap(r))
	})
	user("GET /ai/people/persons/{id}/faces", func(r *http.Request, u app.User) (any, error) {
		id, e := idParam(r, "id")
		if e != nil {
			return nil, e
		}
		return p.faces(r.Context(), u.ID, id, 0, queryMap(r))
	})
	user("GET /ai/people/photos/{id}", func(r *http.Request, u app.User) (any, error) {
		id, e := idParam(r, "id")
		if e != nil {
			return nil, e
		}
		return p.faces(r.Context(), u.ID, 0, id, queryMap(r))
	})
	user("PATCH /ai/people/faces/{id}", func(r *http.Request, u app.User) (any, error) {
		id, e := idParam(r, "id")
		if e != nil {
			return nil, e
		}
		d, e := body(r)
		if e != nil {
			return nil, e
		}
		return p.editFace(r.Context(), u.ID, id, d)
	})
	// Binary endpoints require the account session header, never photo sharing tokens.
	binary := func(pattern string, fn func(*http.Request, app.User) ([]byte, string, error)) {
		a.Mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			called := false
			a.Auth(func(_ http.ResponseWriter, r *http.Request) {
				called = true
				data, kind, e := fn(r, a.UserFrom(r.Context()))
				if e != nil {
					writeErr(w, e)
					return
				}
				w.Header().Set("Content-Type", kind)
				w.Header().Set("Cache-Control", "private, no-store")
				w.Header().Set("X-Content-Type-Options", "nosniff")
				w.Write(data)
			})(&discard{http.Header{}}, r)
			if !called {
				writeErr(w, fail("unauthenticated", 401))
			}
		})
	}
	binary("POST /ai/people/jobs/image", func(r *http.Request, u app.User) ([]byte, string, error) {
		j, e := readJob(r)
		if e != nil {
			return nil, "", e
		}
		im, e := p.image(r.Context(), u.ID, j)
		if e != nil {
			return nil, "", e
		}
		return im.Bytes(), "application/octet-stream", nil
	})
	binary("GET /ai/people/faces/{id}/thumbnail", func(r *http.Request, u app.User) ([]byte, string, error) {
		id, e := idParam(r, "id")
		if e != nil {
			return nil, "", e
		}
		if e = p.readable(r.Context(), p.pool, u.ID); e != nil {
			return nil, "", e
		}
		var photoID int64
		var version int32
		var box []float32
		e = p.pool.QueryRow(r.Context(), `SELECT f.photo_id,f.content_version,f.box FROM people_face f JOIN photo ph ON ph.photo_id=f.photo_id WHERE f.face_id=$1 AND f.user_id=$2 AND photo_visible(ph.storage_id,ph.user_id) AND ph.content_version=f.content_version`, id, u.ID).Scan(&photoID, &version, &box)
		if e != nil {
			return nil, "", peopleNotFound(e)
		}
		release, e := faceDecodeSlot(r.Context())
		if e != nil {
			return nil, "", e
		}
		defer release()
		snap, e := snapshotPhoto(r.Context(), p.pool, p.root, photoID, &u.ID, &version)
		if e != nil {
			return nil, "", e
		}
		im, e := canonicalFaceImage(snap.Data)
		if e != nil {
			return nil, "", e
		}
		// Generated on demand: no stale face-crop files survive deletion or replacement.
		out := image.NewRGBA(image.Rect(0, 0, 128, 128))
		cx, cy := float64(box[0]+box[2]/2)*float64(im.Width), float64(box[1]+box[3]/2)*float64(im.Height)
		size := max(float64(box[2])*float64(im.Width), float64(box[3])*float64(im.Height)) * 1.3
		for y := 0; y < 128; y++ {
			for x := 0; x < 128; x++ {
				i := (y*128 + x) * 4
				for c := 0; c < 3; c++ {
					out.Pix[i+c] = sampleFace(im, cx+(float64(x)/127-.5)*size, cy+(float64(y)/127-.5)*size, c, true)
				}
				out.Pix[i+3] = 255
			}
		}
		var buf bytes.Buffer
		e = jpeg.Encode(&buf, out, &jpeg.Options{Quality: 85})
		return buf.Bytes(), "image/jpeg", e
	})
	a.Mux.HandleFunc("GET /models/people/{fingerprint}/{file}", func(w http.ResponseWriter, r *http.Request) {
		cfg, e := loadSettings(r.Context(), p.pool)
		if e != nil || !cfg.Bool("enabled") || !cfg.Bool("people_enabled") {
			http.Error(w, "People is disabled", 403)
			return
		}
		if r.PathValue("fingerprint") != faceFingerprint {
			http.NotFound(w, r)
			return
		}
		kind := ""
		for k, f := range faceModelFiles {
			if f.License == r.PathValue("file") {
				data, err := faceLicenses.ReadFile("peopledata/" + f.License)
				if err != nil {
					writeErr(w, err)
					return
				}
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				w.Write(data)
				return
			}
			if f.File == r.PathValue("file") {
				kind = k
			}
		}
		if kind == "" {
			http.NotFound(w, r)
			return
		}
		path, e := ensureFaceModel(r.Context(), kind)
		if e != nil {
			writeErr(w, e)
			return
		}
		f, e := os.Open(path)
		if e != nil {
			writeErr(w, e)
			return
		}
		defer f.Close()
		stat, e := f.Stat()
		if e != nil {
			writeErr(w, e)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))
		http.ServeContent(w, r, stat.Name(), stat.ModTime(), f)
	})
}
