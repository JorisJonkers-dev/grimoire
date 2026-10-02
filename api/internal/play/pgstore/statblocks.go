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
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/actions"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/attack"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/mastery"
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
		UnarmedDC: actions.UnarmedDC(rules.Modifier(int(m.Strength)), rules.ProficiencyByChallenge(m.ChallengeRating)),
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
	if stats.Senses, err = s.senses(ctx, m.ID); err != nil {
		return "", domain.Stats{}, err
	}
	if err := s.ambush(ctx, m, &stats); err != nil {
		return "", domain.Stats{}, err
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
		Shield: sheet.Class == "wizard" || sheet.Class == "sorcerer", Saves: map[string]int{}, UnarmedDC: actions.UnarmedDC(str, pb),
		Attacks:          []domain.Attack{{Name: "Unarmed Strike", ToHit: str + pb, ReachFt: 5, DamageBonus: 1 + str, DamageType: "bludgeoning", DamageMod: str}},
		AttacksPerAction: s.attacksPerAction(ctx, sheet.Class, sheet.Level),
	}
	for _, sv := range sheet.Derived.Saves {
		stats.Saves[string(sv.Ability)] = sv.Bonus
	}
	stats.Initiative, stats.SpeedFt = sheet.Derived.Initiative, sheet.Derived.SpeedFeet
	for _, sk := range sheet.Derived.Skills {
		switch sk.Skill {
		case "stealth":
			stats.Stealth = sk.Bonus
		case "perception":
			stats.Perception = sk.Bonus
		}
	}
	if ability, casts := spellcasting()[sheet.Class]; casts {
		stats.SpellDC = 8 + pb + rules.Modifier(sheet.Scores[ability])
	}
	var carried []string
	for _, w := range sheet.Weapons {
		carried = append(carried, w.Slug)
	}
	mastered := mastery.Mastered(carried, s.masteryCount(ctx, sheet.Class, sheet.Level))
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
			DamageBonus: flat, DamageType: w.DamageType, Light: slices.Contains(w.Properties, "Light"), DamageMod: bonus,
			Mastery: masteryOf(w.Properties, slices.Contains(mastered, w.Slug)),
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

// ambush fills in what an ambush needs from a monster: Stealth and Perception (the ability modifier
// without the skill), initiative from Dexterity and walking speed.
func (s Statblocks) ambush(ctx context.Context, m queries.MonsterStatblockRow, stats *domain.Stats) error {
	stats.Stealth, stats.Perception = rules.Modifier(int(m.Dexterity)), rules.Modifier(int(m.Wisdom))
	stats.Initiative, stats.SpeedFt = stats.Stealth, 30
	rows, err := s.Store.q.MonsterAmbushStats(ctx, m.ID)
	if err != nil {
		return err
	}
	for _, r := range rows {
		switch r.Name {
		case "stealth":
			stats.Stealth = int(r.Value)
		case "perception":
			stats.Perception = int(r.Value)
		default:
			stats.SpeedFt = int(r.Value)
		}
	}
	return nil
}

// attacksPerAction is how many attacks a Character's Attack action holds: one, or what their class's
// Extra Attack gives at their level.
func (s Statblocks) attacksPerAction(ctx context.Context, class string, level int) int {
	cat, err := s.Store.Features(ctx)
	if err != nil {
		return 1
	}
	n, _ := cat.Scales[class+"-extra-attack"].Steps.At(level)
	if v, err := strconv.Atoi(n); err == nil {
		return v
	}
	return 1
}

// masteryCount is how many weapons a Character's class lets it master at its level.
func (s Statblocks) masteryCount(ctx context.Context, class string, level int) int {
	cat, err := s.Store.Features(ctx)
	if err != nil {
		return 0
	}
	n, _ := cat.Scales[class+"-weapon-mastery"].Steps.At(level)
	count, _ := strconv.Atoi(n)
	return count
}

// masteryOf is a weapon's mastery when the Character has mastered it.
func masteryOf(properties []string, mastered bool) string {
	m, ok := mastery.Of(properties)
	if !ok || !mastered {
		return ""
	}
	return string(m)
}

// senses reads the blindsight, tremorsense and truesight a monster has, in feet.
func (s Statblocks) senses(ctx context.Context, id int64) (map[string]int, error) {
	rows, err := s.Store.q.MonsterStats(ctx, id)
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, r := range rows {
		if r.Kind == "sense" && slices.Contains([]string{"blindsight", "tremorsense", "truesight"}, r.Name) && r.Value >= 5 {
			out[r.Name] = int(min(r.Value, 1000))
		}
	}
	return out, nil
}
