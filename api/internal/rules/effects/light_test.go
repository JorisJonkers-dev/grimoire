package effects_test

import (
	"slices"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

func TestAnAreaCanDeclareItsSaveAndShedLight(t *testing.T) {
	t.Parallel()
	cat := effects.Catalog{"lantern": {Slug: "lantern", Name: "Lantern", Components: []effects.Component{
		effects.Area{Shape: hex.EmanationArea, SizeFt: 10},
		effects.AreaSave{Ability: "wisdom"},
		effects.Light{BrightFt: 20, DimFt: 20},
	}}}
	spell, ok := cat.AreaOf("lantern")
	if !ok || spell.Save != "wisdom" || spell.Light != (effects.Light{BrightFt: 20, DimFt: 20}) {
		t.Fatalf("area = %+v %v", spell, ok)
	}
	text := cat["lantern"].Text()
	for _, want := range []string{
		"Each creature in the area makes a Wisdom saving throw.",
		"Bright light fills 20 feet around where it lands, and dim light another 20 feet.",
	} {
		if !slices.Contains(text, want) {
			t.Errorf("missing %q in %v", want, text)
		}
	}
}
