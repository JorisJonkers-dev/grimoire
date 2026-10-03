package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/monsterbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// MonsterBuild is a homebrew creature in the monster builder: its entry, its design, the slug it is
// placed by in play, its stat block, and an estimated Challenge.
type MonsterBuild struct {
	Entry  domain.Entry
	Design monsterbuild.Design
	Slug   string
	Lines  []string
	// Estimate is a rough Challenge from the stat block.
	Estimate string
}

// firstMonster is where a new creature starts: a Medium beast with one bite.
func firstMonster() monsterbuild.Design {
	return monsterbuild.Design{
		Size: "medium", CreatureType: "beast", AC: 12, HP: 11, SpeedFt: 30, Challenge: 0.5,
		Abilities: map[string]int{"strength": 12, "dexterity": 12, "constitution": 12, "intelligence": 3, "wisdom": 10, "charisma": 6},
		Saves:     []string{}, Senses: []monsterbuild.Measure{}, Resistances: []string{}, Immunities: []string{}, Vulnerabilities: []string{},
		Traits: []monsterbuild.Trait{}, Phases: []monsterbuild.Phase{},
		Actions: []monsterbuild.Action{{Name: "Bite", Kind: "melee", ToHit: 3, ReachFt: 5, Damage: "1d6", DamageBonus: 1, DamageType: "piercing"}},
	}
}

func checkMonster(d monsterbuild.Design) error {
	if err := monsterbuild.Check(d); err != nil {
		return apperr.Refuse(err.Error())
	}
	return nil
}

// Species reads a homebrew creature in the builder: one of the caller's, or a Shared Library copy.
func (s *Service) Monster(ctx context.Context, c caller.Caller, id uuid.UUID) (MonsterBuild, error) {
	e, err := s.designed(ctx, c, id, "creature", "monster builder")
	if err != nil {
		return MonsterBuild{}, err
	}
	d, slug := designOf(e, firstMonster()), spellbuild.Slug(id.String())
	m := monsterbuild.Compile(slug, e.Name, d)
	return MonsterBuild{Entry: e, Design: d, Slug: slug, Lines: monsterbuild.Lines(m), Estimate: monsterbuild.Estimate(m)}, nil
}

// SaveMonster saves a design for one of the caller's creatures as its next Revision.
func (s *Service) SaveMonster(ctx context.Context, c caller.Caller, id uuid.UUID, d monsterbuild.Design) (MonsterBuild, error) {
	e, err := s.ownDesigned(ctx, c, id, "creature", "monster builder")
	if err == nil {
		err = checkMonster(d)
	}
	if err == nil {
		err = s.saveDesign(ctx, c, e, d)
	}
	if err != nil {
		return MonsterBuild{}, err
	}
	return s.Monster(ctx, c, id)
}

// PreviewMonster checks a design without saving it and reads it back.
func (s *Service) PreviewMonster(name string, d monsterbuild.Design) (MonsterBuild, error) {
	if err := checkMonster(d); err != nil {
		return MonsterBuild{}, err
	}
	m := monsterbuild.Compile("preview", name, d)
	return MonsterBuild{Design: d, Lines: monsterbuild.Lines(m), Estimate: monsterbuild.Estimate(m)}, nil
}
