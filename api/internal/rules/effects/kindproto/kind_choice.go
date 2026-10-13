package kindproto

import (
	"errors"
	"strings"
)

// Choice lets the one activating pick one mode; each mode holds parts of its own.
type Choice struct{ Modes []string }

type choiceKind struct{}

func (choiceKind) Describe() Description {
	return Description{
		Slug: "choice", Label: "Choice of modes", Summary: "The one activating picks one mode; each mode has parts of its own.",
		Children: "modes",
		Table:    Table{Name: "effect_modes", Columns: []string{"name text CHECK (char_length(name) BETWEEN 1 AND 40)", "parent integer REFERENCES effect_components"}},
		Template: "Choose one: {modes}.",
		Files:    files("choice"),
	}
}

func (choiceKind) Decode(v Values) (Part, error) {
	c := Choice{Modes: list(v, "modes")}
	if len(c.Modes) < 2 {
		return nil, errors.New("A choice needs at least two modes.")
	}
	return c, nil
}

// Text is the rules sentence, with each mode's parts under it.
func (c Choice) Text(under []string) string {
	return "Choose one: " + strings.Join(c.Modes, " or ") + ". " + strings.Join(under, " ")
}
