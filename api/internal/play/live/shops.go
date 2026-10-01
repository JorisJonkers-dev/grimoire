package live

import (
	"context"
	"maps"
	"slices"
	"sort"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/shops"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// planShop opens or closes a Shop, or trades or haggles at the open one.
func (r *runtime) planShop(m domain.Member, cmd Command) (Write, string) {
	switch cmd.Kind {
	case CmdOpenShop:
		open, err := r.store.LoadShop(context.Background(), r.campaign, prep.ShopID(parseID(cmd.ShopID)))
		if err != nil {
			return Write{}, "No such shop."
		}
		return Write{Kind: domain.ActionShopOpened, Shop: open}, ""
	case CmdCloseShop:
		if r.st.shop == nil {
			return Write{}, "No shop is open."
		}
		return Write{Kind: domain.ActionShopClosed}, ""
	}
	o := r.st.shop
	if o == nil {
		return Write{}, "No shop is open."
	}
	c, ok := r.st.container(cmd.FromID)
	switch {
	case !ok || c.Kind != domain.ContainerCharacter:
		return Write{}, "Trade from a Character's pack."
	case !r.st.mine(m, c, true):
		return Write{}, "That is not yours to trade."
	case cmd.Kind == CmdHaggle:
		return r.haggle(m, o, c)
	case cmd.Count < 1:
		return Write{}, "Trade at least one."
	case cmd.Kind == CmdBuy:
		return buy(o, c, cmd)
	}
	return r.sell(o, c, cmd)
}

// adjustFor is the haggled price adjustment a Character has at the open Shop.
func adjustFor(o *domain.OpenShop, c domain.Container) int {
	if h, ok := o.Haggles[*c.CharacterID]; ok && h.Adjust != nil {
		return *h.Adjust
	}
	return 0
}

// buy pays for items from the Shop's Stock out of a Character's purse, with change.
func buy(o *domain.OpenShop, c domain.Container, cmd Command) (Write, string) {
	i := slices.IndexFunc(o.Shop.Stock, func(k prep.StockItem) bool { return k.Slug == cmd.ItemSlug })
	if i < 0 || o.Shop.Stock[i].Quantity < cmd.Count {
		return Write{}, "The shop has not that many."
	}
	k := o.Shop.Stock[i]
	unit := shops.Price(k.PriceCP, 0, adjustFor(o, c))
	purse, ok := shops.Pay(c.Coins, unit*cmd.Count)
	if !ok {
		return Write{}, "That is more than the purse holds."
	}
	t := domain.Trade{
		Container: c.ID, Label: c.Label, Item: k.Slug, Count: cmd.Count, Purse: purse, Carried: c.Items[k.Slug] + cmd.Count, StockLeft: k.Quantity - cmd.Count,
		StockPrice: k.PriceCP, PriceCP: unit * cmd.Count, Shop: o.Shop.Name,
	}
	return Write{Kind: domain.ActionItemBought, Trade: &t}, ""
}

// sell pays half an item's base price for each one a Character sells, and puts them in Stock at the Shop's markup.
func (r *runtime) sell(o *domain.OpenShop, c domain.Container, cmd Command) (Write, string) {
	if c.Items[cmd.ItemSlug] < cmd.Count {
		return Write{}, "There are not that many to sell."
	}
	prices, err := r.store.ItemPrices(context.Background(), r.campaign, []string{cmd.ItemSlug})
	p, ok := prices[cmd.ItemSlug]
	if err != nil || !ok {
		return Write{}, "The shopkeeper does not know what that is worth."
	}
	pay := shops.SellPrice(p.BaseCP()) * cmd.Count
	left, ask := cmd.Count, shops.Price(p.BaseCP(), o.Shop.MarkupPct, 0)
	if i := slices.IndexFunc(o.Shop.Stock, func(k prep.StockItem) bool { return k.Slug == cmd.ItemSlug }); i >= 0 {
		left, ask = left+o.Shop.Stock[i].Quantity, o.Shop.Stock[i].PriceCP
	}
	t := domain.Trade{
		Container: c.ID, Label: c.Label, Item: cmd.ItemSlug, Count: cmd.Count, Purse: shops.Coins(shops.Worth(c.Coins) + pay), Carried: c.Items[cmd.ItemSlug] - cmd.Count,
		StockLeft: left, StockPrice: ask, PriceCP: pay, Shop: o.Shop.Name,
	}
	return Write{Kind: domain.ActionItemSold, Trade: &t, price: &p}, ""
}

// haggle opens a Persuasion Roll Card for a Character's Controller; one haggle per Character per visit.
func (r *runtime) haggle(m domain.Member, o *domain.OpenShop, c domain.Container) (Write, string) {
	if _, done := o.Haggles[*c.CharacterID]; done {
		return Write{}, "Each Character haggles once per visit."
	}
	b, _ := r.st.bearer(c)
	bonus, err := r.store.TradeBonus(context.Background(), r.campaign, *c.CharacterID)
	if err != nil {
		r.log.Error("live: trade bonus", "error", err)
		return Write{}, "The haggling could not start."
	}
	owner := b.Owner
	roll := r.request(m, domain.Token{Controller: &owner}, "Haggle at "+o.Shop.Name, "1d20", domain.Modifier{Label: "Persuasion", Value: bonus})
	return Write{Kind: domain.ActionHaggleStarted, Rolls: []domain.Roll{roll}, haggle: &haggleChange{character: *c.CharacterID, roll: &roll.ID}}, ""
}

// haggleChange is a Character's haggle the write starts or settles.
type haggleChange struct {
	character uuid.UUID
	roll      *domain.RollID
	adjust    *int
}

// pendingHaggle finds the Character whose haggle waits on a roll.
func (s *state) pendingHaggle(id domain.RollID) (uuid.UUID, bool) {
	if s.shop == nil {
		return uuid.UUID{}, false
	}
	for c, h := range s.shop.Haggles {
		if h.RollID != nil && *h.RollID == id && h.Adjust == nil {
			return c, true
		}
	}
	return uuid.UUID{}, false
}

// haggled settles a haggle once its roll is in: beating the Shop's DC lowers the price, missing it badly raises it.
func (r *runtime) haggled(character uuid.UUID, id domain.RollID) {
	roll, err := r.store.Roll(context.Background(), r.campaign, id)
	if err != nil || roll.Status != domain.StatusResolved {
		return
	}
	adjust := shops.Haggle(roll.Total, r.st.shop.Shop.HaggleDC, r.st.shop.Shop.HagglePct)
	w := Write{Kind: domain.ActionHaggled, haggle: &haggleChange{character: character, roll: &id, adjust: &adjust}}
	r.commit(request{}, w, roll.Roller, caller.Caller{Subject: roll.Roller.Subject, Origin: caller.OriginSystem})
}

// restock rolls fresh Stock for every Shop due after a long rest or the days a Travel Leg took.
func (r *runtime) restock(longRest bool, actor domain.Member, c caller.Caller) {
	ctx := context.Background()
	all, err := r.store.LoadShops(ctx, r.campaign)
	if err != nil {
		r.log.Error("live: load shops", "error", err)
		return
	}
	for _, s := range all {
		if !s.Due(longRest, r.st.day) {
			continue
		}
		stock, err := r.store.Stock(ctx, r.campaign, s, r.source(r.seed()))
		if err != nil {
			r.log.Error("live: stock", "error", err)
			return
		}
		s.Stock, s.StockedDay = stock, r.st.day
		r.commit(request{}, Write{Kind: domain.ActionStockRolled, Restock: &s}, actor, c)
	}
}

// applyShop opens, closes, trades at or restocks a Shop, and settles haggles.
func applyShop(s *state, w *Write) {
	switch w.Kind {
	case domain.ActionShopOpened:
		s.shop = w.Shop.Clone()
	case domain.ActionShopClosed:
		s.shop = nil
	case domain.ActionHaggleStarted, domain.ActionHaggled:
		s.shop.Haggles[w.haggle.character] = domain.Haggle{RollID: w.haggle.roll, Adjust: w.haggle.adjust}
	case domain.ActionStockRolled:
		if s.shop != nil && s.shop.Shop.ID == w.Restock.ID {
			s.shop.Shop.Stock = append([]prep.StockItem(nil), w.Restock.Stock...)
		}
	default:
		trade(s, w)
	}
}

// trade moves the item and the coins of a purchase or a sale.
func trade(s *state, w *Write) {
	t := w.Trade
	i := slices.IndexFunc(s.inventory.Containers, func(c domain.Container) bool { return c.ID == t.Container })
	c := &s.inventory.Containers[i]
	c.Coins = maps.Clone(t.Purse)
	c.Items[t.Item] = t.Carried
	if t.Carried == 0 {
		delete(c.Items, t.Item)
	}
	stock := s.shop.Shop.Stock
	k := slices.IndexFunc(stock, func(x prep.StockItem) bool { return x.Slug == t.Item })
	switch {
	case k < 0:
		stock = append(stock, prep.StockItem{Slug: t.Item, Quantity: t.StockLeft, PriceCP: t.StockPrice})
	case t.StockLeft == 0:
		stock = slices.Delete(stock, k, k+1)
	default:
		stock[k].Quantity = t.StockLeft
	}
	s.shop.Shop.Stock = stock
	if w.price != nil {
		s.shop.Items[t.Item] = *w.price
	}
	info := s.shop.Items[t.Item]
	s.inventory.Items[t.Item] = domain.ItemInfo{Name: info.Name, WeightLb: info.WeightLb}
}

// shopView shows the open Shop to every audience: its Stock and prices, and each Character's haggling.
func (s *state) shopView() *ShopView {
	o := s.shop
	if o == nil {
		return nil
	}
	v := &ShopView{ID: uuid.UUID(o.Shop.ID).String(), Name: o.Shop.Name, Kind: o.Shop.Kind, Settlement: o.Settlement, Owner: o.Owner, Stock: []StockView{}, Haggles: []HaggleView{}}
	for _, k := range o.Shop.Stock {
		info := o.Items[k.Slug]
		name := info.Name
		if name == "" {
			name = k.Slug
		}
		v.Stock = append(v.Stock, StockView{Slug: k.Slug, Name: name, Count: k.Quantity, PriceCP: k.PriceCP, WeightLb: info.WeightLb})
	}
	for c, h := range o.Haggles {
		hv := HaggleView{CharacterID: c.String(), AdjustPct: h.Adjust}
		if h.RollID != nil && h.Adjust == nil {
			hv.RollID = uuid.UUID(*h.RollID).String()
		}
		v.Haggles = append(v.Haggles, hv)
	}
	sort.Slice(v.Haggles, func(i, j int) bool { return v.Haggles[i].CharacterID < v.Haggles[j].CharacterID })
	return v
}

// HaggleChange is what a write does to a Character's haggle, for the store.
type HaggleChange struct {
	Character uuid.UUID
	Roll      uuid.UUID
	Adjust    *int
}

// HaggleOf reads the haggle a write starts or settles.
func HaggleOf(w Write) HaggleChange {
	return HaggleChange{Character: w.haggle.character, Roll: uuid.UUID(*w.haggle.roll), Adjust: w.haggle.adjust}
}

// RollIDOf reads a roll id sent on the wire.
func RollIDOf(id string) domain.RollID {
	return domain.RollID(parseID(id))
}

// WithHaggle sets the haggle a write starts or settles; the store's tests build writes with it.
func WithHaggle(w Write, character uuid.UUID, roll domain.RollID, adjust *int) Write {
	w.haggle = &haggleChange{character: character, roll: &roll, adjust: adjust}
	return w
}
