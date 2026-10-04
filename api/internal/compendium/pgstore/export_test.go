package pgstore

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

// NewFaulty is a Store whose database calls fail as f says.
func NewFaulty(pool *pgxpool.Pool, f *pgtest.Faulty) *Store {
	return &Store{pool: pool, q: queries.New(f.On(pool))}
}
