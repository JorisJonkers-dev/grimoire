package effects

import (
	"strings"
	"unicode/utf8"
)

// Choice offers modes; whoever applies the Effect picks one, and only that mode's Parts count
// (Enlarge/Reduce).
type Choice struct {
	Modes []Mode
}

// Mode is one option of a Choice.
type Mode struct {
	Name       string
	Components []Component
}

func (Choice) isComponent() {}

// Kind is choice.
func (Choice) Kind() string { return "choice" }

// Text names each mode with the sentences of its Parts.
func (c Choice) Text() string {
	modes := make([]string, 0, len(c.Modes))
	for _, m := range c.Modes {
		modes = append(modes, m.Name+": "+strings.Join(sentences(m.Components), " "))
	}
	return "Choose one: " + strings.Join(modes, " Or ")
}

// Groups are the modes, each named.
func (c Choice) Groups() []Group {
	out := make([]Group, 0, len(c.Modes))
	for _, m := range c.Modes {
		out = append(out, Group{Name: m.Name, Parts: m.Components})
	}
	return out
}

// Chosen is the Parts of the mode picked.
func (c Choice) Chosen(mode string) []Component {
	var out []Component
	for _, m := range c.Modes {
		if m.Name == mode {
			out = append(out, m.Components...)
		}
	}
	return out
}

type choiceKind struct{}

func (choiceKind) Describe() Description {
	return Description{
		Slug: "choice", Label: "Choice of modes", Summary: "Offers named modes; whoever applies the Effect picks one, and only its Parts count.",
		Fields: nil, Groups: nil, OpenGroups: true,
	}
}

func (choiceKind) Decode(_ Values, beneath Groups) (Part, error) {
	if len(beneath) == 0 {
		return nil, refuse("modes", "offer at least one mode")
	}
	c := Choice{Modes: make([]Mode, 0, len(beneath))}
	seen := map[string]bool{}
	for _, g := range beneath {
		if g.Name == "" || utf8.RuneCountInString(g.Name) > 80 {
			return nil, refuse("modes", "name each mode in up to 80 letters")
		}
		if seen[g.Name] {
			return nil, refuse("modes", "name each mode once")
		}
		if len(g.Parts) == 0 {
			return nil, refuse("modes", "add at least one Part to each mode")
		}
		seen[g.Name] = true
		c.Modes = append(c.Modes, Mode{Name: g.Name, Components: g.Parts})
	}
	return c, nil
}
