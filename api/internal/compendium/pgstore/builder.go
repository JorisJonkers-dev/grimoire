package pgstore

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

var feetPattern = regexp.MustCompile(`(\d+) feet`)

// speedFeet reads a walking speed from trait text, defaulting to 30 feet.
func speedFeet(text string) int {
	if m := feetPattern.FindStringSubmatch(text); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 30
}

// listed splits "Insight and Religion" or "Insight, Religion" into slugs.
func listed(text string) []string {
	out := []string{}
	for _, part := range strings.Split(strings.ReplaceAll(text, " and ", ", "), ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, strings.ToLower(strings.ReplaceAll(p, " ", "-")))
		}
	}
	return out
}

// BuilderOptions lists what a character in the ruleset can choose.
func (s *Store) BuilderOptions(ctx context.Context, ruleset string) (compendium.BuilderOptions, error) {
	year, err := s.q.RulesetYear(ctx, ruleset)
	if errors.Is(err, pgx.ErrNoRows) {
		return compendium.BuilderOptions{}, compendium.ErrNotFound
	}
	if err != nil {
		return compendium.BuilderOptions{}, err
	}
	out := compendium.BuilderOptions{Ruleset: ruleset, RulesetYear: int(year)}
	steps := []func() error{
		func() error { return s.builderClasses(ctx, &out) },
		func() error { return s.builderSpecies(ctx, &out) },
		func() error { return s.builderBackgrounds(ctx, &out) },
		func() error { return s.builderArmor(ctx, &out) },
		func() error { return s.builderWeapons(ctx, &out) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return compendium.BuilderOptions{}, err
		}
	}
	return out, nil
}

func (s *Store) builderClasses(ctx context.Context, out *compendium.BuilderOptions) error {
	rows, err := s.q.BuilderClasses(ctx, out.Ruleset)
	if err != nil {
		return err
	}
	out.Classes = make([]compendium.ClassOption, 0, len(rows))
	for _, r := range rows {
		out.Classes = append(out.Classes, compendium.ClassOption{Slug: r.Slug, Name: r.Name, HitDie: int(r.HitDie), Saves: r.Saves})
	}
	return nil
}

func (s *Store) builderSpecies(ctx context.Context, out *compendium.BuilderOptions) error {
	rows, err := s.q.BuilderSpecies(ctx, out.Ruleset)
	if err != nil {
		return err
	}
	out.Species = make([]compendium.SpeciesOption, 0, len(rows))
	for _, r := range rows {
		out.Species = append(out.Species, compendium.SpeciesOption{Slug: r.Slug, Name: r.Name, SpeedFeet: speedFeet(r.Speed)})
	}
	return nil
}

func (s *Store) builderBackgrounds(ctx context.Context, out *compendium.BuilderOptions) error {
	rows, err := s.q.BuilderBackgrounds(ctx, out.Ruleset)
	if err != nil {
		return err
	}
	out.Backgrounds = make([]compendium.BackgroundOption, 0, len(rows))
	for _, r := range rows {
		out.Backgrounds = append(out.Backgrounds, compendium.BackgroundOption{
			Slug: r.Slug, Name: r.Name, Abilities: listed(r.Abilities), Skills: listed(r.Skills),
		})
	}
	return nil
}

func (s *Store) builderArmor(ctx context.Context, out *compendium.BuilderOptions) error {
	rows, err := s.q.BuilderArmor(ctx, out.Ruleset)
	if err != nil {
		return err
	}
	out.Armor = make([]compendium.ArmorOption, 0, len(rows))
	for _, r := range rows {
		a := compendium.ArmorOption{
			Slug: r.Slug, Name: r.Name, Category: r.Category, Shield: r.Slug == "shield", ACBase: int(r.AcBase), AddDex: r.AddDex,
			DexCap: -1, StrengthRequired: int(r.StrengthRequired.Int32), Stealth: r.StealthDisadvantage,
		}
		if r.DexCap.Valid {
			a.DexCap = int(r.DexCap.Int32)
		}
		out.Armor = append(out.Armor, a)
	}
	return nil
}

func (s *Store) builderWeapons(ctx context.Context, out *compendium.BuilderOptions) error {
	rows, err := s.q.BuilderWeapons(ctx, out.Ruleset)
	if err != nil {
		return err
	}
	out.Weapons = make([]compendium.WeaponOption, 0, len(rows))
	for _, r := range rows {
		out.Weapons = append(out.Weapons, compendium.WeaponOption{
			Slug: r.Slug, Name: r.Name, DamageDice: r.DamageDice, DamageType: r.DamageType, Simple: r.Simple,
			RangeFeet: int(r.RangeFeet), LongRangeFeet: int(r.LongRangeFeet), Properties: r.Properties,
		})
	}
	return nil
}

// Traits are a Character's class features up to its level, then its species traits.
func (s *Store) Traits(ctx context.Context, ruleset, class, species string, level int) ([]compendium.Trait, error) {
	features, err := s.q.SheetClassFeatures(ctx, queries.SheetClassFeaturesParams{Ruleset: ruleset, Class: class, Level: int32(level)}) //nolint:gosec // 1 to 20
	if err != nil {
		return nil, err
	}
	traits, err := s.q.SheetSpeciesTraits(ctx, queries.SheetSpeciesTraitsParams{Ruleset: ruleset, Species: species})
	if err != nil {
		return nil, err
	}
	out := make([]compendium.Trait, 0, len(features)+len(traits))
	for _, f := range features {
		out = append(out, compendium.Trait{Name: f.Name, Source: "class", Level: int(f.Level), Description: f.Description})
	}
	for _, t := range traits {
		out = append(out, compendium.Trait{Name: t.Name, Source: "species", Level: 0, Description: t.Description})
	}
	return out, nil
}
