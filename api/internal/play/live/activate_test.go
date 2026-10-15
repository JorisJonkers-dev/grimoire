package live_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
)

// activationTable is the shaped table, which stores two chosen-target Effects: a blessing on up to three
// creatures within 30 feet, and a ward on one willing creature by touch.
func activationTable(t *testing.T) (world, *table, map[string]string) {
	t.Helper()
	return shapedTable(t)
}

// An Activation offers only legal targets and refuses the rest with the reason; the Effect lands on
// every target with its caster as source, its duration and its concentration.
func TestAnActivationLandsOnLegalTargetsOnly(t *testing.T) {
	t.Parallel()
	_, tb, ids := activationTable(t)
	bless := live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "blessing", Targets: []string{ids["Aria"], ids["Goblin"]}}
	u := tb.playerSays(bless)
	if u.View == nil {
		t.Fatalf("blessing = %+v", u)
	}
	for _, label := range []string{"Aria", "Goblin"} {
		e := effect(token(u.View, label), "Blessing")
		if e == nil || e.SourceID != ids["Aria"] || !e.Concentration || e.RoundsLeft != 10 {
			t.Fatalf("%s blessed = %+v", label, token(u.View, label).Effects)
		}
	}
	for _, c := range []struct {
		cmd  live.Command
		want string
	}{
		{live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "ward", Targets: []string{ids["Aria"], ids["Goblin"]}}, "Choose at most 1 target."},
		{live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "ward", Targets: []string{ids["Goblin"]}}, "Goblin: Out of range: the range is touch."},
		{live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "ward", Targets: []string{ids["Goblin"]}, Force: true}, "Goblin: Out of range: the range is touch."},
		{live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "ward", Targets: []string{uuid.NewString()}}, "Choose targets on the map."},
		{live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "hold-fast", Targets: []string{ids["Goblin"]}}, "Hold Fast states no Targeting; the DM applies it by hand."},
		{live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "frost-burst", Targets: []string{ids["Goblin"]}}, "Cast Frost Burst as an area spell."},
		{live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "nothing"}, "That is not an Effect the rules know."},
		{live.Command{Kind: live.CmdActivate, TokenID: ids["Goblin"], Effect: "ward", Targets: []string{ids["Goblin"]}}, "That token is not yours to play."},
	} {
		if u := tb.playerSays(c.cmd); u.Reason != c.want {
			t.Errorf("%+v: %+v", c.cmd, u.Reason)
		}
	}
}

// The DM may force an illegal pick; the Action Log records the override apart from an ordinary
// Activation.
func TestTheDMMayForceAnIllegalTarget(t *testing.T) {
	t.Parallel()
	w, tb, ids := activationTable(t)
	tb.w.hub.Submit(tb.dm, live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "ward", Targets: []string{ids["Goblin"]}})
	if u := next(t, tb.dm); u.Reason != "Goblin: Out of range: the range is touch." {
		t.Fatalf("the DM without forcing = %+v", u.Reason)
	}
	d, _ := tb.dmSays(live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "ward", Targets: []string{ids["Goblin"]}, Force: true})
	if effect(token(d.View, "Goblin"), "Ward") == nil {
		t.Fatalf("forced ward = %+v", token(d.View, "Goblin").Effects)
	}
	tb.dmSays(live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "ward", Targets: []string{ids["Aria"]}})
	log, err := pgstore.New(w.pool).ActionLog(context.Background(), w.session.CampaignID, 50)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, a := range log {
		kinds[a.Kind]++
	}
	if kinds[domain.ActionActivationForced] != 1 || kinds[domain.ActionActivated] != 1 {
		t.Fatalf("the Action Log = %v", kinds)
	}
}

// In combat an Activation takes the caster's action on its turn, and only once.
func TestAnActivationTakesTheAction(t *testing.T) {
	t.Parallel()
	_, tb, ids := activationTable(t)
	tb.fight(ids, "Aria", "Goblin")
	u := tb.playerSays(live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "ward", Targets: []string{ids["Aria"]}})
	if u.View == nil || combatant(u.View, "Aria").Action {
		t.Fatalf("ward in combat = %+v", u)
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "blessing", Targets: []string{ids["Aria"]}}); u.Reason != "Aria has already used their action." {
		t.Fatalf("a second activation = %+v", u.Reason)
	}
}

// Sight is the caster's own; an Effect that does more as it lands, one with modes, and one cast below its
// lowest slot are refused; a new concentration Effect ends the caster's last; and only the acting
// creature activates in combat.
func TestActivationChecks(t *testing.T) {
	t.Parallel()
	_, tb, ids := activationTable(t)
	tb.dmSays(live.Command{Kind: live.CmdSetVisibility, TokenID: ids["Goblin"], Qualities: []string{"invisible"}})
	glare := live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "glare", Targets: []string{ids["Goblin"]}}
	if u := tb.playerSays(glare); u.Reason != "Choose targets on the map." {
		t.Fatalf("a player picks a creature the party cannot see = %+v", u.Reason)
	}
	tb.w.hub.Submit(tb.dm, glare)
	if u := next(t, tb.dm); u.Reason != "Goblin: Choose a creature you can see." {
		t.Fatalf("the DM picks for a caster who cannot see = %+v", u.Reason)
	}
	tb.dmSays(live.Command{Kind: live.CmdSetVisibility, TokenID: ids["Goblin"], Qualities: []string{}})
	tb.dmSays(live.Command{Kind: live.CmdSetHidden, TokenID: ids["Goblin"], Hidden: true})
	tb.w.hub.Submit(tb.dm, glare)
	if u := next(t, tb.dm); u.Reason != "Goblin: Choose a creature you can see." {
		t.Fatalf("the DM picks a hidden creature = %+v", u.Reason)
	}
	tb.dmSays(live.Command{Kind: live.CmdSetHidden, TokenID: ids["Goblin"], Hidden: false})
	for _, c := range []struct {
		cmd  live.Command
		want string
	}{
		{live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "glare", Slot: 1, Targets: []string{ids["Aria"]}}, "Cast Glare with a slot of level 2 to 9."},
		{live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "boon", Targets: []string{ids["Aria"]}}, "Boon does more as it lands than an Activation can yet; the DM applies it by hand."},
		{live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "twin-ward"}, "Choose Fast or Slow."},
		{live.Command{Kind: live.CmdActivate, TokenID: uuid.NewString(), Effect: "ward"}, "No such caster."},
	} {
		if u := tb.playerSays(c.cmd); u.Reason != c.want {
			t.Errorf("%s: %q", c.cmd.Effect, u.Reason)
		}
	}
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Goblin"], Q: 1, R: 0})
	tb.dmSays(live.Command{Kind: live.CmdSetVisibility, TokenID: ids["Goblin"], Qualities: []string{"disguised"}, Disguise: "Merchant"})
	ward := live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "ward", Targets: []string{ids["Goblin"]}}
	if u := tb.playerSays(ward); u.Reason != "Merchant: Choose a willing creature." {
		t.Fatalf("a disguised foe is not willing = %+v", u.Reason)
	}
	tb.w.hub.Submit(tb.dm, ward)
	if u := next(t, tb.dm); u.Reason != "Goblin: Choose a willing creature." {
		t.Fatalf("the DM sees the goblin = %+v", u.Reason)
	}
	tb.dmSays(live.Command{Kind: live.CmdSetVisibility, TokenID: ids["Goblin"], Qualities: []string{}})
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Goblin"], Q: 3, R: 0})
	u := tb.playerSays(live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "twin-ward", EffectMode: "Slow"})
	if e := effect(token(u.View, "Aria"), "Twin Ward"); u.View == nil || e == nil || e.Mode != "Slow" {
		t.Fatalf("twin ward = %+v", u)
	}
	tb.playerSays(live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "blessing", Targets: []string{ids["Aria"]}})
	u = tb.playerSays(live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "blessing", Targets: []string{ids["Aria"]}})
	blessed := 0
	for _, e := range token(u.View, "Aria").Effects {
		if e.Name == "Blessing" {
			blessed++
		}
	}
	if blessed != 1 {
		t.Fatalf("a new concentration Effect ends the last = %+v", token(u.View, "Aria").Effects)
	}
	tb.dmSays(live.Command{Kind: live.CmdSetVisibility, TokenID: ids["Goblin"], Qualities: []string{}})
	tb.fight(ids, "Goblin", "Aria")
	if u := tb.playerSays(live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "ward", Targets: []string{ids["Aria"]}}); u.Reason != "It is not Aria's turn." {
		t.Fatalf("out of turn = %+v", u.Reason)
	}
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Goblin"], Effect: "stunned"})
	tb.w.hub.Submit(tb.dm, live.Command{Kind: live.CmdActivate, TokenID: ids["Goblin"], Effect: "ward", Targets: []string{ids["Goblin"]}})
	if u := next(t, tb.dm); u.Reason != "Goblin can't act while Incapacitated." {
		t.Fatalf("an incapacitated caster = %+v", u.Reason)
	}
}
