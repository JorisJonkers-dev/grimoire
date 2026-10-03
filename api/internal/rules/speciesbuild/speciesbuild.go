// Package speciesbuild turns a homebrew species's design, its sizes, creature type, speeds, senses,
// resistances, traits, innate spells by character level and lineages, into the species a character can
// be built from: one for each lineage, with the traits the sheet shows.
package speciesbuild

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// DesignError is why a design cannot be built, fit to show its author.
type DesignError string

func (e DesignError) Error() string { return string(e) }

// Design is a homebrew species as its author builds it.
type Design struct {
	Sizes        []string  `json:"sizes"`
	CreatureType string    `json:"creatureType"`
	SpeedFt      int       `json:"speedFt"`
	Speeds       []Speed   `json:"speeds"`
	Senses       []Sense   `json:"senses"`
	Resistances  []string  `json:"resistances"`
	Traits       []Trait   `json:"traits"`
	Spells       []Spell   `json:"spells"`
	Lineages     []Lineage `json:"lineages"`
}

// Speed is a speed beside walking.
type Speed struct {
	Kind string `json:"kind"`
	Feet int    `json:"feet"`
}

// Sense is a special sense and its range.
type Sense struct {
	Kind string `json:"kind"`
	Feet int    `json:"feet"`
}

// Trait is a named trait every member of the species has.
type Trait struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// Spell is an innate spell gained at a character level, cast at will or once per long rest.
type Spell struct {
	Level int    `json:"level"`
	Spell string `json:"spell"`
	Name  string `json:"name"`
	Uses  string `json:"uses"`
}

// Lineage is one branch of the species, picked at creation, with its own trait and spells.
type Lineage struct {
	Name   string  `json:"name"`
	Text   string  `json:"text"`
	Spells []Spell `json:"spells"`
}

// Gained is a trait as the sheet shows it, from the character level it is gained at; 0 is always.
type Gained struct {
	Name  string
	Level int
	Text  string
}

// Option is a species a character can be built from: the species itself, or one of its lineages.
type Option struct {
	Slug      string
	Name      string
	SpeedFeet int
	Traits    []Gained
}

var (
	slugPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,79}$`)
	notWord     = regexp.MustCompile(`[^a-z0-9]+`)
)

func sizes() []string { return []string{"tiny", "small", "medium", "large", "huge"} }

func creatureTypes() []string {
	return []string{"aberration", "beast", "celestial", "construct", "dragon", "elemental", "fey", "fiend", "giant", "humanoid", "monstrosity", "ooze", "plant", "undead"}
}

func damageTypes() []string {
	return []string{"acid", "bludgeoning", "cold", "fire", "force", "lightning", "necrotic", "piercing", "poison", "psychic", "radiant", "slashing", "thunder"}
}

func key(name string) string {
	return strings.Trim(notWord.ReplaceAllString(strings.ToLower(name), "-"), "-")
}

func title(s string) string { return strings.ToUpper(s[:1]) + s[1:] }

func sized(s string, most int) bool {
	n := len([]rune(strings.TrimSpace(s)))
	return n > 0 && n <= most
}

// Check validates a design.
func Check(d Design) error {
	for _, check := range []func(Design) error{body, movement, traits} {
		if err := check(d); err != nil {
			return err
		}
	}
	for _, s := range d.Spells {
		if err := spell(s); err != nil {
			return err
		}
	}
	return lineages(d.Lineages)
}

func body(d Design) error {
	switch {
	case len(d.Sizes) < 1 || len(d.Sizes) > 2:
		return DesignError("choose one or two sizes")
	case slices.ContainsFunc(d.Sizes, func(s string) bool { return !slices.Contains(sizes(), s) }):
		return DesignError("choose sizes from " + strings.Join(sizes(), ", "))
	case !slices.Contains(creatureTypes(), d.CreatureType):
		return DesignError("choose a creature type: " + strings.Join(creatureTypes(), ", "))
	case slices.ContainsFunc(d.Resistances, func(r string) bool { return !slices.Contains(damageTypes(), r) }):
		return DesignError("resist real damage types: " + strings.Join(damageTypes(), ", "))
	}
	return nil
}

func movement(d Design) error {
	if d.SpeedFt < 10 || d.SpeedFt > 60 {
		return DesignError("give it a Speed of 10 to 60 feet")
	}
	for _, s := range d.Speeds {
		switch {
		case !slices.Contains([]string{"climb", "fly", "swim", "burrow"}, s.Kind):
			return DesignError("give it climb, fly, swim or burrow speeds")
		case s.Feet < 5 || s.Feet > 120:
			return DesignError("make each speed 5 to 120 feet")
		}
	}
	for _, s := range d.Senses {
		switch {
		case !slices.Contains([]string{"darkvision", "blindsight", "tremorsense", "truesight"}, s.Kind):
			return DesignError("give it darkvision, blindsight, tremorsense or truesight")
		case s.Feet < 5 || s.Feet > 300:
			return DesignError("make each sense 5 to 300 feet")
		}
	}
	return nil
}

func traits(d Design) error {
	if len(d.Traits) > 20 {
		return DesignError("keep it to up to 20 traits")
	}
	for _, t := range d.Traits {
		if !sized(t.Name, 60) {
			return DesignError("name each trait in up to 60 characters")
		}
		if len([]rune(t.Text)) > 2000 {
			return DesignError("describe " + t.Name + " in up to 2000 characters")
		}
	}
	return nil
}

func spell(s Spell) error {
	switch {
	case s.Level < 1 || s.Level > 20:
		return DesignError("gain each spell from character level 1 to 20")
	case !slugPattern.MatchString(s.Spell):
		return DesignError("name each spell by its slug, like fog-cloud")
	case !sized(s.Name, 60):
		return DesignError("name each spell in up to 60 characters")
	case s.Uses != "at_will" && s.Uses != "long_rest":
		return DesignError("cast each spell at will, or once per long rest")
	}
	return nil
}

func lineages(ls []Lineage) error {
	if len(ls) > 10 {
		return DesignError("keep it to up to 10 lineages")
	}
	seen := make([]string, 0, len(ls))
	for _, l := range ls {
		switch {
		case !sized(l.Name, 40) || key(l.Name) == "":
			return DesignError("name each lineage in up to 40 characters")
		case slices.Contains(seen, key(l.Name)):
			return DesignError("there are two lineages by one name: " + l.Name)
		case len([]rune(l.Text)) > 2000:
			return DesignError("describe the " + l.Name + " lineage in up to 2000 characters")
		}
		seen = append(seen, key(l.Name))
		for _, s := range l.Spells {
			if err := spell(s); err != nil {
				return DesignError("a " + l.Name + " lineage spell: " + err.Error())
			}
		}
	}
	return nil
}

// Compile builds a checked design into the species a slug names: one option, or one for each lineage.
func Compile(slug, name string, d Design) []Option {
	shared := sharedTraits(d)
	if len(d.Lineages) == 0 {
		return []Option{{Slug: slug, Name: name, SpeedFeet: d.SpeedFt, Traits: append(shared, spellTraits(d.Spells)...)}}
	}
	out := make([]Option, 0, len(d.Lineages))
	for _, l := range d.Lineages {
		traits := append(slices.Clone(shared), Gained{Name: l.Name + " lineage", Level: 0, Text: strings.TrimSpace(l.Text)})
		traits = append(traits, spellTraits(append(slices.Clone(l.Spells), d.Spells...))...)
		out = append(out, Option{Slug: slug + "-" + key(l.Name), Name: name + ", " + l.Name + " lineage", SpeedFeet: d.SpeedFt, Traits: traits})
	}
	return out
}

func sharedTraits(d Design) []Gained {
	named := make([]string, 0, len(d.Sizes))
	for _, s := range d.Sizes {
		named = append(named, title(s))
	}
	size := named[0] + "."
	if len(named) == 2 {
		size = named[0] + " or " + named[1] + ", chosen when you select this species."
	}
	speed := []string{strconv.Itoa(d.SpeedFt) + " feet"}
	for _, s := range d.Speeds {
		speed = append(speed, title(s.Kind)+" "+strconv.Itoa(s.Feet)+" feet")
	}
	out := []Gained{
		{Name: "Size", Level: 0, Text: size},
		{Name: "Creature Type", Level: 0, Text: title(d.CreatureType) + "."},
		{Name: "Speed", Level: 0, Text: strings.Join(speed, "; ") + "."},
	}
	if len(d.Senses) > 0 {
		senses := make([]string, 0, len(d.Senses))
		for _, s := range d.Senses {
			senses = append(senses, title(s.Kind)+" "+strconv.Itoa(s.Feet)+" feet")
		}
		out = append(out, Gained{Name: "Senses", Level: 0, Text: strings.Join(senses, "; ") + "."})
	}
	if len(d.Resistances) > 0 {
		kinds := make([]string, 0, len(d.Resistances))
		for _, r := range d.Resistances {
			kinds = append(kinds, title(r))
		}
		out = append(out, Gained{Name: "Resistances", Level: 0, Text: "You have Resistance to " + strings.Join(kinds, ", ") + " damage."})
	}
	for _, t := range d.Traits {
		out = append(out, Gained{Name: t.Name, Level: 0, Text: strings.TrimSpace(t.Text)})
	}
	return out
}

func spellTraits(spells []Spell) []Gained {
	out := make([]Gained, 0, len(spells))
	for _, s := range spells {
		text := "You can cast " + s.Name + " at will."
		if s.Uses == "long_rest" {
			text = "You can cast " + s.Name + " once, and again after a Long Rest."
		}
		out = append(out, Gained{Name: s.Name, Level: s.Level, Text: text})
	}
	return out
}

// Lines are a species as its author reads it back: each option and its traits.
func Lines(opts []Option) []string {
	var out []string
	for _, o := range opts {
		speed := ""
		for _, t := range o.Traits {
			if t.Name == "Speed" {
				speed = t.Text
			}
		}
		out = append(out, o.Name+": "+speed)
		for _, t := range o.Traits {
			line := t.Name + ". " + t.Text
			if t.Level > 0 {
				line = "Level " + strconv.Itoa(t.Level) + ": " + line
			}
			out = append(out, line)
		}
	}
	return out
}
