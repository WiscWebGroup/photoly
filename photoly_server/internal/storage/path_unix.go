//go:build linux || darwin

package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// openDirectory anchors each component with a directory descriptor. A directory
// replaced by a symlink between validation and I/O cannot escape the library.
func openDirectory(path string, create bool) (int, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return -1, err
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, err
	}
	for _, part := range strings.Split(strings.TrimPrefix(abs, "/"), "/") {
		if part == "" {
			continue
		}
		next, e := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if e == unix.ENOENT && create {
			if e = unix.Mkdirat(fd, part, 0755); e == nil || e == unix.EEXIST {
				next, e = unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
			}
		}
		_ = unix.Close(fd)
		if e != nil {
			return -1, &os.PathError{Op: "open directory", Path: path, Err: e}
		}
		fd = next
	}
	return fd, nil
}

// OpenFile reads a regular file without following any user-directory symlinks.
// Callers pass canonical paths returned by the source resolver.
func OpenFile(path string) (*os.File, error) {
	dir, err := openDirectory(filepath.Dir(path), false)
	if err != nil {
		return nil, err
	}
	defer unix.Close(dir)
	fd, err := unix.Openat(dir, filepath.Base(path), unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(fd), path)
	st, err := f.Stat()
	if err == nil && !st.Mode().IsRegular() {
		err = errors.New("not_a_regular_file")
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

func mkdirAll(path string) error {
	fd, err := openDirectory(path, true)
	if err == nil {
		_ = unix.Close(fd)
	}
	return err
}
func mkdir(path string) error {
	dir, err := openDirectory(filepath.Dir(path), false)
	if err != nil {
		return err
	}
	defer unix.Close(dir)
	return unix.Mkdirat(dir, filepath.Base(path), 0755)
}
func removeEntry(path string) error {
	dir, err := openDirectory(filepath.Dir(path), false)
	if err != nil {
		return err
	}
	defer unix.Close(dir)
	err = unix.Unlinkat(dir, filepath.Base(path), 0)
	if err == unix.EISDIR || err == unix.EPERM {
		err = unix.Unlinkat(dir, filepath.Base(path), unix.AT_REMOVEDIR)
	}
	return err
}

func createFile(path string) (*os.File, error) {
	dir, e := openDirectory(filepath.Dir(path), false)
	if e != nil {
		return nil, e
	}
	defer unix.Close(dir)
	fd, e := unix.Openat(dir, filepath.Base(path), unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if e != nil {
		return nil, e
	}
	return os.NewFile(uintptr(fd), path), nil
}

// removeTree never follows any intermediate or leaf symlink during cleanup.
func removeTree(path string) error {
	parent, e := openDirectory(filepath.Dir(path), false)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	defer unix.Close(parent)
	var remove func(int, string) error
	remove = func(parent int, name string) error {
		e := unix.Unlinkat(parent, name, 0)
		if e == nil || e == unix.ENOENT {
			return nil
		}
		if e != unix.EISDIR && e != unix.EPERM {
			return e
		}
		fd, e := unix.Openat(parent, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if e != nil {
			return e
		}
		dir := os.NewFile(uintptr(fd), name)
		defer dir.Close()
		entries, e := dir.ReadDir(-1)
		if e != nil {
			return e
		}
		for _, entry := range entries {
			if e = remove(fd, entry.Name()); e != nil {
				return e
			}
		}
		return unix.Unlinkat(parent, name, unix.AT_REMOVEDIR)
	}
	return remove(parent, filepath.Base(path))
}
