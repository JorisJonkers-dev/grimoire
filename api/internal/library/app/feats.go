package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/featbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// FeatBuild is a homebrew feat in the feat builder: its entry, its design, the slug it is known by,
// and how it reads back.
type FeatBuild struct {
	Entry  domain.Entry
	Design featbuild.Feat
	Slug   string
	Lines  []string
}

// BackgroundBuild is a homebrew background in the background builder.
type BackgroundBuild struct {
	Entry  domain.Entry
	Design featbuild.Background
	Slug   string
	Lines  []string
}

func firstFeat() featbuild.Feat {
	return featbuild.Feat{Category: "general", Text: "", Repeatable: false, Prerequisites: []featbuild.Prerequisite{}}
}

func firstBackground() featbuild.Background {
	return featbuild.Background{
		Abilities: []string{"strength", "dexterity", "constitution"}, Skills: []string{"athletics", "survival"}, Feat: "alert", FeatName: "Alert",
		Tool: "", Equipment: "", Gold: 50, Text: "",
	}
}

func checkFeat(d featbuild.Feat) error {
	if err := featbuild.CheckFeat(d); err != nil {
		return apperr.Refuse(err.Error())
	}
	return nil
}

func checkBackground(d featbuild.Background) error {
	if err := featbuild.CheckBackground(d); err != nil {
		return apperr.Refuse(err.Error())
	}
	return nil
}

// Feat reads a homebrew feat in the builder: one of the caller's, or a Shared Library copy.
func (s *Service) Feat(ctx context.Context, c caller.Caller, id uuid.UUID) (FeatBuild, error) {
	e, err := s.designed(ctx, c, id, "feat", "feat builder")
	if err != nil {
		return FeatBuild{}, err
	}
	d, slug := designOf(e, firstFeat()), spellbuild.Slug(id.String())
	return FeatBuild{Entry: e, Design: d, Slug: slug, Lines: featbuild.FeatLines(featbuild.CompileFeat(slug, e.Name, d))}, nil
}

// SaveFeat saves a design for one of the caller's feats as its next Revision.
func (s *Service) SaveFeat(ctx context.Context, c caller.Caller, id uuid.UUID, d featbuild.Feat) (FeatBuild, error) {
	e, err := s.ownDesigned(ctx, c, id, "feat", "feat builder")
	if err == nil {
		err = checkFeat(d)
	}
	if err == nil {
		err = s.saveDesign(ctx, c, e, d)
	}
	if err != nil {
		return FeatBuild{}, err
	}
	return s.Feat(ctx, c, id)
}

// PreviewFeat checks a design without saving it and reads it back.
func (s *Service) PreviewFeat(name string, d featbuild.Feat) (FeatBuild, error) {
	if err := checkFeat(d); err != nil {
		return FeatBuild{}, err
	}
	return FeatBuild{Design: d, Lines: featbuild.FeatLines(featbuild.CompileFeat("preview", name, d))}, nil
}

// Background reads a homebrew background in the builder: one of the caller's, or a Shared Library copy.
func (s *Service) Background(ctx context.Context, c caller.Caller, id uuid.UUID) (BackgroundBuild, error) {
	e, err := s.designed(ctx, c, id, "background", "background builder")
	if err != nil {
		return BackgroundBuild{}, err
	}
	d, slug := designOf(e, firstBackground()), spellbuild.Slug(id.String())
	return BackgroundBuild{Entry: e, Design: d, Slug: slug, Lines: featbuild.BackgroundLines(featbuild.CompileBackground(slug, e.Name, d))}, nil
}

// SaveBackground saves a design for one of the caller's backgrounds as its next Revision.
func (s *Service) SaveBackground(ctx context.Context, c caller.Caller, id uuid.UUID, d featbuild.Background) (BackgroundBuild, error) {
	e, err := s.ownDesigned(ctx, c, id, "background", "background builder")
	if err == nil {
		err = checkBackground(d)
	}
	if err == nil {
		err = s.saveDesign(ctx, c, e, d)
	}
	if err != nil {
		return BackgroundBuild{}, err
	}
	return s.Background(ctx, c, id)
}

// PreviewBackground checks a design without saving it and reads it back.
func (s *Service) PreviewBackground(name string, d featbuild.Background) (BackgroundBuild, error) {
	if err := checkBackground(d); err != nil {
		return BackgroundBuild{}, err
	}
	return BackgroundBuild{Design: d, Lines: featbuild.BackgroundLines(featbuild.CompileBackground("preview", name, d))}, nil
}
