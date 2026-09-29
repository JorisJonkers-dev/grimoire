package pgstore

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

func presentMonster(ctx context.Context, q *queries.Queries, id int64, e *compendium.EntryDetail) error {
	m, err := q.GetMonsterDetail(ctx, id)
	if err != nil {
		return err
	}
	ac := strconv.Itoa(int(m.ArmorClass))
	if m.ArmorDetail.String != "" {
		ac += " (" + m.ArmorDetail.String + ")"
	}
	addFact(e, "Type", strings.TrimSpace(title(m.Size)+" "+m.CreatureType+", "+m.Alignment))
	addFact(e, "Armor Class", ac)
	addFact(e, "Hit Points", strconv.Itoa(int(m.HitPoints))+" ("+m.HitDice+")")
	addFact(e, "Challenge", strconv.FormatFloat(m.ChallengeRating, 'f', -1, 64)+" ("+strconv.Itoa(int(m.Xp))+" XP)")
	for _, a := range []struct {
		name  string
		score int32
	}{{"STR", m.Strength}, {"DEX", m.Dexterity}, {"CON", m.Constitution}, {"INT", m.Intelligence}, {"WIS", m.Wisdom}, {"CHA", m.Charisma}} {
		addFact(e, a.name, fmt.Sprintf("%d (%+d)", a.score, modifier(a.score)))
	}
	if err := monsterStats(ctx, q, id, e); err != nil {
		return err
	}
	addFact(e, "Passive Perception", strconv.Itoa(int(m.PassivePerception)))
	addFact(e, "Languages", m.Languages.String)
	traits, err := q.MonsterTraits(ctx, id)
	if err != nil {
		return err
	}
	for _, t := range traits {
		addSection(e, t.Name, t.Description)
	}
	actions, err := q.MonsterActions(ctx, id)
	if err != nil {
		return err
	}
	for _, a := range actions {
		heading := a.Name
		if a.ActionType != "action" {
			heading += " (" + strings.ReplaceAll(a.ActionType, "_", " ") + ")"
		}
		addSection(e, heading, a.Description)
	}
	return nil
}

func monsterStats(ctx context.Context, q *queries.Queries, id int64, e *compendium.EntryDetail) error {
	stats, err := q.MonsterStats(ctx, id)
	if err != nil {
		return err
	}
	grouped := map[string][]string{}
	for _, s := range stats {
		value := fmt.Sprintf("%s %+d", title(s.Name), s.Value)
		if s.Kind == "speed" || s.Kind == "sense" {
			value = fmt.Sprintf("%s %d ft", title(s.Name), s.Value)
		}
		grouped[s.Kind] = append(grouped[s.Kind], value)
	}
	addFact(e, "Speed", strings.Join(grouped["speed"], ", "))
	addFact(e, "Saving Throws", strings.Join(grouped["save"], ", "))
	addFact(e, "Skills", strings.Join(grouped["skill"], ", "))
	addFact(e, "Senses", strings.Join(grouped["sense"], ", "))
	relations, err := q.MonsterRelations(ctx, id)
	if err != nil {
		return err
	}
	byRelation := map[string][]string{}
	for _, r := range relations {
		byRelation[r.Relation] = append(byRelation[r.Relation], title(r.TargetSlug))
	}
	addFact(e, "Resistances", strings.Join(byRelation["resistance"], ", "))
	addFact(e, "Immunities", strings.Join(byRelation["immunity"], ", "))
	addFact(e, "Vulnerabilities", strings.Join(byRelation["vulnerability"], ", "))
	addFact(e, "Condition Immunities", strings.Join(byRelation["condition-immunity"], ", "))
	return nil
}

func modifier(score int32) int {
	m := int(score) - 10
	if m < 0 {
		m--
	}
	return m / 2
}
