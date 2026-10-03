package httpapi

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/conditionbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// ConditionBuilder is the condition builder for homebrew condition entries.
type ConditionBuilder interface {
	Condition(ctx context.Context, c caller.Caller, id uuid.UUID) (app.ConditionBuild, error)
	SaveCondition(ctx context.Context, c caller.Caller, id uuid.UUID, d conditionbuild.Design) (app.ConditionBuild, error)
	PreviewCondition(name string, d conditionbuild.Design) (app.ConditionBuild, error)
}

var _ ConditionBuilder = (*app.Service)(nil)

// conditionDesignIn reads a design off the wire; the two share their JSON shape field for field.
func conditionDesignIn(d *oas.ConditionDesign) conditionbuild.Design {
	raw, _ := d.MarshalJSON()
	var out conditionbuild.Design
	_ = json.Unmarshal(raw, &out)
	return out
}

func conditionBuildOut(b app.ConditionBuild) *oas.ConditionBuildHeaders {
	raw, _ := json.Marshal(b.Design) //nolint:errchkjson // a design is plain data
	out := oas.ConditionBuild{Lines: append([]string{}, b.Lines...)}
	_ = out.Design.UnmarshalJSON(raw)
	if b.Entry.ID != uuid.Nil {
		out.Entry, out.Slug = oas.NewOptLibraryEntry(libraryEntryOut(b.Entry)), oas.NewOptString(b.Slug)
	}
	return &oas.ConditionBuildHeaders{Response: out}
}

func (h *Handler) conditionCall(ctx context.Context, op string, run func(c caller.Caller) (app.ConditionBuild, error)) (*oas.ConditionBuildHeaders, *oas.ProblemStatusCodeWithHeaders) {
	b, bad := libraryCall(ctx, h, op, run)
	if bad != nil {
		return nil, bad
	}
	return conditionBuildOut(b), nil
}

// GetConditionBuild opens a homebrew condition in the condition builder.
func (h *Handler) GetConditionBuild(ctx context.Context, p oas.GetConditionBuildParams) (oas.GetConditionBuildRes, error) {
	out, bad := h.conditionCall(ctx, "get condition build", func(c caller.Caller) (app.ConditionBuild, error) {
		return h.ConditionBuilds.Condition(ctx, c, uuid.UUID(p.EntryId))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// SaveConditionBuild saves a homebrew condition's design.
func (h *Handler) SaveConditionBuild(ctx context.Context, req *oas.ConditionDesign, p oas.SaveConditionBuildParams) (oas.SaveConditionBuildRes, error) {
	out, bad := h.conditionCall(ctx, "save condition build", func(c caller.Caller) (app.ConditionBuild, error) {
		return h.ConditionBuilds.SaveCondition(ctx, c, uuid.UUID(p.EntryId), conditionDesignIn(req))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// PreviewCondition checks a design without saving it.
func (h *Handler) PreviewCondition(ctx context.Context, req *oas.ConditionPreviewInput) (oas.PreviewConditionRes, error) {
	out, bad := h.conditionCall(ctx, "preview condition", func(caller.Caller) (app.ConditionBuild, error) {
		return h.ConditionBuilds.PreviewCondition(req.Name, conditionDesignIn(&req.Design))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}
