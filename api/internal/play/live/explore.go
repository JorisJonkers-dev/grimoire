package live

import (
	"strconv"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
)

// planExplore switches Exploration to turns, for the DM, with the party moving in order a speed of
// movement each; or ends them; or passes the turn to the next member, for its Controller or the DM.
func (r *runtime) planExplore(m domain.Member, cmd Command) (Write, string) {
	e := r.st.explore
	if cmd.Kind == CmdPassTurn {
		if e == nil {
			return Write{}, "Exploration is not in turns."
		}
		t := r.st.tokens[e.Order[e.Turn]]
		if !m.DM && (t.Controller == nil || *t.Controller != m.ID) {
			return Write{}, "It is " + t.Label + "'s turn to explore."
		}
		next := &domain.Exploration{Order: e.Order, Turn: (e.Turn + 1) % len(e.Order), MovedFt: 0}
		return Write{Kind: domain.ActionExplorationTurn, Token: r.st.tokens[next.Order[next.Turn]], explore: next}, ""
	}
	switch {
	case !m.DM:
		return Write{}, "Only the DM switches exploration turns."
	case !cmd.On && e == nil:
		return Write{}, "Exploration is not in turns."
	case !cmd.On:
		return Write{Kind: domain.ActionExplorationEnded, explore: nil}, ""
	case r.st.combat != nil:
		return Write{}, "A fight has its own turns."
	case e != nil:
		return Write{}, "Exploration is already in turns."
	}
	order := make([]domain.TokenID, 0)
	for _, t := range r.st.partyMembers() {
		order = append(order, t.ID)
	}
	if len(order) == 0 {
		return Write{}, "No party member is on the board to take turns."
	}
	return Write{Kind: domain.ActionExplorationStarted, Token: r.st.tokens[order[0]], explore: &domain.Exploration{Order: order, Turn: 0, MovedFt: 0}}, ""
}

// exploreLeft refuses a walk out of turn in exploration turns, or longer than the member has left.
func (s *state) exploreLeft(t domain.Token, cost int) string {
	e := s.explore
	if e == nil || t.Kind != domain.TokenParty {
		return ""
	}
	if e.Order[e.Turn] != t.ID {
		return "It is not " + t.Label + "'s turn to explore."
	}
	if left := speed(t) - e.MovedFt; cost > left {
		return t.Label + " has " + strconv.Itoa(left) + " ft of movement left this turn."
	}
	return ""
}

func speed(t domain.Token) int {
	if t.Stats == nil || t.Stats.SpeedFt == 0 {
		return 30
	}
	return t.Stats.SpeedFt
}

// ExplorationView is Exploration in turns: the order, whose turn it is and the movement it has left.
type ExplorationView struct {
	Order  []string `json:"order"`
	Turn   string   `json:"turn"`
	LeftFt int      `json:"leftFt"`
}

func (s *state) explorationView() *ExplorationView {
	e := s.explore
	if e == nil {
		return nil
	}
	v := &ExplorationView{Order: make([]string, 0, len(e.Order)), Turn: uuid.UUID(e.Order[e.Turn]).String(), LeftFt: max(0, speed(s.tokens[e.Order[e.Turn]])-e.MovedFt)}
	for _, id := range e.Order {
		v.Order = append(v.Order, uuid.UUID(id).String())
	}
	return v
}
