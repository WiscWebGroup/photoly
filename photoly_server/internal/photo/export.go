package photo

import "net/http"

// Helpers shared with the uploads package.

func Body(r *http.Request) (map[string]any, error) { return body(r) }

func IntOf(v any) (int64, bool) { return intOf(v) }

func ArgInt(r *http.Request, name string, required bool) (int64, bool, error) {
	return argInt(r, name, required)
}

// Meta keys of ReadMetadata are exposed for live pairing.
func (m Meta) String(key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
