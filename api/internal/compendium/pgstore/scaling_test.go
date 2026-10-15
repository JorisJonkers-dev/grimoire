package pgstore_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
)

// The seeded SRD spells that scale read as they always have, Scaling line last.
func TestSeededScalingReadsAsBefore(t *testing.T) {
	t.Parallel()
	got, err := pgstore.New(openPool(t)).Effects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"burning-hands":    {"A 15-foot Cone extends from the caster.", "Each creature in the area makes a Dexterity saving throw, taking 3d6 fire damage on a failed save or half as much damage on a successful one.", "Duration: Instantaneous.", "Using a Higher-Level Spell Slot: the damage increases by 1d6 for each slot level above 1."},
		"thunderwave":      {"A 15-foot Cube extends from the caster.", "Each creature in the area makes a Constitution saving throw, taking 2d8 thunder damage on a failed save or half as much damage on a successful one.", "A creature that fails is pushed up to 10 feet straight away from the caster.", "Duration: Instantaneous.", "Using a Higher-Level Spell Slot: the damage increases by 1d8 for each slot level above 1."},
		"shatter":          {"A 10-foot Sphere appears at a point within 60 feet.", "Each creature in the area makes a Constitution saving throw, taking 3d8 thunder damage on a failed save or half as much damage on a successful one.", "Duration: Instantaneous.", "Using a Higher-Level Spell Slot: the damage increases by 1d8 for each slot level above 2."},
		"fireball":         {"A 20-foot Sphere appears at a point within 150 feet.", "Each creature in the area makes a Dexterity saving throw, taking 8d6 fire damage on a failed save or half as much damage on a successful one.", "Duration: Instantaneous.", "Using a Higher-Level Spell Slot: the damage increases by 1d6 for each slot level above 3."},
		"lightning-bolt":   {"A 100-foot Line extends from the caster.", "Each creature in the area makes a Dexterity saving throw, taking 8d6 lightning damage on a failed save or half as much damage on a successful one.", "Duration: Instantaneous.", "Using a Higher-Level Spell Slot: the damage increases by 1d6 for each slot level above 3."},
		"cone-of-cold":     {"A 60-foot Cone extends from the caster.", "Each creature in the area makes a Constitution saving throw, taking 8d8 cold damage on a failed save or half as much damage on a successful one.", "Duration: Instantaneous.", "Using a Higher-Level Spell Slot: the damage increases by 1d8 for each slot level above 5."},
		"spirit-guardians": {"A 15-foot Emanation surrounds the caster and moves with it.", "Each creature in the area makes a Wisdom saving throw, taking 3d8 radiant damage on a failed save or half as much damage on a successful one.", "Duration: Concentration, up to 10 minutes.", "Using a Higher-Level Spell Slot: the damage increases by 1d8 for each slot level above 3."},
		"wall-of-fire":     {"A 60-foot Wall appears at a point within 120 feet.", "Each creature in the area makes a Dexterity saving throw, taking 5d8 fire damage on a failed save or half as much damage on a successful one.", "Duration: Concentration, up to 1 minute.", "Using a Higher-Level Spell Slot: the damage increases by 1d8 for each slot level above 4."},
	}
	for slug, lines := range want {
		if text := got[slug].Text(); !reflect.DeepEqual(text, lines) {
			t.Errorf("%s reads\n%q\nwant\n%q", slug, text, lines)
		}
	}
}
