package live_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
)

// The Campaign keeps a date and a time of day. Rests and the DM move it on, and each dawn that passes
// gives back the charges that come back at dawn: no rest does that by itself.
func TestTheGameClockMovesOnAndDawnComesFromIt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb, ids := restingParty(t)
	wand := uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO compendium.documents (key, title, ruleset_year, precedence, license, attribution, url) VALUES ('clock-doc', 'Clock', 2024, 1, 'CC-BY-4.0', 'a', 'https://a') ON CONFLICT DO NOTHING`, nil},
		{`INSERT INTO compendium.items (document_id, slug, name, description, category, cost_gp, weight_lb, magic, requires_attunement)
			SELECT id, 'dawn-wand', 'Dawn Wand', '', 'wand', 1, 1, true, false FROM compendium.documents WHERE key = 'clock-doc' ON CONFLICT DO NOTHING`, nil},
		// Two charges come back at each dawn, of seven: no dice, so the count is certain.
		{`INSERT INTO compendium.item_charges (item_slug, max_charges, regain_dice, regain_faces, regain_bonus, recharge_on) VALUES ('dawn-wand', 7, 0, 0, 2, 'dawn') ON CONFLICT DO NOTHING`, nil},
		{`INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, charges, identified, attuned, created_at)
			SELECT $1, id, 'dawn-wand', 1, 0, true, false, now() FROM campaign.containers WHERE character_id = $2`, []any{wand, ids["Aria"]}},
		{`UPDATE campaign.campaigns SET game_day = 2, game_minute = 480 WHERE id = $1`, []any{w.session.CampaignID}},
	} {
		if _, err := w.pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	restart := func() {
		t.Helper()
		w.hub.Close(w.session.ID)
		tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
		tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	}
	restart()
	charges := func() int {
		t.Helper()
		var n int
		if err := w.pool.QueryRow(ctx, "SELECT charges FROM campaign.item_instances WHERE id = $1", wand).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	at := func(v *live.View, day, hour, minute, want int, when string) {
		t.Helper()
		if v.GameDay != day || v.GameMinute != hour*60+minute || charges() != want {
			t.Fatalf("%s: day %d at %d minutes with %d charges; want day %d at %02d:%02d with %d", when, v.GameDay, v.GameMinute, charges(), day, hour, minute, want)
		}
	}
	at(look(t, w, tb.player), 2, 8, 0, 0, "to begin with")

	// A Short Rest is an hour.
	tb.playerSays(live.Command{Kind: live.CmdProposeRest, Rest: live.RestShort})
	tb.dmSays(live.Command{Kind: live.CmdAgreeRest})
	d, p := tb.dmSays(live.Command{Kind: live.CmdFinishRest})
	at(d.View, 2, 9, 0, 0, "after a Short Rest")
	at(p.View, 2, 9, 0, 0, "after a Short Rest, for the player")
	// A Long Rest is eight; taken by day it passes no dawn, and the wand stays empty.
	tb.dmSays(live.Command{Kind: live.CmdProposeRest, Rest: live.RestLong})
	tb.playerSays(live.Command{Kind: live.CmdAgreeRest})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdFinishRest})
	at(d.View, 2, 17, 0, 0, "after a Long Rest by day")

	// The DM sets the clock. Up to the evening: no dawn. On to six the next morning: one.
	d, _ = tb.dmSays(live.Command{Kind: live.CmdSetClock, GameDay: 2, GameMinute: 22 * 60})
	at(d.View, 2, 22, 0, 0, "set to the evening")
	d, _ = tb.dmSays(live.Command{Kind: live.CmdSetClock, GameDay: 3, GameMinute: 5*60 + 59})
	at(d.View, 3, 5, 59, 0, "a minute before dawn")
	d, p = tb.dmSays(live.Command{Kind: live.CmdSetClock, GameDay: 3, GameMinute: 6 * 60})
	at(d.View, 3, 6, 0, 2, "at dawn")
	if wandOf := instanceNamed(t, containerNamed(t, p.View, "Aria"), "Dawn Wand"); wandOf.Charges == nil || *wandOf.Charges != 2 {
		t.Fatalf("the player's wand at dawn = %+v", wandOf)
	}
	// Two more dawns pass in one go; set back, none; and the wand never holds more than it can.
	d, _ = tb.dmSays(live.Command{Kind: live.CmdSetClock, GameDay: 5, GameMinute: 12 * 60})
	at(d.View, 5, 12, 0, 6, "two dawns on")
	d, _ = tb.dmSays(live.Command{Kind: live.CmdSetClock, GameDay: 1, GameMinute: 0})
	at(d.View, 1, 0, 0, 6, "set back")
	d, _ = tb.dmSays(live.Command{Kind: live.CmdSetClock, GameDay: 1, GameMinute: 7 * 60})
	at(d.View, 1, 7, 0, 7, "a dawn with a full wand")

	// A Long Rest through the night passes dawn like anything else.
	if _, err := w.pool.Exec(ctx, "UPDATE campaign.item_instances SET charges = 1 WHERE id = $1", wand); err != nil {
		t.Fatal(err)
	}
	restart()
	tb.dmSays(live.Command{Kind: live.CmdSetClock, GameDay: 1, GameMinute: 22 * 60})
	tb.dmSays(live.Command{Kind: live.CmdProposeRest, Rest: live.RestLong})
	tb.playerSays(live.Command{Kind: live.CmdAgreeRest})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdFinishRest})
	at(d.View, 2, 6, 0, 3, "after a night's Long Rest")

	for want, cmd := range map[string]live.Command{
		"minutes":  {Kind: live.CmdSetClock, GameDay: 2, GameMinute: 1440},
		"a minute": {Kind: live.CmdSetClock, GameDay: 2, GameMinute: -1},
		"day":      {Kind: live.CmdSetClock, GameDay: -1, GameMinute: 0},
		"days":     {Kind: live.CmdSetClock, GameDay: 1_000_001, GameMinute: 0},
	} {
		w.hub.Submit(tb.dm, cmd)
		if u := next(t, tb.dm); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, "day 0 to 1000000") {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	w.hub.Submit(tb.player, live.Command{Kind: live.CmdSetClock, GameDay: 9, GameMinute: 0})
	if u := next(t, tb.player); u.Kind != live.UpdRejected {
		t.Fatalf("a player sets the clock = %+v", u)
	}
	// The last day and the last minute of a day are allowed.
	d, _ = tb.dmSays(live.Command{Kind: live.CmdSetClock, GameDay: 1_000_000, GameMinute: 1439})
	at(d.View, 1_000_000, 23, 59, 7, "the end of time")
	tb.dmSays(live.Command{Kind: live.CmdSetClock, GameDay: 4, GameMinute: 30})
	// It is kept.
	restart()
	at(look(t, w, tb.dm), 4, 0, 30, 7, "after a restart")
}
