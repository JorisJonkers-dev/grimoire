package httpapi

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/monsterbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// MonsterBuilder is the monster builder for homebrew creatures.
type MonsterBuilder interface {
	Monster(ctx context.Context, c caller.Caller, id uuid.UUID) (app.MonsterBuild, error)
	SaveMonster(ctx context.Context, c caller.Caller, id uuid.UUID, d monsterbuild.Design) (app.MonsterBuild, error)
	PreviewMonster(name string, d monsterbuild.Design) (app.MonsterBuild, error)
}

var _ MonsterBuilder = (*app.Service)(nil)

// monsterDesignIn reads a design off the wire; the two share their JSON shape field for field.
func monsterDesignIn(d *oas.MonsterDesign) monsterbuild.Design {
	raw, _ := d.MarshalJSON()
	var out monsterbuild.Design
	_ = json.Unmarshal(raw, &out)
	return out
}

func monsterBuildOut(b app.MonsterBuild) *oas.MonsterBuildHeaders {
	raw, _ := json.Marshal(b.Design) //nolint:errchkjson // a design is plain data
	out := oas.MonsterBuild{Lines: append([]string{}, b.Lines...), Estimate: b.Estimate}
	_ = out.Design.UnmarshalJSON(raw)
	if b.Entry.ID != uuid.Nil {
		out.Entry, out.Slug = oas.NewOptLibraryEntry(libraryEntryOut(b.Entry)), oas.NewOptString(b.Slug)
	}
	return &oas.MonsterBuildHeaders{Response: out}
}

func (h *Handler) monsterCall(ctx context.Context, op string, run func(c caller.Caller) (app.MonsterBuild, error)) (*oas.MonsterBuildHeaders, *oas.ProblemStatusCodeWithHeaders) {
	b, bad := libraryCall(ctx, h, op, run)
	if bad != nil {
		return nil, bad
	}
	return monsterBuildOut(b), nil
}

// GetMonsterBuild opens a homebrew creature in the monster builder.
func (h *Handler) GetMonsterBuild(ctx context.Context, p oas.GetMonsterBuildParams) (oas.GetMonsterBuildRes, error) {
	out, bad := h.monsterCall(ctx, "get monster build", func(c caller.Caller) (app.MonsterBuild, error) {
		return h.MonsterBuilds.Monster(ctx, c, uuid.UUID(p.EntryId))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// SaveMonsterBuild saves a homebrew creature's design.
func (h *Handler) SaveMonsterBuild(ctx context.Context, req *oas.MonsterDesign, p oas.SaveMonsterBuildParams) (oas.SaveMonsterBuildRes, error) {
	out, bad := h.monsterCall(ctx, "save monster build", func(c caller.Caller) (app.MonsterBuild, error) {
		return h.MonsterBuilds.SaveMonster(ctx, c, uuid.UUID(p.EntryId), monsterDesignIn(req))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// PreviewMonster checks a design without saving it.
func (h *Handler) PreviewMonster(ctx context.Context, req *oas.MonsterPreviewInput) (oas.PreviewMonsterRes, error) {
	out, bad := h.monsterCall(ctx, "preview monster", func(caller.Caller) (app.MonsterBuild, error) {
		return h.MonsterBuilds.PreviewMonster(req.Name, monsterDesignIn(&req.Design))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}
