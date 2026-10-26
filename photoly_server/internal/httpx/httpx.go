// Package httpx is the legacy response envelope shared by every route package.
package httpx

import (
	"encoding/json"
	"net/http"
)

// JSON writes {"t": v, "msgCode": code}.
func JSON(w http.ResponseWriter, v any, code int) {
	write(w, map[string]any{"t": v, "msgCode": code})
}

// Page writes {"t": v, "pageNum": pages, "msgCode": code}.
func Page(w http.ResponseWriter, v any, pages, code int) {
	write(w, map[string]any{"t": v, "pageNum": pages, "msgCode": code})
}

// Token writes {"t": v, "token": token, "msgCode": code}.
func Token(w http.ResponseWriter, v any, token string, code int) {
	write(w, map[string]any{"t": v, "token": token, "msgCode": code})
}

func write(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}
