package app

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// SpellBuild is a homebrew spell in the Effect builder: its entry, its design, the Effect it builds
// into with its rules text, and its area drawn as hexes.
type SpellBuild struct {
	Entry  domain.Entry
	Design spellbuild.Design
	Built  spellbuild.Built
	Hexes  []hex.Coord
}

// Surfaces lists the Surface kinds a homebrew spell may lay down.
type Surfaces func(ctx context.Context) ([]string, error)

// firstDesign is where a new spell starts: a 20-foot sphere within 60 feet, cast with an action.
func firstDesign() spellbuild.Design {
	return spellbuild.Design{
		Targeting: spellbuild.Targeting{Shape: "sphere", SizeFt: 20, RangeFt: 60}, Duration: spellbuild.Duration{Unit: "instant"},
		CastingTime: spellbuild.CastingTime{Kind: "action"}, Components: spellbuild.Components{Verbal: true, Somatic: true}, Parts: []spellbuild.Part{},
	}
}

// build checks a design against the Surfaces there are and builds it.
func (s *Service) build(ctx context.Context, id uuid.UUID, name string, d spellbuild.Design) (spellbuild.Built, error) {
	kinds, err := s.Surfaces(ctx)
	if err != nil {
		return spellbuild.Built{}, err
	}
	b, err := spellbuild.Build(spellbuild.Slug(id.String()), name, d, kinds)
	if err != nil {
		return b, apperr.Refuse(err.Error())
	}
	return b, nil
}

// Spell reads a homebrew spell in the builder: one of the caller's, or a Shared Library copy.
func (s *Service) Spell(ctx context.Context, c caller.Caller, id uuid.UUID) (SpellBuild, error) {
	e, err := s.linkable(ctx, c, id)
	if err == nil && e.Kind != "spell" {
		err = apperr.Refuse("only a spell is built in the Effect builder")
	}
	if err != nil {
		return SpellBuild{}, err
	}
	d := firstDesign()
	if e.Design != nil {
		_ = json.Unmarshal(e.Design, &d) // stored designs were checked when saved
	}
	b, err := s.build(ctx, id, e.Name, d)
	return SpellBuild{Entry: e, Design: d, Built: b, Hexes: spellbuild.Preview(d.Targeting)}, err
}

// SaveSpell saves a design for one of the caller's spells as its next Revision.
func (s *Service) SaveSpell(ctx context.Context, c caller.Caller, id uuid.UUID, d spellbuild.Design) (SpellBuild, error) {
	e, err := s.owned(ctx, c, id)
	if err == nil && e.Kind != "spell" {
		err = apperr.Refuse("only a spell is built in the Effect builder")
	}
	if err != nil {
		return SpellBuild{}, err
	}
	if _, err := s.build(ctx, id, e.Name, d); err != nil {
		return SpellBuild{}, err
	}
	raw, _ := json.Marshal(d) //nolint:errchkjson // a design is plain data
	now := s.Now()
	err = s.Repo.InTx(ctx, func(r Repository) error {
		no, err := r.UpdateEntry(ctx, id, e.Name, e.Fields, raw, now)
		if err != nil {
			return err
		}
		return r.InsertRevision(ctx, id, domain.Revision{No: no, Name: e.Name, Fields: e.Fields, Design: raw, Author: c.Subject, At: now})
	})
	if err != nil {
		return SpellBuild{}, err
	}
	return s.Spell(ctx, c, id)
}

// PreviewSpell builds a design without saving it: its rules text and its area.
func (s *Service) PreviewSpell(ctx context.Context, name string, d spellbuild.Design) (SpellBuild, error) {
	b, err := s.build(ctx, uuid.Nil, name, d)
	return SpellBuild{Design: d, Built: b, Hexes: spellbuild.Preview(d.Targeting)}, err
}
