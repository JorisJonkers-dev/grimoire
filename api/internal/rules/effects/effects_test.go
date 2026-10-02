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
	p := srd().ForAttack(blessed, marked, "ranger", false)
	want := effects.AttackProfile{
		Advantages: []string{"Faerie Fire: advantage"}, AttackDice: []string{"1d4"}, DamageDice: []string{"1d6"},
		Notes: []string{"Bless: +1d4 to hit", "Hunter's Mark: +1d6 damage"},
	}
	if !reflect.DeepEqual(p, want) {
		t.Fatalf("profile = %+v", p)
	}
	if p := srd().ForAttack(nil, marked, "someone else", true); p.DamageDice != nil || len(p.Advantages) != 1 {
		t.Fatalf("only the marker deals the extra damage = %+v", p)
	}
	prone := []effects.Active{{Slug: "prone"}}
	if p := srd().ForAttack(nil, prone, "x", true); !reflect.DeepEqual(p.Advantages, []string{"Prone: advantage"}) || p.Disadvantages != nil {
		t.Fatalf("close to a prone target = %+v", p)
	}
	if p := srd().ForAttack(nil, prone, "x", false); !reflect.DeepEqual(p.Disadvantages, []string{"Prone: disadvantage"}) || p.Advantages != nil {
		t.Fatalf("far from a prone target = %+v", p)
	}
	sick := []effects.Active{{Slug: "poisoned"}, {Slug: "prone"}, {Slug: "unknown-curse"}}
	if p := srd().ForAttack(sick, nil, "x", true); !reflect.DeepEqual(p.Disadvantages, []string{"Poisoned: disadvantage", "Prone: disadvantage"}) {
		t.Fatalf("a poisoned, prone attacker = %+v", p)
	}
	if p := srd().ForAttack(marked, blessed, "x", true); !reflect.DeepEqual(p, effects.AttackProfile{}) {
		t.Fatalf("effects that do nothing from that side = %+v", p)
	}
}

func TestSavesAndMovement(t *testing.T) {
	t.Parallel()
	if got := srd().SaveDice([]effects.Active{{Slug: "bless"}, {Slug: "prone"}}); !reflect.DeepEqual(got, []string{"1d4"}) {
		t.Fatalf("save dice = %v", got)
	}
	if got := srd().SaveDice(nil); got != nil {
		t.Fatalf("no effects, no dice = %v", got)
	}
	if got := srd().MoveMultiplier([]effects.Active{{Slug: "bless"}, {Slug: "prone"}}); got != 2 {
		t.Fatalf("prone movement = %d", got)
	}
	if got := srd().MoveMultiplier(nil); got != 1 {
		t.Fatalf("free movement = %d", got)
	}
}

func TestManualFallback(t *testing.T) {
	t.Parallel()
	if got := srd().Instructions("poisoned", "Poisoned"); !reflect.DeepEqual(got, []string{"Poisoned: ability checks are made with disadvantage."}) {
		t.Fatalf("partly modelled = %v", got)
	}
	if got := srd().Instructions("hold-person", "Hold Person"); !reflect.DeepEqual(got, []string{"Resolve Hold Person by hand."}) {
		t.Fatalf("not modelled = %v", got)
	}
	if got := srd().Instructions("bless", "Bless"); got != nil {
		t.Fatalf("fully modelled = %v", got)
	}
	want := []string{"bless", "burning-hands", "cone-of-cold", "faerie-fire", "fireball", "grease", "hunters-mark", "lightning-bolt", "prone", "shatter"}
	if got := srd().Automated(); !reflect.DeepEqual(got, want) {
		t.Fatalf("automated = %v", got)
	}
	if got := srd().Partial(); !reflect.DeepEqual(got, []string{"poisoned", "thunderwave"}) {
		t.Fatalf("partial = %v", got)
	}
	d, ok := srd().Lookup("bless")
	if !ok || !d.Concentration || d.Name != "Bless" || !d.Automated() {
		t.Fatalf("bless = %+v", d)
	}
	if _, ok := srd().Lookup("wish"); ok {
		t.Fatal("wish is not modelled")
	}
	if d, _ := srd().Lookup("poisoned"); d.Automated() || d.Concentration {
		t.Fatalf("poisoned = %+v", d)
	}
}

func TestAreaSpells(t *testing.T) {
	t.Parallel()
	fb, ok := srd().AreaOf("fireball")
	if !ok || fb.Name != "Fireball" || fb.Area.Shape != hex.SphereArea || fb.Area.SizeFt != 20 || fb.Area.RangeFt != 150 || fb.Save != "dexterity" ||
		fb.Damage.Dice != "8d6" || fb.Damage.Type != "fire" || !fb.Damage.Half || fb.Condition != "" || fb.Surface.Kind != surface.None || fb.Instructions != nil {
		t.Fatalf("fireball = %+v", fb)
	}
	g, _ := srd().AreaOf("grease")
	if g.Save != "dexterity" || g.Condition != "prone" || g.Surface.Kind != surface.Grease || g.Surface.Rounds != 10 || g.Damage.Dice != "" {
		t.Fatalf("grease = %+v", g)
	}
	if tw, _ := srd().AreaOf("thunderwave"); len(tw.Instructions) != 1 || tw.Save != "constitution" {
		t.Fatalf("thunderwave = %+v", tw)
	}
	for _, slug := range []string{"bless", "wish"} {
		if _, ok := srd().AreaOf(slug); ok {
			t.Errorf("%s is not an area spell", slug)
		}
	}
	if p := srd().ForAttack([]effects.Active{{Slug: "fireball"}}, []effects.Active{{Slug: "grease"}}, "", true); !reflect.DeepEqual(p, effects.AttackProfile{}) {
		t.Fatalf("areas do nothing to attacks = %+v", p)
	}
}

// srd mirrors the Effects the migrations seed, so the rules can be tested without a database.
func srd() effects.Catalog {
	return effects.Catalog{
		"bless": {Slug: "bless", Name: "Bless", Concentration: true, Components: []effects.Component{
			effects.BonusDie{On: []effects.Roll{effects.AttackRolls, effects.SavingThrows}, Dice: "1d4"},
		}},
		"faerie-fire": {Slug: "faerie-fire", Name: "Faerie Fire", Concentration: true, Components: []effects.Component{
			effects.Edge{Against: true, Advantage: true, Range: effects.AnyRange},
		}},
		"hunters-mark": {Slug: "hunters-mark", Name: "Hunter's Mark", Concentration: true, Components: []effects.Component{
			effects.ExtraDamage{Dice: "1d6"},
		}},
		"fireball": {Slug: "fireball", Name: "Fireball", Concentration: false, Components: []effects.Component{
			effects.Area{Shape: hex.SphereArea, SizeFt: 20, RangeFt: 150}, effects.SaveDamage{Ability: "dexterity", Dice: "8d6", Type: "fire", Half: true},
		}},
		"burning-hands": {Slug: "burning-hands", Name: "Burning Hands", Concentration: false, Components: []effects.Component{
			effects.Area{Shape: hex.ConeArea, SizeFt: 15, RangeFt: 0}, effects.SaveDamage{Ability: "dexterity", Dice: "3d6", Type: "fire", Half: true},
		}},
		"lightning-bolt": {Slug: "lightning-bolt", Name: "Lightning Bolt", Concentration: false, Components: []effects.Component{
			effects.Area{Shape: hex.LineArea, SizeFt: 100, RangeFt: 0}, effects.SaveDamage{Ability: "dexterity", Dice: "8d6", Type: "lightning", Half: true},
		}},
		"cone-of-cold": {Slug: "cone-of-cold", Name: "Cone of Cold", Concentration: false, Components: []effects.Component{
			effects.Area{Shape: hex.ConeArea, SizeFt: 60, RangeFt: 0}, effects.SaveDamage{Ability: "constitution", Dice: "8d8", Type: "cold", Half: true},
		}},
		"shatter": {Slug: "shatter", Name: "Shatter", Concentration: false, Components: []effects.Component{
			effects.Area{Shape: hex.SphereArea, SizeFt: 10, RangeFt: 60}, effects.SaveDamage{Ability: "constitution", Dice: "3d8", Type: "thunder", Half: true},
		}},
		"thunderwave": {Slug: "thunderwave", Name: "Thunderwave", Concentration: false, Components: []effects.Component{
			effects.Area{Shape: hex.CubeArea, SizeFt: 15, RangeFt: 0},
			effects.SaveDamage{Ability: "constitution", Dice: "2d8", Type: "thunder", Half: true},
			effects.Manual{Instruction: "Thunderwave: creatures that failed their save are pushed 10 feet away."},
		}},
		"grease": {Slug: "grease", Name: "Grease", Concentration: false, Components: []effects.Component{
			effects.Area{Shape: hex.CylinderArea, SizeFt: 5, RangeFt: 60},
			effects.CreateSurface{Kind: surface.Grease, Rounds: 10},
			effects.SaveCondition{Ability: "dexterity", Slug: "prone"},
		}},
		"prone": {Slug: "prone", Name: "Prone", Concentration: false, Components: []effects.Component{
			effects.Edge{Against: false, Advantage: false, Range: effects.AnyRange},
			effects.Edge{Against: true, Advantage: true, Range: effects.WithinFive},
			effects.Edge{Against: true, Advantage: false, Range: effects.BeyondFive},
			effects.MoveCost{Multiplier: 2},
		}},
		"poisoned": {Slug: "poisoned", Name: "Poisoned", Concentration: false, Components: []effects.Component{
			effects.Edge{Against: false, Advantage: false, Range: effects.AnyRange},
			effects.Manual{Instruction: "Poisoned: ability checks are made with disadvantage."},
		}},
	}
}
