package live_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
)

type failingAtlas struct{ live.Store }

func (failingAtlas) LoadWorld(context.Context, uuid.UUID, domain.SessionID, domain.MapID) (*domain.World, error) {
	return nil, errors.New("gone")
}

func nodeNamed(t *testing.T, v *live.WorldView, name string) live.NodeView {
	t.Helper()
	for _, n := range v.Nodes {
		if n.Name == name {
			return n
		}
	}
	t.Fatalf("no %s in %+v", name, v.Nodes)
	return live.NodeView{}
}

func names(v *live.WorldView) []string {
	out := []string{}
	for _, n := range v.Nodes {
		out = append(out, n.Name)
	}
	slices.Sort(out)
	return out
}

func TestThePartyTravelsTheWorldMap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	realm := w.realm(t)
	dm := join(t, w, w.dm, dmCaller, live.AudienceDM)
	player := join(t, w, w.player, playerCaller, live.AudienceParty)
	table := join(t, w, w.player, playerCaller, live.AudienceTable)
	var seen []string
	say := func(cmd live.Command) (live.Update, live.Update) {
		t.Helper()
		w.hub.Submit(dm, cmd)
		d := next(t, dm)
		if d.Kind != live.UpdView {
			t.Fatalf("%s rejected: %+v", cmd.Kind, d)
		}
		p := next(t, player)
		tb := next(t, table)
		for _, u := range []live.Update{p, tb} {
			raw, _ := json.Marshal(u)
			seen = append(seen, string(raw))
		}
		return d, p
	}
	refuse := func(want string, cmd live.Command) {
		t.Helper()
		w.hub.Submit(dm, cmd)
		if u := next(t, dm); u.Kind != live.UpdRejected || !strings.Contains(strings.ToLower(u.Reason), want) {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	refuse("choose a world map first", live.Command{Kind: live.CmdAddNode, Label: "Oakford"})
	refuse("choose a world map", live.Command{Kind: live.CmdSetWorld, MapID: uuid.UUID(w.dungeon(t).ID).String()})
	refuse("no such map", live.Command{Kind: live.CmdSetMap, MapID: uuid.UUID(realm.ID).String()})
	d, p := say(live.Command{Kind: live.CmdSetWorld, MapID: uuid.UUID(realm.ID).String()})
	if d.View.World == nil || d.View.World.Map.Name != "Realm" || len(p.View.World.Nodes) != 0 || p.View.World.PartyNodeID != "" {
		t.Fatalf("world set = dm %+v player %+v", d.View.World, p.View.World)
	}
	say(live.Command{Kind: live.CmdAddNode, Label: " Oakford ", Q: 0, R: 0})
	say(live.Command{Kind: live.CmdAddNode, Label: "Mill", Q: 5, R: 0})
	d, p = say(live.Command{Kind: live.CmdAddNode, Label: "Keep", Q: 1, R: 4})
	if got := names(d.View.World); !slices.Equal(got, []string{"Keep", "Mill", "Oakford"}) || len(p.View.World.Nodes) != 0 {
		t.Fatalf("nodes = dm %v player %+v", got, p.View.World.Nodes)
	}
	oak, mill, keep := nodeNamed(t, d.View.World, "Oakford").ID, nodeNamed(t, d.View.World, "Mill").ID, nodeNamed(t, d.View.World, "Keep").ID
	refuse("name of up to 40", live.Command{Kind: live.CmdAddNode, Label: " ", Q: 2, R: 0})
	refuse("off the map", live.Command{Kind: live.CmdAddNode, Label: "Far", Q: 50, R: 0})
	refuse("already stands there", live.Command{Kind: live.CmdAddNode, Label: "Twin", Q: 5, R: 0})
	say(live.Command{Kind: live.CmdAddRoute, NodeID: oak, ToNodeID: mill, DistanceMi: 12})
	d, _ = say(live.Command{Kind: live.CmdAddRoute, NodeID: mill, ToNodeID: keep, DistanceMi: 30})
	road := d.View.World.Routes[slices.IndexFunc(d.View.World.Routes, func(r live.RouteView) bool { return r.DistanceMi == 12 })]
	pass := d.View.World.Routes[slices.IndexFunc(d.View.World.Routes, func(r live.RouteView) bool { return r.DistanceMi == 30 })]
	if want := []live.PlanView{{Pace: "slow", Minutes: 360, Days: 1}, {Pace: "normal", Minutes: 240, Days: 1}, {Pace: "fast", Minutes: 180, Days: 1}}; !slices.Equal(road.Plans, want) {
		t.Fatalf("plans = %+v", road.Plans)
	}
	refuse("two different locations", live.Command{Kind: live.CmdAddRoute, NodeID: oak, ToNodeID: oak, DistanceMi: 3})
	refuse("two different locations", live.Command{Kind: live.CmdAddRoute, NodeID: oak, ToNodeID: uuid.NewString(), DistanceMi: 3})
	refuse("1 to 2000 miles", live.Command{Kind: live.CmdAddRoute, NodeID: oak, ToNodeID: keep, DistanceMi: 2001})
	refuse("already joined", live.Command{Kind: live.CmdAddRoute, NodeID: mill, ToNodeID: oak, DistanceMi: 5})
	refuse("place the party", live.Command{Kind: live.CmdTravel, RouteID: road.ID, Pace: "normal"})

	_, p = say(live.Command{Kind: live.CmdPlaceParty, NodeID: oak})
	pw := p.View.World
	if got := names(pw); pw.PartyNodeID != oak || !slices.Equal(got, []string{"Mill", "Oakford"}) || len(pw.Routes) != 1 || pw.Routes[0].ID != road.ID {
		t.Fatalf("at Oakford the party sees %v %+v", got, pw)
	}
	if len(pw.Revealed) != 7 || !slices.Contains(pw.Revealed, live.Hex{Q: 2, R: 0}) || slices.Contains(pw.Revealed, live.Hex{Q: 3, R: 0}) {
		t.Fatalf("revealed around Oakford = %v", pw.Revealed)
	}
	refuse("neither end", live.Command{Kind: live.CmdTravel, RouteID: pass.ID, Pace: "normal"})
	refuse("slow, normal or fast", live.Command{Kind: live.CmdTravel, RouteID: road.ID, Pace: "gallop"})
	refuse("no such route", live.Command{Kind: live.CmdTravel, RouteID: uuid.NewString(), Pace: "fast"})
	_, p = say(live.Command{Kind: live.CmdTravel, RouteID: road.ID, Pace: "normal"})
	pw = p.View.World
	if got := names(pw); pw.PartyNodeID != mill || !slices.Equal(got, []string{"Keep", "Mill", "Oakford"}) || len(pw.Routes) != 2 {
		t.Fatalf("at the Mill the party sees %v %+v", got, pw.Routes)
	}
	if want := []live.LegView{{From: "Oakford", To: "Mill", Pace: "normal", DistanceMi: 12, Minutes: 240, Days: 1}}; !slices.Equal(pw.Legs, want) {
		t.Fatalf("legs = %+v", pw.Legs)
	}
	w.hub.Submit(player, live.Command{Kind: live.CmdTravel, RouteID: pass.ID, Pace: "fast"})
	if u := next(t, player); u.Kind != live.UpdRejected {
		t.Fatalf("a player travels = %+v", u)
	}
	_, p = say(live.Command{Kind: live.CmdTravel, RouteID: road.ID, Pace: "fast"})
	if pw = p.View.World; pw.PartyNodeID != oak || len(pw.Legs) != 2 || pw.Legs[1].From != "Mill" || pw.Legs[1].Minutes != 180 {
		t.Fatalf("back to Oakford = %+v", pw)
	}
	for _, frame := range seen[:14] {
		if strings.Contains(frame, "Keep") {
			t.Fatalf("the Keep reached a player before the party could reach it: %s", frame)
		}
	}

	w.hub.Close(w.session.ID)
	dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	w.hub.Submit(dm, live.Command{Kind: live.CmdResync})
	if dw := next(t, dm).View.World; dw == nil || dw.PartyNodeID != oak || len(dw.Nodes) != 3 || len(dw.Routes) != 2 || len(dw.Legs) != 2 || len(dw.Revealed) != 14 {
		t.Fatalf("the world survives a restart = %+v", dw)
	}
	player = join(t, w, w.player, playerCaller, live.AudienceParty)
	table = join(t, w, w.player, playerCaller, live.AudienceTable)
	refuse("no such route", live.Command{Kind: live.CmdRemoveRoute, RouteID: uuid.NewString()})
	refuse("no such location", live.Command{Kind: live.CmdPlaceParty, NodeID: uuid.NewString()})
	d, _ = say(live.Command{Kind: live.CmdRemoveRoute, RouteID: pass.ID})
	if len(d.View.World.Routes) != 1 {
		t.Fatalf("route removed = %+v", d.View.World.Routes)
	}
	d, _ = say(live.Command{Kind: live.CmdRemoveNode, NodeID: mill})
	if dw := d.View.World; len(dw.Nodes) != 2 || len(dw.Routes) != 0 || dw.PartyNodeID != oak {
		t.Fatalf("the Mill removed = %+v", dw)
	}
	d, _ = say(live.Command{Kind: live.CmdRemoveNode, NodeID: oak})
	if dw := d.View.World; len(dw.Nodes) != 1 || dw.PartyNodeID != "" {
		t.Fatalf("Oakford removed with the party on it = %+v", dw)
	}
	d, _ = say(live.Command{Kind: live.CmdSetWorld})
	if d.View.World != nil {
		t.Fatalf("world cleared = %+v", d.View.World)
	}
	say(live.Command{Kind: live.CmdSetWorld, MapID: uuid.UUID(realm.ID).String()})
	w.hub.Close(w.session.ID)
	w.hub.Store = failingAtlas{Store: pgstore.New(w.pool)}
	if _, err := w.hub.Join(ctx, w.session.ID, w.dm, dmCaller, live.AudienceDM); err == nil {
		t.Fatal("world load failure ignored")
	}
}
