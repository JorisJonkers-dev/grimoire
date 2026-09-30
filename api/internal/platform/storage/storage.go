// Package storage keeps binary assets (portraits, maps, handouts) in a private object store.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// ErrNotFound is returned for a key that holds nothing.
var ErrNotFound = errors.New("storage: not found")

// Blobs stores immutable objects by key.
type Blobs interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
}

// Dir keeps objects as files under a directory; for development and tests.
type Dir struct {
	Path string
}

func (d Dir) file(key string) (string, error) {
	clean := filepath.Clean(key)
	if clean != key || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", fmt.Errorf("storage: bad key %q", key)
	}
	return filepath.Join(d.Path, clean), nil
}

// Put writes an object.
func (d Dir) Put(_ context.Context, key, _ string, data []byte) error {
	name, err := d.file(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(name), 0o750); err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	return os.WriteFile(name, data, 0o600) //nolint:gosec // G703: the key was cleaned and confined to the directory
}

// Get reads an object.
func (d Dir) Get(_ context.Context, key string) ([]byte, error) {
	name, err := d.file(key)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(name) //nolint:gosec // G304: the key was cleaned and confined to the directory
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return data, err
}

// S3 keeps objects in an S3-compatible bucket such as Garage.
type S3 struct {
	Client *s3.Client
	Bucket string
}

// S3Config locates a bucket.
type S3Config struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
}

// NewS3 connects to an S3-compatible endpoint with static credentials and path-style addressing.
func NewS3(c S3Config) *S3 {
	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(c.Endpoint),
		Region:       c.Region,
		Credentials:  credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, ""),
		UsePathStyle: true,
	})
	return &S3{Client: client, Bucket: c.Bucket}
}

// Put uploads an object.
func (s *S3) Put(ctx context.Context, key, contentType string, data []byte) error {
	_, err := s.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.Bucket), Key: aws.String(key), Body: bytes.NewReader(data), ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("storage: put %s: %w", key, err)
	}
	return nil
}

// Get downloads an object.
func (s *S3) Get(ctx context.Context, key string) ([]byte, error) {
	out, err := s.Client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.Bucket), Key: aws.String(key)})
	var missing *types.NoSuchKey
	if errors.As(err, &missing) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("storage: get %s: %w", key, err)
	}
	defer func() { _ = out.Body.Close() }()
	return io.ReadAll(out.Body)
}
