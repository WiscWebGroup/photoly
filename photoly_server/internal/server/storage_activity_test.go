package server

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/photo"
)

func TestFolderDirectCountAndDescendantCovers(t *testing.T) {
	f := newLibraryTest(t)
	parent := f.folder(f.root, "parent")
	child := f.folder(parent, "child")
	deep := f.folder(child, "deep")
	deepPhoto := f.upload(deep, "deep.png")
	get := func() map[string]any {
		return f.ok("GET", fmt.Sprintf("/photo/getCovers?source=folder&ids=%d", parent), nil).([]any)[0].(map[string]any)
	}
	cover := get()
	if cover["count"] != float64(1) || len(cover["coverIds"].([]any)) != 1 || cover["coverIds"].([]any)[0] != float64(deepPhoto) {
		t.Fatalf("descendant cover: %#v", cover)
	}
	direct := f.upload(parent, "direct.png")
	cover = get()
	if cover["count"] != float64(2) || len(cover["coverIds"].([]any)) != 1 || cover["coverIds"].([]any)[0] != float64(direct) {
		t.Fatalf("direct cover: %#v", cover)
	}
}

func TestScanFailuresAndRecycleRetention(t *testing.T) {
	for _, kind := range []string{"local", "webdav"} {
		t.Run(kind, func(t *testing.T) {
			f := newLibraryTest(t)
			dir, endpoint, input := externalBinding(t, f, kind)
			created := f.ok("POST", endpoint, input).(map[string]any)
			sid := int64(created["sourceId"].(float64))
			var mount int64
			if err := f.a.Pool.QueryRow(f.ctx, `SELECT ns_id FROM namespace WHERE source_id=$1 AND is_mount`, sid).Scan(&mount); err != nil {
				t.Fatal(err)
			}
			recycle := f.folder(mount, "@Recycle")
			recycled := f.upload(recycle, "kept.png")
			if err := os.WriteFile(filepath.Join(dir, "broken.jpg"), []byte("not an image"), 0600); err != nil {
				t.Fatal(err)
			}
			scanner := photo.For(f.a)
			scan := func() {
				t.Helper()
				if err := scanner.ScanSource(f.ctx, sid); err != nil {
					t.Fatal(err)
				}
			}
			scan()
			scan()
			var n int
			if err := f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM photo WHERE photo_id=$1`, recycled).Scan(&n); err != nil || n != 0 {
				t.Fatalf("recycle index retained: %d %v", n, err)
			}
			if _, err := os.Stat(filepath.Join(dir, "@Recycle", "kept.png")); err != nil {
				t.Fatal("recycle original deleted", err)
			}
			var before, after time.Time
			var retry bool
			if err := f.a.Pool.QueryRow(f.ctx, `SELECT attempted_at,retryable FROM storage_scan_failure WHERE source_id=$1 AND path='broken.jpg'`, sid).Scan(&before, &retry); err != nil || retry {
				t.Fatalf("classification: %v %v", retry, err)
			}
			scan()
			if err := f.a.Pool.QueryRow(f.ctx, `SELECT attempted_at FROM storage_scan_failure WHERE source_id=$1 AND path='broken.jpg'`, sid).Scan(&after); err != nil || !after.Equal(before) {
				t.Fatalf("unchanged failure retried: %v %v", before, after)
			}
			response := f.ok("GET", fmt.Sprintf("/user/storage/sources/%d/failures?limit=1", sid), nil).(map[string]any)
			if response["total"] != float64(1) || response["items"].([]any)[0].(map[string]any)["code"] != "invalid_image" {
				t.Fatalf("failure response: %#v", response)
			}
			f.ok("POST", fmt.Sprintf("/user/storage/sources/%d/scan", sid), map[string]any{"force": true})
			scan()
			if err := f.a.Pool.QueryRow(f.ctx, `SELECT attempted_at FROM storage_scan_failure WHERE source_id=$1 AND path='broken.jpg'`, sid).Scan(&after); err != nil || !after.After(before) {
				t.Fatal("force did not retry", err)
			}
			if err := os.WriteFile(filepath.Join(dir, "broken.jpg"), pngData(), 0600); err != nil {
				t.Fatal(err)
			}
			scan()
			scan()
			if err := f.a.Pool.QueryRow(f.ctx, `SELECT count(*) FROM storage_scan_failure WHERE source_id=$1`, sid).Scan(&n); err != nil || n != 0 {
				t.Fatalf("repaired file still failing: %d %v", n, err)
			}
		})
	}
}

func TestDisableS3DoesNotRewriteCredentials(t *testing.T) {
	f := newLibraryTest(t)
	var sid int64
	if err := f.a.Pool.QueryRow(f.ctx, `INSERT INTO storage_source(user_id,kind,enabled,config,secret) VALUES($1,'s3',true,'{"bucket":"kept"}',decode('aabb','hex')) RETURNING id`, f.uid).Scan(&sid); err != nil {
		t.Fatal(err)
	}
	f.ok("PUT", "/user/storage/s3", map[string]any{"enabled": false})
	var enabled bool
	var bucket, secret string
	if err := f.a.Pool.QueryRow(f.ctx, `SELECT enabled,config->>'bucket',encode(secret,'hex') FROM storage_source WHERE id=$1`, sid).Scan(&enabled, &bucket, &secret); err != nil {
		t.Fatal(err)
	}
	if enabled || bucket != "kept" || secret != "aabb" {
		t.Fatal("disable changed saved configuration")
	}
	other := newLibraryTest(t)
	result := other.json("GET", fmt.Sprintf("/user/storage/sources/%d/failures", sid), nil)
	if result["msgCode"] != float64(404) {
		t.Fatalf("cross-user failures exposed: %#v", result)
	}
}
