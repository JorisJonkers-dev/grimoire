package live_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
)

// The party keeps a Marching Order: anyone at the table arranges it, every screen shows it, and of two
// of the party equally near an enemy the one further to the front is the one it goes for.
func TestThePartyKeepsAMarchingOrder(t *testing.T) {
	t.Parallel()
	w, tb, ids := restingParty(t)
	order := func(v *live.View) []live.MarchView { return v.MarchingOrder }
	aria, brom := ids["Aria"], ids["Brom"]
	// Until it is arranged, every Character of the Campaign is listed, by name, with no place.
	if got := order(look(t, w, tb.player)); !reflect.DeepEqual(got, []live.MarchView{{CharacterID: aria, Name: "Aria"}, {CharacterID: brom, Name: "Brom"}}) {
		t.Fatalf("before it is arranged = %+v", got)
	}
	p := tb.playerSays(live.Command{Kind: live.CmdSetMarchingOrder, CharacterIDs: []string{brom, aria}})
	want := []live.MarchView{{CharacterID: brom, Name: "Brom", Place: 1}, {CharacterID: aria, Name: "Aria", Place: 2}}
	if got := order(p.View); !reflect.DeepEqual(got, want) {
		t.Fatalf("arranged by a player = %+v", got)
	}
	if got := order(look(t, w, tb.dm)); !reflect.DeepEqual(got, want) {
		t.Fatalf("on the DM's screen = %+v", got)
	}
	// Whoever is left out marches behind those who are placed.
	d, _ := tb.dmSays(live.Command{Kind: live.CmdSetMarchingOrder, CharacterIDs: []string{aria}})
	if got := order(d.View); !reflect.DeepEqual(got, []live.MarchView{{CharacterID: aria, Name: "Aria", Place: 1}, {CharacterID: brom, Name: "Brom"}}) {
		t.Fatalf("with one placed = %+v", got)
	}
	for want, cmd := range map[string]live.Command{
		"characters of this campaign": {Kind: live.CmdSetMarchingOrder, CharacterIDs: []string{aria, uuid.NewString()}},
		"of this campaign":            {Kind: live.CmdSetMarchingOrder, CharacterIDs: []string{"nobody"}},
		"once":                        {Kind: live.CmdSetMarchingOrder, CharacterIDs: []string{aria, brom, aria}},
	} {
		w.hub.Submit(tb.player, cmd)
		if u := next(t, tb.player); u.Kind != live.UpdRejected || !strings.Contains(strings.ToLower(u.Reason), want) {
			t.Fatalf("%s = %+v", want, u)
		}
	}

	// A goblin stands between Aria and Brom, five feet from each, and acts first.
	d, _ = tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 1})
	// The stand-in sheets share a name, so the tokens are told apart by where they stand.
	tokens := map[string]string{}
	var fighters []live.CombatantSetup
	for _, tv := range d.View.Tokens {
		tokens[map[int]string{0: "Aria", 1: "Goblin", 2: "Brom"}[tv.Q]] = tv.ID
		fighters = append(fighters, live.CombatantSetup{TokenID: tv.ID, SpeedFt: 30})
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: fighters})
	if d.View == nil || d.View.Combat == nil || len(d.View.Combat.Combatants) != 3 {
		t.Fatalf("the fight = %+v, tokens %v", d, tokens)
	}
	for i := range d.View.Combat.Combatants {
		c := &d.View.Combat.Combatants[i]
		who, face := w.player, 5-i
		if c.TokenID == tokens["Goblin"] {
			who, face = w.dm, 20
		}
		tb.roll(c, who, face)
	}
	target := func() string {
		t.Helper()
		s := combatant(look(t, w, tb.dm), "Goblin").Suggestion
		if s == nil {
			t.Fatal("the goblin suggests nothing")
		}
		return s.TargetID
	}
	if got := target(); got != tokens["Aria"] {
		t.Fatalf("with Aria in front the goblin goes for %s, not Aria %s", got, tokens["Aria"])
	}
	tb.playerSays(live.Command{Kind: live.CmdSetMarchingOrder, CharacterIDs: []string{brom, aria}})
	if got := target(); got != tokens["Brom"] {
		t.Fatalf("with Brom in front the goblin goes for %s, not Brom %s", got, tokens["Brom"])
	}
	// No order at all is an order too; and it is kept.
	tb.dmSays(live.Command{Kind: live.CmdSetMarchingOrder})
	tb.dmSays(live.Command{Kind: live.CmdSetMarchingOrder, CharacterIDs: []string{brom, aria}})
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	if got := order(look(t, w, tb.dm)); !reflect.DeepEqual(got, want) {
		t.Fatalf("after a restart = %+v", got)
	}
}
