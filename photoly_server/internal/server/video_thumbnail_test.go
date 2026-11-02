package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/photo"
)

func TestVideoThumbnailQueueLifecycle(t *testing.T) {
	if _, e := exec.LookPath("ffmpeg"); e != nil {
		t.Skip("ffmpeg required")
	}
	f := newLibraryTest(t)
	path := filepath.Join(t.TempDir(), "movie.mp4")
	if out, e := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=640x360:d=0.2", "-pix_fmt", "yuv420p", path).CombinedOutput(); e != nil {
		t.Fatalf("fixture: %v %s", e, out)
	}
	content, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	w.WriteField("photo", fmt.Sprintf(`{"nsId":%d}`, f.root))
	part, _ := w.CreateFormFile("file", "movie.mp4")
	part.Write(content)
	w.Close()
	res := f.request("POST", "/photo/insert", w.FormDataContentType(), &body)
	var response map[string]any
	json.Unmarshal(res.Body.Bytes(), &response)
	if response["msgCode"] != float64(200) {
		t.Fatal(res.Body.String())
	}
	var pid int64
	f.a.Pool.QueryRow(f.ctx, `SELECT max(photo_id) FROM photo WHERE user_id=$1`, f.uid).Scan(&pid)
	url := fmt.Sprintf("/photo/renderThumbnail/%s?photoId=%d", f.token, pid)
	before := f.request("GET", url, "", nil)
	if before.Code == 200 && bytes.Equal(before.Body.Bytes(), content) {
		t.Fatal("served original video as thumbnail")
	}
	// Other tests may leave queued videos in this shared integration database.
	f.a.Pool.Exec(f.ctx, `UPDATE video_thumbnail_job SET next_attempt='1970-01-01' WHERE photo_id=$1`, pid)
	q := dbgen.New(f.a.Pool)
	first, e := q.ClaimVideoThumbnail(f.ctx, "old-lease")
	if e != nil || first.PhotoID != pid {
		t.Fatal(first, e)
	}
	if second, e := q.ClaimVideoThumbnail(f.ctx, "second"); e == nil && second.PhotoID == pid {
		t.Fatal("concurrent claim accepted")
	}
	f.a.Pool.Exec(f.ctx, `UPDATE photo SET content_version=content_version+1 WHERE photo_id=$1`, pid)
	var lease *string
	f.a.Pool.QueryRow(f.ctx, `SELECT lease_token FROM video_thumbnail_job WHERE photo_id=$1`, pid).Scan(&lease)
	if lease != nil {
		t.Fatal("old lease survived replacement")
	}
	f.a.Pool.Exec(f.ctx, `UPDATE video_thumbnail_job SET state='processing',lease_token='expired',next_attempt=now()-interval '1 second' WHERE photo_id=$1`, pid)
	ctx, cancel := context.WithCancel(f.ctx)
	done := make(chan struct{})
	go func() { defer close(done); photo.VideoThumbnailWorker(ctx, f.a) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(20 * time.Second)
	for {
		var state string
		f.a.Pool.QueryRow(f.ctx, `SELECT state FROM video_thumbnail_job WHERE photo_id=$1`, pid).Scan(&state)
		if state == "ready" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("thumbnail did not complete", state)
		}
		time.Sleep(100 * time.Millisecond)
	}
	result := f.request("GET", url, "", nil)
	if result.Code != 200 || result.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatal(result.Code, result.Header(), result.Body.String())
	}
	if _, e = jpeg.Decode(bytes.NewReader(result.Body.Bytes())); e != nil {
		t.Fatal(e)
	}
	f.a.Pool.Exec(f.ctx, `DELETE FROM photo WHERE photo_id=$1`, pid)
	var n int
	f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM video_thumbnail_job WHERE photo_id=$1`, pid).Scan(&n)
	if n != 0 {
		t.Fatal("orphan job")
	}
}
