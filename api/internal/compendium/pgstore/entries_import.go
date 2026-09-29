package pgstore

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

//nolint:gosec // every narrowing conversion in this file is of small, bounded game values
func importEntries(ctx context.Context, q *queries.Queries, l *lookupCache, docIDs map[string]int64, snap snapshot.Snapshot) error {
	steps := []func() error{
		func() error { return importClasses(ctx, q, docIDs, snap.Classes) },
		func() error { return importSpecies(ctx, q, docIDs, snap.Species) },
		func() error { return importBackgrounds(ctx, q, docIDs, snap.Backgrounds) },
		func() error { return importFeats(ctx, q, docIDs, snap.Feats) },
		func() error { return importWeapons(ctx, q, l, docIDs, snap.Weapons) },
		func() error { return importArmor(ctx, q, docIDs, snap.Armor) },
		func() error { return importItems(ctx, q, docIDs, snap.Items) },
		func() error { return importMonsters(ctx, q, docIDs, snap.Monsters) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

func importClasses(ctx context.Context, q *queries.Queries, docIDs map[string]int64, classes []snapshot.Class) error {
	for _, c := range classes {
		id, err := q.UpsertClass(ctx, queries.UpsertClassParams{
			DocumentID: docIDs[c.Document], Slug: c.Slug, Name: c.Name, Description: c.Description,
			ParentSlug: optText(c.Parent), HitDie: optPositive(c.HitDie), CasterType: orDefault(c.CasterType, "none"),
		})
		if err != nil {
			return fmt.Errorf("compendium: class %s: %w", c.Slug, err)
		}
		if err := q.ClearClassChildren(ctx, id); err != nil {
			return err
		}
		for _, a := range c.SavingThrows {
			if err := q.AddClassSave(ctx, queries.AddClassSaveParams{ClassID: id, Ability: a}); err != nil {
				return err
			}
		}
		if err := importClassFeatures(ctx, q, id, c); err != nil {
			return err
		}
	}
	return nil
}

func importClassFeatures(ctx context.Context, q *queries.Queries, id int64, c snapshot.Class) error {
	for i, f := range c.Features {
		fid, err := q.AddClassFeature(ctx, queries.AddClassFeatureParams{
			ClassID: id, Slug: f.Slug, Name: f.Name, Description: f.Description, Ordering: int32(i), //nolint:gosec // bounded
		})
		if err != nil {
			return fmt.Errorf("compendium: class feature %s/%s: %w", c.Slug, f.Slug, err)
		}
		for _, lvl := range f.Levels {
			if err := q.AddClassFeatureLevel(ctx, queries.AddClassFeatureLevelParams{FeatureID: fid, Level: int32(lvl)}); err != nil { //nolint:gosec // 1..20
				return err
			}
		}
	}
	return nil
}

func importSpecies(ctx context.Context, q *queries.Queries, docIDs map[string]int64, species []snapshot.Species) error {
	for _, s := range species {
		id, err := q.UpsertSpecies(ctx, queries.UpsertSpeciesParams{
			DocumentID: docIDs[s.Document], Slug: s.Slug, Name: s.Name, Description: s.Description, Subspecies: s.Subspecies,
		})
		if err != nil {
			return fmt.Errorf("compendium: species %s: %w", s.Slug, err)
		}
		if err := q.ClearSpeciesTraits(ctx, id); err != nil {
			return err
		}
		for i, t := range s.Traits {
			if err := q.AddSpeciesTrait(ctx, queries.AddSpeciesTraitParams{SpeciesID: id, Ordering: int32(i), Name: t.Name, Description: t.Description}); err != nil { //nolint:gosec // bounded
				return err
			}
		}
	}
	return nil
}

func importBackgrounds(ctx context.Context, q *queries.Queries, docIDs map[string]int64, backgrounds []snapshot.Background) error {
	for _, b := range backgrounds {
		id, err := q.UpsertBackground(ctx, queries.UpsertBackgroundParams{DocumentID: docIDs[b.Document], Slug: b.Slug, Name: b.Name, Description: b.Description})
		if err != nil {
			return fmt.Errorf("compendium: background %s: %w", b.Slug, err)
		}
		if err := q.ClearBackgroundBenefits(ctx, id); err != nil {
			return err
		}
		for i, t := range b.Benefits {
			if err := q.AddBackgroundBenefit(ctx, queries.AddBackgroundBenefitParams{BackgroundID: id, Ordering: int32(i), Name: t.Name, Description: t.Description}); err != nil { //nolint:gosec // bounded
				return err
			}
		}
	}
	return nil
}

func importFeats(ctx context.Context, q *queries.Queries, docIDs map[string]int64, feats []snapshot.Feat) error {
	for _, f := range feats {
		id, err := q.UpsertFeat(ctx, queries.UpsertFeatParams{
			DocumentID: docIDs[f.Document], Slug: f.Slug, Name: f.Name, Description: f.Description, FeatType: orDefault(f.Type, "General"),
			Prerequisite: f.Prerequisite,
		})
		if err != nil {
			return fmt.Errorf("compendium: feat %s: %w", f.Slug, err)
		}
		if err := q.ClearFeatBenefits(ctx, id); err != nil {
			return err
		}
		for i, b := range f.Benefits {
			if err := q.AddFeatBenefit(ctx, queries.AddFeatBenefitParams{FeatID: id, Ordering: int32(i), Description: b}); err != nil { //nolint:gosec // bounded
				return err
			}
		}
	}
	return nil
}

func importWeapons(ctx context.Context, q *queries.Queries, l *lookupCache, docIDs map[string]int64, weapons []snapshot.Weapon) error {
	for _, w := range weapons {
		damageType := pgtype.Int8{}
		if w.DamageType != "" {
			id, err := l.damageType(ctx, w.DamageType)
			if err != nil {
				return err
			}
			damageType = pgtype.Int8{Int64: id, Valid: true}
		}
		id, err := q.UpsertWeapon(ctx, queries.UpsertWeaponParams{
			DocumentID: docIDs[w.Document], Slug: w.Slug, Name: w.Name, DamageDice: w.DamageDice, DamageTypeID: damageType,
			RangeFeet: int32(w.RangeFeet), LongRangeFeet: int32(w.LongRangeFeet), Simple: w.Simple, //nolint:gosec // ranges fit
		})
		if err != nil {
			return fmt.Errorf("compendium: weapon %s: %w", w.Slug, err)
		}
		if err := q.ClearWeaponProperties(ctx, id); err != nil {
			return err
		}
		for _, p := range w.Properties {
			if err := q.AddWeaponProperty(ctx, queries.AddWeaponPropertyParams{WeaponID: id, Name: p.Name, Mastery: p.Mastery, Detail: optText(p.Detail)}); err != nil {
				return err
			}
		}
	}
	return nil
}

func importArmor(ctx context.Context, q *queries.Queries, docIDs map[string]int64, armor []snapshot.Armor) error {
	for _, a := range armor {
		if err := q.UpsertArmor(ctx, queries.UpsertArmorParams{
			DocumentID: docIDs[a.Document], Slug: a.Slug, Name: a.Name, Category: a.Category, AcBase: int32(a.ACBase), //nolint:gosec // AC fits
			AddDex: a.AddDex, DexCap: optInt(a.DexCap), StealthDisadvantage: a.StealthDisadvantage, StrengthRequired: optInt(a.StrengthRequired),
		}); err != nil {
			return fmt.Errorf("compendium: armor %s: %w", a.Slug, err)
		}
	}
	return nil
}

func importItems(ctx context.Context, q *queries.Queries, docIDs map[string]int64, items []snapshot.Item) error {
	for _, it := range items {
		if err := q.UpsertItem(ctx, queries.UpsertItemParams{
			DocumentID: docIDs[it.Document], Slug: it.Slug, Name: it.Name, Description: it.Description, Category: orDefault(it.Category, "gear"),
			CostGp: it.CostGP, WeightLb: it.WeightLB, Magic: it.Magic, Rarity: optText(it.Rarity),
			RequiresAttunement: it.RequiresAttunement, AttunementDetail: optText(it.AttunementDetail),
		}); err != nil {
			return fmt.Errorf("compendium: item %s: %w", it.Slug, err)
		}
	}
	return nil
}

func importMonsters(ctx context.Context, q *queries.Queries, docIDs map[string]int64, monsters []snapshot.Monster) error {
	for _, m := range monsters {
		id, err := q.UpsertMonster(ctx, queries.UpsertMonsterParams{
			DocumentID: docIDs[m.Document], Slug: m.Slug, Name: m.Name, Size: m.Size, CreatureType: m.Type, Alignment: m.Alignment,
			ArmorClass: int32(m.ArmorClass), ArmorDetail: optText(m.ArmorDetail), HitPoints: int32(m.HitPoints), HitDice: m.HitDice, //nolint:gosec // statblock values fit
			ChallengeRating: m.ChallengeRating, Xp: int32(m.XP), Strength: int32(m.Abilities["strength"]), //nolint:gosec // statblock values fit
			Dexterity: int32(m.Abilities["dexterity"]), Constitution: int32(m.Abilities["constitution"]), //nolint:gosec // statblock values fit
			Intelligence: int32(m.Abilities["intelligence"]), Wisdom: int32(m.Abilities["wisdom"]), //nolint:gosec // statblock values fit
			Charisma: int32(m.Abilities["charisma"]), PassivePerception: int32(m.PassivePerception), Languages: optText(m.Languages), //nolint:gosec // statblock values fit
		})
		if err != nil {
			return fmt.Errorf("compendium: monster %s: %w", m.Slug, err)
		}
		if err := q.ClearMonsterChildren(ctx, id); err != nil {
			return err
		}
		if err := importMonsterChildren(ctx, q, id, m); err != nil {
			return fmt.Errorf("compendium: monster %s: %w", m.Slug, err)
		}
	}
	return nil
}

func importMonsterChildren(ctx context.Context, q *queries.Queries, id int64, m snapshot.Monster) error {
	for kind, values := range map[string]map[string]int{"save": m.Saves, "skill": m.Skills, "speed": m.Speeds, "sense": m.Senses} {
		for _, name := range sortedKeys(values) {
			if err := q.AddMonsterStat(ctx, queries.AddMonsterStatParams{MonsterID: id, Kind: kind, Name: name, Value: int32(values[name])}); err != nil { //nolint:gosec // bounded
				return err
			}
		}
	}
	for relation, targets := range map[string][]string{"resistance": m.Resistances, "immunity": m.Immunities, "vulnerability": m.Vulnerabilities, "condition-immunity": m.ConditionImmunities} {
		for _, t := range targets {
			if err := q.AddMonsterRelation(ctx, queries.AddMonsterRelationParams{MonsterID: id, Relation: relation, TargetSlug: t}); err != nil {
				return err
			}
		}
	}
	return importMonsterText(ctx, q, id, m)
}

func importMonsterText(ctx context.Context, q *queries.Queries, id int64, m snapshot.Monster) error {
	for i, t := range m.Traits {
		if err := q.AddMonsterTrait(ctx, queries.AddMonsterTraitParams{MonsterID: id, Ordering: int32(i), Name: t.Name, Description: t.Description}); err != nil { //nolint:gosec // bounded
			return err
		}
	}
	for i, a := range m.Actions {
		aid, err := q.AddMonsterAction(ctx, queries.AddMonsterActionParams{MonsterID: id, Ordering: int32(i), Name: a.Name, Description: a.Description, ActionType: orDefault(a.Type, "action")}) //nolint:gosec // bounded
		if err != nil {
			return err
		}
		if err := importAttacks(ctx, q, aid, a.Attacks); err != nil {
			return err
		}
	}
	return nil
}

func importAttacks(ctx context.Context, q *queries.Queries, actionID int64, attacks []snapshot.Attack) error {
	for j, at := range attacks {
		if err := q.AddMonsterAttack(ctx, queries.AddMonsterAttackParams{
			ActionID: actionID, Ordering: int32(j), Name: at.Name, Kind: orDefault(at.Kind, "weapon"), ToHit: int32(at.ToHit), //nolint:gosec // bounded
			ReachFeet: int32(at.ReachFeet), RangeFeet: int32(at.RangeFeet), LongRangeFeet: int32(at.LongRangeFeet), //nolint:gosec // bounded
			DamageDice: optText(at.DamageDice), DamageBonus: int32(at.DamageBonus), DamageType: optText(at.DamageType), //nolint:gosec // bounded
			ExtraDice: optText(at.ExtraDice), ExtraType: optText(at.ExtraType),
		}); err != nil {
			return err
		}
	}
	return nil
}

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func optPositive(v int) pgtype.Int4 {
	if v <= 0 {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(v), Valid: true} //nolint:gosec // bounded
}

func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
