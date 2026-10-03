package httpapi

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/rolltable"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// RollTableBuilder is the builder for Roll Table entries.
type RollTableBuilder interface {
	RollTable(ctx context.Context, c caller.Caller, id uuid.UUID) (app.RollTableBuild, error)
	SaveRollTable(ctx context.Context, c caller.Caller, id uuid.UUID, d rolltable.Design) (app.RollTableBuild, error)
	PreviewRollTable(d rolltable.Design) (app.RollTableBuild, error)
}

var _ RollTableBuilder = (*app.Service)(nil)

// rollTableDesignIn reads a design off the wire; the two share their JSON shape field for field.
func rollTableDesignIn(d *oas.RollTableDesign) rolltable.Design {
	raw, _ := d.MarshalJSON()
	var out rolltable.Design
	_ = json.Unmarshal(raw, &out)
	return out
}

func rollTableBuildOut(b app.RollTableBuild) *oas.RollTableBuildHeaders {
	raw, _ := json.Marshal(b.Design) //nolint:errchkjson // a design is plain data
	out := oas.RollTableBuild{Lines: append([]string{}, b.Lines...)}
	_ = out.Design.UnmarshalJSON(raw)
	if b.Entry.ID != uuid.Nil {
		out.Entry = oas.NewOptLibraryEntry(libraryEntryOut(b.Entry))
	}
	return &oas.RollTableBuildHeaders{Response: out}
}

func (h *Handler) rollTableCall(ctx context.Context, op string, run func(c caller.Caller) (app.RollTableBuild, error)) (*oas.RollTableBuildHeaders, *oas.ProblemStatusCodeWithHeaders) {
	b, bad := libraryCall(ctx, h, op, run)
	if bad != nil {
		return nil, bad
	}
	return rollTableBuildOut(b), nil
}

// GetRollTableBuild opens a Roll Table in its builder.
func (h *Handler) GetRollTableBuild(ctx context.Context, p oas.GetRollTableBuildParams) (oas.GetRollTableBuildRes, error) {
	out, bad := h.rollTableCall(ctx, "get roll table build", func(c caller.Caller) (app.RollTableBuild, error) {
		return h.RollTableBuilds.RollTable(ctx, c, uuid.UUID(p.EntryId))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// SaveRollTableBuild saves a Roll Table's design.
func (h *Handler) SaveRollTableBuild(ctx context.Context, req *oas.RollTableDesign, p oas.SaveRollTableBuildParams) (oas.SaveRollTableBuildRes, error) {
	out, bad := h.rollTableCall(ctx, "save roll table build", func(c caller.Caller) (app.RollTableBuild, error) {
		return h.RollTableBuilds.SaveRollTable(ctx, c, uuid.UUID(p.EntryId), rollTableDesignIn(req))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// PreviewRollTable checks a design without saving it.
func (h *Handler) PreviewRollTable(ctx context.Context, req *oas.RollTablePreviewInput) (oas.PreviewRollTableRes, error) {
	out, bad := h.rollTableCall(ctx, "preview roll table", func(caller.Caller) (app.RollTableBuild, error) {
		return h.RollTableBuilds.PreviewRollTable(rollTableDesignIn(&req.Design))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}
