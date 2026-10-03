package httpapi

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/classbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// ClassBuilder is the class builder for homebrew classes.
type ClassBuilder interface {
	Class(ctx context.Context, c caller.Caller, id uuid.UUID) (app.ClassBuild, error)
	SaveClass(ctx context.Context, c caller.Caller, id uuid.UUID, d classbuild.Design) (app.ClassBuild, error)
	PreviewClass(name string, d classbuild.Design) (app.ClassBuild, error)
}

var _ ClassBuilder = (*app.Service)(nil)

// classDesignIn reads a design off the wire; the two share their JSON shape field for field.
func classDesignIn(d *oas.ClassDesign) classbuild.Design {
	raw, _ := d.MarshalJSON()
	var out classbuild.Design
	_ = json.Unmarshal(raw, &out)
	return out
}

func classBuildOut(b app.ClassBuild) *oas.ClassBuildHeaders {
	raw, _ := json.Marshal(b.Design) //nolint:errchkjson // a design is plain data
	out := oas.ClassBuild{Lines: append([]string{}, b.Lines...)}
	_ = out.Design.UnmarshalJSON(raw)
	if b.Entry.ID != uuid.Nil {
		out.Entry, out.Slug = oas.NewOptLibraryEntry(libraryEntryOut(b.Entry)), oas.NewOptString(b.Slug)
	}
	return &oas.ClassBuildHeaders{Response: out}
}

func (h *Handler) classCall(ctx context.Context, op string, run func(c caller.Caller) (app.ClassBuild, error)) (*oas.ClassBuildHeaders, *oas.ProblemStatusCodeWithHeaders) {
	b, bad := libraryCall(ctx, h, op, run)
	if bad != nil {
		return nil, bad
	}
	return classBuildOut(b), nil
}

// GetClassBuild opens a homebrew class in the class builder.
func (h *Handler) GetClassBuild(ctx context.Context, p oas.GetClassBuildParams) (oas.GetClassBuildRes, error) {
	out, bad := h.classCall(ctx, "get class build", func(c caller.Caller) (app.ClassBuild, error) {
		return h.Classes.Class(ctx, c, uuid.UUID(p.EntryId))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// SaveClassBuild saves a homebrew class's design.
func (h *Handler) SaveClassBuild(ctx context.Context, req *oas.ClassDesign, p oas.SaveClassBuildParams) (oas.SaveClassBuildRes, error) {
	out, bad := h.classCall(ctx, "save class build", func(c caller.Caller) (app.ClassBuild, error) {
		return h.Classes.SaveClass(ctx, c, uuid.UUID(p.EntryId), classDesignIn(req))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// PreviewClass checks a design without saving it.
func (h *Handler) PreviewClass(ctx context.Context, req *oas.ClassPreviewInput) (oas.PreviewClassRes, error) {
	out, bad := h.classCall(ctx, "preview class", func(caller.Caller) (app.ClassBuild, error) {
		return h.Classes.PreviewClass(req.Name, classDesignIn(&req.Design))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}
