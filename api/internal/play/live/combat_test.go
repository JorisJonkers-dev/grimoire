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

func combatant(v *live.View, label string) *live.CombatantView {
	if v.Combat == nil {
		return nil
	}
	for i := range v.Combat.Combatants {
		if v.Combat.Combatants[i].Label == label {
			return &v.Combat.Combatants[i]
		}
	}
	return nil
}

type table struct {
	t      *testing.T
	w      world
	rolls  *app.Rolls
	dm     *live.Subscriber
	player *live.Subscriber
	party  []live.Update
}

// dm sends a DM command and returns what the DM and the player see.
func (tb *table) dmSays(cmd live.Command) (live.Update, live.Update) {
	tb.t.Helper()
	tb.w.hub.Submit(tb.dm, cmd)
	d := next(tb.t, tb.dm)
	if d.Kind != live.UpdView {
		tb.t.Fatalf("%s rejected: %+v", cmd.Kind, d)
	}
	p := next(tb.t, tb.player)
	tb.party = append(tb.party, p)
	return d, p
}

// playerSays sends a player command; a view reaches the DM too.
func (tb *table) playerSays(cmd live.Command) live.Update {
	tb.t.Helper()
	tb.w.hub.Submit(tb.player, cmd)
	p := next(tb.t, tb.player)
	if p.Kind == live.UpdView {
		next(tb.t, tb.dm)
		tb.party = append(tb.party, p)
	}
	return p
}

// roll fills an initiative die on a Roll Card; the resolved roll reaches the Combat by itself.
func (tb *table) roll(c *live.CombatantView, who domain.Member, face int) {
	tb.t.Helper()
	cl := dmCaller
	if !who.DM {
		cl = playerCaller
	}
	id, _ := uuid.Parse(c.RollID)
	if _, err := tb.rolls.SetDie(context.Background(), cl, tb.w.session.CampaignID, domain.RollID(id), 0, app.Fill{Value: face}); err != nil {
		tb.t.Fatal(err)
	}
	next(tb.t, tb.dm)
	tb.party = append(tb.party, next(tb.t, tb.player))
}

func TestCombatTurnsAndTheActionEconomy(t *testing.T) {
	t.Parallel()
	w := setup(t)
	members := pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: members, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdPlace, Label: "Aria", TokenKind: domain.TokenParty, ControllerID: w.player.ID.String()})
	tb.dmSays(live.Command{Kind: live.CmdPlace, Label: "Brom", TokenKind: domain.TokenParty, Q: 1})
	tb.dmSays(live.Command{Kind: live.CmdPlace, Label: "Goblin", TokenKind: domain.TokenEnemy, Q: 3})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, Label: "Spy", TokenKind: domain.TokenEnemy, Q: -3, Hidden: true})
	ids := map[string]string{}
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
	}
	d, p := tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: []live.CombatantSetup{
		{TokenID: ids["Aria"], InitiativeBonus: 2, SpeedFt: 30},
		{TokenID: ids["Brom"], SpeedFt: 25},
		{TokenID: ids["Goblin"], InitiativeBonus: 1, SpeedFt: 30},
		{TokenID: ids["Spy"], InitiativeBonus: 3, SpeedFt: 30},
	}})
	if d.View.Combat.Status != domain.CombatRolling || len(d.View.Combat.Combatants) != 4 || len(p.View.Combat.Combatants) != 3 || combatant(p.View, "Spy") != nil {
		t.Fatalf("start = dm %+v party %+v", d.View.Combat, p.View.Combat)
	}
	aria := combatant(p.View, "Aria")
	if aria.ControllerID != w.player.ID.String() || aria.Initiative != nil || aria.Rank != 0 {
		t.Fatalf("aria before rolling = %+v", aria)
	}
	roll, _ := pgstore.New(w.pool).Roll(context.Background(), w.session.CampaignID, domain.RollID(uuid.MustParse(aria.RollID)))
	if roll.Roller.ID != w.player.ID || roll.Purpose != "Initiative for Aria" || roll.Modifiers[0].Value != 2 {
		t.Fatalf("aria's roll = %+v", roll)
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: 0, R: 1}); !strings.Contains(u.Reason, "not Aria's turn") {
		t.Fatalf("walking before initiative = %+v", u)
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdSpend, CombatantID: aria.ID, Resource: live.ResourceReaction}); u.Reason != "Roll initiative first." {
		t.Fatalf("reaction before initiative = %+v", u)
	}

	tb.roll(aria, w.player, 15)
	tb.roll(combatant(d.View, "Brom"), w.dm, 17)
	tb.roll(combatant(d.View, "Goblin"), w.dm, 9)
	w.hub.Close(w.session.ID)
	id, _ := uuid.Parse(combatant(d.View, "Spy").RollID)
	if _, err := rolls.SetDie(context.Background(), dmCaller, w.session.CampaignID, domain.RollID(id), 0, app.Fill{Value: 4}); err != nil {
		t.Fatal(err)
	}
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	w.hub.Submit(tb.player, live.Command{Kind: live.CmdResync})
	p = next(t, tb.player)
	c := p.View.Combat
	if c.Status != domain.CombatActive || c.Round != 1 || c.Combatants[0].Rank != 1 || c.Combatants[1].Rank != 1 || combatant(p.View, "Goblin").Rank != 2 {
		t.Fatalf("initiative order after a restart = %+v", c)
	}
	aria = combatant(p.View, "Aria")
	if !aria.Acting || !combatant(p.View, "Brom").Acting || combatant(p.View, "Goblin").Acting || *aria.Initiative != 17 || aria.MovementFt != 30 || !aria.Action {
		t.Fatalf("tied turns act together = %+v", c)
	}

	if u := tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: 0, R: 2}); combatant(u.View, "Aria").MovementFt != 20 {
		t.Fatalf("walk spends movement = %+v", u)
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdPlanWalk, TokenID: ids["Aria"], Q: 0, R: 7}); u.Reason != "Aria has 20 ft of movement left." {
		t.Fatalf("walk beyond speed = %+v", u)
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdSpend, CombatantID: aria.ID, Resource: live.ResourceAction}); combatant(u.View, "Aria").Action {
		t.Fatalf("spend action = %+v", u)
	}
	refused := map[string]live.Command{
		"already spent":     {Kind: live.CmdSpend, CombatantID: aria.ID, Resource: live.ResourceAction},
		"bonus action or":   {Kind: live.CmdSpend, CombatantID: aria.ID, Resource: "dance"},
		"not yours to play": {Kind: live.CmdEndTurn, CombatantID: combatant(p.View, "Brom").ID},
		"no such combatant": {Kind: live.CmdEndTurn, CombatantID: "nope"},
		"only the dm":       {Kind: live.CmdEndCombat},
	}
	for want, cmd := range refused {
		if u := tb.playerSays(cmd); u.Kind != live.UpdRejected || !strings.Contains(strings.ToLower(u.Reason), want) {
			t.Errorf("%s = %+v", want, u)
		}
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: aria.ID}); !combatant(u.View, "Aria").Done || !combatant(u.View, "Brom").Acting {
		t.Fatalf("one of a tie ends its turn = %+v", u.View.Combat)
	}
	for _, cmd := range []live.Command{
		{Kind: live.CmdEndTurn, CombatantID: aria.ID}, {Kind: live.CmdSpend, CombatantID: aria.ID, Resource: live.ResourceBonusAction},
	} {
		if u := tb.playerSays(cmd); u.Reason != "It is not Aria's turn." {
			t.Fatalf("%s after the turn = %+v", cmd.Kind, u)
		}
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdSpend, CombatantID: aria.ID, Resource: live.ResourceReaction}); combatant(u.View, "Aria").Reaction {
		t.Fatalf("a reaction off turn = %+v", u)
	}
	_, p = tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(p.View, "Brom").ID})
	if g := combatant(p.View, "Goblin"); !g.Acting || g.MovementFt != 30 {
		t.Fatalf("the next count acts = %+v", p.View.Combat)
	}
	d, p = tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(p.View, "Goblin").ID})
	if !combatant(d.View, "Spy").Acting || p.View.Combat.Combatants[0].Acting || p.View.Combat.Combatants[2].Acting {
		t.Fatalf("a hidden combatant's turn = dm %+v party %+v", d.View.Combat, p.View.Combat)
	}
	_, p = tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Spy").ID})
	if aria = combatant(p.View, "Aria"); p.View.Combat.Round != 2 || !aria.Acting || !aria.Action || !aria.Reaction || aria.MovementFt != 30 {
		t.Fatalf("a new round = %+v", p.View.Combat)
	}
	if body := payloads(t, tb.party...); strings.Contains(body, "Spy") || strings.Contains(body, ids["Spy"]) {
		t.Fatalf("a hidden combatant reached the party: %s", body)
	}

	_, p = tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: ids["Brom"]})
	if len(p.View.Combat.Combatants) != 2 || !combatant(p.View, "Aria").Acting {
		t.Fatalf("a removed combatant = %+v", p.View.Combat)
	}
	_, p = tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: ids["Aria"]})
	if !combatant(p.View, "Goblin").Acting {
		t.Fatalf("removing the one acting moves the turn on = %+v", p.View.Combat)
	}
	tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: ids["Goblin"]})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: ids["Spy"]})
	if d.View.Combat == nil || len(d.View.Combat.Combatants) != 0 {
		t.Fatalf("an empty fight = %+v", d.View.Combat)
	}
	if d, p = tb.dmSays(live.Command{Kind: live.CmdEndCombat}); d.View.Combat != nil || p.View.Combat != nil {
		t.Fatalf("end = %+v", d.View)
	}
	w.hub.RollResolved(uuid.New(), domain.RollID(id))
	w.hub.RollResolved(w.session.CampaignID, domain.RollID(id))
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdEndTurn})
	if u := next(t, tb.dm); u.Reason != "There is no fight on." {
		t.Fatalf("no fight = %+v", u)
	}
}

func TestStartingCombatIsValidated(t *testing.T) {
	t.Parallel()
	w := setup(t)
	dm := join(t, w, w.dm, dmCaller, live.AudienceDM)
	w.hub.Submit(dm, live.Command{Kind: live.CmdPlace, Label: "Aria", TokenKind: domain.TokenParty, ControllerID: w.player.ID.String()})
	id := token(next(t, dm).View, "Aria").ID
	bad := map[string][]live.CombatantSetup{
		"none":    nil,
		"unknown": {{TokenID: uuid.NewString(), SpeedFt: 30}},
		"twice":   {{TokenID: id, SpeedFt: 30}, {TokenID: id, SpeedFt: 30}},
		"bonus":   {{TokenID: id, InitiativeBonus: 21, SpeedFt: 30}},
		"low":     {{TokenID: id, InitiativeBonus: -11, SpeedFt: 30}},
		"speed":   {{TokenID: id, SpeedFt: 121}},
		"slow":    {{TokenID: id, SpeedFt: -1}},
	}
	for name, setup := range bad {
		w.hub.Submit(dm, live.Command{Nonce: name, Kind: live.CmdStartCombat, Combatants: setup})
		if u := next(t, dm); u.Kind != live.UpdRejected {
			t.Errorf("%s = %+v", name, u)
		}
	}
	w.hub.Submit(dm, live.Command{Kind: live.CmdStartCombat, Combatants: []live.CombatantSetup{{TokenID: id, SpeedFt: 30}}})
	u := next(t, dm)
	roll, _ := pgstore.New(w.pool).Roll(context.Background(), w.session.CampaignID, domain.RollID(uuid.MustParse(u.View.Combat.Combatants[0].RollID)))
	if roll.Roller.ID != w.player.ID || len(roll.Modifiers) != 0 {
		t.Fatalf("roll without a bonus = %+v", roll)
	}
	w.hub.Submit(dm, live.Command{Kind: live.CmdStartCombat, Combatants: []live.CombatantSetup{{TokenID: id, SpeedFt: 30}}})
	if u := next(t, dm); u.Reason != "A fight is already on." {
		t.Fatalf("second fight = %+v", u)
	}
	w.hub.RollResolved(w.session.CampaignID, domain.RollID(uuid.MustParse(u.View.Combat.Combatants[0].RollID)))
	w.hub.Submit(dm, live.Command{Kind: live.CmdSpend, CombatantID: u.View.Combat.Combatants[0].ID, Resource: live.ResourceAction})
	if u := next(t, dm); u.Kind != live.UpdRejected {
		t.Fatalf("a pending roll does not start the fight = %+v", u)
	}
}
