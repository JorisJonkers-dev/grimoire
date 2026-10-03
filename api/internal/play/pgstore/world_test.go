package pgstore_test

import (
	"context"
	"errors"
	"slices"
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

// What the party has found is kept: the world map, the local Maps at its locations, and which places
// are secret. A Map of another Campaign is never found from here.
func TestFoundMapsAndSecretPlacesAreKept(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	s, _ := sessions(tb, pgstore.New(tb.pool), &closed{}).Start(ctx, dm, tb.campaign)
	store := pgstore.New(tb.pool)
	picture := func(name, kind string) domain.MapID {
		t.Helper()
		m, err := store.InsertMap(ctx, domain.Map{CampaignID: tb.campaign, Name: name, Kind: kind, ImageKey: "k", ImageType: "image/png", Width: 400, Height: 300, Grid: "hexes", GridStrength: 20, ScaleMiles: 6, HexSize: 40, OriginX: 35, OriginY: 40}, time.Now())
		if err != nil || m.Found {
			t.Fatalf("%s = %+v %v", name, m, err)
		}
		return m.ID
	}
	realm, crypt := picture("Realm", domain.MapWorld), picture("Crypt", domain.MapLocal)
	foreign := uuid.New()
	if _, err := tb.pool.Exec(ctx, `WITH other AS (INSERT INTO campaign.campaigns (name, ruleset_pref, created_by, created_at, updated_at)
			VALUES ('Elsewhere', 'srd-2024', 'someone', now(), now()) RETURNING id)
		INSERT INTO campaign.maps (id, campaign_id, name, kind, image_key, image_type, width_px, height_px, hex_size_px, origin_x, origin_y, created_at, updated_at)
		SELECT $1, other.id, 'Theirs', 'world', 'k', 'image/png', 400, 300, 40, 35, 40, now(), now() FROM other`, foreign); err != nil {
		t.Fatal(err)
	}
	commit := func(w live.Write) {
		t.Helper()
		if _, err := store.Commit(ctx, s, nil, w, tb.dmMember(t), dm, time.Now()); err != nil {
			t.Fatalf("%s: %v", w.Kind, err)
		}
	}
	lair := domain.WorldNode{ID: domain.NodeID(uuid.New()), Name: "Lair", At: hex.Coord{Q: 1, R: 1}, Secret: true}
	keep := domain.WorldNode{ID: domain.NodeID(uuid.New()), Name: "Keep", At: hex.Coord{Q: 2, R: 2}, LocalMap: &crypt}
	commit(live.Write{Kind: domain.ActionWorldSet, WorldMapID: &realm})
	commit(live.Write{Kind: domain.ActionNodeAdded, WorldMap: realm, Node: lair})
	commit(live.Write{Kind: domain.ActionNodeAdded, WorldMap: realm, Node: keep, WorldReveal: []hex.Coord{{Q: 2, R: 2}}})
	read := func() (*domain.World, domain.WorldNode, domain.WorldNode) {
		t.Helper()
		w, err := store.LoadWorld(ctx, tb.campaign, s.ID, realm)
		if err != nil || len(w.Nodes) != 2 {
			t.Fatalf("world = %+v %v", w, err)
		}
		return w, w.Nodes[0], w.Nodes[1]
	}
	w, k, l := read()
	if w.Map.Found || !l.Secret || l.LocalMap != nil || l.LocalFound || k.Secret || k.LocalMap == nil || *k.LocalMap != crypt || k.LocalFound || len(w.Reveals) != 1 {
		t.Fatalf("before anything is found: %+v %+v %+v", w.Map, k, l)
	}
	commit(live.Write{Kind: domain.ActionMapFound, WorldMap: realm, Found: &domain.FoundMap{Map: crypt, On: true}, WorldReveal: []hex.Coord{{Q: 3, R: 2}}})
	if w, k, _ = read(); w.Map.Found || !k.LocalFound || len(w.Reveals) != 2 {
		t.Fatalf("the Crypt found: %+v %+v", w.Map, k)
	}
	commit(live.Write{Kind: domain.ActionMapFound, WorldMap: realm, Found: &domain.FoundMap{Map: realm, On: true}})
	commit(live.Write{Kind: domain.ActionMapLost, WorldMap: realm, Found: &domain.FoundMap{Map: crypt}})
	if w, k, _ = read(); !w.Map.Found || k.LocalFound || len(w.Reveals) != 2 {
		t.Fatalf("the Realm found and the Crypt lost: %+v %+v", w.Map, k)
	}
	commit(live.Write{Kind: domain.ActionMapFound, WorldMap: realm, Found: &domain.FoundMap{Map: domain.MapID(foreign), On: true}})
	var theirs bool
	if err := tb.pool.QueryRow(ctx, "SELECT found FROM campaign.maps WHERE id = $1", foreign).Scan(&theirs); err != nil || theirs {
		t.Fatalf("another Campaign's map found from here: %v %v", theirs, err)
	}
	// A local Map lies at one place only; its location forgets it when the Map goes.
	twin := domain.WorldNode{ID: domain.NodeID(uuid.New()), Name: "Twin", At: hex.Coord{Q: 0, R: 2}, LocalMap: &crypt}
	if _, err := store.Commit(ctx, s, nil, live.Write{Kind: domain.ActionNodeAdded, WorldMap: realm, Node: twin}, tb.dmMember(t), dm, time.Now()); err == nil {
		t.Fatal("one local Map at two places")
	}
	fresh := 0
	for name, x := range map[string]live.Write{
		"a place with a map": {Kind: domain.ActionNodeAdded, WorldMap: realm, Node: domain.WorldNode{Name: "New", Secret: true}, WorldReveal: []hex.Coord{{Q: 0, R: 0}}},
		"a map found":        {Kind: domain.ActionMapFound, WorldMap: realm, Found: &domain.FoundMap{Map: crypt, On: true}, WorldReveal: []hex.Coord{{Q: 0, R: 1}}},
	} {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			y := x
			fresh++
			y.Node.ID, y.Node.At = domain.NodeID(uuid.New()), hex.Coord{Q: 3, R: fresh}
			_, err := pgstore.NewFaulty(tb.pool, f).Commit(ctx, s, nil, y, tb.dmMember(t), dm, time.Now())
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
	if _, err := tb.pool.Exec(ctx, "DELETE FROM campaign.maps WHERE id = $1", uuid.UUID(crypt)); err != nil {
		t.Fatal(err)
	}
	if w, _ := store.LoadWorld(ctx, tb.campaign, s.ID, realm); slices.ContainsFunc(w.Nodes, func(n domain.WorldNode) bool { return n.LocalMap != nil || n.LocalFound }) {
		t.Fatalf("a place kept a Map that is gone: %+v", w.Nodes)
	}
}
