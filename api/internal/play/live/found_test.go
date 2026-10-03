package live_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
)

// Fog on the world map follows what the party has found. Without the world map all is dark but where
// the party has been, the road it walked and the local maps it found; with it, every place on the map
// shows. A secret place never reaches a player either way; the DM sees everything.
func TestFogOnTheWorldMapFollowsFoundMaps(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	realm, crypt, cellar, attic := w.realm(t), w.dungeon(t), w.picture(t, "Cellar", domain.MapLocal), w.picture(t, "Attic", domain.MapLocal)
	shed := w.picture(t, "Shed", domain.MapLocal)
	// Small hexes, so that there is land the party has not seen: fourteen to a row.
	if _, err := w.pool.Exec(ctx, "UPDATE campaign.maps SET hex_size_px = 16, origin_x = 14, origin_y = 16 WHERE id = $1", uuid.UUID(realm.ID)); err != nil {
		t.Fatal(err)
	}
	id := func(m domain.Map) string { return uuid.UUID(m.ID).String() }
	dm := join(t, w, w.dm, dmCaller, live.AudienceDM)
	player := join(t, w, w.player, playerCaller, live.AudienceParty)
	table := join(t, w, w.player, playerCaller, live.AudienceTable)
	var seen []string
	say := func(cmd live.Command) (*live.WorldView, *live.WorldView) {
		t.Helper()
		w.hub.Submit(dm, cmd)
		d := next(t, dm)
		if d.Kind != live.UpdView {
			t.Fatalf("%s rejected: %+v", cmd.Kind, d)
		}
		p, tb := next(t, player), next(t, table)
		for _, u := range []live.Update{p, tb} {
			raw, _ := json.Marshal(u)
			seen = append(seen, string(raw))
		}
		return d.View.World, p.View.World
	}
	refuse := func(want string, cmd live.Command) {
		t.Helper()
		w.hub.Submit(dm, cmd)
		if u := next(t, dm); u.Kind != live.UpdRejected || !strings.Contains(strings.ToLower(u.Reason), want) {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	has := func(v *live.WorldView, q, r int) bool { return slices.Contains(v.Revealed, live.Hex{Q: q, R: r}) }

	refuse("choose a world map first", live.Command{Kind: live.CmdFindMap, MapID: id(realm), On: true})
	say(live.Command{Kind: live.CmdSetWorld, MapID: id(realm)})
	say(live.Command{Kind: live.CmdAddNode, Label: "Oakford", Q: 0, R: 0})
	say(live.Command{Kind: live.CmdAddNode, Label: "Mill", Q: 9, R: 0})
	// The Lair is secret; it lies in sight of Oakford and a route leads to it.
	say(live.Command{Kind: live.CmdAddNode, Label: "Lair", Q: 1, R: 1, Secret: true})
	// The Keep has a local map of its own, the Tower none; both lie far from any road.
	say(live.Command{Kind: live.CmdAddNode, Label: "Keep", Q: 2, R: 8, MapID: id(crypt)})
	d, p := say(live.Command{Kind: live.CmdAddNode, Label: "Tower", Q: 6, R: 6})
	refuse("local map of this campaign", live.Command{Kind: live.CmdAddNode, Label: "Nowhere", Q: 5, R: 5, MapID: uuid.NewString()})
	refuse("local map of this campaign", live.Command{Kind: live.CmdAddNode, Label: "Nowhere", Q: 5, R: 5, MapID: id(realm)})
	refuse("already has that map", live.Command{Kind: live.CmdAddNode, Label: "Twin", Q: 5, R: 5, MapID: id(crypt)})
	if got := names(d); !slices.Equal(got, []string{"Keep", "Lair", "Mill", "Oakford", "Tower"}) || d.Found || len(p.Nodes) != 0 || p.Found {
		t.Fatalf("before the party is anywhere: dm %v %+v, player %+v", got, d, p)
	}
	lair, keep := nodeNamed(t, d, "Lair"), nodeNamed(t, d, "Keep")
	if !lair.Secret || lair.MapID != "" || keep.Secret || keep.MapID != id(crypt) || keep.Found || nodeNamed(t, d, "Oakford").Secret {
		t.Fatalf("the DM's places = %+v %+v", lair, keep)
	}
	// A place whose local map the party already has is lit from the moment it is on the map.
	if _, err := w.pool.Exec(ctx, "UPDATE campaign.maps SET found = true WHERE id = $1", uuid.UUID(cellar.ID)); err != nil {
		t.Fatal(err)
	}
	_, p = say(live.Command{Kind: live.CmdAddNode, Label: "Vault", Q: 6, R: 10, MapID: id(cellar)})
	if got := names(p); !slices.Equal(got, []string{"Vault"}) || !nodeNamed(t, p, "Vault").Found || !has(p, 6, 10) || !has(p, 7, 10) || has(p, 8, 10) {
		t.Fatalf("a place with a map the party has: %v %v", got, p.Revealed)
	}
	// A secret place stays dark even when the party has the local map that lies there, and when it finds it.
	if _, err := w.pool.Exec(ctx, "UPDATE campaign.maps SET found = true WHERE id = $1", uuid.UUID(attic.ID)); err != nil {
		t.Fatal(err)
	}
	before := len(p.Revealed)
	say(live.Command{Kind: live.CmdAddNode, Label: "Den", Q: 4, R: 10, Secret: true, MapID: id(attic)})
	say(live.Command{Kind: live.CmdFindMap, MapID: id(attic)})
	_, p = say(live.Command{Kind: live.CmdFindMap, MapID: id(attic), On: true})
	if got := names(p); !slices.Equal(got, []string{"Vault"}) || len(p.Revealed) != before || has(p, 4, 10) {
		t.Fatalf("a secret place with a found map: %v %v", got, p.Revealed)
	}
	oak, mill := nodeNamed(t, d, "Oakford").ID, nodeNamed(t, d, "Mill").ID
	say(live.Command{Kind: live.CmdAddRoute, NodeID: oak, ToNodeID: mill, DistanceMi: 54})
	d, _ = say(live.Command{Kind: live.CmdAddRoute, NodeID: oak, ToNodeID: lair.ID, DistanceMi: 6})
	road := d.Routes[slices.IndexFunc(d.Routes, func(r live.RouteView) bool { return r.DistanceMi == 54 })]

	// Without the world map: dark but for where the party stands. The secret Lair is in sight and on a
	// route from here, and still is not there.
	d, p = say(live.Command{Kind: live.CmdPlaceParty, NodeID: oak})
	if got := names(p); !slices.Equal(got, []string{"Mill", "Oakford", "Vault"}) || len(p.Routes) != 1 || p.Routes[0].ID != road.ID || !has(p, 1, 1) || has(p, 5, 0) {
		t.Fatalf("at Oakford without the map the party sees %v %+v", got, p)
	}
	if len(d.Routes) != 2 || len(d.Nodes) != 7 {
		t.Fatalf("the DM sees %+v", d)
	}
	// The road walked stays lit: every hex along it, not only the land around each end.
	_, p = say(live.Command{Kind: live.CmdTravel, RouteID: road.ID, Pace: "normal"})
	for q := range 10 {
		if !has(p, q, 0) {
			t.Fatalf("hex %d of the road walked is dark: %v", q, p.Revealed)
		}
	}
	if has(p, 5, 2) || has(p, 4, 3) || slices.Contains(names(p), "Keep") || slices.Contains(names(p), "Tower") {
		t.Fatalf("off the road the party sees %v %v", names(p), p.Revealed)
	}

	// A local map the party finds lights its place on the world map, and marks it as found.
	refuse("not on this world map", live.Command{Kind: live.CmdFindMap, MapID: id(shed), On: true})
	refuse("not on this world map", live.Command{Kind: live.CmdFindMap, MapID: uuid.NewString(), On: true})
	refuse("does not have that map", live.Command{Kind: live.CmdFindMap, MapID: id(crypt)})
	w.hub.Submit(player, live.Command{Kind: live.CmdFindMap, MapID: id(crypt), On: true})
	if u := next(t, player); u.Kind != live.UpdRejected {
		t.Fatalf("a player finds a map = %+v", u)
	}
	d, p = say(live.Command{Kind: live.CmdFindMap, MapID: id(crypt), On: true})
	if k := nodeNamed(t, p, "Keep"); !k.Found || k.MapID != id(crypt) || !has(p, 2, 8) || !has(p, 3, 8) || has(p, 4, 8) || slices.Contains(names(p), "Tower") {
		t.Fatalf("the Keep's map found: %+v %v", k, p.Revealed)
	}
	if k := nodeNamed(t, d, "Keep"); !k.Found {
		t.Fatalf("the DM's Keep = %+v", k)
	}
	refuse("already has that map", live.Command{Kind: live.CmdFindMap, MapID: id(crypt), On: true})
	lit := len(p.Revealed)

	// With the world map every place on it shows, dimmed where the party has not been: all but the secret one.
	version := p.Map.ImageVersion
	d, p = say(live.Command{Kind: live.CmdFindMap, MapID: id(realm), On: true})
	if got := names(p); !p.Found || !d.Found || !slices.Equal(got, []string{"Keep", "Mill", "Oakford", "Tower", "Vault"}) || len(p.Routes) != 1 || len(p.Revealed) != lit || p.Map.ImageVersion == version {
		t.Fatalf("with the world map the party sees %v %+v", got, p)
	}
	if got := names(d); len(got) != 7 || len(d.Routes) != 2 {
		t.Fatalf("the DM sees %v", got)
	}
	// Losing it again darkens the map, and the picture's address changes back with it.
	_, p = say(live.Command{Kind: live.CmdFindMap, MapID: id(realm)})
	if got := names(p); p.Found || slices.Contains(got, "Tower") || !slices.Contains(got, "Keep") || len(p.Revealed) != lit || p.Map.ImageVersion != version {
		t.Fatalf("without the world map again the party sees %v %+v", got, p)
	}
	say(live.Command{Kind: live.CmdFindMap, MapID: id(crypt)})
	_, p = say(live.Command{Kind: live.CmdFindMap, MapID: id(realm), On: true})
	if k := nodeNamed(t, p, "Keep"); k.Found || k.MapID != "" {
		t.Fatalf("a map the party lost again = %+v", k)
	}

	// Even walking to the secret place and standing there, the party's screens are not told of it: the
	// journey they are shown names neither end of that leg, though the DM's does.
	say(live.Command{Kind: live.CmdPlaceParty, NodeID: oak})
	hidden := d.Routes[slices.IndexFunc(d.Routes, func(r live.RouteView) bool { return r.DistanceMi == 6 })]
	d, p = say(live.Command{Kind: live.CmdTravel, RouteID: hidden.ID, Pace: "normal"})
	if d.PartyNodeID != lair.ID || p.PartyNodeID != "" || slices.Contains(names(p), "Lair") {
		t.Fatalf("at the Lair: dm %+v, player %v at %q", d.PartyNodeID, names(p), p.PartyNodeID)
	}
	if last := d.Legs[len(d.Legs)-1]; last.From != "Oakford" || last.To != "Lair" || len(d.Legs) != 2 {
		t.Fatalf("the DM's journey = %+v", d.Legs)
	}
	if last := p.Legs[len(p.Legs)-1]; len(p.Legs) != 2 || last.From != "Oakford" || last.To != live.SecretPlace || last.DistanceMi != 6 || p.Legs[0].To != "Mill" {
		t.Fatalf("the party's journey = %+v", p.Legs)
	}
	_, p = say(live.Command{Kind: live.CmdTravel, RouteID: hidden.ID, Pace: "normal"})
	if last := p.Legs[len(p.Legs)-1]; last.From != live.SecretPlace || last.To != "Oakford" {
		t.Fatalf("the party's way back = %+v", p.Legs)
	}
	say(live.Command{Kind: live.CmdPlaceParty, NodeID: lair.ID})
	lit = len(p.Revealed)

	for _, frame := range seen {
		if strings.Contains(frame, "Lair") || strings.Contains(frame, lair.ID) || strings.Contains(frame, "Den") || strings.Contains(frame, id(attic)) || strings.Contains(frame, `"secret"`) {
			t.Fatalf("the secret place reached a player's screen: %s", frame)
		}
	}
	// It is all kept: a new runtime reads the same world.
	w.hub.Close(w.session.ID)
	again := join(t, w, w.player, playerCaller, live.AudienceParty)
	w.hub.Submit(again, live.Command{Kind: live.CmdResync})
	u := next(t, again)
	if got := names(u.View.World); !u.View.World.Found || !slices.Equal(got, []string{"Keep", "Mill", "Oakford", "Tower", "Vault"}) || len(u.View.World.Revealed) != lit {
		t.Fatalf("after a restart the party sees %v %+v", got, u.View.World)
	}
	if legs := u.View.World.Legs; len(legs) != 3 || legs[1].To != live.SecretPlace || legs[2].From != live.SecretPlace {
		t.Fatalf("after a restart the party's journey = %+v", legs)
	}
	if raw, _ := json.Marshal(u); strings.Contains(string(raw), "Lair") {
		t.Fatalf("after a restart the secret place reached a player's screen: %s", raw)
	}
	back := join(t, w, w.dm, dmCaller, live.AudienceDM)
	w.hub.Submit(back, live.Command{Kind: live.CmdResync})
	if dv := next(t, back).View.World; !nodeNamed(t, dv, "Lair").Secret || nodeNamed(t, dv, "Keep").MapID != id(crypt) || nodeNamed(t, dv, "Keep").Found {
		t.Fatalf("after a restart the DM sees %+v", dv.Nodes)
	}
}
