package effects_test

import (
	"reflect"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
)

func enlarge() effects.Definition {
	return effects.Definition{Slug: "enlarge-reduce", Name: "Enlarge/Reduce", Owner: effects.OwnedBySpell, Concentration: true, Components: []effects.Component{
		effects.Choice{Modes: []effects.Mode{
			{Name: "Enlarge", Components: []effects.Component{effects.SaveEdge{Ability: "strength", Mode: effects.SaveAdvantage}, effects.ExtraDamage{Dice: "1d4"}}},
			{Name: "Reduce", Components: []effects.Component{effects.SaveEdge{Ability: "strength", Mode: effects.SaveDisadvantage}, effects.Manual{Instruction: "Weapon damage drops by 1d4."}}},
		}},
		effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 5}, Then: []effects.Component{effects.SaveCondition{Ability: "constitution", Slug: "stunned"}}},
	}}
}

// A Choice counts only its chosen mode; a Branch only when its condition holds.
func TestChoicesAndBranches(t *testing.T) {
	t.Parallel()
	d := enlarge()
	if !reflect.DeepEqual(d.Modes(), []string{"Enlarge", "Reduce"}) || (effects.Definition{}).Modes() != nil {
		t.Fatalf("modes = %v", d.Modes())
	}
	if got := d.Parts("Enlarge"); !reflect.DeepEqual(got, []effects.Component{effects.SaveEdge{Ability: "strength", Mode: effects.SaveAdvantage}, effects.ExtraDamage{Dice: "1d4"}}) {
		t.Fatalf("enlarged = %+v", got)
	}
	if got := d.Parts(""); got != nil {
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
	if got := d.Branches("Enlarge", failedBadly); !reflect.DeepEqual(got, []effects.Component{effects.SaveCondition{Ability: "constitution", Slug: "stunned"}}) {
		t.Fatalf("failed by 5 = %+v", got)
	}
	if got := d.Branches("Enlarge", effects.Situation{Saved: false, Margin: 4}); got != nil {
		t.Fatalf("failed by 4 = %+v", got)
	}
	nested := effects.Definition{Components: []effects.Component{
		effects.Choice{Modes: []effects.Mode{{Name: "Hot", Components: []effects.Component{
			effects.Branch{When: effects.Condition{Kind: effects.CreatureIs, Type: "undead"}, Then: []effects.Component{
				effects.ExtraDamage{Dice: "2d6"},
				effects.Branch{When: effects.Condition{Kind: effects.HPAtMost, N: 10}, Then: []effects.Component{effects.Dispel{}}},
			}},
		}}}},
	}}
	if got := nested.Branches("Hot", effects.Situation{Type: "Undead", HP: 8}); !reflect.DeepEqual(got, []effects.Component{effects.ExtraDamage{Dice: "2d6"}, effects.Dispel{}}) {
		t.Fatalf("nested branches = %+v", got)
	}
	if got := nested.Branches("Cold", effects.Situation{Type: "undead"}); got != nil {
		t.Fatalf("another mode's branches = %+v", got)
	}
	if !nested.Automated() || (effects.Definition{Components: []effects.Component{effects.Branch{Then: []effects.Component{effects.Manual{}}}}}).Automated() {
		t.Fatal("a manual part inside a branch makes the Effect partial")
	}
}

func TestSummons(t *testing.T) {
	t.Parallel()
	owl := effects.Summon{Monster: "owl", Count: 1, Shares: true}
	cat := effects.Catalog{"find-familiar": {Slug: "find-familiar", Components: []effects.Component{effects.Manual{}, owl}}, "bless": {Slug: "bless"}}
	if got, ok := cat.SummonOf("find-familiar"); !ok || got != owl {
		t.Fatalf("summon = %+v %v", got, ok)
	}
	if got, ok := cat.SummonOf("bless"); ok || got != (effects.Summon{}) {
		t.Fatalf("bless summons = %+v", got)
	}
}

func TestRevealsInAreas(t *testing.T) {
	t.Parallel()
	cat := effects.Catalog{"outline": {Slug: "outline", Components: []effects.Component{
		effects.Area{Shape: hex.CubeArea, SizeFt: 20, RangeFt: 60}, effects.Reveal{Qualities: []string{"invisible"}}, effects.Reveal{Qualities: []string{"hidden"}},
	}}}
	if a, ok := cat.AreaOf("outline"); !ok || !reflect.DeepEqual(a.Reveals, []string{"invisible", "hidden"}) {
		t.Fatalf("reveals = %+v", a)
	}
}

func TestFormsLand(t *testing.T) {
	t.Parallel()
	cat := effects.Catalog{"polymorph": {Slug: "polymorph", Components: []effects.Component{effects.Form{Monster: "", TempHP: 0}}}}
	if l := cat.LandingOf("polymorph", ""); l.Form == nil || *l.Form != (effects.Form{}) {
		t.Fatalf("polymorph lands a form = %+v", l)
	}
	cat["outline"] = effects.Definition{Components: []effects.Component{effects.Reveal{Qualities: []string{"invisible"}}}}
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

func TestScaling(t *testing.T) {
	t.Parallel()
	fireball := effects.Scaling{Axis: effects.SlotLevel, Base: 3, Dice: "1d6"}
	for slot, want := range map[int]string{0: "8d6", 3: "8d6", 4: "9d6", 5: "10d6"} {
		if got := fireball.Apply("8d6", effects.Level{Slot: slot}); got != want {
			t.Errorf("fireball at slot %d = %s", slot, got)
		}
	}
	if got := (effects.Scaling{Axis: effects.SlotLevel, Base: 1, Dice: "1d8"}).Apply("2d6", effects.Level{Slot: 2}); got != "2d6+1d8" {
		t.Errorf("unlike dice = %s", got)
	}
	for _, bad := range []string{"x", "2d6", "ad6", "2d6+1"} {
		if got := (effects.Scaling{Axis: effects.SlotLevel, Base: 1, Dice: bad}).Apply("bd6", effects.Level{Slot: 2}); got != "bd6+"+bad {
			t.Errorf("odd dice %q = %s", bad, got)
		}
	}
	cantrip := effects.Scaling{Axis: effects.CharacterLevel, Steps: []effects.Step{{At: 5, Dice: "2d10"}, {At: 11, Dice: "3d10"}, {At: 17, Dice: "4d10"}}}
	for level, want := range map[int]string{1: "1d10", 4: "1d10", 5: "2d10", 10: "2d10", 11: "3d10", 17: "4d10", 20: "4d10"} {
		if got := cantrip.Apply("1d10", effects.Level{Character: level}); got != want {
			t.Errorf("cantrip at %d = %s", level, got)
		}
	}
	aura := effects.Scaling{Axis: effects.ClassLevel, Class: "paladin", Steps: []effects.Step{{At: 18, Dice: "2d8"}}}
	if aura.Apply("1d8", effects.Level{Character: 20, Classes: map[string]int{"paladin": 17}}) != "1d8" || aura.Apply("1d8", effects.Level{Classes: map[string]int{"paladin": 18}}) != "2d8" {
		t.Error("class levels count only that class")
	}
	sneak := effects.Scaling{Axis: effects.TableColumn, Class: "rogue", Column: "Sneak Attack"}
	if sneak.Apply("1d6", effects.Level{Columns: map[string]string{"Sneak Attack": "3d6"}}) != "3d6" || sneak.Apply("1d6", effects.Level{}) != "1d6" {
		t.Error("a table column replaces the dice when it has a value")
	}
	if (effects.Scaling{Axis: "phase_of_moon", Dice: "1d6", Base: 0}).Apply("1d4", effects.Level{Slot: 9}) != "1d4" {
		t.Error("an unknown axis scales nothing")
	}
	for s, want := range map[*effects.Scaling]string{
		&fireball: "Using a Higher-Level Spell Slot: the damage increases by 1d6 for each slot level above 3.",
		&cantrip:  "Cantrip Upgrade: the damage becomes 2d10 at level 5, 3d10 at level 11, 4d10 at level 17.",
		&aura:     "The damage becomes 2d8 at Paladin level 18.",
		&sneak:    "The damage equals the Sneak Attack column of the Rogue table.",
	} {
		if got := s.Text(); got != want {
			t.Errorf("scaling reads %q", got)
		}
	}
}

// Rules text reads every component, the duration, the repeat save and the scaling.
func TestRulesText(t *testing.T) {
	t.Parallel()
	every := effects.Definition{
		Concentration: true, Duration: effects.Duration{Kind: effects.Minutes, Amount: 1, RepeatSave: "wisdom"},
		Scaling: &effects.Scaling{Axis: effects.SlotLevel, Base: 2, Dice: "1d8"},
		Components: []effects.Component{
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
			effects.CreateSurface{Kind: surface.Grease, Rounds: 1},
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
			effects.Choice{Modes: []effects.Mode{{Name: "Enlarge", Components: []effects.Component{effects.TempHP{Amount: 1}}}, {Name: "Reduce", Components: []effects.Component{effects.Dispel{}}}}},
			effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 5}, Then: []effects.Component{effects.Incapacitated{}}},
			effects.Branch{When: effects.Condition{Kind: effects.HPAtMost, N: 50}, Then: []effects.Component{effects.Dispel{}}},
			effects.Branch{When: effects.Condition{Kind: effects.FirstEachTurn}, Then: []effects.Component{effects.ExtraDamage{Dice: "1d8"}}},
			effects.Branch{When: effects.Condition{Kind: effects.CreatureIs, Type: "undead"}, Then: []effects.Component{effects.Immobile{}}},
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
	if got := (effects.Definition{Components: []effects.Component{effects.ResourceChange{Resource: "ki", Delta: 0}}}).Text(); got[0] != "The target expends 0 uses of Ki." {
		t.Errorf("no change reads %q", got[0])
	}
	if got := (effects.Definition{}).Text(); len(got) != 0 {
		t.Errorf("an empty Effect reads %v", got)
	}
}
