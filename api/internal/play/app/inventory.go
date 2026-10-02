package app

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/inventory"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// InventoryStore reads a Campaign's Containers and writes changes to them in one transaction.
type InventoryStore interface {
	LoadInventory(ctx context.Context, campaign uuid.UUID) (domain.Inventory, error)
	LiveSession(ctx context.Context, campaign uuid.UUID) (bool, error)
	WriteInventory(ctx context.Context, fn func(InventoryWriter) error) error
}

// InventoryWriter changes Containers inside a transaction.
type InventoryWriter interface {
	SetSlot(ctx context.Context, id domain.InstanceID, slot string) error
	AddInstance(ctx context.Context, id domain.InstanceID, to domain.ContainerID, slug, slot string, attuned bool) error
	SetAttuned(ctx context.Context, id domain.InstanceID, attuned bool) error
	Identify(ctx context.Context, id domain.InstanceID) error
	SetCharges(ctx context.Context, id domain.InstanceID, n int) error
	RemoveInstance(ctx context.Context, id domain.InstanceID) error
	MoveInstance(ctx context.Context, id domain.InstanceID, to domain.ContainerID) error
	SetStack(ctx context.Context, in domain.ContainerID, slug string, n int) error
	Heal(ctx context.Context, character uuid.UUID, hp int) error
	SyncEquipment(ctx context.Context, character uuid.UUID, armor string, shield bool, weapons []string) error
}

// Inventories runs the Inventory screen: a Character's equipment slots and bag, the Party Stash, and
// moving items between them outside a live Session.
type Inventories struct {
	Store   InventoryStore
	Members Members
	// Roll sums a number of dice with a number of faces, for a healing potion.
	Roll func(count, faces int) int
}

// ItemRef names one item in a Container: an Item Instance by id, or a plain stack by its item's slug.
type ItemRef struct {
	Instance domain.InstanceID
	Slug     string
}

// Destinations of a move.
const (
	ToBag       = "bag"
	ToSlot      = "slot"
	ToCharacter = "character"
	ToStash     = "stash"
)

// ItemMove sends some of an item somewhere: to the bag (unequipped), into a slot, to another Character
// or to the Party Stash.
type ItemMove struct {
	Item      ItemRef
	To        string
	Slot      string
	Character uuid.UUID
	Count     int
}

// InventoryView is what the Inventory screen shows.
type InventoryView struct {
	Bearer   domain.Bearer
	Mine     domain.Container
	Bags     []domain.Container
	Stash    domain.Container
	Party    []domain.Bearer
	Items    map[string]domain.ItemInfo
	WeightLb float64
	Capacity float64
	Load     inventory.Load
	// DM means the viewer is a DM, who sees what unidentified items are.
	DM bool
}

// View shows a Character's Inventory; its player or a DM.
func (s *Inventories) View(ctx context.Context, c caller.Caller, campaign, character uuid.UUID) (InventoryView, error) {
	inv, dm, err := s.load(ctx, c, campaign, character)
	if err != nil {
		return InventoryView{}, err
	}
	return view(inv, character, dm), nil
}

func (s *Inventories) load(ctx context.Context, c caller.Caller, campaign, character uuid.UUID) (domain.Inventory, bool, error) {
	me, err := s.Members.Membership(ctx, campaign, c.Subject)
	if err != nil {
		return domain.Inventory{}, false, err
	}
	inv, err := s.Store.LoadInventory(ctx, campaign)
	if err != nil {
		return inv, false, err
	}
	b, ok := bearerOf(inv, character)
	if !ok {
		return inv, false, apperr.ErrNotFound
	}
	if b.Owner != me.ID && !me.DM {
		return inv, false, apperr.ErrForbidden
	}
	return inv, me.DM, nil
}

func bearerOf(inv domain.Inventory, character uuid.UUID) (domain.Bearer, bool) {
	i := slices.IndexFunc(inv.Bearers, func(b domain.Bearer) bool { return b.CharacterID == character })
	if i < 0 {
		return domain.Bearer{}, false
	}
	return inv.Bearers[i], true
}

func containerOf(inv domain.Inventory, match func(domain.Container) bool) domain.Container {
	i := slices.IndexFunc(inv.Containers, match)
	if i < 0 {
		return domain.Container{}
	}
	return inv.Containers[i]
}

func characterContainer(inv domain.Inventory, character uuid.UUID) domain.Container {
	return containerOf(inv, func(c domain.Container) bool { return c.CharacterID != nil && *c.CharacterID == character })
}

func view(inv domain.Inventory, character uuid.UUID, dm bool) InventoryView {
	b, _ := bearerOf(inv, character)
	v := InventoryView{
		Bearer: b, Mine: characterContainer(inv, character), Items: inv.Items, DM: dm,
		Stash: containerOf(inv, func(c domain.Container) bool { return c.Kind == domain.ContainerStash }),
	}
	for _, c := range inv.Containers {
		if c.ParentID != nil && *c.ParentID == v.Mine.ID {
			v.Bags = append(v.Bags, c)
		}
	}
	for _, other := range inv.Bearers {
		if other.CharacterID != character {
			v.Party = append(v.Party, other)
		}
	}
	for _, c := range append([]domain.Container{v.Mine}, v.Bags...) {
		v.WeightLb += weight(c, inv.Items)
	}
	v.Capacity = inventory.Capacity(b.Strength)
	v.Load = inventory.LoadOf(v.WeightLb, v.Capacity)
	return v
}

func weight(c domain.Container, items map[string]domain.ItemInfo) float64 {
	w := inventory.CoinWeight(c.Coins)
	for slug, n := range c.Items {
		w += float64(n) * items[slug].WeightLb
	}
	for _, in := range c.Instances {
		w += float64(in.Quantity) * items[in.Slug].WeightLb
	}
	return w
}

// held is an item as found in a Container: a whole Instance, or a plain stack of a slug.
type held struct {
	instance *domain.Instance
	slug     string
	count    int
}

func findItem(c domain.Container, ref ItemRef) (held, bool) {
	if ref.Instance != (domain.InstanceID{}) {
		i := slices.IndexFunc(c.Instances, func(in domain.Instance) bool { return in.ID == ref.Instance })
		if i < 0 {
			return held{}, false
		}
		in := c.Instances[i]
		return held{instance: &in, slug: in.Slug, count: in.Quantity}, true
	}
	n, ok := c.Items[ref.Slug]
	return held{instance: nil, slug: ref.Slug, count: n}, ok
}

// Move sends an item from a Character's Inventory: into a slot, back into the bag, to another Character
// or to the Party Stash. Not during a live Session, where the Session's own Inventory panel rules.
func (s *Inventories) Move(ctx context.Context, c caller.Caller, campaign, character uuid.UUID, mv ItemMove) (InventoryView, error) {
	inv, dm, err := s.ready(ctx, c, campaign, character)
	if err != nil {
		return InventoryView{}, err
	}
	mine := characterContainer(inv, character)
	h, ok := findItem(mine, mv.Item)
	if !ok {
		return InventoryView{}, apperr.ErrNotFound
	}
	err = s.Store.WriteInventory(ctx, func(w InventoryWriter) error {
		switch mv.To {
		case ToSlot:
			return equip(ctx, w, inv, mine, h, mv.Slot)
		case ToBag:
			if err := unequip(ctx, w, mine, h); err != nil || h.instance == nil {
				return err
			}
			mine.Instances = slices.DeleteFunc(slices.Clone(mine.Instances), func(in domain.Instance) bool { return in.ID == h.instance.ID })
			return sync(ctx, w, inv, mine)
		case ToCharacter:
			to, ok := bearerOf(inv, mv.Character)
			if !ok || to.CharacterID == character {
				return apperr.Refuse("give it to another Character in this Campaign")
			}
			return giveAway(ctx, w, inv, mine, characterContainer(inv, to.CharacterID), h, mv.Count)
		case ToStash:
			return giveAway(ctx, w, inv, mine, containerOf(inv, func(c domain.Container) bool { return c.Kind == domain.ContainerStash }), h, mv.Count)
		}
		return apperr.Refuse("move it to the bag, a slot, another Character or the Party Stash")
	})
	if err != nil {
		return InventoryView{}, err
	}
	return s.after(ctx, campaign, character, dm)
}

// Take moves an item from the Party Stash into a Character's bag.
func (s *Inventories) Take(ctx context.Context, c caller.Caller, campaign, character uuid.UUID, ref ItemRef, count int) (InventoryView, error) {
	inv, dm, err := s.ready(ctx, c, campaign, character)
	if err != nil {
		return InventoryView{}, err
	}
	stash := containerOf(inv, func(c domain.Container) bool { return c.Kind == domain.ContainerStash })
	h, ok := findItem(stash, ref)
	if !ok {
		return InventoryView{}, apperr.ErrNotFound
	}
	err = s.Store.WriteInventory(ctx, func(w InventoryWriter) error {
		return transfer(ctx, w, stash, characterContainer(inv, character), h, count)
	})
	if err != nil {
		return InventoryView{}, err
	}
	return s.after(ctx, campaign, character, dm)
}

// Item uses.
const (
	Drink    = "drink"
	Throw    = "throw"
	Attune   = "attune"
	Unattune = "unattune"
	Study    = "identify"
	Charge   = "charge"
)

// Use acts on one item in a Character's Inventory: drinks a potion, restoring hit points if it heals;
// throws an item away; attunes or unattunes it; identifies it; or spends count of its charges.
func (s *Inventories) Use(ctx context.Context, c caller.Caller, campaign, character uuid.UUID, ref ItemRef, use string, count int) (InventoryView, int, error) {
	inv, dm, err := s.ready(ctx, c, campaign, character)
	if err != nil {
		return InventoryView{}, 0, err
	}
	mine := characterContainer(inv, character)
	h, ok := findItem(mine, ref)
	if !ok {
		return InventoryView{}, 0, apperr.ErrNotFound
	}
	b, _ := bearerOf(inv, character)
	healed := 0
	var change func(InventoryWriter) error
	switch use {
	case Drink:
		change, healed, err = s.drink(ctx, inv, mine, h, character)
	case Throw:
		change = func(w InventoryWriter) error { return takeOne(ctx, w, mine, h) }
	case Attune:
		change, err = attune(ctx, inv, mine, h, b)
	case Unattune, Study:
		change, err = settleChange(ctx, mine, h, use)
	case Charge:
		change, err = spendCharges(ctx, inv, mine, h, count)
	default:
		err = apperr.Refuse("drink, throw, attune, identify or spend charges")
	}
	if err != nil {
		return InventoryView{}, 0, err
	}
	if err := s.Store.WriteInventory(ctx, change); err != nil {
		return InventoryView{}, 0, err
	}
	v, err := s.after(ctx, campaign, character, dm)
	return v, healed, err
}

func (s *Inventories) drink(ctx context.Context, inv domain.Inventory, mine domain.Container, h held, character uuid.UUID) (func(InventoryWriter) error, int, error) {
	if inv.Items[h.slug].Category != "potion" {
		return nil, 0, apperr.Refuse("only a potion can be drunk")
	}
	healed := 0
	if p, heals := inventory.Healing(h.slug); heals {
		healed = s.Roll(p.Dice, p.Faces) + p.Bonus
	}
	return func(w InventoryWriter) error {
		if err := takeOne(ctx, w, mine, h); err != nil || healed == 0 {
			return err
		}
		return w.Heal(ctx, character, healed)
	}, healed, nil
}

// attune binds an identified magic item that needs it to the Character, up to three, when it meets the
// item's requirement.
func attune(ctx context.Context, inv domain.Inventory, mine domain.Container, h held, b domain.Bearer) (func(InventoryWriter) error, error) {
	info := inv.Items[h.slug]
	attuned := 0
	for _, in := range mine.Instances {
		if in.Attuned {
			attuned++
		}
	}
	caster := slices.ContainsFunc(b.Classes, func(c string) bool { return rules.CasterFor(c) != rules.NoCaster })
	switch {
	case !info.RequiresAttunement:
		return nil, apperr.Refuse(info.Name + " needs no attunement")
	case h.instance != nil && !h.instance.Identified:
		return nil, apperr.Refuse("identify it before attuning to it")
	case h.instance != nil && h.instance.Attuned:
		return nil, apperr.Refuse("already attuned")
	case attuned >= inventory.MaxAttuned:
		return nil, apperr.Refuse("you are attuned to three items already")
	case !inventory.CanAttune(info.AttunementDetail, b.Classes, caster):
		return nil, apperr.Refuse(info.AttunementDetail)
	}
	return func(w InventoryWriter) error {
		if h.instance != nil {
			return w.SetAttuned(ctx, h.instance.ID, true)
		}
		if err := w.SetStack(ctx, mine.ID, h.slug, h.count-1); err != nil {
			return err
		}
		return w.AddInstance(ctx, domain.InstanceID(uuid.New()), mine.ID, h.slug, "", true)
	}, nil
}

// settleChange unattunes or identifies an Item Instance; one that no longer stands out joins its plain
// stack.
func settleChange(ctx context.Context, mine domain.Container, h held, use string) (func(InventoryWriter) error, error) {
	switch {
	case h.instance == nil, use == Unattune && !h.instance.Attuned:
		return nil, apperr.Refuse("it is not attuned")
	case use == Study && h.instance.Identified:
		return nil, apperr.Refuse("it is already identified")
	}
	in := *h.instance
	if use == Unattune {
		in.Attuned = false
	} else {
		in.Identified = true
	}
	return func(w InventoryWriter) error {
		if in.Plain() {
			// Gone first: a plain Instance beside its plain stack would break the one-stack rule.
			if err := w.RemoveInstance(ctx, in.ID); err != nil {
				return err
			}
			return w.SetStack(ctx, mine.ID, in.Slug, mine.Items[in.Slug]+in.Quantity)
		}
		if use == Unattune {
			return w.SetAttuned(ctx, in.ID, false)
		}
		return w.Identify(ctx, in.ID)
	}, nil
}

// spendCharges uses some of an item's charges; a plain item becomes its own Instance to keep count.
func spendCharges(ctx context.Context, inv domain.Inventory, mine domain.Container, h held, count int) (func(InventoryWriter) error, error) {
	info := inv.Items[h.slug]
	if info.MaxCharges == 0 {
		return nil, apperr.Refuse(info.Name + " has no charges")
	}
	current := info.MaxCharges
	if h.instance != nil && h.instance.Charges != nil {
		current = *h.instance.Charges
	}
	if count < 1 || count > current {
		return nil, apperr.Refuse("there are not that many charges left")
	}
	return func(w InventoryWriter) error {
		if h.instance != nil {
			return w.SetCharges(ctx, h.instance.ID, current-count)
		}
		id := domain.InstanceID(uuid.New())
		if err := w.SetStack(ctx, mine.ID, h.slug, h.count-1); err != nil {
			return err
		}
		if err := w.AddInstance(ctx, id, mine.ID, h.slug, "", false); err != nil {
			return err
		}
		return w.SetCharges(ctx, id, current-count)
	}, nil
}

// ready loads an Inventory the caller may change now: theirs or as a DM, and with no live Session.
func (s *Inventories) ready(ctx context.Context, c caller.Caller, campaign, character uuid.UUID) (domain.Inventory, bool, error) {
	live, err := s.Store.LiveSession(ctx, campaign)
	if err != nil {
		return domain.Inventory{}, false, err
	}
	if live {
		return domain.Inventory{}, false, apperr.Refuse("a Session is live: move items from its Inventory panel")
	}
	return s.load(ctx, c, campaign, character)
}

func (s *Inventories) after(ctx context.Context, campaign, character uuid.UUID, dm bool) (InventoryView, error) {
	inv, err := s.Store.LoadInventory(ctx, campaign)
	return view(inv, character, dm), err
}

func takeOne(ctx context.Context, w InventoryWriter, from domain.Container, h held) error {
	if h.instance != nil {
		return w.RemoveInstance(ctx, h.instance.ID)
	}
	return w.SetStack(ctx, from.ID, h.slug, h.count-1)
}

// giveAway moves an item out of a Character's Inventory; one that was worn or held leaves its sheet too.
func giveAway(ctx context.Context, w InventoryWriter, inv domain.Inventory, mine, to domain.Container, h held, count int) error {
	if err := transfer(ctx, w, mine, to, h, count); err != nil || h.instance == nil || h.instance.Slot == "" {
		return err
	}
	mine.Instances = slices.DeleteFunc(slices.Clone(mine.Instances), func(in domain.Instance) bool { return in.ID == h.instance.ID })
	return sync(ctx, w, inv, mine)
}

// transfer moves a whole Instance, or some of a plain stack, into another Container; a plain Instance
// joins the stack already there.
func transfer(ctx context.Context, w InventoryWriter, from, to domain.Container, h held, count int) error {
	if h.instance == nil {
		if count < 1 || count > h.count {
			return apperr.Refuse("there are not that many to move")
		}
		if err := w.SetStack(ctx, from.ID, h.slug, h.count-count); err != nil {
			return err
		}
		return w.SetStack(ctx, to.ID, h.slug, to.Items[h.slug]+count)
	}
	in := *h.instance
	in.Slot = ""
	if in.Plain() {
		if err := w.RemoveInstance(ctx, in.ID); err != nil {
			return err
		}
		return w.SetStack(ctx, to.ID, in.Slug, to.Items[in.Slug]+in.Quantity)
	}
	return w.MoveInstance(ctx, in.ID, to.ID)
}

// equip puts one item into a slot it fits, first taking out whatever was there, and keeps the sheet's
// armor, shield and weapons in step.
func equip(ctx context.Context, w InventoryWriter, inv domain.Inventory, mine domain.Container, h held, slot string) error {
	if !inventory.Fits(slot, inv.Items[h.slug].Category, h.slug) {
		return apperr.Refuse(inv.Items[h.slug].Name + " does not go there")
	}
	if i := slices.IndexFunc(mine.Instances, func(in domain.Instance) bool { return in.Slot == slot }); i >= 0 {
		worn := mine.Instances[i]
		if err := unequip(ctx, w, mine, held{instance: &worn, slug: worn.Slug, count: worn.Quantity}); err != nil {
			return err
		}
		mine.Instances[i].Slot = ""
	}
	if h.instance != nil {
		if err := w.SetSlot(ctx, h.instance.ID, slot); err != nil {
			return err
		}
		h.instance.Slot = slot
		mine.Instances = replaceInstance(mine.Instances, *h.instance)
	} else {
		if err := w.SetStack(ctx, mine.ID, h.slug, h.count-1); err != nil {
			return err
		}
		if err := w.AddInstance(ctx, domain.InstanceID(uuid.New()), mine.ID, h.slug, slot, false); err != nil {
			return err
		}
		mine.Instances = append(mine.Instances, domain.Instance{Slug: h.slug, Quantity: 1, Identified: true, Slot: slot})
	}
	return sync(ctx, w, inv, mine)
}

func replaceInstance(list []domain.Instance, in domain.Instance) []domain.Instance {
	out := slices.Clone(list)
	for i := range out {
		if out[i].ID == in.ID {
			out[i] = in
		}
	}
	return out
}

// unequip takes an item out of its slot into the bag, joining a plain stack when nothing singles it out.
func unequip(ctx context.Context, w InventoryWriter, mine domain.Container, h held) error {
	if h.instance == nil || h.instance.Slot == "" {
		return nil
	}
	in := *h.instance
	in.Slot = ""
	if in.Plain() {
		if err := w.RemoveInstance(ctx, in.ID); err != nil {
			return err
		}
		return w.SetStack(ctx, mine.ID, in.Slug, mine.Items[in.Slug]+in.Quantity)
	}
	return w.SetSlot(ctx, in.ID, "")
}

// sync keeps the Character's armor, shield and weapons on its sheet in step with its slots.
func sync(ctx context.Context, w InventoryWriter, inv domain.Inventory, mine domain.Container) error {
	if mine.CharacterID == nil {
		return nil
	}
	armor, shield, weapons := "", false, []string{}
	for _, in := range mine.Instances {
		switch {
		case in.Slot == inventory.Body:
			armor = in.Slug
		case in.Slot == inventory.OffHand && in.Slug == "shield":
			shield = true
		case slices.Contains([]string{inventory.MainHand, inventory.OffHand, inventory.Ranged}, in.Slot) && inv.Items[in.Slug].Category == "weapon":
			weapons = append(weapons, in.Slug)
		}
	}
	return w.SyncEquipment(ctx, *mine.CharacterID, armor, shield, weapons)
}
