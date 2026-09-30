package live

import (
	"maps"
	"slices"
	"sort"
	"strconv"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/imaging"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/vision"
)

// state is a Session's live truth. The runtime changes a copy, commits it, then keeps it.
type state struct {
	session domain.Session
	tokens  map[domain.TokenID]domain.Token
	board   *domain.MapState
	cells   map[hex.Coord]bool
	combat  *domain.Combat
}

func (s *state) clone() *state {
	next := &state{session: s.session, tokens: maps.Clone(s.tokens), cells: s.cells}
	if s.combat != nil {
		c := *s.combat
		c.Combatants = slices.Clone(c.Combatants)
		next.combat = &c
	}
	if s.board != nil {
		b := *s.board
		b.Walls, b.Reveals = maps.Clone(b.Walls), maps.Clone(b.Reveals)
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
		wall := s.board.Walls[c]
		g.Cells[c] = hex.Cell{Blocked: wall, BlocksSight: wall}
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
	v := View{Tokens: []TokenView{}, Visible: []Hex{}, Remembered: []Hex{}}
	seen := s.vision()
	if s.board != nil {
		s.projectBoard(&v, a, seen)
	}
	for _, t := range s.tokens {
		if a == AudienceDM || (!t.Hidden && (s.board == nil || seen[hex.Coord{Q: t.Q, R: t.R}])) {
			v.Tokens = append(v.Tokens, tokenView(t, a))
		}
	}
	sort.Slice(v.Tokens, func(i, j int) bool { return v.Tokens[i].ID < v.Tokens[j].ID })
	s.projectCombat(&v, a, seen)
	return v
}

func (s *state) projectBoard(v *View, a Audience, seen map[hex.Coord]bool) {
	{
		m := s.board.Map
		v.Fog = true
		v.Map = &MapView{
			ID: uuid.UUID(m.ID).String(), Name: m.Name, Width: m.Width, Height: m.Height, HexSizePx: m.HexSize,
			OriginX: m.OriginX, OriginY: m.OriginY, ImageVersion: len(s.board.Reveals),
			ImageURL: "/api/v1/campaigns/" + m.CampaignID.String() + "/maps/" + uuid.UUID(m.ID).String() + "/image?v=" + strconv.Itoa(len(s.board.Reveals)),
		}
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
