package pgstore

import (
	"context"
	"slices"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/guides"
)

// spellLevels is how many spell levels there are, cantrips included.
const spellLevels = 10

// Guides works the compendium's guides out of its SRD entries, for one ruleset or, with none named,
// for the newest rules each entry has.
func (s *Store) Guides(ctx context.Context, ruleset string) (compendium.Guides, error) {
	out := compendium.Guides{Spells: make([]compendium.GuideLevel, spellLevels), Challenges: []compendium.GuideChallenge{}, Tiers: guides.Tiers(), Rarities: []compendium.GuideRarity{}}
	for level := range out.Spells {
		out.Spells[level] = compendium.GuideLevel{Level: level, Spells: []compendium.GuideSpell{}}
	}
	spells, err := s.q.GuideSpells(ctx, optText(ruleset))
	if err != nil {
		return out, err
	}
	for _, r := range spells {
		out.Spells[r.Level].Spells = append(out.Spells[r.Level].Spells, compendium.GuideSpell{Slug: r.Slug, Name: r.Name, School: r.School})
	}
	if out.Challenges, err = s.guideChallenges(ctx, ruleset); err != nil {
		return out, err
	}
	items, err := s.q.GuideMagicItems(ctx, optText(ruleset))
	if err != nil {
		return out, err
	}
	// The rarities come in the order the tiers open them; one the guide never offers is left out.
	for _, rarity := range out.Tiers[len(out.Tiers)-1].Rarities {
		out.Rarities = append(out.Rarities, compendium.GuideRarity{Rarity: rarity, FirstTier: guides.FirstTier(rarity), Items: []compendium.GuideItem{}})
	}
	for _, r := range items {
		if i := slices.IndexFunc(out.Rarities, func(g compendium.GuideRarity) bool { return g.Rarity == r.Rarity }); i >= 0 {
			out.Rarities[i].Items = append(out.Rarities[i].Items, compendium.GuideItem{Slug: r.Slug, Name: r.Name})
		}
	}
	return out, nil
}

// guideChallenges sums up the monsters of each Challenge Rating and their attacks, from the lowest.
func (s *Store) guideChallenges(ctx context.Context, ruleset string) ([]compendium.GuideChallenge, error) {
	out := []compendium.GuideChallenge{}
	counts, err := s.q.GuideChallenges(ctx, optText(ruleset))
	if err != nil {
		return out, err
	}
	attacks, err := s.q.GuideAttacks(ctx, optText(ruleset))
	if err != nil {
		return out, err
	}
	for _, c := range counts {
		var theirs []guides.Attack
		for _, a := range attacks {
			if a.ChallengeRating == c.ChallengeRating {
				theirs = append(theirs, guides.Attack{ToHit: int(a.ToHit), Dice: a.DamageDice, Bonus: int(a.DamageBonus), ExtraDice: a.ExtraDice})
			}
		}
		out = append(out, compendium.GuideChallenge{Challenge: guides.Challenge(c.ChallengeRating), Monsters: int(c.Monsters), Band: guides.Summarise(theirs)})
	}
	return out, nil
}
