package httpapi

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/auth"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

func diceCalls(ctx context.Context, h *Handler, picture io.Reader) map[string]func() (any, error) {
	return map[string]func() (any, error){
		"list":   func() (any, error) { return h.ListDiceSets(ctx) },
		"shared": func() (any, error) { return h.ListSharedDiceSets(ctx) },
		"create": func() (any, error) { return h.CreateDiceSet(ctx, &oas.DiceSetChange{}) },
		"edit":   func() (any, error) { return h.EditDiceSet(ctx, &oas.DiceSetChange{}, oas.EditDiceSetParams{}) },
		"delete": func() (any, error) { return h.DeleteDiceSet(ctx, oas.DeleteDiceSetParams{}) },
		"share":  func() (any, error) { return h.ShareDiceSet(ctx, &oas.DiceSetSharingChange{}, oas.ShareDiceSetParams{}) },
		"copy":   func() (any, error) { return h.CopyDiceSet(ctx, oas.CopyDiceSetParams{}) },
		"choose": func() (any, error) { return h.ChooseDiceSet(ctx, &oas.DiceSetChoice{}) },
		"choose one": func() (any, error) {
			return h.ChooseDiceSet(ctx, &oas.DiceSetChoice{DiceSetId: oas.NewOptID(oas.ID{1})})
		},
		"upload": func() (any, error) {
			return h.SetDiceSetImage(ctx, oas.SetDiceSetImageReq{Data: picture}, oas.SetDiceSetImageParams{})
		},
		"take off":    func() (any, error) { return h.ClearDiceSetImage(ctx, oas.ClearDiceSetImageParams{}) },
		"picture":     func() (any, error) { return h.GetDiceSetImage(ctx, oas.GetDiceSetImageParams{}) },
		"review list": func() (any, error) { return h.ListDiceSetsToReview(ctx) },
		"review":      func() (any, error) { return h.ReviewDiceSet(ctx, &oas.DiceSetVerdict{}, oas.ReviewDiceSetParams{}) },
	}
}

// Without an identity every Dice Set call is refused before it reaches the service.
func TestDiceSetHandlersNeedAnIdentity(t *testing.T) {
	t.Parallel()
	for name, call := range diceCalls(context.Background(), &Handler{}, bytes.NewReader(nil)) {
		res, err := call()
		p, ok := res.(*oas.ProblemStatusCodeWithHeaders)
		if err != nil || !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s = %#v, %v", name, res, err)
		}
	}
}

type brokenDice struct{}

func (brokenDice) DiceSets(context.Context, string) ([]domain.DiceSet, *domain.DiceSetID, error) {
	return nil, nil, errFriends
}

func (brokenDice) SharedDiceSets(context.Context, string) ([]domain.DiceSet, error) {
	return nil, errFriends
}

func (brokenDice) CreateDiceSet(context.Context, string, string, domain.DiceDesign) (domain.DiceSet, error) {
	return domain.DiceSet{}, errFriends
}

func (brokenDice) EditDiceSet(context.Context, string, domain.DiceSetID, string, domain.DiceDesign) (domain.DiceSet, error) {
	return domain.DiceSet{}, errFriends
}

func (brokenDice) ShareDiceSet(context.Context, string, domain.DiceSetID, string) (domain.DiceSet, error) {
	return domain.DiceSet{}, errFriends
}

func (brokenDice) CopyDiceSet(context.Context, string, domain.DiceSetID) (domain.DiceSet, error) {
	return domain.DiceSet{}, errFriends
}

func (brokenDice) DeleteDiceSet(context.Context, string, domain.DiceSetID) error { return errFriends }

func (brokenDice) SetDiceSetPicture(context.Context, string, domain.DiceSetID, []byte) (domain.DiceSet, error) {
	return domain.DiceSet{}, errFriends
}

func (brokenDice) ClearDiceSetPicture(context.Context, string, domain.DiceSetID) (domain.DiceSet, error) {
	return domain.DiceSet{}, errFriends
}

func (brokenDice) DiceSetPicture(context.Context, string, domain.DiceSetID, bool) (domain.Picture, []byte, error) {
	return domain.Picture{}, nil, errFriends
}

func (brokenDice) ChooseDiceSet(context.Context, string, *domain.DiceSetID) error { return errFriends }

func (brokenDice) DiceSetsToReview(context.Context) ([]domain.DiceSet, error) { return nil, errFriends }

func (brokenDice) ReviewDiceSet(context.Context, domain.DiceSetID, bool) (domain.DiceSet, error) {
	return domain.DiceSet{}, errFriends
}
func (brokenDice) OwnsDiceSet(context.Context, string, domain.DiceSet) bool { return false }

// When the Dice Set store fails, every call answers 503 without saying why; a picture over the limit is
// refused before the store is asked.
func TestDiceSetsWhenTheStoreFails(t *testing.T) {
	t.Parallel()
	ctx := auth.WithIdentity(context.Background(), auth.Identity{Subject: "root"})
	h := &Handler{DiceSets: brokenDice{}, Accounts: admitAll{}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	for name, call := range diceCalls(ctx, h, bytes.NewReader([]byte("\x89PNG\r\n\x1a\n"))) {
		res, err := call()
		p, ok := res.(*oas.ProblemStatusCodeWithHeaders)
		if err != nil || !ok || p.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("%s = %#v, %v", name, res, err)
		}
	}
	huge := io.LimitReader(zeroes{}, 11<<20)
	res, err := h.SetDiceSetImage(ctx, oas.SetDiceSetImageReq{Data: huge}, oas.SetDiceSetImageParams{})
	if p, ok := res.(*oas.ProblemStatusCodeWithHeaders); err != nil || !ok || p.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("a picture over the limit = %#v, %v", res, err)
	}
}

type zeroes struct{}

func (zeroes) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
