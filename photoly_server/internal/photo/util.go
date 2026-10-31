// Package photo implements the photo routes and their services
// (photo, media, live import, OCR) plus the file/EXIF/imaging infrastructure.
package photo

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Biz is Flask's BusinessError(value, code).
type Biz struct {
	Value any
	Code  int
}

func (e *Biz) Error() string { return fmt.Sprint(e.Value) }

// Err builds a BusinessError.
func Err(v any, code int) error { return &Biz{v, code} }

// Bad is BusinessError(-1).
func Bad() error { return &Biz{-1, 400} }

// BizOf unwraps a BusinessError (or maps a PostgreSQL integrity error to Bad()).
func BizOf(err error) (*Biz, bool) {
	if storage.ClientError(err) {
		return &Biz{err.Error(), 409}, true
	}
	var b *Biz
	if errors.As(err, &b) {
		return b, true
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && strings.HasPrefix(pg.Code, "23") {
		return &Biz{-1, 400}, true
	}
	return nil, false
}

// DB is satisfied by *pgxpool.Pool and pgx.Tx.
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// NewUUID returns a random (v4) UUID string.
func NewUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmtUUID(b)
}

func fmtUUID(b []byte) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ParseUUID mirrors str(UUID(value)): it normalizes any accepted UUID spelling.
func ParseUUID(v string) (string, [16]byte, bool) {
	var out [16]byte
	s := strings.ToLower(strings.TrimSpace(v))
	s = strings.TrimPrefix(s, "urn:uuid:")
	s = strings.Trim(s, "{}")
	s = strings.ReplaceAll(s, "-", "")
	if len(s) != 32 {
		return "", out, false
	}
	for i := 0; i < 16; i++ {
		n, err := strconv.ParseUint(s[2*i:2*i+2], 16, 8)
		if err != nil {
			return "", out, false
		}
		out[i] = byte(n)
	}
	return fmtUUID(out[:]), out, true
}

// UUID5 is uuid.uuid5(namespace, name).
func UUID5(namespace [16]byte, name string) string {
	h := sha1.New()
	h.Write(namespace[:])
	h.Write([]byte(name))
	sum := h.Sum(nil)[:16]
	sum[6] = sum[6]&0x0f | 0x50
	sum[8] = sum[8]&0x3f | 0x80
	return fmtUUID(sum)
}

// isoNaive is datetime.isoformat() for a naive datetime.
func isoNaive(t time.Time) string {
	if t.Nanosecond()/1000 == 0 {
		return t.Format("2006-01-02T15:04:05")
	}
	return t.Format("2006-01-02T15:04:05.000000")
}

// IsoAware is isoformat() for a timestamptz read in a UTC session.
func IsoAware(t time.Time) string {
	t = t.UTC()
	if t.Nanosecond()/1000 == 0 {
		return t.Format("2006-01-02T15:04:05") + "+00:00"
	}
	return t.Format("2006-01-02T15:04:05.000000") + "+00:00"
}

func optTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return isoNaive(*t)
}

// ---- request arguments (controllers.common.arg/ids/body) ----

func argStr(r *http.Request, name string) (string, bool) {
	if q := r.URL.Query(); q.Has(name) {
		return q.Get(name), true
	}
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/") {
		_ = r.ParseMultipartForm(32 << 20)
	} else {
		_ = r.ParseForm()
	}
	if r.PostForm.Has(name) {
		return r.PostForm.Get(name), true
	}
	return "", false
}

func pyInt(s string) (int64, bool) {
	s = strings.ReplaceAll(strings.TrimSpace(s), "_", "")
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil
}

// argInt returns (value, present). A malformed or (when required) missing value is Bad().
func argInt(r *http.Request, name string, required bool) (int64, bool, error) {
	s, ok := argStr(r, name)
	if !ok {
		if required {
			return 0, false, Bad()
		}
		return 0, false, nil
	}
	n, ok := pyInt(s)
	if !ok {
		return 0, false, Bad()
	}
	return n, true, nil
}

func argIDs(r *http.Request, name string) ([]int64, error) {
	_ = r.ParseForm()
	values := append([]string{}, r.URL.Query()[name]...)
	values = append(values, r.PostForm[name]...)
	var out []int64
	for _, v := range values {
		for _, item := range strings.Split(v, ",") {
			if strings.TrimSpace(item) == "" {
				continue
			}
			n, ok := pyInt(item)
			if !ok {
				return nil, Bad()
			}
			out = append(out, n)
		}
	}
	return out, nil
}

func inInt32(n int64) bool { return n >= math.MinInt32 && n <= math.MaxInt32 }

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

// ---- Python-compatible number formatting ----

// normNum mimics exiftool's JSON numbers: 15 significant digits, whole values as integers.
func normNum(f float64) any {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return f
	}
	r, _ := strconv.ParseFloat(strconv.FormatFloat(f, 'g', 15, 64), 64)
	if r == math.Trunc(r) && math.Abs(r) < 1e15 {
		return int64(r)
	}
	return r
}

// pyStr is str(value) for the JSON scalars exiftool produces.
func pyStr(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return pyFloat(x)
	case bool:
		if x {
			return "True"
		}
		return "False"
	case nil:
		return "None"
	}
	return fmt.Sprint(v)
}

// pyFloat is repr(float).
func pyFloat(f float64) string {
	if math.IsInf(f, 1) {
		return "inf"
	} else if math.IsInf(f, -1) {
		return "-inf"
	} else if math.IsNaN(f) {
		return "nan"
	}
	e := strconv.FormatFloat(f, 'e', -1, 64) // d.ddde±XX
	idx := strings.IndexByte(e, 'e')
	exp, _ := strconv.Atoi(e[idx+1:])
	if exp >= -4 && exp < 16 {
		s := strconv.FormatFloat(f, 'f', -1, 64)
		if !strings.Contains(s, ".") {
			s += ".0"
		}
		return s
	}
	mant := e[:idx]
	sign := "+"
	if exp < 0 {
		sign, exp = "-", -exp
	}
	return fmt.Sprintf("%se%s%02d", mant, sign, exp)
}

// pyG is format(value, 'g') (six significant digits).
func pyG(f float64) string {
	return strconv.FormatFloat(f, 'g', 6, 64)
}

// limitDenominator implements fractions.Fraction(f).limit_denominator(max) and returns str().
func limitDenominator(f float64, maxDen int64) string {
	r := new(big.Rat)
	if r.SetFloat64(f) == nil {
		return pyFloat(f)
	}
	n, d := r.Num(), r.Denom()
	bound := big.NewInt(maxDen)
	if d.Cmp(bound) <= 0 {
		return ratStr(n, d)
	}
	p0, q0, p1, q1 := big.NewInt(0), big.NewInt(1), big.NewInt(1), big.NewInt(0)
	nn, dd := new(big.Int).Set(n), new(big.Int).Set(d)
	for {
		a := new(big.Int).Div(nn, dd) // floor division (dd > 0)
		q2 := new(big.Int).Add(q0, new(big.Int).Mul(a, q1))
		if q2.Cmp(bound) > 0 {
			break
		}
		p0, q0, p1, q1 = p1, q1, new(big.Int).Add(p0, new(big.Int).Mul(a, p1)), q2
		nn, dd = dd, new(big.Int).Sub(nn, new(big.Int).Mul(a, dd))
	}
	k := new(big.Int).Div(new(big.Int).Sub(bound, q0), q1)
	b1n, b1d := new(big.Int).Add(p0, new(big.Int).Mul(k, p1)), new(big.Int).Add(q0, new(big.Int).Mul(k, q1))
	b2n, b2d := p1, q1
	// pick the bound closest to the original value (ties to bound2)
	diff1 := new(big.Rat).Abs(new(big.Rat).Sub(new(big.Rat).SetFrac(b1n, b1d), r))
	diff2 := new(big.Rat).Abs(new(big.Rat).Sub(new(big.Rat).SetFrac(b2n, b2d), r))
	if diff2.Cmp(diff1) <= 0 {
		return ratStr(b2n, b2d)
	}
	return ratStr(b1n, b1d)
}

func ratStr(n, d *big.Int) string {
	rr := new(big.Rat).SetFrac(n, d)
	if rr.IsInt() {
		return rr.Num().String()
	}
	return rr.Num().String() + "/" + rr.Denom().String()
}
