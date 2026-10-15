package effects_test

import (
	"reflect"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
)

func enlarge() effects.Definition {
	return effects.Definition{Slug: "enlarge-reduce", Name: "Enlarge/Reduce", Owner: effects.OwnedBySpell, Concentration: true, Parts: []effects.Part{
		effects.Choice{Modes: []effects.Group{
			{Name: "Enlarge", Parts: []effects.Part{effects.SaveEdge{Ability: "strength", Mode: effects.SaveAdvantage}, effects.ExtraDamage{Dice: "1d4"}}},
			{Name: "Reduce", Parts: []effects.Part{effects.SaveEdge{Ability: "strength", Mode: effects.SaveDisadvantage}, effects.Manual{Instruction: "Weapon damage drops by 1d4."}}},
		}},
		effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 5}, Then: []effects.Part{effects.SaveCondition{Ability: "constitution", Slug: "stunned"}}},
	}}
}

// A Choice counts only its chosen mode; a Branch only when its condition holds.
func TestChoicesAndBranches(t *testing.T) {
	t.Parallel()
	d := enlarge()
	if !reflect.DeepEqual(d.Modes(), []string{"Enlarge", "Reduce"}) || (effects.Definition{}).Modes() != nil {
		t.Fatalf("modes = %v", d.Modes())
	}
	if got := d.PartsIn("Enlarge"); !reflect.DeepEqual(got, []effects.Part{effects.SaveEdge{Ability: "strength", Mode: effects.SaveAdvantage}, effects.ExtraDamage{Dice: "1d4"}}) {
		t.Fatalf("enlarged = %+v", got)
	}
	if got := d.PartsIn(""); got != nil {
		t.Fatalf("no mode chosen = %+v", got)
	}
	cat := effects.Catalog{d.Slug: d}
	if p := cat.ForSave([]effects.Active{{Slug: d.Slug, Mode: "Reduce"}}, "strength"); !reflect.DeepEqual(p.Disadvantages, []string{"Enlarge/Reduce: disadvantage"}) {
		t.Fatalf("reduced save = %+v", p)
	}
	if got := cat.Instructions(d.Slug, d.Name, "Reduce"); !reflect.DeepEqual(got, []string{"Weapon damage drops by 1d4."}) || d.Automated() {
		t.Fatalf("reduce leaves the DM a note = %v", got)
	}
	failedBadly := effects.Situation{Saved: false, Margin: 5}
	if got := d.Branches("Enlarge", failedBadly); !reflect.DeepEqual(got, []effects.Part{effects.SaveCondition{Ability: "constitution", Slug: "stunned"}}) {
		t.Fatalf("failed by 5 = %+v", got)
	}
	if got := d.Branches("Enlarge", effects.Situation{Saved: false, Margin: 4}); got != nil {
		t.Fatalf("failed by 4 = %+v", got)
	}
	nested := effects.Definition{Parts: []effects.Part{
		effects.Choice{Modes: []effects.Group{{Name: "Hot", Parts: []effects.Part{
			effects.Branch{When: effects.Condition{Kind: effects.CreatureIs, Type: "undead"}, Then: []effects.Part{
				effects.ExtraDamage{Dice: "2d6"},
				effects.Branch{When: effects.Condition{Kind: effects.HPAtMost, N: 10}, Then: []effects.Part{effects.Dispel{}}},
			}},
		}}}},
	}}
	if got := nested.Branches("Hot", effects.Situation{Type: "Undead", HP: 8}); !reflect.DeepEqual(got, []effects.Part{effects.ExtraDamage{Dice: "2d6"}, effects.Dispel{}}) {
		t.Fatalf("nested branches = %+v", got)
	}
	if got := nested.Branches("Cold", effects.Situation{Type: "undead"}); got != nil {
		t.Fatalf("another mode's branches = %+v", got)
	}
	if !nested.Automated() || (effects.Definition{Parts: []effects.Part{effects.Branch{Then: []effects.Part{effects.Manual{}}}}}).Automated() {
		t.Fatal("a manual part inside a branch makes the Effect partial")
	}
}

func TestSummons(t *testing.T) {
	t.Parallel()
	owl := effects.Summon{Monster: "owl", Count: 1, Shares: true}
	cat := effects.Catalog{"find-familiar": {Slug: "find-familiar", Parts: []effects.Part{effects.Manual{}, owl}}, "bless": {Slug: "bless"}}
	if got, ok := cat.SummonOf("find-familiar"); !ok || got != owl {
		t.Fatalf("summon = %+v %v", got, ok)
	}
	if got, ok := cat.SummonOf("bless"); ok || got != (effects.Summon{}) {
		t.Fatalf("bless summons = %+v", got)
	}
}

func TestRevealsInAreas(t *testing.T) {
	t.Parallel()
	cat := effects.Catalog{"outline": {Slug: "outline", Parts: []effects.Part{
		effects.Area{Shape: hex.CubeArea, SizeFt: 20, RangeFt: 60}, effects.Reveal{Qualities: []string{"invisible"}}, effects.Reveal{Qualities: []string{"hidden"}},
	}}}
	if a, ok := cat.AreaOf("outline"); !ok || !reflect.DeepEqual(a.Reveals, []string{"invisible", "hidden"}) {
		t.Fatalf("reveals = %+v", a)
	}
}

func TestFormsLand(t *testing.T) {
	t.Parallel()
	cat := effects.Catalog{"polymorph": {Slug: "polymorph", Parts: []effects.Part{effects.Form{Monster: "", TempHP: 0}}}}
	if l := cat.LandingOf("polymorph", ""); l.Form == nil || *l.Form != (effects.Form{}) {
		t.Fatalf("polymorph lands a form = %+v", l)
	}
	cat["outline"] = effects.Definition{Parts: []effects.Part{effects.Reveal{Qualities: []string{"invisible"}}}}
	if l := cat.LandingOf("outline", ""); !reflect.DeepEqual(l.Reveals, []string{"invisible"}) {
		t.Fatalf("outline reveals = %+v", l)
	}
	if l := cat.LandingOf("bless", ""); l.Form != nil {
		t.Fatalf("bless lands no form = %+v", l)
	}
}

func TestConditionsHold(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		when effects.Condition
		at   effects.Situation
		want bool
	}{
		{effects.Condition{Kind: effects.FailsBy, N: 5}, effects.Situation{Saved: false, Margin: 6}, true},
		{effects.Condition{Kind: effects.FailsBy, N: 5}, effects.Situation{Saved: true, Margin: 6}, false},
		{effects.Condition{Kind: effects.HPAtMost, N: 50}, effects.Situation{HP: 50}, true},
		{effects.Condition{Kind: effects.HPAtMost, N: 50}, effects.Situation{HP: 51}, false},
		{effects.Condition{Kind: effects.FirstEachTurn}, effects.Situation{First: true}, true},
		{effects.Condition{Kind: effects.FirstEachTurn}, effects.Situation{First: false}, false},
		{effects.Condition{Kind: effects.CreatureIs, Type: "fiend"}, effects.Situation{Type: "Fiend"}, true},
		{effects.Condition{Kind: effects.CreatureIs, Type: "fiend"}, effects.Situation{Type: "beast"}, false},
		{effects.Condition{Kind: "full_moon"}, effects.Situation{Saved: false, Margin: 99, HP: 0, First: true}, false},
	} {
		if got := c.when.Holds(c.at); got != c.want {
			t.Errorf("%+v in %+v = %v", c.when, c.at, got)
		}
	}
}

func TestDurations(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		d      effects.Duration
		rounds int
		text   string
	}{
		{effects.Duration{Kind: effects.Instant}, 0, "Duration: Instantaneous."},
		{effects.Duration{Kind: effects.Rounds, Amount: 3}, 3, "Duration: 3 rounds."},
		{effects.Duration{Kind: effects.Minutes, Amount: 1}, 10, "Duration: 1 minute."},
		{effects.Duration{Kind: effects.Hours, Amount: 2}, 1200, "Duration: 2 hours."},
		{effects.Duration{Kind: effects.UntilDispelled}, 0, "Duration: Until dispelled."},
		{effects.Duration{Kind: effects.UntilRest}, 0, "Duration: Until the target finishes a Short or Long Rest."},
		{effects.Duration{Kind: effects.Permanent}, 0, "Duration: Permanent."},
		{effects.Duration{Kind: effects.EndOfNextTurn}, 1, "Duration: Until the end of the target's next turn."},
		{effects.Duration{Kind: effects.UntilCured}, 0, "Duration: Until cured."},
		{effects.Duration{}, 0, ""},
	} {
		if got := c.d.Rounds(); got != c.rounds {
			t.Errorf("%+v lasts %d rounds", c.d, got)
		}
		if got := c.d.Text(false); got != c.text {
			t.Errorf("%+v reads %q", c.d, got)
		}
		if c.d.EndsOnRest() != (c.d.Kind == effects.UntilRest) {
			t.Errorf("%+v ends on a rest = %v", c.d, c.d.EndsOnRest())
		}
		// Only what lasts until cured lingers: no rest ends it, and it goes with its bearer from Session to Session.
		if c.d.Lingers() != (c.d.Kind == effects.UntilCured) {
			t.Errorf("%+v lingers = %v", c.d, c.d.Lingers())
		}
	}
	if got := (effects.Duration{Kind: effects.Minutes, Amount: 10}).Text(true); got != "Duration: Concentration, up to 10 minutes." {
		t.Errorf("concentration = %q", got)
	}
}

// Rules text reads every Part, the duration, the repeat save and the scaling.
func TestRulesText(t *testing.T) {
	t.Parallel()
	every := effects.Definition{
		Concentration: true, Duration: effects.Duration{Kind: effects.Minutes, Amount: 1, RepeatSave: "wisdom"},
		Scaling: []effects.Scaling{{Part: 10, Field: "dice", Axis: effects.SlotLevel, Class: "", Rule: effects.Linear{Base: 2, Per: effects.Value{Text: "1d8"}}}},
		Parts: []effects.Part{
			effects.BonusDie{On: []effects.Roll{effects.AttackRolls, effects.SavingThrows}, Dice: "1d4"},
			effects.Edge{Against: false, Advantage: true},
			effects.Edge{Against: true, Advantage: true, Range: effects.WithinFive},
			effects.Edge{Against: true, Advantage: false, Range: effects.BeyondFive, SourceOnly: true},
			effects.ExtraDamage{Dice: "1d6"},
			effects.MoveCost{Multiplier: 2},
			effects.Manual{Instruction: "The DM decides."},
			effects.Area{Shape: hex.SphereArea, SizeFt: 20, RangeFt: 150},
			effects.Area{Shape: hex.ConeArea, SizeFt: 15},
			effects.Area{Shape: hex.EmanationArea, SizeFt: 15},
			effects.SaveDamage{Ability: "dexterity", Dice: "8d6", Type: "fire", Half: true},
			effects.SaveDamage{Ability: "constitution", Dice: "2d8", Type: "thunder"},
			effects.SaveCondition{Ability: "wisdom", Slug: "frightened"},
			effects.CreateSurface{Surface: surface.Grease, Rounds: 1},
			effects.Incapacitated{},
			effects.Immobile{},
			effects.SaveEdge{Ability: "dexterity", Mode: effects.SaveFails},
			effects.SaveEdge{Ability: "strength", Mode: effects.SaveAdvantage},
			effects.SaveEdge{Ability: "charisma", Mode: effects.SaveDisadvantage},
			effects.CritWithin{Feet: 5},
			effects.Exhausting{D20PerLevel: 2, SpeedFtPerLevel: 5, DeathAt: 6, MaxLevel: 0},
			effects.Exhausting{D20PerLevel: 0, SpeedFtPerLevel: 0, DeathAt: 0, MaxLevel: 4},
			effects.Exhausting{D20PerLevel: 1, SpeedFtPerLevel: 0, DeathAt: 0, MaxLevel: 0},
			effects.Exhausting{D20PerLevel: 0, SpeedFtPerLevel: 10, DeathAt: 0, MaxLevel: 3},
			effects.SpeedPenalty{Ft: 10},
			effects.Reacts{Trigger: "damaged", Instruction: "it deals 2d10 fire damage to the attacker."},
			effects.TempHP{Amount: 9},
			effects.Teleport{RangeFt: 30},
			effects.ForcedMove{Ft: 10},
			effects.ForcedMove{Ft: 15, Toward: true},
			effects.Dispel{},
			effects.Counter{RangeFt: 60},
			effects.GrantFeature{Name: "Darkvision"},
			effects.ResourceChange{Resource: "channel-divinity", Delta: 1},
			effects.ResourceChange{Resource: "rage", Delta: -2},
			effects.Choice{Modes: []effects.Group{{Name: "Enlarge", Parts: []effects.Part{effects.TempHP{Amount: 1}}}, {Name: "Reduce", Parts: []effects.Part{effects.Dispel{}}}}},
			effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 5}, Then: []effects.Part{effects.Incapacitated{}}},
			effects.Branch{When: effects.Condition{Kind: effects.HPAtMost, N: 50}, Then: []effects.Part{effects.Dispel{}}},
			effects.Branch{When: effects.Condition{Kind: effects.FirstEachTurn}, Then: []effects.Part{effects.ExtraDamage{Dice: "1d8"}}},
			effects.Branch{When: effects.Condition{Kind: effects.CreatureIs, Type: "undead"}, Then: []effects.Part{effects.Immobile{}}},
			effects.Branch{When: effects.Condition{Kind: effects.CreatureIs, Type: "fiend"}, Then: nil},
			effects.Summon{Monster: "owl", Count: 1, Shares: true},
			effects.Summon{Monster: "skeleton", Count: 2, NeedsCommand: true},
			effects.Summon{Monster: "wolf", Count: 3, Shares: true},
			effects.Summon{Monster: "zombie", Count: 1, NeedsCommand: true},
			effects.Form{Monster: "", TempHP: 0},
			effects.Form{Monster: "owl", TempHP: 5},
			effects.Reveal{Qualities: []string{"invisible", "hidden"}},
		},
	}
	want := []string{
		"Add 1d4 to attack rolls and saving throws.",
		"The target has Advantage on attack rolls.",
		"Attack rolls against the target have Advantage from within 5 feet.",
		"The source's attack rolls against the target have Disadvantage from more than 5 feet away.",
		"The source deals an extra 1d6 damage whenever it hits the target.",
		"Every foot of movement costs 2 feet.",
		"The DM decides.",
		"A 20-foot Sphere appears at a point within 150 feet.",
		"A 15-foot Cone extends from the caster.",
		"A 15-foot Emanation surrounds the caster and moves with it.",
		"Each creature in the area makes a Dexterity saving throw, taking 8d6 fire damage on a failed save or half as much damage on a successful one.",
		"Each creature in the area makes a Constitution saving throw, taking 2d8 thunder damage on a failed save.",
		"On a failed Wisdom saving throw, a creature has the Frightened condition.",
		"The area is covered in grease for 1 round.",
		"The target has the Incapacitated condition.",
		"The target's Speed is 0.",
		"The target automatically fails Dexterity saving throws.",
		"The target has Advantage on Strength saving throws.",
		"The target has Disadvantage on Charisma saving throws.",
		"Any hit against the target from within 5 feet is a Critical Hit.",
		"Each level gives a −2 penalty to D20 Tests and reduces Speed by 5 feet; a creature dies at level 6.",
		"It stacks; it rises to level 4 at most.",
		"Each level gives a −1 penalty to D20 Tests.",
		"Each level reduces Speed by 10 feet; it rises to level 3 at most.",
		"The target's Speed is reduced by 10 feet.",
		"When the target is damaged, it can take a Reaction: it deals 2d10 fire damage to the attacker.",
		"The target gains 9 Temporary Hit Points.",
		"The caster teleports up to 30 feet to an unoccupied space it can see.",
		"A creature that fails is pushed up to 10 feet straight away from the caster.",
		"A creature that fails is pulled up to 15 feet straight toward the caster.",
		"Every spell on the target ends.",
		"As a Reaction, the caster interrupts a creature it can see casting a spell within 60 feet; the spell fails.",
		"The target gains Darkvision.",
		"The target regains 1 use of Channel Divinity.",
		"The target expends 2 uses of Rage.",
		"Choose one: Enlarge: The target gains 1 Temporary Hit Points. Or Reduce: Every spell on the target ends.",
		"If a creature fails the save by 5 or more, the target has the Incapacitated condition.",
		"If a creature has 50 Hit Points or fewer, every spell on the target ends.",
		"The first time on a turn the effect touches a creature, the source deals an extra 1d8 damage whenever it hits the target.",
		"If the creature is an undead, the target's Speed is 0.",
		"If the creature is a fiend, ",
		"The caster summons an Owl, which acts on the caster's turn.",
		"The caster summons 2 Skeleton creatures, which roll their own Initiative. They take the Dodge action unless the caster commands them with a Bonus Action.",
		"The caster summons 3 Wolf creatures, which act on the caster's turn.",
		"The caster summons a Zombie, which rolls its own Initiative. It takes the Dodge action unless the caster commands it with a Bonus Action.",
		"The target takes the form of a creature of the caster's choice, using its statistics and gaining Temporary Hit Points equal to that creature's Hit Point maximum; it reverts when they are gone or the effect ends, and any damage left over carries to its own Hit Points.",
		"The target takes the form of an Owl, using its statistics and gaining 5 Temporary Hit Points; it reverts when they are gone or the effect ends, and any damage left over carries to its own Hit Points.",
		"Every creature and object in the area loses Invisible and Hidden, whatever anyone's senses.",
		"Duration: Concentration, up to 1 minute.",
		"The target repeats the Wisdom saving throw at the end of each of its turns, ending the effect on a success.",
		"Using a Higher-Level Spell Slot: the damage increases by 1d8 for each slot level above 2.",
	}
	got := every.Text()
	if len(got) != len(want) {
		t.Fatalf("text has %d lines, want %d:\n%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
	if got := (effects.Definition{Parts: []effects.Part{effects.ResourceChange{Resource: "ki", Delta: 0}}}).Text(); got[0] != "The target expends 0 uses of Ki." {
		t.Errorf("no change reads %q", got[0])
	}
	if got := (effects.Definition{}).Text(); len(got) != 0 {
		t.Errorf("an empty Effect reads %v", got)
	}
}

// Where each area and structural kind is read: an area reads only the Effect's own Parts, the later
// of two settings wins and reveals and instructions add up; a chosen mode counts for landing and for
// the DM's instructions, and holds choices and branches of its own; Parts beneath a branch count only
// once it lands, and a Manual Part anywhere makes the Effect partial.
func TestWhereEachStructuralKindIsRead(t *testing.T) {
	t.Parallel()
	inMode := []effects.Part{
		effects.Area{Shape: hex.LineArea, SizeFt: 60},
		effects.AreaSave{Ability: "charisma"},
		effects.SaveCondition{Ability: "wisdom", Slug: "charmed"},
		effects.CreateSurface{Surface: surface.Grease, Rounds: 2},
		effects.ForcedMove{Ft: 20, Toward: true},
		effects.Light{BrightFt: 40, DimFt: 40},
		effects.Reveal{Qualities: []string{"secret"}},
		effects.Manual{Instruction: "Sing."},
		effects.Form{Monster: "bear", TempHP: 0},
		effects.Summon{Monster: "bat", Count: 1},
		effects.Choice{Modes: []effects.Group{{Name: "Grim", Parts: []effects.Part{effects.Manual{Instruction: "Deeper."}}}}},
		effects.Branch{When: effects.Condition{Kind: effects.FirstEachTurn}, Then: []effects.Part{effects.SaveCondition{Ability: "dexterity", Slug: "prone"}}},
	}
	beneath := []effects.Part{effects.Reveal{Qualities: []string{"hidden"}}, effects.Manual{Instruction: "Later."}, effects.Form{Monster: "owl"}, effects.Summon{Monster: "rat", Count: 2}}
	cat := effects.Catalog{"storm": {Name: "Storm", Parts: []effects.Part{
		effects.Area{Shape: hex.SphereArea, SizeFt: 10, RangeFt: 60},
		effects.AreaSave{Ability: "strength"},
		effects.SaveCondition{Ability: "constitution", Slug: "stunned"},
		effects.CreateSurface{Surface: surface.Grease, Rounds: 1},
		effects.ForcedMove{Ft: 5},
		effects.Light{BrightFt: 10, DimFt: 10},
		effects.Reveal{Qualities: []string{"invisible"}},
		effects.Manual{Instruction: "Thunder."},
		effects.Area{Shape: hex.CubeArea, SizeFt: 20, RangeFt: 90},
		effects.AreaSave{Ability: "dexterity"},
		effects.CreateSurface{Surface: surface.Grease, Rounds: 3},
		effects.ForcedMove{Ft: 15},
		effects.Light{BrightFt: 20, DimFt: 30},
		effects.Reveal{Qualities: []string{"hidden", "disguised"}},
		effects.Manual{Instruction: "Rain."},
		effects.Form{Monster: "wolf", TempHP: 3},
		effects.Form{Monster: "hawk", TempHP: 1},
		effects.Summon{Monster: "owl", Count: 1, Shares: true},
		effects.Summon{Monster: "cat", Count: 1},
		effects.Choice{Modes: []effects.Group{{Name: "Grim", Parts: inMode}, {Name: "Calm", Parts: nil}}},
		effects.Choice{Modes: []effects.Group{{Name: "Loud", Parts: []effects.Part{effects.Dispel{}}}}},
		effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 1}, Then: beneath},
	}}}

	a, ok := cat.AreaOf("storm")
	want := effects.AreaSpell{
		Name: "Storm", Area: effects.Area{Shape: hex.CubeArea, SizeFt: 20, RangeFt: 90}, Save: "dexterity", Condition: "stunned",
		Surface: effects.CreateSurface{Surface: surface.Grease, Rounds: 3}, Push: effects.ForcedMove{Ft: 15},
		Reveals: []string{"invisible", "hidden", "disguised"}, Instructions: []string{"Thunder.", "Rain."}, Light: effects.Light{BrightFt: 20, DimFt: 30},
	}
	if !ok || a.Name != want.Name || a.Area != want.Area || a.Save != want.Save || a.Condition != want.Condition || a.Surface != want.Surface ||
		a.Push != want.Push || !reflect.DeepEqual(a.Reveals, want.Reveals) || !reflect.DeepEqual(a.Instructions, want.Instructions) || a.Light != want.Light || a.Damage != (effects.RollDamage{}) {
		t.Errorf("the area reads its own Parts, the later winning = %+v", a)
	}
	if a.Lands(effects.Outcome{Saved: false, Damage: 9}) != nil {
		t.Errorf("none of them hurts = %+v", a.Lands(effects.Outcome{Damage: 9}))
	}
	order := effects.Catalog{"order": {Parts: []effects.Part{effects.SaveCondition{Ability: "wisdom", Slug: "frightened"}, effects.AreaSave{Ability: "strength"}, effects.Area{Shape: hex.SphereArea, SizeFt: 5}}}}
	if o, _ := order.AreaOf("order"); o.Save != "strength" || o.Condition != "frightened" {
		t.Errorf("a later save sets the ability, and the condition stays = %+v", o)
	}
	if _, ok := (effects.Catalog{"bare": {Parts: []effects.Part{effects.AreaSave{Ability: "wisdom"}, effects.Light{BrightFt: 5}}}}).AreaOf("bare"); ok {
		t.Error("an Effect without an area is no area spell")
	}

	if got := cat["storm"].Modes(); !reflect.DeepEqual(got, []string{"Grim", "Calm", "Loud"}) {
		t.Errorf("modes of every choice, in order = %v", got)
	}
	grim := cat["storm"].PartsIn("Grim")
	if len(grim) != 30 || !reflect.DeepEqual(grim[19:], append(inMode[:10:10], effects.Manual{Instruction: "Deeper."})) {
		t.Errorf("the chosen mode, with its own choice in the same mode, and no branch = %+v", grim[19:])
	}
	if got := cat["storm"].PartsIn("Calm"); len(got) != 19 {
		t.Errorf("an empty mode adds nothing = %d", len(got))
	}
	land := cat.LandingOf("storm", "Grim")
	if !reflect.DeepEqual(land.Reveals, []string{"invisible", "hidden", "disguised", "secret"}) || land.Form == nil || *land.Form != (effects.Form{Monster: "bear"}) {
		t.Errorf("a chosen mode lands its reveal and the last form = %+v", land)
	}
	if l := cat.LandingOf("storm", ""); l.Form == nil || *l.Form != (effects.Form{Monster: "hawk", TempHP: 1}) || l.TempHP != 0 {
		t.Errorf("the Effect's own last form = %+v", l)
	}
	if got := cat.Instructions("storm", "Storm", "Grim"); !reflect.DeepEqual(got, []string{"Thunder.", "Rain.", "Sing.", "Deeper."}) {
		t.Errorf("instructions of the Effect and its chosen mode, not its branch = %v", got)
	}
	if s, ok := cat.SummonOf("storm"); !ok || s != (effects.Summon{Monster: "owl", Count: 1, Shares: true}) {
		t.Errorf("the first summon of the Effect's own Parts = %+v", s)
	}
	if _, ok := (effects.Catalog{"deep": {Parts: []effects.Part{effects.Choice{Modes: []effects.Group{{Name: "A", Parts: []effects.Part{effects.Summon{Monster: "rat"}}}}}, effects.Branch{Then: beneath}}}}).SummonOf("deep"); ok {
		t.Error("a summon in a mode or a branch is not the Effect's own")
	}
	lands := cat["storm"].Branches("Grim", effects.Situation{Saved: false, Margin: 2, First: true})
	if !reflect.DeepEqual(lands, append([]effects.Part{effects.SaveCondition{Ability: "dexterity", Slug: "prone"}}, beneath...)) {
		t.Errorf("what lands, in the order the Parts come = %+v", lands)
	}
	if got := cat["storm"].Branches("Calm", effects.Situation{Saved: true}); got != nil {
		t.Errorf("a save keeps the branch off = %+v", got)
	}
	automated := effects.Definition{Parts: []effects.Part{effects.Choice{Modes: []effects.Group{{Name: "A", Parts: []effects.Part{effects.Reveal{}, effects.Form{}}}}}, effects.Branch{Then: []effects.Part{effects.Summon{}}}}}
	if !automated.Automated() || cat["storm"].Automated() {
		t.Error("only a Manual Part, wherever it sits, makes an Effect partial")
	}
	if (effects.Definition{Parts: []effects.Part{effects.Choice{Modes: []effects.Group{{Name: "A", Parts: []effects.Part{effects.Choice{Modes: []effects.Group{{Name: "B", Parts: []effects.Part{effects.Manual{}}}}}}}}}}}).Automated() {
		t.Error("a Manual Part in a mode within a mode")
	}
}
