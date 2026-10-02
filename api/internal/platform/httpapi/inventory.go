package httpapi

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	playapp "github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/inventory"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// InventoryService is what the Inventory screen's operations need.
type InventoryService interface {
	View(ctx context.Context, c caller.Caller, campaign, character uuid.UUID) (playapp.InventoryView, error)
	Move(ctx context.Context, c caller.Caller, campaign, character uuid.UUID, mv playapp.ItemMove) (playapp.InventoryView, error)
	Take(ctx context.Context, c caller.Caller, campaign, character uuid.UUID, ref playapp.ItemRef, count int) (playapp.InventoryView, error)
	Use(ctx context.Context, c caller.Caller, campaign, character uuid.UUID, ref playapp.ItemRef, use string, count int) (playapp.InventoryView, int, error)
	Swap(ctx context.Context, c caller.Caller, campaign, character uuid.UUID) (playapp.InventoryView, error)
}

func itemRef(instance oas.OptID, slug oas.OptSlug) playapp.ItemRef {
	ref := playapp.ItemRef{Slug: string(slug.Or(""))}
	if id, ok := instance.Get(); ok {
		ref.Instance = playdomain.InstanceID(id)
	}
	return ref
}

// GetInventory shows a Character's Inventory.
func (h *Handler) GetInventory(ctx context.Context, p oas.GetInventoryParams) (oas.GetInventoryRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	v, err := h.Inventory.View(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(p.CharacterId))
	if err != nil {
		return h.campaignProblem(ctx, "inventory", err), nil
	}
	return &oas.InventoryViewHeaders{Response: inventoryOut(v)}, nil
}

// MoveItem moves an item out of a Character's bag or slot.
func (h *Handler) MoveItem(ctx context.Context, req *oas.InventoryMove, p oas.MoveItemParams) (oas.MoveItemRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	mv := playapp.ItemMove{Item: itemRef(req.InstanceId, req.Slug), To: string(req.To), Slot: string(req.Slot.Or("")), Count: int(req.Count.Or(1))}
	if id, ok := req.CharacterId.Get(); ok {
		mv.Character = uuid.UUID(id)
	}
	v, err := h.Inventory.Move(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(p.CharacterId), mv)
	if err != nil {
		return h.campaignProblem(ctx, "move item", err), nil
	}
	return &oas.InventoryViewHeaders{Response: inventoryOut(v)}, nil
}

// TakeFromStash moves an item from the Party Stash into a Character's bag.
func (h *Handler) TakeFromStash(ctx context.Context, req *oas.InventoryTake, p oas.TakeFromStashParams) (oas.TakeFromStashRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	v, err := h.Inventory.Take(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(p.CharacterId), itemRef(req.InstanceId, req.Slug), int(req.Count.Or(1)))
	if err != nil {
		return h.campaignProblem(ctx, "take from stash", err), nil
	}
	return &oas.InventoryViewHeaders{Response: inventoryOut(v)}, nil
}

// SwapWeaponSet changes the weapon set in hand.
func (h *Handler) SwapWeaponSet(ctx context.Context, p oas.SwapWeaponSetParams) (oas.SwapWeaponSetRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	v, err := h.Inventory.Swap(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(p.CharacterId))
	if err != nil {
		return h.campaignProblem(ctx, "swap weapons", err), nil
	}
	return &oas.InventoryViewHeaders{Response: inventoryOut(v)}, nil
}

// UseItem drinks or throws an item.
func (h *Handler) UseItem(ctx context.Context, req *oas.InventoryUse, p oas.UseItemParams) (oas.UseItemRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	v, healed, err := h.Inventory.Use(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(p.CharacterId), itemRef(req.InstanceId, req.Slug), string(req.Use), int(req.Count.Or(1)))
	if err != nil {
		return h.campaignProblem(ctx, "use item", err), nil
	}
	return &oas.InventoryUseResultHeaders{Response: oas.InventoryUseResult{Inventory: inventoryOut(v), Healed: int32(healed)}}, nil //nolint:gosec // potion dice
}

func inventoryOut(v playapp.InventoryView) oas.InventoryView {
	out := oas.InventoryView{
		CharacterId: oas.ID(v.Bearer.CharacterID), Name: v.Bearer.Name, Slots: make([]oas.SlotLine, 0, len(inventory.Slots())),
		Bag: cardsOf(v.Mine, v.Items, false, v.DM), Coins: coinsOut(v.Mine.Coins), WeightLb: v.WeightLb, CapacityLb: v.Capacity,
		Load: oas.InventoryViewLoad(v.Load), Stash: cardsOf(v.Stash, v.Items, false, v.DM), StashCoins: coinsOut(v.Stash.Coins),
		Party: make([]oas.PartyBearer, 0, len(v.Party)), WeaponSet: oas.InventoryViewWeaponSet(v.Bearer.WeaponSet),
	}
	worn := cardsOf(v.Mine, v.Items, true, v.DM)
	for _, slot := range inventory.Slots() {
		line := oas.SlotLine{Slot: oas.EquipmentSlot(slot)}
		if i := slices.IndexFunc(worn, func(c oas.ItemCard) bool { return string(c.Slot.Or("")) == slot }); i >= 0 {
			line.Item = oas.NewOptItemCard(worn[i])
		}
		out.Slots = append(out.Slots, line)
	}
	for _, b := range v.Party {
		out.Party = append(out.Party, oas.PartyBearer{CharacterId: oas.ID(b.CharacterID), Name: b.Name})
	}
	return out
}

// card is an item as the Inventory screen shows it, with the slots it fits.
//
//nolint:gosec // counts are bounded by the constraints
func card(items map[string]playdomain.ItemInfo, slug string, n int) oas.ItemCard {
	info := items[slug]
	name := info.Name
	if name == "" {
		name = slug
	}
	k := oas.ItemCard{
		Slug: oas.Slug(slug), Name: name, Category: info.Category, Quantity: int32(n), WeightLb: info.WeightLb, Identified: true, Fits: []oas.EquipmentSlot{},
		RequiresAttunement: oas.NewOptBool(info.RequiresAttunement), MaxCharges: oas.NewOptInt32(int32(info.MaxCharges)),
	}
	if info.AttunementDetail != "" {
		k.AttunementDetail = oas.NewOptString(info.AttunementDetail)
	}
	for _, slot := range inventory.Slots() {
		if inventory.Fits(slot, info.Category, slug) {
			k.Fits = append(k.Fits, oas.EquipmentSlot(slot))
		}
	}
	return k
}

// instanceCard is an Item Instance as the Inventory screen shows it; a player sees an unidentified one
// only as an unknown item of its kind.
//
//nolint:gosec // charges are bounded by the constraints
func instanceCard(items map[string]playdomain.ItemInfo, in playdomain.Instance, dm bool) oas.ItemCard {
	if !in.Identified && !dm {
		info := items[in.Slug]
		return oas.ItemCard{
			InstanceId: oas.NewOptID(oas.ID(in.ID)), Slug: "unknown", Name: inventory.UnknownName(info.Category), Category: info.Category,
			Quantity: int32(in.Quantity), WeightLb: info.WeightLb, Identified: false, Attuned: false, Fits: []oas.EquipmentSlot{},
		}
	}
	k := card(items, in.Slug, in.Quantity)
	k.InstanceId = oas.NewOptID(oas.ID(in.ID))
	k.Identified, k.Attuned = in.Identified, in.Attuned
	if in.CustomName != "" {
		k.CustomName = oas.NewOptString(in.CustomName)
	}
	if in.Slot != "" {
		k.Slot = oas.NewOptEquipmentSlot(oas.EquipmentSlot(in.Slot))
	}
	if in.Charges != nil {
		k.Charges = oas.NewOptInt32(int32(*in.Charges))
	}
	return k
}

// cardsOf lists a Container's items, worn or carried in the bag.
func cardsOf(c playdomain.Container, items map[string]playdomain.ItemInfo, worn, dm bool) []oas.ItemCard {
	out := []oas.ItemCard{}
	if !worn {
		slugs := make([]string, 0, len(c.Items))
		for slug := range c.Items {
			slugs = append(slugs, slug)
		}
		slices.Sort(slugs)
		for _, slug := range slugs {
			out = append(out, card(items, slug, c.Items[slug]))
		}
	}
	for _, in := range c.Instances {
		if (in.Slot != "") == worn {
			out = append(out, instanceCard(items, in, dm))
		}
	}
	return out
}

//nolint:gosec // purses are bounded by their constraints
func coinsOut(coins map[string]int) []oas.LiveCoins {
	out := []oas.LiveCoins{}
	for _, coin := range []string{"pp", "gp", "ep", "sp", "cp"} {
		if n := coins[coin]; n > 0 {
			out = append(out, oas.LiveCoins{Coin: oas.Coin(coin), Count: int32(n)})
		}
	}
	return out
}
