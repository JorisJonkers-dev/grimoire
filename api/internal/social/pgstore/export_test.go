package pgstore

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
)

// NewFaulty is a Store whose database calls fail as f says.
func NewFaulty(pool *pgxpool.Pool, f *pgtest.Faulty) *Store {
	return newWrapped(pool, f.On)
}
