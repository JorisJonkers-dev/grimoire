package pgstore_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/vehicles"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

func gull() vehicles.Design {
	return vehicles.Design{
		Name: "  The Gull ", Kind: vehicles.Water, HullMax: 300, Threshold: 15, MilesPerDay: 48,
		Components: []vehicles.Component{{Name: "Mainsail", HPMax: 100, Drives: true}, {Name: "Foresail", HPMax: 80, Drives: true}, {Name: "Helm", HPMax: 50}},
		Stations:   []vehicles.Station{{Name: "Helm", Crew: 1}, {Name: "Rigging", Crew: 6}, {Name: "Lookout", Crew: 0}},
	}
}

// lingeringVehicles makes every change wait, once it has read the vehicles, until the others have read too.
type lingeringVehicles struct {
	app.VehicleStore
	read func()
}

type lingeringVehicleWriter struct {
	app.VehicleWriter
	read func()
}

func (l lingeringVehicleWriter) Vehicles(ctx context.Context, campaign uuid.UUID) ([]domain.Vehicle, error) {
	list, err := l.VehicleWriter.Vehicles(ctx, campaign)
	l.read()
	return list, err
}

func (l lingeringVehicles) WriteVehicles(ctx context.Context, fn func(app.VehicleWriter) error) error {
	return l.VehicleStore.WriteVehicles(ctx, func(w app.VehicleWriter) error {
		return fn(lingeringVehicleWriter{VehicleWriter: w, read: l.read})
	})
}

// The DM builds vehicles and ships, damages and repairs their hulls and components and posts crew to
// their stations. A vehicle goes slower for every broken drive and at half speed with a short crew,
// and not at all as a wreck. Every Member sees them; only the DM changes them.
func TestVehiclesHullsComponentsAndCrew(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	members := pgstore.CampaignMembers{Store: campaignpg.New(tb.pool)}
	s := &app.Vehicles{Store: pgstore.New(tb.pool), Members: members, Now: time.Now}
	var elsewhere uuid.UUID
	if err := tb.pool.QueryRow(ctx, `INSERT INTO campaign.campaigns (name, ruleset_pref, created_by, created_at, updated_at) VALUES ('Elsewhere', 'srd-2024', 'dm', now(), now()) RETURNING id`).Scan(&elsewhere); err != nil {
		t.Fatal(err)
	}
	if _, err := tb.pool.Exec(ctx, `INSERT INTO campaign.members (campaign_id, auth_subject, display_name, role) VALUES ($1, 'dm', 'Joris', 'dm')`, elsewhere); err != nil {
		t.Fatal(err)
	}
	refusedWith := func(err error, want string) bool {
		var rule *apperr.RuleError
		return errors.As(err, &rule) && strings.Contains(rule.Reason, want)
	}
	only := func(campaign uuid.UUID) domain.Vehicle {
		t.Helper()
		v, err := s.List(ctx, player, campaign)
		if campaign == elsewhere {
			v, err = s.List(ctx, dm, campaign)
		}
		if err != nil || len(v.Vehicles) != 1 {
			t.Fatalf("the Campaign's one vehicle = %+v %v", v, err)
		}
		return v.Vehicles[0]
	}

	if v, err := s.List(ctx, player, tb.campaign); err != nil || v.DM || v.Vehicles == nil || len(v.Vehicles) != 0 {
		t.Fatalf("no vehicles yet, as a Player sees it = %+v %v", v, err)
	}
	made, err := s.Add(ctx, dm, tb.campaign, gull())
	if err != nil {
		t.Fatal(err)
	}
	// What building answers with is the vehicle as it was kept.
	if made.Name != "The Gull" || made.Hull != 300 || made.Parts[0].HP != 100 || made.Parts[1].HP != 80 || !made.Parts[0].Drives || made.Posts[1].Crew != 6 || made.Posts[1].Posted != 0 {
		t.Fatalf("the Gull as building answers = %+v", made)
	}
	// Built whole, with nobody aboard: named as given, its parts in the order they were listed.
	kept := only(tb.campaign)
	if kept.ID != made.ID || kept.Name != "The Gull" || kept.Kind != vehicles.Water || kept.Hull != 300 || kept.HullMax != 300 || kept.Threshold != 15 || kept.MilesPerDay != 48 {
		t.Fatalf("the Gull as built = %+v", kept)
	}
	if len(kept.Parts) != 3 || kept.Parts[0] != (domain.VehiclePart{ID: made.Parts[0].ID, Name: "Mainsail", HP: 100, HPMax: 100, Drives: true}) || kept.Parts[1].Name != "Foresail" || kept.Parts[2] != (domain.VehiclePart{ID: made.Parts[2].ID, Name: "Helm", HP: 50, HPMax: 50}) {
		t.Fatalf("its components = %+v", kept.Parts)
	}
	if len(kept.Posts) != 3 || kept.Posts[0] != (domain.VehiclePost{ID: made.Posts[0].ID, Name: "Helm", Crew: 1}) || kept.Posts[1] != (domain.VehiclePost{ID: made.Posts[1].ID, Name: "Rigging", Crew: 6}) || kept.Posts[2].Name != "Lookout" {
		t.Fatalf("its crew stations = %+v", kept.Posts)
	}
	if v, err := s.List(ctx, dm, tb.campaign); err != nil || !v.DM || len(v.Vehicles) != 1 {
		t.Fatalf("as the DM sees it = %+v %v", v, err)
	}
	// Nobody at the helm or in the rigging: half speed. The lookout that takes no crew is never short.
	if kept.Speed() != 24 {
		t.Fatalf("unmanned, the Gull makes %d miles a day", kept.Speed())
	}
	mainsail, foresail, helm, rigging := made.Parts[0].ID, made.Parts[1].ID, made.Posts[0].ID, made.Posts[1].ID
	post := func(station uuid.UUID, n int) domain.Vehicle {
		t.Helper()
		v, err := s.Post(ctx, dm, tb.campaign, made.ID, station, n)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if v := post(helm, 1); v.Speed() != 24 || v.Posts[0].Posted != 1 {
		t.Fatalf("a helmsman alone = %+v at %d", v.Posts, v.Speed())
	}
	if v := post(rigging, 5); v.Speed() != 24 {
		t.Fatalf("one short in the rigging: %d", v.Speed())
	}
	if v := post(rigging, 6); v.Speed() != 48 || only(tb.campaign).Posts[1].Posted != 6 {
		t.Fatalf("fully crewed = %+v at %d", v.Posts, v.Speed())
	}
	strike := func(part *uuid.UUID, amount int, repair bool) domain.Vehicle {
		t.Helper()
		v, err := s.Strike(ctx, dm, tb.campaign, made.ID, app.VehicleBlow{Part: part, Amount: amount, Repair: repair})
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	// A blow under the threshold glances off the hull; one at it lands in full, and is kept.
	if v := strike(nil, 14, false); v.Hull != 300 {
		t.Fatalf("14 damage against a threshold of 15: %d", v.Hull)
	}
	if v := strike(nil, 15, false); v.Hull != 285 || only(tb.campaign).Hull != 285 {
		t.Fatalf("15 damage: %d, kept %d", v.Hull, only(tb.campaign).Hull)
	}
	// A component has hit points of its own, behind the same threshold. A sail gone takes half the speed.
	if v := strike(&mainsail, 14, false); v.Parts[0].HP != 100 {
		t.Fatalf("14 damage to the mainsail: %d", v.Parts[0].HP)
	}
	if v := strike(&mainsail, 60, false); v.Parts[0].HP != 40 || v.Parts[1].HP != 80 || v.Hull != 285 || v.Speed() != 48 {
		t.Fatalf("60 damage to the mainsail = %+v", v)
	}
	if v := strike(&mainsail, 60, false); v.Parts[0].HP != 0 || v.Speed() != 24 || only(tb.campaign).Parts[0].HP != 0 {
		t.Fatalf("the mainsail gone = %+v at %d", v.Parts, v.Speed())
	}
	if v := strike(&foresail, 200, false); v.Speed() != 0 {
		t.Fatalf("no sail left: %d", v.Speed())
	}
	// Repairs give hit points back, up to what the part can have.
	if v := strike(&mainsail, 30, true); v.Parts[0].HP != 30 || v.Speed() != 24 {
		t.Fatalf("the mainsail patched = %+v at %d", v.Parts[0], v.Speed())
	}
	if v := strike(&foresail, 500, true); v.Parts[1].HP != 80 || v.Speed() != 48 {
		t.Fatalf("the foresail made new = %+v at %d", v.Parts[1], v.Speed())
	}
	if v := strike(nil, 5, true); v.Hull != 290 {
		t.Fatalf("5 repaired on the hull: %d", v.Hull)
	}
	if v := strike(nil, 10000, true); v.Hull != 300 || only(tb.campaign).Hull != 300 {
		t.Fatalf("the hull made whole: %d", v.Hull)
	}
	// Holed through, the Gull is a wreck and goes nowhere, whatever its sails and crew.
	if v := strike(nil, 10000, false); v.Hull != 0 || v.Speed() != 0 {
		t.Fatalf("the Gull wrecked = %d at %d", v.Hull, v.Speed())
	}
	strike(nil, 300, true)

	// What the rules refuse, with the reason.
	blow := app.VehicleBlow{Amount: 1}
	for want, err := range map[string]error{
		"Give the vehicle a name of up to 80 characters.": func() error { d := gull(); d.Name = "   "; _, err := s.Add(ctx, dm, tb.campaign, d); return err }(),
		"A vehicle goes by land, water or air.":           func() error { d := gull(); d.Kind = "rail"; _, err := s.Add(ctx, dm, tb.campaign, d); return err }(),
		"damage and repairs run from 1 to 10000 hit points": func() error {
			_, err := s.Strike(ctx, dm, tb.campaign, made.ID, app.VehicleBlow{Amount: 0})
			return err
		}(),
		"damage and repairs run from 1 to 10000 hit points ": func() error {
			_, err := s.Strike(ctx, dm, tb.campaign, made.ID, app.VehicleBlow{Amount: 10001, Repair: true})
			return err
		}(),
		"post no more crew than the station takes":  func() error { _, err := s.Post(ctx, dm, tb.campaign, made.ID, rigging, 7); return err }(),
		"post no more crew than the station takes ": func() error { _, err := s.Post(ctx, dm, tb.campaign, made.ID, rigging, -1); return err }(),
	} {
		if !refusedWith(err, strings.TrimSpace(want)) {
			t.Errorf("want %q, got %v", want, err)
		}
	}
	if _, err := s.Strike(ctx, dm, tb.campaign, made.ID, app.VehicleBlow{Amount: 10000}); err != nil {
		t.Errorf("the hardest blow there is: %v", err)
	}
	strike(nil, 300, true)

	// Only the DM changes a vehicle; a stranger sees nothing; nothing reaches through another Campaign.
	forbidden := map[string]error{}
	_, forbidden["build"] = s.Add(ctx, player, tb.campaign, gull())
	forbidden["remove"] = s.Remove(ctx, player, tb.campaign, made.ID)
	_, forbidden["strike"] = s.Strike(ctx, player, tb.campaign, made.ID, blow)
	_, forbidden["post"] = s.Post(ctx, player, tb.campaign, made.ID, helm, 0)
	for name, err := range forbidden {
		if !errors.Is(err, apperr.ErrForbidden) {
			t.Errorf("a Player may %s: %v", name, err)
		}
	}
	other, err := s.Add(ctx, dm, elsewhere, gull())
	if err != nil {
		t.Fatal(err)
	}
	missing := map[string]error{}
	_, missing["a stranger lists"] = s.List(ctx, stranger, tb.campaign)
	_, missing["a stranger builds"] = s.Add(ctx, stranger, tb.campaign, gull())
	_, missing["no such vehicle struck"] = s.Strike(ctx, dm, tb.campaign, uuid.New(), blow)
	_, missing["no such component"] = s.Strike(ctx, dm, tb.campaign, made.ID, app.VehicleBlow{Part: &helm, Amount: 20})
	_, missing["another vehicle's component"] = s.Strike(ctx, dm, tb.campaign, made.ID, app.VehicleBlow{Part: &other.Parts[0].ID, Amount: 20})
	_, missing["no such station"] = s.Post(ctx, dm, tb.campaign, made.ID, mainsail, 0)
	_, missing["another vehicle's station"] = s.Post(ctx, dm, tb.campaign, made.ID, other.Posts[0].ID, 1)
	_, missing["no such vehicle crewed"] = s.Post(ctx, dm, tb.campaign, uuid.New(), helm, 0)
	_, missing["a vehicle struck through another Campaign"] = s.Strike(ctx, dm, elsewhere, made.ID, blow)
	_, missing["a vehicle crewed through another Campaign"] = s.Post(ctx, dm, elsewhere, made.ID, helm, 0)
	missing["a vehicle removed through another Campaign"] = s.Remove(ctx, dm, elsewhere, made.ID)
	missing["the removal of no vehicle"] = s.Remove(ctx, dm, tb.campaign, uuid.New())
	for name, err := range missing {
		if !errors.Is(err, apperr.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if here, there := only(tb.campaign), only(elsewhere); here.Hull != 300 || here.Posts[0].Posted != 1 || there.Hull != 300 || there.Posts[0].Posted != 0 || there.Parts[0].HP != 100 {
		t.Fatalf("after all that was refused = %+v, %+v", here, there)
	}

	// Six blows at the same moment all land: none is worked out from a hull another has since holed.
	var arrived atomic.Int32
	all := make(chan struct{})
	crowd := &app.Vehicles{Members: members, Now: time.Now, Store: lingeringVehicles{VehicleStore: pgstore.New(tb.pool), read: func() {
		if arrived.Add(1) == 6 {
			close(all)
		}
		select {
		case <-all:
		case <-time.After(60 * time.Millisecond):
		}
	}}}
	var blows sync.WaitGroup
	for range 6 {
		blows.Go(func() {
			if _, err := crowd.Strike(ctx, dm, tb.campaign, made.ID, app.VehicleBlow{Amount: 20}); err != nil {
				t.Errorf("a blow among six: %v", err)
			}
		})
	}
	blows.Wait()
	if v := only(tb.campaign); v.Hull != 300-6*20 {
		t.Fatalf("six blows of 20 at once leave the hull at %d", v.Hull)
	}

	// Every operation reports a database fault, and a vehicle is built whole or not at all.
	svc := func(f *pgtest.Faulty) *app.Vehicles {
		return &app.Vehicles{Store: pgstore.NewFaulty(tb.pool, f), Members: members, Now: time.Now}
	}
	for name, op := range map[string]func(f *pgtest.Faulty) error{
		"list":  func(f *pgtest.Faulty) error { _, err := svc(f).List(ctx, player, tb.campaign); return err },
		"build": func(f *pgtest.Faulty) error { _, err := svc(f).Add(ctx, dm, elsewhere, gull()); return err },
		"hull":  func(f *pgtest.Faulty) error { _, err := svc(f).Strike(ctx, dm, tb.campaign, made.ID, blow); return err },
		"part": func(f *pgtest.Faulty) error {
			_, err := svc(f).Strike(ctx, dm, tb.campaign, made.ID, app.VehicleBlow{Part: &mainsail, Amount: 1, Repair: true})
			return err
		},
		"post": func(f *pgtest.Faulty) error {
			_, err := svc(f).Post(ctx, dm, tb.campaign, made.ID, helm, 1)
			return err
		},
		"remove": func(f *pgtest.Faulty) error { return svc(f).Remove(ctx, dm, elsewhere, other.ID) },
	} {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := op(f)
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
	there, err := s.List(ctx, dm, elsewhere)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range there.Vehicles {
		if v.ID == other.ID {
			t.Fatalf("the removed vehicle is still there")
		}
		if len(v.Parts) != 3 || len(v.Posts) != 3 {
			t.Fatalf("a vehicle kept with %d of its 3 components and %d of its 3 stations", len(v.Parts), len(v.Posts))
		}
	}

	// A Campaign keeps fifty vehicles and no more.
	if _, err := tb.pool.Exec(ctx, `INSERT INTO campaign.vehicles (id, campaign_id, name, kind, hull_hp, hull_max, threshold, miles_per_day, created_at)
		SELECT gen_random_uuid(), $1, 'Cart ' || n, 'land', 10, 10, 0, 24, now() FROM generate_series(1, $2::int) n`, tb.campaign, app.MaxVehicles-2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(ctx, dm, tb.campaign, gull()); err != nil {
		t.Fatalf("the fiftieth vehicle: %v", err)
	}
	if _, err := s.Add(ctx, dm, tb.campaign, gull()); !refusedWith(err, "up to 50 vehicles") {
		t.Errorf("the fifty-first: %v", err)
	}
	// Two built at the same moment with room for one: one is kept.
	if _, err := tb.pool.Exec(ctx, `DELETE FROM campaign.vehicles WHERE campaign_id = $1 AND name = 'Cart 1'`, tb.campaign); err != nil {
		t.Fatal(err)
	}
	var both atomic.Int32
	pair := make(chan struct{})
	builders := &app.Vehicles{Members: members, Now: time.Now, Store: lingeringVehicles{VehicleStore: pgstore.New(tb.pool), read: func() {
		if both.Add(1) == 2 {
			close(pair)
		}
		select {
		case <-pair:
		case <-time.After(60 * time.Millisecond):
		}
	}}}
	var built, turned atomic.Int32
	var building sync.WaitGroup
	for range 2 {
		building.Go(func() {
			_, err := builders.Add(ctx, dm, tb.campaign, gull())
			var no *apperr.RuleError
			switch {
			case err == nil:
				built.Add(1)
			case errors.As(err, &no):
				turned.Add(1)
			default:
				t.Errorf("building: %v", err)
			}
		})
	}
	building.Wait()
	if v, _ := s.List(ctx, dm, tb.campaign); built.Load() != 1 || turned.Load() != 1 || len(v.Vehicles) != app.MaxVehicles {
		t.Fatalf("two at once: %d built, %d refused, %d vehicles", built.Load(), turned.Load(), len(v.Vehicles))
	}

	// Each vehicle has its own components and stations, and no other's.
	fleet, err := s.List(ctx, dm, tb.campaign)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range fleet.Vehicles {
		want := 0
		if v.Name == "The Gull" {
			want = 3
		}
		if len(v.Parts) != want || len(v.Posts) != want {
			t.Fatalf("%s has %d components and %d stations, want %d of each", v.Name, len(v.Parts), len(v.Posts), want)
		}
	}

	// Removed, a vehicle takes its components and stations with it.
	if err := s.Remove(ctx, dm, tb.campaign, made.ID); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := tb.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM campaign.vehicle_components WHERE vehicle_id = $1) + (SELECT count(*) FROM campaign.vehicle_stations WHERE vehicle_id = $1)
		+ (SELECT count(*) FROM campaign.vehicles WHERE id = $1)`, made.ID).Scan(&left); err != nil || left != 0 {
		t.Fatalf("after removing the Gull: %d rows left, %v", left, err)
	}
}
