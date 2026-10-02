package surface_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
)

func TestBuiltinSurfaces(t *testing.T) {
	t.Parallel()
	cat := surface.Builtin()
	if len(cat.Kinds()) != 14 || cat.Kinds()[0] != surface.Consecrated || cat.Valid(surface.None) || cat.Valid("lava-lamp") {
		t.Fatalf("kinds = %v", cat.Kinds())
	}
	for _, k := range cat.Kinds() {
		if !cat.Valid(k) || cat[k].Name == "" {
			t.Errorf("%s is a named surface", k)
		}
	}
	for k, want := range map[surface.Kind]int{
		surface.Grease: 2, surface.Ice: 2, surface.Web: 2, surface.Spikes: 2, surface.Mud: 2, surface.PlantGrowth: 4, surface.Fire: 1, surface.None: 1, "unknown": 1,
	} {
		if got := cat.Cost(k); got != want {
			t.Errorf("%s costs %d per foot", k, got)
		}
	}
	for k, want := range map[surface.Kind]string{
		surface.Fire: "1d4 fire false", surface.Electrified: "1d4 lightning false", surface.Lava: "10d10 fire false", surface.Spikes: "2d4 piercing true",
	} {
		if dice, kind, every, ok := cat.Hazard(k); !ok || dice+" "+kind+" "+map[bool]string{true: "true", false: "false"}[every] != want {
			t.Errorf("%s hazard = %s %s %v", k, dice, kind, every)
		}
	}
	for _, k := range []surface.Kind{surface.None, surface.Grease, surface.Water, surface.Fog, surface.Mud} {
		if _, _, _, ok := cat.Hazard(k); ok {
			t.Errorf("%s is harmless", k)
		}
	}
	for k, want := range map[surface.Kind]surface.Obscurement{
		surface.Fog: surface.ObscuredHeavy, surface.StinkingCloud: surface.ObscuredHeavy, surface.Darkness: surface.ObscuredDark, surface.Fire: surface.Clear,
	} {
		if got := cat.Obscures(k); got != want {
			t.Errorf("%s obscures %q", k, got)
		}
	}
	for k, want := range map[surface.Kind]string{surface.StinkingCloud: "poisoned", surface.Consecrated: "bless", surface.Fog: ""} {
		if got := cat.Effect(k); got != want {
			t.Errorf("%s puts on %q", k, got)
		}
	}
}

func TestInteractions(t *testing.T) {
	t.Parallel()
	cat := surface.Builtin()
	for _, c := range []struct {
		k      surface.Kind
		damage string
		want   surface.Kind
	}{
		{surface.Grease, "fire", surface.Fire},
		{surface.Web, "fire", surface.None},
		{surface.PlantGrowth, "fire", surface.None},
		{surface.Ice, "fire", surface.Water},
		{surface.Water, "cold", surface.Ice},
		{surface.Fire, "cold", surface.None},
		{surface.Water, "lightning", surface.Electrified},
		{surface.Water, "fire", surface.Water},
		{surface.Grease, "cold", surface.Grease},
		{surface.Ice, "lightning", surface.Ice},
		{surface.None, "fire", surface.None},
		{surface.Fog, "thunder", surface.Fog},
	} {
		if got := cat.React(c.k, c.damage); got != c.want {
			t.Errorf("%s hit by %s = %s", c.k, c.damage, got)
		}
	}
}
