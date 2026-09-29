// Package pg is the Postgres adapter: connection pool, migrations and sqlc-backed queries.
package pg

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"

	"github.com/JorisJonkers-dev/grimoire/api/db"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

// Store owns the connection pool and exposes the queries the application needs.
type Store struct {
	pool *pgxpool.Pool
	q    *queries.Queries
}

// Open connects to Postgres and verifies the connection.
func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("pg: open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pg: ping: %w", err)
	}
	return &Store{pool: pool, q: queries.New(pool)}, nil
}

// Close releases every pooled connection.
func (s *Store) Close() { s.pool.Close() }

// Pool exposes the pool to other adapters in this package tree.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// Queries exposes the generated queries to other adapters in this package tree.
func (s *Store) Queries() *queries.Queries { return s.q }

// Ping reports whether the database answers.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// InstanceCreatedAt returns when this database was first migrated.
func (s *Store) InstanceCreatedAt(ctx context.Context) (time.Time, error) {
	return s.q.GetInstanceCreatedAt(ctx)
}

// Migrate applies every pending embedded migration.
func Migrate(ctx context.Context, url string) error {
	return MigrateFS(ctx, url, db.Migrations)
}

// MigrateFS applies the migrations under fsys/migrations; tests use it with synthetic sets.
func MigrateFS(ctx context.Context, url string, fsys fs.FS) error {
	sqlDB, err := sql.Open("pgx", url)
	if err != nil {
		return fmt.Errorf("pg: open for migrate: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()
	return migrateDB(ctx, sqlDB, fsys)
}

func migrateDB(ctx context.Context, sqlDB *sql.DB, fsys fs.FS) error {
	sub, err := fs.Sub(fsys, "migrations")
	if err != nil {
		return fmt.Errorf("pg: migrations dir: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, sub)
	if err != nil {
		return fmt.Errorf("pg: migration provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("pg: migrate up: %w", err)
	}
	return nil
}
