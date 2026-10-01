package live

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/imaging"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/travel"
)

func parseID(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}

// planWorld changes the world map: which one the Session travels, its locations and routes, and where
// the party stands.
func (r *runtime) planWorld(cmd Command) (Write, string) {
	if cmd.Kind == CmdSetWorld {
		return r.planSetWorld(cmd)
	}
	w := r.st.world
	if w == nil {
		return Write{}, "Choose a world map first."
	}
	switch cmd.Kind {
	case CmdAddNode:
		return r.planNode(cmd)
	case CmdAddRoute:
		return planRoute(w, cmd)
	case CmdRemoveRoute:
		route, ok := w.Route(domain.RouteID(parseID(cmd.RouteID)))
		if !ok {
			return Write{}, "No such route."
		}
		return Write{Kind: domain.ActionRouteRemoved, Route: route}, ""
	case CmdTravel:
		leg, reason := planTravel(w, cmd)
		if reason == "" {
			day := r.st.day + leg.Leg.Days
			leg.Day = &day
		}
		return leg, reason
	}
	n, ok := w.Node(domain.NodeID(parseID(cmd.NodeID)))
	if !ok {
		return Write{}, "No such location."
	}
	kind := domain.ActionPartyPlaced
	if cmd.Kind == CmdRemoveNode {
		kind = domain.ActionNodeRemoved
	}
	return Write{Kind: kind, Node: n}, ""
}

func (r *runtime) planSetWorld(cmd Command) (Write, string) {
	if cmd.MapID == "" {
		return Write{Kind: domain.ActionWorldSet}, ""
	}
	id := domain.MapID(parseID(cmd.MapID))
	world, err := r.store.LoadWorld(context.Background(), r.st.session.CampaignID, r.st.session.ID, id)
	if err != nil || world.Map.Kind != domain.MapWorld {
		return Write{}, "Choose a world map."
	}
	return Write{Kind: domain.ActionWorldSet, WorldMapID: &id, world: world}, ""
}

func (r *runtime) planNode(cmd Command) (Write, string) {
	name, at := strings.TrimSpace(cmd.Label), hex.Coord{Q: cmd.Q, R: cmd.R}
	switch {
	case name == "" || utf8.RuneCountInString(name) > 40:
		return Write{}, "Give the location a name of up to 40 characters."
	case !r.st.worldCells[at]:
		return Write{}, "That hex is off the map."
	case slices.ContainsFunc(r.st.world.Nodes, func(n domain.WorldNode) bool { return n.At == at }):
		return Write{}, "A location already stands there."
	}
	return Write{Kind: domain.ActionNodeAdded, Node: domain.WorldNode{ID: domain.NodeID(uuid.New()), Name: name, At: at}}, ""
}

func joins(route domain.WorldRoute, a, b domain.NodeID) bool {
	return (route.From == a && route.To == b) || (route.From == b && route.To == a)
}

func planRoute(w *domain.World, cmd Command) (Write, string) {
	from, ok := w.Node(domain.NodeID(parseID(cmd.NodeID)))
	to, ok2 := w.Node(domain.NodeID(parseID(cmd.ToNodeID)))
	switch {
	case !ok || !ok2 || from.ID == to.ID:
		return Write{}, "Join two different locations."
	case cmd.DistanceMi < 1 || cmd.DistanceMi > 2000:
		return Write{}, "Routes run from 1 to 2000 miles."
	case slices.ContainsFunc(w.Routes, func(x domain.WorldRoute) bool { return joins(x, from.ID, to.ID) }):
		return Write{}, "Those locations are already joined."
	}
	return Write{Kind: domain.ActionRouteAdded, Route: domain.WorldRoute{ID: domain.RouteID(uuid.New()), From: from.ID, To: to.ID, DistanceMi: cmd.DistanceMi}}, ""
}

// planTravel is a Travel Leg: the party walks a route from where it stands to the other end.
func planTravel(w *domain.World, cmd Command) (Write, string) {
	route, ok := w.Route(domain.RouteID(parseID(cmd.RouteID)))
	pace, known := travel.ParsePace(cmd.Pace)
	switch {
	case !ok:
		return Write{}, "No such route."
	case w.Party == nil:
		return Write{}, "Place the party on the world map first."
	case route.From != *w.Party && route.To != *w.Party:
		return Write{}, "The party is at neither end of that route."
	case !known:
		return Write{}, "Travel at a slow, normal or fast pace."
	}
	here, _ := w.Node(*w.Party)
	dest := route.To
	if dest == here.ID {
		dest = route.From
	}
	there, _ := w.Node(dest)
	p := travel.Plan(route.DistanceMi, pace)
	leg := domain.TravelLeg{From: here.Name, To: there.Name, Pace: pace.String(), DistanceMi: route.DistanceMi, Minutes: p.Minutes, Days: p.Days}
	return Write{Kind: domain.ActionTravelLeg, Node: there, Leg: &leg}, ""
}

// setWorld switches the world map and works out which hexes it has.
func (s *state) setWorld(w *domain.World) {
	s.world, s.worldCells = w, nil
	if w == nil {
		return
	}
	s.worldCells = map[hex.Coord]bool{}
	for _, c := range imaging.Cells(w.Map.Layout(), w.Map.Width, w.Map.Height) {
		s.worldCells[c] = true
	}
}

// applyWorld changes the world map; arriving somewhere shows the party the land around it.
func applyWorld(s *state, w *Write) {
	if w.Kind == domain.ActionWorldSet {
		s.session.WorldMapID = w.WorldMapID
		s.setWorld(w.world)
		return
	}
	world := s.world
	w.WorldMap = world.Map.ID
	switch w.Kind {
	case domain.ActionNodeAdded:
		world.Nodes = append(world.Nodes, w.Node)
	case domain.ActionNodeRemoved:
		world.Nodes = slices.DeleteFunc(world.Nodes, func(n domain.WorldNode) bool { return n.ID == w.Node.ID })
		world.Routes = slices.DeleteFunc(world.Routes, func(x domain.WorldRoute) bool { return x.From == w.Node.ID || x.To == w.Node.ID })
		if world.Party != nil && *world.Party == w.Node.ID {
			world.Party = nil
		}
	case domain.ActionRouteAdded:
		world.Routes = append(world.Routes, w.Route)
	case domain.ActionRouteRemoved:
		world.Routes = slices.DeleteFunc(world.Routes, func(x domain.WorldRoute) bool { return x.ID == w.Route.ID })
	default:
		arrive(s, w)
	}
}

func arrive(s *state, w *Write) {
	world := s.world
	id := w.Node.ID
	world.Party = &id
	for _, c := range hex.Disk(w.Node.At, travel.SightHexes) {
		if s.worldCells[c] && !world.Reveals[c] {
			world.Reveals[c] = true
			w.WorldReveal = append(w.WorldReveal, c)
		}
	}
	if w.Leg != nil {
		world.Legs = append(world.Legs, *w.Leg)
	}
}

// worldView shows the world map to one audience. Players and the Table get the locations the party has
// seen or can reach from where it stands, and only routes between two of those.
func (s *state) worldView(a Audience) *WorldView {
	w := s.world
	if w == nil {
		return nil
	}
	v := &WorldView{Map: mapView(w.Map, len(w.Reveals)), Revealed: hexes(w.Reveals), Nodes: []NodeView{}, Routes: []RouteView{}, Legs: []LegView{}}
	known := knownNodes(w, a)
	for _, n := range w.Nodes {
		if known[n.ID] {
			v.Nodes = append(v.Nodes, NodeView{ID: uuid.UUID(n.ID).String(), Name: n.Name, Q: n.At.Q, R: n.At.R})
		}
	}
	for _, x := range w.Routes {
		if known[x.From] && known[x.To] {
			v.Routes = append(v.Routes, routeView(x))
		}
	}
	if w.Party != nil {
		v.PartyNodeID = uuid.UUID(*w.Party).String()
	}
	for _, l := range w.Legs {
		v.Legs = append(v.Legs, LegView{From: l.From, To: l.To, Pace: l.Pace, DistanceMi: l.DistanceMi, Minutes: l.Minutes, Days: l.Days})
	}
	return v
}

func knownNodes(w *domain.World, a Audience) map[domain.NodeID]bool {
	known := map[domain.NodeID]bool{}
	for _, n := range w.Nodes {
		known[n.ID] = a == AudienceDM || w.Reveals[n.At]
	}
	if w.Party == nil {
		return known
	}
	for _, x := range w.Routes {
		if x.From == *w.Party || x.To == *w.Party {
			known[x.From], known[x.To] = true, true
		}
	}
	return known
}

func routeView(x domain.WorldRoute) RouteView {
	v := RouteView{ID: uuid.UUID(x.ID).String(), FromNodeID: uuid.UUID(x.From).String(), ToNodeID: uuid.UUID(x.To).String(), DistanceMi: x.DistanceMi}
	for _, p := range []travel.Pace{travel.Slow, travel.Normal, travel.Fast} {
		leg := travel.Plan(x.DistanceMi, p)
		v.Plans = append(v.Plans, PlanView{Pace: p.String(), Minutes: leg.Minutes, Days: leg.Days})
	}
	return v
}

// mapView is a Map's geometry and its picture, versioned by how much of it the party has seen.
func mapView(m domain.Map, version int) MapView {
	return MapView{
		ID: uuid.UUID(m.ID).String(), Name: m.Name, Width: m.Width, Height: m.Height, HexSizePx: m.HexSize, OriginX: m.OriginX, OriginY: m.OriginY,
		ImageVersion: version, ImageURL: "/api/v1/campaigns/" + m.CampaignID.String() + "/maps/" + uuid.UUID(m.ID).String() + "/image?v=" + strconv.Itoa(version),
	}
}
