package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/config"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/photo"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/session"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/jackc/pgx/v5"
)

type libraryTest struct {
	t                *testing.T
	a                *app.App
	h                http.Handler
	ctx              context.Context
	uid, root        int64
	uuid, token, dir string
}

func newLibraryTest(t *testing.T) *libraryTest {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cfg := config.Load()
	cfg.DatabaseURL = url
	cfg.UploadFolder = t.TempDir()
	a, h, e := New(ctx, cfg, []byte("{}"))
	if e != nil {
		t.Fatal(e)
	}
	f := &libraryTest{t: t, a: a, h: h, ctx: ctx, uuid: photo.NewUUID()}
	e = a.Pool.QueryRow(ctx, `INSERT INTO "user"(uuid,email,user_name,role,group_id,used_bytes) VALUES($1,$2,'folders','user',(SELECT id FROM user_group WHERE is_default),0) RETURNING user_id`, f.uuid, f.uuid+"@example.test").Scan(&f.uid)
	if e != nil {
		t.Fatal(e)
	}
	if e = a.Pool.QueryRow(ctx, `INSERT INTO namespace(ns_name,ns_parent_id,user_id) VALUES('/',-1,$1) RETURNING ns_id`, f.uid).Scan(&f.root); e != nil {
		t.Fatal(e)
	}
	f.dir = filepath.Join(cfg.UploadFolder, f.uuid)
	if e = os.Mkdir(f.dir, 0755); e != nil {
		t.Fatal(e)
	}
	if e = storage.RecordRootIdentity(ctx, a.Pool, f.uid, f.dir); e != nil {
		t.Fatal(e)
	}
	f.token, e = session.Store{Pool: a.Pool}.Create(ctx, f.uid, 1)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		_, _ = a.Pool.Exec(context.Background(), `DELETE FROM "user" WHERE user_id=$1`, f.uid)
		cancel()
		a.Pool.Close()
	})
	return f
}
func (f *libraryTest) request(method, path, kind string, body io.Reader) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("HRD-Token", f.token)
	if kind != "" {
		req.Header.Set("Content-Type", kind)
	}
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, req)
	return w
}
func (f *libraryTest) json(method, path string, data any) map[string]any {
	f.t.Helper()
	b, _ := json.Marshal(data)
	w := f.request(method, path, "application/json", bytes.NewReader(b))
	var out map[string]any
	if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
		f.t.Fatalf("%s %s = %d %s", method, path, w.Code, w.Body.String())
	}
	return out
}
func (f *libraryTest) ok(method, path string, data any) any {
	f.t.Helper()
	out := f.json(method, path, data)
	if out["msgCode"] != float64(200) {
		f.t.Fatalf("%s %s: %v", method, path, out)
	}
	return out["t"]
}
func (f *libraryTest) folder(parent int64, name string) int64 {
	f.t.Helper()
	v := f.ok("POST", "/namespace/insert", map[string]any{"nsParentId": parent, "nsName": name}).(map[string]any)
	return int64(v["nsId"].(float64))
}
func pngData() []byte {
	var b bytes.Buffer
	im := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := range im.Pix {
		im.Pix[i] = byte(i)
	}
	_ = png.Encode(&b, im)
	return b.Bytes()
}
func (f *libraryTest) upload(ns int64, name string) int64 {
	f.t.Helper()
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	_ = mw.WriteField("photo", fmt.Sprintf(`{"nsId":%d}`, ns))
	fw, _ := mw.CreateFormFile("file", name)
	_, _ = fw.Write(pngData())
	_ = mw.Close()
	w := f.request("POST", "/photo/insert", mw.FormDataContentType(), &b)
	if !strings.Contains(w.Body.String(), `"msgCode":200`) {
		f.t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}
	var id int64
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT max(photo_id) FROM photo WHERE user_id=$1`, f.uid).Scan(&id); e != nil {
		f.t.Fatal(e)
	}
	return id
}
func (f *libraryTest) scan() {
	f.t.Helper()
	if e := photo.For(f.a).Scan(f.ctx, f.uid); e != nil {
		f.t.Fatal(e)
	}
}

func TestRealFolderLifecycle(t *testing.T) {
	f := newLibraryTest(t)
	ns := f.folder(f.root, "旅行")
	empty := f.folder(ns, "empty")
	_ = empty
	pid := f.upload(ns, "camera.png")
	f.upload(ns, "camera.png")
	for _, name := range []string{"camera.png", "camera (1).png"} {
		if _, e := os.Stat(filepath.Join(f.dir, "旅行", name)); e != nil {
			t.Fatal(e)
		}
	}
	f.ok("POST", "/namespace/updateName", map[string]any{"nsId": ns, "nsName": "Holiday"})
	if _, e := os.Stat(filepath.Join(f.dir, "Holiday", "camera.png")); e != nil {
		t.Fatal(e)
	}
	data := f.request("GET", fmt.Sprintf("/photo/render/%s?photoId=%d", f.token, pid), "", nil)
	if data.Code != 200 || !bytes.Equal(data.Body.Bytes(), pngData()) {
		t.Fatalf("render after rename: %d %s", data.Code, data.Body.String())
	}
	os.WriteFile(filepath.Join(f.dir, "Holiday", "notes.txt"), []byte("notes"), 0644)
	os.Symlink("/etc/passwd", filepath.Join(f.dir, "Holiday", "do-not-follow"))
	w := f.request("GET", fmt.Sprintf("/namespace/downloadNs/%s?nsId=%d", f.token, ns), "", nil)
	zr, e := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if e != nil {
		t.Fatalf("zip: %d %s", w.Code, w.Body.String())
	}
	names := map[string]bool{}
	for _, entry := range zr.File {
		names[entry.Name] = true
	}
	for _, name := range []string{"Holiday/", "Holiday/empty/", "Holiday/notes.txt", "Holiday/camera.png", "Holiday/camera (1).png"} {
		if !names[name] {
			t.Fatalf("missing %s in %v", name, names)
		}
	}
	if names["Holiday/do-not-follow"] {
		t.Fatal("zip followed symlink")
	}
	// Moving then failing rolls back both the directory entry and DB metadata.
	err := photo.For(f.a).Atomic(f.ctx, f.uid, func(tx pgx.Tx, u photo.UserRow, files *photo.Files) error {
		_, e := storage.RehomePhoto(f.ctx, tx, files, f.a.Cfg.UploadFolder, f.uid, pid, f.root, "")
		if e != nil {
			return e
		}
		return errors.New("abort")
	})
	if err == nil {
		t.Fatal("expected abort")
	}
	if _, e = os.Stat(filepath.Join(f.dir, "Holiday", "camera.png")); e != nil {
		t.Fatal("rollback failed", e)
	}
	f.ok("POST", fmt.Sprintf("/namespace/delete?nsId=%d", ns), nil)
	if _, e = os.Stat(filepath.Join(f.dir, "Holiday")); !os.IsNotExist(e) {
		t.Fatalf("folder remained: %v", e)
	}
}

func TestRealFolderScan(t *testing.T) {
	f := newLibraryTest(t)
	f.scan()
	os.Mkdir(filepath.Join(f.dir, "outside"), 0755)
	os.Mkdir(filepath.Join(f.dir, "outside", "empty"), 0755)
	path := filepath.Join(f.dir, "outside", "external.png")
	os.WriteFile(path, pngData(), 0644)
	f.scan()
	var n int
	f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM photo WHERE user_id=$1`, f.uid).Scan(&n)
	if n != 0 {
		t.Fatal("indexed unstable first observation")
	}
	f.scan()
	var id, ns int64
	var token string
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT photo_id,ns_id,token FROM photo WHERE user_id=$1`, f.uid).Scan(&id, &ns, &token); e != nil {
		t.Fatal(e)
	}
	os.Rename(filepath.Join(f.dir, "outside"), filepath.Join(f.dir, "renamed"))
	f.scan()
	f.scan()
	var newNS int64
	f.a.Pool.QueryRow(f.ctx, `SELECT ns_id FROM namespace WHERE user_id=$1 AND ns_name='renamed'`, f.uid).Scan(&newNS)
	if newNS != ns {
		t.Fatal("directory identity lost")
	}
	os.Rename(filepath.Join(f.dir, "renamed", "external.png"), filepath.Join(f.dir, "different.png"))
	f.scan()
	f.scan()
	var got int64
	f.a.Pool.QueryRow(f.ctx, `SELECT photo_id FROM photo WHERE user_id=$1 AND file_name='different.png'`, f.uid).Scan(&got)
	if got != id {
		t.Fatalf("photo identity lost: %d -> %d", id, got)
	}
	// Same-path replacement increments content version without dropping its token.
	different := image.NewRGBA(image.Rect(0, 0, 9, 9))
	var b bytes.Buffer
	png.Encode(&b, different)
	os.WriteFile(filepath.Join(f.dir, "different.png"), b.Bytes(), 0644)
	f.scan()
	f.scan()
	var cv int64
	var newToken string
	f.a.Pool.QueryRow(f.ctx, `SELECT content_version,token FROM photo WHERE photo_id=$1`, id).Scan(&cv, &newToken)
	if cv != 2 || newToken != token {
		t.Fatalf("replace: version %d token %s", cv, newToken)
	}
	os.Remove(filepath.Join(f.dir, "different.png"))
	f.scan()
	f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM photo WHERE photo_id=$1`, id).Scan(&n)
	if n != 1 {
		t.Fatal("deleted after one scan")
	}
	f.scan()
	f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM photo WHERE photo_id=$1`, id).Scan(&n)
	if n != 0 {
		t.Fatal("not deleted after two scans")
	}
	// Removing the whole source is an error, never a successful empty scan.
	ns = f.folder(f.root, "keep")
	os.Rename(f.dir, f.dir+"-offline")
	if e := photo.For(f.a).Scan(f.ctx, f.uid); e == nil {
		t.Fatal("offline root accepted")
	}
	f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM namespace WHERE ns_id=$1`, ns).Scan(&n)
	if n != 1 {
		t.Fatal("offline root deleted records")
	}
	if v := f.json("POST", "/namespace/insert", map[string]any{"nsParentId": f.root, "nsName": "must fail"}); v["msgCode"] == float64(200) {
		t.Fatal("app recreated offline root")
	}
	if _, e := os.Stat(f.dir); !os.IsNotExist(e) {
		t.Fatal("offline root was recreated", e)
	}
}

func TestRealFolderConcurrentUploads(t *testing.T) {
	f := newLibraryTest(t)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); f.upload(f.root, "same.png") }()
	}
	wg.Wait()
	files, e := os.ReadDir(f.dir)
	if e != nil {
		t.Fatal(e)
	}
	if len(files) != 4 {
		t.Fatalf("got %d files", len(files))
	}
}

func TestRealFolderInitialRoot(t *testing.T) {
	f := newLibraryTest(t)
	if _, e := f.a.Pool.Exec(f.ctx, `UPDATE storage_source SET root_identity='' WHERE user_id=$1 AND is_default`, f.uid); e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(f.dir); e != nil {
		t.Fatal(e)
	}
	if e := storage.EnsureRoots(f.ctx, f.a.Pool, f.a.Cfg.UploadFolder); e != nil {
		t.Fatal(e)
	}
	if st, e := os.Stat(f.dir); e != nil || !st.IsDir() {
		t.Fatalf("root not initialized: %v", e)
	}
	f.scan()
	if e := os.Remove(f.dir); e != nil {
		t.Fatal(e)
	}
	if e := storage.EnsureRoots(f.ctx, f.a.Pool, f.a.Cfg.UploadFolder); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(f.dir); !os.IsNotExist(e) {
		t.Fatalf("recreated established missing root: %v", e)
	}
}

func TestRealFolderMoveAndNameReuse(t *testing.T) {
	f := newLibraryTest(t)
	id := f.upload(f.root, "a.png")
	other := f.upload(f.root, "other.png")
	renamed := f.ok("POST", "/photo/update", map[string]any{"photoId": other, "photoName": "a"}).(map[string]any)
	if renamed["photoName"] != "a (1)" {
		t.Fatalf("wrong resolved name: %v", renamed)
	}
	if e := os.Rename(filepath.Join(f.dir, "a.png"), filepath.Join(f.dir, "z.png")); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(f.dir, "a.png"), pngData(), 0644); e != nil {
		t.Fatal(e)
	}
	f.scan()
	f.scan()
	var got int64
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT photo_id FROM photo WHERE user_id=$1 AND file_name='z.png'`, f.uid).Scan(&got); e != nil || got != id {
		t.Fatalf("inode lost to reused path: %d %v", got, e)
	}
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT photo_id FROM photo WHERE user_id=$1 AND file_name='a.png'`, f.uid).Scan(&got); e != nil || got == id {
		t.Fatalf("new file stole old ID: %d %v", got, e)
	}
	oldDir := f.folder(f.root, "a")
	if e := os.Rename(filepath.Join(f.dir, "a"), filepath.Join(f.dir, "z")); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(filepath.Join(f.dir, "a"), 0755); e != nil {
		t.Fatal(e)
	}
	f.scan()
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT ns_id FROM namespace WHERE user_id=$1 AND ns_name='z'`, f.uid).Scan(&got); e != nil || got != oldDir {
		t.Fatalf("directory ID lost: %d %v", got, e)
	}
}

func TestRealFolderHashIdentityAndDuplicates(t *testing.T) {
	f := newLibraryTest(t)
	id := f.upload(f.root, "original.png")
	// Create before deleting to prevent inode reuse; content hash is the only match.
	if e := os.WriteFile(filepath.Join(f.dir, "copy.png"), pngData(), 0644); e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(filepath.Join(f.dir, "original.png")); e != nil {
		t.Fatal(e)
	}
	f.scan()
	f.scan()
	var got int64
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT photo_id FROM photo WHERE user_id=$1 AND file_name='copy.png'`, f.uid).Scan(&got); e != nil || got != id {
		t.Fatalf("unique hash lost ID: %d %v", got, e)
	}
	for _, name := range []string{"duplicate1.png", "duplicate2.png"} {
		if e := os.WriteFile(filepath.Join(f.dir, name), pngData(), 0644); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.Remove(filepath.Join(f.dir, "copy.png")); e != nil {
		t.Fatal(e)
	}
	f.scan()
	f.scan()
	var count int
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM photo WHERE user_id=$1`, f.uid).Scan(&count); e != nil || count != 2 {
		t.Fatalf("duplicates merged: %d %v", count, e)
	}
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM photo WHERE photo_id=$1`, id).Scan(&count); e != nil || count != 0 {
		t.Fatalf("ambiguous hash retained ID: %d %v", count, e)
	}
}

func TestRealFolderExternalQuotaAndMissingDownload(t *testing.T) {
	f := newLibraryTest(t)
	if _, e := f.a.Pool.Exec(f.ctx, `UPDATE "user" SET storage_override=true,max_storage_bytes=1 WHERE user_id=$1`, f.uid); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(f.dir, "notes.txt"), []byte("nonmedia uses quota too"), 0644); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(f.dir, "external.png"), pngData(), 0644); e != nil {
		t.Fatal(e)
	}
	f.scan()
	f.scan()
	v := f.ok("GET", "/user/getStorage", nil).(map[string]any)
	if v["overQuota"] != true || int64(v["usedBytes"].(float64)) != storage.Usage(f.a.Cfg.UploadFolder, f.uuid) {
		t.Fatalf("quota: %v", v)
	}
	// A failed application addition is compensated without dropping external data.
	err := photo.For(f.a).Atomic(f.ctx, f.uid, func(tx pgx.Tx, u photo.UserRow, files *photo.Files) error {
		_, e := files.InstallNew(strings.NewReader("new"), filepath.Join(f.dir, "blocked.txt"))
		return e
	})
	if err == nil {
		t.Fatal("over-quota write accepted")
	}
	if _, e := os.Stat(filepath.Join(f.dir, "blocked.txt")); !os.IsNotExist(e) {
		t.Fatal("quota rollback failed", e)
	}
	if e := os.Remove(filepath.Join(f.dir, "external.png")); e != nil {
		t.Fatal(e)
	}
	w := f.request("GET", fmt.Sprintf("/namespace/downloadNs/%s?nsId=%d", f.token, f.root), "", nil)
	if w.Header().Get("Content-Type") == "application/zip" {
		t.Fatal("missing indexed original silently omitted")
	}
}

func TestScanDirectoryReplacedByOrdinaryFile(t *testing.T) {
	f := newLibraryTest(t)
	ns := f.folder(f.root, "was-a-folder")
	id := f.upload(ns, "original.png")
	f.scan()
	if e := os.RemoveAll(filepath.Join(f.dir, "was-a-folder")); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(f.dir, "was-a-folder"), []byte("ordinary file"), 0644); e != nil {
		t.Fatal(e)
	}
	f.scan()
	f.scan()
	var count int
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM photo WHERE photo_id=$1`, id).Scan(&count); e != nil || count != 0 {
		t.Fatalf("retained replaced photo: %d %v", count, e)
	}
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM namespace WHERE ns_id=$1`, ns).Scan(&count); e != nil || count != 0 {
		t.Fatalf("retained replaced directory: %d %v", count, e)
	}
	if b, e := os.ReadFile(filepath.Join(f.dir, "was-a-folder")); e != nil || string(b) != "ordinary file" {
		t.Fatal("scanner removed replacement file")
	}
}

func TestRealFolderLiveAssets(t *testing.T) {
	f := newLibraryTest(t)
	id := f.upload(f.root, "still.png")
	unrelated := filepath.Join(f.dir, "still_live.mov")
	if e := os.WriteFile(unrelated, []byte("unrelated external file"), 0644); e != nil {
		t.Fatal(e)
	}
	f.ok("POST", "/photo/update", map[string]any{"photoId": id, "photoName": "renamed"})
	f.ok("POST", fmt.Sprintf("/photo/delete?photoId=%d", id), nil)
	if b, e := os.ReadFile(unrelated); e != nil || string(b) != "unrelated external file" {
		t.Fatalf("non-live photo touched an unrelated file: %q %v", b, e)
	}
	id = f.upload(f.root, "live.png")
	var uuid string
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT photo_uuid FROM photo WHERE photo_id=$1`, id).Scan(&uuid); e != nil {
		t.Fatal(e)
	}
	err := photo.For(f.a).Atomic(f.ctx, f.uid, func(tx pgx.Tx, u photo.UserRow, files *photo.Files) error {
		p, e := storage.Resolve(f.ctx, tx, f.a.Cfg.UploadFolder, f.uuid, uuid, "live_original")
		if e != nil {
			return e
		}
		if _, e = files.InstallNew(strings.NewReader("original live movie"), p); e != nil {
			return e
		}
		_, e = tx.Exec(f.ctx, `UPDATE photo_media SET live=true WHERE photo_id=$1`, id)
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	ns := f.folder(f.root, "destination")
	f.ok("POST", fmt.Sprintf("/photo/changeNamespace?photoId=%d&nsId=%d", id, ns), nil)
	if b, e := os.ReadFile(filepath.Join(f.dir, "destination", "live_live.mov")); e != nil || string(b) != "original live movie" {
		t.Fatalf("companion move failed: %v", e)
	}
	f.scan()
	f.scan()
	var n int
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM photo WHERE user_id=$1`, f.uid).Scan(&n); e != nil || n != 2 {
		t.Fatalf("companion imported separately: %d %v", n, e)
	}
	f.ok("POST", fmt.Sprintf("/photo/delete?photoId=%d", id), nil)
	if _, e := os.Stat(filepath.Join(f.dir, "destination", "live_live.mov")); !os.IsNotExist(e) {
		t.Fatalf("companion not removed: %v", e)
	}
}

func TestRealFolderHEICAndVideo(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "heif-enc", "heif-convert"} {
		if _, e := exec.LookPath(tool); e != nil {
			t.Skip("media tools required: " + tool)
		}
	}
	f := newLibraryTest(t)
	src := filepath.Join(t.TempDir(), "source.png")
	if e := os.WriteFile(src, pngData(), 0644); e != nil {
		t.Fatal(e)
	}
	heic := filepath.Join(f.dir, "camera.HEIC")
	if b, e := exec.Command("heif-enc", "-q", "60", src, "-o", heic).CombinedOutput(); e != nil {
		t.Fatalf("heif fixture: %s %v", b, e)
	}
	video := filepath.Join(f.dir, "clip.mp4")
	if b, e := exec.Command("ffmpeg", "-loglevel", "error", "-f", "lavfi", "-i", "color=blue:s=32x32:d=0.2", "-c:v", "libx264", "-pix_fmt", "yuv420p", video).CombinedOutput(); e != nil {
		t.Fatalf("video fixture: %s %v", b, e)
	}
	f.scan()
	f.scan()
	rows, e := f.a.Pool.Query(f.ctx, `SELECT photo_id,photo_uuid,file_name FROM photo WHERE user_id=$1`, f.uid)
	if e != nil {
		t.Fatal(e)
	}
	type item struct {
		id         int64
		uuid, name string
	}
	var items []item
	for rows.Next() {
		var v item
		if e = rows.Scan(&v.id, &v.uuid, &v.name); e != nil {
			t.Fatal(e)
		}
		items = append(items, v)
	}
	rows.Close()
	if len(items) != 2 {
		t.Fatalf("media missing: %v", items)
	}
	for _, v := range items {
		w := f.request("GET", fmt.Sprintf("/photo/download/%s?photoId=%d", f.token, v.id), "", nil)
		original, e := os.ReadFile(filepath.Join(f.dir, v.name))
		if e != nil || !bytes.Equal(w.Body.Bytes(), original) {
			t.Fatalf("original changed: %s %v", v.name, e)
		}
		if v.name == "camera.HEIC" {
			for _, role := range []string{"thumbnail", "preview"} {
				p, e := storage.Resolve(f.ctx, f.a.Pool, f.a.Cfg.UploadFolder, f.uuid, v.uuid, role)
				if e != nil {
					t.Fatal(e)
				}
				if st, e := os.Stat(p); e != nil || st.Size() == 0 {
					t.Fatalf("HEIC derivative missing: %s %v", role, e)
				}
			}
		}
	}
}
