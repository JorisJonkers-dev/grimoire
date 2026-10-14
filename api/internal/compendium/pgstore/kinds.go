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

// holdingRows is the store of a kind with Parts beneath it: they are written under the Part's row in
// their named groups, and put back beneath it when it is read.
type holdingRows interface {
	nest(p effects.Part, groups []effects.Group) effects.Part
}

// kindRows lists the store of every kind, in the order of effects.Kinds.
// It is this layer's one shared line per kind: rules cannot name their adapters.
func kindRows() []partRows {
	return []partRows{
		areaRows{},
		areaSaveRows{},
		bonusDieRows{},
		branchRows{},
		choiceRows{},
		counterRows{},
		createSurfaceRows{},
		critWithinRows{},
		dispelRows{},
		edgeRows{},
		exhaustingRows{},
		extraDamageRows{},
		forcedMoveRows{},
		formRows{},
		grantFeatureRows{},
		immobileRows{},
		incapacitatedRows{},
		lightRows{},
		manualRows{},
		moveCostRows{},
		reactsRows{},
		resourceChangeRows{},
		revealRows{},
		saveConditionRows{},
		saveDamageRows{},
		saveEdgeRows{},
		speedPenaltyRows{},
		summonRows{},
		teleportRows{},
		tempHPRows{},
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

// readParts reads the rows of every kind.
func readParts(ctx context.Context, q *queries.Queries, into map[slot]effects.Component) error {
	for _, r := range kindRows() {
		if err := r.read(ctx, q, into); err != nil {
			return err
		}
	}
	return nil
}

// bare puts a Part that is the whole of its kind at every component row of that kind.
func bare(ctx context.Context, q *queries.Queries, into map[slot]effects.Component, p effects.Part) error {
	rows, err := q.ListEffectComponentsOfKind(ctx, p.Kind())
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = p
	}
	return nil
}
