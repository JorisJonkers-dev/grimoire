package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/classbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// ClassBuild is a homebrew class in the class builder: its entry, its design, the slug it is known by
// on a sheet, and how it reads back with its level table.
type ClassBuild struct {
	Entry  domain.Entry
	Design classbuild.Design
	Slug   string
	Lines  []string
}

// firstClass is where a new class starts: a d8 class without spellcasting, its subclass at 3 and the
// SRD's feat levels.
func firstClass() classbuild.Design {
	return classbuild.Design{
		HitDie: 8, Primary: []string{"strength"}, AnyPrimary: false, Saves: []string{"strength", "constitution"}, Armor: []string{"light"},
		Weapons: []string{"simple"}, Skills: 2, SubclassLevel: 3, FeatLevels: []int{4, 8, 12, 16, 19}, Columns: []classbuild.Column{},
		Features: []classbuild.Feature{{Level: 1, Name: "First feature", Text: ""}}, Casting: classbuild.Casting{Kind: "none"},
	}
}

func checkClass(d classbuild.Design) error {
	if err := classbuild.Check(d, rules.Classes()); err != nil {
		return apperr.Refuse(err.Error())
	}
	return nil
}

// Class reads a homebrew class in the builder: one of the caller's, or a Shared Library copy.
func (s *Service) Class(ctx context.Context, c caller.Caller, id uuid.UUID) (ClassBuild, error) {
	e, err := s.designed(ctx, c, id, "class", "class builder")
	if err != nil {
		return ClassBuild{}, err
	}
	d, slug := designOf(e, firstClass()), spellbuild.Slug(id.String())
	return ClassBuild{Entry: e, Design: d, Slug: slug, Lines: classbuild.Lines(classbuild.Compile(slug, e.Name, d))}, nil
}

// SaveClass saves a design for one of the caller's classes as its next Revision.
func (s *Service) SaveClass(ctx context.Context, c caller.Caller, id uuid.UUID, d classbuild.Design) (ClassBuild, error) {
	e, err := s.ownDesigned(ctx, c, id, "class", "class builder")
	if err == nil {
		err = checkClass(d)
	}
	if err == nil {
		err = s.saveDesign(ctx, c, e, d)
	}
	if err != nil {
		return ClassBuild{}, err
	}
	return s.Class(ctx, c, id)
}

// PreviewClass checks a design without saving it and reads it back.
func (s *Service) PreviewClass(name string, d classbuild.Design) (ClassBuild, error) {
	if err := checkClass(d); err != nil {
		return ClassBuild{}, err
	}
	return ClassBuild{Design: d, Lines: classbuild.Lines(classbuild.Compile("preview", name, d))}, nil
}
