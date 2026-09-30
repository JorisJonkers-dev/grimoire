package hex

// EyeHeightFt is how far above its hex a creature sees from and is seen at.
const EyeHeightFt = 5

// Sight is what one creature sees of another.
type Sight struct {
	Visible bool
	Cover   Cover
}

// LineOfSight traces from one hex to another. Terrain that blocks sight, or rises above the line
// between the two eyes, hides the target completely; the best cover of the hexes in between, and
// any creature standing in between (half cover), protects it otherwise.
func LineOfSight(g Grid, from, to Coord) Sight {
	line := Line(from, to)
	eyeFrom := float64(g.Cells[from].ElevationFt + EyeHeightFt)
	eyeTo := float64(g.Cells[to].ElevationFt + EyeHeightFt)
	cover := NoCover
	for i := 1; i < len(line)-1; i++ {
		c := line[i]
		cell := g.Cells[c]
		height := eyeFrom + (eyeTo-eyeFrom)*float64(i)/float64(len(line)-1)
		if cell.BlocksSight || float64(cell.ElevationFt) > height {
			return Sight{Visible: false, Cover: TotalCover}
		}
		cover = max(cover, cell.Cover)
		if g.Occupants[c] != 0 {
			cover = max(cover, HalfCover)
		}
	}
	if cover == TotalCover {
		return Sight{Visible: false, Cover: TotalCover}
	}
	return Sight{Visible: true, Cover: cover}
}
