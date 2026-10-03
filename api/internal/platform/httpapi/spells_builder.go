package httpapi

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// SpellBuilder is the Effect builder for homebrew spells.
type SpellBuilder interface {
	Spell(ctx context.Context, c caller.Caller, id uuid.UUID) (app.SpellBuild, error)
	SaveSpell(ctx context.Context, c caller.Caller, id uuid.UUID, d spellbuild.Design) (app.SpellBuild, error)
	PreviewSpell(ctx context.Context, name string, d spellbuild.Design) (app.SpellBuild, error)
}

var _ SpellBuilder = (*app.Service)(nil)

// designIn reads a design off the wire; the two share their JSON shape field for field.
func designIn(d *oas.SpellDesign) spellbuild.Design {
	raw, _ := d.MarshalJSON()
	var out spellbuild.Design
	_ = json.Unmarshal(raw, &out)
	return out
}

func designOut(d spellbuild.Design) oas.SpellDesign {
	raw, _ := json.Marshal(d) //nolint:errchkjson // a design is plain data
	var out oas.SpellDesign
	_ = out.UnmarshalJSON(raw)
	return out
}

func spellBuildOut(b app.SpellBuild) *oas.SpellBuildHeaders {
	out := oas.SpellBuild{Design: designOut(b.Design), Effect: b.Built.Definition.Slug, Text: append([]string{}, b.Built.Text...), Hexes: []oas.BuilderHex{}}
	if b.Entry.ID != uuid.Nil {
		out.Entry = oas.NewOptLibraryEntry(libraryEntryOut(b.Entry))
	}
	for _, h := range b.Hexes {
		out.Hexes = append(out.Hexes, oas.BuilderHex{Q: int32(h.Q), R: int32(h.R)}) //nolint:gosec // areas are small
	}
	return &oas.SpellBuildHeaders{Response: out}
}

func (h *Handler) spellCall(ctx context.Context, op string, run func(c caller.Caller) (app.SpellBuild, error)) (*oas.SpellBuildHeaders, *oas.ProblemStatusCodeWithHeaders) {
	b, bad := libraryCall(ctx, h, op, run)
	if bad != nil {
		return nil, bad
	}
	return spellBuildOut(b), nil
}

// GetSpellBuild opens a homebrew spell in the Effect builder.
func (h *Handler) GetSpellBuild(ctx context.Context, p oas.GetSpellBuildParams) (oas.GetSpellBuildRes, error) {
	out, bad := h.spellCall(ctx, "get spell build", func(c caller.Caller) (app.SpellBuild, error) {
		return h.Spells.Spell(ctx, c, uuid.UUID(p.EntryId))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// SaveSpellBuild saves a homebrew spell's design.
func (h *Handler) SaveSpellBuild(ctx context.Context, req *oas.SpellDesign, p oas.SaveSpellBuildParams) (oas.SaveSpellBuildRes, error) {
	out, bad := h.spellCall(ctx, "save spell build", func(c caller.Caller) (app.SpellBuild, error) {
		return h.Spells.SaveSpell(ctx, c, uuid.UUID(p.EntryId), designIn(req))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// PreviewSpell builds a design without saving it.
func (h *Handler) PreviewSpell(ctx context.Context, req *oas.SpellPreviewInput) (oas.PreviewSpellRes, error) {
	out, bad := h.spellCall(ctx, "preview spell", func(caller.Caller) (app.SpellBuild, error) {
		return h.Spells.PreviewSpell(ctx, req.Name, designIn(&req.Design))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}
