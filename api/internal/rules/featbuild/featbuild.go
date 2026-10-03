// Package featbuild turns homebrew feats and 2024-shaped backgrounds into what character creation and
// levelling up read: a feat with its category, prerequisites and whether it repeats, and a background
// with its three ability options, two skills, Origin feat, tool and equipment.
package featbuild

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/features"
)

// DesignError is why a design cannot be built, fit to show its author.
type DesignError string

func (e DesignError) Error() string { return string(e) }

// Feat is a homebrew feat as its author builds it.
type Feat struct {
	Category      string         `json:"category"`
	Text          string         `json:"text"`
	Repeatable    bool           `json:"repeatable,omitempty"`
	Prerequisites []Prerequisite `json:"prerequisites"`
}

// Prerequisite is one thing a feat needs. Prerequisites in one group are alternatives; every group
// must hold.
type Prerequisite struct {
	Kind    string `json:"kind"`
	Ability string `json:"ability,omitempty"`
	Minimum int    `json:"minimum,omitempty"`
	Feat    string `json:"feat,omitempty"`
	Group   int    `json:"group"`
}

// Background is a homebrew background as its author builds it: the three abilities its score
// increases may go to, two skills, an Origin feat, a tool, and equipment or gold instead.
type Background struct {
	Abilities []string `json:"abilities"`
	Skills    []string `json:"skills"`
	Feat      string   `json:"feat"`
	FeatName  string   `json:"featName"`
	Tool      string   `json:"tool,omitempty"`
	Equipment string   `json:"equipment,omitempty"`
	Gold      int      `json:"gold,omitempty"`
	Text      string   `json:"text,omitempty"`
}

// BuiltFeat is a feat as levelling up offers it.
type BuiltFeat struct {
	Slug         string
	Name         string
	Category     string
	Description  string
	Repeatable   bool
	Requirements []features.Requirement
}

// Trait is a background's grant as the sheet shows it.
type Trait struct {
	Name string
	Text string
}

// BuiltBackground is a background as character creation offers it.
type BuiltBackground struct {
	Slug      string
	Name      string
	Abilities []string
	Skills    []string
	Feat      string
	Traits    []Trait
}

var slugPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,79}$`)

func categories() map[string]string {
	return map[string]string{"origin": "Origin", "general": "General", "fighting_style": "Fighting Style", "epic_boon": "Epic Boon"}
}

func abilities() []string {
	return []string{"strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"}
}

func title(s string) string {
	words := strings.Split(s, "-")
	for i, w := range words {
		if w != "of" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func distinct(xs []string) int {
	seen := make([]string, 0, len(xs))
	for _, x := range xs {
		if !slices.Contains(seen, x) {
			seen = append(seen, x)
		}
	}
	return len(seen)
}

// CheckFeat validates a feat design.
func CheckFeat(f Feat) error {
	if _, ok := categories()[f.Category]; !ok {
		return DesignError("choose a category: origin, general, fighting_style or epic_boon")
	}
	if len([]rune(f.Text)) > 4000 {
		return DesignError("describe the feat in up to 4000 characters")
	}
	if len(f.Prerequisites) > 10 {
		return DesignError("keep it to up to 10 prerequisites")
	}
	for _, p := range f.Prerequisites {
		if err := prerequisite(p); err != nil {
			return err
		}
	}
	return nil
}

func prerequisite(p Prerequisite) error {
	switch {
	case !slices.Contains([]string{"level", "ability", "spellcasting", "feat"}, p.Kind):
		return DesignError("make each prerequisite a level, ability, spellcasting or feat")
	case p.Group < 0 || p.Group > 9:
		return DesignError("put each prerequisite in a group of 0 to 9")
	case p.Kind == "level" && (p.Minimum < 1 || p.Minimum > 20):
		return DesignError("need a level of 1 to 20")
	case p.Kind == "ability" && !slices.Contains(abilities(), p.Ability):
		return DesignError("name the ability a prerequisite reads")
	case p.Kind == "ability" && (p.Minimum < 1 || p.Minimum > 30):
		return DesignError("need an ability score of 1 to 30")
	case p.Kind == "feat" && !slugPattern.MatchString(p.Feat):
		return DesignError("name the feat it needs by its slug, like alert")
	}
	return nil
}

// CheckBackground validates a background design.
func CheckBackground(b Background) error {
	known := func(s string) bool { return rules.Skill(s).Valid() }
	switch {
	case slices.ContainsFunc(b.Abilities, func(a string) bool { return !slices.Contains(abilities(), a) }):
		return DesignError("choose real abilities: " + strings.Join(abilities(), ", "))
	case len(b.Abilities) != 3 || distinct(b.Abilities) != 3:
		return DesignError("offer three different abilities for its score increases")
	case !all(b.Skills, known):
		return DesignError("choose real skills")
	case len(b.Skills) != 2 || distinct(b.Skills) != 2:
		return DesignError("grant two different skills")
	case !slugPattern.MatchString(b.Feat):
		return DesignError("name its Origin feat by its slug, like alert")
	case !sized(b.FeatName, 60):
		return DesignError("name the Origin feat in up to 60 characters")
	case len([]rune(b.Tool)) > 80:
		return DesignError("name a tool in up to 80 characters")
	case len([]rune(b.Equipment)) > 500:
		return DesignError("describe its equipment in up to 500 characters")
	case b.Gold < 0 || b.Gold > 1000:
		return DesignError("offer 0 to 1000 GP instead")
	case len([]rune(b.Text)) > 2000:
		return DesignError("describe the background in up to 2000 characters")
	}
	return nil
}

func all(xs []string, ok func(string) bool) bool {
	return !slices.ContainsFunc(xs, func(x string) bool { return !ok(x) })
}

func sized(s string, most int) bool {
	n := len([]rune(strings.TrimSpace(s)))
	return n > 0 && n <= most
}

// CompileFeat builds a checked feat design into the feat a slug names.
func CompileFeat(slug, name string, f Feat) BuiltFeat {
	out := BuiltFeat{
		Slug: slug, Name: name, Category: categories()[f.Category], Description: strings.TrimSpace(f.Text), Repeatable: f.Repeatable,
		Requirements: make([]features.Requirement, 0, len(f.Prerequisites)),
	}
	kinds := map[string]features.Kind{"level": features.MinLevel, "ability": features.MinAbility, "spellcasting": features.Spellcasting, "feat": features.HasFeat}
	for _, p := range f.Prerequisites {
		out.Requirements = append(out.Requirements, features.Requirement{Kind: kinds[p.Kind], Ability: p.Ability, Minimum: p.Minimum, Slug: p.Feat, Group: p.Group})
	}
	return out
}

// FeatLines are a feat as its author reads it back.
func FeatLines(f BuiltFeat) []string {
	head := f.Category + " feat"
	if f.Repeatable {
		head += ", repeatable"
	}
	out := []string{head}
	if len(f.Requirements) > 0 {
		out = append(out, "Prerequisites: "+strings.Join(features.Unmet(f.Requirements, features.Candidate{Level: 0, Abilities: nil, Spellcasting: false, Feats: nil, Features: nil}), "; "))
	}
	if f.Description != "" {
		out = append(out, f.Description)
	}
	return out
}

// CompileBackground builds a checked background design into the background a slug names.
func CompileBackground(slug, name string, b Background) BuiltBackground {
	out := BuiltBackground{Slug: slug, Name: name, Abilities: slices.Clone(b.Abilities), Skills: slices.Clone(b.Skills), Feat: b.Feat, Traits: []Trait{}}
	if text := strings.TrimSpace(b.Text); text != "" {
		out.Traits = append(out.Traits, Trait{Name: name, Text: text})
	}
	out.Traits = append(out.Traits, Trait{Name: "Origin Feat", Text: strings.TrimSpace(b.FeatName) + "."})
	if tool := strings.TrimSpace(b.Tool); tool != "" {
		out.Traits = append(out.Traits, Trait{Name: "Tool Proficiency", Text: tool + "."})
	}
	gold := strconv.Itoa(b.Gold) + " GP"
	kit := strings.TrimSpace(b.Equipment)
	switch {
	case kit != "" && b.Gold > 0:
		out.Traits = append(out.Traits, Trait{Name: "Equipment", Text: "Choose " + kit + "; or " + gold + "."})
	case kit != "":
		out.Traits = append(out.Traits, Trait{Name: "Equipment", Text: kit + "."})
	case b.Gold > 0:
		out.Traits = append(out.Traits, Trait{Name: "Equipment", Text: gold + "."})
	}
	return out
}

// BackgroundLines are a background as its author reads it back.
func BackgroundLines(b BuiltBackground) []string {
	names := func(xs []string) string {
		out := make([]string, 0, len(xs))
		for _, x := range xs {
			out = append(out, title(x))
		}
		return strings.Join(out, ", ")
	}
	out := []string{"Ability Scores: " + names(b.Abilities), "Skill Proficiencies: " + names(b.Skills)}
	for _, t := range b.Traits {
		out = append(out, t.Name+". "+t.Text)
	}
	return out
}
