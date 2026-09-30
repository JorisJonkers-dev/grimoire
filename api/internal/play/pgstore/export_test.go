package pgstore

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
)

// NewFaulty is a Store whose every query goes through f.
func NewFaulty(pool *pgxpool.Pool, f *pgtest.Faulty) *Store {
	return newWrapped(pool, f.On)
}

// AddRevealForTest marks a hex as seen by the party.
func (s *Store) AddRevealForTest(ctx context.Context, id domain.MapID, c hex.Coord) error {
	return s.addReveals(ctx, id, []hex.Coord{c})
}
