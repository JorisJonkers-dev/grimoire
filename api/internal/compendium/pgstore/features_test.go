package pgstore_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/features"
)

// A fresh database holds the SRD 5.2 class mechanics as data, and the rules resolve them.
func TestSRDFeatureMechanicsResolveFromRows(t *testing.T) {
	t.Parallel()
	cat, err := pgstore.New(openPool(t)).Features(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	rage := cat.Resources["rage"]
	at6 := features.Stats{Level: 6, AbilityMod: 0, Proficiency: 3}
	if rage.Max(at6) != 4 || rage.Regain(features.ShortRest, at6, 0, 0) != 1 || rage.Regain(features.LongRest, at6, 0, 1) != 4 {
		t.Fatalf("Rage from rows = %+v", rage)
	}
	focus := cat.Resources["focus-points"]
	at5 := features.Stats{Level: 5, AbilityMod: 3, Proficiency: 3}
	if focus.Max(at5) != 5 || focus.Regain(features.ShortRest, at5, 0, 2) != 5 || focus.Max(features.Stats{Level: 1, AbilityMod: 3, Proficiency: 2}) != 0 {
		t.Fatalf("Focus Points from rows = %+v", focus)
	}
	inspiration := cat.Resources["bardic-inspiration"]
	if d, _ := inspiration.DieAt(10); d != "d10" || inspiration.Max(at5) != 3 || inspiration.Ability != "charisma" {
		t.Fatalf("Bardic Inspiration from rows = %+v", inspiration)
	}
	for level, want := range map[int]string{1: "1d6", 7: "4d6", 19: "10d6"} {
		if got, _ := cat.Scales["sneak-attack"].Steps.At(level); got != want {
			t.Errorf("Sneak Attack at %d = %s", level, got)
		}
	}
	fighter := features.Owner{Kind: "class", Slug: "fighter"}
	var first []string
	for _, c := range features.ChoicesAt(cat.Choices[fighter], 1) {
		first = append(first, c.Slug)
	}
	if !reflect.DeepEqual(first, []string{"fighting-style", "skills", "weapon-mastery"}) {
		t.Fatalf("a fighter chooses at level 1 = %v", first)
	}
	if got := len(features.ChoicesAt(cat.Choices[fighter], 6)); got != 1 {
		t.Fatalf("a fighter's extra feat at 6 = %d", got)
	}
	if got := cat.ResourcesOf(fighter); len(got) != 3 || got[0].Slug != "action-surge" {
		t.Fatalf("the fighter's resources = %+v", got)
	}
	grappler := cat.Prerequisites[features.Owner{Kind: "feat", Slug: "grappler"}]
	weak := features.Candidate{Level: 4, Abilities: map[string]int{"strength": 12, "dexterity": 12}, Spellcasting: false, Feats: nil, Features: nil}
	if got := features.Unmet(grappler, weak); !reflect.DeepEqual(got, []string{"Strength 13+ or Dexterity 13+"}) {
		t.Fatalf("Grappler for the weak = %v", got)
	}
	archery := cat.Prerequisites[features.Owner{Kind: "feat", Slug: "archery"}]
	if got := features.Unmet(archery, features.Candidate{Level: 1, Abilities: nil, Spellcasting: false, Feats: nil, Features: []string{"fighting-style"}}); got != nil {
		t.Fatalf("Archery with a Fighting Style = %v", got)
	}
}
