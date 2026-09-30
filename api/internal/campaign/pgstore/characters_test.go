package pgstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/storage"
)

type fakeOptions struct{ err error }

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

type noShield struct{}

func (noShield) BuilderOptions(ctx context.Context, ruleset string) (compendium.BuilderOptions, error) {
	o, err := fakeOptions{}.BuilderOptions(ctx, ruleset)
	o.Armor = o.Armor[:2]
	return o, err
}

func TestOlderRulesetsChooseIncreasesFreely(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := pgstore.New(open(t).Pool())
	s, _ := service(t, repo)
	d, err := s.Create(ctx, dmCaller, app.CreateInput{Name: "Old school", Ruleset: "srd-2014", DisplayName: "DM"})
	if err != nil {
		t.Fatal(err)
	}
	chars := &app.Characters{Repo: repo, Compendium: fakeOptions{}, Combat: app.NoCombat{}, Now: time.Now}
	b := fighter()
	b.Bonus = map[string]int{"wisdom": 1, "intelligence": 1, "charisma": 1}
	sheet, err := chars.Create(ctx, dmCaller, d.ID, b)
	if err != nil || sheet.Ruleset != "srd-2014" || sheet.Scores["wisdom"] != 13 {
		t.Fatalf("2014 sheet = %+v %v", sheet, err)
	}
	if in, _ := (app.NoCombat{}).InCombat(ctx, sheet.ID); in {
		t.Fatal("no combat yet")
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
	boom := errors.New("boom")
	combat.err = boom
	if _, err := chars.Get(ctx, playerCaller, d.ID, sheet.ID); !errors.Is(err, boom) {
		t.Fatalf("combat status error: %v", err)
	}
	combat.err = nil
	chars.Compendium = fakeOptions{err: boom}
	if _, err := chars.Get(ctx, playerCaller, d.ID, sheet.ID); !errors.Is(err, boom) {
		t.Fatalf("compendium error on get: %v", err)
	}
	if _, err := chars.Preview(ctx, playerCaller, d.ID, fighter()); !errors.Is(err, boom) {
		t.Fatalf("compendium error on preview: %v", err)
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

var errSecond = errors.New("second lookup failed")

// secondFails answers the first lookup and fails the next, as a compendium outage mid-edit would.
type secondFails struct{ calls int }

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
	if err := chars.SetImage(ctx, playerCaller, d.ID, sheet.ID, domain.Portrait, png); err != nil {
		t.Fatal(err)
	}
	name := "Renamed"
	ops := map[string]func(c *app.Characters) error{
		"create":  func(c *app.Characters) error { _, err := c.Create(ctx, playerCaller, d.ID, fighter()); return err },
		"preview": func(c *app.Characters) error { _, err := c.Preview(ctx, playerCaller, d.ID, fighter()); return err },
		"list":    func(c *app.Characters) error { _, err := c.List(ctx, playerCaller, d.ID); return err },
		"get":     func(c *app.Characters) error { _, err := c.Get(ctx, playerCaller, d.ID, sheet.ID); return err },
		"update": func(c *app.Characters) error {
			_, err := c.Update(ctx, playerCaller, d.ID, sheet.ID, app.Edit{Name: &name})
			return err
		},
		"delete": func(c *app.Characters) error { return c.Delete(ctx, playerCaller, d.ID, doomed.ID) },
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
			c := &app.Characters{Repo: pgstore.NewFaulty(db.Pool(), f), Compendium: fakeOptions{}, Combat: app.NoCombat{}, Blobs: chars.Blobs, Now: time.Now}
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
