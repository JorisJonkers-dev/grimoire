package pgstore

import (
	"context"
	"errors"
	"slices"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/attack"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

var _ live.Statblocks = Statblocks{}

// Statblocks copies fighting stats onto tokens: a monster from the compendium, or a Character's sheet.
type Statblocks struct {
	Store      *Store
	Characters *campaignapp.Characters
}

// Monster reads a monster's AC, hit points and attacks in the Campaign's ruleset.
func (s Statblocks) Monster(ctx context.Context, campaign uuid.UUID, slug string) (string, domain.Stats, error) {
	ruleset, err := s.Store.q.CampaignRuleset(ctx, campaign)
	if err != nil {
		return "", domain.Stats{}, notFound(err)
	}
	m, err := s.Store.q.MonsterStatblock(ctx, queries.MonsterStatblockParams{Slug: slug, Ruleset: pgtype.Text{String: ruleset, Valid: true}})
	if errors.Is(err, pgx.ErrNoRows) {
		m, err = s.Store.q.MonsterStatblock(ctx, queries.MonsterStatblockParams{Slug: slug})
	}
	if err != nil {
		return "", domain.Stats{}, notFound(err)
	}
	rows, err := s.Store.q.MonsterAttackRows(ctx, m.ID)
	if err != nil {
		return "", domain.Stats{}, err
	}
	stats := domain.Stats{
		Source: "monster:" + slug, AC: int(m.ArmorClass), HP: int(m.HitPoints), HPMax: int(m.HitPoints), Attacks: []domain.Attack{},
		Intelligence: int(m.Intelligence), Saves: map[string]int{},
	}
	scores := map[string]int32{
		"strength": m.Strength, "dexterity": m.Dexterity, "constitution": m.Constitution, "intelligence": m.Intelligence, "wisdom": m.Wisdom,
		"charisma": m.Charisma,
	}
	for ability, score := range scores {
		stats.Saves[ability] = rules.Modifier(int(score))
	}
	saves, err := s.Store.q.MonsterSaves(ctx, m.ID)
	if err != nil {
		return "", domain.Stats{}, err
	}
	for _, sv := range saves {
		stats.Saves[sv.Name] = int(sv.Value)
	}
	for _, r := range rows {
		stats.Attacks = append(stats.Attacks, domain.Attack{
			Name: r.Name, ToHit: int(r.ToHit), ReachFt: int(r.ReachFeet), RangeFt: int(r.RangeFeet), LongRangeFt: max(int(r.LongRangeFeet), int(r.RangeFeet)),
			DamageType: r.DamageType,
		})
		last := &stats.Attacks[len(stats.Attacks)-1]
		last.Damage, last.DamageBonus = damage(r.DamageDice, int(r.DamageBonus))
		if _, err := dice.Parse(r.ExtraDice); err == nil && last.Damage != "" {
			last.Damage += "+" + r.ExtraDice
			last.DamageType += " and " + r.ExtraType
		}
	}
	return m.Name, stats, nil
}

// Character reads a Character's sheet as its owner or the DM sees it, with an attack per weapon it
// carries and an unarmed strike.
func (s Statblocks) Character(ctx context.Context, c caller.Caller, campaign, id uuid.UUID) (string, uuid.UUID, domain.Stats, error) {
	sheet, err := s.Characters.Get(ctx, c, campaigndomain.CampaignID(campaign), campaigndomain.CharacterID(id))
	if err != nil {
		return "", uuid.UUID{}, domain.Stats{}, err
	}
	str, dex := rules.Modifier(sheet.Scores[rules.Strength]), rules.Modifier(sheet.Scores[rules.Dexterity])
	pb := sheet.Derived.ProficiencyBonus
	stats := domain.Stats{
		Source: "character:" + id.String(), AC: sheet.Derived.ArmorClass, HP: sheet.HPCurrent, HPMax: sheet.HPMax,
		Shield: sheet.Class == "wizard" || sheet.Class == "sorcerer", Saves: map[string]int{},
		Attacks: []domain.Attack{{Name: "Unarmed Strike", ToHit: str + pb, ReachFt: 5, DamageBonus: 1 + str, DamageType: "bludgeoning"}},
	}
	for _, sv := range sheet.Derived.Saves {
		stats.Saves[string(sv.Ability)] = sv.Bonus
	}
	if ability, casts := spellcasting()[sheet.Class]; casts {
		stats.SpellDC = 8 + pb + rules.Modifier(sheet.Scores[ability])
	}
	for _, w := range sheet.Weapons {
		props := attack.Weapon{
			Finesse: slices.Contains(w.Properties, "Finesse"), Ammunition: slices.Contains(w.Properties, "Ammunition"),
			Reach: slices.Contains(w.Properties, "Reach"),
		}
		toHit, bonus, reach := attack.WeaponAttack(props, str, dex, pb)
		if props.Ammunition {
			reach = 0
		}
		dmg, flat := damage(w.DamageDice, bonus)
		stats.Attacks = append(stats.Attacks, domain.Attack{
			Name: w.Name, ToHit: toHit, ReachFt: reach, RangeFt: w.RangeFeet, LongRangeFt: w.LongRangeFeet, Damage: dmg,
			DamageBonus: flat, DamageType: w.DamageType,
		})
	}
	return sheet.Name, uuid.UUID(sheet.Owner.ID), stats, nil
}

// damage splits compendium damage into dice notation and a flat bonus: a plain number such as a
// blowgun's "1" is flat damage.
func damage(notation string, bonus int) (string, int) {
	if _, err := dice.Parse(notation); err == nil {
		return notation, bonus
	}
	n, _ := strconv.Atoi(notation)
	return "", bonus + n
}

// spellcasting is the ability each spellcasting class casts with.
func spellcasting() map[string]rules.Ability {
	return map[string]rules.Ability{
		"bard": rules.Charisma, "cleric": rules.Wisdom, "druid": rules.Wisdom, "paladin": rules.Charisma, "ranger": rules.Wisdom,
		"sorcerer": rules.Charisma, "warlock": rules.Charisma, "wizard": rules.Intelligence,
	}
}
