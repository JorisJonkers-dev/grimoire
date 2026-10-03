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
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/clock"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/travel"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/vehicles"
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
	case CmdFindMap:
		return planFind(w, cmd)
	case CmdAddRoute:
		return planRoute(w, cmd)
	case CmdRemoveRoute:
		route, ok := w.Route(domain.RouteID(parseID(cmd.RouteID)))
		if !ok {
			return Write{}, "No such route."
		}
		return Write{Kind: domain.ActionRouteRemoved, Route: route}, ""
	case CmdTravel:
		leg, elapsed, reason := r.planTravel(w, cmd)
		if reason == "" {
			r.pass(&leg, r.st.gameTime().Add(elapsed))
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
	n := domain.WorldNode{ID: domain.NodeID(uuid.New()), Name: name, At: at, Secret: cmd.Secret}
	if cmd.MapID == "" {
		return Write{Kind: domain.ActionNodeAdded, Node: n}, ""
	}
	// The local Map that lies there: one of this Campaign, at one place only.
	local := domain.MapID(parseID(cmd.MapID))
	board, err := r.store.LoadMap(context.Background(), r.st.session.CampaignID, local)
	switch {
	case err != nil || board.Map.Kind != domain.MapLocal:
		return Write{}, "Choose a local map of this Campaign."
	case slices.ContainsFunc(r.st.world.Nodes, func(o domain.WorldNode) bool { return o.LocalMap != nil && *o.LocalMap == local }):
		return Write{}, "Another location already has that map."
	}
	n.LocalMap, n.LocalFound = &local, board.Map.Found
	return Write{Kind: domain.ActionNodeAdded, Node: n}, ""
}

// planFind is the party finding a Map in play, or losing it: the world map itself, or a local Map that
// lies at one of its locations.
func planFind(w *domain.World, cmd Command) (Write, string) {
	id := domain.MapID(parseID(cmd.MapID))
	has := w.Map.Found
	if id != w.Map.ID {
		i := slices.IndexFunc(w.Nodes, func(n domain.WorldNode) bool { return n.LocalMap != nil && *n.LocalMap == id })
		if i < 0 {
			return Write{}, "That map is not on this world map."
		}
		has = w.Nodes[i].LocalFound
	}
	switch {
	case has && cmd.On:
		return Write{}, "The party already has that map."
	case !has && !cmd.On:
		return Write{}, "The party does not have that map."
	case cmd.On:
		return Write{Kind: domain.ActionMapFound, Found: &domain.FoundMap{Map: id, On: true}}, ""
	}
	return Write{Kind: domain.ActionMapLost, Found: &domain.FoundMap{Map: id}}, ""
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
// planTravel works out a Travel Leg along a route from where the party stands, and how long it takes
// on the clock: on foot at a pace, or aboard a vehicle at the speed the vehicle makes as it stands.
func (r *runtime) planTravel(w *domain.World, cmd Command) (Write, int, string) {
	route, ok := w.Route(domain.RouteID(parseID(cmd.RouteID)))
	pace, known := travel.ParsePace(cmd.Pace)
	switch {
	case !ok:
		return Write{}, 0, "No such route."
	case w.Party == nil:
		return Write{}, 0, "Place the party on the world map first."
	case route.From != *w.Party && route.To != *w.Party:
		return Write{}, 0, "The party is at neither end of that route."
	case !known && cmd.VehicleID == "":
		return Write{}, 0, "Travel at a slow, normal or fast pace."
	}
	here, _ := w.Node(*w.Party)
	dest := route.To
	if dest == here.ID {
		dest = route.From
	}
	there, _ := w.Node(dest)
	p := travel.Plan(route.DistanceMi, pace)
	leg := domain.TravelLeg{
		From: here.Name, To: there.Name, Pace: pace.String(), DistanceMi: route.DistanceMi, Minutes: p.Minutes, Days: p.Days,
		FromSecret: here.Secret, ToSecret: there.Secret,
	}
	elapsed := clock.Journey(p.Minutes, p.Days)
	if cmd.VehicleID != "" {
		v, reason := r.vehicle(cmd.VehicleID)
		if reason != "" {
			return Write{}, 0, reason
		}
		leg.Vehicle = v.Name
		leg.Minutes, leg.Days = vehicles.Leg(route.DistanceMi, v.Speed(), v.Kind)
		elapsed = vehicles.Elapsed(leg.Minutes, leg.Days, v.Kind)
	}
	return Write{Kind: domain.ActionTravelLeg, Node: there, Leg: &leg}, elapsed, ""
}

// vehicle finds a vehicle of the Campaign that can move, as it stands now.
func (r *runtime) vehicle(raw string) (domain.Vehicle, string) {
	list, err := r.store.Vehicles(context.Background(), r.st.session.CampaignID)
	if err != nil {
		r.log.Error("live: vehicles", "error", err)
	}
	i := slices.IndexFunc(list, func(v domain.Vehicle) bool { return v.ID.String() == raw })
	switch {
	case i < 0:
		return domain.Vehicle{}, "No such vehicle."
	case list[i].Speed() == 0:
		return domain.Vehicle{}, list[i].Name + " cannot move as it stands."
	}
	return list[i], ""
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
		if w.Node.LocalFound && !w.Node.Secret {
			s.light(w, hex.Disk(w.Node.At, 1))
		}
	case domain.ActionMapFound, domain.ActionMapLost:
		found(s, w)
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

// light shows the party hexes of the world map for good.
func (s *state) light(w *Write, cells []hex.Coord) {
	for _, c := range cells {
		if s.worldCells[c] && !s.world.Reveals[c] {
			s.world.Reveals[c] = true
			w.WorldReveal = append(w.WorldReveal, c)
		}
	}
}

// arrive puts the party at a location: it sees the land around it, and the road it walked to get there.
func arrive(s *state, w *Write) {
	world := s.world
	if w.Leg != nil {
		from, _ := world.Node(*world.Party)
		s.light(w, hex.Line(from.At, w.Node.At))
		world.Legs = append(world.Legs, *w.Leg)
	}
	id := w.Node.ID
	world.Party = &id
	s.light(w, hex.Disk(w.Node.At, travel.SightHexes))
}

// found is the party finding or losing a Map. A local Map found lights its place on the world map,
// unless that place is secret, and what the party has seen stays seen when a Map is lost.
func found(s *state, w *Write) {
	world := s.world
	if w.Found.Map == world.Map.ID {
		world.Map.Found = w.Found.On
		return
	}
	for i, n := range world.Nodes {
		if n.LocalMap != nil && *n.LocalMap == w.Found.Map {
			world.Nodes[i].LocalFound = w.Found.On
			if w.Found.On && !n.Secret {
				s.light(w, hex.Disk(n.At, 1))
			}
		}
	}
}

// worldView shows the world map to one audience. Players and the Table get the locations the party has
// seen or can reach from where it stands, and only routes between two of those.
func (s *state) worldView(a Audience) *WorldView {
	w := s.world
	if w == nil {
		return nil
	}
	v := &WorldView{Map: mapView(w.Map, pictureVersion(w)), Found: w.Map.Found, Revealed: hexes(w.Reveals), Nodes: []NodeView{}, Routes: []RouteView{}, Legs: []LegView{}}
	known := knownNodes(w, a)
	for _, n := range w.Nodes {
		if known[n.ID] {
			v.Nodes = append(v.Nodes, nodeView(n, a))
		}
	}
	for _, x := range w.Routes {
		if known[x.From] && known[x.To] {
			v.Routes = append(v.Routes, routeView(x))
		}
	}
	if w.Party != nil && known[*w.Party] {
		v.PartyNodeID = uuid.UUID(*w.Party).String()
	}
	for _, l := range w.Legs {
		v.Legs = append(v.Legs, legView(l, a))
	}
	return v
}

// legView is a Travel Leg as one audience may read it: a secret place at either end is named to the DM only.
func legView(l domain.TravelLeg, a Audience) LegView {
	v := LegView{From: l.From, To: l.To, Pace: l.Pace, DistanceMi: l.DistanceMi, Minutes: l.Minutes, Days: l.Days, Vehicle: l.Vehicle}
	if a == AudienceDM {
		return v
	}
	if l.FromSecret {
		v.From = SecretPlace
	}
	if l.ToSecret {
		v.To = SecretPlace
	}
	return v
}

// foundPicture is added to the picture's version while the party has the world map: the picture a
// player is sent is then the whole of it, and its address must change with it.
const foundPicture = 1_000_000_000

func pictureVersion(w *domain.World) int {
	if w.Map.Found {
		return foundPicture + len(w.Reveals)
	}
	return len(w.Reveals)
}

// nodeView is a location as one audience may see it. A secret place only ever reaches the DM, and only
// the DM is told that a local Map the party has not found lies somewhere.
func nodeView(n domain.WorldNode, a Audience) NodeView {
	v := NodeView{ID: uuid.UUID(n.ID).String(), Name: n.Name, Q: n.At.Q, R: n.At.R, Secret: n.Secret, Found: n.LocalFound}
	if n.LocalMap != nil && (a == AudienceDM || n.LocalFound) {
		v.MapID = uuid.UUID(*n.LocalMap).String()
	}
	return v
}

// knownNodes are the locations an audience is shown. The DM sees them all. Anyone else sees those the
// party has seen or can reach from where it stands, or every one when the party has the world map, and
// never a secret one.
func knownNodes(w *domain.World, a Audience) map[domain.NodeID]bool {
	known := map[domain.NodeID]bool{}
	for _, n := range w.Nodes {
		known[n.ID] = a == AudienceDM || w.Map.Found || w.Reveals[n.At]
	}
	if w.Party != nil {
		for _, x := range w.Routes {
			if x.From == *w.Party || x.To == *w.Party {
				known[x.From], known[x.To] = true, true
			}
		}
	}
	if a != AudienceDM {
		for _, n := range w.Nodes {
			known[n.ID] = known[n.ID] && !n.Secret
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
		GridKind: m.Grid, GridStrength: m.GridStrength,
		ImageVersion: version, ImageURL: "/api/v1/campaigns/" + m.CampaignID.String() + "/maps/" + uuid.UUID(m.ID).String() + "/image?v=" + strconv.Itoa(version),
	}
}

// measure answers whoever asked with the length of a route over the world map. It reads the map and
// changes nothing, so anyone at the table may ask.
func (r *runtime) measure(req request) {
	w, hexes := r.st.world, req.cmd.Hexes
	switch {
	case w == nil:
		r.reject(req, "There is no world map to measure on.")
		return
	case len(hexes) < 2:
		r.reject(req, "Pick at least two points to measure between.")
		return
	case len(hexes) > MaxWaypoints:
		r.reject(req, "A route has at most "+strconv.Itoa(MaxWaypoints)+" points.")
		return
	}
	waypoints := make([]hex.Coord, 0, len(hexes))
	for _, h := range hexes {
		at := hex.Coord{Q: h.Q, R: h.R}
		if !r.st.worldCells[at] {
			r.reject(req, "That point is off the map.")
			return
		}
		waypoints = append(waypoints, at)
	}
	m := travel.Measure(waypoints, w.Map.ScaleMiles)
	if m.Miles > MaxMeasuredMiles {
		r.reject(req, "That route is too long to measure.")
		return
	}
	out := &MeasureView{Hexes: m.Hexes, Miles: m.Miles}
	for _, p := range []travel.Pace{travel.Slow, travel.Normal, travel.Fast} {
		minutes, days := travel.Time(m.Miles, p)
		out.Plans = append(out.Plans, PlanView{Pace: p.String(), Minutes: minutes, Days: days})
	}
	r.send(req.from, Update{Kind: UpdMeasured, Seq: r.st.session.Seq, Nonce: req.cmd.Nonce, Measure: out})
}
