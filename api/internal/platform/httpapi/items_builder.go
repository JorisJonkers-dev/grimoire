package httpapi

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/itembuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// ItemBuilder is the item builder for homebrew items.
type ItemBuilder interface {
	Item(ctx context.Context, c caller.Caller, id uuid.UUID) (app.ItemBuild, error)
	SaveItem(ctx context.Context, c caller.Caller, id uuid.UUID, d itembuild.Design) (app.ItemBuild, error)
	PreviewItem(name string, d itembuild.Design) (app.ItemBuild, error)
}

var _ ItemBuilder = (*app.Service)(nil)

// itemDesignIn reads a design off the wire; the two share their JSON shape field for field.
func itemDesignIn(d *oas.ItemDesign) itembuild.Design {
	raw, _ := d.MarshalJSON()
	var out itembuild.Design
	_ = json.Unmarshal(raw, &out)
	return out
}

func itemDesignOut(d itembuild.Design) oas.ItemDesign {
	raw, _ := json.Marshal(d) //nolint:errchkjson // a design is plain data
	var out oas.ItemDesign
	_ = out.UnmarshalJSON(raw)
	return out
}

func itemBuildOut(b app.ItemBuild) *oas.ItemBuildHeaders {
	p := b.Price
	out := oas.ItemBuild{
		Design: itemDesignOut(b.Design), Card: append([]string{}, b.Card...),
		Price: oas.ItemBuildPrice{Points: int32(p.Points), Suggested: p.Suggested, PriceGp: int32(p.PriceGP), Fits: p.Fits, Notes: append([]string{}, p.Notes...)}, //nolint:gosec // bounded by the builder
	}
	if b.Entry.ID != uuid.Nil {
		out.Entry, out.Slug = oas.NewOptLibraryEntry(libraryEntryOut(b.Entry)), oas.NewOptString(b.Slug)
	}
	return &oas.ItemBuildHeaders{Response: out}
}

func (h *Handler) itemCall(ctx context.Context, op string, run func(c caller.Caller) (app.ItemBuild, error)) (*oas.ItemBuildHeaders, *oas.ProblemStatusCodeWithHeaders) {
	b, bad := libraryCall(ctx, h, op, run)
	if bad != nil {
		return nil, bad
	}
	return itemBuildOut(b), nil
}

// GetItemBuild opens a homebrew item in the item builder.
func (h *Handler) GetItemBuild(ctx context.Context, p oas.GetItemBuildParams) (oas.GetItemBuildRes, error) {
	out, bad := h.itemCall(ctx, "get item build", func(c caller.Caller) (app.ItemBuild, error) {
		return h.ItemBuilder.Item(ctx, c, uuid.UUID(p.EntryId))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// SaveItemBuild saves a homebrew item's design.
func (h *Handler) SaveItemBuild(ctx context.Context, req *oas.ItemDesign, p oas.SaveItemBuildParams) (oas.SaveItemBuildRes, error) {
	out, bad := h.itemCall(ctx, "save item build", func(c caller.Caller) (app.ItemBuild, error) {
		return h.ItemBuilder.SaveItem(ctx, c, uuid.UUID(p.EntryId), itemDesignIn(req))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// PreviewItem checks a design without saving it.
func (h *Handler) PreviewItem(ctx context.Context, req *oas.ItemPreviewInput) (oas.PreviewItemRes, error) {
	out, bad := h.itemCall(ctx, "preview item", func(caller.Caller) (app.ItemBuild, error) {
		return h.ItemBuilder.PreviewItem(req.Name, itemDesignIn(&req.Design))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}
