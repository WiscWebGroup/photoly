//go:build darwin

package storage

import (
	"golang.org/x/sys/unix"
	"path/filepath"
)

func RenameNoReplace(src, dst string) error {
	from, err := openDirectory(filepath.Dir(src), false)
	if err != nil {
		return err
	}
	defer unix.Close(from)
	to, err := openDirectory(filepath.Dir(dst), false)
	if err != nil {
		return err
	}
	defer unix.Close(to)
	return unix.RenameatxNp(from, filepath.Base(src), to, filepath.Base(dst), unix.RENAME_EXCL)
}
