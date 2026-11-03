package backup

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
)

func TestBackupIDsRejectPathsBeforeFilesystemAccess(t *testing.T) {
	valid := "01234567-89ab-cdef-0123-456789abcdef"
	got, err := validatedID(valid)
	if err != nil || got != valid {
		t.Fatalf("canonical UUID rejected: %q, %v", got, err)
	}
	root := t.TempDir()
	e := &Engine{Cfg: app.Config{BackupDir: root, UploadFolder: t.TempDir()}}
	for _, id := range []string{
		"", ".", "..", "../" + valid, "/" + valid, valid + "/../outside",
		`..\` + valid, "%2e%2e%2f" + valid, valid + "\x00",
		strings.Repeat("g", 36), strings.Replace(valid, "-", "_", 1),
		strings.ToUpper(valid), "{" + valid + "}", strings.ReplaceAll(valid, "-", ""),
		".partial-" + valid, valid + "\n", strings.Repeat("a", 4096),
	} {
		t.Run(id, func(t *testing.T) {
			if got, err := validatedID(id); err == nil || got != "" {
				t.Fatal("invalid ID returned a usable value")
			}
			if _, err := e.Backup(context.Background(), id, "a valid backup password"); err == nil || err.Error() != "invalid backup ID" {
				t.Fatal("backup did not reject ID first", err)
			}
			if err := e.Restore(context.Background(), id, "a valid backup password"); err == nil || err.Error() != "invalid backup ID" {
				t.Fatal("restore did not reject ID first", err)
			}
			if err := e.rollback(context.Background(), &Journal{ID: id}); err == nil || err.Error() != "invalid backup ID" {
				t.Fatal("rollback did not reject ID first", err)
			}
			controller := &Controller{}
			if _, err := controller.launch("backup", id, "a valid backup password"); err == nil || err.Error() != "invalid backup ID" {
				t.Fatal("task admitted an invalid ID", err)
			}
		})
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid IDs changed backup directory", entries, err)
	}
}
