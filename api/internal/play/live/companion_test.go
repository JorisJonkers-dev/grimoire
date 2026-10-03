package live_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/rng"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
)

// worth says what creatures are worth in XP. failing makes the worth unknown when it is 1, and the
// count of sharing Companions when it is 2.
type worth struct {
	live.Store
	xp      map[string]int
	failing *int
}

func (w worth) CreatureXP(context.Context, uuid.UUID, string) (int, error) {
	if *w.failing == 1 {
		return 0, errors.New("gone")
	}
	return w.xp["goblin"], nil
}

func (w worth) SharingCompanions(ctx context.Context, campaign uuid.UUID, ids []uuid.UUID) (int, error) {
	if *w.failing == 2 {
		return 0, errors.New("gone")
	}
	return w.Store.SharingCompanions(ctx, campaign, ids)
}

// companyTable has Aria and Brom as Characters of the Campaign, Fang the wolf as Aria's player's
// Companion with a share of the XP, and Bors the hireling, whom the DM runs for his pay alone.
func companyTable(t *testing.T) (world, *table, map[string]string, *int, map[string]int) {
	t.Helper()
	ctx := context.Background()
	w := setup(t)
	stocked(t, w)
	ids := map[string]string{}
	for _, name := range []string{"Aria", "Brom"} {
		var id uuid.UUID
		if err := w.pool.QueryRow(ctx, "SELECT id FROM campaign.characters WHERE campaign_id = $1 AND name = $2", w.session.CampaignID, name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[name+"'s sheet"] = id.String()
	}
	for _, c := range []struct {
		name, kind, slug string
		who              *uuid.UUID
		shares           bool
	}{{"Fang", "companion", "wolf", &w.player.ID, true}, {"Bors", "hireling", "goblin", nil, false}} {
		id := uuid.New()
		if _, err := w.pool.Exec(ctx, `INSERT INTO campaign.companions (id, campaign_id, name, kind, monster_slug, controller_member_id, shares_xp, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, now(), now())`, id, w.session.CampaignID, c.name, c.kind, c.slug, c.who, c.shares); err != nil {
			t.Fatal(err)
		}
		ids[c.name+" the Companion"] = id.String()
	}
	failing, xp := 0, map[string]int{"goblin": 50}
	w.hub.Stats = bestiary{owner: w.player.ID, companions: pgstore.Statblocks{Store: pgstore.New(w.pool)}.Companion}
	w.hub.Store = worth{Store: w.hub.Store, xp: xp, failing: &failing}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	place := func(cmd live.Command) {
		t.Helper()
		before := map[string]bool{}
		for _, tv := range look(t, w, tb.dm).Tokens {
			before[tv.ID] = true
		}
		d, _ := tb.dmSays(cmd)
		for _, tv := range d.View.Tokens {
			if !before[tv.ID] {
				ids[tv.Label] = tv.ID
			}
		}
	}
	place(live.Command{Kind: live.CmdPlace, CharacterID: ids["Aria's sheet"]})
	place(live.Command{Kind: live.CmdPlace, CompanionID: ids["Fang the Companion"], Q: 1})
	place(live.Command{Kind: live.CmdPlace, CompanionID: ids["Bors the Companion"], Q: -1})
	place(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Grik", Q: 3})
	return w, tb, ids, &failing, xp
}

// xpOf reads what a Character has earned.
func xpOf(t *testing.T, w world, sheet string) int {
	t.Helper()
	var xp int
	if err := w.pool.QueryRow(context.Background(), "SELECT xp FROM campaign.characters WHERE id = $1", uuid.MustParse(sheet)).Scan(&xp); err != nil {
		t.Fatal(err)
	}
	return xp
}

// A Companion stands with the party on the map: a party token under its own name, with its creature's
// statblock, run by the Player it is given to or by the DM, and marked in the roster. The DM hands it
// from one to the other, and the Companion keeps its hand and its hit points between Sessions.
func TestCompanionsStandWithTheParty(t *testing.T) {
	t.Parallel()
	w, tb, ids, _, _ := companyTable(t)
	ctx := context.Background()
	v := look(t, w, tb.dm)
	fang, bors := tokenNamed(t, v, "Fang"), tokenNamed(t, v, "Bors")
	if fang.Kind != domain.TokenParty || fang.CompanionID != ids["Fang the Companion"] || fang.ControllerID != w.player.ID.String() || fang.HP == nil || *fang.HP != 11 || len(fang.Attacks) == 0 {
		t.Fatalf("Fang on the map = %+v", fang)
	}
	if bors.Kind != domain.TokenParty || bors.CompanionID == "" || bors.ControllerID != "" || tokenNamed(t, v, "Aria").CompanionID != "" || tokenNamed(t, v, "Grik").CompanionID != "" {
		t.Fatalf("Bors on the map = %+v", bors)
	}
	// The party sees its Companions, in the roster too.
	p := look(t, w, tb.player)
	marked := map[string]bool{}
	for _, e := range p.Roster {
		marked[e.Label] = e.Companion
	}
	if !marked["Fang"] || !marked["Bors"] || marked["Aria"] || tokenNamed(t, p, "Fang").CompanionID == "" {
		t.Fatalf("the roster marks = %v", marked)
	}

	// Fang is the player's to walk; Bors is the DM's.
	if u := tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Fang"], Q: 1, R: 1}); u.Kind != live.UpdView || tokenNamed(t, u.View, "Fang").R != 1 {
		t.Fatalf("the player walks Fang = %+v", u)
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Bors"], Q: -1, R: 1}); u.Kind != live.UpdRejected {
		t.Fatalf("the player walks Bors = %+v", u)
	}
	refuse := func(sub *live.Subscriber, want string, cmd live.Command) {
		t.Helper()
		w.hub.Submit(sub, cmd)
		if u := next(t, sub); u.Kind != live.UpdRejected || u.Reason != want {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	// One Companion, one token; and only a Companion there is.
	refuse(tb.dm, "That Companion is on the map already.", live.Command{Kind: live.CmdPlace, CompanionID: ids["Fang the Companion"], Q: 2})
	refuse(tb.dm, "No such Companion.", live.Command{Kind: live.CmdPlace, CompanionID: uuid.NewString(), Q: 2})
	refuse(tb.dm, "No such Companion.", live.Command{Kind: live.CmdPlace, CompanionID: "nope", Q: 2})

	// The DM hands Bors to the player, and takes Fang over.
	refuse(tb.player, "Only the DM can change the table.", live.Command{Kind: live.CmdAssignControl, TokenID: ids["Bors"], ControllerID: w.player.ID.String()})
	refuse(tb.dm, "No such token.", live.Command{Kind: live.CmdAssignControl, TokenID: uuid.NewString()})
	refuse(tb.dm, "Only a Companion changes hands.", live.Command{Kind: live.CmdAssignControl, TokenID: ids["Aria"]})
	refuse(tb.dm, "No such member.", live.Command{Kind: live.CmdAssignControl, TokenID: ids["Bors"], ControllerID: uuid.NewString()})
	refuse(tb.dm, "No such member.", live.Command{Kind: live.CmdAssignControl, TokenID: ids["Bors"], ControllerID: "nope"})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdAssignControl, TokenID: ids["Bors"], ControllerID: w.player.ID.String()})
	if tokenNamed(t, d.View, "Bors").ControllerID != w.player.ID.String() {
		t.Fatalf("Bors after the hand-over = %+v", tokenNamed(t, d.View, "Bors"))
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdAssignControl, TokenID: ids["Fang"]})
	if tokenNamed(t, d.View, "Fang").ControllerID != "" {
		t.Fatalf("Fang after the DM took over = %+v", tokenNamed(t, d.View, "Fang"))
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Bors"], Q: -1, R: 1}); u.Kind != live.UpdView {
		t.Fatalf("the player walks Bors after the hand-over = %+v", u)
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Fang"], Q: 2, R: 1}); u.Kind != live.UpdRejected {
		t.Fatalf("the player walks Fang after the DM took over = %+v", u)
	}

	// Fang is hurt and leaves the map; Bors is hurt and still on it when the Session ends. Both keep
	// their hit points, and their new hands, for the next Session.
	tb.dmSays(live.Command{Kind: live.CmdAdjustHP, TokenID: ids["Fang"], HPDelta: -4})
	tb.dmSays(live.Command{Kind: live.CmdAdjustHP, TokenID: ids["Bors"], HPDelta: -2})
	tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: ids["Fang"]})
	sessions := &app.Sessions{Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Live: w.hub, Now: time.Now}
	if _, err := sessions.End(ctx, dmCaller, w.session.CampaignID, w.session.ID); err != nil {
		t.Fatal(err)
	}
	next1, err := sessions.Start(ctx, dmCaller, w.session.CampaignID)
	if err != nil {
		t.Fatal(err)
	}
	dm2, err := w.hub.Join(ctx, next1.ID, w.dm, dmCaller, live.AudienceDM)
	if err != nil {
		t.Fatal(err)
	}
	next(t, dm2)
	for name, want := range map[string]struct {
		hp  int
		who string
	}{"Fang": {7, ""}, "Bors": {5, w.player.ID.String()}} {
		w.hub.Submit(dm2, live.Command{Kind: live.CmdPlace, CompanionID: ids[name+" the Companion"]})
		u := next(t, dm2)
		if u.Kind != live.UpdView {
			t.Fatalf("placing %s in the next Session = %+v", name, u)
		}
		if tv := tokenNamed(t, u.View, name); *tv.HP != want.hp || tv.ControllerID != want.who {
			t.Fatalf("%s in the next Session = %+v", name, tv)
		}
		w.hub.Submit(dm2, live.Command{Kind: live.CmdRemove, TokenID: tokenNamed(t, u.View, name).ID})
		next(t, dm2)
	}
}

// XP for a fight is what its defeated enemies were worth, split evenly among the Characters who fought
// and the Companions who take a share; a Companion's share goes to nobody.
func TestCompanionsShareTheXP(t *testing.T) {
	t.Parallel()
	w, tb, ids, failing, worthOf := companyTable(t)
	fight := func() {
		t.Helper()
		tb.fight(ids, "Aria", "Fang", "Bors", "Grik")
	}
	end := func() {
		t.Helper()
		tb.dmSays(live.Command{Kind: live.CmdEndCombat})
	}
	sheet := ids["Aria's sheet"]

	// The goblin lives: nothing was defeated, nothing is earned.
	fight()
	end()
	if got := xpOf(t, w, sheet); got != 0 {
		t.Fatalf("XP for a fight nobody fell in = %d", got)
	}
	// The goblin falls. Aria and Fang split its 50 XP; Bors fights for pay, not for a share.
	tb.dmSays(live.Command{Kind: live.CmdAdjustHP, TokenID: ids["Grik"], HPDelta: -7})
	fight()
	end()
	if got := xpOf(t, w, sheet); got != 25 {
		t.Fatalf("Aria's XP with Fang taking a share = %d", got)
	}
	var awarded, rows int
	if err := w.pool.QueryRow(context.Background(), `SELECT coalesce(sum(x.amount), 0), count(*) FROM play.xp_awards x JOIN play.actions a ON a.id = x.action_id
		WHERE a.session_id = $1 AND a.kind = 'combat_ended'`, uuid.UUID(w.session.ID)).Scan(&awarded, &rows); err != nil || awarded != 25 || rows != 1 {
		t.Fatalf("the award on record = %d in %d rows, %v", awarded, rows, err)
	}
	// Without its share, Fang fights for nothing and Aria takes it all.
	if _, err := w.pool.Exec(context.Background(), "UPDATE campaign.companions SET shares_xp = false WHERE id = $1", uuid.MustParse(ids["Fang the Companion"])); err != nil {
		t.Fatal(err)
	}
	fight()
	end()
	if got := xpOf(t, w, sheet); got != 75 {
		t.Fatalf("Aria's XP with nobody sharing = %d", got)
	}
	// A fight without a Character in it earns nobody anything, and what cannot be reckoned is not guessed.
	crate, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, Label: "Cade", TokenKind: domain.TokenParty, Q: -2})
	ids["Cade"] = tokenNamed(t, crate.View, "Cade").ID
	tb.fight(ids, "Fang", "Bors", "Cade", "Grik")
	end()
	for _, unknown := range []int{1, 2} {
		*failing = unknown
		fight()
		end()
	}
	*failing = 0
	// Nor is XP too little to go round given to anyone: one point does not split in two.
	if _, err := w.pool.Exec(context.Background(), "UPDATE campaign.companions SET shares_xp = true WHERE id = $1", uuid.MustParse(ids["Fang the Companion"])); err != nil {
		t.Fatal(err)
	}
	worthOf["goblin"] = 1
	fight()
	end()
	worthOf["goblin"] = 50
	if got := xpOf(t, w, sheet); got != 75 {
		t.Fatalf("Aria's XP after fights that award nothing = %d", got)
	}
	// Two Characters and a sharing Companion: 50 over three is 16 each, and the remainder is nobody's.
	if _, err := w.pool.Exec(context.Background(), "UPDATE campaign.companions SET shares_xp = true WHERE id = $1", uuid.MustParse(ids["Fang the Companion"])); err != nil {
		t.Fatal(err)
	}
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: ids["Brom's sheet"], Label: "Brom", Q: 2, R: 1})
	ids["Brom"] = tokenNamed(t, d.View, "Brom").ID
	tb.fight(ids, "Aria", "Brom", "Fang", "Grik")
	end()
	if a, b := xpOf(t, w, sheet), xpOf(t, w, ids["Brom's sheet"]); a != 91 || b != 16 {
		t.Fatalf("XP split three ways = Aria %d, Brom %d", a, b)
	}
}
