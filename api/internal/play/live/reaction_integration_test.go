//go:build integration

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

func TestReactionsPauseTheFight(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	if _, err := w.pool.Exec(ctx, "UPDATE campaign.campaigns SET reaction_timeout_s = 3 WHERE id = $1", w.session.CampaignID); err != nil {
		t.Fatal(err)
	}
	w.hub.Stats = bestiary{owner: w.player.ID}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Shield: true})
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Label: "Brom", Q: 2, R: -1})
	d, p := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 1})
	if !token(p.View, "Aria").Shield || token(p.View, "Brom").Shield {
		t.Fatalf("who knows Shield = %+v", p.View.Tokens)
	}
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
	if u := tb.playerSays(live.Command{Kind: live.CmdReact}); u.Reason != "Nothing is waiting on a reaction." {
		t.Fatalf("react with nothing asked = %+v", u)
	}

	u := tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: -2, R: 0})
	pr := u.View.Combat.Prompt
	if pr == nil || pr.Kind != domain.PromptOpportunity || pr.ReactorID != ids["Goblin"] || pr.SecondsLeft != 3 || token(u.View, "Aria").Q != 0 ||
		pr.Effect != "Aria leaves Goblin's reach: Scimitar attack (+4) before they go on." {
		t.Fatalf("leaving the goblin's reach = %+v", u.View.Combat)
	}
	for want, cmd := range map[string]live.Command{
		"The fight waits for a reaction.":     {Kind: live.CmdEndTurn, CombatantID: combatant(u.View, "Aria").ID},
		"That reaction is not yours to take.": {Kind: live.CmdReact, Use: true},
	} {
		if u := tb.playerSays(cmd); u.Reason != want {
			t.Errorf("%s = %+v", want, u)
		}
	}
	tb.dmSays(live.Command{Kind: live.CmdReact})
	p = next(t, tb.player)
	next(t, tb.dm)
	if a := token(p.View, "Aria"); a.Q != -2 || p.View.Combat.Prompt != nil || combatant(p.View, "Aria").MovementFt != 20 || !combatant(p.View, "Goblin").Reaction {
		t.Fatalf("declined, the walk goes on = %+v %+v", a, p.View.Combat)
	}

	tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: 0, R: 0})
	u = tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: -1, R: 0})
	if u.View.Combat.Prompt == nil {
		t.Fatalf("leaving again = %+v", u.View.Combat)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdReact, Use: true})
	oa := d.View.Combat.Attack
	if oa == nil || oa.AttackerID != ids["Goblin"] || oa.Stage != domain.StageToHit || combatant(d.View, "Goblin").Reaction || d.View.Combat.Prompt != nil {
		t.Fatalf("the opportunity attack = %+v", d.View.Combat)
	}
	id := domain.RollID(uuid.MustParse(oa.RollID))
	if _, err := rolls.SetDie(ctx, dmCaller, w.session.CampaignID, id, 0, app.Fill{Value: 15}); err != nil {
		t.Fatal(err)
	}
	next(t, tb.dm)
	p = next(t, tb.player)
	shield := p.View.Combat.Prompt
	if shield == nil || shield.Kind != domain.PromptShield || shield.ReactorID != ids["Aria"] || shield.Effect != "Shield: AC 16 → 21, so the attack (19) would miss." ||
		p.View.Combat.Attack.Stage != domain.StageReaction {
		t.Fatalf("a hit Aria can shield = %+v", p.View.Combat)
	}
	u = tb.playerSays(live.Command{Kind: live.CmdReact, Use: true})
	for range 2 {
		next(t, tb.dm)
		u = next(t, tb.player)
	}
	if a := token(u.View, "Aria"); a.Q != -1 || *a.HP != 12 || u.View.Combat.Attack != nil || combatant(u.View, "Aria").Reaction {
		t.Fatalf("shielded, the attack misses and the walk goes on = %+v %+v", a, u.View.Combat)
	}
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(u.View, "Aria").ID})
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(u.View, "Brom").ID})
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdPreviewAttack, TokenID: ids["Goblin"], AttackNo: 3, TargetID: ids["Aria"]})
	if u := next(t, tb.dm); u.Preview == nil || !strings.Contains(strings.Join(u.Preview.Reasons, "|"), "|Shield: +5 to the target's AC|") || u.Preview.HitChance != 4 {
		t.Fatalf("shield holds until Aria's next turn = %+v", u)
	}

	d, _ = tb.dmSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Goblin"], Q: -1, R: 2})
	if pr := d.View.Combat.Prompt; pr == nil || pr.ReactorID != ids["Brom"] {
		t.Fatalf("leaving Brom's reach = %+v", d.View.Combat)
	}
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdResync})
	if u := next(t, tb.dm); u.View.Combat.Prompt == nil || u.View.Combat.Prompt.ReactorID != ids["Brom"] {
		t.Fatalf("the prompt survives a restart = %+v", u.View.Combat)
	}
	var last live.Update
	for range 2 {
		last = next(t, tb.dm)
	}
	if g := token(last.View, "Goblin"); g.Q != -1 || g.R != 2 || last.View.Combat.Prompt != nil || !combatant(last.View, "Brom").Reaction {
		t.Fatalf("nobody answered: declined, and the goblin walks on = %+v %+v", g, last.View.Combat)
	}
	log, err := pgstore.New(w.pool).ActionLog(ctx, w.session.CampaignID, 200)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, a := range log {
		kinds[a.Kind]++
	}
	if kinds[domain.ActionReactionOffered] != 4 || kinds[domain.ActionReactionUsed] != 2 || kinds[domain.ActionReactionDeclined] != 2 {
		t.Fatalf("the Action Log records every reaction: %v", kinds)
	}
	if !strings.Contains(payloads(t, tb.party...), "leaves Goblin's reach") {
		t.Fatal("the party saw the goblin's prompt")
	}
}

// edges is a fight between Aria, with Shield, and a goblin beside her; each step plays one beat of
// it, so a test replays the beats before the one it checks.
type edges struct {
	t     *testing.T
	w     world
	tb    *table
	rolls *app.Rolls
	ids   map[string]string
	start live.View
	q, r  int
	last  live.Update
}

func newEdges(t *testing.T) *edges {
	t.Helper()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Shield: true})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 2})
	e := &edges{t: t, w: w, tb: tb, rolls: rolls, ids: map[string]string{}}
	var setup []live.CombatantSetup
	for _, tv := range d.View.Tokens {
		e.ids[tv.Label] = tv.ID
		setup = append(setup, live.CombatantSetup{TokenID: tv.ID, SpeedFt: 30})
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: setup})
	tb.roll(combatant(d.View, "Aria"), w.player, 20)
	tb.roll(combatant(d.View, "Goblin"), w.dm, 10)
	e.start = *d.View
	return e
}

func (e *edges) fill(rollID string, faces ...int) {
	e.t.Helper()
	for i, f := range faces {
		if _, err := e.rolls.SetDie(context.Background(), dmCaller, e.w.session.CampaignID, domain.RollID(uuid.MustParse(rollID)), i, app.Fill{Value: f}); err != nil {
			e.t.Fatal(err)
		}
	}
}

func (e *edges) drain(n int) live.Update {
	e.t.Helper()
	var u live.Update
	for range n {
		next(e.t, e.tb.player)
		u = next(e.t, e.tb.dm)
	}
	return u
}

// walkOut has Aria walk away past the goblin, which stops her where she leaves its reach.
func (e *edges) walkOut() live.Update {
	e.t.Helper()
	u := e.tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: e.ids["Aria"], Q: 4, R: -1})
	a := token(u.View, "Aria")
	e.q, e.r = a.Q, a.R
	return u
}

// blockedAndDeclined drops a crate where she was going, and the goblin lets her go.
func (e *edges) blockedAndDeclined() live.Update {
	e.t.Helper()
	e.tb.dmSays(live.Command{Kind: live.CmdPlace, Label: "Crate", TokenKind: domain.TokenObject, Q: 4, R: -1})
	e.tb.dmSays(live.Command{Kind: live.CmdReact})
	return e.drain(1)
}

// leavesAgain has her step out of its reach once more.
func (e *edges) leavesAgain() live.Update {
	e.t.Helper()
	return e.tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: e.ids["Aria"], Q: e.q + 1, R: e.r - 1})
}

// critted has the goblin take its opportunity attack and roll a 20.
func (e *edges) critted() live.Update {
	e.t.Helper()
	d, _ := e.tb.dmSays(live.Command{Kind: live.CmdReact, Use: true})
	e.fill(d.View.Combat.Attack.RollID, 20)
	return e.drain(1)
}

// felled is the critical's damage, enough to drop her.
func (e *edges) felled(crit live.Update) live.Update {
	e.t.Helper()
	e.fill(crit.View.Combat.Attack.RollID, 6, 6)
	return e.drain(2)
}

func (e *edges) flush() {
	for _, sub := range []*live.Subscriber{e.tb.dm, e.tb.player} {
		for done := false; !done; {
			select {
			case <-sub.Out:
			case <-time.After(250 * time.Millisecond):
				done = true
			}
		}
	}
}

// attackedOnItsTurn undoes the damage, ends Aria's turn and has the goblin hit her with a 15.
func (e *edges) attackedOnItsTurn(fallen live.Update) live.Update {
	e.t.Helper()
	e.flush()
	e.tb.dmSays(live.Command{Kind: live.CmdUndoDamage})
	e.flush()
	e.tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(fallen.View, "Aria").ID})
	d, _ := e.tb.dmSays(live.Command{Kind: live.CmdAttack, TokenID: e.ids["Goblin"], TargetID: e.ids["Aria"]})
	e.fill(d.View.Combat.Attack.RollID, 15)
	return e.drain(1)
}

// toShield plays the fight up to the goblin's hit that Aria may Shield against.
func (e *edges) toShield() live.Update {
	e.t.Helper()
	e.walkOut()
	e.blockedAndDeclined()
	e.leavesAgain()
	return e.attackedOnItsTurn(e.felled(e.critted()))
}

func TestAWalkGoesAsFarAsLeavingAReactorsReachAndWaits(t *testing.T) {
	t.Parallel()
	e := newEdges(t)
	u := e.walkOut()
	if u.View.Combat.Prompt == nil || (e.q == 0 && e.r == 0) || len(u.Steps) == 0 {
		t.Fatalf("Aria at %d,%d, combat %+v", e.q, e.r, u.View.Combat)
	}
}

func TestADeclinedWalkGoesOnUntilWhatNowBlocksIt(t *testing.T) {
	t.Parallel()
	e := newEdges(t)
	e.walkOut()
	if u := e.blockedAndDeclined(); token(u.View, "Aria").Q != e.q || token(u.View, "Aria").R != e.r || u.View.Combat.Prompt != nil {
		t.Fatalf("Aria = %+v", token(u.View, "Aria"))
	}
}

func TestLeavingReachAgainPromptsAgain(t *testing.T) {
	t.Parallel()
	e := newEdges(t)
	e.walkOut()
	e.blockedAndDeclined()
	if u := e.leavesAgain(); u.View == nil || u.View.Combat.Prompt == nil {
		t.Fatalf("from %d,%d = %+v", e.q, e.r, u)
	}
}

func TestShieldIsNotOfferedAgainstACriticalHit(t *testing.T) {
	t.Parallel()
	e := newEdges(t)
	e.walkOut()
	e.blockedAndDeclined()
	e.leavesAgain()
	if u := e.critted(); u.View.Combat.Prompt != nil || u.View.Combat.Attack.Stage != domain.StageDamage || !u.View.Combat.Attack.Critical {
		t.Fatalf("combat = %+v", u.View.Combat)
	}
}

func TestAFallenWalkerWalksNoFurther(t *testing.T) {
	t.Parallel()
	e := newEdges(t)
	e.walkOut()
	e.blockedAndDeclined()
	e.leavesAgain()
	if b := token(e.felled(e.critted()).View, "Aria"); *b.HP != 0 || b.Q != e.q {
		t.Fatalf("Aria = %+v", b)
	}
}

func TestShieldIsOfferedAgainstAHit(t *testing.T) {
	t.Parallel()
	if u := newEdges(t).toShield(); u.View.Combat.Prompt == nil || u.View.Combat.Prompt.Kind != domain.PromptShield {
		t.Fatalf("combat = %+v", u.View.Combat)
	}
}

func TestDecliningShieldLetsTheHitStand(t *testing.T) {
	t.Parallel()
	e := newEdges(t)
	e.toShield()
	e.tb.playerSays(live.Command{Kind: live.CmdReact})
	if at := e.drain(1).View.Combat.Attack; at == nil || at.Stage != domain.StageDamage {
		t.Fatalf("attack = %+v", at)
	}
}

// nextTurnOut plays on to Aria's next turn, where she leaves the goblin's reach once more.
func (e *edges) nextTurnOut() live.Update {
	e.t.Helper()
	e.toShield()
	e.tb.playerSays(live.Command{Kind: live.CmdReact})
	u := e.drain(1)
	e.fill(u.View.Combat.Attack.RollID, 1)
	e.drain(1)
	e.tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(&e.start, "Goblin").ID})
	return e.tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: e.ids["Aria"], Q: e.q + 1, R: e.r - 1})
}

func TestANewRoundBringsANewOpportunity(t *testing.T) {
	t.Parallel()
	if u := newEdges(t).nextTurnOut(); u.View.Combat.Prompt == nil {
		t.Fatalf("combat = %+v", u.View.Combat)
	}
}

func TestRemovingTheReactorEndsItsPrompt(t *testing.T) {
	t.Parallel()
	e := newEdges(t)
	e.nextTurnOut()
	if _, p := e.tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: e.ids["Goblin"]}); p.View.Combat.Prompt != nil {
		t.Fatalf("combat = %+v", p.View.Combat)
	}
}
