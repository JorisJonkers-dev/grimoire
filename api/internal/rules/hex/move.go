package hex

import "container/heap"

// MoveOptions describe the mover.
type MoveOptions struct {
	SpeedFt    int
	ClimbSpeed bool
}

// Step is how a hex is reached.
type Step struct {
	CostFt int
	From   Coord
	// CanEnd is false for hexes the mover may pass through but not stop in (an ally's space).
	CanEnd bool
}

// Reach is every hex a mover can get to, keyed by coordinate.
type Reach map[Coord]Step

// StepCost is the movement spent entering to from its neighbour from: 5 ft, doubled in difficult
// terrain, plus one extra foot per foot climbed without a climb speed. ok is false where the hex
// cannot be entered.
func StepCost(g Grid, from, to Coord, climb bool) (int, bool) {
	cell, onMap := g.Cells[to]
	if !onMap || cell.Blocked || g.Occupants[to] == Enemy {
		return 0, false
	}
	cost := FeetPerHex
	if cell.Difficult {
		cost *= 2
	}
	if rise := cell.ElevationFt - g.Cells[from].ElevationFt; rise > 0 && !climb {
		cost += rise
	}
	return cost, true
}

// Reachable finds every hex within the mover's speed, cheapest path first.
func Reachable(g Grid, start Coord, o MoveOptions) Reach {
	reach := Reach{start: {CostFt: 0, From: start, CanEnd: true}}
	queue := &frontier{{coord: start, cost: 0}}
	for queue.Len() > 0 {
		cur := heap.Pop(queue).(entry) //nolint:forcetypeassert // the heap only holds entries
		if cur.cost > reach[cur.coord].CostFt {
			continue
		}
		for _, next := range cur.coord.Neighbors() {
			step, ok := StepCost(g, cur.coord, next, o.ClimbSpeed)
			total := cur.cost + step
			if !ok || total > o.SpeedFt {
				continue
			}
			if known, seen := reach[next]; seen && known.CostFt <= total {
				continue
			}
			reach[next] = Step{CostFt: total, From: cur.coord, CanEnd: g.Occupants[next] != Ally}
			heap.Push(queue, entry{coord: next, cost: total})
		}
	}
	return reach
}

// Path returns the hexes from the start to a reachable hex where the mover may stop.
func (r Reach) Path(to Coord) ([]Coord, bool) {
	step, ok := r[to]
	if !ok || !step.CanEnd {
		return nil, false
	}
	path := []Coord{to}
	for cur := to; r[cur].From != cur; cur = r[cur].From {
		path = append([]Coord{r[cur].From}, path...)
	}
	return path, true
}

type entry struct {
	coord Coord
	cost  int
}

type frontier []entry

func (f frontier) Len() int           { return len(f) }
func (f frontier) Less(i, j int) bool { return f[i].cost < f[j].cost }
func (f frontier) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f *frontier) Push(x any)        { *f = append(*f, x.(entry)) } //nolint:forcetypeassert // the heap only holds entries
func (f *frontier) Pop() any {
	old := *f
	last := old[len(old)-1]
	*f = old[:len(old)-1]
	return last
}
