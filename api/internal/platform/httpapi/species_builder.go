package httpapi

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/speciesbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// SpeciesBuilder is the species builder for homebrew species entries.
type SpeciesBuilder interface {
	Species(ctx context.Context, c caller.Caller, id uuid.UUID) (app.SpeciesBuild, error)
	SaveSpecies(ctx context.Context, c caller.Caller, id uuid.UUID, d speciesbuild.Design) (app.SpeciesBuild, error)
	PreviewSpecies(name string, d speciesbuild.Design) (app.SpeciesBuild, error)
}

var _ SpeciesBuilder = (*app.Service)(nil)

// speciesDesignIn reads a design off the wire; the two share their JSON shape field for field.
func speciesDesignIn(d *oas.SpeciesDesign) speciesbuild.Design {
	raw, _ := d.MarshalJSON()
	var out speciesbuild.Design
	_ = json.Unmarshal(raw, &out)
	return out
}

func speciesBuildOut(b app.SpeciesBuild) *oas.SpeciesBuildHeaders {
	raw, _ := json.Marshal(b.Design) //nolint:errchkjson // a design is plain data
	out := oas.SpeciesBuild{Lines: append([]string{}, b.Lines...)}
	_ = out.Design.UnmarshalJSON(raw)
	if b.Entry.ID != uuid.Nil {
		out.Entry, out.Slug = oas.NewOptLibraryEntry(libraryEntryOut(b.Entry)), oas.NewOptString(b.Slug)
	}
	return &oas.SpeciesBuildHeaders{Response: out}
}

func (h *Handler) speciesCall(ctx context.Context, op string, run func(c caller.Caller) (app.SpeciesBuild, error)) (*oas.SpeciesBuildHeaders, *oas.ProblemStatusCodeWithHeaders) {
	b, bad := libraryCall(ctx, h, op, run)
	if bad != nil {
		return nil, bad
	}
	return speciesBuildOut(b), nil
}

// GetSpeciesBuild opens a homebrew species in the species builder.
func (h *Handler) GetSpeciesBuild(ctx context.Context, p oas.GetSpeciesBuildParams) (oas.GetSpeciesBuildRes, error) {
	out, bad := h.speciesCall(ctx, "get species build", func(c caller.Caller) (app.SpeciesBuild, error) {
		return h.SpeciesBuilds.Species(ctx, c, uuid.UUID(p.EntryId))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// SaveSpeciesBuild saves a homebrew species's design.
func (h *Handler) SaveSpeciesBuild(ctx context.Context, req *oas.SpeciesDesign, p oas.SaveSpeciesBuildParams) (oas.SaveSpeciesBuildRes, error) {
	out, bad := h.speciesCall(ctx, "save species build", func(c caller.Caller) (app.SpeciesBuild, error) {
		return h.SpeciesBuilds.SaveSpecies(ctx, c, uuid.UUID(p.EntryId), speciesDesignIn(req))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// PreviewSpecies checks a design without saving it.
func (h *Handler) PreviewSpecies(ctx context.Context, req *oas.SpeciesPreviewInput) (oas.PreviewSpeciesRes, error) {
	out, bad := h.speciesCall(ctx, "preview species", func(caller.Caller) (app.SpeciesBuild, error) {
		return h.SpeciesBuilds.PreviewSpecies(req.Name, speciesDesignIn(&req.Design))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}
