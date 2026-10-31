package photo

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const scanProcessorVersion = "large-preview-v1"

type savedScanFailure struct {
	Code, Version, Processor string
	Retryable                bool
	Attempted                time.Time
}

func (e diskEntry) fingerprint() string {
	return fmt.Sprintf("%s:%d:%d:%s", e.identity, e.size, e.mtime, e.version)
}
func retryScanFailure(code string) bool {
	switch code {
	case "invalid_image", "unsupported_image", "image_too_many_pixels", "animation_too_large", "large_image_unsupported":
		return false
	}
	return true
}
func recycleComponent(name string) bool {
	switch strings.ToLower(name) {
	case "@recycle", "#recycle", "$recycle.bin":
		return true
	}
	return false
}
func (snap *sourceSnapshot) excluded(path string) bool {
	if snap.source.Default {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if recycleComponent(part) {
			return true
		}
	}
	return false
}
func (s *Service) previousScanFailures(ctx context.Context, id int64) (map[string]savedScanFailure, error) {
	rows, err := s.Pool.Query(ctx, `SELECT path,error,file_version,retryable,processor_version,attempted_at FROM storage_scan_failure WHERE source_id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]savedScanFailure{}
	for rows.Next() {
		var path string
		var v savedScanFailure
		if err = rows.Scan(&path, &v.Code, &v.Version, &v.Retryable, &v.Processor, &v.Attempted); err != nil {
			return nil, err
		}
		out[path] = v
	}
	return out, rows.Err()
}
