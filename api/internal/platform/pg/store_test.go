package pg_test

import (
	"context"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

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

// Migrations take timestamp versions, so two branches can each add one: whichever merges second may
// carry the lower version, and it still applies after the higher one has run.
func TestMigrationsApplyOutOfOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	url := pgtest.EmptyURL(t)
	table := func(name string) *fstest.MapFile {
		return &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE " + name + " (id integer PRIMARY KEY);\n")}
	}
	merged := fstest.MapFS{
		"migrations/20261014090000_effect_kind_a.sql": table("kind_a"),
		"migrations/20261014110000_effect_kind_c.sql": table("kind_c"),
	}
	if err := pg.MigrateFS(ctx, url, merged); err != nil {
		t.Fatal(err)
	}
	merged["migrations/20261014100000_effect_kind_b.sql"] = table("kind_b")
	if err := pg.MigrateFS(ctx, url, merged); err != nil {
		t.Fatalf("a lower version merged later is refused: %v", err)
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var applied []int64
	rows, err := pool.Query(ctx, "SELECT version_id FROM goose_db_version WHERE version_id > 0 ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		applied = append(applied, v)
	}
	if len(applied) != 3 || applied[2] != 20261014100000 {
		t.Fatalf("applied in order of arrival = %v", applied)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO kind_b (id) VALUES (1)"); err != nil {
		t.Fatalf("the late migration ran: %v", err)
	}
}
