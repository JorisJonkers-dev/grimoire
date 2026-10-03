package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/conditionbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// ConditionBuild is a homebrew condition in the condition builder: its entry, its design, the slug it
// is known by in play, and how it reads back.
type ConditionBuild struct {
	Entry  domain.Entry
	Design conditionbuild.Design
	Slug   string
	Lines  []string
}

// firstCondition is where a new condition starts: a plain one, lasting until removed.
func firstCondition() conditionbuild.Design {
	return conditionbuild.Design{
		Icon: "spiral", Color: "#bfb199", Text: "", Ends: "removed", Ability: "", Stacks: false, MaxLevel: 0,
		PerLevel: conditionbuild.Penalty{D20: 0, SpeedFt: 0, DeathAt: 0}, Parts: []conditionbuild.Part{},
	}
}

func checkCondition(d conditionbuild.Design) error {
	if err := conditionbuild.Check(d); err != nil {
		return apperr.Refuse(err.Error())
	}
	return nil
}

// Species reads a homebrew condition in the builder: one of the caller's, or a Shared Library copy.
func (s *Service) Condition(ctx context.Context, c caller.Caller, id uuid.UUID) (ConditionBuild, error) {
	e, err := s.designed(ctx, c, id, "condition", "condition builder")
	if err != nil {
		return ConditionBuild{}, err
	}
	d, slug := designOf(e, firstCondition()), spellbuild.Slug(id.String())
	return ConditionBuild{Entry: e, Design: d, Slug: slug, Lines: conditionbuild.Lines(conditionbuild.Compile(slug, e.Name, d))}, nil
}

// SaveCondition saves a design for one of the caller's conditions as its next Revision.
func (s *Service) SaveCondition(ctx context.Context, c caller.Caller, id uuid.UUID, d conditionbuild.Design) (ConditionBuild, error) {
	e, err := s.ownDesigned(ctx, c, id, "condition", "condition builder")
	if err == nil {
		err = checkCondition(d)
	}
	if err == nil {
		err = s.saveDesign(ctx, c, e, d)
	}
	if err != nil {
		return ConditionBuild{}, err
	}
	return s.Condition(ctx, c, id)
}

// PreviewCondition checks a design without saving it and reads it back.
func (s *Service) PreviewCondition(name string, d conditionbuild.Design) (ConditionBuild, error) {
	if err := checkCondition(d); err != nil {
		return ConditionBuild{}, err
	}
	return ConditionBuild{Design: d, Lines: conditionbuild.Lines(conditionbuild.Compile("preview", name, d))}, nil
}
