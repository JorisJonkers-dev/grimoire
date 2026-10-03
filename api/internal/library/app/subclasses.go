package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/subclassbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// SubclassBuild is a homebrew subclass in the subclass builder: its entry, its design, the slug it is
// known by on a sheet, and how it reads back.
type SubclassBuild struct {
	Entry  domain.Entry
	Design subclassbuild.Design
	Slug   string
	Lines  []string
}

// firstSubclass is where a new subclass starts: a fighter's, with one feature at level 3.
func firstSubclass() subclassbuild.Design {
	return subclassbuild.Design{
		Class: "fighter", Features: []subclassbuild.Feature{{Level: 3, Name: "First feature", Text: "", Uses: "", Spell: "", SpellName: ""}},
		Resources: []subclassbuild.Resource{}, Choices: []subclassbuild.Choice{},
	}
}

func checkSubclass(d subclassbuild.Design) error {
	if err := subclassbuild.Check(d, rules.Classes()); err != nil {
		return apperr.Refuse(err.Error())
	}
	return nil
}

// subclassLines reads a checked design back; stored designs were checked when saved.
func subclassLines(slug, name string, d subclassbuild.Design) []string {
	return subclassbuild.Lines(subclassbuild.Compile(slug, name, d))
}

// Subclass reads a homebrew subclass in the builder: one of the caller's, or a Shared Library copy.
func (s *Service) Subclass(ctx context.Context, c caller.Caller, id uuid.UUID) (SubclassBuild, error) {
	e, err := s.designed(ctx, c, id, "subclass", "subclass builder")
	if err != nil {
		return SubclassBuild{}, err
	}
	d, slug := designOf(e, firstSubclass()), spellbuild.Slug(id.String())
	return SubclassBuild{Entry: e, Design: d, Slug: slug, Lines: subclassLines(slug, e.Name, d)}, nil
}

// SaveSubclass saves a design for one of the caller's subclasses as its next Revision.
func (s *Service) SaveSubclass(ctx context.Context, c caller.Caller, id uuid.UUID, d subclassbuild.Design) (SubclassBuild, error) {
	e, err := s.ownDesigned(ctx, c, id, "subclass", "subclass builder")
	if err == nil {
		err = checkSubclass(d)
	}
	if err == nil {
		err = s.saveDesign(ctx, c, e, d)
	}
	if err != nil {
		return SubclassBuild{}, err
	}
	return s.Subclass(ctx, c, id)
}

// PreviewSubclass checks a design without saving it and reads it back.
func (s *Service) PreviewSubclass(name string, d subclassbuild.Design) (SubclassBuild, error) {
	if err := checkSubclass(d); err != nil {
		return SubclassBuild{}, err
	}
	return SubclassBuild{Design: d, Lines: subclassLines("preview", name, d)}, nil
}
