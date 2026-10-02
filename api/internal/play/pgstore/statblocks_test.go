package pgstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/features"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

type armoury struct{}

func (armoury) Traits(context.Context, string, string, []compendium.ClassLevel, []string) ([]compendium.Trait, error) {
	return nil, nil
}

func (armoury) LevelUpOptions(context.Context, string, string, int) (compendium.LevelUpOptions, error) {
	return compendium.LevelUpOptions{}, nil
}

func (armoury) ClassSpells(context.Context, string, string, int) ([]compendium.SpellOption, error) {
	return nil, nil
}

func (armoury) AlwaysPrepared(context.Context, string, string, string, int) ([]compendium.SpellOption, error) {
	return nil, nil
}

func (armoury) Features(context.Context) (features.Catalog, error) { return features.Catalog{}, nil }

func (armoury) BuilderOptions(_ context.Context, ruleset string) (compendium.BuilderOptions, error) {
	return compendium.BuilderOptions{
		Ruleset: ruleset, RulesetYear: 2024,
		Classes: []compendium.ClassOption{
			{Slug: "fighter", Name: "Fighter", HitDie: 10, Saves: []string{"strength", "constitution"}},
			{Slug: "wizard", Name: "Wizard", HitDie: 6, Saves: []string{"intelligence", "wisdom"}},
		},
		Species:     []compendium.SpeciesOption{{Slug: "human", Name: "Human", SpeedFeet: 30}},
		Backgrounds: []compendium.BackgroundOption{{Slug: "soldier", Name: "Soldier", Abilities: []string{"strength", "dexterity", "constitution"}, Skills: []string{"athletics", "intimidation"}}},
		Weapons: []compendium.WeaponOption{
			{Slug: "rapier", Name: "Rapier", DamageDice: "1d8", DamageType: "piercing", Properties: []string{"Finesse", "Vex"}},
			{Slug: "shortbow", Name: "Shortbow", DamageDice: "1d6", DamageType: "piercing", RangeFeet: 80, LongRangeFeet: 320, Properties: []string{"Ammunition", "Vex"}},
			{Slug: "glaive", Name: "Glaive", DamageDice: "1d10", DamageType: "slashing", Properties: []string{"Reach", "Graze"}},
			{Slug: "blowgun", Name: "Blowgun", DamageDice: "1", DamageType: "piercing", RangeFeet: 25, LongRangeFeet: 100, Properties: []string{"Ammunition", "Vex"}},
		},
	}, nil
}

func TestStatblocksComeFromTheCompendiumAndCharacterSheets(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	monster := func(doc, slug string, ac int, attacks ...snapshot.Attack) snapshot.Monster {
		return snapshot.Monster{
			Entry: snapshot.Entry{Document: doc, Slug: slug, Name: "Ogre"}, Size: "large", Type: "giant", Alignment: "chaotic evil", ArmorClass: ac,
			HitPoints: 59, HitDice: "7d10", ChallengeRating: 2, XP: 450,
			Abilities: map[string]int{"strength": 19, "dexterity": 8, "constitution": 16, "intelligence": 5, "wisdom": 7, "charisma": 7},
			Saves:     map[string]int{"wisdom": 1}, Skills: map[string]int{}, Speeds: map[string]int{"walk": 40}, Senses: map[string]int{},
			Resistances: []string{}, Immunities: []string{}, Vulnerabilities: []string{}, ConditionImmunities: []string{}, Traits: []snapshot.Named{},
			Actions: []snapshot.Action{{Name: "Hits", Description: "Hits.", Type: "action", Attacks: attacks}},
		}
	}
	snap := snapshot.Snapshot{
		Documents: []snapshot.Document{
			{Key: "srd-2024", Title: "SRD 5.2", RulesetYear: 2024, Precedence: 20, License: "CC-BY-4.0", Attribution: "new", URL: "https://a"},
			{Key: "srd-2014", Title: "SRD 5.1", RulesetYear: 2014, Precedence: 10, License: "CC-BY-4.0", Attribution: "old", URL: "https://b"},
		},
		Monsters: []snapshot.Monster{
			monster("srd-2024", "ogre", 11,
				snapshot.Attack{Name: "Greatclub", Kind: "weapon", ToHit: 6, ReachFeet: 5, DamageDice: "2d8", DamageBonus: 4, DamageType: "bludgeoning", ExtraDice: "1d6", ExtraType: "fire"},
				snapshot.Attack{Name: "Javelin", Kind: "weapon", ToHit: 6, RangeFeet: 30, DamageDice: "2d6", DamageBonus: 4, DamageType: "piercing"},
				snapshot.Attack{Name: "Spit", Kind: "weapon", ToHit: 2, RangeFeet: 10, LongRangeFeet: 20},
			),
			monster("srd-2014", "ogre", 30),
			monster("srd-2014", "old-ogre", 12),
		},
	}
	snap.Monsters[0].Skills = map[string]int{"stealth": 3, "perception": 2}
	if _, err := comppg.New(tb.pool).Import(ctx, snap, "statblocks"); err != nil {
		t.Fatal(err)
	}
	chars := &campaignapp.Characters{Repo: campaignpg.New(tb.pool), Compendium: armoury{}, Combat: campaignapp.NoCombat{}, Now: time.Now}
	s := pgstore.Statblocks{Store: pgstore.New(tb.pool), Characters: chars}

	name, ogre, err := s.Monster(ctx, tb.campaign, "ogre")
	if err != nil || name != "Ogre" || ogre.AC != 11 || ogre.HP != 59 || ogre.HPMax != 59 || len(ogre.Attacks) != 3 || ogre.Saves["strength"] != 4 || ogre.Saves["wisdom"] != 1 || ogre.Intelligence != 5 {
		t.Fatalf("ogre = %s %+v %v", name, ogre, err)
	}
	want := []domain.Attack{
		{Name: "Greatclub", ToHit: 6, ReachFt: 5, Damage: "2d8+1d6", DamageBonus: 4, DamageType: "bludgeoning and fire"},
		{Name: "Javelin", ToHit: 6, RangeFt: 30, LongRangeFt: 30, Damage: "2d6", DamageBonus: 4, DamageType: "piercing"},
		{Name: "Spit", ToHit: 2, RangeFt: 10, LongRangeFt: 20},
	}
	for i, a := range want {
		if ogre.Attacks[i] != a {
			t.Errorf("attack %d = %+v, want %+v", i, ogre.Attacks[i], a)
		}
	}
	if ogre.Stealth != 3 || ogre.Perception != 2 || ogre.Initiative != -1 || ogre.SpeedFt != 40 {
		t.Fatalf("ogre ambush stats = %+v", ogre)
	}
	if _, old, err := s.Monster(ctx, tb.campaign, "old-ogre"); err != nil || old.AC != 12 || old.Source != "monster:old-ogre" || old.Stealth != -1 || old.Perception != -2 {
		t.Fatalf("a monster only in the other ruleset = %+v %v", old, err)
	}
	if _, _, err := s.Monster(ctx, tb.campaign, "dragon"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("unknown monster = %v", err)
	}
	if _, _, err := s.Monster(ctx, uuid.New(), "ogre"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("unknown campaign = %v", err)
	}

	sheet, err := chars.Create(ctx, player, campaigndomain.CampaignID(tb.campaign), campaigndomain.Build{
		Name: "Mira", Species: "human", Class: "fighter", Background: "soldier", Method: "standard-array",
		Base:   map[string]int{"strength": 10, "dexterity": 15, "constitution": 14, "intelligence": 8, "wisdom": 12, "charisma": 13},
		Bonus:  map[string]int{"dexterity": 2, "constitution": 1},
		Skills: []string{"perception", "survival"}, Weapons: []string{"rapier", "shortbow", "glaive", "blowgun"},
	})
	if err != nil {
		t.Fatal(err)
	}
	name, owner, mira, err := s.Character(ctx, dm, tb.campaign, uuid.UUID(sheet.ID))
	if err != nil || name != "Mira" || owner != tb.playerID || mira.AC != 13 || mira.HP != mira.HPMax || mira.HPMax != 12 {
		t.Fatalf("mira = %s %v %+v %v", name, owner, mira, err)
	}
	wantMira := []domain.Attack{
		{Name: "Unarmed Strike", ToHit: 2, ReachFt: 5, DamageBonus: 1, DamageType: "bludgeoning"},
		{Name: "Rapier", ToHit: 5, ReachFt: 5, Damage: "1d8", DamageBonus: 3, DamageType: "piercing", DamageMod: 3, Mastery: "vex"},
		{Name: "Shortbow", ToHit: 5, RangeFt: 80, LongRangeFt: 320, Damage: "1d6", DamageBonus: 3, DamageType: "piercing", DamageMod: 3, Mastery: "vex"},
		{Name: "Glaive", ToHit: 2, ReachFt: 10, Damage: "1d10", DamageType: "slashing", Mastery: "graze"},
		{Name: "Blowgun", ToHit: 5, RangeFt: 25, LongRangeFt: 100, DamageBonus: 4, DamageType: "piercing", DamageMod: 3},
	}
	for i, a := range wantMira {
		if mira.Attacks[i] != a {
			t.Errorf("mira's attack %d = %+v, want %+v", i, mira.Attacks[i], a)
		}
	}
	if mira.Stealth != 3 || mira.Perception != 3 || mira.Initiative != 3 || mira.SpeedFt != 30 {
		t.Fatalf("mira ambush stats = %+v", mira)
	}
	if mira.SpellDC != 0 || mira.Shield || mira.Saves["strength"] != 2 || mira.AttacksPerAction != 1 || mira.UnarmedDC != 10 {
		t.Fatalf("a level 1 fighter casts nothing and attacks once = %+v", mira)
	}
	if _, err := tb.pool.Exec(ctx, "UPDATE campaign.characters SET level = 5 WHERE id = $1", uuid.UUID(sheet.ID)); err != nil {
		t.Fatal(err)
	}
	if _, _, five, err := s.Character(ctx, dm, tb.campaign, uuid.UUID(sheet.ID)); err != nil || five.AttacksPerAction != 2 {
		t.Fatalf("Extra Attack at level 5 = %+v %v", five.AttacksPerAction, err)
	}
	nim, err := chars.Create(ctx, player, campaigndomain.CampaignID(tb.campaign), campaigndomain.Build{
		Name: "Nim", Species: "human", Class: "wizard", Background: "soldier", Method: "standard-array",
		Base:   map[string]int{"strength": 8, "dexterity": 14, "constitution": 13, "intelligence": 15, "wisdom": 12, "charisma": 10},
		Bonus:  map[string]int{"dexterity": 1, "constitution": 2},
		Skills: []string{"arcana", "history"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, st, err := s.Character(ctx, dm, tb.campaign, uuid.UUID(nim.ID)); err != nil || st.SpellDC != 12 || !st.Shield || st.Saves["intelligence"] != 4 {
		t.Fatalf("a wizard = %+v %v", st, err)
	}
	if _, _, _, err := s.Character(ctx, dm, tb.campaign, uuid.New()); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("unknown character = %v", err)
	}
	pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
		_, _, err := pgstore.Statblocks{Store: pgstore.NewFaulty(tb.pool, f), Characters: chars}.Monster(ctx, tb.campaign, "ogre")
		if err != nil && !errors.Is(err, pgtest.ErrInjected) {
			t.Fatalf("monster: %v", err)
		}
		return err
	})
}
