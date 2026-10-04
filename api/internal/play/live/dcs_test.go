package live_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
)

// showDCs has the Campaign show its DCs, as the DM sets it outside the Session.
func showDCs(t *testing.T, w world) {
	t.Helper()
	if _, err := w.pool.Exec(context.Background(), "UPDATE campaign.campaigns SET show_dcs = true WHERE id = $1", w.session.CampaignID); err != nil {
		t.Fatal(err)
	}
}

// A save's DC is on its Roll Card, and goes to the party with the save, only while the Campaign shows
// DCs. The DM always has it. The Session under way follows the setting as the DM changes it, and
// when it cannot be read the DC stays the DM's to know.
func TestASavesDCIsShownOnlyWhileTheCampaignShowsIt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb, ids := magicTable(t)
	shows := func(shown bool) {
		t.Helper()
		if _, err := w.pool.Exec(ctx, "UPDATE campaign.campaigns SET show_dcs = $2 WHERE id = $1", w.session.CampaignID, shown); err != nil {
			t.Fatal(err)
		}
	}
	tb.fight(ids, "Aria", "Goblin")
	d, _ := tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Goblin"], Effect: "frightened", SaveAbility: "wisdom", SaveDC: 10})
	aria, goblin := combatant(d.View, "Aria").ID, combatant(d.View, "Goblin").ID
	// round plays one round: the Goblin saves against the Effect as its turn ends, and fails.
	round := func() (dc, partyDC int, purpose string) {
		t.Helper()
		tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: aria})
		d, p := tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: goblin})
		if len(d.View.Saves) != 1 || len(p.View.Saves) != 1 {
			t.Fatalf("saves = %+v, party %+v", d.View.Saves, p.View.Saves)
		}
		purpose = lastRoll(t, w, "Wisdom save to end").Purpose
		tb.fill(d.View.Saves[0].RollID, w.dm, 1)
		next(t, tb.dm)
		next(t, tb.player)
		return d.View.Saves[0].DC, p.View.Saves[0].DC, purpose
	}

	// Hidden, as a Campaign starts.
	if dc, party, purpose := round(); dc != 10 || party != 0 || purpose != "Wisdom save to end Frightened" {
		t.Fatalf("hidden = %d, party %d, %q", dc, party, purpose)
	}
	shows(true)
	if dc, party, purpose := round(); dc != 10 || party != 10 || purpose != "Wisdom save to end Frightened (DC 10)" {
		t.Fatalf("shown = %d, party %d, %q", dc, party, purpose)
	}
	shows(false)
	if dc, party, purpose := round(); dc != 10 || party != 0 || purpose != "Wisdom save to end Frightened" {
		t.Fatalf("hidden again = %d, party %d, %q", dc, party, purpose)
	}

	shows(true)
	w.hub.Close(w.session.ID)
	w.hub.Store = unreadableDCs{Store: w.hub.Store}
	tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)
	if dc, party, purpose := round(); dc != 10 || party != 0 || purpose != "Wisdom save to end Frightened" {
		t.Fatalf("unreadable = %d, party %d, %q", dc, party, purpose)
	}
}

// Karmic dice go by the rolls a fight asks of a roller. A check a player takes whenever they like
// outside a fight is of their own making: it is not asked of them, so it can neither lean nor count,
// and nobody can take free checks until their next attack leans.
func TestOnlyAFightsRollsAreAskedOfTheirRoller(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb, ids := magicTable(t)
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenNPC, Label: "Innkeeper", Q: 1})
	keeper := token(d.View, "Innkeeper")
	asked := func(purpose string) (yes, all int) {
		t.Helper()
		if err := w.pool.QueryRow(ctx, "SELECT count(*) FILTER (WHERE asked), count(*) FROM play.roll_requests WHERE campaign_id = $1 AND purpose LIKE $2",
			w.session.CampaignID, purpose+"%").Scan(&yes, &all); err != nil {
			t.Fatal(err)
		}
		return yes, all
	}
	for range 3 {
		if p := tb.playerSays(live.Command{Kind: live.CmdTakeAction, TokenID: ids["Aria"], Action: "influence", TargetID: keeper.ID}); p.View == nil {
			t.Fatalf("an Influence check outside a fight = %+v", p)
		}
		tb.fill(uuid.UUID(lastRoll(t, w, "Influence").ID).String(), w.player, 3)
		// The roll settling is a change of its own: both screens catch up before the next try.
		look(t, w, tb.player)
		look(t, w, tb.dm)
	}
	if yes, all := asked("Influence"); yes != 0 || all != 3 {
		t.Fatalf("checks outside a fight asked = %d of %d", yes, all)
	}
	// In a fight, the initiative and the attack are asked.
	tb.fight(ids, "Aria", "Goblin")
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Goblin"], Q: 0, R: 1})
	if u := tb.playerSays(live.Command{Kind: live.CmdAttack, TokenID: ids["Aria"], TargetID: ids["Goblin"]}); u.View == nil || u.View.Combat.Attack == nil {
		t.Fatalf("the attack = %+v", u)
	}
	if yes, all := asked("Initiative"); yes != 2 || all != 2 {
		t.Fatalf("initiative asked = %d of %d", yes, all)
	}
	if yes, all := asked("Longsword attack"); yes != 1 || all != 1 {
		t.Fatalf("the attack asked = %d of %d", yes, all)
	}
}
