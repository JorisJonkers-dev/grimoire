package live_test

import (
	"encoding/json"
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

// A creature's Suggested Action and its Tactics are the DM's alone: through a whole turn of a goblin
// that has one, nothing the party or the Table Display is sent mentions either.
func TestSuggestedActionsAndTacticsNeverReachPlayers(t *testing.T) {
	t.Parallel()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tv := join(t, w, w.player, playerCaller, live.AudienceTable)
	var sent []live.Update
	keep := func() {
		t.Helper()
		barrier(t, w, tb)
		sent = append(sent, live.Update{Kind: live.UpdSnapshot, View: look(t, w, tb.player)}, live.Update{Kind: live.UpdSnapshot, View: look(t, w, tv)})
	}
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Q: 0})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 1})
	var fighters []live.CombatantSetup
	for _, v := range d.View.Tokens {
		fighters = append(fighters, live.CombatantSetup{TokenID: v.ID, SpeedFt: 30})
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: fighters})
	tb.roll(combatant(d.View, "Goblin"), w.dm, 18)
	tb.roll(combatant(d.View, "Aria"), w.player, 3)
	keep()

	dm := look(t, w, tb.dm)
	goblin := combatant(dm, "Goblin")
	if goblin.Suggestion == nil || goblin.Tactics != "auto" {
		t.Fatalf("the DM should see the goblin's Suggested Action and Tactics: %+v", goblin)
	}
	_, p := tb.dmSays(live.Command{Kind: live.CmdSetTactics, TokenID: goblin.TokenID, Tactics: "cunning"})
	sent = append(sent, p)
	keep()
	_, p = tb.dmSays(live.Command{Kind: live.CmdAttack, TokenID: goblin.TokenID, AttackNo: *goblin.Suggestion.AttackNo, TargetID: goblin.Suggestion.TargetID})
	sent = append(sent, p)
	keep()
	sent = append(sent, tb.party...)

	for i, u := range sent {
		raw, err := json.Marshal(u)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{`"suggestion"`, `"tactics"`, "nearest enemy", "Cunning", "Simple"} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("update %d to the players carries %s: %s", i, secret, raw)
			}
		}
	}
	if raw, _ := json.Marshal(dm); !strings.Contains(string(raw), `"suggestion"`) || !strings.Contains(string(raw), `"tactics"`) {
		t.Fatalf("the check itself is blind: the DM's view has neither word: %s", raw)
	}
}
