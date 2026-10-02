package live

import (
	"maps"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/imaging"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/vision"
)

// state is a Session's live truth. The runtime changes a copy, commits it, then keeps it.
type state struct {
	session domain.Session
	tokens  map[domain.TokenID]domain.Token
	board   *domain.MapState
	cells   map[hex.Coord]bool
	combat  *domain.Combat
	// surfaces is terrain on hexes this Session; cast is the area spell waiting on its rolls.
	surfaces map[hex.Coord]domain.Surface
	cast     *domain.AreaCast
	// table is what the Table Display shows; tableMap is the map of its world scene.
	table    domain.TableDisplay
	tableMap *domain.Map
	// world is the world map the party travels; worldCells are its hexes.
	world      *domain.World
	worldCells map[hex.Coord]bool
	zones      []domain.Zone
	// checks are this Session's Encounter Checks.
	checks []prep.Check
	// inventory is every Container of the Campaign.
	inventory domain.Inventory
	// shop is the Shop open in the Session; day the Campaign's in-game day.
	shop *domain.OpenShop
	day  int
	// observed is the ranged damage each creature has seen each other creature deal.
	observed map[domain.TokenID]map[domain.TokenID]int
	now      func() time.Time
	fx       domain.Effects
	// catalog is every Effect the rules know.
	catalog effects.Catalog
}

// cloneEffects copies a Session's Effects so a change never touches the committed state.
func cloneEffects(fx domain.Effects) domain.Effects {
	return domain.Effects{Active: slices.Clone(fx.Active), Manual: slices.Clone(fx.Manual), Saves: slices.Clone(fx.Saves)}
}

func (s *state) clone() *state {
	next := &state{session: s.session, tokens: maps.Clone(s.tokens), cells: s.cells, worldCells: s.worldCells, observed: map[domain.TokenID]map[domain.TokenID]int{}, now: s.now, fx: cloneEffects(s.fx), catalog: s.catalog}
	for k, v := range s.observed {
		next.observed[k] = maps.Clone(v)
	}
	next.surfaces, next.table, next.tableMap = maps.Clone(s.surfaces), s.table, s.tableMap
	if s.world != nil {
		next.world = s.world.Clone()
	}
	for _, z := range s.zones {
		next.zones = append(next.zones, cloneZone(z))
	}
	next.checks = slices.Clone(s.checks)
	next.inventory, next.day = cloneInventory(s.inventory), s.day
	if s.shop != nil {
		next.shop = s.shop.Clone()
	}
	if s.cast != nil {
		c := *s.cast
		next.cast = &c
	}
	if s.combat != nil {
		c := *s.combat
		c.Combatants = slices.Clone(c.Combatants)
		next.combat = &c
	}
	if s.board != nil {
		b := *s.board
		b.Walls, b.Reveals, b.Elevation = maps.Clone(b.Walls), maps.Clone(b.Reveals), maps.Clone(b.Elevation)
		b.Lights = append([]domain.MapLight(nil), b.Lights...)
		next.board = &b
	}
	return next
}

// setBoard switches the active Map and works out which hexes it has.
func (s *state) setBoard(b *domain.MapState) {
	s.board, s.cells = b, nil
	if b == nil {
		return
	}
	s.cells = map[hex.Coord]bool{}
	for _, c := range imaging.Cells(b.Map.Layout(), b.Map.Width, b.Map.Height) {
		s.cells[c] = true
	}
}

// onBoard reports whether a hex exists: on the Map's picture, or within the grid radius without one.
func (s *state) onBoard(c hex.Coord) bool {
	if s.board != nil {
		return s.cells[c]
	}
	return hex.Distance(hex.Coord{Q: 0, R: 0}, c) <= s.session.GridRadius
}

func ambient(a string) vision.Ambient {
	switch a {
	case domain.AmbientDim:
		return vision.Dim
	case domain.AmbientDark:
		return vision.Dark
	default:
		return vision.Bright
	}
}

// vision is the party's shared sight on the active Map.
func (s *state) vision() map[hex.Coord]bool {
	if s.board == nil {
		return nil
	}
	g := hex.Grid{Cells: map[hex.Coord]hex.Cell{}, Occupants: nil}
	for c := range s.cells {
		g.Cells[c] = s.cell(c)
	}
	scene := vision.Scene{Grid: g, Ambient: ambient(s.board.Map.Ambient)}
	for _, l := range s.board.Lights {
		scene.Lights = append(scene.Lights, vision.Light{At: l.At, BrightFt: l.BrightFt, DimFt: l.DimFt})
	}
	var viewers []vision.Viewer
	for _, t := range s.tokens {
		if t.Kind == domain.TokenParty && !t.Hidden {
			viewers = append(viewers, vision.Viewer{At: hex.Coord{Q: t.Q, R: t.R}, DarkvisionFt: t.DarkvisionFt})
		}
	}
	return scene.Visible(viewers)
}

// project builds what one audience may see. This is the security boundary: party and Table never get
// hidden tokens, tokens outside current sight, never-seen hexes, walls or lights.
func (s *state) project(a Audience) View {
	if a == AudienceTable && s.table.Blackout {
		return View{Tokens: []TokenView{}, Visible: []Hex{}, Remembered: []Hex{}, Table: s.tableView()}
	}
	v := View{Tokens: []TokenView{}, Visible: []Hex{}, Remembered: []Hex{}}
	seen := s.vision()
	if s.board != nil {
		s.projectBoard(&v, a, seen)
	}
	for _, t := range s.tokens {
		if a == AudienceDM || (!t.Hidden && (s.board == nil || seen[hex.Coord{Q: t.Q, R: t.R}])) {
			tv := tokenView(t, a)
			tv.Effects = s.effectViews(t.ID)
			v.Tokens = append(v.Tokens, tv)
		}
	}
	sort.Slice(v.Tokens, func(i, j int) bool { return v.Tokens[i].ID < v.Tokens[j].ID })
	s.projectCombat(&v, a, seen)
	s.terrainViews(&v, a, seen)
	s.projectPending(&v, a, seen)
	v.Table, v.World, v.Perception, v.Checks, v.Inventory = s.tableView(), s.worldView(a), s.perceptionViews(), s.checkViews(a), s.inventoryViews(a)
	v.Shop, v.GameDay = s.shopView(), s.day
	if a == AudienceDM {
		v.Zones = s.zoneViews()
	}
	return v
}

func (s *state) projectPending(v *View, a Audience, seen map[hex.Coord]bool) {
	for _, p := range s.fx.Saves {
		i := slices.IndexFunc(s.fx.Active, func(e domain.Effect) bool { return e.ID == p.Effect })
		e := s.fx.Active[i]
		if a == AudienceDM || s.shows(s.tokens[e.Target], seen) {
			v.Saves = append(v.Saves, SaveView{RollID: uuid.UUID(p.RollID).String(), TokenID: uuid.UUID(e.Target).String(), Effect: e.Name, DC: p.DC})
		}
	}
	if a == AudienceDM {
		for _, m := range s.fx.Manual {
			v.Manual = append(v.Manual, ManualView{ID: m.ID.String(), Text: m.Text})
		}
	} else {
		v.Resolving = len(s.fx.Manual) > 0
	}
}

func (s *state) projectBoard(v *View, a Audience, seen map[hex.Coord]bool) {
	{
		m := s.board.Map
		v.Fog = true
		mv := mapView(m, len(s.board.Reveals))
		v.Map = &mv
		v.Visible = hexes(seen)
		remembered := map[hex.Coord]bool{}
		for c := range s.board.Reveals {
			if !seen[c] {
				remembered[c] = true
			}
		}
		v.Remembered = hexes(remembered)
		if a == AudienceDM {
			v.Walls, v.Ambient, v.Lights = hexes(s.board.Walls), m.Ambient, []LightView{}
			for _, l := range s.board.Lights {
				v.Lights = append(v.Lights, LightView{ID: uuid.UUID(l.ID).String(), Q: l.At.Q, R: l.At.R, BrightFt: l.BrightFt, DimFt: l.DimFt})
			}
		}
	}
}

func sortHexes(hs []Hex) {
	sort.Slice(hs, func(i, j int) bool { return hs[i].Q < hs[j].Q || (hs[i].Q == hs[j].Q && hs[i].R < hs[j].R) })
}

// cell is what the rules see of a hex: walls, difficult Surfaces and height.
func (s *state) cell(c hex.Coord) hex.Cell {
	out := hex.Cell{Difficult: surface.Difficult(s.surfaces[c].Kind), Blocked: false, BlocksSight: false, ElevationFt: 0, Cover: hex.NoCover}
	if s.board != nil {
		out.Blocked, out.BlocksSight, out.ElevationFt = s.board.Walls[c], s.board.Walls[c], s.board.Elevation[c]
	}
	return out
}
