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

// An Effect with an initial save waits on each target's save and counts for nothing meanwhile: it lands
// on a failure, with the parts the DM resolves, and is gone on a success. The DC is the caster's spell
// save DC unless the Effect fixes one.
func TestAnInitialSaveDecidesWhetherTheEffectLands(t *testing.T) {
	t.Parallel()
	w, tb, ids := activationTable(t)
	activate := func(slug string) *live.View {
		t.Helper()
		u := tb.playerSays(live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: slug, Targets: []string{ids["Goblin"]}})
		if u.View == nil || len(u.View.Saves) != 1 {
			t.Fatalf("%s = %+v", slug, u)
		}
		return u.View
	}
	settle := func(v *live.View, face int) *live.View {
		t.Helper()
		tb.fill(v.Saves[0].RollID, tb.w.dm, face)
		next(t, tb.player)
		return next(t, tb.dm).View
	}
	v := activate("slumber")
	if effect(token(v, "Goblin"), "Slumber") != nil {
		t.Fatalf("slumber before the save = %+v", token(v, "Goblin").Effects)
	}
	if roll := lastRoll(t, w, "Wisdom save against Slumber"); roll.Purpose != "Wisdom save against Slumber" {
		t.Fatalf("roll = %q", roll.Purpose)
	}
	if dm := look(t, w, tb.dm); dm.Saves[0].DC != 14 {
		t.Fatalf("against Aria's spell save DC = %+v", dm.Saves)
	}
	if v = settle(v, 1); effect(token(v, "Goblin"), "Slumber") == nil || len(v.Saves) != 0 {
		t.Fatalf("slumber after a failed save = %+v %+v", token(v, "Goblin").Effects, v.Saves)
	}
	v = activate("snare")
	if dm := look(t, w, tb.dm); dm.Saves[0].DC != 15 || len(v.Manual) != 0 {
		t.Fatalf("snare = %+v %+v", dm.Saves, v.Manual)
	}
	if v = settle(v, 20); effect(token(v, "Goblin"), "Snare") != nil || len(v.Manual) != 0 {
		t.Fatalf("snare after a passed save = %+v %+v", token(v, "Goblin").Effects, v.Manual)
	}
	if v = settle(activate("snare"), 1); effect(token(v, "Goblin"), "Snare") == nil || len(v.Manual) != 1 || v.Manual[0].Text != "Goblin: Roots grip it." {
		t.Fatalf("snare after a failed save = %+v %+v", token(v, "Goblin").Effects, v.Manual)
	}
	ward := live.Command{Kind: live.CmdActivate, TokenID: ids["Goblin"], Effect: "ward", Targets: []string{ids["Goblin"]}}
	v = activate("daze")
	tb.dmSays(ward)
	settle(v, 1)
	tb.w.hub.Submit(tb.dm, ward)
	if u := next(t, tb.dm); u.Reason != "Goblin can't act while Incapacitated." {
		t.Fatalf("a dazed goblin = %+v", u.Reason)
	}
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Goblin"], Effect: "paralyzed"})
	u := tb.playerSays(live.Command{Kind: live.CmdActivate, TokenID: ids["Aria"], Effect: "snare", Targets: []string{ids["Goblin"]}})
	if m := look(t, w, tb.dm).Manual; len(m) == 0 || len(u.View.Saves) != 0 || effectCount(token(u.View, "Goblin"), "Snare") != 2 || m[len(m)-1].Text != "Goblin: Roots grip it." {
		t.Fatalf("a paralyzed goblin fails its Dexterity save unrolled = %+v %+v %+v", u.View.Saves, token(u.View, "Goblin").Effects, u.View.Manual)
	}
	tb.fight(ids, "Aria", "Goblin")
	activate("slumber")
	w.hub.Close(w.session.ID)
	tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)
	if v := look(t, w, tb.dm); len(v.Saves) != 1 || effectCount(token(v, "Goblin"), "Slumber") != 1 {
		t.Fatalf("after a restart the second slumber still waits = %+v %+v", v.Saves, token(v, "Goblin").Effects)
	}
	v = look(t, w, tb.dm)
	aria, goblin := combatant(v, "Aria").ID, combatant(v, "Goblin").ID
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: aria})
	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: goblin})
	v = settle(look(t, w, tb.dm), 1)
	fresh := 0
	for _, e := range token(v, "Goblin").Effects {
		if e.Name == "Slumber" && e.RoundsLeft == 10 {
			fresh++
		}
	}
	if fresh != 1 {
		t.Fatalf("a waiting slumber does not count down = %+v", token(v, "Goblin").Effects)
	}
}

func effectCount(t *live.TokenView, name string) int {
	n := 0
	for _, e := range t.Effects {
		if e.Name == name {
			n++
		}
	}
	return n
}
