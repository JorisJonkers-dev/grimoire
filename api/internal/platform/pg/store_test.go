package pg_test

import (
	"context"
	"testing"
	"testing/fstest"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
)

func TestStoreAgainstMigratedDatabase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	url := pgtest.URL(t)
	if err := pg.Migrate(ctx, url); err != nil {
		t.Fatalf("migrate is not idempotent: %v", err)
	}
	store, err := pg.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	created, err := store.InstanceCreatedAt(ctx)
	if err != nil || time.Since(created) > time.Hour {
		t.Fatalf("created = %v, %v", created, err)
	}
	if store.Pool() == nil || store.Queries() == nil {
		t.Fatal("accessors must expose the pool and queries")
	}
}

func TestOpenFailsOnUnreachableDatabase(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := pg.Open(ctx, "postgres://nobody:nothing@127.0.0.1:1/none?sslmode=disable&connect_timeout=1"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := pg.Open(ctx, "::not a url::"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestMigrateFailsOnUnreachableDatabase(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := pg.Migrate(ctx, "postgres://nobody:nothing@127.0.0.1:1/none?sslmode=disable&connect_timeout=1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMigrateRejectsEmptyMigrationSet(t *testing.T) {
	t.Parallel()
	if err := pg.MigrateFS(context.Background(), pgtest.URL(t), fstest.MapFS{"migrations/.keep": {}}); err == nil {
		t.Fatal("expected an error for an empty migration set")
	}
}
