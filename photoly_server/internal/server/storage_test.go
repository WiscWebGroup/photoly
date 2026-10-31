package server

import (
	"testing"
)

func TestPersonalStorageSettings(t *testing.T) {
	f := newLibraryTest(t)
	t.Setenv("STORAGE_SECRET_KEY", "test")
	out := f.ok("GET", "/user/storage/s3", nil).(map[string]any)
	if len(out["connections"].([]any)) != 0 {
		t.Fatal(out)
	}
	bad := map[string]any{"values": map[string]any{"bucket": "b"}, "enabled": true}
	if v := f.json("PUT", "/user/storage/s3", bad); v["msgCode"] != float64(400) {
		t.Fatal(v)
	}
	t.Setenv("STORAGE_SECRET_KEY", "")
	if v := f.json("POST", "/user/storage/s3/test", bad); v["t"] != "storage_secret_key_missing" {
		t.Fatal(v)
	}
	if w := f.request("GET", "/admin/storage", "", nil); w.Code != 404 {
		t.Fatal("global storage route still exists")
	}
}
