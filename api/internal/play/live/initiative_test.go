package live_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
)

// initiativeTable sets a Campaign's initiative options before anyone joins, then puts Aria, two goblins
// and a hobgoblin on the board.
func initiativeTable(t *testing.T, mode string, share bool) (world, *table, *live.View) {
	t.Helper()
	w := setup(t)
	if _, err := w.pool.Exec(context.Background(), "UPDATE campaign.campaigns SET initiative_mode = $2, share_initiative = $3 WHERE id = $1", w.session.CampaignID, mode, share); err != nil {
		t.Fatal(err)
	}
	_, tb, _ := magicTableIn(t, w)
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Goblin 2", Q: 4})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "hobgoblin", TokenKind: domain.TokenEnemy, Q: -3})
	var setup []live.CombatantSetup
	for _, tv := range d.View.Tokens {
		setup = append(setup, live.CombatantSetup{TokenID: tv.ID, SpeedFt: 30})
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: setup})
	return w, tb, d.View
}

func rollsOf(v *live.View) map[string][]string {
	out := map[string][]string{}
	for _, c := range v.Combat.Combatants {
		out[c.RollID] = append(out[c.RollID], c.Label)
	}
	return out
}

// Side initiative rolls once for the party and once for its foes; every foe takes the foes' roll.
func TestSideInitiative(t *testing.T) {
	t.Parallel()
	w, tb, v := initiativeTable(t, "side", false)
	if rolls := rollsOf(v); len(rolls) != 2 {
		t.Fatalf("two rolls = %v", rolls)
	}
	foes := uuid.UUID(lastRoll(t, w, "Initiative for the foes").ID).String()
	party := uuid.UUID(lastRoll(t, w, "Initiative for the party").ID).String()
	tb.fill(foes, w.dm, 12)
	tb.fill(party, w.player, 7)
	for range 2 {
		next(t, tb.dm)
		next(t, tb.player)
	}
	v = look(t, w, tb.dm)
	for _, label := range []string{"Goblin", "Goblin 2", "Hobgoblin"} {
		if c := combatant(v, label); c.Initiative == nil || *c.Initiative != 12 {
			t.Errorf("%s acts on the foes' 12 = %+v", label, c)
		}
	}
	if c := combatant(v, "Aria"); *c.Initiative != 7 {
		t.Errorf("Aria = %+v", c)
	}
}

// Identical monsters share one roll; everyone else rolls alone.
func TestSharedInitiativeForIdenticalMonsters(t *testing.T) {
	t.Parallel()
	w, tb, v := initiativeTable(t, "individual", true)
	if rolls := rollsOf(v); len(rolls) != 3 {
		t.Fatalf("Aria, the goblins and the hobgoblin roll = %v", rolls)
	}
	tb.fill(uuid.UUID(lastRoll(t, w, "Initiative for Goblin").ID).String(), w.dm, 9)
	next(t, tb.dm)
	next(t, tb.player)
	v = look(t, w, tb.dm)
	if a, b := combatant(v, "Goblin"), combatant(v, "Goblin 2"); a.RollID != b.RollID || *a.Initiative != *b.Initiative || combatant(v, "Hobgoblin").Initiative != nil {
		t.Fatalf("the goblins share = %+v %+v", a, b)
	}
}

// The DM can put Exploration into turns: the party moves in order, a speed each, and passes the turn on.
func TestExplorationTurns(t *testing.T) {
	t.Parallel()
	w, tb, ids := magicTable(t)
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Label: "Brom", Q: 0, R: 2})
	for _, tv := range d.View.Tokens {
		if tv.Q == 0 && tv.R == 2 {
			ids["Brom"] = tv.ID
		}
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdExplore, On: true}); !strings.Contains(u.Reason, "Only the DM") {
		t.Fatalf("a player switching = %+v", u)
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdPassTurn}); !strings.Contains(u.Reason, "not in turns") {
		t.Fatalf("passing without turns = %+v", u)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdExplore, On: true})
	if e := d.View.Exploration; e == nil || e.Turn != ids["Aria"] || e.LeftFt != 30 || len(e.Order) != 2 {
		t.Fatalf("exploration turns = %+v", e)
	}
	walk := func(who string, q, r int) live.Update {
		return tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids[who], Q: q, R: r})
	}
	if u := walk("Brom", 1, 2); !strings.Contains(u.Reason, "not Brom's turn to explore") {
		t.Fatalf("out of turn = %+v", u)
	}
	if u := walk("Aria", -4, 0); u.View.Exploration.LeftFt != 10 {
		t.Fatalf("20 feet walked = %+v", u.View.Exploration)
	}
	if u := walk("Aria", -7, 0); !strings.Contains(u.Reason, "has 10 ft of movement left this turn") {
		t.Fatalf("too far = %+v", u)
	}
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	if u := tb.playerSays(live.Command{Kind: live.CmdPassTurn}); u.View.Exploration.Turn != ids["Brom"] || u.View.Exploration.LeftFt != 30 {
		t.Fatalf("passed after a restart = %+v", u.View.Exploration)
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdPassTurn}); u.View.Exploration.Turn != ids["Aria"] {
		t.Fatalf("round the order = %+v", u.View.Exploration)
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdExplore}); !strings.Contains(u.Reason, "Only the DM") {
		t.Fatalf("a player ending = %+v", u)
	}
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdExplore, On: true})
	if u := next(t, tb.dm); !strings.Contains(u.Reason, "already in turns") {
		t.Fatalf("twice = %+v", u)
	}
	tb.fight(ids, "Aria", "Goblin")
	v := look(t, w, tb.dm)
	if v.Exploration != nil {
		t.Fatal("a fight ends exploration turns")
	}
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdExplore, On: true})
	if u := next(t, tb.dm); !strings.Contains(u.Reason, "own turns") {
		t.Fatalf("in a fight = %+v", u)
	}
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdExplore})
	if u := next(t, tb.dm); !strings.Contains(u.Reason, "not in turns") {
		t.Fatalf("ending none = %+v", u)
	}
}
