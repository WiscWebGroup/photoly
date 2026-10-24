package ai

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	mrand "math/rand/v2"
	"os"
	"strconv"
	"time"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func dataDir() string { return env("PHOTOLY_DATA", "data") }

// Err is the AI API error: a machine code and an HTTP status (app/ai/types.py AIError).
type Err struct {
	Code   string
	Status int
}

func (e *Err) Error() string { return e.Code }

func fail(code string, status ...int) *Err {
	s := 400
	if len(status) > 0 {
		s = status[0]
	}
	return &Err{code, s}
}

// asErr turns any error into an *Err (unknown errors become 500 internal_error).
func asErr(err error) *Err {
	var e *Err
	if errors.As(err, &e) {
		return e
	}
	return &Err{"internal_error", 500}
}

// ---- loosely typed JSON (settings, request bodies) ----

// toInt reports whether v is an integer JSON value (Python's `type(v) is int`: no bool, no 1.5).
func toInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), n == math.Trunc(n) && math.Abs(n) < 1<<53
	case json.Number:
		i, err := strconv.ParseInt(n.String(), 10, 64)
		return i, err == nil
	}
	return 0, false
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

// iso formats like Python's datetime.isoformat() for aware datetimes.
func iso(t time.Time) string {
	if t.Nanosecond()/1000 == 0 {
		return t.Format("2006-01-02T15:04:05-07:00")
	}
	return t.Format("2006-01-02T15:04:05.000000-07:00")
}

func isoPtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return iso(*t)
}

func must(err error) {
	if err != nil {
		panic(fmt.Sprintf("ai: %v", err))
	}
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func randFloat() float64 { return float64(mrand.Uint64()>>11) / (1 << 53) }

func logf(format string, args ...any) { log.Printf(format, args...) }

const spotCheckRate = 0.01
