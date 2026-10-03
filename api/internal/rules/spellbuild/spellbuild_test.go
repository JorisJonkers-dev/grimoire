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

// lantern is the Marsh Lantern: a moving light that reveals the invisible, charms Undead and Fey that
// fail a Wisdom save, and burns with radiant light those who start their turn near it.
func lantern() spellbuild.Design {
	return spellbuild.Design{
		Targeting: spellbuild.Targeting{Shape: "emanation", SizeFt: 10}, Save: "wisdom", Concentration: true,
		Duration: spellbuild.Duration{Unit: "minutes", Amount: 1}, Ritual: true, CastingTime: spellbuild.CastingTime{Kind: "action"},
		Components: spellbuild.Components{Verbal: true, Somatic: true, Material: &spellbuild.Material{Text: "a lantern of bog glass", CostGP: 25, Consumed: true, Item: "lantern"}},
		Parts: []spellbuild.Part{
			{Type: "light", BrightFt: 20, DimFt: 20},
			{Type: "reveal", Qualities: []string{"invisible"}},
			{Type: "condition", Condition: "charmed", OnlyTypes: []string{"undead", "fey"}},
			{Type: "damage", When: "start_of_turn", Dice: "1d6", DamageType: "radiant"},
		},
	}
}

func TestTheMarshLanternBuildsIntoAnEffect(t *testing.T) {
	t.Parallel()
	b, err := spellbuild.Build("hb-lantern", "Marsh Lantern", lantern(), surfaces)
	if err != nil {
		t.Fatal(err)
	}
	failed := effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 1}, Then: []effects.Component{effects.SaveCondition{Ability: "wisdom", Slug: "charmed"}}}
	want := effects.Definition{
		Slug: "hb-lantern", Name: "Marsh Lantern", Owner: effects.OwnedBySpell, Concentration: true, Duration: effects.Duration{Kind: effects.Minutes, Amount: 1},
		Components: []effects.Component{
			effects.Area{Shape: hex.EmanationArea, SizeFt: 10},
			effects.AreaSave{Ability: "wisdom"},
			effects.Light{BrightFt: 20, DimFt: 20},
			effects.Reveal{Qualities: []string{"invisible"}},
			effects.Branch{When: effects.Condition{Kind: effects.CreatureIs, Type: "undead"}, Then: []effects.Component{failed}},
			effects.Branch{When: effects.Condition{Kind: effects.CreatureIs, Type: "fey"}, Then: []effects.Component{failed}},
			effects.CreateSurface{Kind: "hb-lantern-ground", Rounds: 10},
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
		"Bright light fills 20 feet around the spell, and dim light another 20 feet.",
		"Every creature and object in the area loses Invisible.",
		"A creature that fails the Wisdom saving throw has the Charmed condition, if it is Undead or Fey.",
		"A creature that starts its turn in the area takes 1d6 radiant damage.",
	}
	if !reflect.DeepEqual(b.Text, wantText) {
		t.Fatalf("text =\n%s", strings.Join(b.Text, "\n"))
	}
}

func TestOtherPartsAndMetadata(t *testing.T) {
	t.Parallel()
	d := spellbuild.Design{
		Targeting: spellbuild.Targeting{Shape: "sphere", SizeFt: 20, RangeFt: 150}, Save: "dexterity",
		Duration: spellbuild.Duration{Unit: "instant"}, CastingTime: spellbuild.CastingTime{Kind: "reaction", Trigger: "when_damaged"},
		Components: spellbuild.Components{Material: &spellbuild.Material{Text: "a pinch of sulfur"}},
		Parts: []spellbuild.Part{
			{Type: "damage", When: "on_cast", Dice: "8d6", DamageType: "fire", Half: true},
			{Type: "condition", Condition: "prone"},
			{Type: "surface", Surface: "grease", Rounds: 3},
			{Type: "manual", Text: "Flammable objects ignite."},
		},
	}
	b, err := spellbuild.Build("hb-burst", "Burst", d, surfaces)
	if err != nil {
		t.Fatal(err)
	}
	comps := b.Definition.Components
	if len(comps) != 5 || comps[1] != (effects.SaveDamage{Ability: "dexterity", Dice: "8d6", Type: "fire", Half: true}) || b.Surface != nil {
		t.Fatalf("components = %+v", comps)
	}
	if comps[3] != (effects.CreateSurface{Kind: "grease", Rounds: 3}) || comps[4] != (effects.Manual{Instruction: "Flammable objects ignite."}) {
		t.Fatalf("surface and manual = %+v", comps[3:])
	}
	for i, want := range []string{
		"Casting Time: 1 reaction, which you take when damaged.",
		"Components: M (a pinch of sulfur).",
		"Area: a 20-foot sphere at a point within 150 feet.",
		"Duration: Instantaneous.",
		"Each creature in the area takes 8d6 fire damage on a failed Dexterity saving throw, or half as much on a successful one.",
		"A creature that fails the Dexterity saving throw has the Prone condition.",
		"The area is covered in grease for 3 rounds.",
		"Flammable objects ignite.",
	} {
		if b.Text[i] != want {
			t.Errorf("text[%d] = %q, want %q", i, b.Text[i], want)
		}
	}
	d.Parts = []spellbuild.Part{{Type: "damage", When: "on_cast", Dice: "2d8", DamageType: "cold"}}
	d.CastingTime = spellbuild.CastingTime{Kind: "minutes", Minutes: 10}
	d.Components = spellbuild.Components{Somatic: true}
	b, _ = spellbuild.Build("hb-chill", "Chill", d, surfaces)
	if b.Text[0] != "Casting Time: 10 minutes." || b.Text[1] != "Components: S." || b.Text[4] != "Each creature in the area takes 2d8 cold damage on a failed Dexterity saving throw." {
		t.Fatalf("text = %v", b.Text)
	}
	d.CastingTime, d.Save, d.Parts = spellbuild.CastingTime{Kind: "bonus_action"}, "", nil
	if b, _ := spellbuild.Build("hb-x", "X", d, surfaces); b.Text[0] != "Casting Time: 1 bonus action." || len(b.Definition.Components) != 1 {
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
	part := func(p spellbuild.Part) func(*spellbuild.Design) {
		return func(d *spellbuild.Design) { d.Parts = []spellbuild.Part{p} }
	}
	many := make([]spellbuild.Part, 31)
	for i := range many {
		many[i] = spellbuild.Part{Type: "manual", Text: "x"}
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
		"too many parts": {func(d *spellbuild.Design) { d.Parts = many }, "30 parts"},
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
		"no components":  {func(d *spellbuild.Design) { d.Components = spellbuild.Components{} }, "at least one component"},
		"blank material": {func(d *spellbuild.Design) { d.Components.Material.Text = " " }, "describe the Material"},
		"long material":  {func(d *spellbuild.Design) { d.Components.Material.Text = strings.Repeat("a", 201) }, "describe the Material"},
		"negative cost":  {func(d *spellbuild.Design) { d.Components.Material.CostGP = -1 }, "costs 0 to 100000"},
		"dear":           {func(d *spellbuild.Design) { d.Components.Material.CostGP = 100001 }, "costs 0 to 100000"},
		"free consumed":  {func(d *spellbuild.Design) { d.Components.Material.CostGP = 0 }, "only a Material Component with a cost"},
		"part type":      {part(spellbuild.Part{Type: "song"}), "a part is"},
		"blank manual":   {part(spellbuild.Part{Type: "manual", Text: " "}), "a manual part"},
		"long manual":    {part(spellbuild.Part{Type: "manual", Text: strings.Repeat("a", 501)}), "a manual part"},
		"dice":           {part(spellbuild.Part{Type: "damage", When: "on_cast", Dice: "2d7", DamageType: "fire"}), "damage dice"},
		"damage type":    {part(spellbuild.Part{Type: "damage", When: "on_cast", Dice: "2d6", DamageType: "pain"}), "damage type"},
		"when":           {part(spellbuild.Part{Type: "damage", When: "later", Dice: "2d6", DamageType: "fire"}), "on cast or at the start"},
		"twice on cast": {func(d *spellbuild.Design) {
			d.Parts = []spellbuild.Part{{Type: "damage", When: "on_cast", Dice: "2d6", DamageType: "fire"}, {Type: "damage", When: "on_cast", Dice: "2d6", DamageType: "fire"}}
		}, "on cast once"},
		"unsaved damage": {func(d *spellbuild.Design) {
			d.Save, d.Parts = "", []spellbuild.Part{{Type: "damage", When: "on_cast", Dice: "2d6", DamageType: "fire"}}
		}, "needs the save"},
		"instant burn": {func(d *spellbuild.Design) {
			d.Concentration, d.Duration = false, spellbuild.Duration{Unit: "instant"}
		}, "needs a duration in rounds"},
		"two grounds": {func(d *spellbuild.Design) {
			d.Parts = append(d.Parts, spellbuild.Part{Type: "surface", Surface: "fog", Rounds: 2})
		}, "one Surface or one start-of-turn damage"},
		"burn after ground": {func(d *spellbuild.Design) {
			d.Parts = []spellbuild.Part{{Type: "surface", Surface: "fog", Rounds: 2}, {Type: "damage", When: "start_of_turn", Dice: "1d6", DamageType: "radiant"}}
		}, "one Surface or one start-of-turn damage"},
		"condition": {part(spellbuild.Part{Type: "condition", Condition: "sleepy"}), "choose a condition"},
		"unsaved charm": {func(d *spellbuild.Design) {
			d.Save, d.Parts = "", []spellbuild.Part{{Type: "condition", Condition: "charmed"}}
		}, "needs the save"},
		"creature type":  {part(spellbuild.Part{Type: "condition", Condition: "charmed", OnlyTypes: []string{"robot"}}), "creature types"},
		"two lights":     {func(d *spellbuild.Design) { d.Parts = append(d.Parts, spellbuild.Part{Type: "light", BrightFt: 5}) }, "one light"},
		"dark":           {part(spellbuild.Part{Type: "light", BrightFt: 0}), "light reaches"},
		"blinding":       {part(spellbuild.Part{Type: "light", BrightFt: 125}), "light reaches"},
		"uneven bright":  {part(spellbuild.Part{Type: "light", BrightFt: 7}), "light reaches"},
		"negative dim":   {part(spellbuild.Part{Type: "light", BrightFt: 5, DimFt: -5}), "light reaches"},
		"wide dim":       {part(spellbuild.Part{Type: "light", BrightFt: 5, DimFt: 125}), "light reaches"},
		"uneven dim":     {part(spellbuild.Part{Type: "light", BrightFt: 5, DimFt: 3}), "light reaches"},
		"empty reveal":   {part(spellbuild.Part{Type: "reveal"}), "at least one Visibility Quality"},
		"quality":        {part(spellbuild.Part{Type: "reveal", Qualities: []string{"shy"}}), "reveal Visibility Qualities"},
		"surface":        {part(spellbuild.Part{Type: "surface", Surface: "lava", Rounds: 2}), "choose a Surface"},
		"no rounds":      {part(spellbuild.Part{Type: "surface", Surface: "fog"}), "1 to 100 rounds"},
		"endless ground": {part(spellbuild.Part{Type: "surface", Surface: "fog", Rounds: 101}), "1 to 100 rounds"},
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
			d.Parts = append(make([]spellbuild.Part, 0, 30), d.Parts...)
			for len(d.Parts) < 30 {
				d.Parts = append(d.Parts, spellbuild.Part{Type: "manual", Text: strings.Repeat("a", 500)})
			}
		},
		"least": func(d *spellbuild.Design) {
			d.Targeting = spellbuild.Targeting{Shape: "line", SizeFt: 5}
			d.Duration = spellbuild.Duration{Unit: "until_dispelled"}
			d.CastingTime = spellbuild.CastingTime{Kind: "minutes", Minutes: 1440}
			d.Components.Material = &spellbuild.Material{Text: strings.Repeat("a", 200)}
			d.Parts = []spellbuild.Part{
				{Type: "light", BrightFt: 120, DimFt: 120}, {Type: "surface", Surface: "fog", Rounds: 100}, {Type: "damage", When: "on_cast", Dice: "99d20", DamageType: "acid"},
			}
		},
		"brief": func(d *spellbuild.Design) {
			d.Duration = spellbuild.Duration{Unit: "rounds", Amount: 1}
			d.CastingTime = spellbuild.CastingTime{Kind: "minutes", Minutes: 1}
			d.Parts = []spellbuild.Part{{Type: "light", BrightFt: 5}, {Type: "surface", Surface: "fog", Rounds: 1}}
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
	if len(spellbuild.Shapes()) != 8 || len(spellbuild.Triggers()) != 5 || len(spellbuild.CreatureTypes()) != 14 {
		t.Fatal("the builder's lists")
	}
}

func TestAHomebrewSpellRunsUnderASlugOfItsEntry(t *testing.T) {
	t.Parallel()
	if got := spellbuild.Slug("0190c7a8-0000-7000-8000-0000000000e1"); got != "hb-0190c7a80000" {
		t.Fatalf("slug = %q", got)
	}
}
