package pgstore_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/storage"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/features"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

type fakeOptions struct{ err error }

func (f fakeOptions) Traits(context.Context, string, string, []compendium.ClassLevel, []string) ([]compendium.Trait, error) {
	return nil, f.err
}

func (f fakeOptions) LevelUpOptions(context.Context, string, string, int) (compendium.LevelUpOptions, error) {
	return compendium.LevelUpOptions{}, f.err
}

func (f fakeOptions) ClassSpells(context.Context, string, string, int) ([]compendium.SpellOption, error) {
	out := []compendium.SpellOption{{Slug: "alarm", Name: "Alarm", Level: 1, Ritual: true, CastingTime: "1minute"}}
	for _, slug := range []string{"s1", "s2", "s3", "s4", "s5", "s6"} {
		out = append(out, compendium.SpellOption{Slug: slug, Name: slug, Level: 1, Ritual: false, CastingTime: "action"})
	}
	return out, f.err
}

func (f fakeOptions) AlwaysPrepared(context.Context, string, string, string, int) ([]compendium.SpellOption, error) {
	return nil, f.err
}

func (f fakeOptions) Features(context.Context) (features.Catalog, error) {
	return features.Catalog{}, f.err
}

func (f fakeOptions) BuilderOptions(_ context.Context, ruleset string) (compendium.BuilderOptions, error) {
	year, abilities := 2024, []string{"strength", "dexterity", "constitution"}
	if ruleset == "srd-2014" {
		year, abilities = 2014, []string{}
	}
	return compendium.BuilderOptions{
		Ruleset: ruleset, RulesetYear: year,
		Classes: []compendium.ClassOption{
			{Slug: "fighter", Name: "Fighter", HitDie: 10, Saves: []string{"strength", "constitution"}},
			{Slug: "wizard", Name: "Wizard", HitDie: 6, Saves: []string{"intelligence", "wisdom"}},
		},
		Species:     []compendium.SpeciesOption{{Slug: "human", Name: "Human", SpeedFeet: 30}},
		Backgrounds: []compendium.BackgroundOption{{Slug: "soldier", Name: "Soldier", Abilities: abilities, Skills: []string{"athletics", "intimidation"}}},
		Armor: []compendium.ArmorOption{
			{Slug: "chain-mail", Name: "Chain Mail", Category: "heavy", ACBase: 16, DexCap: -1, StrengthRequired: 13, Stealth: true},
			{Slug: "leather-armor", Name: "Leather Armor", Category: "light", ACBase: 11, AddDex: true, DexCap: -1},
			{Slug: "shield", Name: "Shield", Category: "shield", Shield: true, ACBase: 2, DexCap: -1},
		},
		Weapons: []compendium.WeaponOption{
			{Slug: "longsword", Name: "Longsword", DamageDice: "1d8", DamageType: "slashing"},
			{Slug: "longbow", Name: "Longbow", DamageDice: "1d8", DamageType: "piercing", RangeFeet: 150, LongRangeFeet: 600},
		},
	}, f.err
}

type fakeCombat struct {
	in     bool
	err    error
	failOn int
	calls  int
}

func (f *fakeCombat) InCombat(context.Context, domain.CharacterID) (bool, error) {
	f.calls++
	if f.failOn != 0 && f.calls == f.failOn {
		return f.in, errors.New("combat lookup failed")
	}
	return f.in, f.err
}

func fighter() domain.Build {
	return domain.Build{
		Name: " Kara ", Species: "human", Class: "fighter", Background: "soldier", Method: "standard-array",
		Base:   map[string]int{"strength": 15, "dexterity": 13, "constitution": 14, "intelligence": 8, "wisdom": 12, "charisma": 10},
		Bonus:  map[string]int{"strength": 2, "constitution": 1},
		Skills: []string{"perception", "survival"}, Armor: "chain-mail", Shield: true, Weapons: []string{"longsword", "longbow"},
	}
}

func party(t *testing.T, repo app.Repository) (*app.Characters, *fakeCombat, domain.Detail) {
	t.Helper()
	s, _ := service(t, repo)
	d := table(t, s)
	combat := &fakeCombat{}
	return &app.Characters{
		Repo: repo, Compendium: fakeOptions{}, Combat: combat, Blobs: storage.Dir{Path: t.TempDir()},
		Now: func() time.Time { return time.Unix(1_800_000_000, 0) },
	}, combat, d
}

func TestCreateDerivesTheSheet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	chars, _, d := party(t, pgstore.New(open(t).Pool()))
	sheet, err := chars.Create(ctx, playerCaller, d.ID, fighter())
	if err != nil {
		t.Fatal(err)
	}
	if sheet.Name != "Kara" || sheet.HPMax != 12 || sheet.HPCurrent != 12 || sheet.Derived.ArmorClass != 18 || sheet.Derived.SpeedFeet != 30 ||
		sheet.Scores["strength"] != 17 || !sheet.Mine || !sheet.Editable || sheet.ClassName != "Fighter" {
		t.Fatalf("sheet = %+v", sheet)
	}
	got, err := chars.Get(ctx, dmCaller, d.ID, sheet.ID)
	if err != nil || got.Mine || !got.Editable || got.HPMax != 12 || len(got.Weapons) != 2 || got.Armor.Slug != "chain-mail" ||
		len(got.BackgroundSkills) != 2 || len(got.Skills) != 2 || got.Bonus["constitution"] != 1 || len(got.Derived.Warnings) != 1 {
		t.Fatalf("stored sheet = %+v %v", got, err)
	}
	list, err := chars.List(ctx, playerCaller, d.ID)
	if err != nil || len(list) != 1 || !list[0].Mine || list[0].OwnerName != "Tamsin" || list[0].Class != "fighter" {
		t.Fatalf("list = %+v %v", list, err)
	}
	preview, err := chars.Preview(ctx, dmCaller, d.ID, fighter())
	if err != nil || preview.ID != (domain.CharacterID{}) {
		t.Fatalf("preview = %+v %v", preview, err)
	}
	if again, _ := chars.List(ctx, dmCaller, d.ID); len(again) != 1 {
		t.Fatal("preview saved a character")
	}
}

func TestBuildsAreValidated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	chars, _, d := party(t, pgstore.New(open(t).Pool()))
	cases := map[string]func(b *domain.Build){
		"class":       func(b *domain.Build) { b.Class = "artificer" },
		"species":     func(b *domain.Build) { b.Species = "kender" },
		"background":  func(b *domain.Build) { b.Background = "pirate" },
		"array":       func(b *domain.Build) { b.Base["charisma"] = 15 },
		"bonus":       func(b *domain.Build) { b.Bonus = map[string]int{"wisdom": 2, "strength": 1} },
		"skills":      func(b *domain.Build) { b.Skills = []string{"athletics", "perception"} },
		"armor":       func(b *domain.Build) { b.Armor = "shield" },
		"weapon":      func(b *domain.Build) { b.Weapons = []string{"longsword", "longsword"} },
		"many":        func(b *domain.Build) { b.Weapons = []string{"a", "b", "c", "d", "e"} },
		"unknown gun": func(b *domain.Build) { b.Weapons = []string{"blunderbuss"} },
		"name":        func(b *domain.Build) { b.Name = "  " },
	}
	for name, mutate := range cases {
		b := fighter()
		mutate(&b)
		if _, err := chars.Create(ctx, playerCaller, d.ID, b); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	var rule *app.RuleError
	b := fighter()
	b.Class = "wizard"
	if _, err := chars.Preview(ctx, playerCaller, d.ID, b); err != nil {
		t.Fatalf("wizard: %v", err)
	}
	b.Skills = []string{"arcana"}
	if _, err := chars.Preview(ctx, playerCaller, d.ID, b); !errors.As(err, &rule) || rule.Error() != "choose 2 class skills" {
		t.Fatalf("rule error = %v", err)
	}
	if _, err := chars.Create(ctx, stranger, d.ID, fighter()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("stranger: %v", err)
	}
}

func TestShieldNeedsTheRuleset(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	chars, _, d := party(t, pgstore.New(open(t).Pool()))
	chars.Compendium = noShield{}
	b := fighter()
	if _, err := chars.Preview(ctx, playerCaller, d.ID, b); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("shield without a shield: %v", err)
	}
	b.Shield, b.Armor = false, ""
	sheet, err := chars.Preview(ctx, playerCaller, d.ID, b)
	if err != nil || sheet.Armor != nil || sheet.Derived.ArmorClass != 11 {
		t.Fatalf("unarmoured = %+v %v", sheet.Derived, err)
	}
}

type noShield struct{ fakeOptions }

func (noShield) BuilderOptions(ctx context.Context, ruleset string) (compendium.BuilderOptions, error) {
	o, err := fakeOptions{}.BuilderOptions(ctx, ruleset)
	o.Armor = o.Armor[:2]
	return o, err
}

func TestCampaignsPlaySRD52Only(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := pgstore.New(open(t).Pool())
	s, _ := service(t, repo)
	if _, err := s.Create(ctx, dmCaller, app.CreateInput{Name: "Old school", Ruleset: "srd-2014", DisplayName: "DM"}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("2014 campaign created: %v", err)
	}
	d, err := s.Create(ctx, dmCaller, app.CreateInput{Name: "New school", DisplayName: "DM"})
	if err != nil || d.Ruleset != "srd-2024" {
		t.Fatalf("default ruleset = %+v %v", d, err)
	}
	old := "srd-2014"
	if _, err := s.Update(ctx, dmCaller, d.ID, app.UpdateInput{Ruleset: &old}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("switched to 2014: %v", err)
	}
	if _, err := repo.CreateCampaign(ctx, "Sneaky", "srd-2014", "dm-subject", time.Now()); err == nil {
		t.Fatal("the database accepted a 2014 campaign")
	}
}

// A Character whose build had no 2024 counterpart kept its 5.1 ruleset in the migration; it still
// reads, with the 5.1 rule that ability increases may go anywhere.
func TestLegacyCharactersStillRead(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := open(t)
	repo := pgstore.New(store.Pool())
	s, _ := service(t, repo)
	d, err := s.Create(ctx, dmCaller, app.CreateInput{Name: "Old friends", DisplayName: "DM"})
	if err != nil {
		t.Fatal(err)
	}
	chars := &app.Characters{Repo: repo, Compendium: fakeOptions{}, Combat: app.NoCombat{}, Now: time.Now}
	sheet, err := chars.Create(ctx, dmCaller, d.ID, fighter())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Pool().Exec(ctx, `UPDATE campaign.characters SET ruleset = 'srd-2014' WHERE id = $1`, uuid.UUID(sheet.ID)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Pool().Exec(ctx, `UPDATE campaign.character_abilities SET bonus = CASE ability WHEN 'wisdom' THEN 1 WHEN 'intelligence' THEN 1 WHEN 'charisma' THEN 1 ELSE 0 END WHERE character_id = $1`, uuid.UUID(sheet.ID)); err != nil {
		t.Fatal(err)
	}
	got, err := chars.Get(ctx, dmCaller, d.ID, sheet.ID)
	if err != nil || got.Ruleset != "srd-2014" || got.Scores["wisdom"] != 13 {
		t.Fatalf("legacy sheet = %+v %v", got, err)
	}
}

func TestEditsRespectOwnershipAndCombat(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	chars, combat, d := party(t, pgstore.New(open(t).Pool()))
	sheet, _ := chars.Create(ctx, playerCaller, d.ID, fighter())
	name, hp, armor, shield := "Kara the Bold", 5, "leather-armor", false
	got, err := chars.Update(ctx, playerCaller, d.ID, sheet.ID, app.Edit{Name: &name, HPCurrent: &hp, Armor: &armor, Shield: &shield, Weapons: []string{"longbow"}})
	if err != nil || got.Name != name || got.HPCurrent != 5 || got.Armor.Slug != "leather-armor" || got.Shield || len(got.Weapons) != 1 {
		t.Fatalf("edited = %+v %v", got, err)
	}
	if _, err := chars.Update(ctx, dmCaller, d.ID, sheet.ID, app.Edit{}); err != nil {
		t.Fatalf("DM edit: %v", err)
	}
	over := 99
	if _, err := chars.Update(ctx, playerCaller, d.ID, sheet.ID, app.Edit{HPCurrent: &over}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("hp over max: %v", err)
	}
	blank, bad := " ", "plate"
	if _, err := chars.Update(ctx, playerCaller, d.ID, sheet.ID, app.Edit{Name: &blank}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("blank name: %v", err)
	}
	if _, err := chars.Update(ctx, playerCaller, d.ID, sheet.ID, app.Edit{Armor: &bad}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("unknown armour: %v", err)
	}
	other, _ := chars.Create(ctx, dmCaller, d.ID, fighter())
	if _, err := chars.Update(ctx, playerCaller, d.ID, other.ID, app.Edit{Name: &name}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("player edits DM's character: %v", err)
	}
	if err := chars.Delete(ctx, playerCaller, d.ID, other.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("player deletes DM's character: %v", err)
	}
	combat.in = true
	if locked, _ := chars.Get(ctx, playerCaller, d.ID, sheet.ID); locked.Editable {
		t.Fatal("sheet editable in combat")
	}
	if _, err := chars.Update(ctx, playerCaller, d.ID, sheet.ID, app.Edit{Name: &name}); !errors.Is(err, domain.ErrLocked) {
		t.Fatalf("edit in combat: %v", err)
	}
	if err := chars.Delete(ctx, dmCaller, d.ID, sheet.ID); !errors.Is(err, domain.ErrLocked) {
		t.Fatalf("delete in combat: %v", err)
	}
	combat.in = false
	if err := chars.Delete(ctx, playerCaller, d.ID, sheet.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := chars.Get(ctx, playerCaller, d.ID, sheet.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted character: %v", err)
	}
	if _, err := chars.List(ctx, stranger, d.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("stranger list: %v", err)
	}
	if _, err := chars.Get(ctx, stranger, d.ID, other.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("stranger get: %v", err)
	}
}

func TestCharacterPortFailuresSurface(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	chars, combat, d := party(t, pgstore.New(open(t).Pool()))
	sheet, _ := chars.Create(ctx, playerCaller, d.ID, fighter())
	boom := errExtras
	combat.err = boom
	if _, err := chars.Get(ctx, playerCaller, d.ID, sheet.ID); !errors.Is(err, boom) {
		t.Fatalf("combat status error: %v", err)
	}
	combat.err = nil
	wb := fighter()
	wb.Class = "wizard"
	wiz, err := chars.Create(ctx, playerCaller, d.ID, wb)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []app.Compendium{&spellsFail{failAt: 1}, &spellsFail{failAt: 2}, &spellsFail{failAt: 3, always: true}} {
		chars.Compendium = c
		if _, err := chars.Spells(ctx, playerCaller, d.ID, wiz.ID); err == nil {
			if _, err := chars.CopySpell(ctx, playerCaller, d.ID, wiz.ID, "s1"); !errors.Is(err, boom) {
				t.Fatalf("spell list error on copy: %v", err)
			}
		} else if !errors.Is(err, boom) {
			t.Fatalf("spell list error: %v", err)
		}
	}
	chars.Compendium = levelUpFails{}
	if _, err := chars.PlanLevelUp(ctx, playerCaller, d.ID, sheet.ID, ""); !errors.Is(err, boom) {
		t.Fatalf("level-up options error: %v", err)
	}
	chars.Compendium = fakeOptions{}
	var rule *app.RuleError
	for _, class := range []string{"bard", "wizard"} {
		if _, err := chars.PlanLevelUp(ctx, playerCaller, d.ID, sheet.ID, class); !errors.As(err, &rule) {
			t.Fatalf("level up into %s: %v", class, err)
		}
	}
	if err := chars.Repo.LevelUp(ctx, domain.LevelUp{CampaignID: d.ID, ID: sheet.ID, From: 1}, time.Now()); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("a locked level taken: %v", err)
	}
	chars.Compendium = fakeOptions{err: boom}
	if _, err := chars.Get(ctx, playerCaller, d.ID, sheet.ID); !errors.Is(err, boom) {
		t.Fatalf("compendium error on get: %v", err)
	}
	if _, err := chars.Preview(ctx, playerCaller, d.ID, fighter()); !errors.Is(err, boom) {
		t.Fatalf("compendium error on preview: %v", err)
	}
	for _, c := range []app.Compendium{extrasFail{catalog: true}, extrasFail{catalog: false}} {
		chars.Compendium = c
		if _, err := chars.Get(ctx, playerCaller, d.ID, sheet.ID); !errors.Is(err, boom) {
			t.Fatalf("features or traits error on get: %v", err)
		}
	}
	chars.Compendium = noShield{}
	if _, err := chars.Get(ctx, playerCaller, d.ID, sheet.ID); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("stored build the ruleset no longer allows: %v", err)
	}
	chars.Compendium = &secondFails{}
	if _, err := chars.Update(ctx, playerCaller, d.ID, sheet.ID, app.Edit{}); !errors.Is(err, errSecond) {
		t.Fatalf("compendium error on update: %v", err)
	}
	chars.Compendium = fakeOptions{}
	combat.in, combat.calls, combat.failOn = true, 0, 2
	if _, err := chars.Update(ctx, playerCaller, d.ID, sheet.ID, app.Edit{}); err == nil {
		t.Fatal("second combat check error ignored")
	}
}

// spellsFail reads a class's spells until a call number, then fails it; always fails the always-prepared
// spells instead.
type spellsFail struct {
	fakeOptions
	calls, failAt int
	always        bool
}

func (f *spellsFail) ClassSpells(ctx context.Context, ruleset, class string, level int) ([]compendium.SpellOption, error) {
	f.calls++
	if !f.always && f.calls >= f.failAt {
		return nil, errExtras
	}
	return f.fakeOptions.ClassSpells(ctx, ruleset, class, level)
}

func (f *spellsFail) AlwaysPrepared(context.Context, string, string, string, int) ([]compendium.SpellOption, error) {
	if f.always {
		return nil, errExtras
	}
	return nil, nil
}

// levelUpFails builds sheets but cannot read what a level offers.
type levelUpFails struct{ fakeOptions }

func (levelUpFails) LevelUpOptions(context.Context, string, string, int) (compendium.LevelUpOptions, error) {
	return compendium.LevelUpOptions{}, errExtras
}

// extrasFail builds sheets but fails reading the feature catalogue, or the traits.
type extrasFail struct {
	fakeOptions
	catalog bool
}

func (f extrasFail) Features(context.Context) (features.Catalog, error) {
	if f.catalog {
		return features.Catalog{}, errExtras
	}
	return features.Catalog{}, nil
}

func (f extrasFail) Traits(context.Context, string, string, []compendium.ClassLevel, []string) ([]compendium.Trait, error) {
	return nil, errExtras
}

var (
	errSecond = errors.New("second lookup failed")
	errExtras = errors.New("boom")
)

// secondFails answers the first lookup and fails the next, as a compendium outage mid-edit would.
type secondFails struct {
	fakeOptions
	calls int
}

func (f *secondFails) BuilderOptions(ctx context.Context, ruleset string) (compendium.BuilderOptions, error) {
	f.calls++
	if f.calls > 1 {
		return compendium.BuilderOptions{}, errSecond
	}
	return fakeOptions{}.BuilderOptions(ctx, ruleset)
}

func TestEveryCharacterDatabaseFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := open(t)
	chars, _, d := party(t, pgstore.New(db.Pool()))
	sheet, _ := chars.Create(ctx, playerCaller, d.ID, fighter())
	doomed, _ := chars.Create(ctx, playerCaller, d.ID, fighter())
	climber, _ := chars.Create(ctx, playerCaller, d.ID, fighter())
	scholar, _ := chars.Create(ctx, playerCaller, d.ID, fighter())
	store := pgstore.New(db.Pool())
	for _, id := range []domain.CharacterID{climber.ID, scholar.ID} {
		if err := store.SetLevelUpReady(ctx, d.ID, id, true, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	full := domain.LevelUp{
		CampaignID: d.ID, ID: scholar.ID, From: 1, Gain: 4,
		Classes:  []domain.ClassLevel{{Class: "fighter", Subclass: "", Level: 1}, {Class: "wizard", Subclass: "evoker", Level: 1}},
		Picks:    []domain.Pick{{Level: 2, Choice: "feat", Value: "alert"}},
		Spells:   []domain.LearnedSpell{{Class: "wizard", Spell: "light", Level: 2}},
		Increase: map[string]int{"strength": 2},
	}
	ready := true
	fb := fighter()
	rebuild := app.RetrainInput{Species: fb.Species, Background: fb.Background, Method: fb.Method, Base: fb.Base, Bonus: fb.Bonus, Skills: fb.Skills, Picks: nil, Increase: nil, Reason: "again"}
	clearRetrains := func() {
		if _, err := db.Pool().Exec(ctx, "DELETE FROM campaign.retrains WHERE status = 'pending'"); err != nil {
			t.Fatal(err)
		}
	}
	pending := func() uuid.UUID {
		clearRetrains()
		r, err := chars.RequestRetrain(ctx, playerCaller, d.ID, sheet.ID, rebuild)
		if err != nil {
			t.Fatal(err)
		}
		return r.ID
	}
	wb := fighter()
	wb.Class = "wizard"
	wiz, err := chars.Create(ctx, playerCaller, d.ID, wb)
	if err != nil {
		t.Fatal(err)
	}
	book := []domain.LearnedSpell{}
	for _, slug := range []string{"alarm", "s1", "s2", "s3", "s4", "s5"} {
		book = append(book, domain.LearnedSpell{Class: "wizard", Spell: slug, Level: 1, Prepared: false, Spellbook: true})
	}
	if err := store.ReplaceClassSpells(ctx, wiz.ID, "wizard", book, true); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool().Exec(ctx, `INSERT INTO campaign.containers (id, campaign_id, kind, character_id, label, created_at)
		VALUES (gen_random_uuid(), $1, 'character', $2, 'Pack', now())`, uuid.UUID(d.ID), uuid.UUID(wiz.ID)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool().Exec(ctx, `INSERT INTO campaign.container_coins (container_id, coin, amount)
		SELECT id, 'gp', 100 FROM campaign.containers WHERE character_id = $1`, uuid.UUID(wiz.ID)); err != nil {
		t.Fatal(err)
	}
	if err := chars.SetImage(ctx, playerCaller, d.ID, sheet.ID, domain.Portrait, png); err != nil {
		t.Fatal(err)
	}
	name := "Renamed"
	svc, _ := service(t, pgstore.New(db.Pool()))
	svc.Token = app.RandomToken
	other := table(t, svc)
	ops := map[string]func(c *app.Characters) error{
		"mine":  func(c *app.Characters) error { _, err := c.Mine(ctx, playerCaller); return err },
		"owned": func(c *app.Characters) error { _, err := c.Owned(ctx, playerCaller, sheet.Owned); return err },
		"update owned": func(c *app.Characters) error {
			_, err := c.UpdateOwned(ctx, playerCaller, sheet.Owned, "Kara Vale", "")
			return err
		},
		"join": func(c *app.Characters) error { _, err := c.Join(ctx, playerCaller, sheet.Owned, other.ID); return err },
		"draft": func(c *app.Characters) error {
			_, err := c.SaveDraft(ctx, playerCaller, d.ID, 2, []byte(`{}`))
			return err
		},
		"read draft": func(c *app.Characters) error {
			_, err := c.Draft(ctx, playerCaller, d.ID)
			if errors.Is(err, domain.ErrNotFound) {
				return nil
			}
			return err
		},
		"discard": func(c *app.Characters) error { return c.DiscardDraft(ctx, playerCaller, d.ID) },
		"roll": func(c *app.Characters) error {
			_, err := c.RollScores(ctx, playerCaller, d.ID)
			if errors.Is(err, domain.ErrConflict) {
				return nil
			}
			return err
		},
		"create":  func(c *app.Characters) error { _, err := c.Create(ctx, playerCaller, d.ID, fighter()); return err },
		"preview": func(c *app.Characters) error { _, err := c.Preview(ctx, playerCaller, d.ID, fighter()); return err },
		"list":    func(c *app.Characters) error { _, err := c.List(ctx, playerCaller, d.ID); return err },
		"get":     func(c *app.Characters) error { _, err := c.Get(ctx, playerCaller, d.ID, sheet.ID); return err },
		"update": func(c *app.Characters) error {
			_, err := c.Update(ctx, playerCaller, d.ID, sheet.ID, app.Edit{Name: &name})
			return err
		},
		"delete": func(c *app.Characters) error { return c.Delete(ctx, playerCaller, d.ID, doomed.ID) },
		"plan": func(c *app.Characters) error {
			_, err := c.PlanLevelUp(ctx, playerCaller, d.ID, climber.ID, "")
			return err
		},
		"level up": func(c *app.Characters) error {
			if err := store.SetLevelUpReady(ctx, d.ID, climber.ID, true, time.Now()); err != nil {
				t.Fatal(err)
			}
			_, err := c.LevelUp(ctx, playerCaller, d.ID, climber.ID, app.LevelUpRequest{Class: "fighter", Roll: false, Picks: nil, Increase: nil, Spells: nil})
			return err
		},
		"store level up": func(c *app.Characters) error {
			now, err := store.Character(ctx, d.ID, scholar.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.SetLevelUpReady(ctx, d.ID, scholar.ID, true, time.Now()); err != nil {
				t.Fatal(err)
			}
			full.From = now.Level
			return c.Repo.InTx(ctx, func(r app.Repository) error { return r.LevelUp(ctx, full, time.Now()) })
		},
		"spells": func(c *app.Characters) error { _, err := c.Spells(ctx, playerCaller, d.ID, wiz.ID); return err },
		"prepare": func(c *app.Characters) error {
			if _, err := db.Pool().Exec(ctx, "UPDATE campaign.characters SET can_prepare = true WHERE id = $1", uuid.UUID(wiz.ID)); err != nil {
				t.Fatal(err)
			}
			_, err := c.Prepare(ctx, playerCaller, d.ID, wiz.ID, "wizard", []string{"s1", "s2"})
			return err
		},
		"ritual": func(c *app.Characters) error {
			_, err := c.CastRitual(ctx, playerCaller, d.ID, wiz.ID, "alarm")
			return err
		},
		"copy": func(c *app.Characters) error {
			if _, err := db.Pool().Exec(ctx, "DELETE FROM campaign.character_spells WHERE character_id = $1 AND spell_slug = 's6'", uuid.UUID(wiz.ID)); err != nil {
				t.Fatal(err)
			}
			for _, q := range []string{
				"DELETE FROM campaign.container_coins k USING campaign.containers c WHERE k.container_id = c.id AND c.character_id = $1",
				"INSERT INTO campaign.container_coins (container_id, coin, amount) SELECT id, 'gp', 100 FROM campaign.containers WHERE character_id = $1",
			} {
				if _, err := db.Pool().Exec(ctx, q, uuid.UUID(wiz.ID)); err != nil {
					t.Fatal(err)
				}
			}
			_, err := c.CopySpell(ctx, playerCaller, d.ID, wiz.ID, "s6")
			return err
		},
		"grant": func(c *app.Characters) error {
			_, err := c.Update(ctx, dmCaller, d.ID, sheet.ID, app.Edit{HeroicInspiration: &ready})
			return err
		},
		"pass": func(c *app.Characters) error {
			for id, v := range map[domain.CharacterID]bool{sheet.ID: true, climber.ID: false} {
				if err := store.SetHeroicInspiration(ctx, d.ID, id, v); err != nil {
					t.Fatal(err)
				}
			}
			_, err := c.PassInspiration(ctx, playerCaller, d.ID, sheet.ID, climber.ID)
			return err
		},
		"request retrain": func(c *app.Characters) error {
			clearRetrains()
			_, err := c.RequestRetrain(ctx, playerCaller, d.ID, sheet.ID, rebuild)
			return err
		},
		"approve retrain": func(c *app.Characters) error {
			_, err := c.DecideRetrain(ctx, dmCaller, d.ID, pending(), true)
			return err
		},
		"decline retrain": func(c *app.Characters) error {
			_, err := c.DecideRetrain(ctx, dmCaller, d.ID, pending(), false)
			return err
		},
		"retrains": func(c *app.Characters) error { _, err := c.Retrains(ctx, playerCaller, d.ID, sheet.ID); return err },
		"choices": func(c *app.Characters) error {
			_, err := c.RetrainChoices(ctx, playerCaller, d.ID, sheet.ID)
			return err
		},
		"revisions": func(c *app.Characters) error {
			_, err := c.CharacterRevisions(ctx, playerCaller, d.ID, sheet.ID)
			return err
		},
		"unlock": func(c *app.Characters) error {
			_, err := c.Update(ctx, dmCaller, d.ID, sheet.ID, app.Edit{LevelUpReady: &ready})
			return err
		},
		"portrait": func(c *app.Characters) error {
			return c.SetImage(ctx, playerCaller, d.ID, sheet.ID, domain.Portrait, png)
		},
		"clear": func(c *app.Characters) error { return c.ClearToken(ctx, playerCaller, d.ID, sheet.ID) },
		"image": func(c *app.Characters) error {
			_, _, err := c.Image(ctx, playerCaller, d.ID, sheet.ID, domain.Portrait)
			return err
		},
	}
	for name, op := range ops {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			c := &app.Characters{Repo: pgstore.NewFaulty(db.Pool(), f), Compendium: fakeOptions{}, Combat: app.NoCombat{}, Blobs: chars.Blobs, Now: time.Now, Roll: func() []int { return []int{16, 15, 12, 11, 9, 8} }}
			err := op(c)
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}

var (
	png  = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)
	jpeg = append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 32)...)
	webp = append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), make([]byte, 32)...)
)

type brokenBlobs struct{}

func (brokenBlobs) Put(context.Context, string, string, []byte) error {
	return errors.New("bucket gone")
}

func (brokenBlobs) Get(context.Context, string) ([]byte, error) {
	return nil, errors.New("bucket gone")
}

func TestPortraitsAndTokens(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	chars, _, d := party(t, pgstore.New(open(t).Pool()))
	sheet, _ := chars.Create(ctx, playerCaller, d.ID, fighter())
	if _, _, err := chars.Image(ctx, playerCaller, d.ID, sheet.ID, domain.Portrait); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("no portrait yet: %v", err)
	}
	for kind, data := range map[domain.ImageKind][]byte{domain.Portrait: jpeg, domain.TokenIcon: webp} {
		if err := chars.SetImage(ctx, playerCaller, d.ID, sheet.ID, kind, data); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
	}
	img, data, err := chars.Image(ctx, dmCaller, d.ID, sheet.ID, domain.TokenIcon)
	if err != nil || img.Type != "image/webp" || len(data) != len(webp) {
		t.Fatalf("token = %+v %d %v", img, len(data), err)
	}
	got, _ := chars.Get(ctx, playerCaller, d.ID, sheet.ID)
	if got.Portrait == nil || got.Portrait.Type != "image/jpeg" || got.Token == nil {
		t.Fatalf("sheet images = %+v %+v", got.Portrait, got.Token)
	}
	if list, _ := chars.List(ctx, playerCaller, d.ID); list[0].TokenKey == "" {
		t.Fatal("party list lacks the token")
	}
	if err := chars.ClearToken(ctx, playerCaller, d.ID, sheet.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := chars.Image(ctx, playerCaller, d.ID, sheet.ID, domain.TokenIcon); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cleared token: %v", err)
	}
}

func TestPicturesAreRefused(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	chars, combat, d := party(t, pgstore.New(open(t).Pool()))
	sheet, _ := chars.Create(ctx, playerCaller, d.ID, fighter())
	if err := chars.SetImage(ctx, playerCaller, d.ID, sheet.ID, domain.Portrait, png); err != nil {
		t.Fatal(err)
	}
	var rule *app.RuleError
	for name, bad := range map[string][]byte{"empty": nil, "gif": []byte("GIF89a......"), "huge": make([]byte, app.MaxImageBytes+1)} {
		if err := chars.SetImage(ctx, playerCaller, d.ID, sheet.ID, domain.Portrait, bad); !errors.As(err, &rule) {
			t.Errorf("%s accepted: %v", name, err)
		}
	}
	if _, _, err := chars.Image(ctx, stranger, d.ID, sheet.ID, domain.Portrait); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("stranger saw the portrait: %v", err)
	}
	if _, _, err := chars.Image(ctx, playerCaller, d.ID, domain.CharacterID{}, domain.Portrait); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown character: %v", err)
	}
	combat.in = true
	if err := chars.SetImage(ctx, playerCaller, d.ID, sheet.ID, domain.Portrait, png); !errors.Is(err, domain.ErrLocked) {
		t.Fatalf("portrait in combat: %v", err)
	}
	if err := chars.ClearToken(ctx, playerCaller, d.ID, sheet.ID); !errors.Is(err, domain.ErrLocked) {
		t.Fatalf("clear in combat: %v", err)
	}
	combat.in = false
	chars.Blobs = brokenBlobs{}
	if err := chars.SetImage(ctx, playerCaller, d.ID, sheet.ID, domain.Portrait, png); err == nil {
		t.Fatal("storage failure on put ignored")
	}
	if _, _, err := chars.Image(ctx, playerCaller, d.ID, sheet.ID, domain.Portrait); err == nil {
		t.Fatal("storage failure on get ignored")
	}
}

// A draft is the caller's alone, sized and stepped within limits, and a story too long is refused.
func TestDraftLimits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := open(t)
	chars, _, d := party(t, pgstore.New(db.Pool()))
	if _, err := chars.SaveDraft(ctx, playerCaller, d.ID, 9, []byte(`{}`)); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("a step past the review = %v", err)
	}
	if _, err := chars.SaveDraft(ctx, playerCaller, d.ID, 1, make([]byte, 16_001)); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("a huge draft = %v", err)
	}
	stranger := caller.UI("nobody")
	if _, err := chars.Draft(ctx, stranger, d.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a stranger's draft = %v", err)
	}
	if err := chars.DiscardDraft(ctx, stranger, d.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a stranger discards = %v", err)
	}
	if _, err := chars.RollScores(ctx, stranger, d.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a stranger rolls = %v", err)
	}
	if _, err := chars.RollScores(ctx, playerCaller, d.ID); err == nil {
		t.Fatal("rolling without a roller")
	}
	long := fighter()
	long.Backstory = strings.Repeat("x", 4001)
	if _, err := chars.Preview(ctx, playerCaller, d.ID, long); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("a backstory too long = %v", err)
	}
}

// retrainCompendium offers fighters a feat at level 4, and fails the feature catalogue or the level's
// options from a call on.
type retrainCompendium struct {
	fakeOptions
	features, levelUp, failFeaturesAt, failLevelUpAt int
}

func (f *retrainCompendium) Features(context.Context) (features.Catalog, error) {
	f.features++
	if f.failFeaturesAt > 0 && f.features >= f.failFeaturesAt {
		return features.Catalog{}, errExtras
	}
	owner := features.Owner{Kind: "class", Slug: "fighter"}
	return features.Catalog{Choices: map[features.Owner][]features.Choice{owner: {{Slug: "feat", Name: "Feat", Level: 4, Count: 1, Pool: features.FeatCategory, From: "general"}}}}, nil
}

func (f *retrainCompendium) LevelUpOptions(context.Context, string, string, int) (compendium.LevelUpOptions, error) {
	f.levelUp++
	if f.failLevelUpAt > 0 && f.levelUp >= f.failLevelUpAt {
		return compendium.LevelUpOptions{}, errExtras
	}
	return compendium.LevelUpOptions{Feats: []compendium.FeatOption{
		{Slug: "ability-score-improvement", Name: "Ability Score Improvement", Category: "General", Description: ""},
		{Slug: "grappler", Name: "Grappler", Category: "General", Description: ""},
	}}, nil
}

func TestRetrainChecksPicksAndSurfacesPortFailures(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := open(t)
	chars, _, d := party(t, pgstore.New(db.Pool()))
	sheet, err := chars.Create(ctx, playerCaller, d.ID, fighter())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool().Exec(ctx, "INSERT INTO campaign.character_picks (character_id, level, choice, value) VALUES ($1, 4, 'feat', 'ability-score-improvement')", uuid.UUID(sheet.ID)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool().Exec(ctx, "UPDATE campaign.character_abilities SET increase = 2 WHERE character_id = $1 AND ability = 'dexterity'", uuid.UUID(sheet.ID)); err != nil {
		t.Fatal(err)
	}
	fb := fighter()
	in := app.RetrainInput{
		Species: fb.Species, Background: fb.Background, Method: fb.Method, Base: fb.Base, Bonus: fb.Bonus, Skills: fb.Skills,
		Picks: []domain.Pick{{Level: 4, Choice: "feat", Value: "grappler"}}, Increase: nil, Reason: "",
	}
	chars.Compendium = &retrainCompendium{}
	choices, err := chars.RetrainChoices(ctx, playerCaller, d.ID, sheet.ID)
	if err != nil || len(choices) != 1 || len(choices[0].Options) != 2 {
		t.Fatalf("choices = %+v %v", choices, err)
	}
	if _, err := chars.RetrainChoices(ctx, dmCaller, d.ID, sheet.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("the DM's retrain choices: %v", err)
	}
	for name, c := range map[string]*retrainCompendium{
		"features": {failFeaturesAt: 2},
		"level up": {failLevelUpAt: 1},
		"builder":  {fakeOptions: fakeOptions{err: errExtras}},
	} {
		chars.Compendium = c
		if _, err := chars.RetrainChoices(ctx, playerCaller, d.ID, sheet.ID); !errors.Is(err, errExtras) {
			t.Fatalf("%s failing on choices: %v", name, err)
		}
		chars.Compendium = &retrainCompendium{failFeaturesAt: c.failFeaturesAt, failLevelUpAt: c.failLevelUpAt}
		if c.err == nil {
			if _, err := chars.RequestRetrain(ctx, playerCaller, d.ID, sheet.ID, in); !errors.Is(err, errExtras) {
				t.Fatalf("%s failing on request: %v", name, err)
			}
		}
	}
	chars.Compendium = fakeOptions{}
	var rule *app.RuleError
	if _, err := chars.RequestRetrain(ctx, playerCaller, d.ID, sheet.ID, in); !errors.As(err, &rule) {
		t.Fatalf("a pick with no choice behind it: %v", err)
	}
	chars.Compendium = &retrainCompendium{}
	r, err := chars.RequestRetrain(ctx, playerCaller, d.ID, sheet.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := chars.DecideRetrain(ctx, dmCaller, d.ID, uuid.New(), true); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("an unknown retrain: %v", err)
	}
	chars.Compendium = &retrainCompendium{failFeaturesAt: 2}
	if _, err := chars.DecideRetrain(ctx, dmCaller, d.ID, r.ID, true); !errors.Is(err, errExtras) {
		t.Fatalf("approving while the catalogue fails: %v", err)
	}
	chars.Compendium = &retrainCompendium{}
	done, err := chars.DecideRetrain(ctx, dmCaller, d.ID, r.ID, true)
	if err != nil || done.Status != domain.RetrainApproved {
		t.Fatalf("approved = %+v %v", done, err)
	}
	after, err := chars.Get(ctx, playerCaller, d.ID, sheet.ID)
	if err != nil || len(after.Picks) != 1 || after.Picks[0].Value != "grappler" || len(after.Increase) != 0 {
		t.Fatalf("after = %+v %v", after.Picks, err)
	}
}
