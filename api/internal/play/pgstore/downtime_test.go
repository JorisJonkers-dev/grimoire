package pgstore_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/downtime"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/shops"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

// The DM gives downtime days and keeps the Recipes; a Character's Player spends the days crafting,
// working, training and researching between Sessions. Crafting takes ingredients and coin from the
// Character's own Inventory and puts what it makes there, and the Game Clock moves on by the days
// nobody had yet lived through.
func TestDowntimeCraftingAndTheClock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	members := pgstore.CampaignMembers{Store: campaignpg.New(tb.pool)}
	s := &app.Downtimes{Store: pgstore.New(tb.pool), Members: members, Now: time.Now}
	inventories := &app.Inventories{Store: pgstore.New(tb.pool), Members: members, Roll: func(n, _ int) int { return n }}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := tb.pool.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	hero := func(owner uuid.UUID, name string) uuid.UUID {
		t.Helper()
		id := uuid.New()
		exec(`WITH hero AS (INSERT INTO campaign.account_characters (id, owner_subject, name, ruleset, species_slug, class_slug, background_slug, created_at, updated_at)
				SELECT $1, m.auth_subject, $4, 'srd-2024', 'human', 'fighter', 'soldier', now(), now() FROM campaign.members m WHERE m.id = $3)
			INSERT INTO campaign.characters (character_id, id, campaign_id, owner_member_id, name, ruleset, species_slug, class_slug, background_slug,
			ability_method, hp_max, hp_current, level) VALUES ($1, $1, $2, $3, $4, 'srd-2024', 'human', 'fighter', 'soldier', 'standard-array', 20, 20, 4)`, id, tb.campaign, owner, name)
		return id
	}
	aria, brom := hero(tb.playerID, "Aria"), hero(tb.dmID, "Brom")
	exec(`INSERT INTO compendium.documents (key, title, ruleset_year, precedence, license, attribution, url) VALUES ('downtime-doc', 'Downtime', 2024, 1, 'CC-BY-4.0', 'a', 'https://a') ON CONFLICT DO NOTHING`)
	for _, slug := range []string{"healing-herb", "vial", "herbalism-kit", "potion-of-healing"} {
		exec(`INSERT INTO compendium.items (document_id, slug, name, description, category, cost_gp, weight_lb, magic, requires_attunement)
			SELECT id, $1, $1, '', 'gear', 1, 0.1, false, false FROM compendium.documents WHERE key = 'downtime-doc' ON CONFLICT DO NOTHING`, slug)
	}
	for _, c := range []struct {
		who uuid.UUID
		as  string
	}{{aria, "player"}, {brom, "dm"}} {
		cl := player
		if c.as == "dm" {
			cl = dm
		}
		if _, err := inventories.View(ctx, cl, tb.campaign, c.who); err != nil {
			t.Fatal(err)
		}
	}
	stack := func(character uuid.UUID, slug string, n int) {
		t.Helper()
		exec(`INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, identified, attuned, created_at)
			SELECT gen_random_uuid(), id, $2, $3, true, false, now() FROM campaign.containers WHERE character_id = $1`, character, slug, n)
	}
	coins := func(character uuid.UUID, coin string, n int) {
		t.Helper()
		exec(`INSERT INTO campaign.container_coins (container_id, coin, amount) SELECT id, $2, $3 FROM campaign.containers WHERE character_id = $1
			ON CONFLICT (container_id, coin) DO UPDATE SET amount = excluded.amount`, character, coin, n)
	}
	stack(aria, "healing-herb", 3)
	stack(aria, "vial", 1)
	// Her kit is a thing of its own, with a name: an Item Instance, not a stack.
	exec(`INSERT INTO campaign.item_instances (id, container_id, item_slug, custom_name, quantity, identified, attuned, created_at)
		SELECT gen_random_uuid(), id, 'herbalism-kit', 'Old kit', 1, true, false, now() FROM campaign.containers WHERE character_id = $1`, aria)
	coins(aria, "gp", 30)
	// state is what the Campaign's downtime and a Character's Inventory hold now.
	type state struct {
		days, spent  map[string]int
		clock        int
		advanced     int
		items, purse map[string]int
		kits         int
		log          []string
	}
	read := func(character uuid.UUID) state {
		t.Helper()
		v, err := s.View(ctx, dm, tb.campaign)
		if err != nil {
			t.Fatal(err)
		}
		st := state{days: map[string]int{}, spent: map[string]int{}, clock: v.Downtime.Clock.Day*1440 + v.Downtime.Clock.Minute, advanced: v.Downtime.Advanced, items: map[string]int{}, purse: map[string]int{}}
		for _, ch := range v.Downtime.Characters {
			st.days[ch.Name], st.spent[ch.Name] = ch.Days, ch.Spent
		}
		for _, e := range v.Downtime.Log {
			st.log = append(st.log, e.Name+" "+e.Activity+" "+e.Detail)
		}
		inv, err := inventories.View(ctx, dm, tb.campaign, character)
		if err != nil {
			t.Fatal(err)
		}
		for slug, n := range inv.Mine.Items {
			st.items[slug] = n
		}
		for coin, n := range inv.Mine.Coins {
			st.purse[coin] = n
		}
		for _, in := range inv.Mine.Instances {
			if in.Slug == "herbalism-kit" {
				st.kits++
			}
		}
		return st
	}
	var rule *apperr.RuleError
	refusedWith := func(err error, want string) bool { return errors.As(err, &rule) && strings.Contains(rule.Reason, want) }

	// Who sees what, and whose days are theirs to spend.
	if v, err := s.View(ctx, player, tb.campaign); err != nil || v.DM || len(v.Downtime.Characters) != 2 || len(v.Mine) != 1 || v.Mine[0] != aria {
		t.Fatalf("a Player's view = %+v %v", v, err)
	}
	if v, err := s.View(ctx, dm, tb.campaign); err != nil || !v.DM || len(v.Mine) != 2 {
		t.Fatalf("the DM's view = %+v %v", v, err)
	}

	// The DM keeps the Recipes.
	potion := downtime.Recipe{
		Name: "  Potion of Healing ", Makes: "potion-of-healing", Quantity: 2, Tool: "herbalism-kit", Days: 3, CostCP: 2500,
		Ingredients: []downtime.Ingredient{{Item: "healing-herb", Count: 2}, {Item: "vial", Count: 1}},
	}
	made, err := s.AddRecipe(ctx, dm, tb.campaign, potion)
	if err != nil || made.Recipe.Name != "Potion of Healing" {
		t.Fatalf("add recipe = %+v %v", made, err)
	}
	spoon, err := s.AddRecipe(ctx, dm, tb.campaign, downtime.Recipe{Name: "Whittling", Makes: "wooden-spoon", Quantity: 1, Days: 1})
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := s.View(ctx, player, tb.campaign); len(v.Downtime.Recipes) != 2 || v.Downtime.Recipes[0].ID != made.ID || len(v.Downtime.Recipes[0].Recipe.Ingredients) != 2 ||
		v.Downtime.Recipes[0].Recipe.Ingredients[0] != (downtime.Ingredient{Item: "healing-herb", Count: 2}) || v.Downtime.Recipes[0].Recipe.Tool != "herbalism-kit" || v.Downtime.Recipes[1].Recipe.Tool != "" {
		t.Fatalf("the Recipes = %+v", v.Downtime.Recipes)
	}
	if _, err := s.AddRecipe(ctx, dm, tb.campaign, downtime.Recipe{Name: " ", Makes: "x", Quantity: 1, Days: 1}); !refusedWith(err, "needs a name") {
		t.Errorf("a Recipe with no name: %v", err)
	}

	// The DM gives downtime: to everyone, which starts a new downtime, or to one Character.
	for _, days := range []int{0, 3651} {
		if _, err := s.Grant(ctx, dm, tb.campaign, nil, days); !refusedWith(err, "1 to 3650") {
			t.Errorf("a grant of %d days: %v", days, err)
		}
	}
	if _, err := s.Grant(ctx, dm, tb.campaign, nil, 5); err != nil {
		t.Fatal(err)
	}
	if v, err := s.Grant(ctx, dm, tb.campaign, &brom, 2); err != nil || len(v.Downtime.Characters) != 2 {
		t.Fatalf("a grant to Brom = %+v %v", v, err)
	}
	before := read(aria)
	if before.days["Aria"] != 5 || before.days["Brom"] != 7 || before.advanced != 0 {
		t.Fatalf("after the grants = %+v", before)
	}

	// Aria crafts: three days, 25 gp, two herbs and the vial; the kit stays, and the clock moves three days on.
	if _, err := s.Spend(ctx, player, tb.campaign, aria, app.DowntimeActivity{Kind: domain.DowntimeCraft, Recipe: made.ID}); err != nil {
		t.Fatal(err)
	}
	got := read(aria)
	if got.days["Aria"] != 2 || got.spent["Aria"] != 3 || got.clock-before.clock != 3*1440 || got.advanced != 3 {
		t.Fatalf("after crafting: %+v", got)
	}
	if got.items["healing-herb"] != 1 || got.items["vial"] != 0 || got.items["potion-of-healing"] != 2 || got.kits != 1 || got.items["herbalism-kit"] != 0 || got.purse["gp"] != 5 || len(got.purse) != 1 {
		t.Fatalf("her Inventory after crafting: %+v", got)
	}
	if len(got.log) != 1 || got.log[0] != "Aria craft Potion of Healing" {
		t.Fatalf("the log = %q", got.log)
	}
	// Brom works two of the same days: he is paid, and the clock stays; three more move it two days on.
	if _, err := s.Spend(ctx, dm, tb.campaign, brom, app.DowntimeActivity{Kind: domain.DowntimeWork, Days: 2}); err != nil {
		t.Fatal(err)
	}
	if b := read(brom); b.days["Brom"] != 5 || b.spent["Brom"] != 2 || b.clock != got.clock || b.advanced != 3 || b.purse["gp"] != 2 {
		t.Fatalf("after two days of work: %+v", b)
	}
	if _, err := s.Spend(ctx, dm, tb.campaign, brom, app.DowntimeActivity{Kind: domain.DowntimeWork, Days: 3}); err != nil {
		t.Fatal(err)
	}
	if b := read(brom); b.days["Brom"] != 2 || b.clock-got.clock != 2*1440 || b.advanced != 5 || b.purse["gp"] != 5 {
		t.Fatalf("after three more: %+v", b)
	}

	// Training and research cost coin by the day, and need a subject. 20 gp 5 sp less 10 gp leaves 1050
	// copper, given back in the fewest coins.
	train := app.DowntimeActivity{Kind: domain.DowntimeTrain, Days: 2, Subject: "  Thieves' tools "}
	if _, err := s.Spend(ctx, player, tb.campaign, aria, train); !refusedWith(err, "It costs 10 gp, which you cannot pay.") {
		t.Errorf("training she cannot pay for: %v", err)
	}
	coins(aria, "gp", 20)
	coins(aria, "sp", 5)
	if _, err := s.Spend(ctx, player, tb.campaign, aria, train); err != nil {
		t.Fatal(err)
	}
	if a := read(aria); a.days["Aria"] != 0 || a.spent["Aria"] != 5 || shops.Worth(a.purse) != 1050 || a.advanced != 5 || a.log[0] != "Aria train Thieves' tools" {
		t.Fatalf("after training: %+v", a)
	}
	research := app.DowntimeActivity{Kind: domain.DowntimeResearch, Days: 1, Subject: "The Ashen Hand"}
	if _, err := s.Spend(ctx, player, tb.campaign, aria, research); !refusedWith(err, "It takes 1 downtime days; you have 0.") {
		t.Errorf("research with no days left: %v", err)
	}
	if _, err := s.Spend(ctx, dm, tb.campaign, brom, research); err != nil {
		t.Fatal(err)
	}
	if b := read(brom); b.days["Brom"] != 1 || b.purse["gp"] != 4 || b.log[0] != "Brom research The Ashen Hand" {
		t.Fatalf("after research: %+v", b)
	}
	// Fifty days of training in one thing is the end of it; another subject is still open.
	exec(`INSERT INTO campaign.downtime_log (id, campaign_id, character_id, activity, detail, days, created_at) VALUES (gen_random_uuid(), $1, $2, 'train', 'Smith''s tools', 49, now())`, tb.campaign, brom)
	coins(brom, "gp", 100)
	smith := app.DowntimeActivity{Kind: domain.DowntimeTrain, Days: 1, Subject: "Smith's tools"}
	if _, err := s.Spend(ctx, dm, tb.campaign, brom, smith); err != nil {
		t.Fatalf("the fiftieth day: %v", err)
	}
	exec(`UPDATE campaign.characters SET downtime_days = 9 WHERE id = $1`, brom)
	if _, err := s.Spend(ctx, dm, tb.campaign, brom, smith); !refusedWith(err, "Brom has finished training in Smith's tools") {
		t.Errorf("a fifty-first day: %v", err)
	}
	if _, err := s.Spend(ctx, dm, tb.campaign, brom, app.DowntimeActivity{Kind: domain.DowntimeTrain, Days: 1, Subject: "Elvish"}); err != nil {
		t.Errorf("training in something else: %v", err)
	}

	// Refusals.
	for name, a := range map[string]app.DowntimeActivity{
		"no subject":       {Kind: domain.DowntimeResearch, Days: 1, Subject: "  "},
		"a long subject":   {Kind: domain.DowntimeTrain, Days: 1, Subject: strings.Repeat("a", 81)},
		"no such activity": {Kind: "carouse", Days: 1},
		"no days of work":  {Kind: domain.DowntimeWork, Days: 0},
		"too few herbs":    {Kind: domain.DowntimeCraft, Recipe: made.ID},
	} {
		if _, err := s.Spend(ctx, dm, tb.campaign, brom, a); !errors.As(err, &rule) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := s.Spend(ctx, dm, tb.campaign, brom, app.DowntimeActivity{Kind: domain.DowntimeResearch, Days: 1, Subject: strings.Repeat("a", 80)}); err != nil {
		t.Errorf("the longest subject: %v", err)
	}
	var elsewhere uuid.UUID
	if err := tb.pool.QueryRow(ctx, `INSERT INTO campaign.campaigns (name, ruleset_pref, created_by, created_at, updated_at) VALUES ('Elsewhere', 'srd-2024', 'dm', now(), now()) RETURNING id`).Scan(&elsewhere); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO campaign.members (campaign_id, auth_subject, display_name, role) VALUES ($1, 'dm', 'Joris', 'dm')`, elsewhere)
	work := app.DowntimeActivity{Kind: domain.DowntimeWork, Days: 1}
	forbidden := map[string]error{}
	_, forbidden["spend another's days"] = s.Spend(ctx, player, tb.campaign, brom, work)
	_, forbidden["grant"] = s.Grant(ctx, player, tb.campaign, nil, 1)
	_, forbidden["add a Recipe"] = s.AddRecipe(ctx, player, tb.campaign, potion)
	forbidden["remove a Recipe"] = s.RemoveRecipe(ctx, player, tb.campaign, made.ID)
	for name, err := range forbidden {
		if !errors.Is(err, apperr.ErrForbidden) {
			t.Errorf("a Player may %s: %v", name, err)
		}
	}
	missing := map[string]error{}
	_, missing["a stranger views"] = s.View(ctx, stranger, tb.campaign)
	_, missing["a stranger spends"] = s.Spend(ctx, stranger, tb.campaign, aria, work)
	_, missing["a stranger grants"] = s.Grant(ctx, stranger, tb.campaign, nil, 1)
	_, missing["no such Character"] = s.Spend(ctx, dm, tb.campaign, uuid.New(), work)
	_, missing["no such Recipe"] = s.Spend(ctx, dm, tb.campaign, brom, app.DowntimeActivity{Kind: domain.DowntimeCraft, Recipe: uuid.New()})
	_, missing["a grant to nobody"] = s.Grant(ctx, dm, tb.campaign, func() *uuid.UUID { id := uuid.New(); return &id }(), 1)
	_, missing["a grant through another Campaign"] = s.Grant(ctx, dm, elsewhere, &brom, 1)
	_, missing["a Character through another Campaign"] = s.Spend(ctx, dm, elsewhere, brom, work)
	missing["the removal of no Recipe"] = s.RemoveRecipe(ctx, dm, tb.campaign, uuid.New())
	missing["a Recipe removed through another Campaign"] = s.RemoveRecipe(ctx, dm, elsewhere, made.ID)
	for name, err := range missing {
		if !errors.Is(err, apperr.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if there, err := s.View(ctx, dm, elsewhere); err != nil || len(there.Downtime.Recipes) != 0 || len(there.Downtime.Characters) != 0 || len(there.Downtime.Log) != 0 {
		t.Fatalf("the other Campaign = %+v %v", there, err)
	}

	// A new downtime for everyone starts the count again: the first day spent moves the clock.
	if _, err := s.Grant(ctx, dm, tb.campaign, nil, 1); err != nil {
		t.Fatal(err)
	}
	fresh := read(aria)
	if fresh.days["Aria"] != 1 || fresh.spent["Aria"] != 0 || fresh.spent["Brom"] != 0 || fresh.advanced != 0 {
		t.Fatalf("a new downtime = %+v", fresh)
	}
	if _, err := s.Spend(ctx, player, tb.campaign, aria, app.DowntimeActivity{Kind: domain.DowntimeCraft, Recipe: spoon.ID}); err != nil {
		t.Fatal(err)
	}
	if a := read(aria); a.clock-fresh.clock != 1440 || a.items["wooden-spoon"] != 1 || a.advanced != 1 {
		t.Fatalf("after whittling: %+v", a)
	}

	// Every operation reports a database fault, and a craft that fails takes nothing.
	exec(`UPDATE campaign.characters SET downtime_days = 400, downtime_spent = 0 WHERE id = $1`, aria)
	coins(aria, "gp", 1000)
	stack(aria, "vial", 40)
	exec(`UPDATE campaign.item_instances SET quantity = 90 WHERE item_slug = 'healing-herb'`)
	svc := func(f *pgtest.Faulty) *app.Downtimes {
		return &app.Downtimes{Store: pgstore.NewFaulty(tb.pool, f), Members: members, Now: time.Now}
	}
	for name, op := range map[string]func(f *pgtest.Faulty) error{
		"view":  func(f *pgtest.Faulty) error { _, err := svc(f).View(ctx, player, tb.campaign); return err },
		"grant": func(f *pgtest.Faulty) error { _, err := svc(f).Grant(ctx, dm, tb.campaign, &brom, 1); return err },
		"grant to all": func(f *pgtest.Faulty) error {
			_, err := svc(f).Grant(ctx, dm, elsewhere, nil, 1)
			return err
		},
		"add": func(f *pgtest.Faulty) error { _, err := svc(f).AddRecipe(ctx, dm, elsewhere, potion); return err },
		"remove": func(f *pgtest.Faulty) error {
			err := svc(f).RemoveRecipe(ctx, dm, tb.campaign, uuid.New())
			if errors.Is(err, apperr.ErrNotFound) {
				return nil
			}
			return err
		},
		"train": func(f *pgtest.Faulty) error {
			_, err := svc(f).Spend(ctx, player, tb.campaign, aria, app.DowntimeActivity{Kind: domain.DowntimeTrain, Days: 1, Subject: "Dwarvish"})
			return err
		},
		"craft": func(f *pgtest.Faulty) error {
			was := read(aria)
			_, err := svc(f).Spend(ctx, player, tb.campaign, aria, app.DowntimeActivity{Kind: domain.DowntimeCraft, Recipe: made.ID})
			// A fault may also come after the change was kept, when it is read back: the craft is then whole.
			now := read(aria)
			nothing := now.days["Aria"] == was.days["Aria"] && shops.Worth(now.purse) == shops.Worth(was.purse) && now.items["healing-herb"] == was.items["healing-herb"] &&
				now.items["potion-of-healing"] == was.items["potion-of-healing"] && now.clock == was.clock && len(now.log) == len(was.log)
			whole := now.days["Aria"] == was.days["Aria"]-3 && shops.Worth(now.purse) == shops.Worth(was.purse)-2500 && now.items["healing-herb"] == was.items["healing-herb"]-2 &&
				now.items["vial"] == was.items["vial"]-1 && now.items["potion-of-healing"] == was.items["potion-of-healing"]+2 && now.spent["Aria"] == was.spent["Aria"]+3
			if !nothing && !whole {
				t.Fatalf("a craft left half of itself behind: %+v, was %+v", now, was)
			}
			if err == nil && !whole {
				t.Fatalf("a craft that worked did not do all of it: %+v, was %+v", now, was)
			}
			return err
		},
	} {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := op(f)
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
	for _, r := range mustRecipes(t, s, elsewhere) {
		if len(r.Recipe.Ingredients) != 2 {
			t.Fatalf("a Recipe kept with %d of its 2 ingredients", len(r.Recipe.Ingredients))
		}
	}

	// A Campaign keeps two hundred Recipes and no more.
	exec(`INSERT INTO campaign.recipes (id, campaign_id, name, item_slug, quantity, days, cost_cp, created_at)
		SELECT gen_random_uuid(), $1, 'Filler ' || n, 'rope', 1, 1, 0, now() FROM generate_series(1, $2::int) n`, tb.campaign, app.MaxRecipes-3)
	if _, err := s.AddRecipe(ctx, dm, tb.campaign, potion); err != nil {
		t.Fatalf("the two hundredth Recipe: %v", err)
	}
	if _, err := s.AddRecipe(ctx, dm, tb.campaign, potion); !refusedWith(err, "up to 200 Recipes") {
		t.Errorf("the two hundred and first: %v", err)
	}
	exec(`DELETE FROM campaign.recipes WHERE campaign_id = $1 AND id NOT IN ($2, $3)`, tb.campaign, made.ID, spoon.ID)

	// While a Session is live, downtime waits.
	if _, err := sessions(tb, pgstore.New(tb.pool), &closed{}).Start(ctx, dm, tb.campaign); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Spend(ctx, player, tb.campaign, aria, work); !refusedWith(err, "a Session is live") {
		t.Errorf("downtime in a live Session: %v", err)
	}
	if err := s.RemoveRecipe(ctx, dm, tb.campaign, spoon.ID); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.View(ctx, dm, tb.campaign); len(v.Downtime.Recipes) != 1 || v.Downtime.Recipes[0].ID != made.ID {
		t.Fatalf("after removing a Recipe = %+v", v.Downtime.Recipes)
	}
}

func mustRecipes(t *testing.T, s *app.Downtimes, campaign uuid.UUID) []domain.CampaignRecipe {
	t.Helper()
	v, err := s.View(context.Background(), dm, campaign)
	if err != nil {
		t.Fatal(err)
	}
	return v.Downtime.Recipes
}
