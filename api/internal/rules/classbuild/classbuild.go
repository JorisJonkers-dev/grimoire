// Package classbuild turns a homebrew class's design, its Hit Die, training, level table with custom
// columns, features, subclass and feat levels, and spellcasting (an SRD kind, its own slot table or
// spell points), into the profile the rules read and the feature data levelling up reads.
package classbuild

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/features"
)

// DesignError is why a design cannot be built, fit to show its author.
type DesignError string

func (e DesignError) Error() string { return string(e) }

// Design is a homebrew class as its author builds it.
type Design struct {
	HitDie        int       `json:"hitDie"`
	Primary       []string  `json:"primary"`
	AnyPrimary    bool      `json:"anyPrimary,omitempty"`
	Saves         []string  `json:"saves"`
	Armor         []string  `json:"armor"`
	Weapons       []string  `json:"weapons"`
	Skills        int       `json:"skills"`
	SubclassLevel int       `json:"subclassLevel"`
	FeatLevels    []int     `json:"featLevels"`
	Columns       []Column  `json:"columns"`
	Features      []Feature `json:"features"`
	Casting       Casting   `json:"casting"`
}

// Column is a custom column of the level table: one value for each of the 20 levels.
type Column struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// Feature is gained at a class level.
type Feature struct {
	Level int    `json:"level"`
	Name  string `json:"name"`
	Text  string `json:"text"`
}

// Casting is how the class casts: none, an SRD kind (full, half, pact) or its own slot table or spell
// points, from an SRD class's spell list. Tables run over the 20 levels; slots and costs over the nine
// spell levels.
type Casting struct {
	Kind      string  `json:"kind"`
	Ability   string  `json:"ability,omitempty"`
	SpellList string  `json:"spellList,omitempty"`
	Cantrips  []int   `json:"cantrips,omitempty"`
	Prepared  []int   `json:"prepared,omitempty"`
	Slots     [][]int `json:"slots,omitempty"`
	Points    []int   `json:"points,omitempty"`
	Costs     []int   `json:"costs,omitempty"`
	MaxSpell  []int   `json:"maxSpell,omitempty"`
	Spellbook bool    `json:"spellbook,omitempty"`
	AfterRest bool    `json:"afterRest,omitempty"`
}

// Trait is a feature as the sheet shows it.
type Trait struct {
	Level int
	Name  string
	Text  string
}

// Class is a built design: what the builder offers, what the rules read, its features and the choices
// its levels ask for.
type Class struct {
	Slug      string
	Name      string
	HitDie    int
	Saves     []string
	SpellList string
	Ability   string
	Profile   rules.Class
	Traits    []Trait
	Choices   []features.Choice
	Columns   []Column
	featAt    []int
	subAt     int
}

// Owner is the class as feature data names it.
func (c Class) Owner() features.Owner { return features.Owner{Kind: "class", Slug: c.Slug} }

func abilities() []string {
	return []string{"strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"}
}

func armors() []string  { return []string{"light", "medium", "heavy", "shields"} }
func weapons() []string { return []string{"simple", "martial"} }
func kinds() []string   { return []string{"none", "full", "half", "pact", "slots", "points"} }

func armorName(a string) string {
	if a == "shields" {
		return "Shields"
	}
	return title(a) + " armor"
}

func title(s string) string { return strings.ToUpper(s[:1]) + s[1:] }

func sized(s string, most int) bool {
	n := len([]rune(strings.TrimSpace(s)))
	return n > 0 && n <= most
}

func all[T any](xs []T, ok func(T) bool) bool {
	for _, x := range xs {
		if !ok(x) {
			return false
		}
	}
	return true
}

func between(lo, hi int) func(int) bool { return func(n int) bool { return n >= lo && n <= hi } }

// Check validates a design against the spell lists a class may cast from.
func Check(d Design, spellLists []string) error {
	for _, check := range []func(Design) error{basics, training, table, featureRows} {
		if err := check(d); err != nil {
			return err
		}
	}
	return casting(d.Casting, spellLists)
}

func basics(d Design) error {
	known := func(a string) bool { return slices.Contains(abilities(), a) }
	switch {
	case !slices.Contains([]int{6, 8, 10, 12}, d.HitDie):
		return DesignError("give it a Hit Die of d6, d8, d10 or d12")
	case len(d.Primary) < 1 || len(d.Primary) > 2:
		return DesignError("choose 1 or 2 primary abilities")
	case !all(d.Primary, known) || !all(d.Saves, known):
		return DesignError("choose real abilities: " + strings.Join(abilities(), ", "))
	case len(d.Saves) != 2 || d.Saves[0] == d.Saves[1]:
		return DesignError("choose two different saving throws")
	}
	return nil
}

func training(d Design) error {
	switch {
	case !all(d.Armor, func(a string) bool { return slices.Contains(armors(), a) }):
		return DesignError("choose armor from " + strings.Join(armors(), ", "))
	case !all(d.Weapons, func(w string) bool { return slices.Contains(weapons(), w) }):
		return DesignError("choose weapons from " + strings.Join(weapons(), ", "))
	case d.Skills < 1 || d.Skills > 6:
		return DesignError("let it pick 1 to 6 skills")
	}
	return nil
}

func table(d Design) error {
	seen := make([]int, 0, len(d.FeatLevels))
	for _, l := range d.FeatLevels {
		if l < 2 || l > 20 || slices.Contains(seen, l) {
			return DesignError("grant feats at levels 2 to 20, once each")
		}
		seen = append(seen, l)
	}
	switch {
	case d.SubclassLevel < 1 || d.SubclassLevel > 20:
		return DesignError("choose its subclass at level 1 to 20")
	case len(d.Columns) > 6:
		return DesignError("keep it to up to 6 columns")
	}
	for _, c := range d.Columns {
		if !sized(c.Name, 30) {
			return DesignError("name each column in up to 30 characters")
		}
		if len(c.Values) != 20 || !all(c.Values, func(v string) bool { return len([]rune(v)) <= 20 }) {
			return DesignError("give each column 20 values of up to 20 characters")
		}
	}
	return nil
}

func featureRows(d Design) error {
	if len(d.Features) < 1 || len(d.Features) > 60 {
		return DesignError("give it 1 to 60 features")
	}
	for _, f := range d.Features {
		switch {
		case f.Level < 1 || f.Level > 20:
			return DesignError("gain each feature from level 1 to 20")
		case !sized(f.Name, 60):
			return DesignError("name each feature in up to 60 characters")
		case len([]rune(f.Text)) > 2000:
			return DesignError("describe " + f.Name + " in up to 2000 characters")
		}
	}
	return nil
}

func casting(c Casting, lists []string) error {
	switch {
	case !slices.Contains(kinds(), c.Kind):
		return DesignError("choose how the class casts: " + strings.Join(kinds(), ", "))
	case c.Kind == "none":
		return nil
	case !slices.Contains(abilities(), c.Ability):
		return DesignError("choose the ability it casts with")
	case !slices.Contains(lists, c.SpellList):
		return DesignError("choose the spell list it casts from: " + strings.Join(lists, ", "))
	case len(c.Cantrips) != 20 || !all(c.Cantrips, between(0, 30)):
		return DesignError("give it cantrips for each of 20 levels, 0 to 30")
	case len(c.Prepared) != 20 || !all(c.Prepared, between(0, 40)):
		return DesignError("give it prepared spells for each of 20 levels, 0 to 40")
	case c.Kind == "slots":
		return slotTable(c.Slots)
	case c.Kind == "points":
		return points(c)
	}
	return nil
}

func slotTable(slots [][]int) error {
	if len(slots) != 20 || !all(slots, func(row []int) bool { return len(row) == 9 && all(row, between(0, 9)) }) {
		return DesignError("give it slots for each of 20 levels, nine spell levels each, 0 to 9 slots")
	}
	if !slices.ContainsFunc(slots[19], func(n int) bool { return n > 0 }) {
		return DesignError("give it at least one slot by level 20")
	}
	return nil
}

func points(c Casting) error {
	switch {
	case len(c.Points) != 20 || !all(c.Points, between(0, 200)):
		return DesignError("give it points for each of 20 levels, 0 to 200 points")
	case len(c.Costs) != 9 || !all(c.Costs, between(1, 50)):
		return DesignError("give a cost of 1 to 50 for each of the nine spell levels")
	case len(c.MaxSpell) != 20 || !all(c.MaxSpell, between(0, 9)):
		return DesignError("give the highest spell level for each of 20 levels, 0 to 9")
	case c.MaxSpell[19] < 1:
		return DesignError("let it reach a spell of level 1 by level 20")
	}
	return nil
}

// Compile builds a checked design into the class a slug names.
func Compile(slug, name string, d Design) Class {
	out := Class{
		Slug: slug, Name: name, HitDie: d.HitDie, Saves: slices.Clone(d.Saves), SpellList: d.Casting.SpellList, Ability: d.Casting.Ability,
		Profile: profile(slug, name, d), Traits: []Trait{}, Columns: slices.Clone(d.Columns), featAt: slices.Clone(d.FeatLevels), subAt: d.SubclassLevel,
		Choices: []features.Choice{{Slug: "subclass", Name: "Subclass", Level: d.SubclassLevel, Count: 1, Pool: features.Subclass, From: slug, Options: nil}},
	}
	for _, l := range d.FeatLevels {
		out.Choices = append(out.Choices, features.Choice{Slug: "feat", Name: "Ability Score Improvement or feat", Level: l, Count: 1, Pool: features.FeatCategory, From: "general", Options: nil})
	}
	for _, f := range d.Features {
		out.Traits = append(out.Traits, Trait{Level: f.Level, Name: f.Name, Text: strings.TrimSpace(f.Text)})
	}
	return out
}

func profile(slug, name string, d Design) rules.Class {
	c := d.Casting
	out := rules.Class{
		Slug: slug, Name: name, Primary: []rules.Ability{}, AnyPrimary: d.AnyPrimary, Skills: d.Skills,
		Proficiencies: rules.Proficiencies{Armor: []string{}, Weapons: []string{}},
		Casting: rules.Spellcasting{
			Kind: rules.CasterKind(c.Kind), Cantrips: [20]int{}, Prepared: [20]int{}, Slots: [20][9]int{}, Points: [20]int{}, Costs: [9]int{},
			MaxSpell: [20]int{}, Spellbook: c.Spellbook, AfterRest: c.AfterRest,
		},
	}
	for _, a := range d.Primary {
		out.Primary = append(out.Primary, rules.Ability(a))
	}
	for _, a := range armors() {
		if slices.Contains(d.Armor, a) {
			out.Proficiencies.Armor = append(out.Proficiencies.Armor, armorName(a))
		}
	}
	for _, w := range weapons() {
		if slices.Contains(d.Weapons, w) {
			out.Proficiencies.Weapons = append(out.Proficiencies.Weapons, title(w)+" weapons")
		}
	}
	copy(out.Casting.Cantrips[:], c.Cantrips)
	copy(out.Casting.Prepared[:], c.Prepared)
	copy(out.Casting.Points[:], c.Points)
	copy(out.Casting.Costs[:], c.Costs)
	copy(out.Casting.MaxSpell[:], c.MaxSpell)
	for l, row := range c.Slots {
		copy(out.Casting.Slots[l][:], row)
	}
	return out
}

// Merge adds classes' choices to a catalog, leaving the catalog given as it was.
func Merge(cat features.Catalog, classes []Class) features.Catalog {
	out := features.Catalog{Resources: cat.Resources, Scales: cat.Scales, Choices: maps.Clone(cat.Choices), Prerequisites: cat.Prerequisites}
	for _, c := range classes {
		out.Choices[c.Owner()] = c.Choices
	}
	return out
}

// Lines are a class as its author reads it back: its training, its spellcasting and its level table.
func Lines(c Class) []string {
	p := c.Profile
	saves := make([]string, 0, len(c.Saves))
	for _, s := range c.Saves {
		saves = append(saves, title(s))
	}
	head := "Hit Die d" + strconv.Itoa(c.HitDie) + " · Saving throws: " + strings.Join(saves, ", ") + " · Armor: " + listOr(p.Proficiencies.Armor) +
		" · Weapons: " + listOr(p.Proficiencies.Weapons) + " · " + strconv.Itoa(p.Skills) + " skills"
	out := []string{head, castingLine(c)}
	for l := range 20 {
		out = append(out, row(c, l+1))
	}
	return out
}

func listOr(xs []string) string {
	if len(xs) == 0 {
		return "none"
	}
	return strings.Join(xs, ", ")
}

func castingLine(c Class) string {
	k := c.Profile.Casting
	if !c.Profile.Caster() {
		return "No spellcasting"
	}
	line := "Spellcasting: " + title(c.Ability) + ", from the " + c.SpellList + " list, "
	switch k.Kind {
	case rules.CustomSlots:
		line += "its own slot table"
	case rules.SpellPoints:
		costs := make([]string, 0, 9)
		for _, n := range k.Costs {
			costs = append(costs, strconv.Itoa(n))
		}
		line += "spell points (costs " + strings.Join(costs, "/") + " by spell level)"
	case rules.FullCaster, rules.HalfCaster, rules.PactCaster, rules.NoCaster:
		line += "a " + string(k.Kind) + " caster"
	}
	if k.Spellbook {
		line += "; keeps a spellbook"
	}
	if k.AfterRest {
		line += "; prepares after a long rest"
	}
	return line
}

func row(c Class, level int) string {
	var items []string
	var names []string
	for _, t := range c.Traits {
		if t.Level == level {
			names = append(names, t.Name)
		}
	}
	if len(names) > 0 {
		items = append(items, strings.Join(names, ", "))
	}
	if c.subAt == level {
		items = append(items, "Subclass")
	}
	if slices.Contains(c.featAt, level) {
		items = append(items, "Feat")
	}
	if p := c.Profile; p.Caster() {
		items = append(items, "Cantrips "+strconv.Itoa(p.CantripsAt(level)), "Prepared "+strconv.Itoa(p.PreparedAt(level)))
		items = append(items, pool(p, level)...)
	}
	for _, col := range c.Columns {
		items = append(items, col.Name+" "+col.Values[level-1])
	}
	return "Level " + strconv.Itoa(level) + ": " + strings.Join(items, " · ")
}

func pool(p rules.Class, level int) []string {
	if p.Casting.Kind == rules.SpellPoints {
		return []string{"Points " + strconv.Itoa(p.Casting.Points[level-1]) + ", spells to level " + strconv.Itoa(p.MaxSpellLevel(level))}
	}
	slots := p.SlotsAt(level)
	last := 0
	for i, n := range slots {
		if n > 0 {
			last = i + 1
		}
	}
	if last == 0 {
		return nil
	}
	parts := make([]string, 0, last)
	for _, n := range slots[:last] {
		parts = append(parts, strconv.Itoa(n))
	}
	return []string{"Slots " + strings.Join(parts, "/")}
}
