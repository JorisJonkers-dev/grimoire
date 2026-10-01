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

func builtinOwner(slug string) effects.Owner {
	if slug == "prone" || slug == "poisoned" {
		return effects.Owner{Kind: effects.OwnedByCondition, Slug: slug}
	}
	return effects.Owner{Kind: effects.OwnedBySpell, Slug: slug}
}

// Every built-in Effect survives a trip through the database unchanged, so reading Effects as data
// resolves exactly as the catalogue in code did.
func TestBuiltinEffectsRoundTripThroughTheDatabase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := pgstore.New(openPool(t))
	for slug, d := range effects.Builtin() {
		if err := s.SaveEffect(ctx, builtinOwner(slug), d); err != nil {
			t.Fatalf("save %s: %v", slug, err)
		}
	}
	got, err := s.Effects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, effects.Builtin()) {
		t.Fatalf("loaded catalogue differs:\n got %+v\nwant %+v", got, effects.Builtin())
	}
	blessed := []effects.Active{{Slug: "bless", Source: "cleric"}}
	if p := got.ForAttack(blessed, nil, "cleric", true); !reflect.DeepEqual(p.AttackDice, []string{"1d4"}) {
		t.Fatalf("Bless from the database = %+v", p)
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
