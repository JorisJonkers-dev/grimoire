//go:build integration

package live_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
)

// The party sneaks with a group Stealth check: a 15 slips past the goblin's passive Perception of 9, a 2
// does not, and the goblin notices the party, which ends the sneaking. The party only sees the reach of
// creatures it can see.
func sneak(on bool) live.Command { return live.Command{Kind: live.CmdSneak, On: on} }

// sneakingWith is a table where Aria sneaks and rolled total for Stealth.
func sneakingWith(t *testing.T, total int) (world, *table, map[string]string) {
	t.Helper()
	w, tb, ids := magicTable(t)
	tb.playerSays(sneak(true))
	tb.fill(uuid.UUID(lastRoll(t, w, "Stealth while sneaking").ID).String(), w.player, total)
	next(t, tb.player)
	next(t, tb.dm)
	return w, tb, ids
}

func TestStoppingSneakingBeforeStartingIsRefused(t *testing.T) {
	t.Parallel()
	_, tb, _ := magicTable(t)
	if u := tb.playerSays(sneak(false)); !strings.Contains(u.Reason, "not sneaking") {
		t.Fatalf("%+v", u)
	}
}

func TestSneakingWaitsForAStealthRollAndShowsWhatCanNotice(t *testing.T) {
	t.Parallel()
	_, tb, _ := magicTable(t)
	if s := tb.playerSays(sneak(true)).View.Sneak; s == nil || !s.Waiting || len(s.Reach) == 0 || s.Totals != nil {
		t.Fatalf("sneaking = %+v", s)
	}
	if u := tb.playerSays(sneak(true)); !strings.Contains(u.Reason, "already sneaking") {
		t.Fatalf("twice = %+v", u)
	}
}

func TestAfterARestartTheDMStillSeesTheStealthTotal(t *testing.T) {
	t.Parallel()
	w, tb, ids := sneakingWith(t, 15)
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	if s := look(t, w, tb.dm).Sneak; s == nil || s.Waiting || s.Totals[ids["Aria"]] != 15 {
		t.Fatalf("the DM sees %+v", s)
	}
}

func TestAHighStealthSlipsPast(t *testing.T) {
	t.Parallel()
	_, tb, ids := sneakingWith(t, 15)
	if u := tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: 1, R: 0}); u.View.Sneak == nil {
		t.Fatalf("noticed: %+v", u.View)
	}
}

func TestThePartySeesNoReachOfACreatureItCannotSee(t *testing.T) {
	t.Parallel()
	_, tb, ids := sneakingWith(t, 15)
	if _, p := tb.dmSays(live.Command{Kind: live.CmdSetVisibility, TokenID: ids["Goblin"], Qualities: []string{"invisible"}}); len(p.View.Sneak.Reach) != 0 {
		t.Fatalf("reach = %+v", p.View.Sneak.Reach)
	}
}

func TestALowStealthIsNoticed(t *testing.T) {
	t.Parallel()
	w, tb, ids := sneakingWith(t, 2)
	tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: 1, R: 0})
	next(t, tb.player)
	next(t, tb.dm)
	if v := look(t, w, tb.dm); v.Sneak != nil || !strings.Contains(manuals(v), "Goblin notices the party.") {
		t.Fatalf("sneak %+v, %s", v.Sneak, manuals(v))
	}
}

func TestAFightEndsSneakingAndThereIsNoSneakingInOne(t *testing.T) {
	t.Parallel()
	w, tb, ids := magicTable(t)
	tb.playerSays(sneak(true))
	tb.fight(ids, "Aria", "Goblin")
	if v := look(t, w, tb.player); v.Sneak != nil {
		t.Fatalf("still sneaking: %+v", v.Sneak)
	}
	if u := tb.playerSays(sneak(true)); !strings.Contains(u.Reason, "not a fight") {
		t.Fatalf("sneaking in a fight = %+v", u)
	}
}

// On a map a creature's reach is what it sees within 30 feet, so a closed door hides what lies behind it.
func TestSneakingOnAMap(t *testing.T) {
	t.Parallel()
	w, tb, ids := dungeonTable(t)
	tb.dmSays(live.Command{Kind: live.CmdPlaceObject, ObjectKind: "door", Q: 1, R: 0})
	tb.playerSays(live.Command{Kind: live.CmdSneak, On: true})
	tb.fill(uuid.UUID(lastRoll(t, w, "Stealth while sneaking").ID).String(), w.player, 20)
	next(t, tb.player)
	next(t, tb.dm)
	reach := look(t, w, tb.dm).Sneak.Reach
	has := func(q, r int) bool { return containsHex(reach, live.Hex{Q: q, R: r}) }
	if len(reach) == 0 || !has(3, 0) || has(0, 0) {
		t.Fatalf("the goblin at 2,0 watches past itself but not through the door = %+v", reach)
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: 1, R: 1}); u.View.Sneak == nil {
		t.Fatal("a 20 slips past the goblin")
	}
	for want, cmd := range map[string]live.Command{
		"No such creature.":  {Kind: live.CmdCommand, TokenID: "nope"},
		"not yours to play.": {Kind: live.CmdCommand, TokenID: ids["Goblin"]},
	} {
		if u := tb.playerSays(cmd); !strings.Contains(u.Reason, want) {
			t.Errorf("%s = %+v", want, u)
		}
	}
}
