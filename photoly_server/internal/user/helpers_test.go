package user

import (
	"bytes"
	"encoding/json"
	"image"
	_ "image/jpeg"
	"strconv"
	"testing"
)

func decode(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("not JSON: %q", raw)
	}
	return m
}

func decodeCfg(b []byte) (image.Config, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	return cfg, err
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func itoaF(v any) string { return strconv.FormatInt(int64(v.(float64)), 10) }
