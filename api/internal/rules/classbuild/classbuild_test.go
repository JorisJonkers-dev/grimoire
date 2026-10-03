package classbuild_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/classbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/features"
)

var lists = []string{"bard", "wizard"}

func twenty(f func(level int) int) []int {
	out := make([]int, 20)
	for l := range 20 {
		out[l] = f(l + 1)
	}
	return out
}

// lamplighter casts from its own slot table from the bard's list.
func lamplighter() classbuild.Design {
	slots := make([][]int, 20)
	for l := range 20 {
		slots[l] = make([]int, 9)
		slots[l][0] = 2
		if l >= 3 {
			slots[l][1] = 1
		}
	}
	marks := make([]string, 20)
	for l := range 20 {
		marks[l] = "+" + string(rune('1'+min((l+1)/4, 8)))
	}
	return classbuild.Design{
		HitDie: 8, Primary: []string{"charisma"}, AnyPrimary: false, Saves: []string{"dexterity", "charisma"},
		Armor: []string{"light", "shields"}, Weapons: []string{"simple"}, Skills: 3, SubclassLevel: 3, FeatLevels: []int{4, 8, 12, 16, 19},
		Columns:  []classbuild.Column{{Name: "Wick Marks", Values: marks}},
		Features: []classbuild.Feature{{Level: 1, Name: "Wickcraft", Text: "You tend lanterns that burn without oil."}, {Level: 5, Name: "Bright Step", Text: "Teleport between lit lanterns."}},
		Casting: classbuild.Casting{
			Kind: "slots", Ability: "charisma", SpellList: "bard", Cantrips: twenty(func(int) int { return 2 }), Prepared: twenty(func(l int) int { return 1 + l }),
			Slots: slots, Points: nil, Costs: nil, MaxSpell: nil, Spellbook: false, AfterRest: true,
		},
	}
}

// runeweaver casts with spell points from the wizard's list.
func runeweaver() classbuild.Design {
	d := lamplighter()
	d.HitDie, d.Primary, d.AnyPrimary, d.Columns = 6, []string{"intelligence", "wisdom"}, true, nil
	d.Casting = classbuild.Casting{
		Kind: "points", Ability: "intelligence", SpellList: "wizard", Cantrips: twenty(func(int) int { return 3 }), Prepared: twenty(func(l int) int { return 2 + l }),
		Slots: nil, Points: twenty(func(l int) int { return 2 * l }), Costs: []int{2, 3, 5, 6, 7, 9, 10, 11, 13}, MaxSpell: twenty(func(l int) int { return min((l+1)/2, 9) }),
		Spellbook: true, AfterRest: true,
	}
	return d
}

func TestALamplighterCompilesIntoAClass(t *testing.T) {
	t.Parallel()
	d := lamplighter()
	if err := classbuild.Check(d, lists); err != nil {
		t.Fatal(err)
	}
	c := classbuild.Compile("hb-0190c7a80001", "Lamplighter", d)
	if c.Slug != "hb-0190c7a80001" || c.Name != "Lamplighter" || c.HitDie != 8 || !reflect.DeepEqual(c.Saves, []string{"dexterity", "charisma"}) || c.SpellList != "bard" {
		t.Fatalf("class = %+v", c)
	}
	p := c.Profile
	if p.Slug != c.Slug || p.Name != "Lamplighter" || p.Casting.Kind != rules.CustomSlots || p.Skills != 3 || !p.Casting.AfterRest || p.CantripsAt(1) != 2 || p.PreparedAt(5) != 6 {
		t.Fatalf("profile = %+v", p)
	}
	if !reflect.DeepEqual(p.Proficiencies, rules.Proficiencies{Armor: []string{"Light armor", "Shields"}, Weapons: []string{"Simple weapons"}}) || !reflect.DeepEqual(p.Primary, []rules.Ability{rules.Charisma}) {
		t.Fatalf("training = %+v %v", p.Proficiencies, p.Primary)
	}
	if res := rules.ResourcesAt(p, 8, 4); len(res) != 3 || res[2].Key != "spell-slots-2" {
		t.Fatalf("resources at 4 = %+v", res)
	}
	owner := features.Owner{Kind: "class", Slug: "hb-0190c7a80001"}
	if c.Owner() != owner || len(c.Choices) != 6 {
		t.Fatalf("choices = %+v", c.Choices)
	}
	if sub := c.Choices[0]; !reflect.DeepEqual(sub, features.Choice{Slug: "subclass", Name: "Subclass", Level: 3, Count: 1, Pool: features.Subclass, From: "hb-0190c7a80001", Options: nil}) {
		t.Fatalf("subclass choice = %+v", sub)
	}
	if feat := c.Choices[1]; feat.Slug != "feat" || feat.Level != 4 || feat.Pool != features.FeatCategory || feat.From != "general" {
		t.Fatalf("feat choice = %+v", feat)
	}
	if !reflect.DeepEqual(c.Traits, []classbuild.Trait{{Level: 1, Name: "Wickcraft", Text: "You tend lanterns that burn without oil."}, {Level: 5, Name: "Bright Step", Text: "Teleport between lit lanterns."}}) {
		t.Fatalf("traits = %+v", c.Traits)
	}
	lines := strings.Join(classbuild.Lines(c), "\n")
	for _, w := range []string{
		"Hit Die d8 · Saving throws: Dexterity, Charisma · Armor: Light armor, Shields · Weapons: Simple weapons · 3 skills",
		"Spellcasting: Charisma, from the bard list, its own slot table; prepares after a long rest",
		"Level 1: Wickcraft · Cantrips 2 · Prepared 2 · Slots 2 · Wick Marks +1",
		"Level 3: Subclass · Cantrips 2 · Prepared 4 · Slots 2 · Wick Marks +1",
		"Level 4: Feat · Cantrips 2 · Prepared 5 · Slots 2/1 · Wick Marks +2",
	} {
		if !strings.Contains(lines, w) {
			t.Errorf("lines miss %q:\n%s", w, lines)
		}
	}
}

func TestASlotTableThatStartsLate(t *testing.T) {
	t.Parallel()
	d := lamplighter()
	d.Casting.Slots[0] = make([]int, 9)
	if lines := classbuild.Lines(classbuild.Compile("hb-lamp", "Lamplighter", d)); lines[2] != "Level 1: Wickcraft · Cantrips 2 · Prepared 2 · Wick Marks +1" {
		t.Fatalf("level 1 = %q", lines[2])
	}
}

func TestARuneweaverCastsWithSpellPoints(t *testing.T) {
	t.Parallel()
	d := runeweaver()
	d.Armor = nil
	if err := classbuild.Check(d, lists); err != nil {
		t.Fatal(err)
	}
	c := classbuild.Compile("hb-rune", "Runeweaver", d)
	p := c.Profile
	if p.Casting.Kind != rules.SpellPoints || !p.AnyPrimary || !p.Casting.Spellbook || p.MaxSpellLevel(5) != 3 {
		t.Fatalf("profile = %+v", p)
	}
	if cost, err := rules.PointCost(p, 5, 3); err != nil || cost != 5 {
		t.Fatalf("a 3rd-level spell = %d %v", cost, err)
	}
	if res := rules.ResourcesAt(p, 6, 5); res[len(res)-1] != (rules.Resource{Key: "spell-points", Label: "Spell points", Current: 10, Max: 10}) {
		t.Fatalf("resources = %+v", res)
	}
	lines := strings.Join(classbuild.Lines(c), "\n")
	for _, w := range []string{"Armor: none · Weapons: Simple weapons", "Spellcasting: Intelligence, from the wizard list, spell points (costs 2/3/5/6/7/9/10/11/13 by spell level); keeps a spellbook; prepares after a long rest", "Level 5: Bright Step · Cantrips 3 · Prepared 7 · Points 10, spells to level 3"} {
		if !strings.Contains(lines, w) {
			t.Errorf("lines miss %q:\n%s", w, lines)
		}
	}
	for _, kind := range []string{"full", "half", "pact"} {
		d := runeweaver()
		d.Casting.Kind, d.Casting.Points, d.Casting.Costs, d.Casting.MaxSpell, d.Casting.Spellbook, d.Casting.AfterRest = kind, nil, nil, nil, false, false
		if err := classbuild.Check(d, lists); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if c := classbuild.Compile("hb-x", "X", d); string(c.Profile.Casting.Kind) != kind || !strings.Contains(strings.Join(classbuild.Lines(c), "\n"), kind+" caster") {
			t.Fatalf("%s: %+v", kind, c.Profile.Casting)
		}
	}
	none := runeweaver()
	none.Casting = classbuild.Casting{Kind: "none"}
	none.Armor, none.Weapons = []string{"medium", "heavy"}, []string{"martial"}
	if err := classbuild.Check(none, lists); err != nil {
		t.Fatal(err)
	}
	if c := classbuild.Compile("hb-y", "Y", none); c.Profile.Caster() || !strings.Contains(strings.Join(classbuild.Lines(c), "\n"), "No spellcasting") || len(c.Profile.Proficiencies.Armor) != 2 {
		t.Fatalf("a martial class = %+v", c.Profile)
	}
}

func TestClassDesignsThatDoNotBuild(t *testing.T) {
	t.Parallel()
	for want, change := range map[string]func(*classbuild.Design){
		"Hit Die of d6, d8, d10 or d12":  func(d *classbuild.Design) { d.HitDie = 4 },
		"1 or 2 primary abilities":       func(d *classbuild.Design) { d.Primary = nil },
		"choose real abilities":          func(d *classbuild.Design) { d.Primary = []string{"luck"} },
		"two different saving throws":    func(d *classbuild.Design) { d.Saves = []string{"charisma", "charisma"} },
		"armor from light, medium":       func(d *classbuild.Design) { d.Armor = []string{"mithral"} },
		"weapons from simple, martial":   func(d *classbuild.Design) { d.Weapons = []string{"exotic"} },
		"1 to 6 skills":                  func(d *classbuild.Design) { d.Skills = 0 },
		"subclass at level 1 to 20":      func(d *classbuild.Design) { d.SubclassLevel = 21 },
		"feats at levels 2 to 20, once":  func(d *classbuild.Design) { d.FeatLevels = []int{4, 4} },
		"up to 6 columns":                func(d *classbuild.Design) { d.Columns = make([]classbuild.Column, 7) },
		"name each column":               func(d *classbuild.Design) { d.Columns[0].Name = "" },
		"20 values of up to 20":          func(d *classbuild.Design) { d.Columns[0].Values = d.Columns[0].Values[:19] },
		"1 to 60 features":               func(d *classbuild.Design) { d.Features = nil },
		"each feature from level 1":      func(d *classbuild.Design) { d.Features[0].Level = 0 },
		"name each feature":              func(d *classbuild.Design) { d.Features[0].Name = "" },
		"in up to 2000 characters":       func(d *classbuild.Design) { d.Features[0].Text = strings.Repeat("x", 2001) },
		"how the class casts":            func(d *classbuild.Design) { d.Casting.Kind = "psionic" },
		"the ability it casts with":      func(d *classbuild.Design) { d.Casting.Ability = "" },
		"the spell list it casts from":   func(d *classbuild.Design) { d.Casting.SpellList = "cleric" },
		"cantrips for each of 20 levels": func(d *classbuild.Design) { d.Casting.Cantrips = []int{2} },
		"prepared spells for each of 20": func(d *classbuild.Design) { d.Casting.Prepared[3] = 41 },
		"slots for each of 20 levels":    func(d *classbuild.Design) { d.Casting.Slots = d.Casting.Slots[:5] },
		"0 to 9 slots":                   func(d *classbuild.Design) { d.Casting.Slots[0][0] = 10 },
		"at least one slot by level 20":  func(d *classbuild.Design) { d.Casting.Slots = make([][]int, 20); fill(d.Casting.Slots) },
	} {
		d := lamplighter()
		change(&d)
		if err := classbuild.Check(d, lists); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", want, err)
		}
	}
	for want, change := range map[string]func(*classbuild.Design){
		"points for each of 20 levels":         func(d *classbuild.Design) { d.Casting.Points = nil },
		"0 to 200 points":                      func(d *classbuild.Design) { d.Casting.Points[2] = 201 },
		"a cost of 1 to 50 for each":           func(d *classbuild.Design) { d.Casting.Costs = []int{1} },
		"the highest spell level for each":     func(d *classbuild.Design) { d.Casting.MaxSpell[0] = 10 },
		"reach a spell of level 1 by level 20": func(d *classbuild.Design) { d.Casting.MaxSpell = make([]int, 20) },
	} {
		d := runeweaver()
		change(&d)
		if err := classbuild.Check(d, lists); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", want, err)
		}
	}
}

func TestDesignsAtTheirLimits(t *testing.T) {
	t.Parallel()
	d := lamplighter()
	d.Primary, d.Skills, d.SubclassLevel, d.FeatLevels = []string{"charisma", "wisdom"}, 6, 20, []int{2, 20}
	d.Columns = make([]classbuild.Column, 6)
	for i := range d.Columns {
		d.Columns[i] = classbuild.Column{Name: strings.Repeat("c", 30), Values: make([]string, 20)}
		d.Columns[i].Values[0] = strings.Repeat("v", 20)
	}
	d.Features[0].Text = strings.Repeat("x", 2000)
	d.Casting.Prepared[0], d.Casting.Cantrips[0], d.Casting.Slots[19] = 40, 30, []int{9, 0, 0, 0, 0, 0, 0, 0, 0}
	if err := classbuild.Check(d, lists); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{1, 60} {
		d := lamplighter()
		d.Features = make([]classbuild.Feature, n)
		for i := range d.Features {
			d.Features[i] = classbuild.Feature{Level: 1, Name: "F", Text: ""}
		}
		if err := classbuild.Check(d, lists); err != nil {
			t.Fatalf("%d features: %v", n, err)
		}
	}
	if lines := classbuild.Lines(classbuild.Compile("hb-lamp", "L", lamplighter())); len(lines) != 22 || !strings.HasPrefix(lines[21], "Level 20:") {
		t.Fatalf("the table runs to level 20: %d lines", len(lines))
	}
	r := runeweaver()
	r.Casting.Points[0], r.Casting.Costs[0], r.Casting.MaxSpell = 200, 50, make([]int, 20)
	r.Casting.MaxSpell[19] = 9
	if err := classbuild.Check(r, lists); err != nil {
		t.Fatal(err)
	}
}

func fill(rows [][]int) {
	for i := range rows {
		rows[i] = make([]int, 9)
	}
}

func TestMergingAddsAClassToTheCatalogWithoutTouchingIt(t *testing.T) {
	t.Parallel()
	base := features.Catalog{
		Resources: map[string]features.Resource{}, Scales: map[string]features.Named{},
		Choices: map[features.Owner][]features.Choice{{Kind: "class", Slug: "fighter"}: {{Slug: "subclass"}}}, Prerequisites: map[features.Owner][]features.Requirement{},
	}
	c := classbuild.Compile("hb-lamp", "Lamplighter", lamplighter())
	cat := classbuild.Merge(base, []classbuild.Class{c})
	if len(base.Choices) != 1 || len(cat.Choices[c.Owner()]) != 6 || len(cat.Choices) != 2 {
		t.Fatalf("choices = %v", cat.Choices)
	}
}
