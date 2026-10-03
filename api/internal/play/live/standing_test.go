package live_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
)

// A creature can belong to a Faction. Swaying it is rolled by how the Faction regards whoever tries:
// the Character's Personal Standing if it has one, else the party's. The Roll Card says so.
func TestStandingShapesAnInfluenceCheck(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb, ids := restingParty(t)
	watch := uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO campaign.factions (id, campaign_id, name, archetype, goals, territory, notes, score, created_at, updated_at)
			VALUES ($1, $2, 'The Lantern Watch', '', '', '', '', 80, now(), now())`, []any{watch, w.session.CampaignID}},
		{`INSERT INTO campaign.personal_standings (faction_id, character_id, score) VALUES ($1, $2, -70)`, []any{watch, ids["Brom"]}},
	} {
		if _, err := w.pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	refuse := func(want string, sub *live.Subscriber, cmd live.Command) {
		t.Helper()
		w.hub.Submit(sub, cmd)
		if u := next(t, sub); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, want) {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	refuse("No such Faction", tb.dm, live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenNPC, Label: "Stranger", Q: 1, R: 1, FactionID: uuid.NewString()})
	refuse("No such Faction", tb.dm, live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenNPC, Label: "Stranger", Q: 1, R: 1, FactionID: "the watch"})
	d, p := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenNPC, Label: "Watchman", Q: 1, FactionID: watch.String()})
	guard, brom := token(d.View, "Watchman"), ""
	for _, tv := range d.View.Tokens {
		if tv.Q == 2 {
			brom = tv.ID
		}
	}
	// A creature placed as one of a Faction is openly one of it: every screen is told whose it is, as
	// the Roll Card of a check against it will say. How it first takes to the party is the DM's to know.
	if guard.FactionID != watch.String() || guard.FirstReaction != "friendly" {
		t.Fatalf("the DM's watchman = %+v", guard)
	}
	if seen := token(p.View, "Watchman"); seen.FactionID != watch.String() || seen.FirstReaction != "" {
		t.Fatalf("a Player's watchman = %+v", seen)
	}
	// A creature the party cannot see is not there to be swayed: a Player who names it learns nothing,
	// not even whether it exists, and no roll is made that would say whose it is.
	d, p = tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenNPC, Label: "Spy", Q: 1, R: 1, Hidden: true, FactionID: watch.String()})
	spy := token(d.View, "Spy")
	if spy == nil || token(p.View, "Spy") != nil {
		t.Fatalf("the hidden spy: dm %+v, player %+v", spy, token(p.View, "Spy"))
	}
	rolls := func() int {
		t.Helper()
		var n int
		if err := w.pool.QueryRow(ctx, "SELECT count(*) FROM play.roll_requests WHERE campaign_id = $1", w.session.CampaignID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := rolls()
	for _, target := range []string{spy.ID, uuid.NewString()} {
		w.hub.Submit(tb.player, live.Command{Kind: live.CmdTakeAction, TokenID: ids["aria-token"], Action: "influence", TargetID: target})
		if u := next(t, tb.player); u.Kind != live.UpdRejected || u.Reason != "No such creature." {
			t.Fatalf("a Player sways a creature they cannot see = %+v", u)
		}
	}
	if rolls() != before {
		t.Fatal("a roll was made against a creature the party cannot see")
	}
	// The DM, who sees everything, may have one creature sway another the party cannot see.
	if d, _ := tb.dmSays(live.Command{Kind: live.CmdTakeAction, TokenID: guard.ID, Action: "influence", TargetID: spy.ID}); d.View == nil || rolls() != before+1 {
		t.Fatalf("the DM sways the spy = %+v, %d rolls after %d", d, rolls(), before)
	}
	type mod struct {
		Label string
		Value int
	}
	// rolled is the last Roll Request: how its d20 is rolled and what its card says beside the die.
	rolled := func() (string, []mod) {
		t.Helper()
		var id uuid.UUID
		var notation string
		if err := w.pool.QueryRow(ctx, "SELECT id, notation FROM play.roll_requests WHERE campaign_id = $1 ORDER BY created_at DESC, id LIMIT 1", w.session.CampaignID).Scan(&id, &notation); err != nil {
			t.Fatal(err)
		}
		rows, err := w.pool.Query(ctx, "SELECT label, value FROM play.roll_request_modifiers WHERE roll_id = $1 ORDER BY ordering", id)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		mods := []mod{}
		for rows.Next() {
			var m mod
			if err := rows.Scan(&m.Label, &m.Value); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(m.Label, " with ") {
				mods = append(mods, m)
			}
		}
		return notation, mods
	}
	sway := func(who, target string) (string, []mod) {
		t.Helper()
		tb.playerSays(live.Command{Kind: live.CmdTakeAction, TokenID: who, Action: "influence", TargetID: target})
		return rolled()
	}
	score := func(n int) {
		t.Helper()
		if _, err := w.pool.Exec(ctx, "UPDATE campaign.factions SET score = $2 WHERE id = $1", watch, n); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		name     string
		score    int
		who      string
		notation string
		line     []mod
	}{
		{"Allied", 80, ids["aria-token"], "2d20kh1", []mod{{"Allied with The Lantern Watch: Advantage", 0}}},
		{"a Personal Standing, Hostile", 80, brom, "2d20kl1", []mod{{"Hostile with The Lantern Watch: Disadvantage", 0}}},
		{"Friendly", 30, ids["aria-token"], "1d20", []mod{{"Friendly with The Lantern Watch", 2}}},
		{"Neutral", 0, ids["aria-token"], "1d20", []mod{{"Neutral with The Lantern Watch", 0}}},
		{"Unfriendly", -30, ids["aria-token"], "1d20", []mod{{"Unfriendly with The Lantern Watch", -2}}},
		{"Hostile", -90, ids["aria-token"], "2d20kl1", []mod{{"Hostile with The Lantern Watch: Disadvantage", 0}}},
	} {
		score(c.score)
		if notation, line := sway(c.who, guard.ID); notation != c.notation || !reflect.DeepEqual(line, c.line) {
			t.Fatalf("%s: rolled %s with %+v; want %s with %+v", c.name, notation, line, c.notation, c.line)
		}
	}
	// The DM's view follows the Standing as it is now.
	if got := token(look(t, w, tb.dm), "Watchman"); got.FirstReaction != "hostile" {
		t.Fatalf("the watchman's first reaction when Hostile = %+v", got)
	}
	// Nobody in particular, a creature of no Faction, and another kind of check: no Standing in it.
	score(80)
	if notation, line := sway(ids["aria-token"], ""); notation != "1d20" || len(line) != 0 {
		t.Fatalf("swaying nobody in particular: %s %+v", notation, line)
	}
	if notation, line := sway(ids["aria-token"], brom); notation != "1d20" || len(line) != 0 {
		t.Fatalf("swaying a creature of no Faction: %s %+v", notation, line)
	}
	tb.playerSays(live.Command{Kind: live.CmdTakeAction, TokenID: ids["aria-token"], Action: "search", TargetID: guard.ID})
	if notation, line := rolled(); notation != "1d20" || len(line) != 0 {
		t.Fatalf("searching beside a watchman: %s %+v", notation, line)
	}
	refuse("No such creature", tb.player, live.Command{Kind: live.CmdTakeAction, TokenID: ids["aria-token"], Action: "influence", TargetID: uuid.NewString()})
	// It is kept: after a restart the watchman is still the Watch's.
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	if got := token(look(t, w, tb.dm), "Watchman"); got.FactionID != watch.String() || got.FirstReaction != "friendly" {
		t.Fatalf("after a restart = %+v", got)
	}
}

// purseCP is what a Container's coins are worth in copper.
func purseCP(c live.ContainerView) int {
	worth := map[string]int{"cp": 1, "sp": 10, "ep": 50, "gp": 100, "pp": 1000}
	total := 0
	for _, coin := range c.Coins {
		total += worth[coin.Coin] * coin.Count
	}
	return total
}

// A Shop of a Faction prices by how the Faction regards whoever buys: the Character's Personal
// Standing if it has one, else the party's. A Shop of no Faction prices as it always did.
func TestAMemberShopPricesByStanding(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb := ambushTable(t)
	loot := stocked(t, w)
	shops := market(t, w)
	watch := uuid.New()
	var aria uuid.UUID
	if err := w.pool.QueryRow(ctx, "SELECT id FROM campaign.characters WHERE campaign_id = $1 AND name = 'Aria'", w.session.CampaignID).Scan(&aria); err != nil {
		t.Fatal(err)
	}
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO campaign.factions (id, campaign_id, name, archetype, goals, territory, notes, score, created_at, updated_at)
			VALUES ($1, $2, 'The Lantern Watch', '', '', '', '', 30, now(), now())`, []any{watch, w.session.CampaignID}},
		{`UPDATE prep.shops SET faction_id = $1 WHERE id = $2`, []any{watch, shops["General Store"]}},
	} {
		if _, err := w.pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	d, _ := tb.dmSays(live.Command{Kind: live.CmdRollLoot, LootTableID: loot["Purse"]})
	purse, pack := containerNamed(t, d.View, "Loot: Purse"), containerNamed(t, d.View, "Aria")
	tb.dmSays(live.Command{Kind: live.CmdMoveCoins, FromID: purse.ID, ToID: pack.ID, Coin: "gp", Count: 7})
	_, p := tb.dmSays(live.Command{Kind: live.CmdOpenShop, ShopID: shops["General Store"]})
	if s := p.View.Shop.Standing; s == nil || *s != (live.ShopStandingView{Faction: "The Lantern Watch", Tier: "friendly", PricePct: -10}) || p.View.Shop.Stock[0].PriceCP != 150 {
		t.Fatalf("the Watch's store, to its friends = %+v %+v", p.View.Shop.Standing, p.View.Shop.Stock)
	}
	set := func(sql string, args ...any) {
		t.Helper()
		if _, err := w.pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	left := 700
	buy := func(want int, why string) {
		t.Helper()
		p := tb.playerSays(live.Command{Kind: live.CmdBuy, FromID: pack.ID, ItemSlug: "hempen-rope", Count: 1})
		if p.View == nil {
			t.Fatalf("%s: %+v", why, p)
		}
		if got := left - purseCP(containerNamed(t, p.View, "Aria")); got != want {
			t.Fatalf("%s: a rope listed at 150 cp cost %d cp, want %d", why, got, want)
		}
		left -= want
	}
	buy(135, "Friendly")
	set("UPDATE campaign.factions SET score = 0 WHERE id = $1", watch)
	buy(150, "Neutral")
	set("UPDATE campaign.factions SET score = -90 WHERE id = $1", watch)
	// The buyer's own Standing counts, not the party's: Aria is Allied where the party is Hostile.
	set("INSERT INTO campaign.personal_standings (faction_id, character_id, score) VALUES ($1, $2, 90)", watch, aria)
	buy(120, "a Personal Standing, Allied")
	set("DELETE FROM campaign.personal_standings WHERE faction_id = $1", watch)
	buy(225, "Hostile")
	if s := look(t, w, tb.player).Shop.Standing; s == nil || *s != (live.ShopStandingView{Faction: "The Lantern Watch", Tier: "hostile", PricePct: 50}) {
		t.Fatalf("the Watch's store, to its enemies = %+v", s)
	}
	// A Shop of no Faction has no Standing to price by.
	tb.dmSays(live.Command{Kind: live.CmdCloseShop})
	_, p = tb.dmSays(live.Command{Kind: live.CmdOpenShop, ShopID: shops["Curios"]})
	if p.View.Shop.Standing != nil {
		t.Fatalf("a Shop of no Faction = %+v", p.View.Shop.Standing)
	}
}

// On an Encounter Table a Faction's own entries are drawn more the worse the party stands with it.
func TestEncounterDrawsFollowStanding(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb := ambushTable(t)
	_, svc := prepared(t, w)
	watch := uuid.New()
	if _, err := w.pool.Exec(ctx, `INSERT INTO campaign.factions (id, campaign_id, name, archetype, goals, territory, notes, score, created_at, updated_at)
		VALUES ($1, $2, 'The Lantern Watch', '', '', '', '', -80, now(), now())`, watch, w.session.CampaignID); err != nil {
		t.Fatal(err)
	}
	road, err := svc.SaveTable(ctx, dmCaller, w.session.CampaignID, prep.Table{Name: "Watch road", ChancePct: 100, Visibility: prep.Secret, Entries: []prep.Entry{
		{Weight: 10, Kind: prep.EntryEncounter, Label: "Watch patrol", Monsters: []prep.EntryMonster{{Slug: "goblin", Count: 1}}, FactionID: &watch},
		{Weight: 10, Kind: prep.EntryEncounter, Label: "Ogre", Monsters: []prep.EntryMonster{{Slug: "ogre", Count: 1}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	stranger := uuid.New()
	if _, err := svc.SaveTable(ctx, dmCaller, w.session.CampaignID, prep.Table{Name: "Nowhere", ChancePct: 100, Visibility: prep.Secret, Entries: []prep.Entry{
		{Weight: 1, Kind: prep.EntryNothing, FactionID: &stranger},
	}}); err == nil || !strings.Contains(err.Error(), "Factions") {
		t.Fatalf("an entry of no Faction of the Campaign = %v", err)
	}
	patrols := func() int {
		t.Helper()
		n := 0
		for range 40 {
			d, _ := tb.dmSays(live.Command{Kind: live.CmdEncounterCheck, TableID: uuid.UUID(road.ID).String(), Mode: prep.ModeForce})
			if d.View.Checks[0].EntryLabel == "Watch patrol" {
				n++
			}
		}
		return n
	}
	hostile := patrols()
	if _, err := w.pool.Exec(ctx, "UPDATE campaign.factions SET score = 80 WHERE id = $1", watch); err != nil {
		t.Fatal(err)
	}
	allied := patrols()
	// Two draws in three when Hostile, one in six when Allied: forty of each are far apart.
	if hostile < allied+8 {
		t.Fatalf("%d patrols of 40 when Hostile, %d when Allied", hostile, allied)
	}
}
