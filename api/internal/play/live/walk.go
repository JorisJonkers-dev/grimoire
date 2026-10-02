package live

import (
	"strconv"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// MaxWalkFt caps one walk, so a path and the views along it stay small.
const MaxWalkFt = 300

// route finds the path a member may walk a token along. A Player's route only crosses hexes the party
// knows and only avoids creatures the party sees, so neither the path nor its refusal reveals the fog.
func (r *runtime) route(m domain.Member, cmd Command) (domain.Token, []hex.Coord, int, string) {
	id, err := uuid.Parse(cmd.TokenID)
	t, ok := r.st.tokens[domain.TokenID(id)]
	seen := r.st.vision()
	if err != nil || !ok || (!m.DM && !r.st.shows(t, seen)) {
		return domain.Token{}, nil, 0, "No such token."
	}
	if !m.DM && (t.Controller == nil || *t.Controller != m.ID) {
		return domain.Token{}, nil, 0, "That token is not yours to move."
	}
	start, to := hex.Coord{Q: t.Q, R: t.R}, hex.Coord{Q: cmd.Q, R: cmd.R}
	if start == to {
		return domain.Token{}, nil, 0, "The token is already there."
	}
	if r.st.catalog.Immobile(r.st.actives(t.ID)) {
		return domain.Token{}, nil, 0, t.Label + " can't move."
	}
	reach := hex.Reachable(r.st.walkGrid(m.DM, t, seen), start, hex.MoveOptions{SpeedFt: MaxWalkFt, ClimbSpeed: false})
	path, ok := reach.Path(to)
	if !ok {
		return domain.Token{}, nil, 0, "There is no way there."
	}
	cost := reach[to].CostFt * r.st.catalog.MoveMultiplier(r.st.actives(t.ID))
	if reason := r.st.moveLeft(t, cost); reason != "" {
		return domain.Token{}, nil, 0, reason
	}
	return t, path, cost, ""
}

func (r *runtime) previewWalk(req request) {
	_, path, cost, reason := r.route(req.from.Member, req.cmd)
	if reason != "" {
		r.reject(req, reason)
		return
	}
	r.send(req.from, Update{
		Kind: UpdPath, Seq: r.st.session.Seq, Nonce: req.cmd.Nonce,
		Path: &PathView{TokenID: req.cmd.TokenID, Hexes: wireHexes(path), CostFt: cost},
	})
}

// moveLeft refuses a walk a Combatant cannot make now: out of turn, or longer than its movement left.
func (s *state) moveLeft(t domain.Token, cost int) string {
	if s.combat == nil {
		return s.exploreLeft(t, cost)
	}
	for _, x := range s.combat.Combatants {
		switch {
		case x.TokenID != t.ID:
			continue
		case !s.combat.Acting(x):
			return "It is not " + t.Label + "'s turn."
		case cost > x.Economy.MovementFt:
			return t.Label + " has " + strconv.Itoa(x.Economy.MovementFt) + " ft of movement left."
		}
	}
	return ""
}

// shows reports whether the party sees a token right now.
func (s *state) shows(t domain.Token, seen map[hex.Coord]bool) bool {
	return !t.Hidden && (s.board == nil || seen[hex.Coord{Q: t.Q, R: t.R}]) && s.perceived(t).Seen
}

// walkGrid is the ground a mover walks on: every hex for the DM, the hexes the party knows for a Player.
// Tokens on the mover's side can be passed through; any other token blocks its hex.
func (s *state) walkGrid(dm bool, mover domain.Token, seen map[hex.Coord]bool) hex.Grid {
	g := hex.Grid{Cells: map[hex.Coord]hex.Cell{}, Occupants: map[hex.Coord]hex.Occupant{}}
	for _, c := range s.ground() {
		if dm || s.board == nil || seen[c] || s.board.Reveals[c] {
			g.Cells[c] = s.cell(c)
		}
	}
	for _, t := range s.tokens {
		if t.ID == mover.ID || (!dm && !s.shows(t, seen)) {
			continue
		}
		side := hex.Enemy
		if (t.Kind == domain.TokenParty) == (mover.Kind == domain.TokenParty) {
			side = hex.Ally
		}
		g.Occupants[hex.Coord{Q: t.Q, R: t.R}] = side
	}
	return g
}

// ground lists every hex of the board: the Map's hexes, or the open grid's.
func (s *state) ground() []hex.Coord {
	var out []hex.Coord
	if s.board != nil {
		for c := range s.cells {
			out = append(out, c)
		}
		return out
	}
	n := s.session.GridRadius
	for q := -n; q <= n; q++ {
		for r := max(-n, -q-n); r <= min(n, -q+n); r++ {
			out = append(out, hex.Coord{Q: q, R: r})
		}
	}
	return out
}
