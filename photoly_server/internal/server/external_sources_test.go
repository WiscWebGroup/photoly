package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
	"github.com/jackc/pgx/v5"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/photo"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/testutil/davserver"
)

func externalBinding(t *testing.T, f *libraryTest, kind string) (string, string, map[string]any) {
	t.Helper()
	dir := t.TempDir()
	input := map[string]any{"name": "Mounted", "enabled": true, "scanInterval": 60}
	if kind == "local" {
		roots, _ := json.Marshal([]map[string]string{{"id": "test", "name": "Test disk", "path": dir}})
		t.Setenv("STORAGE_LOCAL_ROOTS", string(roots))
		if _, e := f.a.Pool.Exec(f.ctx, `UPDATE "user" SET role='admin' WHERE user_id=$1`, f.uid); e != nil {
			t.Fatal(e)
		}
		input["rootId"] = "test"
		input["relativePath"] = ""
		return dir, fmt.Sprintf("/admin/users/%d/storage/local", f.uid), input
	}
	if kind == "smb" {
		shareRoot := os.Getenv("SMB_FIXTURE_ROOT")
		if shareRoot == "" {
			t.Skip("SMB_FIXTURE_ROOT required")
		}
		dir, e := os.MkdirTemp(shareRoot, "library-")
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { os.RemoveAll(dir) })
		t.Setenv("STORAGE_SECRET_KEY", strings.Repeat("a", 64))
		t.Setenv("STORAGE_SMB_ALLOWED_HOSTS", "127.0.0.1")
		input["server"] = "127.0.0.1"
		input["share"] = "fixture"
		input["subdirectory"] = filepath.Base(dir)
		input["username"] = "photolytest"
		input["password"] = "fixture-password"
		return dir, "/user/storage/smb", input
	}
	h := davserver.Handler(dir)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, _ := r.BasicAuth()
		if u != "user" || p != "password" {
			w.WriteHeader(401)
			return
		}
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	t.Setenv("STORAGE_WEBDAV_ALLOWED_HOSTS", server.URL)
	t.Setenv("STORAGE_SECRET_KEY", strings.Repeat("a", 64))
	input["url"] = server.URL + "/"
	input["username"] = "user"
	input["password"] = "password"
	return dir, "/user/storage/webdav", input
}
func TestExternalSourceLifecycle(t *testing.T) {
	for _, kind := range []string{"local", "webdav", "smb"} {
		t.Run(kind, func(t *testing.T) {
			f := newLibraryTest(t)
			dir, endpoint, input := externalBinding(t, f, kind)
			created := f.ok("POST", endpoint, input).(map[string]any)
			sid := int64(created["sourceId"].(float64))
			var mount int64
			if e := f.a.Pool.QueryRow(f.ctx, `SELECT ns_id FROM namespace WHERE source_id=$1 AND is_mount`, sid).Scan(&mount); e != nil {
				t.Fatal(e)
			}
			child := f.folder(mount, "中文 & folder")
			pid := f.upload(child, "写真 %.png")
			if _, e := os.Stat(filepath.Join(dir, "中文 & folder", "写真 %.png")); e != nil {
				t.Fatal(e)
			}
			// Directory moves must change disk, and object IDs remain stable.
			f.ok("POST", "/namespace/updateName", map[string]any{"nsId": child, "nsName": "renamed"})
			if _, e := os.Stat(filepath.Join(dir, "renamed", "写真 %.png")); e != nil {
				t.Fatal(e)
			}
			_ = pid
			external := filepath.Join(dir, "external.png")
			if e := os.WriteFile(external, pngData(), 0600); e != nil {
				t.Fatal(e)
			}
			scanner := photo.For(f.a)
			for i := 0; i < 2; i++ {
				if e := scanner.ScanSource(f.ctx, sid); e != nil {
					t.Fatal(e)
				}
			}
			var externalID int64
			if e := f.a.Pool.QueryRow(f.ctx, `SELECT photo_id FROM photo WHERE source_id=$1 AND file_name='external.png'`, sid).Scan(&externalID); e != nil {
				t.Fatal(e)
			}
			if e := os.Rename(external, filepath.Join(dir, "moved.png")); e != nil {
				t.Fatal(e)
			}
			for i := 0; i < 2; i++ {
				if e := scanner.ScanSource(f.ctx, sid); e != nil {
					t.Fatal(e)
				}
			}
			var movedID int64
			if e := f.a.Pool.QueryRow(f.ctx, `SELECT photo_id FROM photo WHERE source_id=$1 AND file_name='moved.png'`, sid).Scan(&movedID); e != nil || movedID != externalID {
				t.Fatalf("lost identity %d -> %d: %v", externalID, movedID, e)
			}
			if _, e := scanner.Move(f.ctx, f.uid, externalID, child); e != nil {
				t.Fatal(e)
			}
			if _, e := scanner.Move(f.ctx, f.uid, externalID, mount); e != nil {
				t.Fatal(e)
			}
			// Duplicate bindings maintain separate indexes.
			again := f.ok("POST", endpoint, input).(map[string]any)
			sid2 := int64(again["sourceId"].(float64))
			for i := 0; i < 2; i++ {
				if e := scanner.ScanSource(f.ctx, sid2); e != nil {
					t.Fatal(e)
				}
			}
			var duplicateID int64
			if e := f.a.Pool.QueryRow(f.ctx, `SELECT photo_id FROM photo WHERE source_id=$1 AND file_name='moved.png'`, sid2).Scan(&duplicateID); e != nil || duplicateID == externalID {
				t.Fatalf("merged bindings: %v", e)
			}
			if e := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not media"), 0600); e != nil {
				t.Fatal(e)
			}
			if e := os.Mkdir(filepath.Join(dir, "empty"), 0700); e != nil {
				t.Fatal(e)
			}
			w := f.request("GET", fmt.Sprintf("/namespace/downloadNs/%s?nsId=%d", f.token, mount), "", nil)
			archive, e := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
			if e != nil {
				t.Fatalf("zip %d %s: %v", w.Code, w.Body.String(), e)
			}
			names := map[string]bool{}
			for _, v := range archive.File {
				names[v.Name] = true
				if strings.Contains(v.Name, ".photoly-system") {
					t.Fatal("workspace leaked")
				}
			}
			if !names["Mounted/notes.txt"] || !names["Mounted/empty/"] {
				t.Fatalf("incomplete zip %v", names)
			}
			if e = os.Remove(filepath.Join(dir, "moved.png")); e != nil {
				t.Fatal(e)
			}
			if e = scanner.ScanSource(f.ctx, sid); e != nil {
				t.Fatal(e)
			}
			var count int
			if e = f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM photo WHERE photo_id=$1`, externalID).Scan(&count); e != nil || count != 1 {
				t.Fatal("deleted on first absence")
			}
			if e = scanner.ScanSource(f.ctx, sid); e != nil {
				t.Fatal(e)
			}
			if e = f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM photo WHERE photo_id=$1`, externalID).Scan(&count); e != nil || count != 0 {
				t.Fatal("not deleted after confirmation")
			}
			f.ok("DELETE", fmt.Sprintf("%s/%d", endpoint, sid), nil)
			if _, e = os.Stat(filepath.Join(dir, "renamed", "写真 %.png")); e != nil {
				t.Fatal("detach deleted original", e)
			}
			f.ok("DELETE", fmt.Sprintf("%s/%d", endpoint, sid2), nil)
		})
	}
}

func TestExternalRetentionAndAccess(t *testing.T) {
	for _, kind := range []string{"local", "webdav", "smb"} {
		t.Run(kind, func(t *testing.T) {
			f := newLibraryTest(t)
			dir, endpoint, input := externalBinding(t, f, kind)
			source := f.ok("POST", endpoint, input).(map[string]any)
			sid := int64(source["sourceId"].(float64))
			var mount int64
			if e := f.a.Pool.QueryRow(f.ctx, `SELECT ns_id FROM namespace WHERE source_id=$1 AND is_mount`, sid).Scan(&mount); e != nil {
				t.Fatal(e)
			}
			f.upload(mount, "keep.png")
			input["enabled"] = false
			f.ok("PATCH", fmt.Sprintf("%s/%d", endpoint, sid), input)
			// An account teardown must not even contact an offline/disabled external root.
			f.ok("GET", "/user/deleteUser", nil)
			if _, e := os.Stat(filepath.Join(dir, "keep.png")); e != nil {
				t.Fatalf("account removal deleted external content: %v", e)
			}
		})
	}
}
func TestLocalBindingBoundaries(t *testing.T) {
	f := newLibraryTest(t)
	dir, endpoint, input := externalBinding(t, f, "local")
	if e := os.Symlink(f.dir, filepath.Join(dir, "escape")); e != nil {
		t.Fatal(e)
	}
	for _, relative := range []string{"../", "/etc", "escape", ".photoly-system"} {
		input["relativePath"] = relative
		if out := f.json("POST", endpoint, input); out["msgCode"] == float64(200) {
			t.Fatalf("accepted %q", relative)
		}
	}
	input["relativePath"] = ""
	if _, e := f.a.Pool.Exec(f.ctx, `UPDATE "user" SET role='user' WHERE user_id=$1`, f.uid); e != nil {
		t.Fatal(e)
	}
	for _, url := range []string{"/admin/storage/local-roots", "/admin/storage/local-directories?rootId=test", endpoint} {
		if out := f.json("GET", url, nil); out["msgCode"] != float64(403) {
			t.Fatalf("non-admin access: %v", out)
		}
	}
	if out := f.json("POST", endpoint, input); out["msgCode"] != float64(403) {
		t.Fatal(out)
	}
}
func TestDAVConflictAndScanCoalescing(t *testing.T) {
	f := newLibraryTest(t)
	dir, endpoint, input := externalBinding(t, f, "webdav")
	created := f.ok("POST", endpoint, input).(map[string]any)
	sid := int64(created["sourceId"].(float64))
	var mount int64
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT ns_id FROM namespace WHERE source_id=$1 AND is_mount`, sid).Scan(&mount); e != nil {
		t.Fatal(e)
	}
	pid := f.upload(mount, "versioned.png")
	// A remote writer changes the object after indexing; an app delete cannot erase it.
	if e := os.WriteFile(filepath.Join(dir, "versioned.png"), append(pngData(), []byte("changed")...), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := photo.For(f.a).Delete(f.ctx, f.uid, pid); e == nil {
		t.Fatal("deleted stale version")
	}
	if _, e := os.Stat(filepath.Join(dir, "versioned.png")); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 3; i++ {
		f.ok("POST", fmt.Sprintf("/user/storage/sources/%d/scan", sid), map[string]any{"force": true})
	}
	var count int
	var force bool
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT count(*),bool_and(force) FROM storage_scan_job WHERE source_id=$1 AND state='queued'`, sid).Scan(&count, &force); e != nil || count != 1 || !force {
		t.Fatal("manual scan not queued")
	}
	// Neither encrypted credential field is reflected by the settings API.
	settings := f.request("GET", endpoint, "", nil).Body.String()
	if strings.Contains(settings, "password") || strings.Contains(settings, "username") {
		t.Fatal("credential leaked")
	}
}

func TestExternalSameFilesystemWorkspace(t *testing.T) {
	if _, e := os.Stat("/dev/shm"); e != nil {
		t.Skip("a second filesystem is required")
	}
	f := newLibraryTest(t)
	outside, e := os.MkdirTemp("/dev/shm", "photoly-mount-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(outside)
	roots, _ := json.Marshal([]map[string]string{{"id": "ram", "name": "Other filesystem", "path": outside}})
	t.Setenv("STORAGE_LOCAL_ROOTS", string(roots))
	if _, e = f.a.Pool.Exec(f.ctx, `UPDATE "user" SET role='admin' WHERE user_id=$1`, f.uid); e != nil {
		t.Fatal(e)
	}
	input := map[string]any{"name": "Second disk", "rootId": "ram", "enabled": true}
	result := f.ok("POST", fmt.Sprintf("/admin/users/%d/storage/local", f.uid), input).(map[string]any)
	sid := int64(result["sourceId"].(float64))
	var ns int64
	if e = f.a.Pool.QueryRow(f.ctx, `SELECT ns_id FROM namespace WHERE source_id=$1`, sid).Scan(&ns); e != nil {
		t.Fatal(e)
	}
	pid := f.upload(ns, "across-filesystems.png")
	if _, e = photo.For(f.a).Delete(f.ctx, f.uid, pid); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(outside, "across-filesystems.png")); !os.IsNotExist(e) {
		t.Fatal("delete left original", e)
	}
}

// liveJPEG embeds the Apple MakerNote identifier used by real Live Photos.
func liveJPEG(t *testing.T, identifier string) []byte {
	t.Helper()
	var jpegBytes bytes.Buffer
	if e := jpeg.Encode(&jpegBytes, image.NewRGBA(image.Rect(0, 0, 16, 16)), nil); e != nil {
		t.Fatal(e)
	}
	mn := make([]byte, 32+len(identifier)+1)
	copy(mn, []byte("Apple iOS\x00"))
	copy(mn[12:], "II")
	binary.LittleEndian.PutUint16(mn[14:], 1)
	binary.LittleEndian.PutUint16(mn[16:], 0x11)
	binary.LittleEndian.PutUint16(mn[18:], 2)
	binary.LittleEndian.PutUint32(mn[20:], uint32(len(identifier)+1))
	binary.LittleEndian.PutUint32(mn[24:], 32)
	copy(mn[32:], identifier)
	exif := make([]byte, 44)
	copy(exif, "II")
	binary.LittleEndian.PutUint16(exif[2:], 42)
	binary.LittleEndian.PutUint32(exif[4:], 8)
	binary.LittleEndian.PutUint16(exif[8:], 1)
	binary.LittleEndian.PutUint16(exif[10:], 0x8769)
	binary.LittleEndian.PutUint16(exif[12:], 4)
	binary.LittleEndian.PutUint32(exif[14:], 1)
	binary.LittleEndian.PutUint32(exif[18:], 26)
	binary.LittleEndian.PutUint16(exif[26:], 1)
	binary.LittleEndian.PutUint16(exif[28:], 0x927c)
	binary.LittleEndian.PutUint16(exif[30:], 7)
	binary.LittleEndian.PutUint32(exif[32:], uint32(len(mn)))
	binary.LittleEndian.PutUint32(exif[36:], 44)
	exif = append(exif, mn...)
	payload := append([]byte("Exif\x00\x00"), exif...)
	out := []byte{0xff, 0xd8, 0xff, 0xe1, byte((len(payload) + 2) >> 8), byte(len(payload) + 2)}
	out = append(out, payload...)
	return append(out, jpegBytes.Bytes()[2:]...)
}
func TestExternalLivePhotoScan(t *testing.T) {
	if _, e := exec.LookPath("ffmpeg"); e != nil {
		t.Skip("ffmpeg required")
	}
	for _, kind := range []string{"local", "webdav", "smb"} {
		t.Run(kind, func(t *testing.T) {
			f := newLibraryTest(t)
			dir, endpoint, input := externalBinding(t, f, kind)
			created := f.ok("POST", endpoint, input).(map[string]any)
			sid := int64(created["sourceId"].(float64))
			if e := os.WriteFile(filepath.Join(dir, "cover.jpg"), liveJPEG(t, "pair-identifier"), 0600); e != nil {
				t.Fatal(e)
			}
			if e := os.Mkdir(filepath.Join(dir, "movies"), 0700); e != nil {
				t.Fatal(e)
			}
			movie := filepath.Join(dir, "movies", "different-name.mov")
			command := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c=red:s=16x16:d=0.2", "-c:v", "libx264", "-threads", "1", "-metadata", "com.apple.quicktime.content.identifier=pair-identifier", "-movflags", "use_metadata_tags", movie)
			if out, e := command.CombinedOutput(); e != nil {
				t.Fatalf("fixture: %s %v", out, e)
			}
			scanner := photo.For(f.a)
			for i := 0; i < 2; i++ {
				if e := scanner.ScanSource(f.ctx, sid); e != nil {
					t.Fatal(e)
				}
			}
			var pid int64
			var live bool
			if e := f.a.Pool.QueryRow(f.ctx, `SELECT p.photo_id,m.live FROM photo p JOIN photo_media m USING(photo_id) WHERE p.source_id=$1 AND p.file_name='cover.jpg'`, sid).Scan(&pid, &live); e != nil || !live {
				t.Fatalf("not paired: %v %v", live, e)
			}
			var count int
			if e := f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM photo WHERE source_id=$1`, sid).Scan(&count); e != nil || count != 1 {
				t.Fatalf("attachment indexed separately: %d %v", count, e)
			}
			if e := os.Rename(movie, filepath.Join(dir, "moved-live.mov")); e != nil {
				t.Fatal(e)
			}
			for i := 0; i < 2; i++ {
				if e := scanner.ScanSource(f.ctx, sid); e != nil {
					t.Fatal(e)
				}
			}
			if e := f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM photo WHERE source_id=$1`, sid).Scan(&count); e != nil || count != 1 {
				t.Fatalf("moved attachment indexed separately: %d %v", count, e)
			}
			var filename string
			if e := f.a.Pool.QueryRow(f.ctx, `SELECT filename FROM media_asset WHERE photo_id=$1 AND role='live_original'`, pid).Scan(&filename); e != nil || filename != "moved-live.mov" {
				t.Fatalf("attachment move lost: %s %v", filename, e)
			}
			if _, e := scanner.Delete(f.ctx, f.uid, pid); e != nil {
				t.Fatal(e)
			}
			if _, e := os.Stat(filepath.Join(dir, "moved-live.mov")); !os.IsNotExist(e) {
				t.Fatalf("attachment not deleted: %v", e)
			}
		})
	}
}

func TestSlowDAVScanDoesNotLockLocalLibrary(t *testing.T) {
	f := newLibraryTest(t)
	dir := t.TempDir()
	h := davserver.Handler(dir)
	var block atomic.Bool
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if block.Load() && r.Method == "PROPFIND" && r.Header.Get("Depth") == "1" {
			select {
			case started <- struct{}{}:
			default:
			}
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		h.ServeHTTP(w, r)
	}))
	defer srv.Close()
	defer close(release)
	t.Setenv("STORAGE_WEBDAV_ALLOWED_HOSTS", srv.URL)
	t.Setenv("STORAGE_SECRET_KEY", "slow-scan-test")
	source := f.ok("POST", "/user/storage/webdav", map[string]any{"name": "Slow DAV", "url": srv.URL + "/", "enabled": true}).(map[string]any)
	sid := int64(source["sourceId"].(float64))
	block.Store(true)
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- photo.For(f.a).ScanSource(ctx, sid) }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("scan did not enumerate")
	}
	localCtx, localCancel := context.WithTimeout(f.ctx, time.Second)
	defer localCancel()
	e := photo.For(f.a).Atomic(localCtx, f.uid, func(tx pgx.Tx, u photo.UserRow, files *photo.Files) error {
		_, _, e := storage.CreateFolder(localCtx, tx, files, f.a.Cfg.UploadFolder, f.uid, f.root, "local-remains-available")
		return e
	})
	if e != nil {
		t.Fatalf("slow remote scan blocked local write: %v", e)
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled scan did not finish")
	}
}

func TestSharedLocalMountUsers(t *testing.T) {
	f := newLibraryTest(t)
	dir, endpoint, input := externalBinding(t, f, "local")
	first := f.ok("POST", endpoint, input).(map[string]any)
	sid1 := int64(first["sourceId"].(float64))
	uuid := photo.NewUUID()
	var uid, root int64
	if e := f.a.Pool.QueryRow(f.ctx, `INSERT INTO "user"(uuid,email,user_name,role,group_id,used_bytes) VALUES($1,$2,'shared','user',(SELECT id FROM user_group WHERE is_default),0) RETURNING user_id`, uuid, uuid+"@example.test").Scan(&uid); e != nil {
		t.Fatal(e)
	}
	defer f.a.Pool.Exec(f.ctx, `DELETE FROM "user" WHERE user_id=$1`, uid)
	if e := f.a.Pool.QueryRow(f.ctx, `INSERT INTO namespace(ns_name,ns_parent_id,user_id) VALUES('/',-1,$1) RETURNING ns_id`, uid).Scan(&root); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(filepath.Join(f.a.Cfg.UploadFolder, uuid), 0700); e != nil {
		t.Fatal(e)
	}
	if e := storage.RecordRootIdentity(f.ctx, f.a.Pool, uid, filepath.Join(f.a.Cfg.UploadFolder, uuid)); e != nil {
		t.Fatal(e)
	}
	second := f.ok("POST", fmt.Sprintf("/admin/users/%d/storage/local", uid), input).(map[string]any)
	sid2 := int64(second["sourceId"].(float64))
	var mount int64
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT ns_id FROM namespace WHERE source_id=$1`, sid1).Scan(&mount); e != nil {
		t.Fatal(e)
	}
	original := f.upload(mount, "shared.png")
	scanner := photo.For(f.a)
	for i := 0; i < 2; i++ {
		if e := scanner.ScanSource(f.ctx, sid2); e != nil {
			t.Fatal(e)
		}
	}
	var independent int64
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT photo_id FROM photo WHERE source_id=$1`, sid2).Scan(&independent); e != nil || independent == original {
		t.Fatal("shared indexes merged", e)
	}
	f.ok("POST", "/photo/update", map[string]any{"photoId": original, "photoName": "renamed"})
	if e := scanner.ScanSource(f.ctx, sid2); e != nil {
		t.Fatal(e)
	}
	var name string
	if e := f.a.Pool.QueryRow(f.ctx, `SELECT file_name FROM photo WHERE photo_id=$1`, independent).Scan(&name); e != nil || name != "renamed.png" {
		t.Fatalf("shared mutation did not propagate: %s %v", name, e)
	}
	f.ok("GET", "/user/deleteUser", nil)
	if _, e := os.Stat(filepath.Join(dir, "renamed.png")); e != nil {
		t.Fatal("removing one account deleted shared file", e)
	}
	if e := scanner.ScanSource(f.ctx, sid2); e != nil {
		t.Fatal(e)
	}
}

func TestScopedScanAndQueue(t *testing.T) {
	for _, kind := range []string{"local", "webdav", "smb"} {
		t.Run(kind, func(t *testing.T) {
			f := newLibraryTest(t)
			dir, endpoint, input := externalBinding(t, f, kind)
			created := f.ok("POST", endpoint, input).(map[string]any)
			sid := int64(created["sourceId"].(float64))
			var root int64
			if e := f.a.Pool.QueryRow(f.ctx, `SELECT ns_id FROM namespace WHERE source_id=$1 AND is_mount`, sid).Scan(&root); e != nil {
				t.Fatal(e)
			}
			a := f.folder(root, "A")
			b := f.folder(root, "B")
			outside := f.upload(b, "outside.png")
			gone := f.upload(a, "gone.png")
			scan := photo.For(f.a)
			if e := scan.ScanSource(f.ctx, sid); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(dir, "A", "new.png"), append(pngData(), []byte("new-content")...), 0600); e != nil {
				t.Fatal(e)
			}
			if e := os.Remove(filepath.Join(dir, "A", "gone.png")); e != nil {
				t.Fatal(e)
			}
			if e := os.Remove(filepath.Join(dir, "B", "outside.png")); e != nil {
				t.Fatal(e)
			}
			for i := 0; i < 2; i++ {
				first := f.ok("POST", fmt.Sprintf("/user/storage/sources/%d/scan", sid), map[string]any{"nsId": a}).(map[string]any)
				second := f.ok("POST", fmt.Sprintf("/user/storage/sources/%d/scan", sid), map[string]any{"nsId": a}).(map[string]any)
				if first["jobId"] != second["jobId"] {
					t.Fatal("duplicate not coalesced")
				}
				if e := scan.ScanSource(f.ctx, sid); e != nil {
					t.Fatal(e)
				}
			}
			f.ok("GET", "/user/storage/sources", nil)
			var n, missing int
			if e := f.a.Pool.QueryRow(f.ctx, `SELECT missing_scans FROM photo WHERE photo_id=$1`, outside).Scan(&missing); e != nil || missing != 0 {
				t.Fatal("scoped scan touched outside photo", e, missing)
			}
			if e := f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM photo WHERE photo_id=$1`, gone).Scan(&n); e != nil || n != 0 {
				t.Fatal("scoped deletion not confirmed", e, n)
			}
			if e := f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM photo WHERE ns_id=$1 AND file_name='new.png'`, a).Scan(&n); e != nil || n != 1 {
				t.Fatal("new file not indexed", e, n)
			}
			// Foreign/default source namespace cannot be supplied to this source.
			resp := f.request("POST", fmt.Sprintf("/user/storage/sources/%d/scan", sid), "application/json", strings.NewReader(fmt.Sprintf(`{"nsId":%d}`, f.root)))
			if strings.Contains(resp.Body.String(), `"jobId"`) {
				t.Fatal("accepted foreign namespace")
			}
		})
	}
}

func TestNASMediaIndex(t *testing.T) {
	credentialFile := os.Getenv("SMB_TEST_ENV_FILE")
	if credentialFile == "" {
		t.Skip("SMB_TEST_ENV_FILE required")
	}
	credentials, e := os.ReadFile(credentialFile)
	if e != nil {
		t.Fatal(e)
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(credentials), "\n") {
		p := strings.SplitN(line, "=", 2)
		if len(p) == 2 {
			values[strings.TrimSpace(p[0])] = strings.Trim(strings.TrimSpace(p[1]), "\"'")
		}
	}
	f := newLibraryTest(t)
	t.Setenv("STORAGE_SECRET_KEY", strings.Repeat("a", 64))
	host := os.Getenv("SMB_TEST_HOST")
	if host == "" {
		host = "10.0.0.249"
	}
	share := os.Getenv("SMB_TEST_SHARE")
	if share == "" {
		share = "TestFolder"
	}
	t.Setenv("STORAGE_SMB_ALLOWED_HOSTS", host)
	cfg := map[string]string{"server": host, "share": share, "username": values["user"], "password": values["password"]}
	base, e := storage.NewSMB(cfg)
	if e != nil {
		t.Fatal(e)
	}
	directory := "photoly-media-test-" + photo.NewUUID()
	if e = base.Mkdir(f.ctx, directory); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if e := base.Delete(context.Background(), directory); e != nil {
			t.Error("NAS fixture cleanup", e)
		}
	}()
	cfg["subdirectory"] = directory
	remote, e := storage.NewSMB(cfg)
	if e != nil {
		t.Fatal(e)
	}
	input := map[string]any{"name": "NAS fixture", "server": host, "share": share, "subdirectory": directory, "username": values["user"], "password": values["password"]}
	created := f.ok("POST", "/user/storage/smb", input).(map[string]any)
	sid := int64(created["sourceId"].(float64))
	var mount int64
	if e = f.a.Pool.QueryRow(f.ctx, `SELECT ns_id FROM namespace WHERE source_id=$1 AND is_mount`, sid).Scan(&mount); e != nil {
		t.Fatal(e)
	}
	f.upload(mount, "app upload.png")
	if e = remote.Put(f.ctx, "external.png", bytes.NewReader(pngData()), int64(len(pngData()))); e != nil {
		t.Fatal(e)
	}
	scan := photo.For(f.a)
	for i := 0; i < 2; i++ {
		if e = scan.ScanSource(f.ctx, sid); e != nil {
			t.Fatal(e)
		}
	}
	var pid int64
	if e = f.a.Pool.QueryRow(f.ctx, `SELECT photo_id FROM photo WHERE source_id=$1 AND file_name='external.png'`, sid).Scan(&pid); e != nil {
		t.Fatal(e)
	}
	if e = remote.Move(f.ctx, "external.png", "renamed.png", ""); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		if e = scan.ScanSource(f.ctx, sid); e != nil {
			t.Fatal(e)
		}
	}
	var retained int64
	if e = f.a.Pool.QueryRow(f.ctx, `SELECT photo_id FROM photo WHERE source_id=$1 AND file_name='renamed.png'`, sid).Scan(&retained); e != nil || retained != pid {
		t.Fatal("NAS rename lost ID", e)
	}
	f.ok("DELETE", fmt.Sprintf("/user/storage/smb/%d", sid), nil)
	if _, e = remote.Property(f.ctx, "renamed.png"); e != nil {
		t.Fatal("detach removed NAS original", e)
	}
}
