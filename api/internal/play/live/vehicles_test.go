package live_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/vehicles"
)

type noVehicles struct{ live.Store }

func (noVehicles) Vehicles(context.Context, uuid.UUID) ([]domain.Vehicle, error) {
	return nil, errors.New("no such table")
}

// The party travels the world map aboard a vehicle: at the speed the vehicle makes as it stands now,
// round the clock on a ship and by day in a wagon, and not at all in a wreck.
func TestThePartyTravelsAboardAVehicle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	campaign := w.session.CampaignID
	fleet := &app.Vehicles{Store: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Now: time.Now}
	gull, err := fleet.Add(ctx, dmCaller, campaign, vehicles.Design{
		Name: "The Gull", Kind: vehicles.Water, HullMax: 300, Threshold: 15, MilesPerDay: 48,
		Components: []vehicles.Component{{Name: "Sails", HPMax: 100, Drives: true}}, Stations: []vehicles.Station{{Name: "Helm", Crew: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	wagon, err := fleet.Add(ctx, dmCaller, campaign, vehicles.Design{Name: "Wagon", Kind: vehicles.Land, HullMax: 40, MilesPerDay: 24})
	if err != nil {
		t.Fatal(err)
	}
	dm := join(t, w, w.dm, dmCaller, live.AudienceDM)
	player := join(t, w, w.player, playerCaller, live.AudienceParty)
	say := func(cmd live.Command) (*live.View, *live.View) {
		t.Helper()
		w.hub.Submit(dm, cmd)
		d := next(t, dm)
		if d.Kind != live.UpdView {
			t.Fatalf("%s rejected: %+v", cmd.Kind, d)
		}
		return d.View, next(t, player).View
	}
	refuse := func(want string, cmd live.Command) {
		t.Helper()
		w.hub.Submit(dm, cmd)
		if u := next(t, dm); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, want) {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	say(live.Command{Kind: live.CmdSetWorld, MapID: uuid.UUID(w.realm(t).ID).String()})
	say(live.Command{Kind: live.CmdAddNode, Label: "Oakford", Q: 0, R: 0})
	d, _ := say(live.Command{Kind: live.CmdAddNode, Label: "Mill", Q: 5, R: 0})
	oak, mill := nodeNamed(t, d.World, "Oakford").ID, nodeNamed(t, d.World, "Mill").ID
	d, _ = say(live.Command{Kind: live.CmdAddRoute, NodeID: oak, ToNodeID: mill, DistanceMi: 48})
	sea := d.World.Routes[0].ID
	d, _ = say(live.Command{Kind: live.CmdPlaceParty, NodeID: oak})
	started := d.GameDay*1440 + d.GameMinute
	aboard := func(vehicle string) live.Command {
		return live.Command{Kind: live.CmdTravel, RouteID: sea, VehicleID: vehicle}
	}

	refuse("No such vehicle.", aboard(uuid.NewString()))
	refuse("No such vehicle.", aboard("the gull"))
	// Nobody at the helm: the Gull makes half its 48 miles a day, round the clock, so 48 miles take two days.
	d, p := say(aboard(gull.ID.String()))
	want := live.LegView{From: "Oakford", To: "Mill", Pace: "normal", DistanceMi: 48, Minutes: 2880, Days: 2, Vehicle: "The Gull"}
	if len(d.World.Legs) != 1 || d.World.Legs[0] != want || p.World.Legs[0] != want || d.World.PartyNodeID != mill {
		t.Fatalf("the first leg aboard = %+v", d.World.Legs)
	}
	if now := d.GameDay*1440 + d.GameMinute; now-started != 2880 {
		t.Fatalf("two days under way move the Game Clock %d minutes", now-started)
	}
	// With a helmsman it makes its speed: the vehicle is read as it stands when the party sets out.
	if _, err := fleet.Post(ctx, dmCaller, campaign, gull.ID, gull.Posts[0].ID, 1); err != nil {
		t.Fatal(err)
	}
	d, _ = say(aboard(gull.ID.String()))
	if l := d.World.Legs[1]; l.Minutes != 1440 || l.Days != 1 || l.From != "Mill" || l.Vehicle != "The Gull" || d.GameDay*1440+d.GameMinute-started != 2880+1440 {
		t.Fatalf("fully crewed = %+v", l)
	}
	// A wagon travels a day's eight hours and camps the night: 48 miles at 24 a day are two days on the
	// road with a night between them.
	d, _ = say(aboard(wagon.ID.String()))
	if l := d.World.Legs[2]; l.Minutes != 960 || l.Days != 2 || l.Vehicle != "Wagon" {
		t.Fatalf("by wagon = %+v", l)
	}
	if passed := d.GameDay*1440 + d.GameMinute - started - 2880 - 1440; passed <= 960 || passed >= 2*1440 {
		t.Fatalf("two days by wagon, with a night's camp, move the clock %d minutes", passed)
	}
	// On foot a leg names no vehicle, as before.
	d, _ = say(live.Command{Kind: live.CmdTravel, RouteID: sea, Pace: "fast"})
	if l := d.World.Legs[3]; l.Vehicle != "" || l.Pace != "fast" || l.Minutes != 720 {
		t.Fatalf("on foot = %+v", l)
	}
	// Its sails gone, the Gull goes nowhere; nor does a wreck.
	if _, err := fleet.Strike(ctx, dmCaller, campaign, gull.ID, app.VehicleBlow{Part: &gull.Parts[0].ID, Amount: 100}); err != nil {
		t.Fatal(err)
	}
	refuse("The Gull cannot move as it stands.", aboard(gull.ID.String()))
	if _, err := fleet.Strike(ctx, dmCaller, campaign, wagon.ID, app.VehicleBlow{Amount: 40}); err != nil {
		t.Fatal(err)
	}
	refuse("Wagon cannot move as it stands.", aboard(wagon.ID.String()))
	// The other refusals of a Travel Leg come first: a vehicle does not get the party onto a road it is not at.
	refuse("No such route.", live.Command{Kind: live.CmdTravel, RouteID: uuid.NewString(), VehicleID: gull.ID.String()})

	// The legs keep the vehicle's name across a restart, and after the vehicle is gone.
	if err := fleet.Remove(ctx, dmCaller, campaign, wagon.ID); err != nil {
		t.Fatal(err)
	}
	w.hub.Close(w.session.ID)
	dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	if legs := look(t, w, dm).World.Legs; len(legs) != 4 || legs[0] != want || legs[2].Vehicle != "Wagon" || legs[3].Vehicle != "" {
		t.Fatalf("the legs after a restart = %+v", legs)
	}

	// Vehicles that cannot be read stop a leg aboard, and not one on foot.
	w.hub.Close(w.session.ID)
	w.hub.Store = noVehicles{Store: w.hub.Store}
	dm, player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)
	refuse("No such vehicle.", aboard(gull.ID.String()))
	say(live.Command{Kind: live.CmdTravel, RouteID: sea, Pace: "slow"})
}
