//go:build !linux && !darwin

package storage

import "errors"

func RenameNoReplace(src, dst string) error {
	return errors.New("atomic no-replace rename requires Linux or macOS")
}
