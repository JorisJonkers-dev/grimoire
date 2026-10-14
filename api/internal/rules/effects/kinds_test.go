package effects_test

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// Every kind is listed once, in slug order, and describes the fields an author fills in.
func TestKindsAreListedOnceInOrder(t *testing.T) {
	t.Parallel()
	var slugs []string
	for _, k := range effects.Kinds() {
		d := k.Describe()
		if d.Label == "" || d.Summary == "" || len(d.Fields) == 0 {
			t.Errorf("%s describes too little: %+v", d.Slug, d)
		}
		for _, f := range d.Fields {
			if f.Name == "" || f.Label == "" || f.Type == "" {
				t.Errorf("%s has a field without a name, label or type: %+v", d.Slug, f)
			}
		}
		slugs = append(slugs, d.Slug)
	}
	if !reflect.DeepEqual(slugs, []string{"bonus_die", "branch", "save_damage"}) {
		t.Fatalf("kinds = %v", slugs)
	}
	branch := effects.Kinds()[1].Describe()
	if !reflect.DeepEqual(branch.Groups, []string{"then"}) || effects.Kinds()[0].Describe().Groups != nil {
		t.Fatalf("only a branch holds Parts beneath it = %+v", branch)
	}
	ability := effects.Kinds()[2].Describe().Fields[0]
	if ability.Type != effects.ChoiceField || !ability.Required || !slices.Contains(ability.Options, "dexterity") {
		t.Fatalf("a save names its ability = %+v", ability)
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
			effects.Groups{"then": {stun}},
			effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 5}, Then: []effects.Component{stun}},
			"If a creature fails the save by 5 or more, on a failed Constitution saving throw, a creature has the Stunned condition.",
		},
		{
			"branch",
			effects.Values{"when": {Text: "creature_is"}, "creature_type": {Text: "undead"}},
			effects.Groups{"then": {effects.Dispel{}}},
			effects.Branch{When: effects.Condition{Kind: effects.CreatureIs, Type: "undead"}, Then: []effects.Component{effects.Dispel{}}},
			"If the creature is an undead, every spell on the target ends.",
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
	then := effects.Groups{"then": {effects.Dispel{}}}
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
	} {
		_, err := kind(t, c.kind).Decode(c.values, c.groups)
		var bad effects.ValueError
		if !errors.As(err, &bad) || bad.Field != c.field || bad.Reason == "" || !strings.HasPrefix(err.Error(), c.field+": ") {
			t.Errorf("%s %v = %v, want a refusal of %s", c.kind, c.values, err, c.field)
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
}

// A passive Part folds into the profiles attacks and saves read; a landing Part asks for its rolls and
// then says what happens to each creature in acts from a closed set; a Part with Parts beneath it says
// which of them come into play.
func TestPartsActThroughTheirCapabilities(t *testing.T) {
	t.Parallel()
	var bless effects.Part = effects.BonusDie{On: []effects.Roll{effects.AttackRolls, effects.SavingThrows}, Dice: "1d4"}
	var p effects.AttackProfile
	bless.(effects.AttackFolder).FoldAttacking(&p, "Bless")
	if !reflect.DeepEqual(p, effects.AttackProfile{AttackDice: []string{"1d4"}, Notes: []string{"Bless: +1d4 to hit"}}) {
		t.Fatalf("bless on an attack = %+v", p)
	}
	var s effects.SaveProfile
	bless.(effects.SaveFolder).FoldSave(&s, "Bless", "wisdom")
	if !reflect.DeepEqual(s, effects.SaveProfile{Dice: []string{"1d4"}}) {
		t.Fatalf("bless on a save = %+v", s)
	}
	guidance := effects.BonusDie{On: []effects.Roll{effects.SavingThrows}, Dice: "1d4"}
	p, s = effects.AttackProfile{}, effects.SaveProfile{}
	guidance.FoldAttacking(&p, "Resistance")
	effects.BonusDie{On: []effects.Roll{effects.AttackRolls}, Dice: "1d4"}.FoldSave(&s, "Edge", "wisdom")
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
	var branch effects.Part = effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 1}, Then: []effects.Component{blind}}
	h := branch.(effects.Holder)
	if !reflect.DeepEqual(h.Groups(), []effects.Group{{Name: "then", Parts: []effects.Component{blind}}}) {
		t.Fatalf("groups = %+v", h.Groups())
	}
	if got := h.Landed(effects.Situation{Saved: false, Margin: 1}); !reflect.DeepEqual(got, []effects.Component{blind}) {
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
