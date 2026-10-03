// Package pgtest gives tests a freshly migrated Postgres database each, from one shared container, or
// from the server GRIMOIRE_TEST_POSTGRES_URL names when Docker is not at hand.
package pgtest

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/JorisJonkers-dev/grimoire/api/db"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
)

var (
	template = "grimoire_template"
	once     sync.Once
	adminURL string
	errStart error
)

// URL returns the connection string of a new database migrated to the latest schema.
func URL(t testing.TB) string {
	t.Helper()
	if testing.Short() {
		t.Skip("needs Docker; skipped with -short")
	}
	once.Do(start)
	if errStart != nil {
		t.Fatalf("pgtest: %v", errStart)
	}
	name := "t_" + randomSuffix()
	exec(t, adminURL, fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", name, template))
	dropAfter(t, name)
	return withDatabase(adminURL, name)
}

// EmptyURL returns the connection string of a new database with no schema, for migration tests.
func EmptyURL(t testing.TB) string {
	t.Helper()
	if testing.Short() {
		t.Skip("needs Docker; skipped with -short")
	}
	once.Do(start)
	if errStart != nil {
		t.Fatalf("pgtest: %v", errStart)
	}
	name := "e_" + randomSuffix()
	exec(t, adminURL, "CREATE DATABASE "+name)
	dropAfter(t, name)
	return withDatabase(adminURL, name)
}

// dropAfter removes a test's database when the test ends. A server that outlives the run, as the one
// GRIMOIRE_TEST_POSTGRES_URL names does, would otherwise keep every database of every run.
func dropAfter(t testing.TB, name string) {
	t.Cleanup(func() {
		if err := execErr(context.Background(), adminURL, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Logf("pgtest: drop %s: %v", name, err)
		}
	})
}

func start() {
	ctx := context.Background()
	if external := os.Getenv("GRIMOIRE_TEST_POSTGRES_URL"); external != "" {
		adminURL = external
		template, errStart = sharedTemplate(ctx, adminURL)
		return
	}
	c, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("grimoire"),
		postgres.WithPassword("grimoire"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		errStart = err
		return
	}
	adminURL, errStart = c.ConnectionString(ctx, "sslmode=disable")
	if errStart != nil {
		return
	}
	if errStart = execErr(ctx, adminURL, "CREATE DATABASE "+template); errStart != nil {
		return
	}
	errStart = pg.Migrate(ctx, withDatabase(adminURL, template))
}

// schemaDigest names the schema the embedded migrations make.
func schemaDigest() string {
	h := sha256.New()
	_ = fs.WalkDir(db.Migrations, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(db.Migrations, path)
		_, _ = h.Write([]byte(path))
		_, _ = h.Write(data)
		return err
	})
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// sharedTemplate is the migrated template on a server test binaries share and that outlives the run:
// one per schema, made by whoever needs it first while the others wait, and found there ever after.
func sharedTemplate(ctx context.Context, admin string) (string, error) {
	name := "grimoire_template_" + schemaDigest()
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close(ctx) }()
	// The lock is the connection's: it goes when the connection does.
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock(hashtext($1))", name); err != nil {
		return "", err
	}
	var there bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", name).Scan(&there); err != nil || there {
		return name, err
	}
	building := name + "_building"
	if _, err := conn.Exec(ctx, "DROP DATABASE IF EXISTS "+building+" WITH (FORCE)"); err != nil {
		return "", err
	}
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+building); err != nil {
		return "", err
	}
	if err := pg.Migrate(ctx, withDatabase(admin, building)); err != nil {
		return "", err
	}
	_, err = conn.Exec(ctx, "ALTER DATABASE "+building+" RENAME TO "+name)
	return name, err
}

func exec(t testing.TB, dsn, sql string) {
	t.Helper()
	if err := execErr(context.Background(), dsn, sql); err != nil {
		t.Fatalf("pgtest: %s: %v", sql, err)
	}
}

func execErr(ctx context.Context, dsn, sql string) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	_, err = conn.Exec(ctx, sql)
	return err
}

func withDatabase(dsn, name string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		panic(err)
	}
	u.Path = "/" + name
	return u.String()
}

func randomSuffix() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
