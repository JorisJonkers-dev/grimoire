package effects_test

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
)

// Every kind is listed once, in slug order, and describes the fields an author fills in; only a kind
// that is the whole of its Part has none.
func TestKindsAreListedOnceInOrder(t *testing.T) {
	t.Parallel()
	var slugs []string
	for _, k := range effects.Kinds() {
		d := k.Describe()
		bare := slices.Contains([]string{"choice", "dispel", "immobile", "incapacitated"}, d.Slug)
		if d.Label == "" || d.Summary == "" || (len(d.Fields) == 0) != bare {
			t.Errorf("%s describes too little or too much: %+v", d.Slug, d)
		}
		for _, f := range d.Fields {
			if f.Name == "" || f.Label == "" || f.Type == "" {
				t.Errorf("%s has a field without a name, label or type: %+v", d.Slug, f)
			}
		}
		if (d.Slug != "branch" && d.Groups != nil) || (d.Slug == "choice") != d.OpenGroups {
			t.Errorf("only a branch holds Parts beneath it in a group of its own, and only a choice in groups the author names = %+v", d)
		}
		slugs = append(slugs, d.Slug)
	}
	want := []string{
		"area", "area_save", "bonus_die", "branch", "choice", "counter", "create_surface", "crit_within", "dispel", "edge", "exhausting",
		"extra_damage", "forced_move", "form", "grant_feature", "immobile", "incapacitated", "light", "manual", "move_cost", "reacts",
		"resource_change", "reveal", "save_condition", "save_damage", "save_edge", "speed_penalty", "summon", "teleport", "temp_hp",
	}
	if !reflect.DeepEqual(slugs, want) {
		t.Fatalf("kinds = %v", slugs)
	}
	if branch := kind(t, "branch").Describe(); !reflect.DeepEqual(branch.Groups, []string{"then"}) {
		t.Fatalf("a branch holds Parts under then = %+v", branch)
	}
	ability := kind(t, "save_damage").Describe().Fields[0]
	if ability.Type != effects.ChoiceField || !ability.Required || !slices.Contains(ability.Options, "dexterity") {
		t.Fatalf("a save names its ability = %+v", ability)
	}
	if mult := kind(t, "move_cost").Describe().Fields[0]; mult.Type != effects.NumberField || mult.Min != 1 || mult.Max != 4 {
		t.Fatalf("a move cost is bounded as its table is = %+v", mult)
	}
	if shape := kind(t, "area").Describe().Fields[0]; shape.Type != effects.ChoiceField || len(shape.Options) != 8 || !slices.Contains(shape.Options, "wall") {
		t.Fatalf("an area names its shape = %+v", shape)
	}
	if q := kind(t, "reveal").Describe().Fields[0]; q.Type != effects.ChoicesField || !reflect.DeepEqual(q.Options, []string{"hidden", "invisible", "disguised", "illusory", "ethereal", "darkness", "heavy", "secret"}) {
		t.Fatalf("a reveal strips Visibility Qualities = %+v", q)
	}
	if note := kind(t, "manual").Describe(); note.Label != "Manual Effect" {
		t.Fatalf("a manual Part is a Manual Effect = %+v", note)
	}
}

// kind finds a kind's definition by slug.
func kind(t *testing.T, slug string) effects.Kind {
	t.Helper()
	for _, k := range effects.Kinds() {
		if k.Describe().Slug == slug {
			return k
		}
	}
	t.Fatalf("no kind %s", slug)
	return nil
}

// Authored values become the kind's own Part, which knows its kind and reads as rules text.
func TestKindsDecodeAuthoredValues(t *testing.T) {
	t.Parallel()
	stun := effects.SaveCondition{Ability: "constitution", Slug: "stunned"}
	for _, c := range []struct {
		kind   string
		values effects.Values
		groups effects.Groups
		want   effects.Part
		text   string
	}{
		{
			"bonus_die",
			effects.Values{"dice": {Text: "1d4"}, "on": {Texts: []string{"saving_throws", "attack_rolls"}}},
			nil,
			effects.BonusDie{On: []effects.Roll{effects.AttackRolls, effects.SavingThrows}, Dice: "1d4"},
			"Add 1d4 to attack rolls and saving throws.",
		},
		{
			"bonus_die",
			effects.Values{"dice": {Text: "1d6"}, "on": {Texts: []string{"saving_throws"}}},
			nil,
			effects.BonusDie{On: []effects.Roll{effects.SavingThrows}, Dice: "1d6"},
			"Add 1d6 to saving throws.",
		},
		{
			"save_damage",
			effects.Values{"ability": {Text: "dexterity"}, "dice": {Text: "8d6"}, "damage_type": {Text: "fire"}, "half": {Flag: true}},
			nil,
			effects.SaveDamage{Ability: "dexterity", Dice: "8d6", Type: "fire", Half: true},
			"Each creature in the area makes a Dexterity saving throw, taking 8d6 fire damage on a failed save or half as much damage on a successful one.",
		},
		{
			"branch",
			effects.Values{"when": {Text: "fails_by"}, "n": {Number: 5}},
			effects.Groups{{Name: "then", Parts: []effects.Part{stun}}},
			effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 5}, Then: []effects.Part{stun}},
			"If a creature fails the save by 5 or more, on a failed Constitution saving throw, a creature has the Stunned condition.",
		},
		{
			"branch",
			effects.Values{"when": {Text: "creature_is"}, "creature_type": {Text: "undead"}},
			effects.Groups{{Name: "then", Parts: []effects.Part{effects.Dispel{}}}},
			effects.Branch{When: effects.Condition{Kind: effects.CreatureIs, Type: "undead"}, Then: []effects.Part{effects.Dispel{}}},
			"If the creature is an undead, every spell on the target ends.",
		},
		{
			"counter",
			effects.Values{"range_ft": {Number: 60}},
			nil,
			effects.Counter{RangeFt: 60},
			"As a Reaction, the caster interrupts a creature it can see casting a spell within 60 feet; the spell fails.",
		},
		{"crit_within", effects.Values{"feet": {Number: 5}}, nil, effects.CritWithin{Feet: 5}, "Any hit against the target from within 5 feet is a Critical Hit."},
		{"dispel", nil, nil, effects.Dispel{}, "Every spell on the target ends."},
		{
			"edge",
			effects.Values{"against": {Flag: true}, "advantage": {Flag: true}, "range": {Text: "within_five"}, "source_only": {Flag: true}},
			nil,
			effects.Edge{Against: true, Advantage: true, Range: effects.WithinFive, SourceOnly: true},
			"The source's attack rolls against the target have Advantage from within 5 feet.",
		},
		{
			"edge",
			effects.Values{"range": {Text: "beyond_five"}},
			nil,
			effects.Edge{Range: effects.BeyondFive},
			"The target has Disadvantage on attack rolls from more than 5 feet away.",
		},
		{
			"edge",
			effects.Values{"against": {Flag: true}, "range": {Text: "any"}},
			nil,
			effects.Edge{Against: true, Range: effects.AnyRange},
			"Attack rolls against the target have Disadvantage.",
		},
		{
			"exhausting",
			effects.Values{"d20_per_level": {Number: 2}, "speed_ft_per_level": {Number: 5}, "death_at": {Number: 6}, "max_level": {Number: 6}},
			nil,
			effects.Exhausting{D20PerLevel: 2, SpeedFtPerLevel: 5, DeathAt: 6, MaxLevel: 6},
			"Each level gives a −2 penalty to D20 Tests and reduces Speed by 5 feet; a creature dies at level 6; it rises to level 6 at most.",
		},
		{"extra_damage", effects.Values{"dice": {Text: "1d6"}}, nil, effects.ExtraDamage{Dice: "1d6"}, "The source deals an extra 1d6 damage whenever it hits the target."},
		{"grant_feature", effects.Values{"name": {Text: "Darkvision"}}, nil, effects.GrantFeature{Name: "Darkvision"}, "The target gains Darkvision."},
		{"immobile", nil, nil, effects.Immobile{}, "The target's Speed is 0."},
		{"incapacitated", nil, nil, effects.Incapacitated{}, "The target has the Incapacitated condition."},
		{"move_cost", effects.Values{"multiplier": {Number: 2}}, nil, effects.MoveCost{Multiplier: 2}, "Every foot of movement costs 2 feet."},
		{
			"reacts",
			effects.Values{"trigger": {Text: "damaged"}, "instruction": {Text: "it hisses."}},
			nil,
			effects.Reacts{Trigger: "damaged", Instruction: "it hisses."},
			"When the target is damaged, it can take a Reaction: it hisses.",
		},
		{
			"resource_change",
			effects.Values{"resource": {Text: "rage"}, "delta": {Number: -2}},
			nil,
			effects.ResourceChange{Resource: "rage", Delta: -2},
			"The target expends 2 uses of Rage.",
		},
		{
			"save_edge",
			effects.Values{"ability": {Text: "dexterity"}, "mode": {Text: "fail"}},
			nil,
			effects.SaveEdge{Ability: "dexterity", Mode: effects.SaveFails},
			"The target automatically fails Dexterity saving throws.",
		},
		{"speed_penalty", effects.Values{"ft": {Number: 10}}, nil, effects.SpeedPenalty{Ft: 10}, "The target's Speed is reduced by 10 feet."},
		{"teleport", effects.Values{"range_ft": {Number: 30}}, nil, effects.Teleport{RangeFt: 30}, "The caster teleports up to 30 feet to an unoccupied space it can see."},
		{"temp_hp", effects.Values{"amount": {Number: 9}}, nil, effects.TempHP{Amount: 9}, "The target gains 9 Temporary Hit Points."},
		{
			"area",
			effects.Values{"shape": {Text: "sphere"}, "size_ft": {Number: 20}, "range_ft": {Number: 150}},
			nil,
			effects.Area{Shape: hex.SphereArea, SizeFt: 20, RangeFt: 150},
			"A 20-foot Sphere appears at a point within 150 feet.",
		},
		{"area_save", effects.Values{"ability": {Text: "wisdom"}}, nil, effects.AreaSave{Ability: "wisdom"}, "Each creature in the area makes a Wisdom saving throw."},
		{
			"choice",
			nil,
			effects.Groups{{Name: "Enlarge", Parts: []effects.Part{effects.TempHP{Amount: 1}}}, {Name: "Reduce", Parts: []effects.Part{effects.Dispel{}, effects.Immobile{}}}},
			effects.Choice{Modes: []effects.Group{{Name: "Enlarge", Parts: []effects.Part{effects.TempHP{Amount: 1}}}, {Name: "Reduce", Parts: []effects.Part{effects.Dispel{}, effects.Immobile{}}}}},
			"Choose one: Enlarge: The target gains 1 Temporary Hit Points. Or Reduce: Every spell on the target ends. The target's Speed is 0.",
		},
		{
			"create_surface",
			effects.Values{"surface": {Text: "plant-growth"}, "rounds": {Number: 10}},
			nil,
			effects.CreateSurface{Surface: surface.PlantGrowth, Rounds: 10},
			"The area is covered in plant-growth for 10 rounds.",
		},
		{"forced_move", effects.Values{"ft": {Number: 10}}, nil, effects.ForcedMove{Ft: 10}, "A creature that fails is pushed up to 10 feet straight away from the caster."},
		{"forced_move", effects.Values{"ft": {Number: 15}, "toward": {Flag: true}}, nil, effects.ForcedMove{Ft: 15, Toward: true}, "A creature that fails is pulled up to 15 feet straight toward the caster."},
		{
			"form",
			effects.Values{"monster": {Text: "owl"}, "temp_hp": {Number: 5}},
			nil,
			effects.Form{Monster: "owl", TempHP: 5},
			"The target takes the form of an Owl, using its statistics and gaining 5 Temporary Hit Points; it reverts when they are gone or the effect ends, and any damage left over carries to its own Hit Points.",
		},
		{
			"form",
			nil,
			nil,
			effects.Form{},
			"The target takes the form of a creature of the caster's choice, using its statistics and gaining Temporary Hit Points equal to that creature's Hit Point maximum; it reverts when they are gone or the effect ends, and any damage left over carries to its own Hit Points.",
		},
		{
			"light",
			effects.Values{"bright_ft": {Number: 20}, "dim_ft": {Number: 20}},
			nil,
			effects.Light{BrightFt: 20, DimFt: 20},
			"Bright light fills 20 feet around where it lands, and dim light another 20 feet.",
		},
		{"manual", effects.Values{"instruction": {Text: "The DM decides."}}, nil, effects.Manual{Instruction: "The DM decides."}, "The DM decides."},
		{
			"reveal",
			effects.Values{"qualities": {Texts: []string{"invisible", "hidden"}}},
			nil,
			effects.Reveal{Qualities: []string{"invisible", "hidden"}},
			"Every creature and object in the area loses Invisible and Hidden, whatever anyone's senses.",
		},
		{
			"save_condition",
			effects.Values{"ability": {Text: "wisdom"}, "condition": {Text: "frightened"}},
			nil,
			effects.SaveCondition{Ability: "wisdom", Slug: "frightened"},
			"On a failed Wisdom saving throw, a creature has the Frightened condition.",
		},
		{
			"summon",
			effects.Values{"monster": {Text: "skeleton"}, "count": {Number: 2}, "needs_command": {Flag: true}},
			nil,
			effects.Summon{Monster: "skeleton", Count: 2, NeedsCommand: true},
			"The caster summons 2 Skeleton creatures, which roll their own Initiative. They take the Dodge action unless the caster commands them with a Bonus Action.",
		},
		{
			"summon",
			effects.Values{"monster": {Text: "owl"}, "count": {Number: 1}, "shares": {Flag: true}},
			nil,
			effects.Summon{Monster: "owl", Count: 1, Shares: true},
			"The caster summons an Owl, which acts on the caster's turn.",
		},
	} {
		got, err := kind(t, c.kind).Decode(c.values, c.groups)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s %v = %+v, %v", c.kind, c.values, got, err)
			continue
		}
		if got.Kind() != c.kind || got.Text() != c.text {
			t.Errorf("%s reads %q as kind %q", c.kind, got.Text(), got.Kind())
		}
	}
}

// A kind refuses values its rules cannot hold, naming the field.
func TestKindsRefuseValuesTheyCannotHold(t *testing.T) {
	t.Parallel()
	then := effects.Groups{{Name: "then", Parts: []effects.Part{effects.Dispel{}}}}
	for _, c := range []struct {
		kind, field string
		values      effects.Values
		groups      effects.Groups
	}{
		{"bonus_die", "dice", effects.Values{"dice": {Text: "lots"}, "on": {Texts: []string{"attack_rolls"}}}, nil},
		{"bonus_die", "on", effects.Values{"dice": {Text: "1d4"}}, nil},
		{"bonus_die", "on", effects.Values{"dice": {Text: "1d4"}, "on": {Texts: []string{"attack_rolls", "checks"}}}, nil},
		{"bonus_die", "on", effects.Values{"dice": {Text: "1d4"}, "on": {Texts: []string{"attack_rolls", "attack_rolls"}}}, nil},
		{"save_damage", "ability", effects.Values{"ability": {Text: "luck"}, "dice": {Text: "8d6"}, "damage_type": {Text: "fire"}}, nil},
		{"save_damage", "dice", effects.Values{"ability": {Text: "wisdom"}, "dice": {Text: "8d7"}, "damage_type": {Text: "fire"}}, nil},
		{"save_damage", "damage_type", effects.Values{"ability": {Text: "wisdom"}, "dice": {Text: "8d6"}, "damage_type": {Text: "sadness"}}, nil},
		{"branch", "when", effects.Values{"when": {Text: "full_moon"}}, then},
		{"branch", "n", effects.Values{"when": {Text: "hp_at_most"}, "n": {Number: -1}}, then},
		{"branch", "n", effects.Values{"when": {Text: "hp_at_most"}, "n": {Number: 1001}}, then},
		{"branch", "creature_type", effects.Values{"when": {Text: "creature_is"}}, then},
		{"branch", "creature_type", effects.Values{"when": {Text: "creature_is"}, "creature_type": {Text: strings.Repeat("x", 41)}}, then},
		{"branch", "then", effects.Values{"when": {Text: "first_each_turn"}}, nil},
		{"counter", "range_ft", effects.Values{"range_ft": {Number: 4}}, nil},
		{"counter", "range_ft", effects.Values{"range_ft": {Number: 5281}}, nil},
		{"crit_within", "feet", effects.Values{"feet": {Number: -1}}, nil},
		{"crit_within", "feet", effects.Values{"feet": {Number: 121}}, nil},
		{"edge", "range", effects.Values{"range": {Text: "far"}}, nil},
		{"edge", "range", effects.Values{}, nil},
		{"edge", "source_only", effects.Values{"range": {Text: "any"}, "source_only": {Flag: true}}, nil},
		{"exhausting", "d20_per_level", effects.Values{"d20_per_level": {Number: 11}}, nil},
		{"exhausting", "speed_ft_per_level", effects.Values{"speed_ft_per_level": {Number: 31}}, nil},
		{"exhausting", "death_at", effects.Values{"death_at": {Number: -1}}, nil},
		{"exhausting", "max_level", effects.Values{"max_level": {Number: 11}}, nil},
		{"extra_damage", "dice", effects.Values{"dice": {Text: "1d7"}}, nil},
		{"grant_feature", "name", effects.Values{"name": {Text: ""}}, nil},
		{"grant_feature", "name", effects.Values{"name": {Text: strings.Repeat("é", 121)}}, nil},
		{"move_cost", "multiplier", effects.Values{"multiplier": {Number: 0}}, nil},
		{"move_cost", "multiplier", effects.Values{"multiplier": {Number: 5}}, nil},
		{"reacts", "trigger", effects.Values{"trigger": {Text: "hit"}, "instruction": {Text: "x"}}, nil},
		{"reacts", "instruction", effects.Values{"trigger": {Text: "damaged"}}, nil},
		{"reacts", "instruction", effects.Values{"trigger": {Text: "damaged"}, "instruction": {Text: strings.Repeat("x", 501)}}, nil},
		{"resource_change", "resource", effects.Values{"delta": {Number: 1}}, nil},
		{"resource_change", "resource", effects.Values{"resource": {Text: strings.Repeat("x", 81)}, "delta": {Number: 1}}, nil},
		{"resource_change", "delta", effects.Values{"resource": {Text: "ki"}, "delta": {Number: 0}}, nil},
		{"resource_change", "delta", effects.Values{"resource": {Text: "ki"}, "delta": {Number: -21}}, nil},
		{"resource_change", "delta", effects.Values{"resource": {Text: "ki"}, "delta": {Number: 21}}, nil},
		{"save_edge", "ability", effects.Values{"ability": {Text: "luck"}, "mode": {Text: "fail"}}, nil},
		{"save_edge", "mode", effects.Values{"ability": {Text: "wisdom"}, "mode": {Text: "sometimes"}}, nil},
		{"speed_penalty", "ft", effects.Values{"ft": {Number: -1}}, nil},
		{"speed_penalty", "ft", effects.Values{"ft": {Number: 121}}, nil},
		{"teleport", "range_ft", effects.Values{"range_ft": {Number: 4}}, nil},
		{"teleport", "range_ft", effects.Values{"range_ft": {Number: 5281}}, nil},
		{"temp_hp", "amount", effects.Values{"amount": {Number: 0}}, nil},
		{"temp_hp", "amount", effects.Values{"amount": {Number: 501}}, nil},
		{"area", "shape", effects.Values{"shape": {Text: "pyramid"}, "size_ft": {Number: 10}}, nil},
		{"area", "size_ft", effects.Values{"shape": {Text: "cube"}, "size_ft": {Number: 4}}, nil},
		{"area", "size_ft", effects.Values{"shape": {Text: "cube"}, "size_ft": {Number: 501}}, nil},
		{"area", "range_ft", effects.Values{"shape": {Text: "cube"}, "size_ft": {Number: 10}, "range_ft": {Number: -1}}, nil},
		{"area", "range_ft", effects.Values{"shape": {Text: "cube"}, "size_ft": {Number: 10}, "range_ft": {Number: 5281}}, nil},
		{"area_save", "ability", effects.Values{"ability": {Text: "luck"}}, nil},
		{"choice", "modes", nil, nil},
		{"choice", "modes", nil, effects.Groups{{Name: "", Parts: []effects.Part{effects.Dispel{}}}}},
		{"choice", "modes", nil, effects.Groups{{Name: strings.Repeat("é", 81), Parts: []effects.Part{effects.Dispel{}}}}},
		{"choice", "modes", nil, effects.Groups{{Name: "Hot", Parts: []effects.Part{effects.Dispel{}}}, {Name: "Hot", Parts: []effects.Part{effects.Immobile{}}}}},
		{"choice", "modes", nil, effects.Groups{{Name: "Hot", Parts: []effects.Part{effects.Dispel{}}}, {Name: "Cold", Parts: nil}}},
		{"create_surface", "surface", effects.Values{"surface": {Text: "Lava"}, "rounds": {Number: 1}}, nil},
		{"create_surface", "surface", effects.Values{"surface": {Text: ""}, "rounds": {Number: 1}}, nil},
		{"create_surface", "surface", effects.Values{"surface": {Text: "a" + strings.Repeat("b", 40)}, "rounds": {Number: 1}}, nil},
		{"create_surface", "rounds", effects.Values{"surface": {Text: "fire"}, "rounds": {Number: 0}}, nil},
		{"create_surface", "rounds", effects.Values{"surface": {Text: "fire"}, "rounds": {Number: 1001}}, nil},
		{"forced_move", "ft", effects.Values{"ft": {Number: 4}}, nil},
		{"forced_move", "ft", effects.Values{"ft": {Number: 121}}, nil},
		{"form", "monster", effects.Values{"monster": {Text: strings.Repeat("é", 81)}}, nil},
		{"form", "temp_hp", effects.Values{"temp_hp": {Number: -1}}, nil},
		{"form", "temp_hp", effects.Values{"temp_hp": {Number: 1000}}, nil},
		{"light", "bright_ft", effects.Values{"bright_ft": {Number: 4}}, nil},
		{"light", "bright_ft", effects.Values{"bright_ft": {Number: 121}}, nil},
		{"light", "dim_ft", effects.Values{"bright_ft": {Number: 5}, "dim_ft": {Number: -1}}, nil},
		{"light", "dim_ft", effects.Values{"bright_ft": {Number: 5}, "dim_ft": {Number: 121}}, nil},
		{"manual", "instruction", effects.Values{}, nil},
		{"manual", "instruction", effects.Values{"instruction": {Text: strings.Repeat("x", 501)}}, nil},
		{"reveal", "qualities", effects.Values{}, nil},
		{"reveal", "qualities", effects.Values{"qualities": {Texts: []string{"shiny"}}}, nil},
		{"reveal", "qualities", effects.Values{"qualities": {Texts: []string{"hidden", "hidden"}}}, nil},
		{"save_condition", "ability", effects.Values{"ability": {Text: "luck"}, "condition": {Text: "prone"}}, nil},
		{"save_condition", "condition", effects.Values{"ability": {Text: "wisdom"}}, nil},
		{"save_condition", "condition", effects.Values{"ability": {Text: "wisdom"}, "condition": {Text: strings.Repeat("x", 81)}}, nil},
		{"summon", "monster", effects.Values{"count": {Number: 1}}, nil},
		{"summon", "monster", effects.Values{"monster": {Text: strings.Repeat("x", 81)}, "count": {Number: 1}}, nil},
		{"summon", "count", effects.Values{"monster": {Text: "wolf"}, "count": {Number: 0}}, nil},
		{"summon", "count", effects.Values{"monster": {Text: "wolf"}, "count": {Number: 21}}, nil},
	} {
		_, err := kind(t, c.kind).Decode(c.values, c.groups)
		var bad effects.ValueError
		if !errors.As(err, &bad) || bad.Field != c.field || bad.Reason == "" || !strings.HasPrefix(err.Error(), c.field+": ") {
			t.Errorf("%s %v = %v, want a refusal of %s", c.kind, c.values, err, c.field)
		}
	}
	for _, c := range []struct {
		kind   string
		values effects.Values
	}{
		{"counter", effects.Values{"range_ft": {Number: 5}}},
		{"counter", effects.Values{"range_ft": {Number: 5280}}},
		{"crit_within", effects.Values{"feet": {Number: 0}}},
		{"crit_within", effects.Values{"feet": {Number: 120}}},
		{"edge", effects.Values{"against": {Flag: true}, "range": {Text: "any"}, "source_only": {Flag: true}}},
		{"exhausting", effects.Values{"d20_per_level": {Number: 10}, "speed_ft_per_level": {Number: 30}, "death_at": {Number: 10}, "max_level": {Number: 10}}},
		{"exhausting", effects.Values{}},
		{"grant_feature", effects.Values{"name": {Text: strings.Repeat("é", 120)}}},
		{"move_cost", effects.Values{"multiplier": {Number: 1}}},
		{"move_cost", effects.Values{"multiplier": {Number: 4}}},
		{"reacts", effects.Values{"trigger": {Text: "damaged"}, "instruction": {Text: strings.Repeat("x", 500)}}},
		{"resource_change", effects.Values{"resource": {Text: strings.Repeat("x", 80)}, "delta": {Number: -20}}},
		{"resource_change", effects.Values{"resource": {Text: "ki"}, "delta": {Number: 20}}},
		{"save_edge", effects.Values{"ability": {Text: "charisma"}, "mode": {Text: "advantage"}}},
		{"speed_penalty", effects.Values{"ft": {Number: 0}}},
		{"speed_penalty", effects.Values{"ft": {Number: 120}}},
		{"teleport", effects.Values{"range_ft": {Number: 5}}},
		{"teleport", effects.Values{"range_ft": {Number: 5280}}},
		{"temp_hp", effects.Values{"amount": {Number: 1}}},
		{"temp_hp", effects.Values{"amount": {Number: 500}}},
		{"area", effects.Values{"shape": {Text: "wall"}, "size_ft": {Number: 5}, "range_ft": {Number: 0}}},
		{"area", effects.Values{"shape": {Text: "line"}, "size_ft": {Number: 500}, "range_ft": {Number: 5280}}},
		{"create_surface", effects.Values{"surface": {Text: "a" + strings.Repeat("b", 39)}, "rounds": {Number: 1000}}},
		{"create_surface", effects.Values{"surface": {Text: "w"}, "rounds": {Number: 1}}},
		{"forced_move", effects.Values{"ft": {Number: 5}}},
		{"forced_move", effects.Values{"ft": {Number: 120}}},
		{"form", effects.Values{"monster": {Text: strings.Repeat("é", 80)}, "temp_hp": {Number: 999}}},
		{"light", effects.Values{"bright_ft": {Number: 5}, "dim_ft": {Number: 0}}},
		{"light", effects.Values{"bright_ft": {Number: 120}, "dim_ft": {Number: 120}}},
		{"manual", effects.Values{"instruction": {Text: strings.Repeat("x", 500)}}},
		{"reveal", effects.Values{"qualities": {Texts: []string{"hidden", "invisible", "disguised", "illusory", "ethereal", "darkness", "heavy", "secret"}}}},
		{"save_condition", effects.Values{"ability": {Text: "wisdom"}, "condition": {Text: strings.Repeat("x", 80)}}},
		{"summon", effects.Values{"monster": {Text: strings.Repeat("x", 80)}, "count": {Number: 20}}},
	} {
		if _, err := kind(t, c.kind).Decode(c.values, nil); err != nil {
			t.Errorf("%s %v is at its bounds = %v", c.kind, c.values, err)
		}
	}
	edge := effects.Values{"when": {Text: "hp_at_most"}, "n": {Number: 1000}}
	if _, err := kind(t, "branch").Decode(edge, then); err != nil {
		t.Errorf("1000 hit points is the most a branch tests = %v", err)
	}
	edge = effects.Values{"when": {Text: "creature_is"}, "creature_type": {Text: strings.Repeat("x", 40)}}
	if _, err := kind(t, "branch").Decode(edge, then); err != nil {
		t.Errorf("a creature type runs to 40 letters = %v", err)
	}
	one := effects.Groups{{Name: strings.Repeat("é", 80), Parts: []effects.Part{effects.Dispel{}}}}
	if _, err := kind(t, "choice").Decode(nil, one); err != nil {
		t.Errorf("one mode of up to 80 letters is a choice = %v", err)
	}
}

// An area Part asks for what the area's plan needs once; a Part that lands on its bearer says what it
// does there; a choice gives the Parts of the mode chosen; each in acts from the closed set.
func TestAreaAndStructuralPartsActThroughTheirCapabilities(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		part effects.Asker
		want []effects.Act
	}{
		{effects.Area{Shape: hex.ConeArea, SizeFt: 15}, []effects.Act{effects.Template{Area: effects.Area{Shape: hex.ConeArea, SizeFt: 15}}}},
		{effects.AreaSave{Ability: "wisdom"}, []effects.Act{effects.RollSave{Ability: "wisdom"}}},
		{effects.SaveCondition{Ability: "dexterity", Slug: "prone"}, []effects.Act{effects.RollSave{Ability: "dexterity"}, effects.Afflict{Condition: "prone"}}},
		{effects.CreateSurface{Surface: surface.Grease, Rounds: 10}, []effects.Act{effects.LaySurface{Surface: surface.Grease, Rounds: 10}}},
		{effects.ForcedMove{Ft: 10, Toward: true}, []effects.Act{effects.Push{Ft: 10, Toward: true}}},
		{effects.Light{BrightFt: 20, DimFt: 40}, []effects.Act{effects.ShedLight{BrightFt: 20, DimFt: 40}}},
		{effects.Reveal{Qualities: []string{"invisible"}}, []effects.Act{effects.StripQualities{Qualities: []string{"invisible"}}}},
		{effects.Manual{Instruction: "Ask."}, []effects.Act{effects.AskDM{Instruction: "Ask."}}},
	} {
		if got := c.part.Asks(); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%+v asks %+v", c.part, got)
		}
		if _, hurts := c.part.(effects.Lander); hurts {
			t.Errorf("%+v asks the area's plan only, and lands nothing per creature", c.part)
		}
	}
	for _, c := range []struct {
		part effects.BearerLander
		want []effects.Act
	}{
		{effects.SaveCondition{Ability: "dexterity", Slug: "prone"}, []effects.Act{effects.Afflict{Condition: "prone"}}},
		{effects.Reveal{Qualities: []string{"hidden"}}, []effects.Act{effects.StripQualities{Qualities: []string{"hidden"}}}},
		{effects.Manual{Instruction: "Ask."}, []effects.Act{effects.AskDM{Instruction: "Ask."}}},
		{effects.Form{Monster: "owl", TempHP: 4}, []effects.Act{effects.TakeForm{Monster: "owl", TempHP: 4}}},
		{effects.Summon{Monster: "wolf", Count: 2, Shares: true, NeedsCommand: true}, []effects.Act{effects.SummonCreatures{Monster: "wolf", Count: 2, Shares: true, NeedsCommand: true}}},
	} {
		if got := c.part.LandsOnBearer(); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%+v lands %+v", c.part, got)
		}
	}
	for _, p := range []effects.Part{effects.Area{}, effects.AreaSave{}, effects.CreateSurface{}, effects.ForcedMove{}, effects.Light{}} {
		if _, lands := p.(effects.BearerLander); lands {
			t.Errorf("%+v shapes an area and lands nothing on a bearer", p)
		}
	}
	for _, p := range []effects.Part{effects.Form{}, effects.Summon{}} {
		if _, asks := p.(effects.Asker); asks {
			t.Errorf("%+v asks nothing of an area", p)
		}
	}

	hot := []effects.Part{effects.ExtraDamage{Dice: "1d6"}}
	var choice effects.Part = effects.Choice{Modes: []effects.Group{{Name: "Hot", Parts: hot}, {Name: "Cold", Parts: []effects.Part{effects.Immobile{}}}}}
	ch := choice.(effects.Chooser)
	if !reflect.DeepEqual(ch.Groups(), []effects.Group{{Name: "Hot", Parts: hot}, {Name: "Cold", Parts: []effects.Part{effects.Immobile{}}}}) {
		t.Fatalf("a choice's modes are its groups = %+v", ch.Groups())
	}
	if !reflect.DeepEqual(ch.Chosen("Hot"), hot) || ch.Chosen("Warm") != nil {
		t.Fatalf("the mode chosen = %+v %+v", ch.Chosen("Hot"), ch.Chosen("Warm"))
	}
	if _, lands := choice.(effects.Holder); lands {
		t.Fatal("a choice is made as the Effect is applied, not as it lands")
	}
	var branch effects.Grouper = effects.Branch{When: effects.Condition{Kind: effects.FirstEachTurn}, Then: hot}
	if !reflect.DeepEqual(branch.Groups(), []effects.Group{{Name: "then", Parts: hot}}) {
		t.Fatalf("a branch's group = %+v", branch.Groups())
	}
	if got := (effects.Groups{{Name: "a", Parts: hot}}).Of("b"); got != nil {
		t.Fatalf("a group not given holds nothing = %+v", got)
	}
}

// A passive Part folds into the profiles attacks and saves read; a landing Part asks for its rolls and
// then says what happens to each creature in acts from a closed set; a Part with Parts beneath it says
// which of them come into play.
func TestPartsActThroughTheirCapabilities(t *testing.T) {
	t.Parallel()
	var bless effects.Part = effects.BonusDie{On: []effects.Roll{effects.AttackRolls, effects.SavingThrows}, Dice: "1d4"}
	var p effects.AttackProfile
	bless.(effects.AttackFolder).FoldAttacking(&p, effects.Held{Name: "Bless", Levels: 1})
	if !reflect.DeepEqual(p, effects.AttackProfile{AttackDice: []string{"1d4"}, Notes: []string{"Bless: +1d4 to hit"}}) {
		t.Fatalf("bless on an attack = %+v", p)
	}
	var s effects.SaveProfile
	bless.(effects.SaveFolder).FoldSave(&s, effects.Held{Name: "Bless", Levels: 1}, "wisdom")
	if !reflect.DeepEqual(s, effects.SaveProfile{Dice: []string{"1d4"}}) {
		t.Fatalf("bless on a save = %+v", s)
	}
	guidance := effects.BonusDie{On: []effects.Roll{effects.SavingThrows}, Dice: "1d4"}
	p, s = effects.AttackProfile{}, effects.SaveProfile{}
	guidance.FoldAttacking(&p, effects.Held{Name: "Resistance", Levels: 1})
	effects.BonusDie{On: []effects.Roll{effects.AttackRolls}, Dice: "1d4"}.FoldSave(&s, effects.Held{Name: "Edge", Levels: 1}, "wisdom")
	if p.AttackDice != nil || p.Notes != nil || s.Dice != nil {
		t.Fatalf("a die joins only the rolls it names = %+v %+v", p, s)
	}

	var fire effects.Part = effects.SaveDamage{Ability: "dexterity", Dice: "8d6", Type: "fire", Half: true}
	land := fire.(effects.Lander)
	if !reflect.DeepEqual(land.Asks(), []effects.Act{effects.RollSave{Ability: "dexterity"}, effects.RollDamage{Dice: "8d6", Type: "fire"}}) {
		t.Fatalf("fireball asks = %+v", land.Asks())
	}
	for _, c := range []struct {
		part effects.SaveDamage
		o    effects.Outcome
		want int
	}{
		{effects.SaveDamage{Half: true, Type: "fire"}, effects.Outcome{Saved: false, Damage: 9}, 9},
		{effects.SaveDamage{Half: true, Type: "fire"}, effects.Outcome{Saved: true, Damage: 9}, 4},
		{effects.SaveDamage{Half: false, Type: "fire"}, effects.Outcome{Saved: true, Damage: 9}, 0},
	} {
		if got := c.part.Lands(c.o); !reflect.DeepEqual(got, []effects.Act{effects.Hurt{Amount: c.want, Type: "fire"}}) {
			t.Errorf("%+v lands %+v as %+v", c.part, c.o, got)
		}
	}

	blind := effects.SaveCondition{Ability: "constitution", Slug: "blinded"}
	var branch effects.Part = effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 1}, Then: []effects.Part{blind}}
	h := branch.(effects.Holder)
	if !reflect.DeepEqual(h.Groups(), []effects.Group{{Name: "then", Parts: []effects.Part{blind}}}) {
		t.Fatalf("groups = %+v", h.Groups())
	}
	if got := h.Landed(effects.Situation{Saved: false, Margin: 1}); !reflect.DeepEqual(got, []effects.Part{blind}) {
		t.Fatalf("a failed save lands the branch = %+v", got)
	}
	if got := h.Landed(effects.Situation{Saved: true}); got != nil {
		t.Fatalf("a save keeps it off = %+v", got)
	}
	if _, folds := fire.(effects.AttackFolder); folds {
		t.Fatal("damage on landing folds into no attack")
	}
}

// An area's Parts land through its acts: what the area asks for, and the hurt each creature takes.
func TestAreasLandThroughActs(t *testing.T) {
	t.Parallel()
	fb, _ := srd().AreaOf("fireball")
	if fb.Damage != (effects.RollDamage{Dice: "8d6", Type: "fire"}) || fb.Save != "dexterity" {
		t.Fatalf("fireball asks = %+v", fb)
	}
	if got := fb.Lands(effects.Outcome{Saved: true, Damage: 21}); !reflect.DeepEqual(got, []effects.Act{effects.Hurt{Amount: 10, Type: "fire"}}) {
		t.Fatalf("a save halves it = %+v", got)
	}
	if g, _ := srd().AreaOf("grease"); g.Lands(effects.Outcome{Saved: false, Damage: 0}) != nil {
		t.Fatalf("grease hurts nobody = %+v", g)
	}
}

// A passive Part folds into the profile it changes: the attacks its bearer makes or suffers, its saves,
// its movement and what it can do on a turn. A stacking Part says how high its Effect rises and when it
// kills, and a Part that lands on its bearer says what it does there in acts.
func TestPassiveAndBearerPartsActThroughTheirCapabilities(t *testing.T) {
	t.Parallel()
	hex := effects.Held{Name: "Hex", Levels: 3}
	var p effects.AttackProfile
	var near effects.AttackedFolder = effects.Edge{Against: true, Advantage: true, Range: effects.WithinFive, SourceOnly: true}
	near.FoldAttacked(&p, hex, effects.Attacker{Source: false, WithinFive: true})
	near.FoldAttacked(&p, hex, effects.Attacker{Source: true, WithinFive: false})
	near.FoldAttacked(&p, hex, effects.Attacker{Source: true, WithinFive: true})
	effects.Edge{Against: false, Advantage: true, Range: effects.AnyRange}.FoldAttacked(&p, hex, effects.Attacker{Source: true, WithinFive: true})
	effects.ExtraDamage{Dice: "1d6"}.FoldAttacked(&p, hex, effects.Attacker{Source: false, WithinFive: true})
	effects.ExtraDamage{Dice: "1d6"}.FoldAttacked(&p, hex, effects.Attacker{Source: true, WithinFive: false})
	for range 2 {
		effects.CritWithin{Feet: 5}.FoldAttacked(&p, hex, effects.Attacker{Source: false, WithinFive: true})
	}
	effects.CritWithin{Feet: 4}.FoldAttacked(&p, effects.Held{Name: "Graze", Levels: 1}, effects.Attacker{Source: false, WithinFive: true})
	want := effects.AttackProfile{
		Advantages: []string{"Hex: advantage"}, DamageDice: []string{"1d6"}, Crit: true,
		Notes: []string{"Hex: +1d6 damage", "Hex: a hit from this close is a Critical Hit"},
	}
	if !reflect.DeepEqual(p, want) {
		t.Fatalf("attacked = %+v", p)
	}
	p = effects.AttackProfile{}
	var tired effects.AttackFolder = effects.Exhausting{D20PerLevel: 2, SpeedFtPerLevel: 5, DeathAt: 6, MaxLevel: 0}
	tired.FoldAttacking(&p, hex)
	effects.Edge{Against: false, Advantage: false, Range: effects.BeyondFive}.FoldAttacking(&p, hex)
	effects.Edge{Against: true, Advantage: true, Range: effects.AnyRange}.FoldAttacking(&p, hex)
	if !reflect.DeepEqual(p, effects.AttackProfile{Disadvantages: []string{"Hex: disadvantage"}, Notes: []string{"Hex 3: -6 to hit"}, Penalty: 6}) {
		t.Fatalf("attacking = %+v", p)
	}

	var s effects.SaveProfile
	var fails effects.SaveFolder = effects.SaveEdge{Ability: "dexterity", Mode: effects.SaveFails}
	fails.FoldSave(&s, hex, "wisdom")
	if !reflect.DeepEqual(s, effects.SaveProfile{}) {
		t.Fatalf("another ability's save = %+v", s)
	}
	for _, mode := range []effects.SaveMode{effects.SaveFails, effects.SaveAdvantage, effects.SaveDisadvantage} {
		effects.SaveEdge{Ability: "dexterity", Mode: mode}.FoldSave(&s, hex, "dexterity")
	}
	effects.Exhausting{D20PerLevel: 1}.FoldSave(&s, hex, "dexterity")
	if !reflect.DeepEqual(s, effects.SaveProfile{Fails: true, Advantages: []string{"Hex: advantage"}, Disadvantages: []string{"Hex: disadvantage"}, Penalty: 3}) {
		t.Fatalf("a Dexterity save = %+v", s)
	}

	m := effects.MoveProfile{Multiplier: 1}
	var crawl effects.MoveFolder = effects.MoveCost{Multiplier: 3}
	crawl.FoldMove(&m, hex)
	for _, f := range []effects.MoveFolder{effects.MoveCost{Multiplier: 2}, effects.SpeedPenalty{Ft: 20}, effects.SpeedPenalty{Ft: 10}, effects.Exhausting{SpeedFtPerLevel: 5}} {
		f.FoldMove(&m, hex)
	}
	if !reflect.DeepEqual(m, effects.MoveProfile{Immobile: false, Multiplier: 3, StackedFt: 15, LargestFt: 20}) {
		t.Fatalf("moving = %+v", m)
	}
	effects.Immobile{}.FoldMove(&m, hex)
	if !m.Immobile {
		t.Fatal("immobile holds speed at zero")
	}

	var a effects.ActionProfile
	var out effects.ActionFolder = effects.Incapacitated{}
	effects.Reacts{Trigger: "damaged", Instruction: "Hiss."}.FoldActions(&a, hex)
	effects.Counter{RangeFt: 30}.FoldActions(&a, hex)
	effects.Counter{RangeFt: 30}.FoldActions(&a, effects.Held{Name: "Echo", Levels: 1})
	effects.Counter{RangeFt: 60}.FoldActions(&a, effects.Held{Name: "Counterspell", Levels: 1})
	if a.Incapacitated || !reflect.DeepEqual(a.Reactions, map[string][]effects.Reaction{"damaged": {{Name: "Hex", Instruction: "Hiss."}}}) || a.CounterFt != 60 || a.CounterBy != "Counterspell" {
		t.Fatalf("actions = %+v", a)
	}
	out.FoldActions(&a, hex)
	if !a.Incapacitated {
		t.Fatal("incapacitated takes every action away")
	}

	var stack effects.Stacker = effects.Exhausting{D20PerLevel: 1, DeathAt: 4, MaxLevel: 5}
	if stack.Cap() != 5 || stack.Kills(3) || !stack.Kills(4) || (effects.Exhausting{DeathAt: 0}).Kills(9) {
		t.Fatalf("stacking = %+v", stack)
	}

	for _, c := range []struct {
		part effects.BearerLander
		want []effects.Act
	}{
		{effects.TempHP{Amount: 8}, []effects.Act{effects.GainTempHP{Amount: 8}}},
		{effects.Dispel{}, []effects.Act{effects.EndSpells{}}},
		{effects.GrantFeature{Name: "Darkvision"}, []effects.Act{effects.Grant{Feature: "Darkvision"}}},
		{effects.ResourceChange{Resource: "ki", Delta: -1}, []effects.Act{effects.ChangeUses{Resource: "ki", Delta: -1}}},
		{effects.Teleport{RangeFt: 30}, []effects.Act{effects.TeleportCaster{RangeFt: 30}}},
	} {
		if got := c.part.LandsOnBearer(); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%+v lands %+v", c.part, got)
		}
	}
	var temp effects.Part = effects.TempHP{Amount: 1}
	if _, folds := temp.(effects.AttackFolder); folds {
		t.Fatal("temporary hit points fold into no attack")
	}
}
