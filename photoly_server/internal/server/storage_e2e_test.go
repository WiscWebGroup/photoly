package server

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/config"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/session"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
)

// Uploads a photo with an S3 backend active and follows it through offload, serve,
// quota and delete. Needs TEST_DATABASE_URL and the PHOTOLY_TEST_S3_* variables.
func TestPhotoLifecycleOnS3(t *testing.T) {
	url, ep := os.Getenv("TEST_DATABASE_URL"), os.Getenv("PHOTOLY_TEST_S3_ENDPOINT")
	if url == "" || ep == "" {
		t.Skip("set TEST_DATABASE_URL and PHOTOLY_TEST_S3_ENDPOINT")
	}
	t.Setenv("STORAGE_SECRET_KEY", "test-key")
	cfg := config.Load()
	cfg.DatabaseURL, cfg.UploadFolder = url, t.TempDir()
	ctx := context.Background()
	a, h, err := New(ctx, cfg, []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		a.Pool.Exec(ctx, `UPDATE storage_config SET driver='local', backend_id='local', config='{}', secret=NULL WHERE id=1`)
		a.Pool.Exec(ctx, `DELETE FROM storage_driver_config`)
		storage.Set("local", storage.Local{Dir: cfg.UploadFolder})
	})
	srv := httptest.NewServer(h)
	defer srv.Close()

	var uid int64
	var uuid string
	if err := a.Pool.QueryRow(ctx, `SELECT user_id, uuid FROM "user" WHERE role='admin' LIMIT 1`).Scan(&uid, &uuid); err != nil {
		t.Fatal(err)
	}
	tok, _ := session.Store{Pool: a.Pool}.Create(ctx, uid, 1)
	do := func(method, path, ctype string, body io.Reader) (int, []byte) {
		req, _ := http.NewRequest(method, srv.URL+path, body)
		req.Header.Set("HRD-Token", tok)
		if ctype != "" {
			req.Header.Set("Content-Type", ctype)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, b
	}
	ok := func(b []byte) {
		t.Helper()
		var m map[string]any
		if json.Unmarshal(b, &m) != nil || m["msgCode"] != 200.0 {
			t.Fatalf("reply = %s", b)
		}
	}

	// A photo that already lives on local disk before the switch.
	var stray int64
	if err := a.Pool.QueryRow(ctx, `INSERT INTO photo(photo_name,user_id,photo_uuid,format,storage_id) VALUES ('stray',$1,'stray-e2e','png','local') RETURNING photo_id`, uid).Scan(&stray); err != nil {
		t.Fatal(err)
	}
	defer a.Pool.Exec(ctx, `DELETE FROM photo WHERE photo_id=$1`, stray)

	cfgJSON, _ := json.Marshal(map[string]any{"driver": "s3", "values": map[string]any{
		"endpoint": ep, "bucket": os.Getenv("PHOTOLY_TEST_S3_BUCKET"), "prefix": "e2e",
		"access_key": os.Getenv("PHOTOLY_TEST_S3_ACCESS_KEY"), "secret_key": os.Getenv("PHOTOLY_TEST_S3_SECRET_KEY"),
		"use_ssl": false, "path_style": true}, "confirm": true})
	_, b := do("PUT", "/admin/storage", "application/json", bytes.NewReader(cfgJSON))
	ok(b)

	var ns int64
	if err := a.Pool.QueryRow(ctx, `SELECT ns_id FROM namespace WHERE user_id=$1 LIMIT 1`, uid).Scan(&ns); err != nil {
		if err = a.Pool.QueryRow(ctx, `INSERT INTO namespace(ns_name, user_id) VALUES ('e2e',$1) RETURNING ns_id`, uid).Scan(&ns); err != nil {
			t.Fatal(err)
		}
	}
	var used0 int64
	a.Pool.QueryRow(ctx, `SELECT coalesce(used_bytes,0) FROM "user" WHERE user_id=$1`, uid).Scan(&used0)

	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for i := range img.Pix {
		img.Pix[i] = byte(i * 7)
	}
	var pngBytes bytes.Buffer
	png.Encode(&pngBytes, img)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("photo", `{"nsId":`+itoa(ns)+`}`)
	fw, _ := mw.CreateFormFile("file", "e2e.png")
	fw.Write(pngBytes.Bytes())
	mw.Close()
	_, b = do("POST", "/photo/insert", mw.FormDataContentType(), &body)
	ok(b)

	var pid int64
	var puuid string
	if err := a.Pool.QueryRow(ctx, `SELECT photo_id, photo_uuid FROM photo WHERE user_id=$1 ORDER BY photo_id DESC LIMIT 1`, uid).Scan(&pid, &puuid); err != nil {
		t.Fatal(err)
	}
	// Only media on the active backend is listed; switching back reveals the other side.
	visible := func(id int64) bool {
		var n int
		a.Pool.QueryRow(ctx, `SELECT count(*) FROM photo WHERE photo_id=$1 AND photo_visible(storage_id, user_id)`, id).Scan(&n)
		return n == 1
	}
	if !visible(pid) || visible(stray) {
		t.Fatalf("on s3: new visible=%v (want true), local stray visible=%v (want false)", visible(pid), visible(stray))
	}
	// The user can opt in to listing media from the other backends too.
	_, b = do("POST", "/user/updateStorageVisibility", "application/x-www-form-urlencoded", strings.NewReader("show=true"))
	ok(b)
	if !visible(pid) || !visible(stray) {
		t.Fatal("show-all setting should list media from every backend")
	}
	_, b = do("POST", "/user/updateStorageVisibility", "application/x-www-form-urlencoded", strings.NewReader("show=false"))
	ok(b)
	localJSON, _ := json.Marshal(map[string]any{"driver": "local", "values": map[string]any{}, "confirm": true})
	_, b = do("PUT", "/admin/storage", "application/json", bytes.NewReader(localJSON))
	ok(b)
	if visible(pid) || !visible(stray) {
		t.Fatalf("on local: new visible=%v (want false), stray visible=%v (want true)", visible(pid), visible(stray))
	}
	// The s3 settings were remembered: switching back needs no credentials.
	_, b = do("GET", "/admin/storage", "", nil)
	var saved struct {
		T struct {
			Saved map[string]struct {
				Values    map[string]string `json:"values"`
				HasSecret bool              `json:"hasSecret"`
			} `json:"saved"`
		} `json:"t"`
	}
	if json.Unmarshal(b, &saved) != nil || saved.T.Saved["s3"].Values["bucket"] == "" || !saved.T.Saved["s3"].HasSecret {
		t.Fatalf("s3 settings not remembered after switching to local: %s", b)
	}
	backJSON, _ := json.Marshal(map[string]any{"driver": "s3", "values": map[string]any{
		"endpoint": ep, "bucket": os.Getenv("PHOTOLY_TEST_S3_BUCKET"), "prefix": "e2e",
		"access_key": os.Getenv("PHOTOLY_TEST_S3_ACCESS_KEY"), "use_ssl": false, "path_style": true}, "confirm": true})
	_, b = do("PUT", "/admin/storage", "application/json", bytes.NewReader(backJSON))
	ok(b)
	if !visible(pid) || visible(stray) {
		t.Fatal("switching back to s3 should show its media again")
	}
	remote := storage.Remote()
	if remote == nil {
		t.Fatal("s3 should be active")
	}
	orig, thumb := uuid+"/"+puuid+".png", uuid+"/"+puuid+"_thumbnail.png"
	for _, k := range []string{orig, thumb} {
		if _, err := remote.Stat(ctx, k); err != nil {
			t.Fatalf("%s not uploaded: %v", k, err)
		}
		if _, err := os.Stat(filepath.Join(cfg.UploadFolder, filepath.FromSlash(k))); !os.IsNotExist(err) {
			t.Fatalf("%s should be offloaded from local disk", k)
		}
	}
	var used1 int64
	a.Pool.QueryRow(ctx, `SELECT coalesce(used_bytes,0) FROM "user" WHERE user_id=$1`, uid).Scan(&used1)
	if used1 <= used0 {
		t.Fatalf("used_bytes %d -> %d, upload should count", used0, used1)
	}

	// Serving fetches the original back from S3.
	code, got := do("GET", "/photo/render/"+tok+"?photoId="+itoa(pid), "", nil)
	if code != 200 || !bytes.Equal(got, pngBytes.Bytes()) {
		t.Fatalf("render = %d, %d bytes (want %d)", code, len(got), pngBytes.Len())
	}
	if code, got = do("GET", "/photo/renderThumbnail/"+tok+"?photoId="+itoa(pid), "", nil); code != 200 || len(got) == 0 {
		t.Fatalf("thumbnail = %d, %d bytes", code, len(got))
	}
	// Listing sizes must come from S3 even when nothing is cached locally.
	if n, err := storage.Size(filepath.Join(cfg.UploadFolder, uuid, puuid+".png")); err != nil || n != int64(pngBytes.Len()) {
		t.Fatalf("Size = %d, %v", n, err)
	}

	// Delete: the objects go, and the quota is released even though the file was offloaded.
	_, b = do("POST", "/photo/delete?photoId="+itoa(pid), "", strings.NewReader(""))
	ok(b)
	for _, k := range []string{orig, thumb} {
		if _, err := remote.Stat(ctx, k); err != storage.ErrNotFound {
			t.Fatalf("%s should be deleted from S3: %v", k, err)
		}
	}
	var used2 int64
	a.Pool.QueryRow(ctx, `SELECT coalesce(used_bytes,0) FROM "user" WHERE user_id=$1`, uid).Scan(&used2)
	if used2 != used0 {
		t.Fatalf("used_bytes after delete = %d, want %d", used2, used0)
	}
	if code, _ = do("GET", "/photo/render/"+tok+"?photoId="+itoa(pid), "", nil); code == 200 {
		t.Fatal("deleted photo still served")
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
