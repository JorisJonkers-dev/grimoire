package pgstore_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
)

// The migrations seed the built-in Surfaces exactly; an author's Surface saves and replaces its reactions.
func TestSurfacesAreData(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := pgstore.New(openPool(t))
	got, err := s.Surfaces(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := surface.Builtin()
	for k, d := range want {
		if !reflect.DeepEqual(got[k], d) {
			t.Errorf("%s:\n got %+v\nwant %+v", k, got[k], d)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("surfaces = %v", got.Kinds())
	}
	quicksand := surface.Definition{
		Kind: "quicksand", Name: "Quicksand", Cost: 3, Obscures: surface.Clear, Effect: "restrained",
		Reactions: []surface.Reaction{{Damage: "cold", Becomes: surface.Ice}, {Damage: "fire", Becomes: surface.None}},
	}
	if err := s.SaveSurface(ctx, quicksand); err != nil {
		t.Fatal(err)
	}
	quicksand.Reactions = quicksand.Reactions[:1]
	quicksand.HazardDice, quicksand.HazardType, quicksand.EveryStep = "1d6", "bludgeoning", true
	if err := s.SaveSurface(ctx, quicksand); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Surfaces(ctx); !reflect.DeepEqual(got["quicksand"], quicksand) {
		t.Fatalf("quicksand = %+v", got["quicksand"])
	}
	for _, bad := range []surface.Definition{
		{Kind: "Bad Slug", Name: "x"},
		{Kind: "half-hazard", Name: "x", HazardDice: "1d6"},
		{Kind: "too-dear", Name: "x", Cost: 9},
		{Kind: "murky", Name: "x", Obscures: "murk"},
	} {
		if err := s.SaveSurface(ctx, bad); err == nil {
			t.Errorf("%s saved", bad.Kind)
		}
	}
}
