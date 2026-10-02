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

func TestReactionEdges(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Shield: true})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 2})
	ids := map[string]string{}
	var setup []live.CombatantSetup
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
		setup = append(setup, live.CombatantSetup{TokenID: tv.ID, SpeedFt: 30})
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: setup})
	tb.roll(combatant(d.View, "Aria"), w.player, 20)
	tb.roll(combatant(d.View, "Goblin"), w.dm, 10)
	fill := func(rollID string, faces ...int) {
		t.Helper()
		for i, f := range faces {
			if _, err := rolls.SetDie(ctx, dmCaller, w.session.CampaignID, domain.RollID(uuid.MustParse(rollID)), i, app.Fill{Value: f}); err != nil {
				t.Fatal(err)
			}
		}
	}
	drain := func(n int) live.Update {
		t.Helper()
		var u live.Update
		for range n {
			next(t, tb.player)
			u = next(t, tb.dm)
		}
		return u
	}

	u := tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: 4, R: -1})
	a := token(u.View, "Aria")
	if u.View.Combat.Prompt == nil || (a.Q == 0 && a.R == 0) || len(u.Steps) == 0 {
		t.Fatalf("the walk goes as far as leaving the goblin's reach = %+v %+v", a, u.View.Combat)
	}
	tb.dmSays(live.Command{Kind: live.CmdPlace, Label: "Crate", TokenKind: domain.TokenObject, Q: 4, R: -1})
	tb.dmSays(live.Command{Kind: live.CmdReact})
	u = drain(1)
	if b := token(u.View, "Aria"); b.Q != a.Q || b.R != a.R || u.View.Combat.Prompt != nil {
		t.Fatalf("the crate now blocks the rest of the walk = %+v", b)
	}

	u = tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: a.Q + 1, R: a.R - 1})
	if u.View == nil || u.View.Combat.Prompt == nil {
		t.Fatalf("leaving reach again from %d,%d = %+v", a.Q, a.R, u)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdReact, Use: true})
	fill(d.View.Combat.Attack.RollID, 20)
	u = drain(1)
	if u.View.Combat.Prompt != nil || u.View.Combat.Attack.Stage != domain.StageDamage || !u.View.Combat.Attack.Critical {
		t.Fatalf("no Shield against a critical = %+v", u.View.Combat)
	}
	fill(u.View.Combat.Attack.RollID, 6, 6)
	u = drain(2)
	if b := token(u.View, "Aria"); *b.HP != 0 || b.Q != a.Q {
		t.Fatalf("a fallen walker walks no further = %+v", b)
	}

	flush := func() {
		for _, sub := range []*live.Subscriber{tb.dm, tb.player} {
			for done := false; !done; {
				select {
				case <-sub.Out:
				case <-time.After(50 * time.Millisecond):
					done = true
				}
			}
		}
	}
	flush()
	tb.dmSays(live.Command{Kind: live.CmdUndoDamage})
	flush()
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(u.View, "Aria").ID})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdAttack, TokenID: ids["Goblin"], TargetID: ids["Aria"]})
	fill(d.View.Combat.Attack.RollID, 15)
	u = drain(1)
	if u.View.Combat.Prompt == nil || u.View.Combat.Prompt.Kind != domain.PromptShield {
		t.Fatalf("shield offered = %+v", u.View.Combat)
	}
	tb.playerSays(live.Command{Kind: live.CmdReact})
	u = drain(1)
	if at := u.View.Combat.Attack; at == nil || at.Stage != domain.StageDamage {
		t.Fatalf("declined, the hit stands = %+v", u.View.Combat)
	}
	fill(u.View.Combat.Attack.RollID, 1)
	drain(1)

	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Goblin").ID})
	u = tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: a.Q + 1, R: a.R - 1})
	if u.View.Combat.Prompt == nil {
		t.Fatalf("a third opportunity = %+v", u.View.Combat)
	}
	if _, p := tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: ids["Goblin"]}); p.View.Combat.Prompt != nil {
		t.Fatalf("removing the reactor ends its prompt = %+v", p.View.Combat)
	}
}
