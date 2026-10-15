package httpapi

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// ListPartKinds serves every kind's definition, which builders draw their forms from.
func (h *Handler) ListPartKinds(context.Context) (oas.ListPartKindsRes, error) {
	kinds := effects.Kinds()
	out := make([]oas.PartKind, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, partKindOut(k.Describe()))
	}
	return &oas.ListPartKindsOKHeaders{Response: out}, nil
}

func partKindOut(d effects.Description) oas.PartKind {
	out := oas.PartKind{
		Slug: d.Slug, Label: d.Label, Summary: d.Summary, Fields: make([]oas.PartField, 0, len(d.Fields)),
		Groups: append([]string{}, d.Groups...), OpenGroups: d.OpenGroups,
	}
	for _, f := range d.Fields {
		out.Fields = append(out.Fields, oas.PartField{
			Name: f.Name, Label: f.Label, Type: oas.PartFieldType(f.Type), Options: append([]string{}, f.Options...),
			Required: f.Required, Min: int32(f.Min), Max: int32(f.Max), //nolint:gosec // a field's bounds are small
		})
	}
	return out
}
