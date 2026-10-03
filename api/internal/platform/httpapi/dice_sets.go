package httpapi

import (
	"bytes"
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/auth"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

// DiceSetService is the Dice Set use cases the HTTP adapter needs.
type DiceSetService interface {
	DiceSets(ctx context.Context, subject string) ([]domain.DiceSet, *domain.DiceSetID, error)
	SharedDiceSets(ctx context.Context, subject string) ([]domain.DiceSet, error)
	CreateDiceSet(ctx context.Context, subject, name string, design domain.DiceDesign) (domain.DiceSet, error)
	EditDiceSet(ctx context.Context, subject string, id domain.DiceSetID, name string, design domain.DiceDesign) (domain.DiceSet, error)
	ShareDiceSet(ctx context.Context, subject string, id domain.DiceSetID, sharing string) (domain.DiceSet, error)
	CopyDiceSet(ctx context.Context, subject string, id domain.DiceSetID) (domain.DiceSet, error)
	DeleteDiceSet(ctx context.Context, subject string, id domain.DiceSetID) error
	SetDiceSetPicture(ctx context.Context, subject string, id domain.DiceSetID, data []byte) (domain.DiceSet, error)
	ClearDiceSetPicture(ctx context.Context, subject string, id domain.DiceSetID) (domain.DiceSet, error)
	DiceSetPicture(ctx context.Context, subject string, id domain.DiceSetID, admin bool) (domain.Picture, []byte, error)
	ChooseDiceSet(ctx context.Context, subject string, id *domain.DiceSetID) error
	DiceSetsToReview(ctx context.Context) ([]domain.DiceSet, error)
	ReviewDiceSet(ctx context.Context, id domain.DiceSetID, approve bool) (domain.DiceSet, error)
	OwnsDiceSet(ctx context.Context, subject string, d domain.DiceSet) bool
}

// A Dice Set's picture can change under the same address, so caches keep it only briefly.
const diceImageCache = "private, max-age=60"

func (h *Handler) diceSetOut(ctx context.Context, subject string, d domain.DiceSet) oas.DiceSet {
	out := oas.DiceSet{
		ID: oas.ID(d.ID), Name: d.Name, HasImage: d.Image != nil, Sharing: oas.DiceSetSharing(d.Sharing), Review: oas.DiceSetReview(d.Review),
		Mine: h.DiceSets.OwnsDiceSet(ctx, subject, d), Copy: d.Copy(), By: d.MadeBy, UpdatedAt: d.UpdatedAt.UTC(),
	}
	convert(d.Design, &out.Design)
	if d.Image != nil {
		out.ImageUrl = oas.NewOptAssetUrl(oas.AssetUrl(diceImageURL(d)))
	}
	return out
}

func (h *Handler) diceSetList(ctx context.Context, subject string, sets []domain.DiceSet, chosen *domain.DiceSetID) *oas.DiceSetListHeaders {
	out := oas.DiceSetList{Items: make([]oas.DiceSet, 0, len(sets))}
	for _, d := range sets {
		out.Items = append(out.Items, h.diceSetOut(ctx, subject, d))
	}
	if chosen != nil {
		out.Chosen = oas.NewOptID(oas.ID(*chosen))
	}
	return &oas.DiceSetListHeaders{Response: out}
}

func (h *Handler) diceError(ctx context.Context, op string, err error) *oas.ProblemStatusCodeWithHeaders {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return problem(http.StatusNotFound, "Not found", "There is no such Dice Set, or it is not shared with you.")
	case errors.Is(err, domain.ErrConflict):
		return problem(http.StatusConflict, "Not possible", "A copy cannot be changed or shared, a set is copied once, and only a set that waits can be reviewed.")
	case errors.Is(err, domain.ErrInvalid):
		return problem(http.StatusUnprocessableEntity, "Invalid", "A Dice Set needs a name of up to 60 characters, looks for dice there are, and a PNG, JPEG or WebP picture of at most 10 MB.")
	}
	return h.friendError(ctx, op, err)
}

// ListDiceSets lists the caller's Dice Sets and the one they roll with.
func (h *Handler) ListDiceSets(ctx context.Context) (oas.ListDiceSetsRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	sets, chosen, err := h.DiceSets.DiceSets(ctx, id.Subject)
	if err != nil {
		return h.diceError(ctx, "list dice sets", err), nil
	}
	return h.diceSetList(ctx, id.Subject, sets, chosen), nil
}

// ListSharedDiceSets lists the sets of others the caller may copy.
func (h *Handler) ListSharedDiceSets(ctx context.Context) (oas.ListSharedDiceSetsRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	sets, err := h.DiceSets.SharedDiceSets(ctx, id.Subject)
	if err != nil {
		return h.diceError(ctx, "list shared dice sets", err), nil
	}
	return h.diceSetList(ctx, id.Subject, sets, nil), nil
}

func diceDesignIn(d oas.DiceDesign) domain.DiceDesign {
	out := domain.DiceDesign{Dice: map[string]domain.DieLook{}}
	convert(&d, &out) // the generated type marshals itself only through a pointer
	return out
}

// CreateDiceSet makes a Dice Set.
func (h *Handler) CreateDiceSet(ctx context.Context, req *oas.DiceSetChange) (oas.CreateDiceSetRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	d, err := h.DiceSets.CreateDiceSet(ctx, id.Subject, req.Name, diceDesignIn(req.Design))
	if err != nil {
		return h.diceError(ctx, "create dice set", err), nil
	}
	return &oas.DiceSetHeaders{Response: h.diceSetOut(ctx, id.Subject, d)}, nil
}

// EditDiceSet changes a Dice Set's name and looks.
func (h *Handler) EditDiceSet(ctx context.Context, req *oas.DiceSetChange, p oas.EditDiceSetParams) (oas.EditDiceSetRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	d, err := h.DiceSets.EditDiceSet(ctx, id.Subject, uuid.UUID(p.DiceSetId), req.Name, diceDesignIn(req.Design))
	if err != nil {
		return h.diceError(ctx, "edit dice set", err), nil
	}
	return &oas.DiceSetHeaders{Response: h.diceSetOut(ctx, id.Subject, d)}, nil
}

// DeleteDiceSet removes one of the caller's Dice Sets.
func (h *Handler) DeleteDiceSet(ctx context.Context, p oas.DeleteDiceSetParams) (oas.DeleteDiceSetRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.DiceSets.DeleteDiceSet(ctx, id.Subject, uuid.UUID(p.DiceSetId)); err != nil {
		return h.diceError(ctx, "delete dice set", err), nil
	}
	return &oas.DeleteDiceSetNoContent{}, nil
}

// ShareDiceSet sets who a Dice Set is shared with.
func (h *Handler) ShareDiceSet(ctx context.Context, req *oas.DiceSetSharingChange, p oas.ShareDiceSetParams) (oas.ShareDiceSetRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	d, err := h.DiceSets.ShareDiceSet(ctx, id.Subject, uuid.UUID(p.DiceSetId), string(req.Sharing))
	if err != nil {
		return h.diceError(ctx, "share dice set", err), nil
	}
	return &oas.DiceSetHeaders{Response: h.diceSetOut(ctx, id.Subject, d)}, nil
}

// CopyDiceSet takes a copy of a set shared with the caller.
func (h *Handler) CopyDiceSet(ctx context.Context, p oas.CopyDiceSetParams) (oas.CopyDiceSetRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	d, err := h.DiceSets.CopyDiceSet(ctx, id.Subject, uuid.UUID(p.DiceSetId))
	if err != nil {
		return h.diceError(ctx, "copy dice set", err), nil
	}
	return &oas.DiceSetHeaders{Response: h.diceSetOut(ctx, id.Subject, d)}, nil
}

// ChooseDiceSet sets the Dice Set the caller rolls with.
func (h *Handler) ChooseDiceSet(ctx context.Context, req *oas.DiceSetChoice) (oas.ChooseDiceSetRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	var chosen *domain.DiceSetID
	if v, ok := req.DiceSetId.Get(); ok {
		set := uuid.UUID(v)
		chosen = &set
	}
	if err := h.DiceSets.ChooseDiceSet(ctx, id.Subject, chosen); err != nil {
		return h.diceError(ctx, "choose dice set", err), nil
	}
	return &oas.ChooseDiceSetNoContent{}, nil
}

// SetDiceSetImage puts an uploaded picture on a Dice Set.
func (h *Handler) SetDiceSetImage(ctx context.Context, req oas.SetDiceSetImageReq, p oas.SetDiceSetImageParams) (oas.SetDiceSetImageRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	data, ok := readImage(req.Data)
	if !ok {
		return tooLarge(), nil
	}
	d, err := h.DiceSets.SetDiceSetPicture(ctx, id.Subject, uuid.UUID(p.DiceSetId), data)
	if err != nil {
		return h.diceError(ctx, "set dice set image", err), nil
	}
	return &oas.DiceSetHeaders{Response: h.diceSetOut(ctx, id.Subject, d)}, nil
}

// ClearDiceSetImage takes the uploaded picture off a Dice Set.
func (h *Handler) ClearDiceSetImage(ctx context.Context, p oas.ClearDiceSetImageParams) (oas.ClearDiceSetImageRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	d, err := h.DiceSets.ClearDiceSetPicture(ctx, id.Subject, uuid.UUID(p.DiceSetId))
	if err != nil {
		return h.diceError(ctx, "clear dice set image", err), nil
	}
	return &oas.DiceSetHeaders{Response: h.diceSetOut(ctx, id.Subject, d)}, nil
}

// GetDiceSetImage serves a Dice Set's picture to its owner, to those it is shared with, and to Admins.
func (h *Handler) GetDiceSetImage(ctx context.Context, p oas.GetDiceSetImageParams) (oas.GetDiceSetImageRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	img, data, err := h.DiceSets.DiceSetPicture(ctx, id.Subject, uuid.UUID(p.DiceSetId), h.Accounts.IsAdmin(ctx, id.Subject))
	if err != nil {
		return h.diceError(ctx, "get dice set image", err), nil
	}
	cache, body := oas.NewOptString(diceImageCache), bytes.NewReader(data)
	switch img.Type {
	case "image/png":
		return &oas.GetDiceSetImageOKImagePNGHeaders{CacheControl: cache, Response: oas.GetDiceSetImageOKImagePNG{Data: body}}, nil
	case "image/webp":
		return &oas.GetDiceSetImageOKImageWEBPHeaders{CacheControl: cache, Response: oas.GetDiceSetImageOKImageWEBP{Data: body}}, nil
	default:
		return &oas.GetDiceSetImageOKImageJpegHeaders{CacheControl: cache, Response: oas.GetDiceSetImageOKImageJpeg{Data: body}}, nil
	}
}

// ListDiceSetsToReview lists the sets that wait for an Admin.
func (h *Handler) ListDiceSetsToReview(ctx context.Context) (oas.ListDiceSetsToReviewRes, error) {
	by, bad := h.asAdmin(ctx)
	if bad != nil {
		return bad, nil
	}
	sets, err := h.DiceSets.DiceSetsToReview(ctx)
	if err != nil {
		return h.diceError(ctx, "list dice sets to review", err), nil
	}
	return h.diceSetList(ctx, by, sets, nil), nil
}

// ReviewDiceSet records an Admin's decision on a set that waits.
func (h *Handler) ReviewDiceSet(ctx context.Context, req *oas.DiceSetVerdict, p oas.ReviewDiceSetParams) (oas.ReviewDiceSetRes, error) {
	by, bad := h.asAdmin(ctx)
	if bad != nil {
		return bad, nil
	}
	d, err := h.DiceSets.ReviewDiceSet(ctx, uuid.UUID(p.DiceSetId), req.Approve)
	if err != nil {
		return h.diceError(ctx, "review dice set", err), nil
	}
	return &oas.DiceSetHeaders{Response: h.diceSetOut(ctx, by, d)}, nil
}
