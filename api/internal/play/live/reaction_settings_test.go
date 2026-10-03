package live_test

import (
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

// Each Controller says how their reactions go: a goblin set never to make opportunity attacks lets Aria
// walk off; Aria set always to cast Shield raises it the moment it turns a hit into a miss; an Effect
// that reacts to damage prompts its bearer; and the settings survive a restart.
func TestReactionSettings(t *testing.T) {
	t.Parallel()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Shield: true})
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Label: "Brom", Q: 4, R: -2})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 1})
	ids := map[string]string{}
	var setup []live.CombatantSetup
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
		setup = append(setup, live.CombatantSetup{TokenID: tv.ID, SpeedFt: 30})
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: setup})
	for label, face := range map[string]int{"Aria": 20, "Brom": 15, "Goblin": 10} {
		who := w.dm
		if label != "Goblin" {
			who = w.player
		}
		tb.roll(combatant(d.View, label), who, face)
	}
	set := func(sub *live.Subscriber, token, kind, mode, condition string) live.Update {
		t.Helper()
		w.hub.Submit(sub, live.Command{Kind: live.CmdSetReaction, TokenID: ids[token], ReactionKind: kind, ReactionMode: mode, Condition: condition})
		u := next(t, sub)
		if u.Kind == live.UpdView {
			other := tb.player
			if sub == tb.player {
				other = tb.dm
			}
			next(t, other)
		}
		return u
	}
	for want, u := range map[string]live.Update{
		"Choose a reaction":  set(tb.player, "Aria", "dance", "always", ""),
		"ask, always":        set(tb.player, "Aria", "shield", "often", ""),
		"Choose a reaction,": set(tb.player, "Aria", "shield", "always", "moonlit"),
		"not yours":          set(tb.player, "Goblin", "opportunity_attack", "never", ""),
	} {
		if u.Kind != live.UpdRejected || !strings.Contains(u.Reason, strings.TrimSuffix(want, ",")) {
			t.Errorf("%s = %+v", want, u)
		}
	}
	set(tb.dm, "Goblin", "opportunity_attack", "never", "")
	p := set(tb.player, "Aria", "shield", "always", "")
	if r := token(p.View, "Aria").Reactions; len(r) != 1 || r[0] != (live.ReactionSettingView{Kind: "shield", Mode: "always"}) {
		t.Fatalf("Aria's settings = %+v", r)
	}
	if token(p.View, "Goblin").Reactions != nil {
		t.Fatal("the party never sees the goblin's settings")
	}

	p = tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: -2, R: 0})
	if p.View.Combat.Prompt != nil || token(p.View, "Aria").Q != -2 {
		t.Fatalf("a goblin that never takes opportunity attacks lets Aria go = %+v", p.View.Combat.Prompt)
	}
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(p.View, "Aria").ID})
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(p.View, "Brom").ID})
	tb.dmSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Goblin"], Q: -1, R: 0})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdAttack, TokenID: ids["Goblin"], TargetID: ids["Aria"]})
	tb.fill(d.View.Combat.Attack.RollID, w.dm, 13)
	v := barrier(t, w, tb)
	if aria := combatant(v, "Aria"); aria.Reaction || v.Combat.Attack != nil || v.Combat.Prompt != nil || *token(v, "Aria").HP != 12 {
		t.Fatalf("Shield went up by itself and turned the 17 into a miss = %+v %+v", aria, v.Combat)
	}
	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(v, "Goblin").ID})

	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Goblin"], Effect: "hellish-rebuke"})
	p = tb.playerSays(live.Command{Kind: live.CmdAttack, TokenID: ids["Aria"], TargetID: ids["Goblin"]})
	tb.fill(p.View.Combat.Attack.RollID, w.player, 15)
	barrier(t, w, tb)
	tb.fill(look(t, w, tb.player).Combat.Attack.RollID, w.player, 1)
	v = barrier(t, w, tb)
	if pr := v.Combat.Prompt; pr == nil || pr.Kind != "effect" || pr.ReactorID != ids["Goblin"] || !strings.HasPrefix(pr.Effect, "Hellish Rebuke:") {
		t.Fatalf("damage sets off the goblin's Hellish Rebuke = %+v", v.Combat.Prompt)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdReact, Use: true})
	if m := d.View.Manual; len(m) == 0 || !strings.HasPrefix(m[len(m)-1].Text, "Goblin: Hellish Rebuke:") || combatant(d.View, "Goblin").Reaction {
		t.Fatalf("the DM resolves the rebuke = %+v", d.View.Manual)
	}

	set(tb.dm, "Goblin", "opportunity_attack", "always", "target_bloodied")
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	if r := token(look(t, w, tb.dm), "Goblin").Reactions; len(r) != 1 || r[0].Condition != "target_bloodied" {
		t.Fatalf("settings survive a restart = %+v", r)
	}
}

// barrier reads both subscribers up to a fresh snapshot, so no broadcast from a roll that just
// resolved is left queued; a quiet-window drain can return before a late one lands.
func barrier(t *testing.T, w world, tb *table) *live.View {
	t.Helper()
	look(t, w, tb.player)
	return look(t, w, tb.dm)
}
