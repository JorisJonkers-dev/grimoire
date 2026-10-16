//go:build integration

package pgstore_test

import (
	"context"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/designs"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

func designed(slug string, dice string) designs.Loaded {
	return designs.Loaded{
		Owner: effects.Owner{Kind: effects.OwnedBySpell, Slug: slug},
		Definition: effects.Definition{
			Slug: slug, Name: slug, Owner: effects.OwnedBySpell, Duration: effects.Duration{Kind: effects.Instant},
			Parts: []effects.Part{
				effects.Area{Shape: hex.SphereArea, SizeFt: 20, RangeFt: 150},
				effects.SaveDamage{Ability: "dexterity", Dice: dice, Type: "fire", Half: true},
			},
			Scaling: []effects.Scaling{{Part: 1, Field: "dice", Axis: effects.SlotLevel, Rule: effects.Linear{Base: 3, Per: effects.Value{Text: "1d6"}}}},
		},
	}
}

func dice(t *testing.T, s *pgstore.Store, slug string) string {
	t.Helper()
	catalog, err := s.Effects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	d, ok := catalog[slug]
	if !ok {
		return ""
	}
	for _, p := range d.Parts {
		if sd, ok := p.(effects.SaveDamage); ok {
			return sd.Dice
		}
	}
	return "none"
}

// The Design files load in one transaction when their hash changes: they own the Effects they write,
// replace a seeded Effect of the same slug, and remove an Effect whose file is gone.
func TestDesignFilesLoadWhenTheyChange(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := pgstore.New(openPool(t))
	first := designs.Set{Effects: []designs.Loaded{designed("fireball", "6d6"), designed("ember-test", "2d6")}, Hash: strings.Repeat("a", 64)}
	if loaded, err := s.LoadDesigns(ctx, first); err != nil || !loaded {
		t.Fatalf("first load = %v %v", loaded, err)
	}
	if dice(t, s, "fireball") != "6d6" || dice(t, s, "ember-test") != "2d6" {
		t.Fatalf("after the first load: fireball %q, ember-test %q", dice(t, s, "fireball"), dice(t, s, "ember-test"))
	}
	if loaded, err := s.LoadDesigns(ctx, designs.Set{Hash: first.Hash}); err != nil || loaded {
		t.Fatalf("an unchanged hash loaded again: %v %v", loaded, err)
	}
	if dice(t, s, "ember-test") != "2d6" {
		t.Fatal("an unchanged hash touched the Effects")
	}

	broken := designed("ember-test", "4d6")
	broken.Definition.Scaling[0].Field = "range_ft"
	bad := designs.Set{Effects: []designs.Loaded{designed("fireball", "9d6"), broken}, Hash: strings.Repeat("b", 64)}
	if _, err := s.LoadDesigns(ctx, bad); err == nil {
		t.Fatal("a load that fails halfway succeeded")
	}
	if dice(t, s, "fireball") != "6d6" || dice(t, s, "ember-test") != "2d6" {
		t.Fatal("a failed load half applied")
	}

	second := designs.Set{Effects: []designs.Loaded{designed("fireball", "8d6")}, Hash: strings.Repeat("c", 64)}
	if loaded, err := s.LoadDesigns(ctx, second); err != nil || !loaded {
		t.Fatalf("second load = %v %v", loaded, err)
	}
	if dice(t, s, "fireball") != "8d6" || dice(t, s, "ember-test") != "" {
		t.Fatalf("after the second load: fireball %q, ember-test %q", dice(t, s, "fireball"), dice(t, s, "ember-test"))
	}
	if dice(t, s, "lightning-bolt") == "" {
		t.Fatal("a seeded Effect no file owns was removed")
	}
}
