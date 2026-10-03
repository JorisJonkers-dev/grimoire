// Package conditionbuild turns a homebrew condition's design, its icon, how it ends, whether and how it
// stacks, and what it does as typed parts, into an Effect the engine runs; and names the exhaustion
// variants a Campaign can play with.
package conditionbuild

import (
	"regexp"
	"slices"
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// DesignError is why a design cannot be built, fit to show its author.
type DesignError string

func (e DesignError) Error() string { return string(e) }

// Design is a homebrew condition as its author builds it.
type Design struct {
	Icon    string `json:"icon"`
	Color   string `json:"color"`
	Text    string `json:"text"`
	Ends    string `json:"ends"`
	Ability string `json:"ability,omitempty"`
	// Cure is what ends a condition that lasts until cured: a lingering injury.
	Cure     string  `json:"cure,omitempty"`
	Stacks   bool    `json:"stacks,omitempty"`
	MaxLevel int     `json:"maxLevel,omitempty"`
	PerLevel Penalty `json:"perLevel"`
	Parts    []Part  `json:"parts"`
}

// Penalty is what each level of a stacking condition takes, and the level it kills at (0 never).
type Penalty struct {
	D20     int `json:"d20"`
	SpeedFt int `json:"speedFt"`
	DeathAt int `json:"deathAt"`
}

// Part is one thing the condition does.
type Part struct {
	Type    string `json:"type"`
	Ability string `json:"ability,omitempty"`
	Feet    int    `json:"feet,omitempty"`
	Text    string `json:"text,omitempty"`
}

// Condition is a built design: the Effect it runs as and how it looks on a token.
type Condition struct {
	Definition effects.Definition
	Icon       string
	Color      string
	Text       string
	// Cure is what ends a lingering injury; empty for any other condition.
	Cure    string
	ends    string
	ability string
}

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Icons are the glyphs a condition can wear.
func Icons() []string {
	return []string{"drop", "flame", "snow", "skull", "spiral", "eye", "chain", "star", "moon", "leaf", "bolt", "heart", "shield", "cloud"}
}

func abilities() []string {
	return []string{"strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"}
}

func partTypes() []string {
	return []string{
		"attack_advantage", "attack_disadvantage", "attacked_advantage", "attacked_disadvantage", "save_advantage", "save_disadvantage",
		"save_fails", "incapacitated", "immobile", "speed_penalty", "crit_within", "manual",
	}
}

// Check validates a design.
func Check(d Design) error {
	switch {
	case !slices.Contains(Icons(), d.Icon):
		return DesignError("choose an icon: " + strings.Join(Icons(), ", "))
	case !colorPattern.MatchString(d.Color):
		return DesignError("pick a colour like #7fa8dd")
	case len([]rune(d.Text)) > 2000:
		return DesignError("describe the condition in up to 2000 characters")
	case !slices.Contains([]string{"removed", "rest", "save", "cure"}, d.Ends):
		return DesignError("make it last until removed, on a rest, until a save, or until cured")
	case d.Ends == "save" && !slices.Contains(abilities(), d.Ability):
		return DesignError("choose the ability its save uses")
	case d.Ends == "cure" && strings.TrimSpace(d.Cure) == "":
		return DesignError("name its cure")
	case len([]rune(strings.TrimSpace(d.Cure))) > 80:
		return DesignError("name its cure in up to 80 characters")
	case d.Ends != "cure" && d.Cure != "":
		return DesignError("only a condition that lasts until cured names a cure")
	case len(d.Parts) > 20:
		return DesignError("keep it to up to 20 parts")
	}
	if d.Stacks {
		if err := stacking(d); err != nil {
			return err
		}
	}
	for _, p := range d.Parts {
		if err := part(p); err != nil {
			return err
		}
	}
	return nil
}

func stacking(d Design) error {
	switch {
	case d.MaxLevel < 2 || d.MaxLevel > 10:
		return DesignError("let it stack 2 to 10 levels")
	case d.PerLevel.D20 < 0 || d.PerLevel.D20 > 5:
		return DesignError("give each level a −0 to −5 penalty to D20 Tests")
	case d.PerLevel.SpeedFt < 0 || d.PerLevel.SpeedFt > 30:
		return DesignError("take 0 to 30 feet per level")
	case d.PerLevel.DeathAt < 0 || d.PerLevel.DeathAt > d.MaxLevel:
		return DesignError("kill at a level up to its highest, or never")
	}
	return nil
}

func part(p Part) error {
	switch {
	case !slices.Contains(partTypes(), p.Type):
		return DesignError("choose each part from " + strings.Join(partTypes(), ", "))
	case strings.HasPrefix(p.Type, "save_") && !slices.Contains(abilities(), p.Ability):
		return DesignError("name the ability each save part reads")
	case (p.Type == "speed_penalty" || p.Type == "crit_within") && (p.Feet < 5 || p.Feet > 60):
		return DesignError("make each distance 5 to 60 feet")
	case p.Type == "manual" && (strings.TrimSpace(p.Text) == "" || len([]rune(p.Text)) > 500):
		return DesignError("describe each manual part in up to 500 characters")
	}
	return nil
}

// Compile builds a checked design into the condition a slug names.
func Compile(slug, name string, d Design) Condition {
	duration := effects.Duration{Kind: effects.UntilDispelled, Amount: 0, RepeatSave: ""}
	switch d.Ends {
	case "rest":
		duration.Kind = effects.UntilRest
	case "save":
		duration.RepeatSave = d.Ability
	case "cure":
		duration.Kind = effects.UntilCured
	}
	components := make([]effects.Component, 0, len(d.Parts)+1)
	if d.Stacks {
		components = append(components, effects.Exhausting{D20PerLevel: d.PerLevel.D20, SpeedFtPerLevel: d.PerLevel.SpeedFt, DeathAt: d.PerLevel.DeathAt, MaxLevel: d.MaxLevel})
	}
	for _, p := range d.Parts {
		components = append(components, component(p))
	}
	return Condition{
		Definition: effects.Definition{Slug: slug, Name: name, Owner: effects.OwnedByCondition, Concentration: false, Duration: duration, Scaling: nil, Components: components},
		Icon:       d.Icon, Color: strings.ToLower(d.Color), Text: strings.TrimSpace(d.Text), Cure: strings.TrimSpace(d.Cure), ends: d.Ends, ability: d.Ability,
	}
}

func component(p Part) effects.Component {
	edge := func(against, advantage bool) effects.Component {
		return effects.Edge{Against: against, Advantage: advantage, Range: effects.AnyRange, SourceOnly: false}
	}
	switch p.Type {
	case "attack_advantage":
		return edge(false, true)
	case "attack_disadvantage":
		return edge(false, false)
	case "attacked_advantage":
		return edge(true, true)
	case "attacked_disadvantage":
		return edge(true, false)
	case "save_advantage":
		return effects.SaveEdge{Ability: p.Ability, Mode: effects.SaveAdvantage}
	case "save_disadvantage":
		return effects.SaveEdge{Ability: p.Ability, Mode: effects.SaveDisadvantage}
	case "save_fails":
		return effects.SaveEdge{Ability: p.Ability, Mode: effects.SaveFails}
	case "incapacitated":
		return effects.Incapacitated{}
	case "immobile":
		return effects.Immobile{}
	case "speed_penalty":
		return effects.SpeedPenalty{Ft: p.Feet}
	case "crit_within":
		return effects.CritWithin{Feet: p.Feet}
	}
	return effects.Manual{Instruction: strings.TrimSpace(p.Text)}
}

// Lines are a condition as its author reads it back: its text, how it ends, and what each part does.
func Lines(c Condition) []string {
	var out []string
	if c.Text != "" {
		out = append(out, c.Text)
	}
	switch c.ends {
	case "rest":
		out = append(out, "Ends on a rest.")
	case "save":
		out = append(out, "Ends when the creature succeeds on a "+strings.ToUpper(c.ability[:1])+c.ability[1:]+" saving throw at the end of its turn.")
	case "cure":
		out = append(out, "Lingers through every rest until cured: "+c.Cure+".")
	default:
		out = append(out, "Lasts until removed.")
	}
	parts := c.Definition
	parts.Duration = effects.Duration{Kind: "", Amount: 0, RepeatSave: ""}
	return append(out, parts.Text()...)
}

// Variants are the exhaustion variants a Campaign can play with; the first is the SRD's.
func Variants() []string { return []string{"srd-2024", "gentle", "grim", "off"} }

// Exhaustion is what each level of exhaustion does under a variant: the SRD's −2 and 5 feet with death
// at 6; a gentle −1 with death at 10; a grim 10 feet with death at 4; or nothing at all.
func Exhaustion(variant string) (effects.Exhausting, bool) {
	out, ok := map[string]effects.Exhausting{
		"srd-2024": {D20PerLevel: 2, SpeedFtPerLevel: 5, DeathAt: 6, MaxLevel: 6},
		"gentle":   {D20PerLevel: 1, SpeedFtPerLevel: 5, DeathAt: 10, MaxLevel: 10},
		"grim":     {D20PerLevel: 2, SpeedFtPerLevel: 10, DeathAt: 4, MaxLevel: 4},
		"off":      {D20PerLevel: 0, SpeedFtPerLevel: 0, DeathAt: 0, MaxLevel: 1},
	}[variant]
	return out, ok
}
