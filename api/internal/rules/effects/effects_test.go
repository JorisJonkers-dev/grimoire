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
	want := []string{"bless", "burning-hands", "cone-of-cold", "exhaustion", "faerie-fire", "fireball", "grease", "hunters-mark", "invisible", "lightning-bolt", "paralyzed", "prone", "restrained", "sapped", "shatter", "slowed", "stunned", "vexed"}
	if got := srd().Automated(); !reflect.DeepEqual(got, want) {
		t.Fatalf("automated = %v", got)
	}
	if got := srd().Partial(); !reflect.DeepEqual(got, []string{"blinded", "charmed", "deafened", "frightened", "grappled", "incapacitated", "petrified", "poisoned", "thunderwave", "unconscious"}) {
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

func TestEveryConditionDoesWhatTheRulesSay(t *testing.T) {
	t.Parallel()
	cat := srd()
	on := func(slugs ...string) []effects.Active {
		var out []effects.Active
		for _, s := range slugs {
			out = append(out, effects.Active{Slug: s})
		}
		return out
	}
	mode := func(p effects.AttackProfile) string {
		return map[[2]bool]string{{true, false}: "advantage", {false, true}: "disadvantage", {false, false}: "normal", {true, true}: "normal"}[[2]bool{len(p.Advantages) > 0, len(p.Disadvantages) > 0}]
	}
	for slug, want := range map[string][2]string{
		"blinded": {"disadvantage", "advantage"}, "invisible": {"advantage", "disadvantage"}, "frightened": {"disadvantage", "normal"},
		"poisoned": {"disadvantage", "normal"}, "restrained": {"disadvantage", "advantage"}, "paralyzed": {"normal", "advantage"},
		"petrified": {"normal", "advantage"}, "stunned": {"normal", "advantage"}, "unconscious": {"normal", "advantage"},
		"charmed": {"normal", "normal"}, "deafened": {"normal", "normal"}, "grappled": {"normal", "normal"}, "incapacitated": {"normal", "normal"},
	} {
		if got := mode(cat.ForAttack(on(slug), nil, "x", true)); got != want[0] {
			t.Errorf("%s attacking = %s, want %s", slug, got, want[0])
		}
		if got := mode(cat.ForAttack(nil, on(slug), "x", false)); got != want[1] {
			t.Errorf("%s attacked = %s, want %s", slug, got, want[1])
		}
	}
	for _, slug := range []string{"incapacitated", "paralyzed", "petrified", "stunned", "unconscious"} {
		if !cat.Incapacitated(on(slug)) {
			t.Errorf("%s incapacitates", slug)
		}
	}
	for _, slug := range []string{"grappled", "paralyzed", "petrified", "restrained", "unconscious"} {
		if !cat.Immobile(on(slug)) {
			t.Errorf("%s holds speed at zero", slug)
		}
	}
	if cat.Incapacitated(on("prone", "grappled")) || cat.Immobile(on("prone", "stunned")) {
		t.Error("prone is neither")
	}
	for _, slug := range []string{"paralyzed", "petrified", "stunned", "unconscious"} {
		for _, ability := range []string{"strength", "dexterity"} {
			if !cat.ForSave(on(slug), ability).Fails {
				t.Errorf("%s fails %s saves", slug, ability)
			}
		}
		if cat.ForSave(on(slug), "wisdom").Fails {
			t.Errorf("%s keeps its Wisdom saves", slug)
		}
	}
	if p := cat.ForSave(on("restrained", "bless"), "dexterity"); p.Fails || !reflect.DeepEqual(p.Disadvantages, []string{"Restrained: disadvantage"}) || !reflect.DeepEqual(p.Dice, []string{"1d4"}) {
		t.Errorf("restrained Dexterity save = %+v", p)
	}
	resolve := effects.Catalog{"resolve": {Name: "Resolve", Components: []effects.Component{effects.SaveEdge{Ability: "wisdom", Mode: effects.SaveAdvantage}}}}
	if p := resolve.ForSave([]effects.Active{{Slug: "resolve"}}, "wisdom"); !reflect.DeepEqual(p.Advantages, []string{"Resolve: advantage"}) || p.Disadvantages != nil || p.Fails {
		t.Errorf("a homebrew advantage on Wisdom saves = %+v", p)
	}
	if p := cat.ForAttack(nil, on("unconscious"), "x", true); !p.Crit || len(p.Notes) != 1 {
		t.Errorf("a hit from 5 ft on the unconscious = %+v", p)
	}
	if p := cat.ForAttack(nil, on("paralyzed", "unconscious"), "x", true); !p.Crit || len(p.Notes) != 1 {
		t.Errorf("one Critical Hit note however many conditions = %+v", p)
	}
	if p := cat.ForAttack(nil, on("unconscious"), "x", false); p.Crit {
		t.Error("a hit from afar is no Critical Hit")
	}
	near := effects.Catalog{"near": {Name: "Near", Components: []effects.Component{effects.CritWithin{Feet: 0}}}}
	if p := near.ForAttack(nil, []effects.Active{{Slug: "near"}}, "x", true); p.Crit {
		t.Error("a zero-foot reach makes no Critical Hit")
	}
}

func TestExhaustionStacksAndKillsAtSix(t *testing.T) {
	t.Parallel()
	cat := srd()
	tired := []effects.Active{{Slug: "exhaustion", Level: 3}}
	if p := cat.ForAttack(tired, nil, "x", true); p.Penalty != 6 || !reflect.DeepEqual(p.Notes, []string{"Exhaustion 3: -6 to hit"}) {
		t.Errorf("attacking while exhausted = %+v", p)
	}
	if p := cat.ForSave(tired, "wisdom"); p.Penalty != 6 || p.Fails {
		t.Errorf("saving while exhausted = %+v", p)
	}
	if got := cat.SpeedPenaltyFt(tired); got != 15 {
		t.Errorf("speed penalty = %d", got)
	}
	if got := cat.SpeedPenaltyFt([]effects.Active{{Slug: "exhaustion"}}); got != 5 {
		t.Errorf("a level-less exhaustion counts once = %d", got)
	}
	if _, dead := cat.Fatal(tired); dead {
		t.Error("three levels are not fatal")
	}
	if name, dead := cat.Fatal([]effects.Active{{Slug: "exhaustion", Level: 6}}); !dead || name != "Exhaustion" {
		t.Errorf("six levels kill = %q %v", name, dead)
	}
	if _, dead := (effects.Catalog{"e": {Name: "E", Components: []effects.Component{effects.Exhausting{D20PerLevel: 1, SpeedFtPerLevel: 0, DeathAt: 0}}}}).Fatal([]effects.Active{{Slug: "e", Level: 9}}); dead {
		t.Error("an exhaustion without a death level never kills")
	}
	if !cat.Stacks("exhaustion") || cat.Stacks("prone") {
		t.Error("only exhaustion stacks")
	}
}

func TestEffectsThatReact(t *testing.T) {
	t.Parallel()
	cat := effects.Catalog{"rebuke": {Name: "Hellish Rebuke", Components: []effects.Component{
		effects.Reacts{Trigger: "damaged", Instruction: "Hellish Rebuke: the attacker makes a Dexterity save or takes 2d10 fire damage."},
	}}}
	got := cat.ReactionsTo([]effects.Active{{Slug: "rebuke"}, {Slug: "bless"}}, "damaged")
	if len(got) != 1 || got[0].Name != "Hellish Rebuke" || got[0].Instruction == "" {
		t.Errorf("reactions = %+v", got)
	}
	if cat.ReactionsTo([]effects.Active{{Slug: "rebuke"}}, "hit") != nil {
		t.Error("another trigger")
	}
	if p := cat.ForAttack([]effects.Active{{Slug: "rebuke"}}, []effects.Active{{Slug: "rebuke"}}, "x", true); !reflect.DeepEqual(p, effects.AttackProfile{}) {
		t.Errorf("a reaction does nothing to attacks = %+v", p)
	}
	if p := cat.ForSave([]effects.Active{{Slug: "rebuke"}}, "dexterity"); p.Fails || p.Penalty != 0 || cat.SpeedPenaltyFt([]effects.Active{{Slug: "rebuke"}}) != 0 {
		t.Errorf("nor to saves = %+v", p)
	}
	if _, ok := cat.AreaOf("rebuke"); ok {
		t.Error("nor is it an area")
	}
}

func TestMasteryEffects(t *testing.T) {
	t.Parallel()
	cat := srd()
	vexed := []effects.Active{{Slug: "vexed", Source: "aria"}}
	if p := cat.ForAttack(nil, vexed, "aria", true); len(p.Advantages) != 1 {
		t.Errorf("the vexer has advantage = %+v", p)
	}
	if p := cat.ForAttack(nil, vexed, "brom", true); len(p.Advantages) != 0 {
		t.Errorf("nobody else does = %+v", p)
	}
	if p := cat.ForAttack([]effects.Active{{Slug: "sapped"}}, nil, "x", true); len(p.Disadvantages) != 1 {
		t.Errorf("a sapped attacker = %+v", p)
	}
	slowed := []effects.Active{{Slug: "slowed"}, {Slug: "slowed"}, {Slug: "exhaustion", Level: 1}}
	if got := cat.SpeedPenaltyFt(slowed); got != 15 {
		t.Errorf("two Slows count once, exhaustion adds = %d", got)
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
		"blinded": {Slug: "blinded", Name: "Blinded", Components: []effects.Component{
			effects.Edge{Against: true, Advantage: true, Range: effects.AnyRange},
			effects.Edge{Against: false, Advantage: false, Range: effects.AnyRange},
			effects.Manual{Instruction: "Blinded: can't see, and fails any ability check that needs sight."},
		}},
		"charmed": {Slug: "charmed", Name: "Charmed", Components: []effects.Component{
			effects.Manual{Instruction: "Charmed: can't attack the charmer or target them with harmful effects; the charmer has advantage on social checks against them."},
		}},
		"deafened": {Slug: "deafened", Name: "Deafened", Components: []effects.Component{
			effects.Manual{Instruction: "Deafened: can't hear, and fails any ability check that needs hearing."},
		}},
		"sapped":     {Slug: "sapped", Name: "Sapped", Components: []effects.Component{effects.Edge{Against: false, Advantage: false, Range: effects.AnyRange}}},
		"slowed":     {Slug: "slowed", Name: "Slowed", Components: []effects.Component{effects.SpeedPenalty{Ft: 10}}},
		"vexed":      {Slug: "vexed", Name: "Vexed", Components: []effects.Component{effects.Edge{Against: true, Advantage: true, Range: effects.AnyRange, SourceOnly: true}}},
		"exhaustion": {Slug: "exhaustion", Name: "Exhaustion", Components: []effects.Component{effects.Exhausting{D20PerLevel: 2, SpeedFtPerLevel: 5, DeathAt: 6}}},
		"frightened": {Slug: "frightened", Name: "Frightened", Components: []effects.Component{
			effects.Edge{Against: false, Advantage: false, Range: effects.AnyRange},
			effects.Manual{Instruction: "Frightened: can't willingly move closer to the source of its fear, and the disadvantage holds only while it can see that source."},
		}},
		"grappled": {Slug: "grappled", Name: "Grappled", Components: []effects.Component{
			effects.Immobile{}, effects.Manual{Instruction: "Grappled: has disadvantage on attacks against anyone but the grappler, who can drag it along at half speed."},
		}},
		"incapacitated": {Slug: "incapacitated", Name: "Incapacitated", Components: []effects.Component{
			effects.Incapacitated{}, effects.Manual{Instruction: "Incapacitated: can't speak, and rolls Initiative with disadvantage."},
		}},
		"invisible": {Slug: "invisible", Name: "Invisible", Components: []effects.Component{
			effects.Edge{Against: true, Advantage: false, Range: effects.AnyRange}, effects.Edge{Against: false, Advantage: true, Range: effects.AnyRange},
		}},
		"paralyzed": {Slug: "paralyzed", Name: "Paralyzed", Components: []effects.Component{
			effects.Incapacitated{},
			effects.Immobile{},
			effects.SaveEdge{Ability: "strength", Mode: effects.SaveFails},
			effects.SaveEdge{Ability: "dexterity", Mode: effects.SaveFails},
			effects.Edge{Against: true, Advantage: true, Range: effects.AnyRange},
			effects.CritWithin{Feet: 5},
		}},
		"petrified": {Slug: "petrified", Name: "Petrified", Components: []effects.Component{
			effects.Incapacitated{},
			effects.Immobile{},
			effects.SaveEdge{Ability: "strength", Mode: effects.SaveFails},
			effects.SaveEdge{Ability: "dexterity", Mode: effects.SaveFails},
			effects.Edge{Against: true, Advantage: true, Range: effects.AnyRange},
			effects.Manual{Instruction: "Petrified: turned to stone with what it wears and carries; it resists all damage and is immune to poison."},
		}},
		"restrained": {Slug: "restrained", Name: "Restrained", Components: []effects.Component{
			effects.Immobile{},
			effects.Edge{Against: true, Advantage: true, Range: effects.AnyRange},
			effects.Edge{Against: false, Advantage: false, Range: effects.AnyRange},
			effects.SaveEdge{Ability: "dexterity", Mode: effects.SaveDisadvantage},
		}},
		"stunned": {Slug: "stunned", Name: "Stunned", Components: []effects.Component{
			effects.Incapacitated{},
			effects.SaveEdge{Ability: "strength", Mode: effects.SaveFails},
			effects.SaveEdge{Ability: "dexterity", Mode: effects.SaveFails},
			effects.Edge{Against: true, Advantage: true, Range: effects.AnyRange},
		}},
		"unconscious": {Slug: "unconscious", Name: "Unconscious", Components: []effects.Component{
			effects.Incapacitated{},
			effects.Immobile{},
			effects.SaveEdge{Ability: "strength", Mode: effects.SaveFails},
			effects.SaveEdge{Ability: "dexterity", Mode: effects.SaveFails},
			effects.Edge{Against: true, Advantage: true, Range: effects.AnyRange},
			effects.CritWithin{Feet: 5},
			effects.Manual{Instruction: "Unconscious: drops what it holds, falls Prone and knows nothing of its surroundings."},
		}},
		"poisoned": {Slug: "poisoned", Name: "Poisoned", Concentration: false, Components: []effects.Component{
			effects.Edge{Against: false, Advantage: false, Range: effects.AnyRange},
			effects.Manual{Instruction: "Poisoned: ability checks are made with disadvantage."},
		}},
	}
}
