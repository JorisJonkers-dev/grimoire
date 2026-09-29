package pgstore

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

// NewFaulty is a Store whose every query goes through f.
func NewFaulty(pool *pgxpool.Pool, f *pgtest.Faulty) *Store {
	return newWrapped(pool, func(db queries.DBTX) queries.DBTX {
		f.DB = db
		return f
	})
}
