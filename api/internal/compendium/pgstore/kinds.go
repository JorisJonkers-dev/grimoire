package pgstore

import (
	"context"
	"fmt"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// partRows stores one kind of Part in its own table, through its own queries.
type partRows interface {
	kind() string
	insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error
	read(ctx context.Context, q *queries.Queries, into map[slot]effects.Component) error
}

// holdingRows is the store of a kind with Parts beneath it: they are written under the Part's row,
// and put back beneath it when it is read.
type holdingRows interface {
	beneath(p effects.Part) []effects.Component
	nest(p effects.Part, beneath []effects.Component) effects.Part
}

// kindRows lists the store of every kind defined in a file of its own, in the order of effects.Kinds.
// It is this layer's one shared line per kind: rules cannot name their adapters.
func kindRows() []partRows {
	return []partRows{
		bonusDieRows{},
		branchRows{},
		saveDamageRows{},
	}
}

func rowsOf(kind string) (partRows, error) {
	for _, r := range kindRows() {
		if r.kind() == kind {
			return r, nil
		}
	}
	return nil, fmt.Errorf("no table stores %s parts", kind)
}

// holdingOf is the store of a kind that holds Parts beneath it, if the kind does.
func holdingOf(kind string) (holdingRows, bool) {
	r, err := rowsOf(kind)
	if err != nil {
		return nil, false
	}
	h, ok := r.(holdingRows)
	return h, ok
}

// readParts reads the rows of every kind defined in a file of its own.
func readParts(ctx context.Context, q *queries.Queries, into map[slot]effects.Component) error {
	for _, r := range kindRows() {
		if err := r.read(ctx, q, into); err != nil {
			return err
		}
	}
	return nil
}
