package live_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	prepapp "github.com/JorisJonkers-dev/grimoire/api/internal/prep/app"
	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	preppg "github.com/JorisJonkers-dev/grimoire/api/internal/prep/pgstore"
)

type failingInventory struct{ live.Store }

func (failingInventory) LoadInventory(context.Context, uuid.UUID) (domain.Inventory, error) {
	return domain.Inventory{}, errors.New("gone")
}

type failingLoot struct{ live.Store }

func (failingLoot) LoadLoot(context.Context, uuid.UUID) ([]prep.LootTable, error) {
	return nil, errors.New("gone")
}

type failingItems struct{ live.Store }

func (failingItems) Items(context.Context, uuid.UUID, []string) (map[string]domain.ItemInfo, error) {
	return nil, errors.New("gone")
}

func item(slug, name string, lb float64) snapshot.Item {
	return snapshot.Item{Entry: snapshot.Entry{Document: "srd-2024", Slug: slug, Name: name, Description: name + "."}, Category: "gear", WeightLB: lb}
}

func containerNamed(t *testing.T, v *live.View, label string) live.ContainerView {
	t.Helper()
	for _, c := range v.Inventory {
		if c.Label == label {
			return c
		}
	}
	t.Fatalf("no %s in %+v", label, v.Inventory)
	return live.ContainerView{}
}

// stocked gives the Campaign two Characters, a few items and Loot Tables, then reopens the Session.
func stocked(t *testing.T, w world) map[string]string {
	t.Helper()
	ctx := context.Background()
	snap := snapshot.Snapshot{
		Documents: []snapshot.Document{{Key: "srd-2024", Title: "SRD 5.2", RulesetYear: 2024, Precedence: 20, License: "CC-BY-4.0", Attribution: "a", URL: "https://a"}},
		Items:     []snapshot.Item{item("anvil", "Anvil", 100), item("rope", "Rope", 5)},
	}
	if _, err := comppg.New(w.pool).Import(ctx, snap, "loot"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name     string
		owner    uuid.UUID
		strength int
	}{{"Aria", w.player.ID, 8}, {"Brom", w.dm.ID, 15}} {
		id := uuid.New()
		if _, err := w.pool.Exec(ctx, `WITH hero AS (INSERT INTO campaign.account_characters (id, owner_subject, name, ruleset, species_slug, class_slug, background_slug, created_at, updated_at)
			SELECT $1, m.auth_subject, 'Hero', 'srd-2024', 'human', 'fighter', 'soldier', now(), now() FROM campaign.members m WHERE m.id = $3)
		INSERT INTO campaign.characters (character_id, id, campaign_id, owner_member_id, name, ruleset, species_slug, class_slug, background_slug,
			ability_method, hp_max, hp_current) VALUES ($1, $1, $2, $3, $4, 'srd-2024', 'human', 'fighter', 'soldier', 'standard-array', 10, 10)`,
			id, w.session.CampaignID, c.owner, c.name); err != nil {
			t.Fatal(err)
		}
		if _, err := w.pool.Exec(ctx, "INSERT INTO campaign.character_abilities (character_id, ability, base, bonus) VALUES ($1, 'strength', $2, 0)", id, c.strength); err != nil {
			t.Fatal(err)
		}
	}
	svc := &prepapp.Service{Repo: preppg.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Now: time.Now}
	ids := map[string]string{}
	save := func(tb prep.LootTable) prep.LootTable {
		saved, err := svc.SaveLootTable(ctx, dmCaller, w.session.CampaignID, tb)
		if err != nil {
			t.Fatal(err)
		}
		ids[saved.Name] = uuid.UUID(saved.ID).String()
		return saved
	}
	anvils := save(prep.LootTable{Name: "Anvils", Rolls: 1, Entries: []prep.LootEntry{{Weight: 1, Kind: "item", Item: "anvil", Amount: "2"}}})
	save(prep.LootTable{Name: "Hoard", Rolls: 2, Entries: []prep.LootEntry{{Weight: 1, Kind: "table", Table: &anvils.ID}}})
	save(prep.LootTable{Name: "Purse", Rolls: 1, Entries: []prep.LootEntry{{Weight: 1, Kind: "currency", Coin: "gp", Amount: "100"}}})
	save(prep.LootTable{Name: "Dust", Rolls: 1, Entries: []prep.LootEntry{{Weight: 1, Kind: "nothing"}}})
	w.hub.Close(w.session.ID)
	return ids
}

func TestLootDropsAndTheirItemsMoveBetweenInventories(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb := ambushTable(t)
	loot := stocked(t, w)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	table := join(t, w, w.player, playerCaller, live.AudienceTable)
	refuse := func(sub *live.Subscriber, want string, cmd live.Command) {
		t.Helper()
		w.hub.Submit(sub, cmd)
		if u := next(t, sub); u.Kind != live.UpdRejected || !strings.Contains(strings.ToLower(u.Reason), want) {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	w.hub.Submit(table, live.Command{Kind: live.CmdResync})
	if v := next(t, table).View; len(v.Inventory) != 1 || v.Inventory[0].Kind != domain.ContainerStash {
		t.Fatalf("the Table sees the stash only = %+v", v.Inventory)
	}
	refuse(tb.dm, "no such loot table", live.Command{Kind: live.CmdRollLoot, LootTableID: uuid.NewString()})
	refuse(tb.dm, "dropped nothing", live.Command{Kind: live.CmdRollLoot, LootTableID: loot["Dust"]})
	refuse(tb.player, "only the dm", live.Command{Kind: live.CmdRollLoot, LootTableID: loot["Hoard"]})
	d, p := tb.dmSays(live.Command{Kind: live.CmdRollLoot, LootTableID: loot["Hoard"]})
	next(t, table)
	hoard := containerNamed(t, p.View, "Loot: Hoard")
	if hoard.Kind != domain.ContainerDrop || len(hoard.Items) != 1 || hoard.Items[0] != (live.ItemView{Slug: "anvil", Name: "Anvil", Count: 4, WeightLb: 400}) || hoard.WeightLb != 400 {
		t.Fatalf("the hoard = %+v", hoard)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdPlace, Label: "Goblin", TokenKind: domain.TokenEnemy})
	next(t, table)
	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: []live.CombatantSetup{{TokenID: token(d.View, "Goblin").ID, SpeedFt: 30}}})
	next(t, table)
	tb.dmSays(live.Command{Kind: live.CmdEndCombat, LootTableID: loot["Purse"]})
	next(t, table)
	d, p = next(t, tb.dm), next(t, tb.player)
	next(t, table)
	purse := containerNamed(t, p.View, "Loot: Purse")
	aria, brom, stash := containerNamed(t, p.View, "Aria"), containerNamed(t, p.View, "Brom"), containerNamed(t, p.View, "Party Stash")
	if len(purse.Coins) != 1 || purse.Coins[0] != (live.CoinView{Coin: "gp", Count: 100}) || purse.WeightLb != 2 || aria.CapacityLb != 120 || aria.OwnerID != w.player.ID.String() {
		t.Fatalf("the purse = %+v, Aria = %+v", purse, aria)
	}
	move := func(from, to live.ContainerView, slug, coin string, n int) live.Command {
		kind := live.CmdMoveItem
		if coin != "" {
			kind = live.CmdMoveCoins
		}
		return live.Command{Kind: kind, FromID: from.ID, ToID: to.ID, ItemSlug: slug, Coin: coin, Count: n}
	}
	refuse(tb.player, "not yours", move(hoard, brom, "anvil", "", 1))
	refuse(tb.player, "not yours", move(brom, aria, "anvil", "", 1))
	refuse(tb.player, "never put back", move(aria, hoard, "anvil", "", 1))
	refuse(tb.player, "two different places", move(hoard, hoard, "anvil", "", 1))
	refuse(tb.player, "not that many", move(hoard, aria, "anvil", "", 5))
	refuse(tb.player, "not that many", move(hoard, aria, "rope", "", 1))
	refuse(tb.player, "not that many", move(purse, aria, "", "gp", 0))
	p = tb.playerSays(move(hoard, aria, "anvil", "", 3))
	if a := containerNamed(t, p.View, "Aria"); a.WeightLb != 300 || !a.Encumbered {
		t.Fatalf("three anvils weigh Aria down = %+v", a)
	}
	next(t, table)
	p = tb.playerSays(move(hoard, stash, "anvil", "", 1))
	next(t, table)
	if slices.ContainsFunc(p.View.Inventory, func(c live.ContainerView) bool { return c.Label == "Loot: Hoard" }) || containerNamed(t, p.View, "Party Stash").Items[0].Count != 1 {
		t.Fatalf("an emptied drop goes away = %+v", p.View.Inventory)
	}
	tb.playerSays(move(purse, aria, "", "gp", 60))
	next(t, table)
	d, _ = tb.dmSays(move(purse, brom, "", "gp", 40))
	next(t, table)
	if a, b := containerNamed(t, d.View, "Aria"), containerNamed(t, d.View, "Brom"); a.Coins[0].Count != 60 || a.WeightLb != 301.2 || b.Coins[0].Count != 40 || b.Encumbered {
		t.Fatalf("coins = Aria %+v Brom %+v", a, b)
	}
	tb.dmSays(move(aria, stash, "anvil", "", 3))
	next(t, table)
	tb.playerSays(move(stash, aria, "anvil", "", 1))

	w.hub.Close(w.session.ID)
	dm := join(t, w, w.dm, dmCaller, live.AudienceDM)
	w.hub.Submit(dm, live.Command{Kind: live.CmdResync})
	v := next(t, dm).View
	if a, s := containerNamed(t, v, "Aria"), containerNamed(t, v, "Party Stash"); len(v.Inventory) != 3 || a.Items[0].Count != 1 || a.Coins[0].Count != 60 || s.Items[0].Count != 3 {
		t.Fatalf("inventories survive a restart = %+v", v.Inventory)
	}
	var events int
	if err := w.pool.QueryRow(ctx, "SELECT count(*) FROM play.action_item_events").Scan(&events); err != nil || events != 8 {
		t.Fatalf("item events = %d %v", events, err)
	}
	w.hub.Close(w.session.ID)
	w.hub.Store = failingLoot{Store: pgstore.New(w.pool)}
	dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	refuse(dm, "could not be read", live.Command{Kind: live.CmdRollLoot, LootTableID: loot["Hoard"]})
	w.hub.Close(w.session.ID)
	w.hub.Store = failingItems{Store: pgstore.New(w.pool)}
	dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	refuse(dm, "could not be read", live.Command{Kind: live.CmdRollLoot, LootTableID: loot["Hoard"]})
	w.hub.Close(w.session.ID)
	w.hub.Store = failingInventory{Store: pgstore.New(w.pool)}
	if _, err := w.hub.Join(ctx, w.session.ID, w.dm, dmCaller, live.AudienceDM); err == nil {
		t.Fatal("inventory load failure ignored")
	}
}
