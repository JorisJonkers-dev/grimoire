package live

import (
	"context"
	"maps"
	"slices"
	"sort"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
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
// Party Stash, loot drops (to take from) and their own Characters' Inventories.
func (s *state) mine(m domain.Member, c domain.Container, taking bool) bool {
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
	case to.Kind == domain.ContainerDrop:
		return Write{}, "Loot is only taken from a drop, never put back."
	case !r.st.mine(m, from, true) || !r.st.mine(m, to, false):
		return Write{}, "That is not yours to move."
	}
	mv := domain.Move{From: from.ID, To: to.ID, Count: cmd.Count, FromLabel: from.Label, ToLabel: to.Label}
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
	if mv.Coin != "" {
		shift(inv.Containers[from].Coins, inv.Containers[to].Coins, mv.Coin)
	} else {
		shift(inv.Containers[from].Items, inv.Containers[to].Items, mv.Item)
	}
	if c := inv.Containers[from]; c.Kind == domain.ContainerDrop && len(c.Items)+len(c.Coins) == 0 {
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
		if a == AudienceTable && c.Kind == domain.ContainerCharacter {
			continue
		}
		out = append(out, s.containerView(c))
	}
	return out
}

func (s *state) containerView(c domain.Container) ContainerView {
	v := ContainerView{ID: uuid.UUID(c.ID).String(), Kind: c.Kind, Label: c.Label, Items: []ItemView{}, Coins: []CoinView{}}
	coins := 0
	for _, coin := range loot.Coins() {
		if n := c.Coins[coin]; n > 0 {
			v.Coins = append(v.Coins, CoinView{Coin: coin, Count: n})
			coins += n
		}
	}
	weight := float64(coins) / loot.CoinsPerPound
	for slug, n := range c.Items {
		info, ok := s.inventory.Items[slug]
		if !ok {
			info = domain.ItemInfo{Name: slug}
		}
		v.Items = append(v.Items, ItemView{Slug: slug, Name: info.Name, Count: n, WeightLb: info.WeightLb * float64(n)})
		weight += info.WeightLb * float64(n)
	}
	sort.Slice(v.Items, func(i, j int) bool { return v.Items[i].Name < v.Items[j].Name })
	v.WeightLb = weight
	if b, ok := s.bearer(c); ok {
		v.CharacterID, v.OwnerID = b.CharacterID.String(), b.Owner.String()
		v.CapacityLb = loot.Capacity(b.Strength, "medium")
		v.Encumbered = weight > v.CapacityLb
	}
	return v
}
