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

func TestWhatIsNotModelledIsLeftToTheDMAsAnInstruction(t *testing.T) {
	t.Parallel()
	if got := srd().Instructions("poisoned", "Poisoned", ""); !reflect.DeepEqual(got, []string{"Poisoned: ability checks are made with disadvantage."}) {
		t.Fatalf("partly modelled = %v", got)
	}
	if got := srd().Instructions("hold-person", "Hold Person", ""); !reflect.DeepEqual(got, []string{"Resolve Hold Person by hand."}) {
		t.Fatalf("not modelled = %v", got)
	}
	if got := srd().Instructions("bless", "Bless", ""); got != nil {
		t.Fatalf("fully modelled = %v", got)
	}
}

func TestTheCatalogListsWhatItAutomatesAndWhatOnlyPartly(t *testing.T) {
	t.Parallel()
	want := []string{"bless", "burning-hands", "cone-of-cold", "exhaustion", "faerie-fire", "fireball", "grease", "hunters-mark", "invisible", "lightning-bolt", "paralyzed", "prone", "restrained", "sapped", "shatter", "slowed", "stunned", "vexed"}
	if got := srd().Automated(); !reflect.DeepEqual(got, want) {
		t.Fatalf("automated = %v", got)
	}
	if got := srd().Partial(); !reflect.DeepEqual(got, []string{"blinded", "charmed", "deafened", "frightened", "grappled", "incapacitated", "petrified", "poisoned", "thunderwave", "unconscious"}) {
		t.Fatalf("partial = %v", got)
	}
}

func TestAModelledEffectIsLookedUpWithItsConcentrationAndAutomation(t *testing.T) {
	t.Parallel()
	if d, ok := srd().Lookup("bless"); !ok || !d.Concentration || d.Name != "Bless" || !d.Automated() {
		t.Fatalf("bless = %+v", d)
	}
	if d, _ := srd().Lookup("poisoned"); d.Automated() || d.Concentration {
		t.Fatalf("poisoned = %+v", d)
	}
}

func TestAnUnmodelledEffectIsNotFound(t *testing.T) {
	t.Parallel()
	if _, ok := srd().Lookup("wish"); ok {
		t.Fatal("found")
	}
}

func TestAnAreaSpellCarriesItsShapeSaveDamageConditionAndSurface(t *testing.T) {
	t.Parallel()
	fb, ok := srd().AreaOf("fireball")
	if !ok || fb.Name != "Fireball" || fb.Area.Shape != hex.SphereArea || fb.Area.SizeFt != 20 || fb.Area.RangeFt != 150 || fb.Save != "dexterity" ||
		fb.Damage.Dice != "8d6" || fb.Damage.Type != "fire" || !reflect.DeepEqual(fb.Lands(effects.Outcome{Saved: true, Damage: 8}), []effects.Act{effects.Hurt{Amount: 4, Type: "fire"}}) || fb.Condition != "" || fb.Surface.Surface != surface.None || fb.Instructions != nil {
		t.Fatalf("fireball = %+v", fb)
	}
	g, _ := srd().AreaOf("grease")
	if g.Save != "dexterity" || g.Condition != "prone" || g.Surface.Surface != surface.Grease || g.Surface.Rounds != 10 || g.Damage.Dice != "" {
		t.Fatalf("grease = %+v", g)
	}
	if tw, _ := srd().AreaOf("thunderwave"); len(tw.Instructions) != 1 || tw.Save != "constitution" {
		t.Fatalf("thunderwave = %+v", tw)
	}
}

func TestASpellWithoutAnAreaHasNone(t *testing.T) {
	t.Parallel()
	for _, slug := range []string{"bless", "wish"} {
		t.Run(slug, func(t *testing.T) {
			if _, ok := srd().AreaOf(slug); ok {
				t.Fatal("an area")
			}
		})
	}
}

func TestAnAreaSpellDoesNothingToAttacks(t *testing.T) {
	t.Parallel()
	if p := srd().ForAttack([]effects.Active{{Slug: "fireball"}}, []effects.Active{{Slug: "grease"}}, "", true); !reflect.DeepEqual(p, effects.AttackProfile{}) {
		t.Fatalf("profile = %+v", p)
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
	resolve := effects.Catalog{"resolve": {Name: "Resolve", Parts: []effects.Part{effects.SaveEdge{Ability: "wisdom", Mode: effects.SaveAdvantage}}}}
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
	near := effects.Catalog{"near": {Name: "Near", Parts: []effects.Part{effects.CritWithin{Feet: 0}}}}
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
	if _, dead := (effects.Catalog{"e": {Name: "E", Parts: []effects.Part{effects.Exhausting{D20PerLevel: 1, SpeedFtPerLevel: 0, DeathAt: 0, MaxLevel: 0}}}}).Fatal([]effects.Active{{Slug: "e", Level: 9}}); dead {
		t.Error("an exhaustion without a death level never kills")
	}
	if !cat.Stacks("exhaustion") || cat.Stacks("prone") {
		t.Error("only exhaustion stacks")
	}
}

func TestTheWiderKinds(t *testing.T) {
	t.Parallel()
	cat := effects.Catalog{
		"false-life": {Name: "False Life", Owner: effects.OwnedBySpell, Parts: []effects.Part{effects.TempHP{Amount: 8}, effects.TempHP{Amount: 3}}},
		"misty-step": {Name: "Misty Step", Owner: effects.OwnedBySpell, Parts: []effects.Part{effects.Teleport{RangeFt: 30}}},
		"dispel":     {Name: "Dispel Magic", Owner: effects.OwnedBySpell, Parts: []effects.Part{effects.Dispel{}}},
		"counter":    {Name: "Counterspell", Owner: effects.OwnedBySpell, Parts: []effects.Part{effects.Counter{RangeFt: 60}}},
		"short":      {Name: "Short", Parts: []effects.Part{effects.Counter{RangeFt: 30}}},
		"echo":       {Name: "Echo", Parts: []effects.Part{effects.Counter{RangeFt: 60}}},
		"darkvision": {Name: "Darkvision", Owner: effects.OwnedBySpell, Parts: []effects.Part{effects.GrantFeature{Name: "Darkvision 60 ft"}}},
		"second":     {Name: "Second Wind", Owner: effects.OwnedByFeature, Parts: []effects.Part{effects.ResourceChange{Resource: "rage", Delta: 1}}},
		"wave": {Name: "Wave", Parts: []effects.Part{
			effects.Area{Shape: hex.CubeArea, SizeFt: 15, RangeFt: 0},
			effects.SaveDamage{Ability: "constitution", Dice: "2d8", Type: "thunder", Half: true},
			effects.ForcedMove{Ft: 10, Toward: false},
		}},
	}
	if got := cat.LandingOf("false-life", ""); got.TempHP != 8 || got.Dispels || got.Grants != nil {
		t.Errorf("temporary hit points keep the highest = %+v", got)
	}
	if got := cat.LandingOf("dispel", ""); !got.Dispels {
		t.Errorf("dispel = %+v", got)
	}
	if got := cat.LandingOf("darkvision", ""); !reflect.DeepEqual(got.Grants, []string{"Darkvision 60 ft"}) {
		t.Errorf("grants = %+v", got)
	}
	if got := cat.LandingOf("second", ""); !reflect.DeepEqual(got.Resources, []effects.ResourceChange{{Resource: "rage", Delta: 1}}) {
		t.Errorf("resources = %+v", got)
	}
	if got := cat.LandingOf("wave", ""); !reflect.DeepEqual(got, effects.Landing{}) {
		t.Errorf("an area lands nothing on its own = %+v", got)
	}
	if ft, ok := cat.TeleportOf("misty-step"); !ok || ft != 30 {
		t.Errorf("misty step = %d %v", ft, ok)
	}
	if _, ok := cat.TeleportOf("dispel"); ok {
		t.Error("dispel teleports nobody")
	}
	if ft, name, ok := cat.CounterOf([]effects.Active{{Slug: "short"}, {Slug: "counter"}, {Slug: "echo"}, {Slug: "dispel"}}); !ok || ft != 60 || name != "Counterspell" {
		t.Errorf("the longest counter = %d %s %v", ft, name, ok)
	}
	if _, _, ok := cat.CounterOf([]effects.Active{{Slug: "dispel"}}); ok {
		t.Error("no counter")
	}
	if !cat.Spell("dispel") || cat.Spell("second") {
		t.Error("only spells are spells")
	}
	if a, ok := cat.AreaOf("wave"); !ok || a.Push != (effects.ForcedMove{Ft: 10, Toward: false}) {
		t.Errorf("an area that pushes = %+v", a)
	}
	every := []effects.Active{{Slug: "false-life"}, {Slug: "misty-step"}, {Slug: "dispel"}, {Slug: "counter"}, {Slug: "darkvision"}, {Slug: "second"}, {Slug: "wave"}}
	if p := cat.ForAttack(every, every, "x", true); !reflect.DeepEqual(p, effects.AttackProfile{}) {
		t.Errorf("none of them touch attacks = %+v", p)
	}
	if p := cat.ForSave(every, "dexterity"); p.Fails || p.Penalty != 0 || cat.SpeedPenaltyFt(every) != 0 || cat.ReactionsTo(every, "damaged") != nil {
		t.Errorf("nor saves, speed or reactions = %+v", p)
	}
	if _, ok := cat.AreaOf("dispel"); ok {
		t.Error("dispel has no area")
	}
}

func TestEffectsThatReact(t *testing.T) {
	t.Parallel()
	cat := effects.Catalog{"rebuke": {Name: "Hellish Rebuke", Parts: []effects.Part{
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

func TestOnlyTheVexerHasAdvantageOnTheVexedTarget(t *testing.T) {
	t.Parallel()
	vexed := []effects.Active{{Slug: "vexed", Source: "aria"}}
	if p := srd().ForAttack(nil, vexed, "aria", true); len(p.Advantages) != 1 {
		t.Errorf("the vexer = %+v", p)
	}
	if p := srd().ForAttack(nil, vexed, "brom", true); len(p.Advantages) != 0 {
		t.Errorf("someone else = %+v", p)
	}
}

func TestASappedAttackerHasDisadvantage(t *testing.T) {
	t.Parallel()
	if p := srd().ForAttack([]effects.Active{{Slug: "sapped"}}, nil, "x", true); len(p.Disadvantages) != 1 {
		t.Fatalf("profile = %+v", p)
	}
}

func TestTwoSlowsCountOnceAndExhaustionAddsToTheSpeedPenalty(t *testing.T) {
	t.Parallel()
	slowed := []effects.Active{{Slug: "slowed"}, {Slug: "slowed"}, {Slug: "exhaustion", Level: 1}}
	if got := srd().SpeedPenaltyFt(slowed); got != 15 {
		t.Fatalf("penalty = %d", got)
	}
}

// srd mirrors the Effects the migrations seed, so the rules can be tested without a database.
func srd() effects.Catalog {
	return effects.Catalog{
		"bless": {Slug: "bless", Name: "Bless", Concentration: true, Parts: []effects.Part{
			effects.BonusDie{On: []effects.Roll{effects.AttackRolls, effects.SavingThrows}, Dice: "1d4"},
		}},
		"faerie-fire": {Slug: "faerie-fire", Name: "Faerie Fire", Concentration: true, Parts: []effects.Part{
			effects.Edge{Against: true, Advantage: true, Range: effects.AnyRange},
		}},
		"hunters-mark": {Slug: "hunters-mark", Name: "Hunter's Mark", Concentration: true, Parts: []effects.Part{
			effects.ExtraDamage{Dice: "1d6"},
		}},
		"fireball": {Slug: "fireball", Name: "Fireball", Concentration: false, Parts: []effects.Part{
			effects.Area{Shape: hex.SphereArea, SizeFt: 20, RangeFt: 150}, effects.SaveDamage{Ability: "dexterity", Dice: "8d6", Type: "fire", Half: true},
		}},
		"burning-hands": {Slug: "burning-hands", Name: "Burning Hands", Concentration: false, Parts: []effects.Part{
			effects.Area{Shape: hex.ConeArea, SizeFt: 15, RangeFt: 0}, effects.SaveDamage{Ability: "dexterity", Dice: "3d6", Type: "fire", Half: true},
		}},
		"lightning-bolt": {Slug: "lightning-bolt", Name: "Lightning Bolt", Concentration: false, Parts: []effects.Part{
			effects.Area{Shape: hex.LineArea, SizeFt: 100, RangeFt: 0}, effects.SaveDamage{Ability: "dexterity", Dice: "8d6", Type: "lightning", Half: true},
		}},
		"cone-of-cold": {Slug: "cone-of-cold", Name: "Cone of Cold", Concentration: false, Parts: []effects.Part{
			effects.Area{Shape: hex.ConeArea, SizeFt: 60, RangeFt: 0}, effects.SaveDamage{Ability: "constitution", Dice: "8d8", Type: "cold", Half: true},
		}},
		"shatter": {Slug: "shatter", Name: "Shatter", Concentration: false, Parts: []effects.Part{
			effects.Area{Shape: hex.SphereArea, SizeFt: 10, RangeFt: 60}, effects.SaveDamage{Ability: "constitution", Dice: "3d8", Type: "thunder", Half: true},
		}},
		"thunderwave": {Slug: "thunderwave", Name: "Thunderwave", Concentration: false, Parts: []effects.Part{
			effects.Area{Shape: hex.CubeArea, SizeFt: 15, RangeFt: 0},
			effects.SaveDamage{Ability: "constitution", Dice: "2d8", Type: "thunder", Half: true},
			effects.Manual{Instruction: "Thunderwave: creatures that failed their save are pushed 10 feet away."},
		}},
		"grease": {Slug: "grease", Name: "Grease", Concentration: false, Parts: []effects.Part{
			effects.Area{Shape: hex.CylinderArea, SizeFt: 5, RangeFt: 60},
			effects.CreateSurface{Surface: surface.Grease, Rounds: 10},
			effects.SaveCondition{Ability: "dexterity", Slug: "prone"},
		}},
		"prone": {Slug: "prone", Name: "Prone", Concentration: false, Parts: []effects.Part{
			effects.Edge{Against: false, Advantage: false, Range: effects.AnyRange},
			effects.Edge{Against: true, Advantage: true, Range: effects.WithinFive},
			effects.Edge{Against: true, Advantage: false, Range: effects.BeyondFive},
			effects.MoveCost{Multiplier: 2},
		}},
		"blinded": {Slug: "blinded", Name: "Blinded", Parts: []effects.Part{
			effects.Edge{Against: true, Advantage: true, Range: effects.AnyRange},
			effects.Edge{Against: false, Advantage: false, Range: effects.AnyRange},
			effects.Manual{Instruction: "Blinded: can't see, and fails any ability check that needs sight."},
		}},
		"charmed": {Slug: "charmed", Name: "Charmed", Parts: []effects.Part{
			effects.Manual{Instruction: "Charmed: can't attack the charmer or target them with harmful effects; the charmer has advantage on social checks against them."},
		}},
		"deafened": {Slug: "deafened", Name: "Deafened", Parts: []effects.Part{
			effects.Manual{Instruction: "Deafened: can't hear, and fails any ability check that needs hearing."},
		}},
		"sapped":     {Slug: "sapped", Name: "Sapped", Parts: []effects.Part{effects.Edge{Against: false, Advantage: false, Range: effects.AnyRange}}},
		"slowed":     {Slug: "slowed", Name: "Slowed", Parts: []effects.Part{effects.SpeedPenalty{Ft: 10}}},
		"vexed":      {Slug: "vexed", Name: "Vexed", Parts: []effects.Part{effects.Edge{Against: true, Advantage: true, Range: effects.AnyRange, SourceOnly: true}}},
		"exhaustion": {Slug: "exhaustion", Name: "Exhaustion", Parts: []effects.Part{effects.Exhausting{D20PerLevel: 2, SpeedFtPerLevel: 5, DeathAt: 6, MaxLevel: 0}}},
		"frightened": {Slug: "frightened", Name: "Frightened", Parts: []effects.Part{
			effects.Edge{Against: false, Advantage: false, Range: effects.AnyRange},
			effects.Manual{Instruction: "Frightened: can't willingly move closer to the source of its fear, and the disadvantage holds only while it can see that source."},
		}},
		"grappled": {Slug: "grappled", Name: "Grappled", Parts: []effects.Part{
			effects.Immobile{}, effects.Manual{Instruction: "Grappled: has disadvantage on attacks against anyone but the grappler, who can drag it along at half speed."},
		}},
		"incapacitated": {Slug: "incapacitated", Name: "Incapacitated", Parts: []effects.Part{
			effects.Incapacitated{}, effects.Manual{Instruction: "Incapacitated: can't speak, and rolls Initiative with disadvantage."},
		}},
		"invisible": {Slug: "invisible", Name: "Invisible", Parts: []effects.Part{
			effects.Edge{Against: true, Advantage: false, Range: effects.AnyRange}, effects.Edge{Against: false, Advantage: true, Range: effects.AnyRange},
		}},
		"paralyzed": {Slug: "paralyzed", Name: "Paralyzed", Parts: []effects.Part{
			effects.Incapacitated{},
			effects.Immobile{},
			effects.SaveEdge{Ability: "strength", Mode: effects.SaveFails},
			effects.SaveEdge{Ability: "dexterity", Mode: effects.SaveFails},
			effects.Edge{Against: true, Advantage: true, Range: effects.AnyRange},
			effects.CritWithin{Feet: 5},
		}},
		"petrified": {Slug: "petrified", Name: "Petrified", Parts: []effects.Part{
			effects.Incapacitated{},
			effects.Immobile{},
			effects.SaveEdge{Ability: "strength", Mode: effects.SaveFails},
			effects.SaveEdge{Ability: "dexterity", Mode: effects.SaveFails},
			effects.Edge{Against: true, Advantage: true, Range: effects.AnyRange},
			effects.Manual{Instruction: "Petrified: turned to stone with what it wears and carries; it resists all damage and is immune to poison."},
		}},
		"restrained": {Slug: "restrained", Name: "Restrained", Parts: []effects.Part{
			effects.Immobile{},
			effects.Edge{Against: true, Advantage: true, Range: effects.AnyRange},
			effects.Edge{Against: false, Advantage: false, Range: effects.AnyRange},
			effects.SaveEdge{Ability: "dexterity", Mode: effects.SaveDisadvantage},
		}},
		"stunned": {Slug: "stunned", Name: "Stunned", Parts: []effects.Part{
			effects.Incapacitated{},
			effects.SaveEdge{Ability: "strength", Mode: effects.SaveFails},
			effects.SaveEdge{Ability: "dexterity", Mode: effects.SaveFails},
			effects.Edge{Against: true, Advantage: true, Range: effects.AnyRange},
		}},
		"unconscious": {Slug: "unconscious", Name: "Unconscious", Parts: []effects.Part{
			effects.Incapacitated{},
			effects.Immobile{},
			effects.SaveEdge{Ability: "strength", Mode: effects.SaveFails},
			effects.SaveEdge{Ability: "dexterity", Mode: effects.SaveFails},
			effects.Edge{Against: true, Advantage: true, Range: effects.AnyRange},
			effects.CritWithin{Feet: 5},
			effects.Manual{Instruction: "Unconscious: drops what it holds, falls Prone and knows nothing of its surroundings."},
		}},
		"poisoned": {Slug: "poisoned", Name: "Poisoned", Concentration: false, Parts: []effects.Part{
			effects.Edge{Against: false, Advantage: false, Range: effects.AnyRange},
			effects.Manual{Instruction: "Poisoned: ability checks are made with disadvantage."},
		}},
	}
}

func TestStackingEffectsRiseToTheirCap(t *testing.T) {
	t.Parallel()
	cat := effects.Catalog{
		"frostbite": {Slug: "frostbite", Name: "Frostbite", Parts: []effects.Part{effects.Exhausting{D20PerLevel: 1, SpeedFtPerLevel: 5, DeathAt: 0, MaxLevel: 3}}},
		"bless":     {Slug: "bless", Name: "Bless", Parts: []effects.Part{effects.BonusDie{On: []effects.Roll{effects.AttackRolls}, Dice: "1d4"}}},
	}
	if !cat.Stacks("frostbite") || cat.MaxLevel("frostbite") != 3 || cat.Stacks("bless") || cat.MaxLevel("bless") != 0 || cat.MaxLevel("unknown") != 0 {
		t.Fatal("frostbite stacks to 3; bless does not stack")
	}
	if ft := cat.SpeedPenaltyFt([]effects.Active{{Slug: "frostbite", Source: "", Level: 3, Mode: ""}}); ft != 15 {
		t.Fatalf("three levels of frostbite = %d feet", ft)
	}
}

// Where each passive and bearer-landing kind is read: a chosen mode counts for folds, landing and
// death, while teleporting, stacking and the level cap read only an Effect's own Parts; Parts beneath
// a branch count for none of them until it lands.
func TestWhereEachPassiveKindIsRead(t *testing.T) {
	t.Parallel()
	cat := effects.Catalog{
		"moody": {Name: "Moody", Parts: []effects.Part{effects.Choice{Modes: []effects.Group{{Name: "Grim", Parts: []effects.Part{
			effects.Edge{Against: false, Advantage: true, Range: effects.AnyRange, SourceOnly: true},
			effects.Edge{Against: true, Advantage: false, Range: effects.AnyRange, SourceOnly: false},
			effects.ExtraDamage{Dice: "1d8"},
			effects.CritWithin{Feet: 30},
			effects.SaveEdge{Ability: "wisdom", Mode: effects.SaveFails},
			effects.Exhausting{D20PerLevel: 1, SpeedFtPerLevel: 5, DeathAt: 2, MaxLevel: 4},
			effects.MoveCost{Multiplier: 3},
			effects.MoveCost{Multiplier: 2},
			effects.SpeedPenalty{Ft: 10},
			effects.SpeedPenalty{Ft: 20},
			effects.Incapacitated{},
			effects.Immobile{},
			effects.Reacts{Trigger: "damaged", Instruction: "Hiss."},
			effects.Counter{RangeFt: 30},
			effects.TempHP{Amount: 4},
			effects.TempHP{Amount: 7},
			effects.Dispel{},
			effects.GrantFeature{Name: "Gloom"},
			effects.ResourceChange{Resource: "ki", Delta: -1},
			effects.Teleport{RangeFt: 30},
		}}}}}},
		"later": {Name: "Later", Parts: []effects.Part{effects.Branch{When: effects.Condition{Kind: effects.FirstEachTurn}, Then: []effects.Part{
			effects.Edge{Against: false, Advantage: true},
			effects.Exhausting{D20PerLevel: 1, DeathAt: 1},
			effects.Incapacitated{},
			effects.Immobile{},
			effects.MoveCost{Multiplier: 4},
			effects.Reacts{Trigger: "damaged"},
			effects.Counter{RangeFt: 60},
			effects.TempHP{Amount: 9},
			effects.Teleport{RangeFt: 60},
		}}}},
		"echo": {Name: "Echo", Parts: []effects.Part{effects.Reacts{Trigger: "damaged", Instruction: "Echo."}, effects.Reacts{Trigger: "hit", Instruction: "No."}, effects.Counter{RangeFt: 30}}},
	}
	grim := []effects.Active{{Slug: "moody", Source: "aria", Level: 2, Mode: "Grim"}}
	p := cat.ForAttack(grim, grim, "brom", true)
	want := effects.AttackProfile{
		Advantages: []string{"Moody: advantage"}, Disadvantages: []string{"Moody: disadvantage"}, DamageDice: nil, AttackDice: nil,
		Notes: []string{"Moody 2: -2 to hit", "Moody: a hit from this close is a Critical Hit"}, Penalty: 2, Crit: true,
	}
	if !reflect.DeepEqual(p, want) {
		t.Errorf("a chosen mode on both sides = %+v", p)
	}
	if s := cat.ForSave(grim, "wisdom"); !s.Fails || s.Penalty != 2 || s.Advantages != nil || s.Dice != nil {
		t.Errorf("a chosen mode's save = %+v", s)
	}
	if !cat.Incapacitated(grim) || !cat.Immobile(grim) || cat.MoveMultiplier(grim) != 3 || cat.SpeedPenaltyFt(grim) != 30 {
		t.Errorf("a chosen mode's movement = %v %v %d %d", cat.Incapacitated(grim), cat.Immobile(grim), cat.MoveMultiplier(grim), cat.SpeedPenaltyFt(grim))
	}
	if name, dead := cat.Fatal(grim); !dead || name != "Moody" {
		t.Errorf("two levels kill = %q %v", name, dead)
	}
	if got := cat.ReactionsTo(append(grim, effects.Active{Slug: "echo"}), "damaged"); !reflect.DeepEqual(got, []effects.Reaction{{Name: "Moody", Instruction: "Hiss."}, {Name: "Echo", Instruction: "Echo."}}) {
		t.Errorf("reactions in order = %+v", got)
	}
	if ft, name, ok := cat.CounterOf(append(grim, effects.Active{Slug: "echo"})); !ok || ft != 30 || name != "Moody" {
		t.Errorf("the first of equal counters = %d %s %v", ft, name, ok)
	}
	land := cat.LandingOf("moody", "Grim")
	if !reflect.DeepEqual(land, effects.Landing{TempHP: 7, Dispels: true, Grants: []string{"Gloom"}, Resources: []effects.ResourceChange{{Resource: "ki", Delta: -1}}}) {
		t.Errorf("a chosen mode lands = %+v", land)
	}
	if got := cat.LandingOf("moody", ""); !reflect.DeepEqual(got, effects.Landing{}) {
		t.Errorf("no mode chosen lands nothing = %+v", got)
	}
	if _, ok := cat.TeleportOf("moody"); ok || cat.Stacks("moody") || cat.MaxLevel("moody") != 0 {
		t.Error("teleporting and stacking read only the Effect's own Parts")
	}
	later := []effects.Active{{Slug: "later", Level: 5}}
	if p := cat.ForAttack(later, later, "x", true); !reflect.DeepEqual(p, effects.AttackProfile{}) {
		t.Errorf("a branch folds nothing before it lands = %+v", p)
	}
	_, dead := cat.Fatal(later)
	_, _, counters := cat.CounterOf(later)
	if _, ok := cat.TeleportOf("later"); ok || dead || counters || cat.Incapacitated(later) || cat.Immobile(later) || cat.MoveMultiplier(later) != 1 ||
		cat.ReactionsTo(later, "damaged") != nil || cat.Stacks("later") || !reflect.DeepEqual(cat.LandingOf("later", ""), effects.Landing{}) {
		t.Error("nor does it act on its bearer")
	}
}
