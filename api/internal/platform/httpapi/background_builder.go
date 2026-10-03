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

// BackgroundBuilder is the background builder for homebrew background entries.
type BackgroundBuilder interface {
	Background(ctx context.Context, c caller.Caller, id uuid.UUID) (app.BackgroundBuild, error)
	SaveBackground(ctx context.Context, c caller.Caller, id uuid.UUID, d featbuild.Background) (app.BackgroundBuild, error)
	PreviewBackground(name string, d featbuild.Background) (app.BackgroundBuild, error)
}

var _ BackgroundBuilder = (*app.Service)(nil)

// backgroundDesignIn reads a design off the wire; the two share their JSON shape field for field.
func backgroundDesignIn(d *oas.BackgroundDesign) featbuild.Background {
	raw, _ := d.MarshalJSON()
	var out featbuild.Background
	_ = json.Unmarshal(raw, &out)
	return out
}

func backgroundBuildOut(b app.BackgroundBuild) *oas.BackgroundBuildHeaders {
	raw, _ := json.Marshal(b.Design) //nolint:errchkjson // a design is plain data
	out := oas.BackgroundBuild{Lines: append([]string{}, b.Lines...)}
	_ = out.Design.UnmarshalJSON(raw)
	if b.Entry.ID != uuid.Nil {
		out.Entry, out.Slug = oas.NewOptLibraryEntry(libraryEntryOut(b.Entry)), oas.NewOptString(b.Slug)
	}
	return &oas.BackgroundBuildHeaders{Response: out}
}

func (h *Handler) backgroundCall(ctx context.Context, op string, run func(c caller.Caller) (app.BackgroundBuild, error)) (*oas.BackgroundBuildHeaders, *oas.ProblemStatusCodeWithHeaders) {
	b, bad := libraryCall(ctx, h, op, run)
	if bad != nil {
		return nil, bad
	}
	return backgroundBuildOut(b), nil
}

// GetBackgroundBuild opens a homebrew background in the background builder.
func (h *Handler) GetBackgroundBuild(ctx context.Context, p oas.GetBackgroundBuildParams) (oas.GetBackgroundBuildRes, error) {
	out, bad := h.backgroundCall(ctx, "get background build", func(c caller.Caller) (app.BackgroundBuild, error) {
		return h.BackgroundBuilds.Background(ctx, c, uuid.UUID(p.EntryId))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// SaveBackgroundBuild saves a homebrew background's design.
func (h *Handler) SaveBackgroundBuild(ctx context.Context, req *oas.BackgroundDesign, p oas.SaveBackgroundBuildParams) (oas.SaveBackgroundBuildRes, error) {
	out, bad := h.backgroundCall(ctx, "save background build", func(c caller.Caller) (app.BackgroundBuild, error) {
		return h.BackgroundBuilds.SaveBackground(ctx, c, uuid.UUID(p.EntryId), backgroundDesignIn(req))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// PreviewBackground checks a design without saving it.
func (h *Handler) PreviewBackground(ctx context.Context, req *oas.BackgroundPreviewInput) (oas.PreviewBackgroundRes, error) {
	out, bad := h.backgroundCall(ctx, "preview background", func(caller.Caller) (app.BackgroundBuild, error) {
		return h.BackgroundBuilds.PreviewBackground(req.Name, backgroundDesignIn(&req.Design))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}
