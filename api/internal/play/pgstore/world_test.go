package pgstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

func TestWorldMapsKeepTheirLocationsRoutesAndParty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	s, _ := sessions(tb, pgstore.New(tb.pool), &closed{}).Start(ctx, dm, tb.campaign)
	store := pgstore.New(tb.pool)
	m, err := store.InsertMap(ctx, domain.Map{CampaignID: tb.campaign, Name: "Realm", Kind: domain.MapWorld, ImageKey: "k", ImageType: "image/png", Width: 400, Height: 300, Grid: "hexes", GridStrength: 20, ScaleMiles: 6, HexSize: 40, OriginX: 35, OriginY: 40}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	mid := m.ID
	a := domain.WorldNode{ID: domain.NodeID(uuid.New()), Name: "Oakford", At: hex.Coord{Q: 0, R: 0}}
	b := domain.WorldNode{ID: domain.NodeID(uuid.New()), Name: "Mill", At: hex.Coord{Q: 3, R: 0}}
	road := domain.WorldRoute{ID: domain.RouteID(uuid.New()), From: a.ID, To: b.ID, DistanceMi: 12}
	leg := domain.TravelLeg{From: "Oakford", To: "Mill", Pace: "normal", DistanceMi: 12, Minutes: 240, Days: 1}
	writes := []live.Write{
		{Kind: domain.ActionWorldSet, WorldMapID: &mid},
		{Kind: domain.ActionNodeAdded, WorldMap: mid, Node: a},
		{Kind: domain.ActionNodeAdded, WorldMap: mid, Node: b},
		{Kind: domain.ActionRouteAdded, WorldMap: mid, Route: road},
		{Kind: domain.ActionPartyPlaced, WorldMap: mid, Node: a, WorldReveal: []hex.Coord{{Q: 0, R: 0}}},
		{Kind: domain.ActionTravelLeg, WorldMap: mid, Node: b, Leg: &leg, WorldReveal: []hex.Coord{{Q: 3, R: 0}}},
	}
	for _, w := range writes {
		if _, err := store.Commit(ctx, s, nil, w, tb.dmMember(t), dm, time.Now()); err != nil {
			t.Fatalf("%s: %v", w.Kind, err)
		}
	}
	sess, _, _, err := store.Load(ctx, s.ID)
	if err != nil || sess.WorldMapID == nil || *sess.WorldMapID != mid {
		t.Fatalf("session world = %+v %v", sess, err)
	}
	w, err := store.LoadWorld(ctx, tb.campaign, s.ID, mid)
	if err != nil || w.Map.Kind != domain.MapWorld || len(w.Nodes) != 2 || len(w.Routes) != 1 || w.Routes[0] != road || *w.Party != b.ID ||
		len(w.Reveals) != 2 || len(w.Legs) != 1 || w.Legs[0] != leg {
		t.Fatalf("world = %+v %v", w, err)
	}
	for _, x := range []live.Write{
		{Kind: domain.ActionRouteRemoved, WorldMap: mid, Route: road},
		{Kind: domain.ActionNodeRemoved, WorldMap: mid, Node: b},
		{Kind: domain.ActionWorldSet},
	} {
		if _, err := store.Commit(ctx, s, nil, x, tb.dmMember(t), dm, time.Now()); err != nil {
			t.Fatalf("%s: %v", x.Kind, err)
		}
	}
	w, _ = store.LoadWorld(ctx, tb.campaign, s.ID, mid)
	if sess, _, _, _ = store.Load(ctx, s.ID); sess.WorldMapID != nil || len(w.Nodes) != 1 || len(w.Routes) != 0 || w.Party != nil {
		t.Fatalf("after removals = %+v %+v", sess, w)
	}
	if _, err := store.LoadWorld(ctx, tb.campaign, s.ID, domain.MapID(uuid.New())); err == nil {
		t.Fatal("an unknown world map loaded")
	}
	for _, x := range writes[2:5] {
		if _, err := store.Commit(ctx, s, nil, x, tb.dmMember(t), dm, time.Now()); err != nil {
			t.Fatalf("%s again: %v", x.Kind, err)
		}
	}
	fresh := 0
	ops := map[string]func(repo *pgstore.Store) error{
		"load": func(repo *pgstore.Store) error { _, err := repo.LoadWorld(ctx, tb.campaign, s.ID, mid); return err },
	}
	for _, x := range writes {
		ops[x.Kind] = func(repo *pgstore.Store) error {
			y := x
			if y.Kind == domain.ActionNodeAdded || y.Kind == domain.ActionRouteAdded {
				fresh++
				y.Node.ID, y.Route.ID, y.Node.At = domain.NodeID(uuid.New()), domain.RouteID(uuid.New()), hex.Coord{Q: 1, R: fresh}
			}
			_, err := repo.Commit(ctx, s, nil, y, tb.dmMember(t), dm, time.Now())
			return err
		}
	}
	for name, op := range ops {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := op(pgstore.NewFaulty(tb.pool, f))
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}
