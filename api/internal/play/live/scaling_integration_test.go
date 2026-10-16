//go:build integration

package live_test

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
)

// Every SRD area spell that scales today rolls the dice it always did at each slot: refused below its
// own level and above 9, its own dice at its own level, and its added dice at the slots above.
func TestSRDAreaSpellsScaleWithTheirSlot(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		slug, name string
		base, slot int
		dice       string
	}{
		{"burning-hands", "Burning Hands", 1, 9, "11d6"},
		{"thunderwave", "Thunderwave", 1, 2, "3d8"},
		{"shatter", "Shatter", 2, 9, "10d8"},
		{"fireball", "Fireball", 3, 9, "14d6"},
		{"fireball", "Fireball", 3, 4, "9d6"},
		{"fireball", "Fireball", 3, 3, "8d6"},
		{"fireball", "Fireball", 3, 0, "8d6"},
		{"lightning-bolt", "Lightning Bolt", 3, 5, "10d6"},
		{"cone-of-cold", "Cone of Cold", 5, 9, "12d8"},
		{"spirit-guardians", "Spirit Guardians", 3, 9, "9d8"},
		{"wall-of-fire", "Wall of Fire", 4, 6, "7d8"},
	} {
		t.Run(c.slug+"-"+strconv.Itoa(c.slot), func(t *testing.T) {
			t.Parallel()
			w, tb, ids := shapedTable(t)
			tb.fight(ids, "Aria", "Goblin")
			cmd := live.Command{Kind: live.CmdPreviewArea, TokenID: ids["Aria"], Effect: c.slug, Q: 3, R: 0}
			refused := "Cast " + c.name + " with a slot of level " + strconv.Itoa(c.base) + " to 9."
			for _, slot := range []int{c.base - 1, 10} {
				if slot == 0 {
					continue
				}
				cmd.Slot = slot
				if u := tb.playerSays(cmd); u.Reason != refused {
					t.Fatalf("slot %d = %+v", slot, u)
				}
			}
			cmd.Kind, cmd.Slot = live.CmdCastArea, c.slot
			u := tb.playerSays(cmd)
			if u.View == nil || u.View.Area == nil {
				t.Fatalf("cast = %+v", u)
			}
			roll, err := pgstore.New(w.pool).Roll(context.Background(), w.session.CampaignID, domain.RollID(uuid.MustParse(u.View.Area.DamageRollID)))
			if err != nil || !strings.EqualFold(roll.Notation, c.dice) {
				t.Fatalf("damage at slot %d = %q %v", c.slot, roll.Notation, err)
			}
		})
	}
}

// A cast applies all its Scaling before it picks targets: a bigger area at a higher slot takes in the
// goblin, rolls more dice, and needs no concentration; the cast keeps its slot through a restart, so it
// resolves as it was cast.
func TestAnAreaSpellIsActivatedAtItsSlot(t *testing.T) {
	t.Parallel()
	w, tb, ids := shapedTable(t)
	tb.fight(ids, "Aria", "Goblin")
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Aria"], SourceID: ids["Aria"], Effect: "bless"})
	burst := live.Command{Kind: live.CmdPreviewArea, TokenID: ids["Aria"], Effect: "swelling-burst", Q: 1, R: 0, Slot: 1}
	targets := func(a *live.AreaPreview) []string {
		var out []string
		for _, tg := range a.Targets {
			out = append(out, tg.TokenID)
		}
		return out
	}
	if a := tb.playerSays(burst).Area; len(a.Hexes) != 7 || slices.Contains(targets(a), ids["Goblin"]) || len(a.Ends) != 1 {
		t.Fatalf("at slot 1 = %+v", a)
	}
	burst.Slot = 2
	if a := tb.playerSays(burst).Area; len(a.Hexes) != 19 || !slices.Contains(targets(a), ids["Goblin"]) || a.Ends != nil {
		t.Fatalf("at slot 2 = %+v", a)
	}
	burst.Kind = live.CmdCastArea
	u := tb.playerSays(burst)
	roll, err := pgstore.New(w.pool).Roll(context.Background(), w.session.CampaignID, domain.RollID(uuid.MustParse(u.View.Area.DamageRollID)))
	if err != nil || roll.Notation != "2d6" || len(u.View.Area.Saves) != 2 {
		t.Fatalf("cast at slot 2 = %q %+v", roll.Notation, u.View.Area)
	}
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	tb.fill(u.View.Area.DamageRollID, w.player, 1, 1)
	for _, s := range u.View.Area.Saves {
		if s.TokenID == ids["Aria"] {
			tb.fill(s.RollID, w.player, 20, 1) // Bless adds its die
			continue
		}
		tb.fill(s.RollID, w.dm, 20)
	}
	v := barrier(t, w, tb)
	if a := token(v, "Aria"); v.Area != nil || effect(a, "Bless") == nil || effect(a, "Swelling Burst") != nil {
		t.Fatalf("resolved at slot 2, Bless kept = %+v %+v", v.Area, a.Effects)
	}
}
