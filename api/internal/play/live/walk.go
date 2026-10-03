package live

import (
	"strconv"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// MaxWalkFt caps one walk, so a path and the views along it stay small.
const MaxWalkFt = 300

// route finds the path a member may walk a token along. Whose tokens they may move goes by who they
// are; what the route may cross goes by their screen. Off the DM's own screen a route only crosses
// hexes the party knows and only avoids creatures the party sees, so neither the path nor its refusal
// reveals the fog.
func (r *runtime) route(m domain.Member, a Audience, cmd Command) (domain.Token, []hex.Coord, int, string) {
	id, err := uuid.Parse(cmd.TokenID)
	t, ok := r.st.tokens[domain.TokenID(id)]
	seen := r.st.vision()
	whole := a == AudienceDM
	if err != nil || !ok || (!whole && !r.st.shows(t, seen)) {
		return domain.Token{}, nil, 0, "No such token."
	}
	if !r.st.steers(m, t) {
		return domain.Token{}, nil, 0, "That token is not yours to move."
	}
	if t.Mount != nil {
		return domain.Token{}, nil, 0, t.Label + " is riding " + r.st.tokens[*t.Mount].Label + ": dismount first."
	}
	start, to := hex.Coord{Q: t.Q, R: t.R}, hex.Coord{Q: cmd.Q, R: cmd.R}
	if start == to {
		return domain.Token{}, nil, 0, "The token is already there."
	}
	if r.st.catalog.Immobile(r.st.actives(t.ID)) {
		return domain.Token{}, nil, 0, t.Label + " can't move."
	}
	reach := hex.Reachable(r.st.walkGrid(whole, t, seen), start, hex.MoveOptions{SpeedFt: MaxWalkFt, ClimbSpeed: false})
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
	t, path, cost, reason := r.route(req.from.Member, req.from.Audience, req.cmd)
	if reason != "" {
		r.reject(req, reason)
		return
	}
	k := r.st.knownTo(req.from.Audience)
	r.send(req.from, Update{
		Kind: UpdPath, Seq: r.st.session.Seq, Nonce: req.cmd.Nonce,
		Path: &PathView{
			TokenID: req.cmd.TokenID, Hexes: wireHexes(path), CostFt: cost,
			Threats: r.st.threats(t, path, k), Sight: r.st.sightAt(t, path[len(path)-1], k),
		},
	})
}

// knownTo is what an audience may be told of the board: all of it for the DM's own screen, and for any
// other the hexes the party has seen and the creatures it sees now. It goes by the screen, not by who
// is signed in, so a DM's Table Display learns no more than the table.
func (s *state) knownTo(a Audience) knowledge {
	if a == AudienceDM {
		return everything
	}
	seen := s.vision()
	return knowledge{
		hex:   func(c hex.Coord) bool { return s.board == nil || seen[c] || s.board.Reveals[c] },
		token: func(o domain.Token) bool { return s.shows(o, seen) },
	}
}

// threats are the opportunity attacks a walk would draw as far as its asker knows: the reactions
// engine's own walk over only the creatures and ground the asker may know, so one they cannot see
// neither shows nor changes what is said of the others.
func (s *state) threats(mover domain.Token, path []hex.Coord, k knowledge) []PathThreat {
	out := []PathThreat{}
	for _, o := range s.opportunities(mover, path, 0, k) {
		out = append(out, PathThreat{TokenID: uuid.UUID(o.reactor.ID).String(), Label: o.reactor.Label, Q: path[o.step].Q, R: path[o.step].R})
	}
	return out
}

// sightAt is how each standing creature of the other side that the asker knows of would see the mover at a hex.
func (s *state) sightAt(mover domain.Token, at hex.Coord, k knowledge) []PathSight {
	out := []PathSight{}
	for _, o := range s.ordered() {
		if o.Stats == nil || !standing(o) || (o.Kind == domain.TokenParty) == (mover.Kind == domain.TokenParty) || !k.token(o) {
			continue
		}
		sight := hex.LineOfSight(s.coverGrid(k, mover.ID, o.ID), hex.Coord{Q: o.Q, R: o.R}, at)
		out = append(out, PathSight{TokenID: uuid.UUID(o.ID).String(), Label: o.Label, Visible: sight.Visible, Cover: coverName(sight.Cover)})
	}
	return out
}

// coverName is a level of cover as the wire names it.
func coverName(c hex.Cover) string {
	return [...]string{hex.NoCover: "none", hex.HalfCover: "half", hex.ThreeQuartersCover: "three_quarters", hex.TotalCover: "total"}[c]
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
