//go:build !linux && !darwin

package storage

import (
	"errors"
	"os"
)

func OpenFile(string) (*os.File, error) { return nil, errors.New("unsupported filesystem platform") }
func mkdirAll(string) error             { return errors.New("unsupported filesystem platform") }
func mkdir(string) error                { return errors.New("unsupported filesystem platform") }
func removeEntry(string) error          { return errors.New("unsupported filesystem platform") }
