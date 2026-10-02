package live_test

import (
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
)

func manuals(v *live.View) string {
	var out []string
	for _, m := range v.Manual {
		out = append(out, m.Text)
	}
	return strings.Join(out, "\n")
}

// Overgrowth costs four feet a foot, spikes cut on every step, lava burns on entry, a stinking cloud
// poisons whoever walks in, and fog hides a goblin from eyes but not from blindsight.
func TestSurfacesFromTheCatalogue(t *testing.T) {
	t.Parallel()
	w, tb, ids := magicTable(t)
	paint := func(kind string, hexes ...live.Hex) {
		t.Helper()
		tb.dmSays(live.Command{Kind: live.CmdPaintSurface, Hexes: hexes, Surface: kind})
	}
	paint("plant-growth", live.Hex{Q: 1, R: 0})
	w.hub.Submit(tb.player, live.Command{Kind: live.CmdPlanWalk, TokenID: ids["Aria"], Q: 1, R: 0})
	if u := next(t, tb.player); u.Path == nil || u.Path.CostFt != 20 {
		t.Fatalf("overgrowth costs 20 feet a hex = %+v", u.Path)
	}
	paint("", live.Hex{Q: 1, R: 0})
	paint("spikes", live.Hex{Q: 0, R: 1}, live.Hex{Q: 1, R: 1}, live.Hex{Q: -1, R: 1}, live.Hex{Q: -1, R: 2}, live.Hex{Q: 0, R: 2})
	tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: 0, R: 2})
	if m := manuals(look(t, w, tb.dm)); !strings.Contains(m, "Aria moves 10 feet through spikes: 2d4 piercing damage for every 5 feet.") {
		t.Fatalf("spikes = %s", m)
	}
	paint("lava", live.Hex{Q: 0, R: 3})
	tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: 0, R: 3})
	if m := manuals(look(t, w, tb.dm)); !strings.Contains(m, "Aria walks into lava: 10d10 fire damage.") {
		t.Fatalf("lava = %s", m)
	}
	paint("stinking-cloud", live.Hex{Q: -1, R: 3})
	tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: -1, R: 3})
	next(t, tb.player)
	next(t, tb.dm)
	v := look(t, w, tb.dm)
	if effect(token(v, "Aria"), "Poisoned") == nil {
		t.Fatalf("the stinking cloud poisons Aria = %+v", token(v, "Aria").Effects)
	}
	_, p := tb.dmSays(live.Command{Kind: live.CmdPaintSurface, Hexes: []live.Hex{{Q: 3, R: 0}}, Surface: "fog"})
	if token(p.View, "Goblin") != nil {
		t.Fatal("a goblin in fog is hidden from eyes")
	}
	if _, p = tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "bat", TokenKind: domain.TokenParty, Q: 2}); token(p.View, "Goblin") == nil {
		t.Fatal("blindsight finds the goblin in the fog")
	}
}

// A creature that starts its turn on consecrated ground is Blessed; one starting it in fire burns.
func TestSurfacesAtTheStartOfATurn(t *testing.T) {
	t.Parallel()
	w, tb, ids := magicTable(t)
	tb.dmSays(live.Command{Kind: live.CmdPaintSurface, Hexes: []live.Hex{{Q: 0, R: 0}}, Surface: "consecrated"})
	tb.dmSays(live.Command{Kind: live.CmdPaintSurface, Hexes: []live.Hex{{Q: 3, R: 0}}, Surface: "fire"})
	tb.fight(ids, "Aria", "Goblin")
	v := look(t, w, tb.dm)
	if effect(token(v, "Aria"), "Bless") == nil {
		t.Fatalf("Aria starts her turn blessed = %+v", token(v, "Aria").Effects)
	}
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(v, "Aria").ID})
	if m := manuals(look(t, w, tb.dm)); !strings.Contains(m, "Goblin starts its turn in fire: 1d4 fire damage.") {
		t.Fatalf("fire = %s", m)
	}
}
