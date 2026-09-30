package effects_test

import (
	"reflect"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
)

func TestAttackProfilesFoldBothSides(t *testing.T) {
	t.Parallel()
	blessed := []effects.Active{{Slug: "bless", Source: "cleric"}}
	marked := []effects.Active{{Slug: "hunters-mark", Source: "ranger"}, {Slug: "faerie-fire", Source: "druid"}}
	p := effects.ForAttack(blessed, marked, "ranger", false)
	want := effects.AttackProfile{
		Advantages: []string{"Faerie Fire: advantage"}, AttackDice: []string{"1d4"}, DamageDice: []string{"1d6"},
		Notes: []string{"Bless: +1d4 to hit", "Hunter's Mark: +1d6 damage"},
	}
	if !reflect.DeepEqual(p, want) {
		t.Fatalf("profile = %+v", p)
	}
	if p := effects.ForAttack(nil, marked, "someone else", true); p.DamageDice != nil || len(p.Advantages) != 1 {
		t.Fatalf("only the marker deals the extra damage = %+v", p)
	}
	prone := []effects.Active{{Slug: "prone"}}
	if p := effects.ForAttack(nil, prone, "x", true); !reflect.DeepEqual(p.Advantages, []string{"Prone: advantage"}) || p.Disadvantages != nil {
		t.Fatalf("close to a prone target = %+v", p)
	}
	if p := effects.ForAttack(nil, prone, "x", false); !reflect.DeepEqual(p.Disadvantages, []string{"Prone: disadvantage"}) || p.Advantages != nil {
		t.Fatalf("far from a prone target = %+v", p)
	}
	sick := []effects.Active{{Slug: "poisoned"}, {Slug: "prone"}, {Slug: "unknown-curse"}}
	if p := effects.ForAttack(sick, nil, "x", true); !reflect.DeepEqual(p.Disadvantages, []string{"Poisoned: disadvantage", "Prone: disadvantage"}) {
		t.Fatalf("a poisoned, prone attacker = %+v", p)
	}
	if p := effects.ForAttack(marked, blessed, "x", true); !reflect.DeepEqual(p, effects.AttackProfile{}) {
		t.Fatalf("effects that do nothing from that side = %+v", p)
	}
}

func TestSavesAndMovement(t *testing.T) {
	t.Parallel()
	if got := effects.SaveDice([]effects.Active{{Slug: "bless"}, {Slug: "prone"}}); !reflect.DeepEqual(got, []string{"1d4"}) {
		t.Fatalf("save dice = %v", got)
	}
	if got := effects.SaveDice(nil); got != nil {
		t.Fatalf("no effects, no dice = %v", got)
	}
	if got := effects.MoveMultiplier([]effects.Active{{Slug: "bless"}, {Slug: "prone"}}); got != 2 {
		t.Fatalf("prone movement = %d", got)
	}
	if got := effects.MoveMultiplier(nil); got != 1 {
		t.Fatalf("free movement = %d", got)
	}
}

func TestManualFallback(t *testing.T) {
	t.Parallel()
	if got := effects.Instructions("poisoned", "Poisoned"); !reflect.DeepEqual(got, []string{"Poisoned: ability checks are made with disadvantage."}) {
		t.Fatalf("partly modelled = %v", got)
	}
	if got := effects.Instructions("hold-person", "Hold Person"); !reflect.DeepEqual(got, []string{"Resolve Hold Person by hand."}) {
		t.Fatalf("not modelled = %v", got)
	}
	if got := effects.Instructions("bless", "Bless"); got != nil {
		t.Fatalf("fully modelled = %v", got)
	}
	want := []string{"bless", "burning-hands", "cone-of-cold", "faerie-fire", "fireball", "grease", "hunters-mark", "lightning-bolt", "prone", "shatter"}
	if got := effects.Automated(); !reflect.DeepEqual(got, want) {
		t.Fatalf("automated = %v", got)
	}
	if got := effects.Partial(); !reflect.DeepEqual(got, []string{"poisoned", "thunderwave"}) {
		t.Fatalf("partial = %v", got)
	}
	d, ok := effects.Lookup("bless")
	if !ok || !d.Concentration || d.Name != "Bless" || !d.Automated() {
		t.Fatalf("bless = %+v", d)
	}
	if _, ok := effects.Lookup("wish"); ok {
		t.Fatal("wish is not modelled")
	}
	if d, _ := effects.Lookup("poisoned"); d.Automated() || d.Concentration {
		t.Fatalf("poisoned = %+v", d)
	}
}

func TestAreaSpells(t *testing.T) {
	t.Parallel()
	fb, ok := effects.AreaOf("fireball")
	if !ok || fb.Name != "Fireball" || fb.Area.Shape != hex.SphereArea || fb.Area.SizeFt != 20 || fb.Area.RangeFt != 150 || fb.Save != "dexterity" ||
		fb.Damage.Dice != "8d6" || fb.Damage.Type != "fire" || !fb.Damage.Half || fb.Condition != "" || fb.Surface.Kind != surface.None || fb.Instructions != nil {
		t.Fatalf("fireball = %+v", fb)
	}
	g, _ := effects.AreaOf("grease")
	if g.Save != "dexterity" || g.Condition != "prone" || g.Surface.Kind != surface.Grease || g.Surface.Rounds != 10 || g.Damage.Dice != "" {
		t.Fatalf("grease = %+v", g)
	}
	if tw, _ := effects.AreaOf("thunderwave"); len(tw.Instructions) != 1 || tw.Save != "constitution" {
		t.Fatalf("thunderwave = %+v", tw)
	}
	for _, slug := range []string{"bless", "wish"} {
		if _, ok := effects.AreaOf(slug); ok {
			t.Errorf("%s is not an area spell", slug)
		}
	}
	if p := effects.ForAttack([]effects.Active{{Slug: "fireball"}}, []effects.Active{{Slug: "grease"}}, "", true); !reflect.DeepEqual(p, effects.AttackProfile{}) {
		t.Fatalf("areas do nothing to attacks = %+v", p)
	}
}
