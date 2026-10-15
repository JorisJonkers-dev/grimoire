package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// SpellBuild is a homebrew spell in the Effect builder: its entry, its design, the Effect it builds
// into with its rules text, its area drawn as hexes, and what it takes as Parts.
type SpellBuild struct {
	Entry  domain.Entry
	Design spellbuild.Design
	Built  spellbuild.Built
	Hexes  []hex.Coord
	Holds  effects.Holds
}

// Surfaces lists the Surface kinds a homebrew spell may lay down.
type Surfaces func(ctx context.Context) ([]string, error)

// firstDesign is where a new spell starts: a 20-foot sphere within 60 feet, cast with an action.
func firstDesign() spellbuild.Design {
	return spellbuild.Design{
		Targeting: spellbuild.Targeting{Shape: "sphere", SizeFt: 20, RangeFt: 60}, Duration: spellbuild.Duration{Unit: "instant"},
		CastingTime: spellbuild.CastingTime{Kind: "action"}, Components: spellbuild.Components{Verbal: true, Somatic: true}, Parts: []effects.Authored{},
	}
}

// build checks a design against the Surfaces there are and builds it, with what a spell takes as Parts.
func (s *Service) build(ctx context.Context, id uuid.UUID, name string, d spellbuild.Design) (spellbuild.Built, effects.Holds, error) {
	kinds, err := s.Surfaces(ctx)
	if err != nil {
		return spellbuild.Built{}, effects.Holds{}, err
	}
	b, err := spellbuild.Build(spellbuild.Slug(id.String()), name, d, kinds)
	if err != nil {
		return b, effects.Holds{}, apperr.Refuse(err.Error())
	}
	return b, spellbuild.Holds(kinds), nil
}

// Spell reads a homebrew spell in the builder: one of the caller's, or a Shared Library copy.
func (s *Service) Spell(ctx context.Context, c caller.Caller, id uuid.UUID) (SpellBuild, error) {
	e, err := s.designed(ctx, c, id, "spell", "Effect builder")
	if err != nil {
		return SpellBuild{}, err
	}
	d := designOf(e, firstDesign())
	b, holds, err := s.build(ctx, id, e.Name, d)
	return SpellBuild{Entry: e, Design: d, Built: b, Hexes: spellbuild.Preview(d.Targeting), Holds: holds}, err
}

// SaveSpell saves a design for one of the caller's spells as its next Revision.
func (s *Service) SaveSpell(ctx context.Context, c caller.Caller, id uuid.UUID, d spellbuild.Design) (SpellBuild, error) {
	e, err := s.ownDesigned(ctx, c, id, "spell", "Effect builder")
	if err != nil {
		return SpellBuild{}, err
	}
	if _, _, err := s.build(ctx, id, e.Name, d); err != nil {
		return SpellBuild{}, err
	}
	if err := s.saveDesign(ctx, c, e, d); err != nil {
		return SpellBuild{}, err
	}
	return s.Spell(ctx, c, id)
}

// PreviewSpell builds a design without saving it: its rules text and its area.
func (s *Service) PreviewSpell(ctx context.Context, name string, d spellbuild.Design) (SpellBuild, error) {
	b, holds, err := s.build(ctx, uuid.Nil, name, d)
	return SpellBuild{Design: d, Built: b, Hexes: spellbuild.Preview(d.Targeting), Holds: holds}, err
}
