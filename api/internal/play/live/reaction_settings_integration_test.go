//go:build integration

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
// settingsTable is a fight between Aria, with Shield, Brom, and a goblin, in that order.
type settingsTable struct {
	t   *testing.T
	w   world
	tb  *table
	ids map[string]string
}

func newSettingsTable(t *testing.T) *settingsTable {
	t.Helper()
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
	s := &settingsTable{t: t, w: w, tb: tb, ids: map[string]string{}}
	var setup []live.CombatantSetup
	for _, tv := range d.View.Tokens {
		s.ids[tv.Label] = tv.ID
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
	return s
}

func (s *settingsTable) set(sub *live.Subscriber, token, kind, mode, condition string) live.Update {
	s.t.Helper()
	s.w.hub.Submit(sub, live.Command{Kind: live.CmdSetReaction, TokenID: s.ids[token], ReactionKind: kind, ReactionMode: mode, Condition: condition})
	u := next(s.t, sub)
	if u.Kind == live.UpdView {
		other := s.tb.player
		if sub == s.tb.player {
			other = s.tb.dm
		}
		next(s.t, other)
	}
	return u
}

// set is the table with the goblin set never to take opportunity attacks and Aria's Shield always up.
func (s *settingsTable) setUp() live.Update {
	s.t.Helper()
	s.set(s.tb.dm, "Goblin", "opportunity_attack", "never", "")
	return s.set(s.tb.player, "Aria", "shield", "always", "")
}

func TestAReactionSettingOutsideTheRulesIsRefused(t *testing.T) {
	t.Parallel()
	s := newSettingsTable(t)
	for want, c := range map[string][4]string{
		"Choose a reaction":  {"Aria", "dance", "always", ""},
		"ask, always":        {"Aria", "shield", "often", ""},
		"Choose a reaction,": {"Aria", "shield", "always", "moonlit"},
		"not yours":          {"Goblin", "opportunity_attack", "never", ""},
	} {
		t.Run(want, func(t *testing.T) {
			if u := s.set(s.tb.player, c[0], c[1], c[2], c[3]); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, strings.TrimSuffix(want, ",")) {
				t.Fatalf("%+v", u)
			}
		})
	}
}

func TestThePartySeesItsOwnReactionSettingsButNeverAFoes(t *testing.T) {
	t.Parallel()
	p := newSettingsTable(t).setUp()
	if r := token(p.View, "Aria").Reactions; len(r) != 1 || r[0] != (live.ReactionSettingView{Kind: "shield", Mode: "always"}) {
		t.Fatalf("Aria's settings = %+v", r)
	}
	if token(p.View, "Goblin").Reactions != nil {
		t.Fatal("the party sees the goblin's settings")
	}
}

func TestAFoeThatNeverTakesOpportunityAttacksLetsAWalkerGo(t *testing.T) {
	t.Parallel()
	s := newSettingsTable(t)
	s.setUp()
	if p := s.tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: s.ids["Aria"], Q: -2, R: 0}); p.View.Combat.Prompt != nil || token(p.View, "Aria").Q != -2 {
		t.Fatalf("prompt = %+v", p.View.Combat.Prompt)
	}
}

// goblinsTurn plays on to the goblin's turn, with Aria walked away.
func (s *settingsTable) goblinsTurn() {
	s.t.Helper()
	s.setUp()
	p := s.tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: s.ids["Aria"], Q: -2, R: 0})
	s.tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(p.View, "Aria").ID})
	s.tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(p.View, "Brom").ID})
}

func TestAShieldSetToAlwaysGoesUpByItself(t *testing.T) {
	t.Parallel()
	s := newSettingsTable(t)
	s.goblinsTurn()
	s.tb.dmSays(live.Command{Kind: live.CmdWalk, TokenID: s.ids["Goblin"], Q: -1, R: 0})
	d, _ := s.tb.dmSays(live.Command{Kind: live.CmdAttack, TokenID: s.ids["Goblin"], TargetID: s.ids["Aria"]})
	s.tb.fill(d.View.Combat.Attack.RollID, s.w.dm, 13)
	v := barrier(t, s.w, s.tb)
	if aria := combatant(v, "Aria"); aria.Reaction || v.Combat.Attack != nil || v.Combat.Prompt != nil || *token(v, "Aria").HP != 12 {
		t.Fatalf("Aria %+v, combat %+v", aria, v.Combat)
	}
}

// rebuked plays on to Aria's next attack, whose damage sets off the goblin's Hellish Rebuke.
func (s *settingsTable) rebuked() *live.View {
	s.t.Helper()
	s.goblinsTurn()
	s.tb.dmSays(live.Command{Kind: live.CmdWalk, TokenID: s.ids["Goblin"], Q: -1, R: 0})
	d, _ := s.tb.dmSays(live.Command{Kind: live.CmdAttack, TokenID: s.ids["Goblin"], TargetID: s.ids["Aria"]})
	s.tb.fill(d.View.Combat.Attack.RollID, s.w.dm, 13)
	v := barrier(s.t, s.w, s.tb)
	s.tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(v, "Goblin").ID})
	s.tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: s.ids["Goblin"], Effect: "hellish-rebuke"})
	p := s.tb.playerSays(live.Command{Kind: live.CmdAttack, TokenID: s.ids["Aria"], TargetID: s.ids["Goblin"]})
	s.tb.fill(p.View.Combat.Attack.RollID, s.w.player, 15)
	barrier(s.t, s.w, s.tb)
	s.tb.fill(look(s.t, s.w, s.tb.player).Combat.Attack.RollID, s.w.player, 1)
	return barrier(s.t, s.w, s.tb)
}

func TestDamageSetsOffAFoesReactionEffect(t *testing.T) {
	t.Parallel()
	s := newSettingsTable(t)
	if pr := s.rebuked().Combat.Prompt; pr == nil || pr.Kind != "effect" || pr.ReactorID != s.ids["Goblin"] || !strings.HasPrefix(pr.Effect, "Hellish Rebuke:") {
		t.Fatalf("prompt = %+v", pr)
	}
}

func TestTheDMResolvesAFoesReactionEffectByHand(t *testing.T) {
	t.Parallel()
	s := newSettingsTable(t)
	s.rebuked()
	d, _ := s.tb.dmSays(live.Command{Kind: live.CmdReact, Use: true})
	if m := d.View.Manual; len(m) == 0 || !strings.HasPrefix(m[len(m)-1].Text, "Goblin: Hellish Rebuke:") || combatant(d.View, "Goblin").Reaction {
		t.Fatalf("manual = %+v", d.View.Manual)
	}
}

func TestReactionSettingsSurviveARestart(t *testing.T) {
	t.Parallel()
	s := newSettingsTable(t)
	s.set(s.tb.dm, "Goblin", "opportunity_attack", "always", "target_bloodied")
	s.w.hub.Close(s.w.session.ID)
	s.tb.dm = join(t, s.w, s.w.dm, dmCaller, live.AudienceDM)
	if r := token(look(t, s.w, s.tb.dm), "Goblin").Reactions; len(r) != 1 || r[0].Condition != "target_bloodied" {
		t.Fatalf("settings = %+v", r)
	}
}

// barrier reads both subscribers up to a fresh snapshot, so no broadcast from a roll that just
// resolved is left queued; a quiet-window drain can return before a late one lands.
func barrier(t *testing.T, w world, tb *table) *live.View {
	t.Helper()
	look(t, w, tb.player)
	return look(t, w, tb.dm)
}
