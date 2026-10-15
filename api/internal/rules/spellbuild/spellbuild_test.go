package spellbuild_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
)

var surfaces = []string{"fog", "grease"}

func part(kind string, values effects.Values, groups ...effects.AuthoredGroup) effects.Authored {
	return effects.Authored{Kind: kind, Values: values, Groups: groups}
}

func text(s string) effects.Value { return effects.Value{Text: s} }

func number(n int) effects.Value { return effects.Value{Number: n} }

// then is the group of Parts a branch adds.
func then(parts ...effects.Authored) effects.AuthoredGroup {
	return effects.AuthoredGroup{Name: "then", Parts: parts}
}

// charmedIf charms a creature of one type that fails the spell's save.
func charmedIf(creature string) effects.Authored {
	failed := part("branch", effects.Values{"when": text("fails_by"), "n": number(1)}, then(part("save_condition", effects.Values{"condition": text("charmed")})))
	return part("branch", effects.Values{"when": text("creature_is"), "creature_type": text(creature)}, then(failed))
}

// lantern is the Marsh Lantern: a moving light that reveals the invisible, charms Undead and Fey that
// fail a Wisdom save, and burns with radiant light those who start their turn near it.
func lantern() spellbuild.Design {
	return spellbuild.Design{
		Targeting: spellbuild.Targeting{Shape: "emanation", SizeFt: 10}, Save: "wisdom", Concentration: true,
		Duration: spellbuild.Duration{Unit: "minutes", Amount: 1}, Ritual: true, CastingTime: spellbuild.CastingTime{Kind: "action"},
		Components: spellbuild.Components{Verbal: true, Somatic: true, Material: &spellbuild.Material{Text: "a lantern of bog glass", CostGP: 25, Consumed: true, Item: "lantern"}},
		TurnDamage: &spellbuild.TurnDamage{Dice: "1d6", DamageType: "radiant"},
		Parts: []effects.Authored{
			part("light", effects.Values{"bright_ft": number(20), "dim_ft": number(20)}),
			part("reveal", effects.Values{"qualities": {Texts: []string{"invisible"}}}),
			charmedIf("undead"),
			charmedIf("fey"),
		},
	}
}

func TestTheMarshLanternBuildsIntoAnEffect(t *testing.T) {
	t.Parallel()
	b, err := spellbuild.Build("hb-lantern", "Marsh Lantern", lantern(), surfaces)
	if err != nil {
		t.Fatal(err)
	}
	failed := effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 1}, Then: []effects.Part{effects.SaveCondition{Ability: "wisdom", Slug: "charmed"}}}
	want := effects.Definition{
		Slug: "hb-lantern", Name: "Marsh Lantern", Owner: effects.OwnedBySpell, Concentration: true, Duration: effects.Duration{Kind: effects.Minutes, Amount: 1},
		Parts: []effects.Part{
			effects.Area{Shape: hex.EmanationArea, SizeFt: 10},
			effects.AreaSave{Ability: "wisdom"},
			effects.Light{BrightFt: 20, DimFt: 20},
			effects.Reveal{Qualities: []string{"invisible"}},
			effects.Branch{When: effects.Condition{Kind: effects.CreatureIs, Type: "undead"}, Then: []effects.Part{failed}},
			effects.Branch{When: effects.Condition{Kind: effects.CreatureIs, Type: "fey"}, Then: []effects.Part{failed}},
			effects.CreateSurface{Surface: "hb-lantern-ground", Rounds: 10},
		},
	}
	if !reflect.DeepEqual(b.Definition, want) {
		t.Fatalf("definition =\n%+v\nwant\n%+v", b.Definition, want)
	}
	if b.Surface == nil || *b.Surface != (spellbuild.Surface{Slug: "hb-lantern-ground", Name: "Marsh Lantern", Dice: "1d6", Type: "radiant"}) {
		t.Fatalf("surface = %+v", b.Surface)
	}
	wantText := []string{
		"Casting Time: 1 action or Ritual.",
		"Components: V, S, M (a lantern of bog glass worth 25+ gp, which the spell consumes).",
		"Area: a 10-foot emanation from the caster.",
		"Duration: Concentration, up to 1 minute.",
		"Bright light fills 20 feet around where it lands, and dim light another 20 feet.",
		"Every creature and object in the area loses Invisible, whatever anyone's senses.",
		"If a creature is an undead and fails the save, it has the Charmed condition.",
		"If a creature is a fey and fails the save, it has the Charmed condition.",
		"A creature that starts its turn in the area takes 1d6 radiant damage.",
	}
	if !reflect.DeepEqual(b.Text, wantText) {
		t.Fatalf("text =\n%s", strings.Join(b.Text, "\n"))
	}
	if d := lantern(); d.Parts[2].Groups[0].Parts[0].Groups[0].Parts[0].Values["ability"].Text != "" {
		t.Fatal("building fills the spell's save in without changing the design")
	}
}

func TestOtherPartsAndMetadata(t *testing.T) {
	t.Parallel()
	d := spellbuild.Design{
		Targeting: spellbuild.Targeting{Shape: "sphere", SizeFt: 20, RangeFt: 150}, Save: "dexterity",
		Duration: spellbuild.Duration{Unit: "instant"}, CastingTime: spellbuild.CastingTime{Kind: "reaction", Trigger: "when_damaged"},
		Components: spellbuild.Components{Material: &spellbuild.Material{Text: "a pinch of sulfur"}},
		Parts: []effects.Authored{
			part("save_damage", effects.Values{"dice": text("8d6"), "damage_type": text("fire"), "half": {Flag: true}}),
			part("save_condition", effects.Values{"ability": text("dexterity"), "condition": text("prone")}),
			part("create_surface", effects.Values{"surface": text("grease"), "rounds": number(3)}),
			part("manual", effects.Values{"instruction": text("Flammable objects ignite.")}),
		},
	}
	b, err := spellbuild.Build("hb-burst", "Burst", d, surfaces)
	if err != nil {
		t.Fatal(err)
	}
	want := []effects.Part{
		effects.Area{Shape: hex.SphereArea, SizeFt: 20, RangeFt: 150},
		effects.SaveDamage{Ability: "dexterity", Dice: "8d6", Type: "fire", Half: true},
		effects.SaveCondition{Ability: "dexterity", Slug: "prone"},
		effects.CreateSurface{Surface: "grease", Rounds: 3},
		effects.Manual{Instruction: "Flammable objects ignite."},
	}
	if !reflect.DeepEqual(b.Definition.Parts, want) || b.Surface != nil {
		t.Fatalf("parts = %+v", b.Definition.Parts)
	}
	for i, want := range []string{
		"Casting Time: 1 reaction, which you take when damaged.",
		"Components: M (a pinch of sulfur).",
		"Area: a 20-foot sphere at a point within 150 feet.",
		"Duration: Instantaneous.",
		"Each creature in the area makes a Dexterity saving throw, taking 8d6 fire damage on a failed save or half as much damage on a successful one.",
		"On a failed Dexterity saving throw, a creature has the Prone condition.",
		"The area is covered in grease for 3 rounds.",
		"Flammable objects ignite.",
	} {
		if b.Text[i] != want {
			t.Errorf("text[%d] = %q, want %q", i, b.Text[i], want)
		}
	}
	d.Parts = []effects.Authored{part("save_damage", effects.Values{"dice": text("2d8"), "damage_type": text("cold")})}
	d.CastingTime = spellbuild.CastingTime{Kind: "minutes", Minutes: 10}
	d.Components = spellbuild.Components{Somatic: true}
	b, _ = spellbuild.Build("hb-chill", "Chill", d, surfaces)
	if b.Text[0] != "Casting Time: 10 minutes." || b.Text[1] != "Components: S." || b.Text[4] != "Each creature in the area makes a Dexterity saving throw, taking 2d8 cold damage on a failed save." {
		t.Fatalf("text = %v", b.Text)
	}
	d.CastingTime, d.Save, d.Parts = spellbuild.CastingTime{Kind: "bonus_action"}, "", nil
	if b, _ := spellbuild.Build("hb-x", "X", d, surfaces); b.Text[0] != "Casting Time: 1 bonus action." || len(b.Definition.Parts) != 1 {
		t.Fatalf("a bare spell = %+v", b)
	}
}

func TestDesignsAreChecked(t *testing.T) {
	t.Parallel()
	with := func(change func(*spellbuild.Design)) spellbuild.Design {
		d := lantern()
		change(&d)
		return d
	}
	only := func(p ...effects.Authored) func(*spellbuild.Design) {
		return func(d *spellbuild.Design) { d.TurnDamage, d.Parts = nil, p }
	}
	many := make([]effects.Authored, 31)
	for i := range many {
		many[i] = part("manual", effects.Values{"instruction": text("x")})
	}
	burn := part("save_damage", effects.Values{"dice": text("2d6"), "damage_type": text("fire")})
	fog := part("create_surface", effects.Values{"surface": text("fog"), "rounds": number(2)})
	light := part("light", effects.Values{"bright_ft": number(5)})
	first := func(p effects.Authored) effects.Authored {
		return part("branch", effects.Values{"when": text("first_each_turn")}, then(p))
	}
	for name, c := range map[string]struct {
		change func(*spellbuild.Design)
		want   string
	}{
		"shape":          {func(d *spellbuild.Design) { d.Targeting.Shape = "blob" }, "choose an area"},
		"small":          {func(d *spellbuild.Design) { d.Targeting.SizeFt = 0 }, "size is 5 to 300"},
		"large":          {func(d *spellbuild.Design) { d.Targeting.SizeFt = 305 }, "size is 5 to 300"},
		"uneven size":    {func(d *spellbuild.Design) { d.Targeting.SizeFt = 12 }, "size is 5 to 300"},
		"negative range": {func(d *spellbuild.Design) { d.Targeting.RangeFt = -5 }, "range is 0 to 1000"},
		"far":            {func(d *spellbuild.Design) { d.Targeting.RangeFt = 1005 }, "range is 0 to 1000"},
		"uneven range":   {func(d *spellbuild.Design) { d.Targeting.RangeFt = 7 }, "range is 0 to 1000"},
		"save":           {func(d *spellbuild.Design) { d.Save = "luck" }, "the save is an ability"},
		"too many parts": {func(d *spellbuild.Design) { d.Parts = many }, "30 Parts"},
		"concentrating":  {func(d *spellbuild.Design) { d.Duration = spellbuild.Duration{Unit: "instant"} }, "needs no concentration"},
		"no time":        {func(d *spellbuild.Design) { d.Duration.Amount = 0 }, "1 to 100 rounds"},
		"long":           {func(d *spellbuild.Design) { d.Duration.Amount = 101 }, "1 to 100 rounds"},
		"unit":           {func(d *spellbuild.Design) { d.Duration.Unit = "days" }, "the duration is"},
		"casting":        {func(d *spellbuild.Design) { d.CastingTime.Kind = "free" }, "the casting time is"},
		"trigger":        {func(d *spellbuild.Design) { d.CastingTime = spellbuild.CastingTime{Kind: "reaction"} }, "answers a trigger"},
		"no minutes":     {func(d *spellbuild.Design) { d.CastingTime = spellbuild.CastingTime{Kind: "minutes"} }, "1 to 1440 minutes"},
		"days":           {func(d *spellbuild.Design) { d.CastingTime = spellbuild.CastingTime{Kind: "minutes", Minutes: 1441} }, "1 to 1440 minutes"},
		"quick ritual":   {func(d *spellbuild.Design) { d.CastingTime = spellbuild.CastingTime{Kind: "bonus_action"} }, "a ritual takes"},
		"reaction ritual": {func(d *spellbuild.Design) {
			d.CastingTime = spellbuild.CastingTime{Kind: "reaction", Trigger: "when_hit"}
		}, "a ritual takes"},
		"no components":   {func(d *spellbuild.Design) { d.Components = spellbuild.Components{} }, "at least one component"},
		"blank material":  {func(d *spellbuild.Design) { d.Components.Material.Text = " " }, "describe the Material"},
		"long material":   {func(d *spellbuild.Design) { d.Components.Material.Text = strings.Repeat("a", 201) }, "describe the Material"},
		"negative cost":   {func(d *spellbuild.Design) { d.Components.Material.CostGP = -1 }, "costs 0 to 100000"},
		"dear":            {func(d *spellbuild.Design) { d.Components.Material.CostGP = 100001 }, "costs 0 to 100000"},
		"free consumed":   {func(d *spellbuild.Design) { d.Components.Material.CostGP = 0 }, "only a Material Component with a cost"},
		"turn dice":       {func(d *spellbuild.Design) { d.TurnDamage.Dice = "2d7" }, "damage dice look like 2d6"},
		"turn type":       {func(d *spellbuild.Design) { d.TurnDamage.DamageType = "pain" }, "choose a damage type"},
		"instant burn":    {func(d *spellbuild.Design) { d.Concentration, d.Duration = false, spellbuild.Duration{Unit: "instant"} }, "needs a duration in rounds"},
		"kind":            {only(part("song", nil)), "a spell holds Parts of the kinds branch, create_surface, light, manual, reveal, save_condition, save_damage"},
		"passive":         {only(part("bonus_die", nil)), "a spell holds Parts of the kinds"},
		"beneath":         {only(first(light)), "beneath a branch, a spell holds Parts of the kinds branch, manual, save_condition"},
		"deep beneath":    {only(first(first(fog))), "beneath a branch, a spell holds Parts of the kinds"},
		"kind's own":      {only(part("save_damage", effects.Values{"dice": text("2d7"), "damage_type": text("fire")})), "part 1: dice: write the damage like 8d6"},
		"beneath's own":   {only(first(part("manual", nil))), "part 1: then: part 1: instruction: write up to 500 letters"},
		"twice on cast":   {only(burn, burn), "on cast once"},
		"unsaved damage":  {func(d *spellbuild.Design) { d.Save, d.TurnDamage, d.Parts = "", nil, []effects.Authored{burn} }, "needs the save"},
		"unsaved charm":   {func(d *spellbuild.Design) { d.Save, d.TurnDamage = "", nil }, "needs the save"},
		"another save":    {only(part("save_damage", effects.Values{"ability": text("strength"), "dice": text("2d6"), "damage_type": text("fire")})), "rolls the spell's save, wisdom"},
		"another beneath": {only(first(part("save_condition", effects.Values{"ability": text("strength"), "condition": text("prone")}))), "rolls the spell's save, wisdom"},
		"two grounds":     {func(d *spellbuild.Design) { d.Parts = append(d.Parts, fog) }, "one Surface or one start-of-turn damage"},
		"two surfaces":    {only(fog, fog), "one Surface or one start-of-turn damage"},
		"condition":       {only(part("save_condition", effects.Values{"condition": text("sleepy")})), "choose a condition"},
		"deep condition":  {only(first(first(part("save_condition", effects.Values{"condition": text("sleepy")})))), "choose a condition"},
		"two lights":      {func(d *spellbuild.Design) { d.Parts = append(d.Parts, light) }, "one light"},
		"surface":         {only(part("create_surface", effects.Values{"surface": text("lava"), "rounds": number(2)})), "choose a surface"},
	} {
		_, err := spellbuild.Build("hb-x", "X", with(c.change), surfaces)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", name, err, c.want)
		}
	}
	for name, change := range map[string]func(*spellbuild.Design){
		"edges": func(d *spellbuild.Design) {
			d.Targeting = spellbuild.Targeting{Shape: "cube", SizeFt: 300, RangeFt: 1000}
			d.Duration = spellbuild.Duration{Unit: "hours", Amount: 100}
			d.Components.Material.CostGP = 100000
			d.Parts = append(make([]effects.Authored, 0, 30), d.Parts...)
			for len(d.Parts) < 30 {
				d.Parts = append(d.Parts, part("manual", effects.Values{"instruction": text(strings.Repeat("a", 500))}))
			}
		},
		"least": func(d *spellbuild.Design) {
			d.Targeting = spellbuild.Targeting{Shape: "line", SizeFt: 5}
			d.Duration = spellbuild.Duration{Unit: "until_dispelled"}
			d.CastingTime = spellbuild.CastingTime{Kind: "minutes", Minutes: 1440}
			d.Components.Material = &spellbuild.Material{Text: strings.Repeat("a", 200)}
			d.TurnDamage, d.Parts = nil, []effects.Authored{
				part("light", effects.Values{"bright_ft": number(120), "dim_ft": number(120)}), fog,
				part("save_damage", effects.Values{"ability": text("wisdom"), "dice": text("99d20"), "damage_type": text("acid")}),
			}
		},
		"brief": func(d *spellbuild.Design) {
			d.Duration = spellbuild.Duration{Unit: "rounds", Amount: 1}
			d.CastingTime = spellbuild.CastingTime{Kind: "minutes", Minutes: 1}
			d.TurnDamage, d.Parts = nil, []effects.Authored{light, fog}
		},
		"deep": func(d *spellbuild.Design) {
			d.TurnDamage, d.Parts = nil, []effects.Authored{first(first(part("manual", effects.Values{"instruction": text("Ring.")})))}
		},
	} {
		if _, err := spellbuild.Build("hb-x", "X", with(change), surfaces); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestTheAreaPreviews(t *testing.T) {
	t.Parallel()
	ring := spellbuild.Preview(spellbuild.Targeting{Shape: "emanation", SizeFt: 5})
	if len(ring) != 6 {
		t.Fatalf("a 5-foot emanation touches the six hexes around = %v", ring)
	}
	line := spellbuild.Preview(spellbuild.Targeting{Shape: "line", SizeFt: 15})
	if len(line) != 3 || line[0] == (hex.Coord{}) {
		t.Fatalf("a 15-foot line from the caster heads east = %v", line)
	}
	ball := spellbuild.Preview(spellbuild.Targeting{Shape: "sphere", SizeFt: 5, RangeFt: 30})
	if len(ball) != 7 {
		t.Fatalf("a 5-foot sphere at a point = %v", ball)
	}
	if len(spellbuild.Shapes()) != 8 || len(spellbuild.Triggers()) != 5 {
		t.Fatal("the builder's lists")
	}
}

func TestAHomebrewSpellRunsUnderASlugOfItsEntry(t *testing.T) {
	t.Parallel()
	if got := spellbuild.Slug("0190c7a8-0000-7000-8000-0000000000e1"); got != "hb-0190c7a80000" {
		t.Fatalf("slug = %q", got)
	}
}

// A spell serves what it holds from the lists it checks Parts against: the kinds at the top and beneath
// a branch, the save it fills in, and the Surfaces and conditions a Part may name.
func TestASpellServesWhatItHolds(t *testing.T) {
	t.Parallel()
	want := effects.Holds{
		Kinds:   []string{"branch", "create_surface", "light", "manual", "reveal", "save_condition", "save_damage"},
		Beneath: []string{"branch", "manual", "save_condition"},
		Fills:   []string{"ability"},
		Narrows: []effects.Narrow{
			{Kind: "create_surface", Field: "surface", Options: surfaces},
			{Kind: "save_condition", Field: "condition", Options: spellbuild.Conditions()},
		},
	}
	if got := spellbuild.Holds(surfaces); !reflect.DeepEqual(got, want) {
		t.Errorf("holds %+v, want %+v", got, want)
	}
}
