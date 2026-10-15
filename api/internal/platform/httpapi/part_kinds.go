package httpapi

import (
	"context"
	"slices"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// ListPartKinds serves every kind's definition, which builders draw their forms from.
func (h *Handler) ListPartKinds(context.Context) (oas.ListPartKindsRes, error) {
	kinds := effects.Kinds()
	out := make([]oas.PartKind, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, partKindOut(k.Describe(), effects.HintsOf(k), effects.ScalesOf(k)))
	}
	return &oas.ListPartKindsOKHeaders{Response: out}, nil
}

func partKindOut(d effects.Description, h effects.Hints, scales []effects.Scalable) oas.PartKind {
	out := oas.PartKind{
		Slug: d.Slug, Label: d.Label, Summary: d.Summary, Fields: make([]oas.PartField, 0, len(d.Fields)),
		Groups: append([]string{}, d.Groups...), OpenGroups: d.OpenGroups, Hints: hintsOut(h),
	}
	for _, f := range d.Fields {
		out.Fields = append(out.Fields, partFieldOut(f, scales))
	}
	return out
}

// partFieldOut is a field as served, with whether it scales and may follow a formula.
func partFieldOut(f effects.Field, scales []effects.Scalable) oas.PartField {
	i := slices.IndexFunc(scales, func(s effects.Scalable) bool { return s.Field == f.Name })
	return oas.PartField{
		Name: f.Name, Label: f.Label, Type: oas.PartFieldType(f.Type), Options: append([]string{}, f.Options...),
		Required: f.Required, Min: int32(f.Min), Max: int32(f.Max), //nolint:gosec // a field's bounds are small
		Scalable: i >= 0, Formula: i >= 0 && scales[i].Formula,
	}
}

func hintsOut(h effects.Hints) oas.PartHints {
	out := oas.PartHints{Order: append([]string{}, h.Order...), Sections: make([]oas.PartSection, 0, len(h.Sections)), Help: make([]oas.PartHelp, 0, len(h.Help))}
	for _, s := range h.Sections {
		out.Sections = append(out.Sections, oas.PartSection{Label: s.Label, Fields: append([]string{}, s.Fields...)})
	}
	for _, l := range h.Help {
		out.Help = append(out.Help, oas.PartHelp{Field: l.Field, Text: l.Text})
	}
	return out
}

// holdsOut is what a builder takes as Parts.
func holdsOut(h effects.Holds) oas.PartHolds {
	out := oas.PartHolds{
		Kinds: append([]string{}, h.Kinds...), Beneath: append([]string{}, h.Beneath...), Fills: append([]string{}, h.Fills...),
		Narrows: make([]oas.PartNarrow, 0, len(h.Narrows)),
	}
	for _, n := range h.Narrows {
		out.Narrows = append(out.Narrows, oas.PartNarrow{Kind: n.Kind, Field: n.Field, Options: append([]string{}, n.Options...)})
	}
	return out
}
