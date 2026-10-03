package httpapi

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/featbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// FeatBuilder is the feat builder for homebrew feat entries.
type FeatBuilder interface {
	Feat(ctx context.Context, c caller.Caller, id uuid.UUID) (app.FeatBuild, error)
	SaveFeat(ctx context.Context, c caller.Caller, id uuid.UUID, d featbuild.Feat) (app.FeatBuild, error)
	PreviewFeat(name string, d featbuild.Feat) (app.FeatBuild, error)
}

var _ FeatBuilder = (*app.Service)(nil)

// featDesignIn reads a design off the wire; the two share their JSON shape field for field.
func featDesignIn(d *oas.FeatDesign) featbuild.Feat {
	raw, _ := d.MarshalJSON()
	var out featbuild.Feat
	_ = json.Unmarshal(raw, &out)
	return out
}

func featBuildOut(b app.FeatBuild) *oas.FeatBuildHeaders {
	raw, _ := json.Marshal(b.Design) //nolint:errchkjson // a design is plain data
	out := oas.FeatBuild{Lines: append([]string{}, b.Lines...)}
	_ = out.Design.UnmarshalJSON(raw)
	if b.Entry.ID != uuid.Nil {
		out.Entry, out.Slug = oas.NewOptLibraryEntry(libraryEntryOut(b.Entry)), oas.NewOptString(b.Slug)
	}
	return &oas.FeatBuildHeaders{Response: out}
}

func (h *Handler) featCall(ctx context.Context, op string, run func(c caller.Caller) (app.FeatBuild, error)) (*oas.FeatBuildHeaders, *oas.ProblemStatusCodeWithHeaders) {
	b, bad := libraryCall(ctx, h, op, run)
	if bad != nil {
		return nil, bad
	}
	return featBuildOut(b), nil
}

// GetFeatBuild opens a homebrew feat in the feat builder.
func (h *Handler) GetFeatBuild(ctx context.Context, p oas.GetFeatBuildParams) (oas.GetFeatBuildRes, error) {
	out, bad := h.featCall(ctx, "get feat build", func(c caller.Caller) (app.FeatBuild, error) {
		return h.FeatBuilds.Feat(ctx, c, uuid.UUID(p.EntryId))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// SaveFeatBuild saves a homebrew feat's design.
func (h *Handler) SaveFeatBuild(ctx context.Context, req *oas.FeatDesign, p oas.SaveFeatBuildParams) (oas.SaveFeatBuildRes, error) {
	out, bad := h.featCall(ctx, "save feat build", func(c caller.Caller) (app.FeatBuild, error) {
		return h.FeatBuilds.SaveFeat(ctx, c, uuid.UUID(p.EntryId), featDesignIn(req))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// PreviewFeat checks a design without saving it.
func (h *Handler) PreviewFeat(ctx context.Context, req *oas.FeatPreviewInput) (oas.PreviewFeatRes, error) {
	out, bad := h.featCall(ctx, "preview feat", func(caller.Caller) (app.FeatBuild, error) {
		return h.FeatBuilds.PreviewFeat(req.Name, featDesignIn(&req.Design))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}
