package httpapi

import (
	"context"
	"sort"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

func coverIn(c oas.CoverLevel) hex.Cover {
	return map[oas.CoverLevel]hex.Cover{
		oas.CoverLevelHalf: hex.HalfCover, oas.CoverLevelThreeQuarters: hex.ThreeQuartersCover, oas.CoverLevelTotal: hex.TotalCover,
	}[c]
}

func coverOut(c hex.Cover) oas.CoverLevel {
	return map[hex.Cover]oas.CoverLevel{
		hex.NoCover: oas.CoverLevelNone, hex.HalfCover: oas.CoverLevelHalf, hex.ThreeQuartersCover: oas.CoverLevelThreeQuarters, hex.TotalCover: oas.CoverLevelTotal,
	}[c]
}

func coordIn(c oas.HexCoord) hex.Coord { return hex.Coord{Q: int(c.Q), R: int(c.R)} }

//nolint:gosec // coordinates are bounded by the API
func coordOut(c hex.Coord) oas.HexCoord { return oas.HexCoord{Q: int32(c.Q), R: int32(c.R)} }

func gridIn(cells []oas.HexCell, occupants []oas.HexOccupant) hex.Grid {
	g := hex.Grid{Cells: make(map[hex.Coord]hex.Cell, len(cells)), Occupants: make(map[hex.Coord]hex.Occupant, len(occupants))}
	for _, c := range cells {
		g.Cells[hex.Coord{Q: int(c.Q), R: int(c.R)}] = hex.Cell{
			Difficult: c.Difficult.Or(false), Blocked: c.Blocked.Or(false), BlocksSight: c.BlocksSight.Or(false),
			ElevationFt: int(c.ElevationFt.Or(0)), Cover: coverIn(c.Cover.Or(oas.CoverLevelNone)),
		}
	}
	for _, o := range occupants {
		side := hex.Ally
		if o.Side == oas.HexOccupantSideEnemy {
			side = hex.Enemy
		}
		g.Occupants[hex.Coord{Q: int(o.Q), R: int(o.R)}] = side
	}
	return g
}

// PreviewReach answers where a mover can go without changing anything.
func (h *Handler) PreviewReach(_ context.Context, req *oas.ReachRequest) (oas.PreviewReachRes, error) {
	g := gridIn(req.Cells, req.Occupants)
	reach := hex.Reachable(g, coordIn(req.From), hex.MoveOptions{SpeedFt: int(req.SpeedFt), ClimbSpeed: req.ClimbSpeed.Or(false)})
	out := oas.ReachPreview{Hexes: make([]oas.ReachHex, 0, len(reach))}
	for c, s := range reach {
		out.Hexes = append(out.Hexes, oas.ReachHex{
			Q: int32(c.Q), R: int32(c.R), CostFt: int32(s.CostFt), CanEnd: s.CanEnd, From: coordOut(s.From), //nolint:gosec // bounded
		})
	}
	sort.Slice(out.Hexes, func(i, j int) bool {
		a, b := out.Hexes[i], out.Hexes[j]
		return a.CostFt < b.CostFt || (a.CostFt == b.CostFt && (a.Q < b.Q || (a.Q == b.Q && a.R < b.R)))
	})
	if to, set := req.To.Get(); set {
		if path, ok := reach.Path(coordIn(to)); ok {
			for _, c := range path {
				out.Path = append(out.Path, coordOut(c))
			}
			out.PathCostFt = oas.NewOptInt32(int32(reach[coordIn(to)].CostFt)) //nolint:gosec // bounded
		}
	}
	return &oas.ReachPreviewHeaders{Response: out}, nil
}

// PreviewSight answers what one hex sees of another without changing anything.
func (h *Handler) PreviewSight(_ context.Context, req *oas.SightRequest) (oas.PreviewSightRes, error) {
	s := hex.LineOfSight(gridIn(req.Cells, req.Occupants), coordIn(req.From), coordIn(req.To))
	return &oas.SightPreviewHeaders{Response: oas.SightPreview{
		Visible: s.Visible, Cover: coverOut(s.Cover), AcBonus: int32(s.Cover.ACBonus()), //nolint:gosec // 0..5
	}}, nil
}
