package live_test

import (
	"slices"
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

func labels(order []live.InitiativeRoll) []string {
	out := make([]string, 0, len(order))
	for _, r := range order {
		out = append(out, r.Label)
	}
	return out
}

// The Update that settles initiative carries the rolls in order for every screen to reveal, and each
// Update that starts a turn names whose it is, so a player's screen can raise its banner. A hidden
// creature's roll and turn stay with the DM.
func TestInitiativeIsRevealedAndTurnsAreAnnounced(t *testing.T) {
	t.Parallel()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Q: 0})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 1})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", Label: "Lurker", TokenKind: domain.TokenEnemy, Q: 2, Hidden: true})
	ids := map[string]string{}
	var fighters []live.CombatantSetup
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
		fighters = append(fighters, live.CombatantSetup{TokenID: tv.ID, SpeedFt: 30})
	}
	d, p := tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: fighters})
	if d.Initiative != nil || d.Turn != nil || p.Initiative != nil || p.Turn != nil {
		t.Fatalf("nothing is revealed while initiative is still being rolled: %+v %+v", d, p)
	}
	tb.roll(combatant(d.View, "Goblin"), w.dm, 18)
	tb.roll(combatant(d.View, "Lurker"), w.dm, 12)
	if last := tb.party[len(tb.party)-1]; last.Initiative != nil || last.Turn != nil {
		t.Fatalf("two of three rolls are in, and the party is told %+v %+v", last.Initiative, last.Turn)
	}
	id, _ := uuid.Parse(combatant(d.View, "Aria").RollID)
	if _, err := rolls.SetDie(t.Context(), playerCaller, w.session.CampaignID, domain.RollID(id), 0, app.Fill{Value: 3}); err != nil {
		t.Fatal(err)
	}
	d, p = next(t, tb.dm), next(t, tb.player)
	if got := labels(d.Initiative.Order); !slices.Equal(got, []string{"Goblin", "Lurker", "Aria"}) || d.Initiative.Order[2].Initiative != 3 || d.Initiative.Order[2].Kind != domain.TokenParty {
		t.Fatalf("the DM's reveal = %+v", d.Initiative.Order)
	}
	if got := labels(p.Initiative.Order); !slices.Equal(got, []string{"Goblin", "Aria"}) {
		t.Fatalf("the party's reveal shows a hidden creature: %v", got)
	}
	for who, u := range map[string]live.Update{"DM": d, "party": p} {
		if u.Turn == nil || u.Turn.Round != 1 || !slices.Equal(u.Turn.TokenIDs, []string{ids["Goblin"]}) {
			t.Fatalf("the %s's first turn = %+v", who, u.Turn)
		}
	}

	end := func(label string) (live.Update, live.Update) {
		t.Helper()
		return tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(look(t, w, tb.dm), label).ID})
	}
	d, p = tb.dmSays(live.Command{Kind: live.CmdSpend, CombatantID: combatant(look(t, w, tb.dm), "Goblin").ID, Resource: live.ResourceAction})
	if d.Turn != nil || p.Turn != nil || d.Initiative != nil {
		t.Fatalf("a change within a turn announces nothing: %+v %+v", d.Turn, p.Turn)
	}
	d, p = end("Goblin")
	if d.Turn == nil || !slices.Equal(d.Turn.TokenIDs, []string{ids["Lurker"]}) || p.Turn != nil {
		t.Fatalf("a hidden creature's turn: the DM %+v, the party %+v", d.Turn, p.Turn)
	}
	d, p = end("Lurker")
	if d.Turn == nil || p.Turn == nil || !slices.Equal(p.Turn.TokenIDs, []string{ids["Aria"]}) || p.Initiative != nil {
		t.Fatalf("Aria's turn: the DM %+v, the party %+v", d.Turn, p.Turn)
	}
	_, p = end("Aria")
	if p.Turn == nil || p.Turn.Round != 2 || !slices.Equal(p.Turn.TokenIDs, []string{ids["Goblin"]}) {
		t.Fatalf("round 2 = %+v", p.Turn)
	}

	// Alone in a fight, the same creature's next turn is still a turn starting.
	tb.dmSays(live.Command{Kind: live.CmdEndCombat})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: []live.CombatantSetup{{TokenID: ids["Aria"], SpeedFt: 30}}})
	tb.roll(combatant(d.View, "Aria"), w.player, 10)
	if p = tb.party[len(tb.party)-1]; p.Initiative == nil || len(p.Initiative.Order) != 1 || p.Turn == nil || p.Turn.Round != 1 {
		t.Fatalf("a solo fight begins = %+v %+v", p.Initiative, p.Turn)
	}
	if _, p = end("Aria"); p.Turn == nil || p.Turn.Round != 2 || !slices.Equal(p.Turn.TokenIDs, []string{ids["Aria"]}) {
		t.Fatalf("a solo fight's round 2 = %+v", p.Turn)
	}
}
