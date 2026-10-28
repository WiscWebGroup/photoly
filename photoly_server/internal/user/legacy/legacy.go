// Package legacy holds the Flask `legacy()` decorator semantics (response families,
// BusinessError mapping, argument parsing) shared by the user and catalog route packages.
package legacy

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/httpx"
	"github.com/jackc/pgx/v5/pgconn"
)

// Biz is Flask's BusinessError(value, code).
type Biz struct {
	Value any
	Code  int
}

func (e *Biz) Error() string { return fmt.Sprint(e.Value) }

func Err(v any, code int) error { return &Biz{v, code} }

// Bad is BusinessError(-1).
func Bad() error { return &Biz{-1, 400} }

// Handler outputs: Res, TokenRes, PageRes, Text, Empty, Raw, Done (response already written).
type (
	Res struct {
		V    any
		Code int
	}
	TokenRes struct {
		V     string
		Token *string
		Code  int
	}
	PageRes struct {
		V           any
		Pages, Code int
	}
	Text  string
	Empty struct{}
	Raw   struct{ V any }
	Done  struct{}
)

// Changed is controllers.common.changed.
func Changed(v int, minimum int) Res {
	if v >= minimum {
		return Res{v, 200}
	}
	return Res{v, 400}
}

type Handler func(w http.ResponseWriter, r *http.Request, uid int64) (any, error)

// Opt mirrors the arguments of Flask's legacy(): auth is "header", "path" (token in the
// {token} path value) or "" (none).
type Opt struct {
	Auth        string
	Shape       string // "result" (default), "token", "page", "raw", "binary"
	Missing     any
	PageMissing int
	Failure     any
	HasFailure  bool
}

func (o Opt) Wrap(a *app.App, h Handler) http.HandlerFunc {
	if o.Shape == "" {
		o.Shape = "result"
	}
	if o.Auth == "header" && o.Shape == "result" && o.Missing == nil && a.Auth != nil {
		return a.Auth(func(w http.ResponseWriter, r *http.Request) {
			o.run(w, r, a.UserFrom(r.Context()).ID, h)
		})
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var uid int64
		if o.Auth != "" {
			token := r.Header.Get("HRD-Token")
			if o.Auth == "path" {
				token = r.PathValue("token")
			}
			id, ok := Sessions.Identify(r.Context(), token)
			if !ok {
				switch o.Shape {
				case "binary":
					w.WriteHeader(403)
				case "page":
					emit(w, PageRes{o.Missing, o.PageMissing, 403})
				default:
					emit(w, Res{o.Missing, 403})
				}
				return
			}
			uid = id
		}
		o.run(w, r, uid, h)
	}
}

func (o Opt) run(w http.ResponseWriter, r *http.Request, uid int64, h Handler) {
	out, err := h(w, r, uid)
	if err == nil {
		if out == nil && o.Shape == "raw" {
			out = Empty{}
		}
		emit(w, out)
		return
	}
	var biz *Biz
	var pg *pgconn.PgError
	switch {
	case errors.As(err, &biz):
	case errors.As(err, &pg) && strings.HasPrefix(pg.Code, "23"):
		biz = &Biz{-1, 400}
	default:
		http.Error(w, "internal error", 500)
		return
	}
	switch o.Shape {
	case "binary":
		code := biz.Code
		if code != 400 && code != 403 && code != 404 {
			code = 400
		}
		w.WriteHeader(code)
	case "raw":
		if !o.HasFailure {
			emit(w, Empty{})
		} else {
			emit(w, Raw{o.Failure})
		}
	case "token":
		emit(w, TokenRes{"Error Create User", nil, biz.Code})
	case "page":
		emit(w, PageRes{nil, 0, biz.Code})
	default:
		v := biz.Value
		if o.HasFailure {
			v = o.Failure
		}
		emit(w, Res{v, biz.Code})
	}
}

func emit(w http.ResponseWriter, out any) {
	switch v := out.(type) {
	case Res:
		httpx.JSON(w, v.V, v.Code)
	case PageRes:
		httpx.Page(w, v.V, v.Pages, v.Code)
	case TokenRes:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"t": v.V, "token": v.Token, "msgCode": v.Code})
	case Text:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(v))
	case Empty:
		w.WriteHeader(200)
	case Raw:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v.V)
	case Done:
	}
}

// Arg is controllers.common.arg: query string first, then form body.
func Arg(r *http.Request, name string) (string, bool) {
	if q := r.URL.Query(); q.Has(name) {
		return q.Get(name), true
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		_ = r.ParseMultipartForm(32 << 20)
	} else {
		_ = r.ParseForm()
	}
	if r.PostForm.Has(name) {
		return r.PostForm.Get(name), true
	}
	return "", false
}

// ArgInt returns (value, present, err); a malformed value or a missing required one is Bad().
func ArgInt(r *http.Request, name string, required bool) (int64, bool, error) {
	s, ok := Arg(r, name)
	if !ok {
		if required {
			return 0, false, Bad()
		}
		return 0, false, nil
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, false, Bad()
	}
	return n, true, nil
}

// Body is controllers.common.body: a JSON object or BusinessError(-1).
func Body(r *http.Request) (map[string]any, error) {
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" && !strings.HasSuffix(mt, "+json") {
		return nil, Bad()
	}
	d := json.NewDecoder(r.Body)
	d.UseNumber()
	var m map[string]any
	if err := d.Decode(&m); err != nil || m == nil {
		return nil, Bad()
	}
	return m, nil
}

// Int reads a JSON integer (not a float, not out of int32 range which cannot match a key).
func Int(v any) (int64, bool) {
	switch n := v.(type) {
	case json.Number:
		i, err := strconv.ParseInt(n.String(), 10, 64)
		return i, err == nil
	case int64:
		return n, true
	}
	return 0, false
}

func Str(v any) (string, bool) { s, ok := v.(string); return s, ok }

// ValidName is services.base.valid_name.
func ValidName(v any, root bool) (string, error) {
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" || len([]rune(s)) > 255 {
		return "", Bad()
	}
	if s == "." || s == ".." || strings.ContainsAny(s, "/\\\x00\r\n") {
		if !(root && s == "/") {
			return "", Bad()
		}
	}
	return s, nil
}

func InInt32(n int64) bool { return n >= math.MinInt32 && n <= math.MaxInt32 }
