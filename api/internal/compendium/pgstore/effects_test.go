package pgstore_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

func seedOwner(slug string) effects.Owner {
	if slug == "prone" || slug == "poisoned" {
		return effects.Owner{Kind: effects.OwnedByCondition, Slug: slug}
	}
	return effects.Owner{Kind: effects.OwnedBySpell, Slug: slug}
}

// The migrations seed the SRD Effects the engine resolves: whole Effects, areas and the parts left to
// the DM all come back from rows.
func TestFreshDatabasesHoldTheSRDEffects(t *testing.T) {
	t.Parallel()
	got, err := pgstore.New(openPool(t)).Effects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"bless", "burning-hands", "cone-of-cold", "dodging", "exhaustion", "faerie-fire", "fireball", "grease", "hunters-mark", "invisible", "lightning-bolt", "paralyzed", "prone", "restrained", "sapped", "shatter", "slowed", "stunned", "vexed"}
	if !reflect.DeepEqual(got.Automated(), want) {
		t.Fatalf("automated = %v", got.Automated())
	}
	partial := []string{"blinded", "charmed", "deafened", "frightened", "grappled", "incapacitated", "petrified", "poisoned", "thunderwave", "unconscious"}
	if !reflect.DeepEqual(got.Partial(), partial) {
		t.Fatalf("partial = %v", got.Partial())
	}
	blessed := []effects.Active{{Slug: "bless", Source: "cleric"}}
	if p := got.ForAttack(blessed, []effects.Active{{Slug: "prone"}}, "cleric", true); !reflect.DeepEqual(p.AttackDice, []string{"1d4"}) || !reflect.DeepEqual(p.Advantages, []string{"Prone: advantage"}) {
		t.Fatalf("Bless against a prone target = %+v", p)
	}
	if fb, ok := got.AreaOf("fireball"); !ok || fb.Area != (effects.Area{Shape: hex.SphereArea, SizeFt: 20, RangeFt: 150}) || fb.Damage.Dice != "8d6" || fb.Save != "dexterity" {
		t.Fatalf("fireball = %+v", fb)
	}
	paralyzed := []effects.Active{{Slug: "paralyzed"}}
	if !got.Incapacitated(paralyzed) || !got.Immobile(paralyzed) || !got.ForSave(paralyzed, "dexterity").Fails || !got.ForAttack(nil, paralyzed, "x", true).Crit {
		t.Fatalf("paralysed from rows = %+v", got["paralyzed"])
	}
	if got.SpeedPenaltyFt([]effects.Active{{Slug: "exhaustion", Level: 2}}) != 10 {
		t.Fatalf("exhaustion from rows = %+v", got["exhaustion"])
	}
	if g, _ := got.AreaOf("grease"); g.Condition != "prone" || g.Surface.Rounds != 10 {
		t.Fatalf("grease = %+v", g)
	}
	if !reflect.DeepEqual(got.Instructions("poisoned", "Poisoned"), []string{"Poisoned: ability checks are made with disadvantage."}) {
		t.Fatalf("poisoned = %v", got.Instructions("poisoned", "Poisoned"))
	}
}

// Every seeded Effect survives being saved again unchanged, so the editor can write back what it read.
func TestSeededEffectsRoundTripThroughTheDatabase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := pgstore.New(openPool(t))
	seeded, err := s.Effects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for slug, d := range seeded {
		if err := s.SaveEffect(ctx, seedOwner(slug), d); err != nil {
			t.Fatalf("save %s: %v", slug, err)
		}
	}
	got, err := s.Effects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, seeded) {
		t.Fatalf("saved again differs:\n got %+v\nwant %+v", got, seeded)
	}
}

// Saving an Effect again replaces its components rather than adding to them.
func TestSavingAnEffectReplacesItsComponents(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := pgstore.New(openPool(t))
	owner := effects.Owner{Kind: effects.OwnedBySpell, Slug: "frost-ring"}
	first := effects.Definition{Slug: "frost-ring", Name: "Frost Ring", Components: []effects.Component{
		effects.Area{Shape: hex.SphereArea, SizeFt: 10, RangeFt: 30},
		effects.SaveDamage{Ability: "constitution", Dice: "2d6", Type: "cold", Half: true},
		effects.Manual{Instruction: "Frost Ring: the ground freezes."},
	}}
	if err := s.SaveEffect(ctx, owner, first); err != nil {
		t.Fatal(err)
	}
	second := effects.Definition{Slug: "frost-ring", Name: "Frost Ring", Concentration: true, Components: []effects.Component{
		effects.MoveCost{Multiplier: 2},
	}}
	if err := s.SaveEffect(ctx, owner, second); err != nil {
		t.Fatal(err)
	}
	got, err := s.Effects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got["frost-ring"], second) {
		t.Fatalf("frost-ring = %+v", got["frost-ring"])
	}
}

// The database refuses parts that cannot be right, so a bad Effect never reaches play.
func TestInvalidEffectsAreRefused(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := pgstore.New(openPool(t))
	owner := effects.Owner{Kind: effects.OwnedBySpell, Slug: "bad"}
	bad := []effects.Definition{
		{Slug: "Bad Slug", Name: "x"},
		{Slug: "bad-dice", Name: "x", Components: []effects.Component{effects.ExtraDamage{Dice: "lots"}}},
		{Slug: "bad-shape", Name: "x", Components: []effects.Component{effects.Area{Shape: "pyramid", SizeFt: 10}}},
		{Slug: "no-roll", Name: "x", Components: []effects.Component{effects.BonusDie{Dice: "1d4"}}},
	}
	for _, d := range bad {
		if err := s.SaveEffect(ctx, owner, d); err == nil {
			t.Errorf("%s saved", d.Slug)
		}
	}
	if err := s.SaveEffect(ctx, effects.Owner{Kind: "spellbook", Slug: "x"}, effects.Definition{Slug: "odd-owner", Name: "x"}); err == nil {
		t.Error("unknown owner kind saved")
	}
}

func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	store, err := pg.Open(context.Background(), pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	return store.Pool()
}
