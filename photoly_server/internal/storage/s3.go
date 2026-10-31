package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func init() {
	Register(Driver{
		Type:  "s3",
		Label: "S3-compatible",
		Fields: []Field{
			{Name: "endpoint", Label: "Endpoint", Kind: "text", Required: true, Help: "Host[:port], or a full URL. e.g. s3.amazonaws.com, minio:9000"},
			{Name: "region", Label: "Region", Kind: "text", Help: "Optional for most non-AWS providers"},
			{Name: "bucket", Label: "Bucket", Kind: "text", Required: true},
			{Name: "prefix", Label: "Key prefix", Kind: "text", Help: "Optional folder inside the bucket"},
			{Name: "access_key", Label: "Access key", Kind: "text", Required: true},
			{Name: "secret_key", Label: "Secret key", Kind: "text", Required: true, Secret: true},
			{Name: "use_ssl", Label: "Use HTTPS", Kind: "bool", Default: "true"},
			{Name: "path_style", Label: "Path-style addressing", Kind: "bool", Help: "Needed by MinIO and some self-hosted providers"},
		},
		New: newS3,
		ID: func(cfg map[string]string) string {
			host := strings.TrimSpace(cfg["endpoint"])
			if u, err := url.Parse(host); err == nil && u.Host != "" && strings.Contains(host, "://") {
				host = u.Host
			}
			return strings.ToLower(strings.Trim(host, "/")) + "/" + strings.TrimSpace(cfg["bucket"]) + "/" + strings.Trim(strings.TrimSpace(cfg["prefix"]), "/")
		},
	})
}

type s3Store struct {
	c      *minio.Client
	bucket string
	prefix string
}

func newS3(cfg map[string]string) (Store, error) {
	endpoint, ssl := strings.TrimSpace(cfg["endpoint"]), cfg["use_ssl"] != "false"
	if strings.Contains(endpoint, "://") {
		u, err := url.Parse(endpoint)
		if err != nil || u.Host == "" {
			return nil, fmt.Errorf("invalid endpoint")
		}
		endpoint, ssl = u.Host, u.Scheme == "https"
	}
	lookup := minio.BucketLookupAuto
	if cfg["path_style"] == "true" {
		lookup = minio.BucketLookupPath
	}
	c, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg["access_key"], cfg["secret_key"], ""),
		Secure:       ssl,
		Region:       cfg["region"],
		BucketLookup: lookup,
	})
	if err != nil {
		return nil, err
	}
	p := strings.Trim(cfg["prefix"], "/")
	if p != "" {
		p += "/"
	}
	return &s3Store{c: c, bucket: cfg["bucket"], prefix: p}, nil
}

func (s *s3Store) key(k string) string { return s.prefix + k }

func notFound(err error) bool {
	r := minio.ToErrorResponse(err)
	return r.StatusCode == 404 || r.Code == "NoSuchKey"
}

func (s *s3Store) Put(ctx context.Context, key string, r io.Reader, size int64) error {
	_, err := s.c.PutObject(ctx, s.bucket, s.key(key), r, size, minio.PutObjectOptions{})
	return err
}

func (s *s3Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	o, err := s.c.GetObject(ctx, s.bucket, s.key(key), minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	if _, err := o.Stat(); err != nil { // GetObject is lazy; Stat surfaces 404 now
		o.Close()
		if notFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return o, nil
}

func (s *s3Store) Stat(ctx context.Context, key string) (Info, error) {
	o, err := s.c.StatObject(ctx, s.bucket, s.key(key), minio.StatObjectOptions{})
	if err != nil {
		if notFound(err) {
			return Info{}, ErrNotFound
		}
		return Info{}, err
	}
	return Info{Size: o.Size}, nil
}

func (s *s3Store) Delete(ctx context.Context, key string) error {
	return s.c.RemoveObject(ctx, s.bucket, s.key(key), minio.RemoveObjectOptions{})
}

func (s *s3Store) DeletePrefix(ctx context.Context, prefix string) error {
	objs := s.c.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: s.key(prefix), Recursive: true})
	for e := range s.c.RemoveObjects(ctx, s.bucket, objs, minio.RemoveObjectsOptions{}) {
		if e.Err != nil {
			return e.Err
		}
	}
	return nil
}

// Check proves the bucket exists and accepts writes by round-tripping a probe object.
func (s *s3Store) Check(ctx context.Context) error {
	ok, err := s.c.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("bucket %q does not exist", s.bucket)
	}
	probe := ".photoly-check-" + uuid.NewString()
	if err := s.Put(ctx, probe, strings.NewReader("ok"), 2); err != nil {
		return fmt.Errorf("bucket is not writable: %w", err)
	}
	return s.Delete(ctx, probe)
}
