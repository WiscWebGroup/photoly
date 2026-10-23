package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	maxUpload = 64 << 20
	// The POC has no sign-in; every request acts as this user. Queries still filter by
	// user, as the real app does.
	demoUser = 1
)

type API struct {
	store   *Store
	profile Profile
	dir     string // photo files, named by SHA-256
}

func (a *API) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/profile", a.getProfile)
	mux.HandleFunc("GET /api/photos", a.listPhotos)
	mux.HandleFunc("POST /api/photos", a.upload)
	mux.HandleFunc("GET /api/photos/{id}/file", a.photoFile)
	mux.HandleFunc("GET /api/photos/{id}/similar", a.similar)
	mux.HandleFunc("DELETE /api/photos/{id}", a.deletePhoto)
	mux.HandleFunc("POST /api/search", a.search)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func serverError(w http.ResponseWriter, err error) {
	log.Printf("api: %v", err)
	fail(w, http.StatusInternalServerError, "internal error")
}

func (a *API) getProfile(w http.ResponseWriter, r *http.Request) {
	info, err := a.store.Info(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profile": a.profile, "database": info})
}

func (a *API) listPhotos(w http.ResponseWriter, r *http.Request) {
	photos, err := a.store.Photos(r.Context(), demoUser, a.profile.Fingerprint)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, photos)
}

// parseEmbedding checks the profile fingerprint and the vector's shape. An empty
// embedding is allowed on upload: the photo is stored but not searchable until a client
// (or, in the full design, the server) indexes it.
func (a *API) parseEmbedding(raw, profile string) ([]float32, error) {
	if raw == "" {
		return nil, nil
	}
	if profile != a.profile.Fingerprint {
		return nil, errors.New("profile mismatch: reload the page to get the current model")
	}
	var v []float32
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil, errors.New("embedding must be a JSON array of numbers")
	}
	return v, validateEmbedding(v)
}

func (a *API) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		fail(w, http.StatusBadRequest, "expected multipart form under 64 MiB")
		return
	}
	defer r.MultipartForm.RemoveAll()
	vec, err := a.parseEmbedding(r.FormValue("embedding"), r.FormValue("profile"))
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		fail(w, http.StatusBadRequest, "missing file")
		return
	}
	defer file.Close()

	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	ctype := http.DetectContentType(head[:n])
	if !strings.HasPrefix(ctype, "image/") {
		fail(w, http.StatusUnsupportedMediaType, "only images are supported in this POC")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		serverError(w, err)
		return
	}
	tmp, err := os.CreateTemp(a.dir, ".upload-*")
	if err != nil {
		serverError(w, err)
		return
	}
	defer os.Remove(tmp.Name())
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, hash), file)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		serverError(w, err)
		return
	}
	sum := hash.Sum(nil)
	dst := filepath.Join(a.dir, hex.EncodeToString(sum))
	if err := os.Rename(tmp.Name(), dst); err != nil {
		serverError(w, err)
		return
	}
	name := filepath.Base(header.Filename)
	id, err := a.store.AddPhoto(r.Context(), demoUser, Photo{Name: name, Mime: ctype, Size: size}, sum, a.profile.Fingerprint, vec)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "indexed": vec != nil})
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func (a *API) photoFile(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var sum []byte
	err := a.store.db.QueryRow(r.Context(), `SELECT sha256 FROM photo WHERE user_id=$1 AND id=$2`, demoUser, id).Scan(&sum)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeFile(w, r, filepath.Join(a.dir, hex.EncodeToString(sum)))
}

func (a *API) deletePhoto(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	// The file is content-addressed and may be shared; the POC leaves it on disk.
	if err := a.store.DeletePhoto(r.Context(), demoUser, id); errors.Is(err, errNotFound) {
		http.NotFound(w, r)
	} else if err != nil {
		serverError(w, err)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func limitParam(r *http.Request) int {
	n, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || n < 1 || n > 100 {
		return 20
	}
	return n
}

func (a *API) search(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Embedding []float32 `json:"embedding"`
		Profile   string    `json:"profile"`
		Limit     int       `json:"limit"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Profile != a.profile.Fingerprint {
		fail(w, http.StatusUnprocessableEntity, "profile mismatch: reload the page to get the current model")
		return
	}
	if err := validateEmbedding(body.Embedding); err != nil {
		fail(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if body.Limit < 1 || body.Limit > 100 {
		body.Limit = 20
	}
	hits, err := a.store.Search(r.Context(), demoUser, a.profile.Fingerprint, body.Embedding, body.Limit, 0)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, hits)
}

func (a *API) similar(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	hits, err := a.store.Similar(r.Context(), demoUser, a.profile.Fingerprint, id, limitParam(r))
	if errors.Is(err, errNotFound) {
		fail(w, http.StatusNotFound, "photo not indexed")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, hits)
}
