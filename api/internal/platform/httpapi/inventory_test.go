package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	playapp "github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

type brokenInventory struct{ err error }

func (b brokenInventory) View(context.Context, caller.Caller, uuid.UUID, uuid.UUID) (playapp.InventoryView, error) {
	return playapp.InventoryView{}, b.err
}

func (b brokenInventory) Move(context.Context, caller.Caller, uuid.UUID, uuid.UUID, playapp.ItemMove) (playapp.InventoryView, error) {
	return playapp.InventoryView{}, b.err
}

func (b brokenInventory) Take(context.Context, caller.Caller, uuid.UUID, uuid.UUID, playapp.ItemRef, int) (playapp.InventoryView, error) {
	return playapp.InventoryView{}, b.err
}

func (b brokenInventory) Use(context.Context, caller.Caller, uuid.UUID, uuid.UUID, playapp.ItemRef, string, int) (playapp.InventoryView, int, error) {
	return playapp.InventoryView{}, 0, b.err
}

func (b brokenInventory) CastFrom(context.Context, caller.Caller, uuid.UUID, uuid.UUID, playapp.ItemRef, string) (playapp.InventoryView, error) {
	return playapp.InventoryView{}, b.err
}

func (b brokenInventory) Swap(context.Context, caller.Caller, uuid.UUID, uuid.UUID) (playapp.InventoryView, error) {
	return playapp.InventoryView{}, b.err
}

func TestInventoryErrorsBecomeProblems(t *testing.T) {
	t.Parallel()
	h := campaignServer(t, brokenCampaigns{}, httpapi.InventoryService(brokenInventory{err: errors.New("disk")}))
	one := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/characters/0190c7a8-0000-7000-8000-000000000002/inventory"
	for _, o := range []struct{ method, path, body string }{
		{http.MethodGet, one, ""},
		{http.MethodPost, one + "/move", `{"slug":"rope","to":"stash"}`},
		{http.MethodPost, one + "/take", `{"instanceId":"0190c7a8-0000-7000-8000-000000000003"}`},
		{http.MethodPost, one + "/use", `{"slug":"rope","use":"throw"}`},
		{http.MethodPost, one + "/use", `{"slug":"rope","use":"cast","spell":"light"}`},
		{http.MethodPost, one + "/swap", ""},
	} {
		if rec := call(h, o.method, o.path, "u", o.body); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s: %d", o.method, o.path, rec.Code)
		}
	}
	ctx := context.Background()
	hh := &httpapi.Handler{Log: quiet}
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.GetInventory(ctx, oas.GetInventoryParams{}))
	add(hh.MoveItem(ctx, &oas.InventoryMove{}, oas.MoveItemParams{}))
	add(hh.TakeFromStash(ctx, &oas.InventoryTake{}, oas.TakeFromStashParams{}))
	add(hh.UseItem(ctx, &oas.InventoryUse{}, oas.UseItemParams{}))
	add(hh.SwapWeaponSet(ctx, oas.SwapWeaponSetParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}
