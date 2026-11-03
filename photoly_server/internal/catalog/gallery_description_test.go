package catalog

import (
	"strings"
	"testing"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy/testutil"
)

func TestGalleryDescriptionValidation(t *testing.T) {
	for _, value := range []any{nil, 42, true, []any{}, map[string]any{}, strings.Repeat("文", 5001), "bad\x00text"} {
		if _, err := galleryDescription(map[string]any{"description": value}); err == nil {
			t.Fatalf("accepted invalid value %T", value)
		}
	}
	for _, value := range []string{"", "中文\n📷 <script>text</script>", strings.Repeat("📷", 5000)} {
		got, err := galleryDescription(map[string]any{"description": value})
		if err != nil || got == nil || *got != value {
			t.Fatalf("valid text did not round-trip: %v", err)
		}
	}
	if got, err := galleryDescription(map[string]any{}); got != nil || err != nil {
		t.Fatal("omission must preserve the existing value")
	}
}

func TestGalleryDescriptionFlow(t *testing.T) {
	owner, other, q, _ := setup(t)
	anon := &testutil.Client{T: t, Mux: owner.Mux}
	description := "旅行说明\n📷 <b>plain text</b>"
	owner.Want(owner.JSON("POST", "/gallery/insert", map[string]any{"gaName": "Trip", "description": description}), 200)
	gallery := owner.Want(owner.JSON("GET", "/gallery/getAll", nil), 200).([]any)[0].(map[string]any)
	gid := gallery["gaId"]
	read := func(path, want string) {
		t.Helper()
		rows := owner.Want(owner.JSON("GET", path, nil), 200).([]any)
		if got := rows[0].(map[string]any)["description"]; got != want {
			t.Fatalf("%s description = %v, want %q", path, got, want)
		}
	}
	read("/gallery/getAll", description)
	read("/gallery/getPage?page=1&rowsPerPage=10", description)
	// Older clients omit description while updating names/covers.
	owner.Want(owner.JSON("POST", "/gallery/update", map[string]any{"gaId": gid, "gaName": "Renamed"}), 200)
	read("/gallery/getAll", description)
	for _, client := range []*testutil.Client{other, anon} {
		want := 400.0
		if client == anon {
			want = 403
		}
		if got := client.JSON("POST", "/gallery/update", map[string]any{"gaId": gid, "gaName": "Hijack", "description": "no"})["msgCode"]; got != want {
			t.Fatalf("unauthorized update: %v", got)
		}
	}
	if rows := other.Want(other.JSON("GET", "/gallery/getAll", nil), 200).([]any); len(rows) != 0 {
		t.Fatal("foreign descriptions visible")
	}
	if got := anon.JSON("POST", "/gallery/insert", map[string]any{"gaName": "No", "description": description})["msgCode"]; got != 403.0 {
		t.Fatal(got)
	}
	if got := anon.JSON("GET", "/gallery/getAll", nil)["msgCode"]; got != 403.0 {
		t.Fatal(got)
	}
	for _, value := range []any{nil, 5, true, []any{}, map[string]any{}, strings.Repeat("文", 5001), "\x00"} {
		for _, path := range []string{"/gallery/insert", "/gallery/update"} {
			if got := owner.JSON("POST", path, map[string]any{"gaId": gid, "gaName": "Invalid", "description": value})["msgCode"]; got != 400.0 {
				t.Fatalf("%s accepted %T: %v", path, value, got)
			}
		}
	}
	read("/gallery/getAll", description)
	// Smart galleries use the same owner-editable field.
	q(`INSERT INTO smart_item(user_id,source,ga_id,name,basis) VALUES ($1,'trip',$2,'Trip','{}') RETURNING item_id`, q("uid"), int64(gid.(float64)))
	for _, value := range []string{strings.Repeat("📷", 5000), "", "New\n说明"} {
		owner.Want(owner.JSON("POST", "/gallery/update", map[string]any{"gaId": gid, "gaName": "Trip", "description": value}), 200)
		read("/gallery/getAll?includeSmart=true", value)
	}
	owner.Want(owner.JSON("POST", "/gallery/update", map[string]any{"gaId": gid, "gaName": "Smart renamed", "coverId": 3}), 200)
	read("/gallery/getAll?includeSmart=true", "New\n说明")
	owner.Want(owner.JSON("POST", "/gallery/insert", map[string]any{"gaName": "Legacy"}), 200)
	rows := owner.Want(owner.JSON("GET", "/gallery/getAll", nil), 200).([]any)
	if len(rows) != 2 || rows[1].(map[string]any)["description"] != "" {
		t.Fatalf("legacy default = %v", rows)
	}
}
