package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirectoryRollbackAndCollision(t *testing.T) {
	root := t.TempDir()
	f, e := NewFiles(root)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	if e = f.Mkdir(a); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(a, "notes.txt"), []byte("keep"), 0644); e != nil {
		t.Fatal(e)
	}
	if e = f.Move(a, b); e != nil {
		t.Fatal(e)
	}
	os.Mkdir(a, 0755)
	if e = f.Move(b, a); e == nil {
		t.Fatal("overwrote destination")
	}
	os.Remove(a)
	// The initial directory creation can only be undone once user files are removed;
	// inverse moves must still put the original tree back.
	f.Rollback()
	if data, e := os.ReadFile(filepath.Join(a, "notes.txt")); e != nil || string(data) != "keep" {
		t.Fatalf("rollback: %q %v", data, e)
	}
}

type failingStore struct {
	*mem
	fail bool
}

func (s *failingStore) Put(ctx context.Context, key string, r io.Reader, size int64) error {
	if s.fail {
		s.fail = false
		return errors.New("injected remote failure")
	}
	return s.mem.Put(ctx, key, r, size)
}
func TestSourceBoundMutation(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "a.png"), filepath.Join(root, "b.png")
	first, second := newMem(), newMem()
	Bind(a, first, "user-a/source/a")
	Bind(b, second, "user-b/source/a")
	f, _ := NewFiles(root)
	defer f.Close()
	f.InstallNew(strings.NewReader("first"), a)
	f.InstallNew(strings.NewReader("second"), b)
	if e := f.Flush(context.Background()); e != nil {
		t.Fatal(e)
	}
	f.Commit()
	os.Remove(a)
	os.Remove(b)
	if !Fetch(a) || !Fetch(b) {
		t.Fatal("source-specific fetch failed")
	}
	if data, _ := os.ReadFile(a); string(data) != "first" {
		t.Fatal("cross-user read")
	}
	if data, _ := os.ReadFile(b); string(data) != "second" {
		t.Fatal("cross-user read")
	}
	fail := &failingStore{mem: first, fail: true}
	Bind(a, fail, "user-a/source/a")
	g, _ := NewFiles(root)
	defer g.Close()
	g.Install(strings.NewReader("replacement"), a)
	if e := g.Flush(context.Background()); e == nil {
		t.Fatal("remote failure accepted")
	}
	g.Rollback()
	if data, _ := os.ReadFile(a); string(data) != "first" {
		t.Fatal("local rollback failed")
	}
	if data := first.m["user-a/source/a"]; string(data) != "first" {
		t.Fatal("remote rollback failed")
	}
}
func TestSourcePathRejectsSymlinksAndTraversal(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	os.Symlink(outside, filepath.Join(root, "user"))
	if _, e := SafePath(root, "user", "photo.png"); e == nil {
		t.Fatal("accepted symlink")
	}
	if _, e := SafePath(root, "..", "photo.png"); e == nil {
		t.Fatal("accepted traversal")
	}
}

func TestDirectoryReplacedByLinkBeforeIO(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if e := os.Mkdir(filepath.Join(root, "user"), 0755); e != nil {
		t.Fatal(e)
	}
	p, e := SafePath(root, "user", "original.png")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(outside, "original.png"), []byte("private"), 0644); e != nil {
		t.Fatal(e)
	}
	if e = os.Rename(filepath.Join(root, "user"), filepath.Join(root, "old-user")); e != nil {
		t.Fatal(e)
	}
	if e = os.Symlink(outside, filepath.Join(root, "user")); e != nil {
		t.Fatal(e)
	}
	if f, e := OpenFile(p); e == nil {
		f.Close()
		t.Fatal("read followed a replaced directory")
	}
	if e = RenameNoReplace(p, filepath.Join(root, "stolen.png")); e == nil {
		t.Fatal("move followed a replaced directory")
	}
	f, e := NewFiles(root)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if _, e = f.InstallNew(strings.NewReader("new"), filepath.Join(root, "user", "new.png")); e == nil {
		t.Fatal("write followed a replaced directory")
	}
}

func TestInstallRetriesFilesystemCollision(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "photo.png"), filepath.Join(root, "photo (1).png")
	if e := os.WriteFile(first, []byte("external"), 0644); e != nil {
		t.Fatal(e)
	}
	f, e := NewFiles(root)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	_, actual, e := f.InstallUnique(strings.NewReader("uploaded"), first, func() (string, error) { return second, nil })
	if e != nil || actual != second {
		t.Fatalf("retry: %s %v", actual, e)
	}
	if b, e := os.ReadFile(first); e != nil || string(b) != "external" {
		t.Fatal("collision overwrote existing file")
	}
	if b, e := os.ReadFile(second); e != nil || string(b) != "uploaded" {
		t.Fatal("retry lost consumed reader")
	}
}
