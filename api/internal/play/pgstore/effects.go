package pgstore

import (
	"context"

	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// Effects reads the Effect catalogue from the compendium.
func (s *Store) Effects(ctx context.Context) (effects.Catalog, error) {
	return comppg.New(s.pool).Effects(ctx)
}
