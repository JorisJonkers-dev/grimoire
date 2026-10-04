package live_test

import (
	"context"
	"testing"

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
