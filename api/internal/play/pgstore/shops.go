package pgstore

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	prepapp "github.com/JorisJonkers-dev/grimoire/api/internal/prep/app"
	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	preppg "github.com/JorisJonkers-dev/grimoire/api/internal/prep/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/clock"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// LoadShops reads the Campaign's Shops with their Stock.
func (s *Store) LoadShops(ctx context.Context, campaign uuid.UUID) ([]prep.Shop, error) {
	return preppg.LoadShops(ctx, s.q, campaign)
}

// ItemPrices reads what items cost.
func (s *Store) ItemPrices(ctx context.Context, campaign uuid.UUID, slugs []string) (map[string]prep.ItemPrice, error) {
	return preppg.LoadItemPrices(ctx, s.q, campaign, slugs)
}

// MarchingOrder reads the Characters placed in the Campaign's Marching Order, from the front.
func (s *Store) MarchingOrder(ctx context.Context, campaign uuid.UUID) ([]uuid.UUID, error) {
	return s.q.MarchingOrder(ctx, campaign)
}

// saveMarch replaces the Campaign's Marching Order.
func (s *Store) saveMarch(ctx context.Context, campaign uuid.UUID, order []uuid.UUID) error {
	if err := s.q.ClearMarchingOrder(ctx, campaign); err != nil {
		return err
	}
	for i, id := range order {
		if err := s.q.InsertMarcher(ctx, queries.InsertMarcherParams{CampaignID: campaign, Position: int32(i), CharacterID: id}); err != nil { //nolint:gosec // as many places as Characters
			return err
		}
	}
	return nil
}

// saveShared writes what a change does to what the whole Campaign shares: the Game Clock, the charges
// the dawns it passed gave back, the count of Short Rests and the Marching Order.
//
//nolint:gosec // days and minutes are bounded by the rules
func (s *Store) saveShared(ctx context.Context, campaign uuid.UUID, w live.Write) error {
	if w.Day != nil {
		if err := s.q.SetCampaignClock(ctx, queries.SetCampaignClockParams{ID: campaign, GameDay: int32(*w.Day), GameMinute: int32(w.Minute)}); err != nil {
			return err
		}
	}
	for _, rc := range w.Dawned {
		if err := s.SetCharges(ctx, rc.Instance, rc.Charges); err != nil {
			return err
		}
	}
	if w.ShortRests != nil {
		if err := s.q.SetCampaignShortRests(ctx, queries.SetCampaignShortRestsParams{ID: campaign, ShortRests: int32(*w.ShortRests)}); err != nil {
			return err
		}
	}
	if w.Kind == domain.ActionMarchingOrderSet {
		return s.saveMarch(ctx, campaign, w.March)
	}
	return nil
}

// Standings reads how each Faction of the Campaign regards the party and its Characters.
func (s *Store) Standings(ctx context.Context, campaign uuid.UUID) ([]domain.Standing, error) {
	rows, err := s.q.Standings(ctx, campaign)
	if err != nil {
		return nil, err
	}
	personal, err := s.q.ListPersonalStandings(ctx, campaign)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Standing, 0, len(rows))
	for _, r := range rows {
		st := domain.Standing{Faction: r.ID, Name: r.Name, Score: int(r.Score), Personal: map[uuid.UUID]int{}}
		for _, p := range personal {
			if p.FactionID == r.ID {
				st.Personal[p.CharacterID] = int(p.Score)
			}
		}
		out = append(out, st)
	}
	return out, nil
}

// GameClock reads the Campaign's Game Clock.
func (s *Store) GameClock(ctx context.Context, campaign uuid.UUID) (clock.Time, error) {
	c, err := s.q.CampaignClock(ctx, campaign)
	return clock.Time{Day: int(c.GameDay), Minute: int(c.GameMinute)}, err
}

// stockReader reads what generating Stock needs through one set of queries.
type stockReader struct{ q *queries.Queries }

func (r stockReader) Settlements(ctx context.Context, campaign uuid.UUID) ([]prep.Settlement, error) {
	return preppg.LoadSettlements(ctx, r.q, campaign)
}

func (r stockReader) LootTables(ctx context.Context, campaign uuid.UUID) ([]prep.LootTable, error) {
	return preppg.LoadLootTables(ctx, r.q, campaign)
}

func (r stockReader) ItemPrices(ctx context.Context, campaign uuid.UUID, slugs []string) (map[string]prep.ItemPrice, error) {
	return preppg.LoadItemPrices(ctx, r.q, campaign, slugs)
}

// Stock rolls fresh Stock for a Shop.
func (s *Store) Stock(ctx context.Context, campaign uuid.UUID, shop prep.Shop, src dice.Source) ([]prep.StockItem, error) {
	return prepapp.Stock(ctx, stockReader{q: s.q}, campaign, shop, src)
}

// TradeBonus is a Character's Persuasion bonus: their Charisma modifier, plus proficiency when skilled.
func (s *Store) TradeBonus(ctx context.Context, campaign, character uuid.UUID) (int, error) {
	rows, err := s.q.CharacterTrade(ctx, campaign)
	if err != nil {
		return 0, err
	}
	i := slices.IndexFunc(rows, func(r queries.CharacterTradeRow) bool { return r.ID == character })
	if i < 0 {
		return 0, apperr.ErrNotFound
	}
	bonus := rules.Modifier(int(rows[i].Charisma))
	if rows[i].Persuasive {
		bonus += rules.ProficiencyBonus(int(rows[i].Level))
	}
	return bonus, nil
}

// LoadShop reads a Shop to open, with where it is, who keeps it and what its items are.
func (s *Store) LoadShop(ctx context.Context, campaign uuid.UUID, id prep.ShopID) (*domain.OpenShop, error) {
	all, err := preppg.LoadShops(ctx, s.q, campaign)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(all, func(x prep.Shop) bool { return x.ID == id })
	if i < 0 {
		return nil, apperr.ErrNotFound
	}
	o := &domain.OpenShop{Shop: all[i], Haggles: map[uuid.UUID]domain.Haggle{}}
	places, err := preppg.LoadSettlements(ctx, s.q, campaign)
	if err != nil {
		return nil, err
	}
	o.Settlement = places[slices.IndexFunc(places, func(x prep.Settlement) bool { return x.ID == o.Shop.SettlementID })].Name
	if o.Shop.OwnerID != nil {
		if o.Owner, err = s.q.CampaignNpc(ctx, queries.CampaignNpcParams{CampaignID: campaign, ID: *o.Shop.OwnerID}); err != nil {
			return nil, err
		}
	}
	slugs := make([]string, 0, len(o.Shop.Stock))
	for _, k := range o.Shop.Stock {
		slugs = append(slugs, k.Slug)
	}
	o.Items, err = preppg.LoadItemPrices(ctx, s.q, campaign, slugs)
	return o, err
}

// LoadOpenShop reads the Shop a Session has open, with each Character's haggling there; nil when none is.
func (s *Store) LoadOpenShop(ctx context.Context, campaign uuid.UUID, sid domain.SessionID) (*domain.OpenShop, error) {
	ids, err := s.q.SessionShop(ctx, uuid.UUID(sid))
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	o, err := s.LoadShop(ctx, campaign, prep.ShopID(ids[0]))
	if err != nil {
		return nil, err
	}
	rows, err := s.q.SessionHaggles(ctx, uuid.UUID(sid))
	if err != nil {
		return nil, err
	}
	for _, h := range rows {
		x := domain.Haggle{}
		if h.RollID.Valid {
			id := domain.RollID(h.RollID.Bytes)
			x.RollID = &id
		}
		if h.AdjustPct.Valid {
			n := int(h.AdjustPct.Int32)
			x.Adjust = &n
		}
		o.Haggles[h.CharacterID] = x
	}
	return o, nil
}

// saveShop writes what a Shop change touched: the open Shop, a trade, a haggle, fresh Stock, and the day.
//
//nolint:gosec // days, counts, prices and adjustments are bounded by the rules
func (s *Store) saveShop(ctx context.Context, sess domain.Session, w live.Write, actor domain.Member, c caller.Caller, now time.Time) error {
	sid := uuid.UUID(sess.ID)
	if err := s.saveShared(ctx, sess.CampaignID, w); err != nil {
		return err
	}
	switch w.Kind {
	case domain.ActionShopOpened:
		if err := s.q.ClearSessionHaggles(ctx, sid); err != nil {
			return err
		}
		return s.q.SetSessionShop(ctx, queries.SetSessionShopParams{SessionID: sid, ShopID: uuid.UUID(w.Shop.Shop.ID)})
	case domain.ActionShopClosed:
		if err := s.q.ClearSessionHaggles(ctx, sid); err != nil {
			return err
		}
		return s.q.ClearSessionShop(ctx, sid)
	case domain.ActionItemBought, domain.ActionItemSold, domain.ActionTradeMade:
		return s.saveTrades(ctx, sess, w)
	case domain.ActionHaggleStarted, domain.ActionHaggled:
		if err := s.openRolls(ctx, sess, w.Rolls, actor, c, now); err != nil {
			return err
		}
		return s.saveHaggle(ctx, sess, w)
	case domain.ActionStockRolled:
		x := *w.Restock
		if err := preppg.WriteStock(ctx, s.q, x.ID, x.Stock, x.StockedDay); err != nil {
			return err
		}
		return preppg.RecordRestock(ctx, s.q, sess.CampaignID, x, actor.Name, c, now)
	}
	return nil
}

func (s *Store) saveHaggle(ctx context.Context, sess domain.Session, w live.Write) error {
	ids, err := s.q.SessionShop(ctx, uuid.UUID(sess.ID))
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return errors.New("pgstore: no shop is open")
	}
	h := live.HaggleOf(w)
	p := queries.SaveHaggleParams{SessionID: uuid.UUID(sess.ID), ShopID: ids[0], CharacterID: h.Character, RollID: pgtype.UUID{Bytes: h.Roll, Valid: true}}
	if h.Adjust != nil {
		p.AdjustPct = pgtype.Int4{Int32: int32(*h.Adjust), Valid: true} //nolint:gosec // bounded by the shop
	}
	return s.q.SaveHaggle(ctx, p)
}

// saveTrades writes a purchase, a sale, or every line of a trade in order.
func (s *Store) saveTrades(ctx context.Context, sess domain.Session, w live.Write) error {
	trades := w.Trades
	if w.Trade != nil {
		trades = append(trades, *w.Trade)
	}
	for _, t := range trades {
		if err := s.saveTrade(ctx, sess, t); err != nil {
			return err
		}
	}
	return nil
}

// saveTrade writes a Character's purse and pack after a trade, and what the Shop has left.
//
//nolint:gosec // counts and prices are bounded by the rules
func (s *Store) saveTrade(ctx context.Context, sess domain.Session, t domain.Trade) error {
	if err := s.q.ClearContainerCoins(ctx, uuid.UUID(t.Container)); err != nil {
		return err
	}
	for coin, n := range t.Purse {
		if err := s.setCount(ctx, t.Container, "", coin, n); err != nil {
			return err
		}
	}
	if err := s.setCount(ctx, t.Container, t.Item, "", t.Carried); err != nil {
		return err
	}
	ids, err := s.q.SessionShop(ctx, uuid.UUID(sess.ID))
	if err != nil {
		return err
	}
	if t.StockLeft == 0 {
		return s.q.DeleteShopStock(ctx, queries.DeleteShopStockParams{ShopID: ids[0], ItemSlug: t.Item})
	}
	return s.q.SetShopStock(ctx, queries.SetShopStockParams{ShopID: ids[0], ItemSlug: t.Item, Quantity: int32(t.StockLeft), PriceCp: int32(t.StockPrice)})
}
