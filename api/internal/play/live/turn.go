package live

import (
	"slices"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
)

// announce adds to a view Update what the change began: the initiative order once every roll is in, and
// the turns that started. It reads the audience's own view, so a hidden creature is never named.
func announce(u *Update, prev, next *state) {
	c := next.combat
	if c == nil || c.Status != domain.CombatActive || u.View == nil || u.View.Combat == nil {
		return
	}
	shown := u.View.Combat.Combatants
	if prev.combat == nil || prev.combat.Status != domain.CombatActive {
		u.Initiative = reveal(shown)
	}
	was := prev.acting()
	if prev.combat != nil && prev.combat.Round != c.Round {
		was = map[domain.TokenID]bool{}
	}
	u.Turn = turnStart(c.Round, shown, was)
}

func reveal(shown []CombatantView) *InitiativeReveal {
	out := InitiativeReveal{Order: make([]InitiativeRoll, 0, len(shown))}
	for _, x := range shown {
		if x.Initiative != nil {
			out.Order = append(out.Order, InitiativeRoll{TokenID: x.TokenID, Label: x.Label, Kind: x.Kind, Initiative: *x.Initiative})
		}
	}
	return &out
}

// turnStart names the shown Combatants that act now and did not before, or nil when no turn started.
func turnStart(round int, shown []CombatantView, was map[domain.TokenID]bool) *TurnStart {
	started := TurnStart{Round: round, TokenIDs: []string{}}
	for _, x := range shown {
		id, _ := uuid.Parse(x.TokenID)
		if x.Acting && !was[domain.TokenID(id)] {
			started.TokenIDs = append(started.TokenIDs, x.TokenID)
		}
	}
	if len(started.TokenIDs) == 0 {
		return nil
	}
	slices.Sort(started.TokenIDs)
	return &started
}
