// Package kindproto is a PROTOTYPE for grimoire issue 255 ("one kind, one place"). Throwaway: it is
// not wired into the engine. It checks that one file per Effect component kind can carry everything
// the rest of the system needs from the kind, with no switch over all kinds anywhere.
package kindproto

import "fmt"

// Kind is what one component kind declares, once. Everything else is derived from it: the builder
// form, the Design file schema, validation, the typed row table and the rules text.
type Kind interface {
	// Describe is the kind's definition as data: what the builder and a Design file are made from.
	Describe() Description
	// Decode turns authored values into the kind's own typed part, or says what is wrong with them.
	Decode(v Values) (Part, error)
}

// Part is one typed part of an Effect. Play never looks at Values, only at typed parts.
type Part interface {
	// Text is the rules sentence for the part; under holds the sentences of the parts beneath it.
	Text(under []string) string
}

// Values are a part's fields as authored, before they are typed.
type Values map[string]any

// Description is a kind's definition as data.
type Description struct {
	Slug     string   `json:"slug"`
	Label    string   `json:"label"`
	Summary  string   `json:"summary"`
	Fields   []Field  `json:"fields"`
	Children string   `json:"children"` // "", "parts" or "modes"
	Table    Table    `json:"table"`
	Template string   `json:"template"` // demo only: real text comes from Part.Text
	Files    []string `json:"files"`
}

// Field is one value an author fills in.
type Field struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Type     string   `json:"type"` // dice, int, bool, enum, enums
	Options  []string `json:"options,omitempty"`
	Required bool     `json:"required,omitempty"`
	Min      int      `json:"min,omitempty"`
	Max      int      `json:"max,omitempty"`
	Help     string   `json:"help,omitempty"`
}

// Table is the kind's typed row table. No JSON column: every field is a column or a child table.
type Table struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
}

// Folder is a capability a kind may have: it changes an attack roll while its Effect is active.
// Capabilities replace the switches over every kind: a kind that folds nothing implements nothing.
type Folder interface {
	FoldAttack(p *AttackProfile)
}

// AttackProfile is what an attack reads of the active Effects.
type AttackProfile struct{ BonusDice []string }

// Lander is a capability: the part does something the moment its Activation lands on a target. It
// returns steps from a small fixed set the runtime already knows how to apply.
type Lander interface {
	Land(t Target) []Step
}

// Target is what a part may read of the creature it lands on.
type Target struct{ Name string }

// Step is one thing the runtime applies. The set is closed and small; kinds are many.
type Step struct {
	Do     string // "heal", "damage", "roll_save", ...
	Detail string
}

func files(slug string) []string {
	return []string{
		"api/internal/rules/effects/kind_" + slug + ".go",
		"api/internal/compendium/pgstore/kind_" + slug + ".go",
		"api/db/queries/effect_kind_" + slug + ".sql",
		"api/db/migrations/<timestamp>_effect_kind_" + slug + ".sql",
	}
}

func str(v Values, name string) string   { s, _ := v[name].(string); return s }
func boolean(v Values, name string) bool { b, _ := v[name].(bool); return b }
func number(v Values, name string) int {
	switch n := v[name].(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return 0
}

func list(v Values, name string) []string {
	var out []string
	if xs, ok := v[name].([]any); ok {
		for _, x := range xs {
			out = append(out, fmt.Sprint(x))
		}
	}
	if xs, ok := v[name].([]string); ok {
		out = xs
	}
	return out
}
