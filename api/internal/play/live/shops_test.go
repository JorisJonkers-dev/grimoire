package live_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/rng"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	prepapp "github.com/JorisJonkers-dev/grimoire/api/internal/prep/app"
	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	preppg "github.com/JorisJonkers-dev/grimoire/api/internal/prep/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
)

func priced(slug, name string, gp, lb float64) snapshot.Item {
	return snapshot.Item{Entry: snapshot.Entry{Document: "srd-2024", Slug: slug, Name: name, Description: name + "."}, Category: "gear", CostGP: gp, WeightLB: lb}
}

// market opens a village with a general store that restocks after long rests and a curio shop every three days.
func market(t *testing.T, w world) map[string]string {
	t.Helper()
	ctx := context.Background()
	snap := snapshot.Snapshot{
		Documents: []snapshot.Document{{Key: "srd-2024", Title: "SRD 5.2", RulesetYear: 2024, Precedence: 20, License: "CC-BY-4.0", Attribution: "a", URL: "https://a"}},
		Items:     []snapshot.Item{priced("lantern", "Lantern", 5, 2), priced("hempen-rope", "Hempen Rope", 1, 5)},
	}
	if _, err := comppg.New(w.pool).Import(ctx, snap, "market"); err != nil {
		t.Fatal(err)
	}
	seed := uint64(100)
	svc := &prepapp.Service{
		Repo: preppg.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Now: time.Now,
		Seed: func() uint64 { seed++; return seed }, Source: func(s uint64) dice.Source { return rng.New(s) },
	}
	cid := w.session.CampaignID
	wares, err := svc.SaveLootTable(ctx, dmCaller, cid, prep.LootTable{Name: "Wares", Rolls: 1, Entries: []prep.LootEntry{{Weight: 1, Kind: "item", Item: "hempen-rope", Amount: "3"}}})
	if err != nil {
		t.Fatal(err)
	}
	fancy, _ := svc.SaveLootTable(ctx, dmCaller, cid, prep.LootTable{Name: "Fancy", Rolls: 1, Entries: []prep.LootEntry{{Weight: 1, Kind: "item", Item: "lantern", Amount: "2"}}})
	village, err := svc.SaveSettlement(ctx, dmCaller, cid, prep.Settlement{Name: "Oakford", Size: "village", Wealth: "modest"})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, s := range []prep.Shop{
		{SettlementID: village.ID, Name: "General Store", Kind: "general", MarkupPct: 50, HaggleDC: 15, HagglePct: 20, LootTable: &wares.ID, Restock: prep.RestockLongRest},
		{SettlementID: village.ID, Name: "Curios", Kind: "curios", HaggleDC: 15, HagglePct: 20, LootTable: &fancy.ID, Restock: prep.RestockDays, RestockDays: 3},
	} {
		shop, err := svc.SaveShop(ctx, dmCaller, cid, s)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.RerollStock(ctx, dmCaller, cid, shop.ID); err != nil {
			t.Fatal(err)
		}
		ids[shop.Name] = uuid.UUID(shop.ID).String()
	}
	return ids
}

func TestShoppingBuysSellsHagglesAndRestocks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb := ambushTable(t)
	loot := stocked(t, w)
	shops := market(t, w)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	refuse := func(sub *live.Subscriber, want string, cmd live.Command) {
		t.Helper()
		w.hub.Submit(sub, cmd)
		if u := next(t, sub); u.Kind != live.UpdRejected || !strings.Contains(strings.ToLower(u.Reason), want) {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	refuse(tb.dm, "no shop is open", live.Command{Kind: live.CmdCloseShop})
	refuse(tb.dm, "no such shop", live.Command{Kind: live.CmdOpenShop, ShopID: uuid.NewString()})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdRollLoot, LootTableID: loot["Purse"]})
	purse, aria, brom, stash := containerNamed(t, d.View, "Loot: Purse"), containerNamed(t, d.View, "Aria"), containerNamed(t, d.View, "Brom"), containerNamed(t, d.View, "Party Stash")
	tb.dmSays(live.Command{Kind: live.CmdMoveCoins, FromID: purse.ID, ToID: aria.ID, Coin: "gp", Count: 7})
	refuse(tb.player, "no shop is open", live.Command{Kind: live.CmdBuy, FromID: aria.ID, ItemSlug: "hempen-rope", Count: 1})
	_, p := tb.dmSays(live.Command{Kind: live.CmdOpenShop, ShopID: shops["General Store"]})
	if s := p.View.Shop; s == nil || s.Settlement != "Oakford" || len(s.Stock) != 1 || s.Stock[0] != (live.StockView{Slug: "hempen-rope", Name: "Hempen Rope", Count: 6, PriceCP: 150, WeightLb: 5}) {
		t.Fatalf("the general store = %+v", p.View.Shop)
	}
	buy := func(c live.ContainerView, slug string, n int) live.Command {
		return live.Command{Kind: live.CmdBuy, FromID: c.ID, ItemSlug: slug, Count: n}
	}
	refuse(tb.player, "not yours to trade", buy(brom, "hempen-rope", 1))
	refuse(tb.player, "character's pack", buy(stash, "hempen-rope", 1))
	refuse(tb.player, "at least one", buy(aria, "hempen-rope", 0))
	refuse(tb.player, "not that many", buy(aria, "hempen-rope", 7))
	refuse(tb.player, "not that many", buy(aria, "lantern", 1))
	refuse(tb.player, "more than the purse", live.Command{Kind: live.CmdBuy, FromID: aria.ID, ItemSlug: "hempen-rope", Count: 6})
	p = tb.playerSays(buy(aria, "hempen-rope", 2))
	if a := containerNamed(t, p.View, "Aria"); a.Items[0].Count != 2 || a.Coins[0] != (live.CoinView{Coin: "gp", Count: 4}) || p.View.Shop.Stock[0].Count != 4 {
		t.Fatalf("two ropes for 3 gp = %+v, stock %+v", a, p.View.Shop.Stock)
	}
	p = tb.playerSays(buy(aria, "hempen-rope", 2))
	if p.View.Shop.Stock[0].Count != 2 || containerNamed(t, p.View, "Aria").Coins[0] != (live.CoinView{Coin: "gp", Count: 1}) {
		t.Fatalf("two more ropes = %+v", p.View.Shop)
	}
	refuse(tb.player, "not that many to sell", live.Command{Kind: live.CmdSell, FromID: aria.ID, ItemSlug: "hempen-rope", Count: 9})
	p = tb.playerSays(live.Command{Kind: live.CmdSell, FromID: aria.ID, ItemSlug: "hempen-rope", Count: 1})
	if a := containerNamed(t, p.View, "Aria"); len(a.Coins) != 2 || a.Coins[0] != (live.CoinView{Coin: "sp", Count: 5}) || a.Items[0].Count != 3 || p.View.Shop.Stock[0].Count != 3 {
		t.Fatalf("a rope sells for half its price = %+v, stock %+v", a, p.View.Shop.Stock)
	}
	p = tb.playerSays(live.Command{Kind: live.CmdHaggle, FromID: aria.ID})
	if h := p.View.Shop.Haggles; len(h) != 1 || h[0].RollID == "" || h[0].AdjustPct != nil {
		t.Fatalf("haggling waits on its roll = %+v", h)
	}
	refuse(tb.player, "once per visit", live.Command{Kind: live.CmdHaggle, FromID: aria.ID})
	roll, _ := pgstore.New(w.pool).Roll(ctx, w.session.CampaignID, live.RollIDOf(p.View.Shop.Haggles[0].RollID))
	if roll.Purpose != "Haggle at General Store" || roll.Roller.ID != w.player.ID {
		t.Fatalf("the haggle roll = %+v", roll)
	}
	tb.fill(p.View.Shop.Haggles[0].RollID, w.player, 18)
	d, p = next(t, tb.dm), next(t, tb.player)
	if h := p.View.Shop.Haggles[0]; h.AdjustPct == nil || *h.AdjustPct != -10 || h.RollID != "" {
		t.Fatalf("an 18 against DC 15 takes 10%% off = %+v", h)
	}
	p = tb.playerSays(buy(aria, "hempen-rope", 1))
	if a := containerNamed(t, p.View, "Aria"); a.Coins[0] != (live.CoinView{Coin: "cp", Count: 5}) || a.Coins[1] != (live.CoinView{Coin: "sp", Count: 1}) {
		t.Fatalf("a haggled rope costs 135 cp = %+v", a.Coins)
	}
	tb.dmSays(live.Command{Kind: live.CmdCloseShop})
	_, p = tb.dmSays(live.Command{Kind: live.CmdOpenShop, ShopID: shops["Curios"]})
	if s := p.View.Shop; s.Name != "Curios" || s.Stock[0] != (live.StockView{Slug: "lantern", Name: "Lantern", Count: 4, PriceCP: 500, WeightLb: 2}) || len(s.Haggles) != 0 {
		t.Fatalf("the curio shop = %+v", s)
	}
	p = tb.playerSays(live.Command{Kind: live.CmdHaggle, FromID: aria.ID})
	pending := p.View.Shop.Haggles[0].RollID

	w.hub.Close(w.session.ID)
	tb.fill(pending, w.player, 1)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdResync})
	if s := next(t, tb.dm).View.Shop; s == nil || s.Name != "Curios" || s.Haggles[0].AdjustPct == nil || *s.Haggles[0].AdjustPct != 10 {
		t.Fatalf("the open shop and a haggle rolled while down survive a restart = %+v", s)
	}
	for day := 1; day <= 3; day++ {
		d, _ = tb.dmSays(live.Command{Kind: live.CmdRest, Rest: live.RestLong})
		if d.View.GameDay != day {
			t.Fatalf("day after rest = %d", d.View.GameDay)
		}
		restocks := 1
		if day == 3 {
			restocks = 2
		}
		for range restocks {
			d, _ = next(t, tb.dm), next(t, tb.player)
		}
	}
	if s := d.View.Shop; s.Stock[0].Count != 4 {
		t.Fatalf("the curios restock on day 3 = %+v", s.Stock)
	}
	var revisions int
	if err := w.pool.QueryRow(ctx, "SELECT count(*) FROM campaign.revisions WHERE entity_type = 'shop' AND action = 'update'").Scan(&revisions); err != nil || revisions != 6 {
		t.Fatalf("stock revisions = %d %v", revisions, err)
	}
	if day, _ := pgstore.New(w.pool).GameDay(ctx, w.session.CampaignID); day != 3 {
		t.Fatalf("the game day is kept = %d", day)
	}
}
