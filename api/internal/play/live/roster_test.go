package live_test

import (
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

func roster(v *live.View) map[string]live.RosterEntry {
	out := map[string]live.RosterEntry{}
	for _, e := range v.Roster {
		out[e.Label] = e
	}
	return out
}

// Everyone at the table shares one roster strip, filtered for each audience: the DM sees exact hit
// points and hidden creatures; players and the Table Display see the party's hit points, only a rough
// health for anyone else, and never a hidden creature. In a fight the strip follows initiative and
// marks who acts.
func TestTheRosterStripIsFilteredForEachAudience(t *testing.T) {
	t.Parallel()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tv := join(t, w, w.player, playerCaller, live.AudienceTable)
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Q: 0})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 1})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "wolf", TokenKind: domain.TokenEnemy, Q: 2, Hidden: true})
	goblin := token(d.View, "Goblin").ID
	tb.dmSays(live.Command{Kind: live.CmdAdjustHP, TokenID: goblin, HPDelta: -2})
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: goblin, Effect: "prone"})
	barrier(t, w, tb)

	dm := roster(look(t, w, tb.dm))
	if len(dm) != 3 || !dm["Wolf"].Hidden || dm["Goblin"].HP == nil || *dm["Goblin"].HP != 5 || *dm["Goblin"].HPMax != 7 || len(dm["Goblin"].Effects) != 1 {
		t.Fatalf("the DM's roster = %+v", dm)
	}
	for who, v := range map[string]*live.View{"party": look(t, w, tb.player), "table": look(t, w, tv)} {
		r := roster(v)
		if _, sees := r["Wolf"]; sees || len(r) != 2 {
			t.Fatalf("the %s sees a hidden creature on the roster: %+v", who, r)
		}
		if g := r["Goblin"]; g.HP != nil || g.HPMax != nil || g.Health != "hurt" || len(g.Effects) != 1 || g.Effects[0].Name != "Prone" {
			t.Fatalf("the goblin on the %s's roster = %+v", who, g)
		}
		if a := r["Aria"]; a.HP == nil || *a.HP != 12 || a.Health != "" {
			t.Fatalf("Aria on the %s's roster = %+v", who, a)
		}
	}

	var setup []live.CombatantSetup
	for _, tv := range look(t, w, tb.dm).Tokens {
		setup = append(setup, live.CombatantSetup{TokenID: tv.ID, SpeedFt: 30})
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: setup})
	for label, face := range map[string]int{"Aria": 5, "Goblin": 18, "Wolf": 12} {
		who := w.player
		if label != "Aria" {
			who = w.dm
		}
		tb.roll(combatant(d.View, label), who, face)
	}
	v := barrier(t, w, tb)
	order := make([]string, 0, 3)
	for _, e := range v.Roster {
		order = append(order, e.Label)
	}
	if len(order) != 3 || order[0] != "Goblin" || order[1] != "Wolf" || order[2] != "Aria" || !v.Roster[0].Acting || v.Roster[2].Acting {
		t.Fatalf("the DM's roster in initiative order = %v %+v", order, v.Roster)
	}
	if p := look(t, w, tb.player).Roster; len(p) != 2 || p[0].Label != "Goblin" || p[1].Label != "Aria" {
		t.Fatalf("the party's roster in a fight = %+v", p)
	}
}
