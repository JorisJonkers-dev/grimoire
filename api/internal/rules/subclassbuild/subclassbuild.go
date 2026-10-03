// Package subclassbuild turns a homebrew subclass's design, level-gated Features with their uses,
// Resources and choices, into the feature data the level-up wizard and the sheet read.
package subclassbuild

import (
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/features"
)

// DesignError is why a design cannot be built, fit to show its author.
type DesignError string

func (e DesignError) Error() string { return string(e) }

// Design is a homebrew subclass as its author builds it.
type Design struct {
	Class     string     `json:"class"`
	Features  []Feature  `json:"features"`
	Resources []Resource `json:"resources"`
	Choices   []Choice   `json:"choices"`
}

// Feature is gained at a class level. It may spend a use of one of the design's Resources, by key, and
// let its bearer cast a spell.
type Feature struct {
	Level     int    `json:"level"`
	Name      string `json:"name"`
	Text      string `json:"text"`
	Uses      string `json:"uses,omitempty"`
	Spell     string `json:"spell,omitempty"`
	SpellName string `json:"spellName,omitempty"`
}

// Resource is a pool of uses: a fixed number, some per class level, an ability modifier or the
// Proficiency Bonus, from a level on, coming back on a rest or at dawn.
type Resource struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Basis     string `json:"basis"`
	Amount    int    `json:"amount,omitempty"`
	Ability   string `json:"ability,omitempty"`
	FromLevel int    `json:"fromLevel"`
	Die       string `json:"die,omitempty"`
	Recharge  string `json:"recharge"`
}

// Bases a Resource counts its uses by.
const (
	Fixed       = "fixed"
	ClassLevel  = "class_level"
	Ability     = "ability"
	Proficiency = "proficiency"
)

// Choice is a pick from listed options at a level.
type Choice struct {
	Level   int      `json:"level"`
	Name    string   `json:"name"`
	Count   int      `json:"count"`
	Options []string `json:"options"`
}

// Trait is a Feature as the sheet shows it.
type Trait struct {
	Level int
	Name  string
	Text  string
}

// Subclass is a built design: its traits by level and the feature data it adds.
type Subclass struct {
	Slug      string
	Name      string
	Class     string
	Traits    []Trait
	Resources []features.Resource
	Choices   []features.Choice
}

var (
	keyPattern  = regexp.MustCompile(`^[a-z][a-z0-9-]{0,29}$`)
	slugPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,79}$`)
	diePattern  = regexp.MustCompile(`^d(4|6|8|10|12)$`)
	notWord     = regexp.MustCompile(`[^a-z0-9]+`)
)

func abilities() []string {
	return []string{"strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"}
}

func bases() []string { return []string{Fixed, ClassLevel, Ability, Proficiency} }

func recharges() []string { return []string{"short_rest", "long_rest", "dawn"} }

// key is a name as a slug: Lantern Style is lantern-style.
func key(name string) string {
	return strings.Trim(notWord.ReplaceAllString(strings.ToLower(name), "-"), "-")
}

func sized(s string, most int) bool {
	n := len([]rune(strings.TrimSpace(s)))
	return n > 0 && n <= most
}

// Check validates a design against the classes a subclass can belong to.
func Check(d Design, classes []string) error {
	switch {
	case !slices.Contains(classes, d.Class):
		return DesignError("choose the class this subclass belongs to: " + strings.Join(classes, ", "))
	case len(d.Features) == 0 || len(d.Features) > 40:
		return DesignError("give it 1 to 40 features")
	case len(d.Resources) > 10:
		return DesignError("keep it to up to 10 Resources")
	case len(d.Choices) > 10:
		return DesignError("keep it to up to 10 choices")
	}
	keys := make([]string, 0, len(d.Resources))
	for _, r := range d.Resources {
		if err := resource(r, keys); err != nil {
			return err
		}
		keys = append(keys, r.Key)
	}
	for _, f := range d.Features {
		if err := feature(f, keys); err != nil {
			return err
		}
	}
	names := make([]string, 0, len(d.Choices))
	for _, c := range d.Choices {
		if err := choice(c, names); err != nil {
			return err
		}
		names = append(names, key(c.Name))
	}
	return nil
}

func feature(f Feature, keys []string) error {
	switch {
	case f.Level < 1 || f.Level > 20:
		return DesignError("gain each feature from level 1 to 20")
	case !sized(f.Name, 60):
		return DesignError("name each feature in up to 60 characters")
	case len([]rune(f.Text)) > 1000:
		return DesignError("describe " + f.Name + " in up to 1000 characters")
	case f.Uses != "" && !slices.Contains(keys, f.Uses):
		return DesignError(f.Name + " uses no Resource called " + f.Uses)
	case (f.Spell != "" || f.SpellName != "") && !slugPattern.MatchString(f.Spell):
		return DesignError("name the spell " + f.Name + " casts by its slug, like light")
	case f.Spell != "" && !sized(f.SpellName, 60):
		return DesignError("name the spell " + f.Name + " casts")
	}
	return nil
}

func resource(r Resource, keys []string) error {
	switch {
	case !keyPattern.MatchString(r.Key):
		return DesignError("key each Resource with a short slug, like embers")
	case slices.Contains(keys, r.Key):
		return DesignError("use each Resource key once each")
	case !sized(r.Name, 60):
		return DesignError("name each Resource in up to 60 characters")
	case !slices.Contains(bases(), r.Basis):
		return DesignError("choose how " + r.Name + " counts its uses: " + strings.Join(bases(), ", "))
	case r.Basis == Fixed && (r.Amount < 1 || r.Amount > 20):
		return DesignError("give " + r.Name + " 1 to 20 uses")
	case r.Basis == ClassLevel && (r.Amount < 1 || r.Amount > 10):
		return DesignError("give " + r.Name + " 1 to 10 uses per level")
	case r.Basis == Ability && !slices.Contains(abilities(), r.Ability):
		return DesignError("choose the ability " + r.Name + " reads")
	case r.FromLevel < 1 || r.FromLevel > 20:
		return DesignError(r.Name + " comes from level 1 to 20")
	case r.Die != "" && !diePattern.MatchString(r.Die):
		return DesignError(r.Name + " rolls a die from d4 to d12")
	case !slices.Contains(recharges(), r.Recharge):
		return DesignError("say when " + r.Name + " comes back on: " + strings.Join(recharges(), ", "))
	}
	return nil
}

func choice(c Choice, names []string) error {
	switch {
	case c.Level < 1 || c.Level > 20:
		return DesignError("make a choice at level 1 to 20")
	case !sized(c.Name, 40) || key(c.Name) == "":
		return DesignError("name each choice in up to 40 characters")
	case slices.Contains(names, key(c.Name)):
		return DesignError("there are two choices by one name: " + c.Name)
	case len(c.Options) < 2 || len(c.Options) > 20:
		return DesignError("offer 2 to 20 options for " + c.Name)
	}
	seen := make([]string, 0, len(c.Options))
	for _, o := range c.Options {
		if !sized(o, 60) || key(o) == "" {
			return DesignError("name each option of " + c.Name + " in up to 60 characters")
		}
		if slices.Contains(seen, key(o)) {
			return DesignError("name each option once in " + c.Name)
		}
		seen = append(seen, key(o))
	}
	if c.Count < 1 || c.Count > len(c.Options) {
		return DesignError("let players pick 1 to " + strconv.Itoa(len(c.Options)) + " options for " + c.Name)
	}
	return nil
}

// Compile builds a checked design into the subclass a slug names.
func Compile(slug, name string, d Design) Subclass {
	owner := features.Owner{Kind: "subclass", Slug: slug}
	out := Subclass{Slug: slug, Name: name, Class: d.Class, Traits: []Trait{}, Resources: []features.Resource{}, Choices: []features.Choice{}}
	names := map[string]string{}
	for _, r := range d.Resources {
		names[r.Key] = r.Name
		out.Resources = append(out.Resources, compileResource(owner, r))
	}
	for _, f := range d.Features {
		out.Traits = append(out.Traits, Trait{Level: f.Level, Name: f.Name, Text: traitText(f, names[f.Uses])})
	}
	for _, c := range d.Choices {
		opts := make([]features.Option, 0, len(c.Options))
		for _, o := range c.Options {
			opts = append(opts, features.Option{Slug: key(o), Name: strings.TrimSpace(o)})
		}
		out.Choices = append(out.Choices, features.Choice{
			Slug: slug + "-" + key(c.Name), Name: c.Name, Level: c.Level, Count: c.Count, Pool: features.Listed, From: "", Options: opts,
		})
	}
	return out
}

func compileResource(owner features.Owner, r Resource) features.Resource {
	out := features.Resource{
		Slug: owner.Slug + "-" + r.Key, Name: r.Name, Owner: owner, Basis: features.ByTable, Multiplier: 0, Ability: r.Ability,
		FromLevel: r.FromLevel, Table: nil, Die: nil, Recharges: []features.Recharge{{On: features.Event(r.Recharge), FromLevel: 0, Amount: features.All, RollAtLeast: 0}},
	}
	switch r.Basis {
	case Fixed:
		out.Table = features.Scale[int]{{Level: r.FromLevel, Value: r.Amount}}
	case ClassLevel:
		out.Basis, out.Multiplier = features.ByClassLevel, r.Amount
	case Ability:
		out.Basis = features.ByAbility
	default:
		out.Basis = features.ByProficiency
	}
	if r.Die != "" {
		out.Die = features.Scale[string]{{Level: r.FromLevel, Value: r.Die}}
	}
	if r.Recharge == "short_rest" {
		out.Recharges = append(out.Recharges, features.Recharge{On: features.LongRest, FromLevel: 0, Amount: features.All, RollAtLeast: 0})
	}
	return out
}

func traitText(f Feature, uses string) string {
	text := strings.TrimSpace(f.Text)
	switch {
	case f.Spell != "" && uses != "":
		text += " You can cast " + f.SpellName + " with it, spending a use of " + uses + "."
	case f.Spell != "":
		text += " You can cast " + f.SpellName + " with it."
	case uses != "":
		text += " It spends a use of " + uses + "."
	}
	return strings.TrimSpace(text)
}

// Merge adds subclasses' Resources and choices to a catalog, leaving the catalog given as it was.
func Merge(cat features.Catalog, subs []Subclass) features.Catalog {
	out := features.Catalog{
		Resources: maps.Clone(cat.Resources), Scales: cat.Scales, Choices: maps.Clone(cat.Choices), Prerequisites: cat.Prerequisites,
	}
	for _, sc := range subs {
		for _, r := range sc.Resources {
			out.Resources[r.Slug] = r
		}
		out.Choices[features.Owner{Kind: "subclass", Slug: sc.Slug}] = sc.Choices
	}
	return out
}

// Lines are a subclass as its author reads it back: its class, features, choices and Resources.
func Lines(sc Subclass) []string {
	out := []string{title(sc.Class) + " subclass"}
	for _, t := range sc.Traits {
		out = append(out, "Level "+strconv.Itoa(t.Level)+": "+t.Name+". "+t.Text)
	}
	for _, c := range sc.Choices {
		names := make([]string, 0, len(c.Options))
		for _, o := range c.Options {
			names = append(names, o.Name)
		}
		out = append(out, "Level "+strconv.Itoa(c.Level)+" choice: "+c.Name+", pick "+strconv.Itoa(c.Count)+" of "+strings.Join(names, ", ")+".")
	}
	for _, r := range sc.Resources {
		out = append(out, resourceLine(r, sc.Class))
	}
	return out
}

func resourceLine(r features.Resource, class string) string {
	var uses string
	switch r.Basis {
	case features.ByTable:
		uses = strconv.Itoa(r.Table[0].Value) + " uses"
	case features.ByClassLevel:
		uses = strconv.Itoa(r.Multiplier) + " uses per " + class + " level"
	case features.ByAbility:
		uses = "uses equal to your " + title(r.Ability) + " modifier (at least 1)"
	case features.ByProficiency:
		uses = "uses equal to your Proficiency Bonus"
	}
	line := r.Name + ": " + uses + " from level " + strconv.Itoa(r.FromLevel)
	if len(r.Die) > 0 {
		line += ", each rolls a " + r.Die[0].Value
	}
	back := map[features.Event]string{features.LongRest: "on a Long Rest", features.Dawn: "at dawn"}[r.Recharges[0].On]
	if len(r.Recharges) > 1 {
		back = "on a Short or Long Rest"
	}
	return line + ", back " + back + "."
}

func title(s string) string { return strings.ToUpper(s[:1]) + s[1:] }
