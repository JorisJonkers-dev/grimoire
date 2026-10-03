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
	libraryapp "github.com/JorisJonkers-dev/grimoire/api/internal/library/app"
	librarydomain "github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	librarypg "github.com/JorisJonkers-dev/grimoire/api/internal/library/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/features"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/itembuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
)

// restingParty gives Aria (a level 3 fighter with Constitution 14) and Brom (a level 6 barbarian who
// has raged three times) tokens on the board, and hurts Aria.
func restingParty(t *testing.T) (world, *table, map[string]string) {
	t.Helper()
	ctx := context.Background()
	w, tb := ambushTable(t)
	stocked(t, w)
	class := func(slug string, die int) snapshot.Class {
		return snapshot.Class{Entry: snapshot.Entry{Document: "srd-2024", Slug: slug, Name: strings.ToUpper(slug[:1]) + slug[1:], Description: slug + "."}, HitDie: die, SavingThrows: []string{}, Features: nil}
	}
	if _, err := comppg.New(w.pool).Import(ctx, snapshot.Snapshot{
		Documents: []snapshot.Document{{Key: "srd-2024", Title: "SRD 5.2", RulesetYear: 2024, Precedence: 20, License: "CC-BY-4.0", Attribution: "a", URL: "https://a"}},
		Classes:   []snapshot.Class{class("fighter", 10), class("barbarian", 12)},
	}, "rest"); err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, c := range []struct {
		name, class string
		level, con  int
	}{{"Aria", "fighter", 3, 14}, {"Brom", "barbarian", 6, 12}} {
		var id uuid.UUID
		if err := w.pool.QueryRow(ctx, `UPDATE campaign.characters SET class_slug = $2, level = $3 WHERE campaign_id = $1 AND name = $4 RETURNING id`,
			w.session.CampaignID, c.class, c.level, c.name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if _, err := w.pool.Exec(ctx, "INSERT INTO campaign.character_abilities (character_id, ability, base, bonus) VALUES ($1, 'constitution', $2, 0)", id, c.con); err != nil {
			t.Fatal(err)
		}
		ids[c.name] = id.String()
	}
	if _, err := w.pool.Exec(ctx, "INSERT INTO campaign.character_resources (character_id, resource_slug, used) VALUES ($1, 'rage', 3)", ids["Brom"]); err != nil {
		t.Fatal(err)
	}
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: ids["Aria"], Q: 0})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: ids["Brom"], Q: 2})
	for _, tv := range d.View.Tokens {
		if tv.Q == 0 {
			ids["aria-token"] = tv.ID
		}
	}
	tb.dmSays(live.Command{Kind: live.CmdAdjustHP, TokenID: ids["aria-token"], HPDelta: -8})
	return w, tb, ids
}

func tokenByID(t *testing.T, v *live.View, id string) live.TokenView {
	t.Helper()
	for _, tv := range v.Tokens {
		if tv.ID == id {
			return tv
		}
	}
	t.Fatalf("no token %s", id)
	return live.TokenView{}
}

func resterNamed(t *testing.T, r *live.RestView, character string) live.ResterView {
	t.Helper()
	for _, x := range r.Resters {
		if x.CharacterID == character {
			return x
		}
	}
	t.Fatalf("no %s in %+v", character, r.Resters)
	return live.ResterView{}
}

func TestAShortRestSpendsHitDice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb, ids := restingParty(t)
	refuse := func(want string, sub *live.Subscriber, cmd live.Command) {
		t.Helper()
		w.hub.Submit(sub, cmd)
		if u := next(t, sub); u.Kind != live.UpdRejected || !strings.Contains(strings.ToLower(u.Reason), want) {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	refuse("nothing to agree to", tb.player, live.Command{Kind: live.CmdAgreeRest})
	refuse("a rest is short or long", tb.player, live.Command{Kind: live.CmdProposeRest, Rest: "nap"})
	p := tb.playerSays(live.Command{Kind: live.CmdProposeRest, Rest: live.RestShort})
	rest := p.View.Rest
	if rest == nil || rest.Kind != live.RestShort || rest.Status != "proposed" || rest.ProposedBy != w.player.ID.String() || len(rest.Agreed) != 1 || len(rest.Waiting) != 0 || !rest.WaitingOnDM {
		t.Fatalf("the proposal = %+v", rest)
	}
	if a := resterNamed(t, rest, ids["Aria"]); a.Name != "Aria" || a.HitDie != "d10" || a.HitDiceLeft != 3 {
		t.Fatalf("Aria rests with her Hit Dice = %+v", a)
	}
	refuse("already", tb.dm, live.Command{Kind: live.CmdProposeRest, Rest: live.RestLong})
	refuse("once the rest has begun", tb.player, live.Command{Kind: live.CmdSpendHitDie, TokenID: ids["aria-token"]})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdAgreeRest})
	if d.View.Rest == nil || d.View.Rest.Status != "resting" {
		t.Fatalf("the DM's word starts the rest = %+v", d.View.Rest)
	}
	p = tb.playerSays(live.Command{Kind: live.CmdSpendHitDie, TokenID: ids["aria-token"]})
	if p.View == nil {
		t.Fatalf("spend = %+v", p)
	}
	aria := resterNamed(t, p.View.Rest, ids["Aria"])
	if aria.HitDiceLeft != 2 || aria.RollID == "" {
		t.Fatalf("spending a Hit Die opens its roll = %+v", aria)
	}
	refuse("still rolling", tb.player, live.Command{Kind: live.CmdSpendHitDie, TokenID: ids["aria-token"]})
	tb.fill(aria.RollID, w.player, 6)
	d, p = next(t, tb.dm), next(t, tb.player)
	if hp := tokenByID(t, p.View, ids["aria-token"]).HP; hp == nil || *hp != 4+6+2 {
		t.Fatalf("a Hit Die heals 1d10 + Constitution = %v", *hp)
	}
	if resterNamed(t, p.View.Rest, ids["Aria"]).RollID != "" {
		t.Fatal("the roll is done")
	}
	refuse("only the dm", tb.player, live.Command{Kind: live.CmdFinishRest})
	d, p = tb.dmSays(live.Command{Kind: live.CmdFinishRest})
	if d.View.Rest != nil || p.View.Rest != nil {
		t.Fatalf("the rest is over = %+v", d.View.Rest)
	}
	var spent, hp, rage int
	var ready bool
	if err := w.pool.QueryRow(ctx, `SELECT c.hit_dice_spent, c.hp_current, c.level_up_ready, r.used FROM campaign.characters c, campaign.character_resources r
		WHERE c.id = $1 AND r.character_id = $2`, ids["Aria"], ids["Brom"]).Scan(&spent, &hp, &ready, &rage); err != nil {
		t.Fatal(err)
	}
	if spent != 1 || hp != 10 || ready || rage != 2 {
		t.Fatalf("after a Short Rest: spent %d, hp %d, ready %v, rage used %d", spent, hp, ready, rage)
	}
}

func TestALongRestCanBeInterruptedAndCostsRations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb, ids := restingParty(t)
	if _, err := w.pool.Exec(ctx, "UPDATE campaign.campaigns SET rest_supplies = true WHERE id = $1", w.session.CampaignID); err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx, "UPDATE campaign.characters SET hit_dice_spent = 2 WHERE id = $1", ids["Aria"]); err != nil {
		t.Fatal(err)
	}
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdProposeRest, Rest: live.RestLong})
	if u := next(t, tb.dm); u.Kind != live.UpdRejected || u.Reason != "A Long Rest needs a day of Rations for each of Aria, Brom." {
		t.Fatalf("no rations = %+v", u)
	}
	stash := func(n int) {
		t.Helper()
		var id uuid.UUID
		if err := w.pool.QueryRow(ctx, "SELECT id FROM campaign.containers WHERE campaign_id = $1 AND kind = 'party_stash'", w.session.CampaignID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if _, err := w.pool.Exec(ctx, `INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, identified, attuned, created_at)
			VALUES ($1, $2, 'rations', $3, true, false, now())`, uuid.New(), id, n); err != nil {
			t.Fatal(err)
		}
		w.hub.Close(w.session.ID)
		tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
		tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	}
	stash(2)
	proposed, _ := tb.dmSays(live.Command{Kind: live.CmdProposeRest, Rest: live.RestLong})
	day := proposed.View.GameDay
	if proposed.View.Rest.Status != "proposed" || len(proposed.View.Rest.Waiting) != 1 || proposed.View.Rest.Waiting[0] != w.player.ID.String() || proposed.View.Rest.WaitingOnDM {
		t.Fatalf("the DM proposes and waits on the player = %+v", proposed.View.Rest)
	}
	tb.playerSays(live.Command{Kind: live.CmdAgreeRest})
	d := look(t, w, tb.dm)
	if d.Rest.Status != "resting" || len(containerNamed(t, d, "Party Stash").Items) != 0 {
		t.Fatalf("agreed, the party eats its rations and beds down = %+v %+v", d.Rest, containerNamed(t, d, "Party Stash"))
	}
	d2, p2 := tb.dmSays(live.Command{Kind: live.CmdInterruptRest})
	if d2.View.Rest != nil || p2.View.Rest != nil || d2.View.GameDay != day || *tokenByID(t, d2.View, ids["aria-token"]).HP != 4 {
		t.Fatalf("an interrupted rest gives nothing = %+v", d2.View)
	}
	wand := uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO compendium.documents (key, title, ruleset_year, precedence, license, attribution, url) VALUES ('rest-doc', 'Rest', 2024, 1, 'CC-BY-4.0', 'a', 'https://a') ON CONFLICT DO NOTHING`, nil},
		{`INSERT INTO compendium.items (document_id, slug, name, description, category, cost_gp, weight_lb, magic, requires_attunement)
			SELECT id, 'rest-wand', 'Rest Wand', '', 'wand', 1, 1, true, false FROM compendium.documents WHERE key = 'rest-doc' ON CONFLICT DO NOTHING`, nil},
		{`INSERT INTO compendium.item_charges (item_slug, max_charges, regain_dice, regain_faces, regain_bonus, recharge_on) VALUES ('rest-wand', 7, 1, 6, 1, 'dawn') ON CONFLICT DO NOTHING`, nil},
		{`INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, charges, identified, attuned, created_at)
			SELECT $1, id, 'rest-wand', 1, 0, true, false, now() FROM campaign.containers WHERE character_id = $2`, []any{wand, ids["Aria"]}},
	} {
		if _, err := w.pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	bow := ashwoodIn(t, w, ids["Aria"])
	stash(2)
	tb.dmSays(live.Command{Kind: live.CmdProposeRest, Rest: live.RestLong})
	tb.playerSays(live.Command{Kind: live.CmdAgreeRest})
	d2, _ = tb.dmSays(live.Command{Kind: live.CmdFinishRest})
	var charges int
	if err := w.pool.QueryRow(ctx, "SELECT charges FROM campaign.item_instances WHERE id = $1", wand).Scan(&charges); err != nil || charges < 2 || charges > 7 {
		t.Fatalf("the wand regains 1d6+1 at dawn: %d %v", charges, err)
	}
	if err := w.pool.QueryRow(ctx, "SELECT charges FROM campaign.item_instances WHERE id = $1", bow).Scan(&charges); err != nil || charges < 1 || charges > 3 {
		t.Fatalf("the homebrew Ashwood Longbow regains 1d4 of its 3 charges at dawn: %d %v", charges, err)
	}
	if d2.View.Rest != nil || d2.View.GameDay != day+1 || *tokenByID(t, d2.View, ids["aria-token"]).HP != 12 {
		t.Fatalf("a finished Long Rest heals and moves the day on = %+v", d2.View)
	}
	var spent, hp, rage int
	var ready bool
	if err := w.pool.QueryRow(ctx, `SELECT c.hit_dice_spent, c.hp_current, c.level_up_ready, r.used FROM campaign.characters c, campaign.character_resources r
		WHERE c.id = $1 AND r.character_id = $2`, ids["Aria"], ids["Brom"]).Scan(&spent, &hp, &ready, &rage); err != nil {
		t.Fatal(err)
	}
	if spent != 0 || hp != 10 || !ready || rage != 0 {
		t.Fatalf("after a Long Rest: spent %d, hp %d, ready %v, rage used %d", spent, hp, ready, rage)
	}
}

type failingRest struct {
	live.Store
	info, features, supplies, load bool
}

func (f failingRest) RestInfo(ctx context.Context, campaign uuid.UUID, ids []uuid.UUID) ([]domain.Rester, error) {
	if f.info {
		return nil, context.Canceled
	}
	return f.Store.RestInfo(ctx, campaign, ids)
}

func (f failingRest) Features(ctx context.Context) (features.Catalog, error) {
	if f.features {
		return features.Catalog{}, context.Canceled
	}
	return f.Store.Features(ctx)
}

func (f failingRest) RestSupplies(ctx context.Context, campaign uuid.UUID) (bool, error) {
	if f.supplies {
		return false, context.Canceled
	}
	return f.Store.RestSupplies(ctx, campaign)
}

func (f failingRest) LoadRest(ctx context.Context, campaign uuid.UUID, id domain.SessionID) (*domain.Rest, error) {
	if f.load {
		return nil, context.Canceled
	}
	return f.Store.LoadRest(ctx, campaign, id)
}

// A rest survives the runtime restarting, and refuses what makes no sense.
func TestARestKeepsItsShapeAndItsRules(t *testing.T) {
	t.Parallel()
	w, tb, ids := restingParty(t)
	refuse := func(want string, sub *live.Subscriber, cmd live.Command) {
		t.Helper()
		w.hub.Submit(sub, cmd)
		if u := next(t, sub); u.Kind != live.UpdRejected || !strings.Contains(strings.ToLower(u.Reason), want) {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	refuse("no rest is under way", tb.dm, live.Command{Kind: live.CmdInterruptRest})
	refuse("no rest is under way", tb.dm, live.Command{Kind: live.CmdFinishRest})
	tb.playerSays(live.Command{Kind: live.CmdProposeRest, Rest: live.RestShort})
	refuse("already agreed", tb.player, live.Command{Kind: live.CmdAgreeRest})
	refuse("finish or interrupt", tb.dm, live.Command{Kind: live.CmdRest, Rest: live.RestShort})
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	if r := look(t, w, tb.player).Rest; r == nil || r.Status != "proposed" || len(r.Resters) != 2 || len(r.Agreed) != 1 {
		t.Fatalf("the proposal survives a restart = %+v", r)
	}
	tb.dmSays(live.Command{Kind: live.CmdAgreeRest})
	refuse("not resting", tb.player, live.Command{Kind: live.CmdSpendHitDie, TokenID: uuid.NewString()})
	p := tb.playerSays(live.Command{Kind: live.CmdSpendHitDie, TokenID: ids["aria-token"]})
	roll := resterNamed(t, p.View.Rest, ids["Aria"]).RollID
	refuse("still rolling", tb.dm, live.Command{Kind: live.CmdFinishRest})
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	if r := look(t, w, tb.player).Rest; r == nil || resterNamed(t, r, ids["Aria"]).RollID != roll || resterNamed(t, r, ids["Aria"]).HitDiceLeft != 2 {
		t.Fatalf("the Hit Die roll survives a restart = %+v", r)
	}
	tb.fill(roll, w.player, 1)
	next(t, tb.dm)
	next(t, tb.player)
	for range 2 {
		tb.playerSays(live.Command{Kind: live.CmdSpendHitDie, TokenID: ids["aria-token"]})
		tb.fill(resterNamed(t, look(t, w, tb.dm).Rest, ids["Aria"]).RollID, w.player, 1)
		next(t, tb.dm)
		next(t, tb.player)
	}
	refuse("no hit dice left", tb.player, live.Command{Kind: live.CmdSpendHitDie, TokenID: ids["aria-token"]})
	w.hub.Store = failingRest{Store: pgstore.New(w.pool), features: true}
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	refuse("could not be finished", tb.dm, live.Command{Kind: live.CmdFinishRest})
	tb.dmSays(live.Command{Kind: live.CmdInterruptRest})
	w.hub.Store = failingRest{Store: pgstore.New(w.pool), info: true}
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	refuse("could not be read", tb.dm, live.Command{Kind: live.CmdProposeRest, Rest: live.RestShort})
	refuse("could not be read", tb.dm, live.Command{Kind: live.CmdRest, Rest: live.RestShort})
	w.hub.Store = failingRest{Store: pgstore.New(w.pool), supplies: true}
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	d, _ := tb.dmSays(live.Command{Kind: live.CmdProposeRest, Rest: live.RestLong})
	if d.View.Rest == nil || d.View.Rest.Status != "proposed" {
		t.Fatalf("an unreadable supplies rule asks for nothing = %+v", d.View.Rest)
	}
	tb.dmSays(live.Command{Kind: live.CmdInterruptRest})
	tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: ids["aria-token"]})
	for _, tv := range look(t, w, tb.dm).Tokens {
		tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: tv.ID})
	}
	refuse("no character is on the board", tb.dm, live.Command{Kind: live.CmdProposeRest, Rest: live.RestShort})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdPlace, Label: "Goblin", TokenKind: domain.TokenEnemy})
	tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: []live.CombatantSetup{{TokenID: d.View.Tokens[0].ID, SpeedFt: 30}}})
	refuse("middle of a fight", tb.dm, live.Command{Kind: live.CmdProposeRest, Rest: live.RestShort})
	w.hub.Store = failingRest{Store: pgstore.New(w.pool), load: true}
	w.hub.Close(w.session.ID)
	if _, err := w.hub.Join(context.Background(), w.session.ID, w.dm, dmCaller, live.AudienceDM); err == nil {
		t.Fatal("an unreadable rest is ignored")
	}
}

// ashwoodIn builds the Ashwood Longbow in the DM's Library, links it into the Campaign and puts a spent
// one in a Character's pack.
func ashwoodIn(t *testing.T, w world, character string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	lib := &libraryapp.Service{Repo: librarypg.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Now: time.Now}
	entry, err := lib.Create(ctx, dmCaller, librarydomain.Draft{Kind: "item", Name: "Ashwood Longbow"})
	if err != nil {
		t.Fatal(err)
	}
	design := itembuild.Design{
		Kind: "weapon", Base: "longbow", Rarity: "rare", Enchantment: 1, WeightLb: 2, ValueGP: 4000, Attunement: &itembuild.Attunement{},
		Charges: &itembuild.Charges{Max: 3, On: "dawn", Dice: 1, Faces: 4},
		Properties: []itembuild.Property{
			{Type: "cantrip", Spell: "light", Name: "Light"}, {Type: "spell", Spell: "hunters-mark", Name: "Hunter's Mark", Level: 1, Cost: 1},
		},
	}
	if _, err := lib.SaveItem(ctx, dmCaller, entry.ID, design); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Link(ctx, dmCaller, w.session.CampaignID, entry.ID); err != nil {
		t.Fatal(err)
	}
	bow := uuid.New()
	if _, err := w.pool.Exec(ctx, `INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, charges, identified, attuned, created_at)
		SELECT $1, id, $3, 1, 0, true, true, now() FROM campaign.containers WHERE character_id = $2`, bow, character, spellbuild.Slug(entry.ID.String())); err != nil {
		t.Fatal(err)
	}
	return bow
}
