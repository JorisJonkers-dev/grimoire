package live_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
)

// Anyone at the table measures a route over the world map: in hexes, in miles by the Map's scale, and
// in time at each pace. Only whoever asked is answered, and nothing in the Session changes.
func TestMeasuringARouteOnTheWorldMap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	realm := w.realm(t)
	dm := join(t, w, w.dm, dmCaller, live.AudienceDM)
	player := join(t, w, w.player, playerCaller, live.AudienceParty)
	route := []live.Hex{{Q: 0, R: 0}, {Q: 3, R: 0}, {Q: 3, R: 2}}
	refuse := func(sub *live.Subscriber, want string, hexes []live.Hex) {
		t.Helper()
		w.hub.Submit(sub, live.Command{Kind: live.CmdMeasureRoute, Hexes: hexes, Nonce: "m"})
		if u := next(t, sub); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, want) || u.Nonce != "m" {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	refuse(player, "no world map", route)

	// A cell of this Map covers two and a half miles.
	if _, err := w.pool.Exec(ctx, "UPDATE campaign.maps SET scale_miles = 2.5 WHERE id = $1", uuid.UUID(realm.ID)); err != nil {
		t.Fatal(err)
	}
	w.hub.Submit(dm, live.Command{Kind: live.CmdSetWorld, MapID: uuid.UUID(realm.ID).String()})
	set := next(t, dm)
	next(t, player)

	w.hub.Submit(player, live.Command{Kind: live.CmdMeasureRoute, Hexes: route, Nonce: "n7"})
	got := next(t, player)
	want := &live.MeasureView{Hexes: 5, Miles: 12.5, Plans: []live.PlanView{{Pace: "slow", Minutes: 375, Days: 1}, {Pace: "normal", Minutes: 250, Days: 1}, {Pace: "fast", Minutes: 188, Days: 1}}}
	if got.Kind != live.UpdMeasured || got.Nonce != "n7" || got.Seq != set.Seq || !reflect.DeepEqual(got.Measure, want) {
		t.Fatalf("measured = %+v %+v", got, got.Measure)
	}
	// The DM measures too; what the player measured never reached the DM's screen.
	w.hub.Submit(dm, live.Command{Kind: live.CmdMeasureRoute, Hexes: []live.Hex{{Q: 0, R: 0}, {Q: 1, R: 0}}})
	if u := next(t, dm); u.Kind != live.UpdMeasured || u.Measure.Hexes != 1 || u.Measure.Miles != 2.5 {
		t.Fatalf("the DM measured = %+v %+v", u, u.Measure)
	}

	refuse(player, "two", route[:1])
	refuse(player, "two", nil)
	long := make([]live.Hex, live.MaxWaypoints+1)
	refuse(player, "at most", long)
	refuse(player, "off the map", []live.Hex{{Q: 0, R: 0}, {Q: 400, R: 0}})
	// As many waypoints as are allowed are measured.
	w.hub.Submit(player, live.Command{Kind: live.CmdMeasureRoute, Hexes: long[:live.MaxWaypoints]})
	if u := next(t, player); u.Kind != live.UpdMeasured || u.Measure.Hexes != 0 || u.Measure.Miles != 0 {
		t.Fatalf("a route that stays put = %+v %+v", u, u.Measure)
	}
	// Measuring left the Session as it was: the next change is the next in sequence.
	w.hub.Submit(dm, live.Command{Kind: live.CmdAddNode, Label: "Oakford"})
	if u := next(t, dm); u.Kind != live.UpdView || u.Seq != set.Seq+1 {
		t.Fatalf("after measuring = %+v, world set at %d", u, set.Seq)
	}
}

// A route is measured up to a million miles; a longer one is refused rather than answered wrongly.
func TestMeasuringStopsAtAMillionMiles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	realm := w.realm(t)
	dm := join(t, w, w.dm, dmCaller, live.AudienceDM)
	// Small hexes of a thousand miles each: twenty-five of them lie along the top of the picture.
	if _, err := w.pool.Exec(ctx, "UPDATE campaign.maps SET scale_miles = 1000, hex_size_px = 8, origin_x = 7, origin_y = 8 WHERE id = $1", uuid.UUID(realm.ID)); err != nil {
		t.Fatal(err)
	}
	w.hub.Submit(dm, live.Command{Kind: live.CmdSetWorld, MapID: uuid.UUID(realm.ID).String()})
	next(t, dm)
	back := func(legs int) []live.Hex {
		out := []live.Hex{{Q: 0, R: 0}}
		for i := range legs {
			out = append(out, live.Hex{Q: 25 * ((i + 1) % 2), R: 0})
		}
		return out
	}
	w.hub.Submit(dm, live.Command{Kind: live.CmdMeasureRoute, Hexes: back(40)})
	u := next(t, dm)
	if u.Kind != live.UpdMeasured || u.Measure.Hexes != 1000 || u.Measure.Miles != live.MaxMeasuredMiles || u.Measure.Plans[0].Minutes != 30_000_000 || u.Measure.Plans[0].Days != 62_500 {
		t.Fatalf("a million miles = %+v %+v", u, u.Measure)
	}
	w.hub.Submit(dm, live.Command{Kind: live.CmdMeasureRoute, Hexes: append(back(40), live.Hex{Q: 1, R: 0})})
	if u := next(t, dm); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, "too long") {
		t.Fatalf("a thousand miles more = %+v", u)
	}
}
