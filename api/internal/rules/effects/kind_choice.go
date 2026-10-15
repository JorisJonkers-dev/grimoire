package effects

import (
	"strings"
	"unicode/utf8"
)

// Choice offers modes, each a named group of Parts; whoever applies the Effect picks one, and only
// that mode's Parts count (Enlarge/Reduce).
type Choice struct {
	Modes []Group
}

func (Choice) isPart() {}

// Kind is choice.
func (Choice) Kind() string { return "choice" }

// Text names each mode with the sentences of its Parts.
func (c Choice) Text() string {
	modes := make([]string, 0, len(c.Modes))
	for _, m := range c.Modes {
		modes = append(modes, m.Name+": "+strings.Join(sentences(m.Parts), " "))
	}
	return "Choose one: " + strings.Join(modes, " Or ")
}

// Groups are the modes.
func (c Choice) Groups() []Group {
	return c.Modes
}

// Regroup is the choice offering the groups as its modes.
func (c Choice) Regroup(gs []Group) Part {
	c.Modes = gs
	return c
}

// Chosen is the Parts of the mode picked.
func (c Choice) Chosen(mode string) []Part {
	var out []Part
	for _, m := range c.Modes {
		if m.Name == mode {
			out = append(out, m.Parts...)
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
	c := Choice{Modes: make([]Group, 0, len(beneath))}
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
		c.Modes = append(c.Modes, g)
	}
	return c, nil
}
