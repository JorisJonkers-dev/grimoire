package pgstore

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/actions"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/monsterbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

// homebrewMonster reads a homebrew creature the Campaign's Library adds, by its slug.
func (s Statblocks) homebrewMonster(ctx context.Context, campaign uuid.UUID, slug string) (string, domain.Stats, error) {
	rows, err := s.Store.q.CampaignHomebrewDesigns(ctx, queries.CampaignHomebrewDesignsParams{CampaignID: campaign, Kind: "creature"})
	if err != nil {
		return "", domain.Stats{}, err
	}
	for _, r := range rows {
		if spellbuild.Slug(r.ID.String()) != slug {
			continue
		}
		var d monsterbuild.Design
		_ = json.Unmarshal(r.Design, &d) // stored designs were checked when saved
		return r.Name, monsterStats(monsterbuild.Compile(slug, r.Name, d)), nil
	}
	return "", domain.Stats{}, apperr.ErrNotFound
}

// monsterStats are a built creature's fighting stats on a token.
func monsterStats(m monsterbuild.Monster) domain.Stats {
	d := m.Design
	stats := domain.Stats{
		Source: "monster:" + m.Slug, AC: m.AC, HP: m.HP, HPMax: m.HP, Senses: map[string]int{}, Strength: d.Abilities["strength"], CreatureType: d.CreatureType,
		Attacks: make([]domain.Attack, 0, len(m.Attacks)), Intelligence: d.Abilities["intelligence"], Saves: m.Saves,
		Stealth: m.Stealth, Perception: m.Perception, Initiative: m.Initiative, SpeedFt: m.SpeedFt,
		UnarmedDC: actions.UnarmedDC(rules.Modifier(d.Abilities["strength"]), m.Proficiency), AttacksPerAction: m.AttacksPerAction,
	}
	for kind, ft := range m.Senses {
		if kind != "darkvision" {
			stats.Senses[kind] = ft
		}
	}
	for _, a := range m.Attacks {
		stats.Attacks = append(stats.Attacks, domain.Attack{
			Name: a.Name, ToHit: a.ToHit, ReachFt: a.ReachFt, RangeFt: a.RangeFt, LongRangeFt: max(a.LongRangeFt, a.RangeFt),
			Damage: a.Damage, DamageBonus: a.DamageBonus, DamageType: a.DamageType,
		})
	}
	if m.Legendary != nil || m.Lair != nil || len(m.Phases) > 0 || m.Threshold > 0 {
		stats.Legend = legendOf(m)
	}
	return stats
}

func legendOf(m monsterbuild.Monster) *domain.Legend {
	l := &domain.Legend{Threshold: m.Threshold, Actions: []domain.LegendAction{}, Lair: []domain.LegendAction{}, Phases: []domain.Phase{}}
	if g := m.Legendary; g != nil {
		l.Uses, l.Left, l.Resistance, l.ResistLeft = g.Uses, g.Uses, g.Resistance, g.Resistance
		for _, a := range g.Actions {
			l.Actions = append(l.Actions, domain.LegendAction{Name: a.Name, Cost: a.Cost, Text: a.Text})
		}
	}
	if m.Lair != nil {
		for _, a := range m.Lair.Actions {
			l.Lair = append(l.Lair, domain.LegendAction{Name: a.Name, Cost: 0, Text: a.Text})
		}
	}
	for _, p := range m.Phases {
		l.Phases = append(l.Phases, domain.Phase{Name: p.Name, HP: p.HP, Text: p.Text})
	}
	return l
}

// saveLegends writes legendary creatures' Legends as a change leaves them.
func (s *Store) saveLegends(ctx context.Context, sid uuid.UUID, changes []live.LegendChange) error {
	for _, c := range changes {
		raw, _ := json.Marshal(c.Legend) //nolint:errchkjson // a Legend is plain data
		p := queries.SetTokenLegendParams{SessionID: sid, ID: uuid.UUID(c.Token), Legend: raw}
		if c.HPMax != nil {
			p.HpMax = pgInt(*c.HPMax)
		}
		if err := s.q.SetTokenLegend(ctx, p); err != nil {
			return err
		}
	}
	return nil
}
