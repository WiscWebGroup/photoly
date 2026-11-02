package user

import (
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

func (s *svc) sourceHealth(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
	w.Header().Set("Cache-Control", "private, no-store")
	ids := strings.Split(r.URL.Query().Get("ids"), ",")
	if len(ids) > 100 {
		return nil, legacy.Bad()
	}
	sources := []storage.Source{}
	seen := map[int64]bool{}
	for _, v := range ids {
		id, e := strconv.ParseInt(v, 10, 64)
		if e != nil || id <= 0 {
			return nil, legacy.Bad()
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		source, e := storage.ReadSource(r.Context(), s.a.Pool, id)
		if e != nil || source.UserID != uid {
			return nil, legacy.Err(nil, 404)
		}
		sources = append(sources, source)
	}
	results := make([]storage.Health, len(sources))
	var wg sync.WaitGroup
	slots := make(chan struct{}, 8)
	for i, source := range sources {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-r.Context().Done():
				return
			}
			v, e := storage.ProbeHealth(r.Context(), s.a.Pool, source, r.Method == "POST")
			if e != nil {
				v = storage.Health{SourceID: source.ID, State: "unknown"}
			}
			results[i] = v
		}()
	}
	wg.Wait()
	return legacy.Res{V: results, Code: 200}, nil
}
