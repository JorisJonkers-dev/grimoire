package live

import (
	"context"
	"maps"
	"slices"
	"sort"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/inventory"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/loot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

func (s *state) container(id string) (domain.Container, bool) {
	cid := domain.ContainerID(parseID(id))
	i := slices.IndexFunc(s.inventory.Containers, func(c domain.Container) bool { return c.ID == cid })
	if i < 0 {
		return domain.Container{}, false
	}
	return s.inventory.Containers[i], true
}

func (s *state) bearer(c domain.Container) (domain.Bearer, bool) {
	if c.CharacterID == nil {
		return domain.Bearer{}, false
	}
	i := slices.IndexFunc(s.inventory.Bearers, func(b domain.Bearer) bool { return b.CharacterID == *c.CharacterID })
	if i < 0 {
		return domain.Bearer{}, false
	}
	return s.inventory.Bearers[i], true
}

// mine reports whether a member may take from or put into a Container: the DM always; a Player the
// Party Stash, loot drops (to take from) and their own Characters' Inventories. A bag belongs to
// whatever it sits in.
func (s *state) mine(m domain.Member, c domain.Container, taking bool) bool {
	c = s.root(c)
	switch {
	case m.DM, c.Kind == domain.ContainerStash:
		return true
	case c.Kind == domain.ContainerDrop:
		return taking
	}
	b, ok := s.bearer(c)
	return ok && b.Owner == m.ID
}

// planMove checks a transfer of items or coins between two Containers.
func (r *runtime) planMove(m domain.Member, cmd Command) (Write, string) {
	from, ok := r.st.container(cmd.FromID)
	to, ok2 := r.st.container(cmd.ToID)
	switch {
	case !ok || !ok2 || from.ID == to.ID:
		return Write{}, "Move it between two different places."
	case r.st.root(to).Kind == domain.ContainerDrop:
		return Write{}, "Loot is only taken from a drop, never put back."
	case !r.st.mine(m, from, true) || !r.st.mine(m, to, false):
		return Write{}, "That is not yours to move."
	}
	mv := domain.Move{From: from.ID, To: to.ID, Count: cmd.Count, FromLabel: from.Label, ToLabel: to.Label}
	if cmd.InstanceID != "" {
		return planMoveInstance(from, mv, cmd.InstanceID)
	}
	have, kind := from.Items[cmd.ItemSlug], domain.ActionItemMoved
	mv.Item = cmd.ItemSlug
	if cmd.Kind == CmdMoveCoins {
		have, kind, mv.Item, mv.Coin = from.Coins[cmd.Coin], domain.ActionCoinsMoved, "", cmd.Coin
	}
	if cmd.Count < 1 || cmd.Count > have {
		return Write{}, "There are not that many to move."
	}
	return Write{Kind: kind, Move: &mv}, ""
}

// planMoveInstance moves an Item Instance whole; it comes out of any slot it was worn in.
func planMoveInstance(from domain.Container, mv domain.Move, id string) (Write, string) {
	iid := domain.InstanceID(parseID(id))
	i := slices.IndexFunc(from.Instances, func(in domain.Instance) bool { return in.ID == iid })
	if i < 0 {
		return Write{}, "There is no such item there."
	}
	mv.Instance, mv.Item, mv.Count = &iid, from.Instances[i].Slug, from.Instances[i].Quantity
	return Write{Kind: domain.ActionItemMoved, Move: &mv}, ""
}

// planLoot rolls a Loot Table into a new drop.
func (r *runtime) planLoot(id string) (Write, string) {
	ctx := context.Background()
	tables, err := r.store.LoadLoot(ctx, r.campaign)
	if err != nil {
		r.log.Error("live: load loot", "error", err)
		return Write{}, "The loot tables could not be read."
	}
	tid := prep.LootTableID(parseID(id))
	i := slices.IndexFunc(tables, func(t prep.LootTable) bool { return t.ID == tid })
	if i < 0 {
		return Write{}, "No such loot table."
	}
	drops := loot.Roll(r.source(r.seed()), uuid.UUID(tid).String(), prep.LootRules(tables))
	if len(drops) == 0 {
		return Write{}, "The loot table dropped nothing."
	}
	c := domain.Container{ID: domain.ContainerID(uuid.New()), Kind: domain.ContainerDrop, Label: "Loot: " + tables[i].Name, Items: map[string]int{}, Coins: map[string]int{}, CreatedAt: r.now()}
	var slugs []string
	for _, d := range drops {
		if d.Coin != "" {
			c.Coins[d.Coin] = d.Count
			continue
		}
		c.Items[d.Slug] = d.Count
		slugs = append(slugs, d.Slug)
	}
	items, err := r.store.Items(ctx, r.campaign, slugs)
	if err != nil {
		r.log.Error("live: item info", "error", err)
		return Write{}, "The loot tables could not be read."
	}
	return Write{Kind: domain.ActionLootDropped, Drop: &c, items: items}, ""
}

// lootAfterFight rolls the Loot Table the DM named when ending a fight.
func (r *runtime) lootAfterFight(w Write, actor domain.Member, c caller.Caller) {
	if w.Kind != domain.ActionCombatEnded || w.lootTable == "" {
		return
	}
	if drop, reason := r.planLoot(w.lootTable); reason == "" {
		r.commit(request{}, drop, actor, c)
	}
}

// applyInventory adds a drop, or moves items or coins and clears a drop once it is empty.
func applyInventory(s *state, w *Write) {
	inv := &s.inventory
	if w.Drop != nil {
		inv.Containers = append(inv.Containers, w.Drop.Clone())
		maps.Copy(inv.Items, w.items)
		return
	}
	mv := w.Move
	from := slices.IndexFunc(inv.Containers, func(c domain.Container) bool { return c.ID == mv.From })
	to := slices.IndexFunc(inv.Containers, func(c domain.Container) bool { return c.ID == mv.To })
	shift := func(src, dst map[string]int, key string) {
		src[key] -= mv.Count
		dst[key] += mv.Count
		mv.Left, mv.Now = src[key], dst[key]
		if src[key] == 0 {
			delete(src, key)
		}
	}
	switch {
	case mv.Instance != nil:
		src := &inv.Containers[from]
		i := slices.IndexFunc(src.Instances, func(in domain.Instance) bool { return in.ID == *mv.Instance })
		in := src.Instances[i]
		in.Slot = ""
		src.Instances = slices.Delete(src.Instances, i, i+1)
		inv.Containers[to].Instances = append(inv.Containers[to].Instances, in)
	case mv.Coin != "":
		shift(inv.Containers[from].Coins, inv.Containers[to].Coins, mv.Coin)
	default:
		shift(inv.Containers[from].Items, inv.Containers[to].Items, mv.Item)
	}
	if c := inv.Containers[from]; c.Kind == domain.ContainerDrop && len(c.Items)+len(c.Instances)+len(c.Coins) == 0 {
		w.Gone = &c.ID
		inv.Containers = slices.Delete(inv.Containers, from, from+1)
	}
}

func cloneInventory(inv domain.Inventory) domain.Inventory {
	out := domain.Inventory{Bearers: inv.Bearers, Items: maps.Clone(inv.Items)}
	for _, c := range inv.Containers {
		out.Containers = append(out.Containers, c.Clone())
	}
	return out
}

// inventoryViews shows every Container to the DM and the party; the Table sees the Party Stash and the drops.
func (s *state) inventoryViews(a Audience) []ContainerView {
	out := []ContainerView{}
	for _, c := range s.inventory.Containers {
		if a == AudienceTable && s.root(c).Kind == domain.ContainerCharacter {
			continue
		}
		out = append(out, s.containerView(c, a))
	}
	return out
}

// maxNesting bounds how deep bags sit in each other, so a loop in stored data never hangs the runtime.
const maxNesting = 8

// root is the outermost Container a bag sits in.
func (s *state) root(c domain.Container) domain.Container {
	for range maxNesting {
		if c.ParentID == nil {
			return c
		}
		parent, ok := s.container(uuid.UUID(*c.ParentID).String())
		if !ok {
			return c
		}
		c = parent
	}
	return c
}

// weight is what a Container weighs with its coins and every bag inside it.
func (s *state) weight(c domain.Container, depth int) float64 {
	coins := 0
	for _, n := range c.Coins {
		coins += n
	}
	lb := float64(coins) / loot.CoinsPerPound
	for slug, n := range c.Items {
		lb += s.inventory.Items[slug].WeightLb * float64(n)
	}
	for _, in := range c.Instances {
		lb += s.inventory.Items[in.Slug].WeightLb * float64(in.Quantity)
	}
	if depth >= maxNesting {
		return lb
	}
	for _, bag := range s.inventory.Containers {
		if bag.ParentID != nil && *bag.ParentID == c.ID {
			lb += s.weight(bag, depth+1)
		}
	}
	return lb
}

func (s *state) itemInfo(slug string) domain.ItemInfo {
	info, ok := s.inventory.Items[slug]
	if !ok {
		info = domain.ItemInfo{Name: slug}
	}
	return info
}

func (s *state) containerView(c domain.Container, a Audience) ContainerView {
	v := ContainerView{ID: uuid.UUID(c.ID).String(), Kind: c.Kind, Label: c.Label, Items: []ItemView{}, Instances: []InstanceView{}, Coins: []CoinView{}}
	if c.ParentID != nil {
		v.ParentID = uuid.UUID(*c.ParentID).String()
	}
	for _, coin := range loot.Coins() {
		if n := c.Coins[coin]; n > 0 {
			v.Coins = append(v.Coins, CoinView{Coin: coin, Count: n})
		}
	}
	for slug, n := range c.Items {
		info := s.itemInfo(slug)
		v.Items = append(v.Items, ItemView{Slug: slug, Name: info.Name, Count: n, WeightLb: info.WeightLb * float64(n)})
	}
	sort.Slice(v.Items, func(i, j int) bool { return v.Items[i].Name < v.Items[j].Name })
	for _, in := range c.Instances {
		v.Instances = append(v.Instances, s.instanceView(in, a))
	}
	v.WeightLb = s.weight(c, 0)
	if b, ok := s.bearer(c); ok {
		v.CharacterID, v.OwnerID = b.CharacterID.String(), b.Owner.String()
		v.CapacityLb = loot.Capacity(b.Strength, "medium")
		v.Encumbered = v.WeightLb > v.CapacityLb
	} else if b, ok := s.bearer(s.root(c)); ok {
		v.OwnerID = b.Owner.String()
	}
	return v
}

// private closes every Inventory that is not the member's own: they see whose it is and what it weighs,
// never what is in it. The shared view is copied, never changed.
func private(u Update, m domain.Member) Update {
	if u.View == nil {
		return u
	}
	v := *u.View
	v.Inventory = closed(v.Inventory, m)
	u.View = &v
	if len(u.Steps) > 0 {
		u.Steps = slices.Clone(u.Steps)
		for i := range u.Steps {
			u.Steps[i].Inventory = closed(u.Steps[i].Inventory, m)
		}
	}
	return u
}

func closed(cs []ContainerView, m domain.Member) []ContainerView {
	if cs == nil {
		return nil
	}
	out := make([]ContainerView, len(cs))
	for i, c := range cs {
		if c.OwnerID != "" && c.OwnerID != m.ID.String() {
			c.Items, c.Instances, c.Coins = []ItemView{}, []InstanceView{}, []CoinView{}
		}
		out[i] = c
	}
	return out
}

func (s *state) instanceView(in domain.Instance, a Audience) InstanceView {
	info := s.itemInfo(in.Slug)
	v := InstanceView{
		ID: uuid.UUID(in.ID).String(), Slug: in.Slug, Name: info.Name, Count: in.Quantity, Identified: in.Identified, Attuned: in.Attuned, Slot: in.Slot,
		WeightLb: info.WeightLb * float64(in.Quantity),
	}
	if !in.Identified && a != AudienceDM {
		v.Slug, v.Name = "unknown", inventory.UnknownName(info.Category)
		return v
	}
	v.Charges = in.Charges
	if in.CustomName != "" {
		v.Name = in.CustomName
	}
	return v
}
