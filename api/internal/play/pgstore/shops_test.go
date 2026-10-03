package pgstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/rng"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	prepapp "github.com/JorisJonkers-dev/grimoire/api/internal/prep/app"
	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	preppg "github.com/JorisJonkers-dev/grimoire/api/internal/prep/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
)

func TestShopsAreStoredWithTheirTradesHagglesAndRestocks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	s, _ := sessions(tb, pgstore.New(tb.pool), &closed{}).Start(ctx, dm, tb.campaign)
	if _, err := comppg.New(tb.pool).Import(ctx, snapshot.Snapshot{
		Documents: []snapshot.Document{{Key: "srd-2024", Title: "SRD 5.2", RulesetYear: 2024, Precedence: 20, License: "CC-BY-4.0", Attribution: "a", URL: "https://a"}},
		Items:     []snapshot.Item{{Entry: snapshot.Entry{Document: "srd-2024", Slug: "rope", Name: "Rope", Description: "Rope."}, Category: "gear", CostGP: 1, WeightLB: 5}},
	}, "shops"); err != nil {
		t.Fatal(err)
	}
	char := uuid.New()
	if _, err := tb.pool.Exec(ctx, `WITH hero AS (INSERT INTO campaign.account_characters (id, owner_subject, name, ruleset, species_slug, class_slug, background_slug, created_at, updated_at)
			SELECT $1, m.auth_subject, 'Hero', 'srd-2024', 'human', 'fighter', 'soldier', now(), now() FROM campaign.members m WHERE m.id = $3)
		INSERT INTO campaign.characters (character_id, id, campaign_id, owner_member_id, name, ruleset, species_slug, class_slug, background_slug,
		ability_method, hp_max, hp_current, level) VALUES ($1, $1, $2, $3, 'Aria', 'srd-2024', 'human', 'bard', 'soldier', 'standard-array', 10, 10, 5)`, char, tb.campaign, tb.playerID); err != nil {
		t.Fatal(err)
	}
	if _, err := tb.pool.Exec(ctx, "INSERT INTO campaign.character_abilities (character_id, ability, base, bonus) VALUES ($1, 'charisma', 16, 0)", char); err != nil {
		t.Fatal(err)
	}
	if _, err := tb.pool.Exec(ctx, "INSERT INTO campaign.character_skills (character_id, skill, source) VALUES ($1, 'persuasion', 'class')", char); err != nil {
		t.Fatal(err)
	}
	var npc uuid.UUID
	if err := tb.pool.QueryRow(ctx, "INSERT INTO campaign.npcs (campaign_id, name) VALUES ($1, 'Hilda') RETURNING id", tb.campaign).Scan(&npc); err != nil {
		t.Fatal(err)
	}
	svc := &prepapp.Service{
		Repo: preppg.New(tb.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(tb.pool)}, Now: time.Now,
		Seed: func() uint64 { return 3 }, Source: func(seed uint64) dice.Source { return rng.New(seed) },
	}
	wares, _ := svc.SaveLootTable(ctx, dm, tb.campaign, prep.LootTable{Name: "Wares", Rolls: 1, Entries: []prep.LootEntry{{Weight: 1, Kind: "item", Item: "rope", Amount: "2"}}})
	town, _ := svc.SaveSettlement(ctx, dm, tb.campaign, prep.Settlement{Name: "Oakford", Size: "hamlet", Wealth: "poor"})
	shop, err := svc.SaveShop(ctx, dm, tb.campaign, prep.Shop{SettlementID: town.ID, Name: "Store", Kind: "general", OwnerID: &npc, HaggleDC: 15, HagglePct: 10, LootTable: &wares.ID, Restock: "long_rest"})
	if err != nil {
		t.Fatal(err)
	}
	store := pgstore.New(tb.pool)
	if bonus, err := store.TradeBonus(ctx, tb.campaign, char); err != nil || bonus != 3+3 {
		t.Fatalf("a level-5 bard with Charisma 16 = %d %v", bonus, err)
	}
	if _, err := store.TradeBonus(ctx, tb.campaign, uuid.New()); err == nil {
		t.Fatal("trade bonus of a stranger")
	}
	inv, _ := store.LoadInventory(ctx, tb.campaign)
	pack := inv.Containers[slicesIndex(inv.Containers, domain.ContainerCharacter)]
	open, err := store.LoadShop(ctx, tb.campaign, shop.ID)
	if err != nil || open.Owner != "Hilda" || open.Settlement != "Oakford" {
		t.Fatalf("open = %+v %v", open, err)
	}
	if none, err := store.LoadOpenShop(ctx, tb.campaign, s.ID); err != nil || none != nil {
		t.Fatalf("nothing open = %+v %v", none, err)
	}
	if _, err := store.LoadShop(ctx, tb.campaign, prep.ShopID(uuid.New())); err == nil {
		t.Fatal("an unknown shop opened")
	}
	stock, err := store.Stock(ctx, tb.campaign, shop, rng.New(1))
	if err != nil || len(stock) != 1 || stock[0].Quantity != 2 {
		t.Fatalf("stock = %+v %v", stock, err)
	}
	me := tb.dmMember(t)
	roll := domain.Roll{
		ID: domain.RollID(uuid.New()), CampaignID: tb.campaign, Purpose: "Haggle at Store", Notation: "1d20", RequestedBy: me.Name, Roller: me,
		Status: domain.StatusPending, Dice: []domain.Die{{No: 0, Group: 0, Faces: 20}},
	}
	minus := -10
	day := 1
	restocked := shop
	restocked.Stock, restocked.StockedDay = stock, 1
	writes := []live.Write{
		{Kind: domain.ActionShopOpened, Shop: open},
		{Kind: domain.ActionItemBought, Trade: &domain.Trade{Container: pack.ID, Label: "Aria", Item: "rope", Count: 1, Purse: map[string]int{"gp": 1, "sp": 2}, Carried: 1, StockLeft: 1, StockPrice: 100, PriceCP: 100, Shop: "Store"}},
		{Kind: domain.ActionItemSold, Trade: &domain.Trade{Container: pack.ID, Label: "Aria", Item: "rope", Count: 1, Purse: map[string]int{"gp": 1, "sp": 7}, Carried: 0, StockLeft: 2, StockPrice: 100, PriceCP: 50, Shop: "Store", Sold: true}},
		{Kind: domain.ActionTradeMade, Trades: []domain.Trade{
			{Container: pack.ID, Label: "Aria", Item: "rope", Count: 1, Purse: map[string]int{"sp": 7}, Carried: 1, StockLeft: 1, StockPrice: 100, PriceCP: 100, Shop: "Store"},
			{Container: pack.ID, Label: "Aria", Item: "rope", Count: 1, Purse: map[string]int{"gp": 1, "sp": 2}, Carried: 0, StockLeft: 2, StockPrice: 100, PriceCP: 50, Shop: "Store", Sold: true},
		}},
		live.WithHaggle(live.Write{Kind: domain.ActionHaggleStarted, Rolls: []domain.Roll{roll}}, char, roll.ID, nil),
		live.WithHaggle(live.Write{Kind: domain.ActionHaggled}, char, roll.ID, &minus),
		{Kind: domain.ActionStockRolled, Restock: &restocked, Day: &day},
	}
	for _, w := range writes {
		if _, err := store.Commit(ctx, s, nil, w, me, dm, time.Now()); err != nil {
			t.Fatalf("%s: %v", w.Kind, err)
		}
	}
	back, err := store.LoadOpenShop(ctx, tb.campaign, s.ID)
	if err != nil || back == nil || *back.Haggles[char].Adjust != -10 || len(back.Shop.Stock) != 1 || back.Shop.Stock[0].Quantity != 2 || back.Shop.StockedDay != 1 {
		t.Fatalf("open shop after = %+v %v", back, err)
	}
	if now, _ := store.GameClock(ctx, tb.campaign); now.Day != 1 || now.Minute != 0 {
		t.Fatalf("game clock = %+v", now)
	}
	inv, _ = store.LoadInventory(ctx, tb.campaign)
	if p := inv.Containers[slicesIndex(inv.Containers, domain.ContainerCharacter)]; len(p.Items) != 0 || p.Coins["sp"] != 2 || p.Coins["gp"] != 1 {
		t.Fatalf("pack after = %+v", p)
	}
	if _, err := store.Commit(ctx, s, nil, live.Write{Kind: domain.ActionShopClosed}, me, dm, time.Now()); err != nil {
		t.Fatal(err)
	}
	if none, _ := store.LoadOpenShop(ctx, tb.campaign, s.ID); none != nil {
		t.Fatalf("closed = %+v", none)
	}
	if _, err := store.Commit(ctx, s, nil, live.WithHaggle(live.Write{Kind: domain.ActionHaggled}, char, roll.ID, &minus), me, dm, time.Now()); err == nil {
		t.Fatal("a haggle with no shop open")
	}
	ops := map[string]func(repo *pgstore.Store) error{
		"load shop": func(repo *pgstore.Store) error { _, err := repo.LoadShop(ctx, tb.campaign, shop.ID); return err },
		"load open": func(repo *pgstore.Store) error { _, err := repo.LoadOpenShop(ctx, tb.campaign, s.ID); return err },
		"shops":     func(repo *pgstore.Store) error { _, err := repo.LoadShops(ctx, tb.campaign); return err },
		"stock":     func(repo *pgstore.Store) error { _, err := repo.Stock(ctx, tb.campaign, shop, rng.New(1)); return err },
		"prices": func(repo *pgstore.Store) error {
			_, err := repo.ItemPrices(ctx, tb.campaign, []string{"rope"})
			return err
		},
		"bonus": func(repo *pgstore.Store) error { _, err := repo.TradeBonus(ctx, tb.campaign, char); return err },
		"day":   func(repo *pgstore.Store) error { _, err := repo.GameClock(ctx, tb.campaign); return err },
	}
	for i, w := range writes {
		ops["write "+w.Kind+string(rune('a'+i))] = func(repo *pgstore.Store) error {
			if w.Kind == domain.ActionHaggleStarted {
				fresh := roll
				fresh.ID = domain.RollID(uuid.New())
				w = live.WithHaggle(live.Write{Kind: domain.ActionHaggleStarted, Rolls: []domain.Roll{fresh}}, char, fresh.ID, nil)
			}
			if _, err := store.Commit(ctx, s, nil, live.Write{Kind: domain.ActionShopOpened, Shop: open}, me, dm, time.Now()); err != nil {
				t.Fatal(err)
			}
			_, err := repo.Commit(ctx, s, nil, w, me, dm, time.Now())
			return err
		}
	}
	for name, op := range ops {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := op(pgstore.NewFaulty(tb.pool, f))
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}

func slicesIndex(cs []domain.Container, kind string) int {
	for i, c := range cs {
		if c.Kind == kind {
			return i
		}
	}
	return -1
}
