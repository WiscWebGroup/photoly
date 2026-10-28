package admin

import (
	"context"
	"net/http"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/httpx"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
)

// storageBody reads {driver, values} from a parsed request body and normalises values to strings (bools become "true"/"false").
func storageBody(d map[string]any) storage.Config {
	driver, _ := d["driver"].(string)
	if _, ok := storage.Lookup(driver); !ok {
		throw("unknown_driver", 400)
	}
	c := storage.Config{Driver: driver, Values: map[string]string{}}
	raw, _ := d["values"].(map[string]any)
	for k, v := range raw {
		switch x := v.(type) {
		case string:
			c.Values[k] = x
		case bool:
			if x {
				c.Values[k] = "true"
			} else {
				c.Values[k] = "false"
			}
		}
	}
	return c
}

// verify merges stored secrets into the submitted config and checks the backend responds.
func (s *svc) verify(ctx context.Context, c storage.Config) (storage.Config, storage.Store) {
	if d, _ := storage.Lookup(c.Driver); len(secretFields(d)) > 0 && !storage.SecretKeyConfigured() {
		throw("storage_secret_key_missing", 400)
	}
	c, err := storage.Merge(ctx, s.a.Pool, c)
	if err != nil {
		throw(err.Error(), 400)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	st, err := storage.Verify(ctx, c)
	if err != nil {
		throw(err.Error(), 400)
	}
	return c, st
}

func secretFields(d storage.Driver) (out []string) {
	for _, f := range d.Fields {
		if f.Secret {
			out = append(out, f.Name)
		}
	}
	return
}

func (s *svc) registerStorage() {
	m := s.a.Mux
	m.HandleFunc("GET /admin/storage", s.legacy(nil, false, func(w http.ResponseWriter, r *http.Request, u app.User) {
		requireAdmin(u)
		c, saved, err := storage.Public(r.Context(), s.a.Pool)
		check(err)
		d, _ := storage.Lookup(c.Driver)
		for _, n := range secretFields(d) { // never leave the wire, even if one were stored in plain config
			delete(c.Values, n)
		}
		httpx.JSON(w, map[string]any{
			"active": storage.ActiveDriver(), "driver": c.Driver, "values": c.Values, "hasSecret": saved[c.Driver].HasSecret, "saved": saved,
			"drivers": storage.Drivers(), "canSave": storage.SecretKeyConfigured(),
		}, 200)
	}))
	m.HandleFunc("POST /admin/storage/test", s.legacy(nil, false, func(w http.ResponseWriter, r *http.Request, u app.User) {
		requireAdmin(u)
		s.verify(r.Context(), storageBody(jsonBody(r)))
		httpx.JSON(w, 1, 200)
	}))
	m.HandleFunc("PUT /admin/storage", s.legacy(nil, false, func(w http.ResponseWriter, r *http.Request, u app.User) {
		requireAdmin(u)
		body := jsonBody(r)
		c, st := s.verify(r.Context(), storageBody(body))
		// Media is tagged with the backend that holds it and only the active backend's media
		// is listed, so changing backends must be confirmed by the caller.
		cur, err := storage.CurrentID(r.Context(), s.a.Pool)
		check(err)
		if confirmed, _ := body["confirm"].(bool); !confirmed && storage.BackendID(c.Driver, c.Values) != cur {
			throw("storage_switch_confirm", 409)
		}
		check(storage.Save(r.Context(), s.a.Pool, c))
		storage.Set(c.Driver, st)
		httpx.JSON(w, 1, 200)
	}))
}
