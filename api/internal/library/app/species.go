package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/speciesbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// SpeciesBuild is a homebrew species in the species builder: its entry, its design, the slug it is
// known by (a lineage adds its own), and how each option reads back.
type SpeciesBuild struct {
	Entry  domain.Entry
	Design speciesbuild.Design
	Slug   string
	Lines  []string
}

// firstSpecies is where a new species starts: a Medium humanoid walking 30 feet.
func firstSpecies() speciesbuild.Design {
	return speciesbuild.Design{
		Sizes: []string{"medium"}, CreatureType: "humanoid", SpeedFt: 30, Speeds: []speciesbuild.Speed{}, Senses: []speciesbuild.Sense{},
		Resistances: []string{}, Traits: []speciesbuild.Trait{}, Spells: []speciesbuild.Spell{}, Lineages: []speciesbuild.Lineage{},
	}
}

func checkSpecies(d speciesbuild.Design) error {
	if err := speciesbuild.Check(d); err != nil {
		return apperr.Refuse(err.Error())
	}
	return nil
}

// Species reads a homebrew species in the builder: one of the caller's, or a Shared Library copy.
func (s *Service) Species(ctx context.Context, c caller.Caller, id uuid.UUID) (SpeciesBuild, error) {
	e, err := s.designed(ctx, c, id, "species", "species builder")
	if err != nil {
		return SpeciesBuild{}, err
	}
	d, slug := designOf(e, firstSpecies()), spellbuild.Slug(id.String())
	return SpeciesBuild{Entry: e, Design: d, Slug: slug, Lines: speciesbuild.Lines(speciesbuild.Compile(slug, e.Name, d))}, nil
}

// SaveSpecies saves a design for one of the caller's species as its next Revision.
func (s *Service) SaveSpecies(ctx context.Context, c caller.Caller, id uuid.UUID, d speciesbuild.Design) (SpeciesBuild, error) {
	e, err := s.ownDesigned(ctx, c, id, "species", "species builder")
	if err == nil {
		err = checkSpecies(d)
	}
	if err == nil {
		err = s.saveDesign(ctx, c, e, d)
	}
	if err != nil {
		return SpeciesBuild{}, err
	}
	return s.Species(ctx, c, id)
}

// PreviewSpecies checks a design without saving it and reads it back.
func (s *Service) PreviewSpecies(name string, d speciesbuild.Design) (SpeciesBuild, error) {
	if err := checkSpecies(d); err != nil {
		return SpeciesBuild{}, err
	}
	return SpeciesBuild{Design: d, Lines: speciesbuild.Lines(speciesbuild.Compile("preview", name, d))}, nil
}
