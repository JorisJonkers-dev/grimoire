package surface_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
)

func TestSurfaces(t *testing.T) {
	t.Parallel()
	for _, k := range surface.Kinds() {
		if !surface.Valid(k) {
			t.Errorf("%s is a surface", k)
		}
	}
	if surface.Valid("lava") || surface.Valid(surface.None) {
		t.Fatal("not surfaces")
	}
	difficult := map[surface.Kind]bool{surface.Grease: true, surface.Ice: true, surface.Web: true}
	for _, k := range append(surface.Kinds(), surface.None) {
		if surface.Difficult(k) != difficult[k] {
			t.Errorf("%s difficult = %v", k, surface.Difficult(k))
		}
	}
	for k, want := range map[surface.Kind]string{surface.Fire: "1d4 fire", surface.Electrified: "1d4 lightning"} {
		if dice, kind, ok := surface.Hazard(k); !ok || dice+" "+kind != want {
			t.Errorf("%s hazard = %s %s", k, dice, kind)
		}
	}
	for _, k := range []surface.Kind{surface.None, surface.Grease, surface.Water, surface.Ice, surface.Web} {
		if _, _, ok := surface.Hazard(k); ok {
			t.Errorf("%s is harmless", k)
		}
	}
}

func TestInteractions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		k      surface.Kind
		damage string
		want   surface.Kind
	}{
		{surface.Grease, "fire", surface.Fire},
		{surface.Web, "fire", surface.None},
		{surface.Ice, "fire", surface.Water},
		{surface.Water, "cold", surface.Ice},
		{surface.Fire, "cold", surface.None},
		{surface.Water, "lightning", surface.Electrified},
		{surface.Water, "fire", surface.Water},
		{surface.Grease, "cold", surface.Grease},
		{surface.Ice, "lightning", surface.Ice},
		{surface.None, "fire", surface.None},
		{surface.Web, "thunder", surface.Web},
	}
	for _, c := range cases {
		if got := surface.React(c.k, c.damage); got != c.want {
			t.Errorf("%q + %s = %q, want %q", c.k, c.damage, got, c.want)
		}
	}
}
