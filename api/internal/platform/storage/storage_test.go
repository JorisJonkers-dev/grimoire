package storage_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/storage"
)

func exercise(t *testing.T, b storage.Blobs) {
	t.Helper()
	ctx := context.Background()
	if err := b.Put(ctx, "sha256/ab.png", "image/png", []byte("png")); err != nil {
		t.Fatal(err)
	}
	got, err := b.Get(ctx, "sha256/ab.png")
	if err != nil || string(got) != "png" {
		t.Fatalf("get = %q %v", got, err)
	}
	if _, err := b.Get(ctx, "sha256/missing.png"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("missing = %v", err)
	}
}

func TestDir(t *testing.T) {
	t.Parallel()
	dir := storage.Dir{Path: t.TempDir()}
	exercise(t, dir)
	ctx := context.Background()
	for _, bad := range []string{"../escape", "/abs", "a/../b"} {
		if err := dir.Put(ctx, bad, "", nil); err == nil {
			t.Errorf("put %q accepted", bad)
		}
		if _, err := dir.Get(ctx, bad); err == nil {
			t.Errorf("get %q accepted", bad)
		}
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (storage.Dir{Path: blocked}).Put(ctx, "x/y", "", nil); err == nil {
		t.Error("put under a file accepted")
	}
	if err := os.MkdirAll(filepath.Join(dir.Path, "folder"), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := dir.Get(ctx, "folder"); err == nil {
		t.Error("reading a directory succeeded")
	}
}

func TestS3(t *testing.T) {
	t.Parallel()
	backend := s3mem.New()
	if err := backend.CreateBucket("grimoire-assets"); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(gofakes3.New(backend).Server())
	t.Cleanup(srv.Close)
	cfg := storage.S3Config{Endpoint: srv.URL, Region: "garage", Bucket: "grimoire-assets", AccessKey: "k", SecretKey: "s"}
	exercise(t, storage.NewS3(cfg))
	cfg.Bucket = "nope"
	broken := storage.NewS3(cfg)
	if err := broken.Put(context.Background(), "a", "image/png", []byte("x")); err == nil {
		t.Error("put to a missing bucket accepted")
	}
	if _, err := broken.Get(context.Background(), "a"); err == nil || errors.Is(err, storage.ErrNotFound) {
		t.Errorf("get from a missing bucket = %v", err)
	}
}
