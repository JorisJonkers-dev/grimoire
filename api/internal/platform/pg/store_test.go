package pg_test

import (
	"context"
	"testing"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
)

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
