package httpapi

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/subclassbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// SubclassBuilder is the subclass builder for homebrew subclasses.
type SubclassBuilder interface {
	Subclass(ctx context.Context, c caller.Caller, id uuid.UUID) (app.SubclassBuild, error)
	SaveSubclass(ctx context.Context, c caller.Caller, id uuid.UUID, d subclassbuild.Design) (app.SubclassBuild, error)
	PreviewSubclass(name string, d subclassbuild.Design) (app.SubclassBuild, error)
}

var _ SubclassBuilder = (*app.Service)(nil)

// subclassDesignIn reads a design off the wire; the two share their JSON shape field for field.
func subclassDesignIn(d *oas.SubclassDesign) subclassbuild.Design {
	raw, _ := d.MarshalJSON()
	var out subclassbuild.Design
	_ = json.Unmarshal(raw, &out)
	return out
}

func subclassBuildOut(b app.SubclassBuild) *oas.SubclassBuildHeaders {
	raw, _ := json.Marshal(b.Design) //nolint:errchkjson // a design is plain data
	out := oas.SubclassBuild{Lines: append([]string{}, b.Lines...)}
	_ = out.Design.UnmarshalJSON(raw)
	if b.Entry.ID != uuid.Nil {
		out.Entry, out.Slug = oas.NewOptLibraryEntry(libraryEntryOut(b.Entry)), oas.NewOptString(b.Slug)
	}
	return &oas.SubclassBuildHeaders{Response: out}
}

func (h *Handler) subclassCall(ctx context.Context, op string, run func(c caller.Caller) (app.SubclassBuild, error)) (*oas.SubclassBuildHeaders, *oas.ProblemStatusCodeWithHeaders) {
	b, bad := libraryCall(ctx, h, op, run)
	if bad != nil {
		return nil, bad
	}
	return subclassBuildOut(b), nil
}

// GetSubclassBuild opens a homebrew subclass in the subclass builder.
func (h *Handler) GetSubclassBuild(ctx context.Context, p oas.GetSubclassBuildParams) (oas.GetSubclassBuildRes, error) {
	out, bad := h.subclassCall(ctx, "get subclass build", func(c caller.Caller) (app.SubclassBuild, error) {
		return h.Subclasses.Subclass(ctx, c, uuid.UUID(p.EntryId))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// SaveSubclassBuild saves a homebrew subclass's design.
func (h *Handler) SaveSubclassBuild(ctx context.Context, req *oas.SubclassDesign, p oas.SaveSubclassBuildParams) (oas.SaveSubclassBuildRes, error) {
	out, bad := h.subclassCall(ctx, "save subclass build", func(c caller.Caller) (app.SubclassBuild, error) {
		return h.Subclasses.SaveSubclass(ctx, c, uuid.UUID(p.EntryId), subclassDesignIn(req))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// PreviewSubclass checks a design without saving it.
func (h *Handler) PreviewSubclass(ctx context.Context, req *oas.SubclassPreviewInput) (oas.PreviewSubclassRes, error) {
	out, bad := h.subclassCall(ctx, "preview subclass", func(caller.Caller) (app.SubclassBuild, error) {
		return h.Subclasses.PreviewSubclass(req.Name, subclassDesignIn(&req.Design))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}
