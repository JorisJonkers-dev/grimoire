package pgstore

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// LoadWorld reads a world map with its locations, routes and the party, and the Session's Travel Legs on it.
func (s *Store) LoadWorld(ctx context.Context, campaign uuid.UUID, sid domain.SessionID, id domain.MapID) (*domain.World, error) {
	board, err := s.LoadMap(ctx, campaign, id)
	if err != nil {
		return nil, err
	}
	w := &domain.World{Map: board.Map, Nodes: []domain.WorldNode{}, Routes: []domain.WorldRoute{}, Reveals: board.Reveals, Legs: []domain.TravelLeg{}}
	mid := uuid.UUID(id)
	nodes, err := s.q.MapNodes(ctx, mid)
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		w.Nodes = append(w.Nodes, domain.WorldNode{ID: domain.NodeID(n.ID), Name: n.Name, At: hex.Coord{Q: int(n.Q), R: int(n.R)}})
	}
	edges, err := s.q.MapEdges(ctx, mid)
	if err != nil {
		return nil, err
	}
	for _, e := range edges {
		w.Routes = append(w.Routes, domain.WorldRoute{ID: domain.RouteID(e.ID), From: domain.NodeID(e.FromNodeID), To: domain.NodeID(e.ToNodeID), DistanceMi: int(e.DistanceMi)})
	}
	party, err := s.q.MapParty(ctx, mid)
	if err != nil {
		return nil, err
	}
	for _, p := range party {
		n := domain.NodeID(p)
		w.Party = &n
	}
	legs, err := s.q.SessionTravelLegs(ctx, queries.SessionTravelLegsParams{SessionID: uuid.UUID(sid), MapID: mid})
	if err != nil {
		return nil, err
	}
	for _, l := range legs {
		w.Legs = append(w.Legs, domain.TravelLeg{From: l.FromName, To: l.ToName, Pace: l.Pace, DistanceMi: int(l.DistanceMi), Minutes: int(l.Minutes), Days: int(l.Days)})
	}
	return w, nil
}

// writeWorld stores a change to the world map.
//
//nolint:gosec // coordinates and distances are bounded by the API
func (s *Store) writeWorld(ctx context.Context, sid uuid.UUID, w live.Write) error {
	mid := uuid.UUID(w.WorldMap)
	switch w.Kind {
	case domain.ActionWorldSet:
		p := queries.SetSessionWorldParams{ID: sid}
		if w.WorldMapID != nil {
			p.MapID = pgtype.UUID{Bytes: *w.WorldMapID, Valid: true}
		}
		return s.q.SetSessionWorld(ctx, p)
	case domain.ActionNodeAdded:
		n := w.Node
		return s.q.InsertNode(ctx, queries.InsertNodeParams{ID: uuid.UUID(n.ID), MapID: mid, Name: n.Name, Q: int32(n.At.Q), R: int32(n.At.R)})
	case domain.ActionNodeRemoved:
		return s.q.DeleteNode(ctx, queries.DeleteNodeParams{MapID: mid, ID: uuid.UUID(w.Node.ID)})
	case domain.ActionRouteAdded:
		r := w.Route
		return s.q.InsertEdge(ctx, queries.InsertEdgeParams{
			ID: uuid.UUID(r.ID), MapID: mid, FromNodeID: uuid.UUID(r.From), ToNodeID: uuid.UUID(r.To), DistanceMi: int32(r.DistanceMi),
		})
	case domain.ActionRouteRemoved:
		return s.q.DeleteEdge(ctx, queries.DeleteEdgeParams{MapID: mid, ID: uuid.UUID(w.Route.ID)})
	default:
		if err := s.q.SetMapParty(ctx, queries.SetMapPartyParams{MapID: mid, NodeID: uuid.UUID(w.Node.ID)}); err != nil {
			return err
		}
		return s.addReveals(ctx, w.WorldMap, w.WorldReveal)
	}
}

// logLeg records a Travel Leg against its Action.
//
//nolint:gosec // distances and times are bounded by the rules
func (s *Store) logLeg(ctx context.Context, actionID, sid uuid.UUID, w live.Write) error {
	l := w.Leg
	return s.q.InsertTravelLeg(ctx, queries.InsertTravelLegParams{
		ActionID: actionID, SessionID: sid, MapID: uuid.UUID(w.WorldMap), FromName: l.From, ToName: l.To, Pace: l.Pace,
		DistanceMi: int32(l.DistanceMi), Minutes: int32(l.Minutes), Days: int32(l.Days),
	})
}
