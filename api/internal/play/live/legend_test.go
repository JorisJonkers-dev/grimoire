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

// lairs adds the Bog King: legendary, with a lair, a second phase and a damage threshold of 5.
type lairs struct{ bestiary }

func (l lairs) Monster(ctx context.Context, campaign uuid.UUID, slug string) (string, domain.Stats, error) {
	if slug != "bog-king" {
		return l.bestiary.Monster(ctx, campaign, slug)
	}
	return "Bog King", domain.Stats{
		Source: "monster:hb-bog", AC: 10, HP: 8, HPMax: 8, Intelligence: 8, SpeedFt: 30, Saves: map[string]int{}, Attacks: []domain.Attack{},
		Legend: &domain.Legend{
			Uses: 2, Left: 2, Resistance: 1, ResistLeft: 1, Threshold: 5,
			Actions: []domain.LegendAction{{Name: "Tail Sweep", Cost: 1, Text: "One Claw attack."}, {Name: "Sink", Cost: 2, Text: "It sinks and moves 30 feet."}},
			Lair:    []domain.LegendAction{{Name: "Rising Water", Text: "The water rises a foot."}},
			Phases:  []domain.Phase{{Name: "Drowned King", HP: 15, Text: "It rises from the water."}},
		},
	}, nil
}

// The Bog King takes a legendary action only right after another creature's turn, one each time and as
// many a round as it has; its lair acts once a round from initiative count 20; it spends its Legendary
// Resistance; blows under its damage threshold do nothing; and at 0 hit points it rises in its second
// phase.
func TestLegendaryWindowsLairsAndPhases(t *testing.T) {
	t.Parallel()
	w := setup(t)
	w.hub.Stats = lairs{bestiary{owner: w.player.ID}}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Q: 0})
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Label: "Brom", Q: 0, R: 1})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "bog-king", TokenKind: domain.TokenEnemy, Q: 1})
	ids := map[string]string{}
	var setup []live.CombatantSetup
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
		setup = append(setup, live.CombatantSetup{TokenID: tv.ID, SpeedFt: 30})
	}
	if l := token(d.View, "Bog King").Legend; l == nil || l.Left != 2 || len(l.Actions) != 2 || l.Phases != 1 {
		t.Fatalf("the DM's view of the Bog King = %+v", l)
	}
	d, p := tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: setup})
	if token(p.View, "Bog King").Legend != nil {
		t.Fatal("the party never sees the Bog King's legendary actions")
	}
	for label, face := range map[string]int{"Aria": 20, "Brom": 16, "Bog King": 10} {
		who := w.player
		if label == "Bog King" {
			who = w.dm
		}
		tb.roll(combatant(d.View, label), who, face)
	}
	king := ids["Bog King"]
	refused := func(cmd live.Command, want string) {
		t.Helper()
		barrier(t, w, tb)
		w.hub.Submit(tb.dm, cmd)
		if u := next(t, tb.dm); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, want) {
			t.Fatalf("%s %s = %+v, want %q", cmd.Kind, cmd.Legend, u, want)
		}
	}
	sweep := live.Command{Kind: live.CmdLegendary, TokenID: king, Legend: "Tail Sweep"}
	water := live.Command{Kind: live.CmdLair, TokenID: king, Legend: "Rising Water"}
	refused(sweep, "only right after another creature's turn")
	refused(water, "initiative count 20")
	refused(live.Command{Kind: live.CmdLegendary, TokenID: ids["Aria"], Legend: "Tail Sweep"}, "no legendary or lair actions")
	refused(live.Command{Kind: live.CmdLegendary, TokenID: king, Legend: "Dance"}, "no legendary action called Dance")

	hit := func(attacker string, damage int) *live.View {
		t.Helper()
		barrier(t, w, tb)
		p := tb.playerSays(live.Command{Kind: live.CmdAttack, TokenID: ids[attacker], TargetID: king})
		tb.fill(p.View.Combat.Attack.RollID, w.player, 15)
		barrier(t, w, tb)
		tb.fill(look(t, w, tb.player).Combat.Attack.RollID, w.player, damage)
		return barrier(t, w, tb)
	}
	if v := hit("Aria", 1); *token(v, "Bog King").HP != 8 {
		t.Fatalf("a 4-point blow under the threshold of 5 = %d", *token(v, "Bog King").HP)
	}
	barrier(t, w, tb)
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(look(t, w, tb.player), "Aria").ID})

	d, _ = tb.dmSays(sweep)
	if l := token(d.View, "Bog King").Legend; l.Left != 1 || l.Ready || !l.LairReady {
		t.Fatalf("after a Tail Sweep = %+v", l)
	}
	if m := d.View.Manual; len(m) == 0 || m[len(m)-1].Text != "Bog King uses Tail Sweep: One Claw attack." {
		t.Fatalf("the DM's prompt = %+v", d.View.Manual)
	}
	refused(sweep, "only right after another creature's turn")
	tb.dmSays(water)
	refused(water, "has acted this round")
	refused(live.Command{Kind: live.CmdLair, TokenID: king, Legend: "Flood"}, "no action called Flood")

	v := hit("Brom", 5)
	if tok := token(v, "Bog King"); *tok.HP != 15 || *tok.HPMax != 15 || tok.Legend.Phase != 1 {
		t.Fatalf("the Bog King at 0 hit points = %d/%d phase %d", *tok.HP, *tok.HPMax, tok.Legend.Phase)
	}
	if m := v.Manual; len(m) == 0 || !strings.Contains(m[len(m)-1].Text, "enters its next phase: Drowned King.") {
		t.Fatalf("the phase prompt = %+v", v.Manual)
	}
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(look(t, w, tb.player), "Brom").ID})
	refused(sweep, "only right after another creature's turn")
	tb.dmSays(live.Command{Kind: live.CmdResist, TokenID: king})
	refused(live.Command{Kind: live.CmdResist, TokenID: king}, "no Legendary Resistance left")
	barrier(t, w, tb)
	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(look(t, w, tb.dm), "Bog King").ID})

	d, _ = tb.dmSays(live.Command{Kind: live.CmdLegendary, TokenID: king, Legend: "Sink"})
	if l := token(d.View, "Bog King").Legend; l.Left != 0 || l.LairReady {
		t.Fatalf("after Sink in round 2 = %+v", l)
	}
	barrier(t, w, tb)
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(look(t, w, tb.player), "Aria").ID})
	refused(sweep, "0 legendary actions left this round")

	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	if l := token(look(t, w, tb.dm), "Bog King").Legend; l == nil || l.Phase != 1 || l.ResistLeft != 0 || l.Left != 0 {
		t.Fatalf("the Bog King's Legend after a restart = %+v", l)
	}
	tb.dmSays(live.Command{Kind: live.CmdEndCombat})
	refused(sweep, "wait for a fight")
}
