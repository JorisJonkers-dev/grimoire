package live_test

import (
	"context"
	"strings"
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

func TestCreaturesSuggestWhatTheyCanObserve(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString()})
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Label: "Brom", Q: 4, R: 1})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 1})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "hobgoblin", TokenKind: domain.TokenEnemy, Q: 5})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "wolf", TokenKind: domain.TokenEnemy, Q: -4})
	ids := map[string]string{}
	var setup []live.CombatantSetup
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
		setup = append(setup, live.CombatantSetup{TokenID: tv.ID, SpeedFt: 30})
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: setup})
	for label, face := range map[string]int{"Aria": 20, "Brom": 15, "Goblin": 10, "Hobgoblin": 5, "Wolf": 1} {
		who := w.dm
		if label == "Aria" || label == "Brom" {
			who = w.player
		}
		tb.roll(combatant(d.View, label), who, face)
	}
	fill := func(rollID string, who domain.Member, faces ...int) {
		t.Helper()
		cl := dmCaller
		if !who.DM {
			cl = playerCaller
		}
		for i, f := range faces {
			if _, err := rolls.SetDie(ctx, cl, w.session.CampaignID, domain.RollID(uuid.MustParse(rollID)), i, app.Fill{Value: f}); err != nil {
				t.Fatal(err)
			}
		}
		next(t, tb.dm)
		tb.party = append(tb.party, next(t, tb.player))
	}
	dmView := func() *live.View {
		t.Helper()
		w.hub.Submit(tb.dm, live.Command{Kind: live.CmdResync})
		return next(t, tb.dm).View
	}

	u := tb.playerSays(live.Command{Kind: live.CmdAttack, TokenID: ids["Aria"], AttackNo: 1, TargetID: ids["Hobgoblin"]})
	fill(u.View.Combat.Attack.RollID, w.player, 19, 19)
	fill(tb.party[len(tb.party)-1].View.Combat.Attack.RollID, w.player, 6)
	if hp := *token(dmView(), "Hobgoblin").HP; hp != 3 {
		t.Fatalf("the longbow hit for 8: hp %d", hp)
	}
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(u.View, "Aria").ID})
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(u.View, "Brom").ID})

	v := dmView()
	gob := combatant(v, "Goblin").Suggestion
	if gob == nil || gob.TargetID != ids["Aria"] || *gob.AttackNo != 0 || gob.Reason != "Simple: Aria is the nearest enemy, 5 ft away." {
		t.Fatalf("the goblin's suggestion = %+v", gob)
	}
	if combatant(v, "Goblin").Tactics != "auto" || combatant(v, "Hobgoblin").Suggestion != nil || combatant(v, "Aria").Tactics != "" {
		t.Fatalf("only the acting creature suggests, and tactics are shown for creatures = %+v", v.Combat)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdAttack, TokenID: ids["Goblin"], AttackNo: *gob.AttackNo, TargetID: gob.TargetID})
	if combatant(d.View, "Goblin").Suggestion != nil {
		t.Fatalf("no suggestion while an attack waits: %+v", d.View.Combat)
	}
	fill(d.View.Combat.Attack.RollID, w.dm, 1)
	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Goblin").ID})

	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	hob := combatant(dmView(), "Hobgoblin").Suggestion
	if hob == nil || hob.TargetID != ids["Aria"] || *hob.AttackNo != 1 || hob.Reason != "Cunning: it saw Aria deal 8 damage from range." {
		t.Fatalf("the cunning hobgoblin remembers who shot it, even after a restart = %+v", hob)
	}
	for want, cmd := range map[string]live.Command{
		"auto, simple, cunning or off": {Kind: live.CmdSetTactics, TokenID: ids["Hobgoblin"], Tactics: "genius"},
		"only creatures the dm plays":  {Kind: live.CmdSetTactics, TokenID: ids["Aria"], Tactics: "simple"},
		"only creatures the dm plays ": {Kind: live.CmdSetTactics, TokenID: "nope", Tactics: "simple"},
	} {
		w.hub.Submit(tb.dm, cmd)
		if u := next(t, tb.dm); u.Kind != live.UpdRejected || !strings.Contains(strings.ToLower(u.Reason), strings.TrimSpace(want)) {
			t.Errorf("%s = %+v", want, u)
		}
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdSetTactics, TokenID: ids["Hobgoblin"], Tactics: "off"}); u.Reason != "Only the DM can change the table." {
		t.Fatalf("a player setting tactics = %+v", u)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdSetTactics, TokenID: ids["Hobgoblin"], Tactics: "simple"})
	if s := combatant(d.View, "Hobgoblin").Suggestion; s.TargetID != ids["Brom"] || combatant(d.View, "Hobgoblin").Tactics != "simple" {
		t.Fatalf("a simple hobgoblin goes for the nearest = %+v", s)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdSetTactics, TokenID: ids["Hobgoblin"], Tactics: "off"})
	if combatant(d.View, "Hobgoblin").Suggestion != nil {
		t.Fatalf("tactics off = %+v", d.View.Combat)
	}
	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Hobgoblin").ID})
	wolf := combatant(dmView(), "Wolf").Suggestion
	if wolf == nil || wolf.AttackNo != nil || wolf.TargetID != ids["Aria"] || !strings.Contains(wolf.Reason, "close in on Aria, 20 ft away") {
		t.Fatalf("a wolf out of reach = %+v", wolf)
	}
	if body := payloads(t, tb.party...); strings.Contains(body, "suggestion") || strings.Contains(body, `"tactics"`) {
		t.Fatalf("suggestions reached the party: %s", body)
	}
}
