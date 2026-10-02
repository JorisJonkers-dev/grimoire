package live

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/actions"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/movement"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/objects"
)

// elevation is a hex's height in feet, 0 without a map.
func (s *state) elevation(c hex.Coord) int {
	if s.board == nil {
		return 0
	}
	return s.board.Elevation[c]
}

// strength is a creature's Strength score, 10 when its statblock gives none.
func strength(t domain.Token) int {
	if t.Stats == nil || t.Stats.Strength == 0 {
		return 10
	}
	return t.Stats.Strength
}

// heights records where every token stands, to find falls after a change moves them.
func (s *state) heights() map[domain.TokenID]int {
	out := map[domain.TokenID]int{}
	for id, t := range s.tokens {
		out[id] = s.elevation(hex.Coord{Q: t.Q, R: t.R})
	}
	return out
}

// falls hands the DM the damage of every creature a change dropped 10 feet or more, and knocks it Prone.
func (s *state) falls(w *Write, before map[domain.TokenID]int) bool {
	fell := false
	ids := slices.SortedFunc(maps.Keys(before), func(a, b domain.TokenID) int { return strings.Compare(s.tokens[a].Label, s.tokens[b].Label) })
	for _, id := range ids {
		t, ok := s.tokens[id]
		if !ok || t.Stats == nil {
			continue
		}
		drop := before[id] - s.elevation(hex.Coord{Q: t.Q, R: t.R})
		if dice, prone := movement.Fall(drop); prone {
			s.fx.Manual = append(s.fx.Manual, domain.ManualPrompt{ID: uuid.New(), Text: fmt.Sprintf("%s falls %d feet: %s bludgeoning damage.", t.Label, drop, dice)})
			w.grounded = append(w.grounded, grounding{token: id, effect: "prone"})
			fell = true
		}
	}
	return fell
}

// planJump leaps a creature to a free hex: as far as its Strength score in feet and as high as 3 plus
// its Strength modifier, half each without a 10-foot run this turn. In a fight it spends the distance
// from its movement. Jumping down far enough is a fall.
func (r *runtime) planJump(m domain.Member, cmd Command) (Write, string) {
	t, ok := r.st.tokenByID(cmd.TokenID)
	to := hex.Coord{Q: cmd.Q, R: cmd.R}
	switch {
	case !ok || t.Stats == nil:
		return Write{}, "No such creature."
	case !m.DM && (t.Controller == nil || *t.Controller != m.ID):
		return Write{}, "That token is not yours to play."
	case r.st.catalog.Incapacitated(r.st.actives(t.ID)) || r.st.catalog.Immobile(r.st.actives(t.ID)):
		return Write{}, t.Label + " can't jump right now."
	case !r.st.onBoard(to) || r.st.occupied(to):
		return Write{}, "Land on a free hex on the map."
	}
	from := hex.Coord{Q: t.Q, R: t.R}
	dist, rise := hex.Distance(from, to)*hex.FeetPerHex, r.st.elevation(to)-r.st.elevation(from)
	w := Write{Kind: domain.ActionJumped, CostFt: dist, forced: true}
	running := true
	if c := r.st.combat; c != nil && c.Status == domain.CombatActive {
		x, acting := r.st.combatantOf(t.ID)
		if !acting {
			return Write{}, "It is not " + t.Label + "'s turn."
		}
		running = movement.Running(r.st.speedOf(x) - x.Economy.MovementFt)
		if dist > x.Economy.MovementFt {
			return Write{}, t.Label + " has too little movement left for that jump."
		}
		w.Combatant = x.ID
	}
	long, high := movement.LongJumpFt(strength(t), running), movement.HighJumpFt(rules.Modifier(strength(t)), running)
	switch {
	case dist > long:
		return Write{}, fmt.Sprintf("%s leaps at most %d feet.", t.Label, long)
	case rise > high:
		return Write{}, fmt.Sprintf("%s jumps at most %d feet high.", t.Label, high)
	}
	t.Q, t.R = to.Q, to.R
	w.Token = t
	return w, ""
}

// applyJump puts the jumper where it landed and spends the movement.
func applyJump(s *state, w *Write) {
	s.tokens[w.Token.ID] = w.Token
	if s.combat == nil {
		return
	}
	if i := slices.IndexFunc(s.combat.Combatants, func(x domain.Combatant) bool { return x.ID == w.Combatant }); i >= 0 {
		s.combat.Combatants[i].Economy, _ = s.combat.Combatants[i].Economy.Move(w.CostFt)
		w.Combat = s.combat
	}
}

// planThrow has a creature throw the creature it is grappling, or a barrel or chest next to it, to a
// free hex within 5 feet per point of Strength modifier. A thrown creature is let go, takes 1d6
// bludgeoning damage and lands Prone; a thrown barrel bursts and sets off its trigger. In a fight it
// takes the action.
func (r *runtime) planThrow(m domain.Member, cmd Command) (Write, string) {
	t, x, reason := r.actor(m, cmd.TokenID, false)
	if reason != "" {
		return Write{}, reason
	}
	to := hex.Coord{Q: cmd.Q, R: cmd.R}
	reach := movement.ThrowRangeFt(rules.Modifier(strength(t)))
	switch {
	case !r.st.onBoard(to) || r.st.occupied(to):
		return Write{}, "Throw it onto a free hex on the map."
	case hex.Distance(hex.Coord{Q: t.Q, R: t.R}, to)*hex.FeetPerHex > reach:
		return Write{}, fmt.Sprintf("%s throws at most %d feet.", t.Label, reach)
	}
	w := Write{Kind: domain.ActionThrown, Token: t, taken: &takenAction{action: actions.Utilize, readied: nil}, forced: true}
	if x != nil {
		w.Combatant = x.ID
	}
	if cmd.ObjectID != "" {
		return r.throwObject(m, cmd, t, to, w)
	}
	held, holding := r.st.dragged(t.ID)
	if !holding || uuid.UUID(held.ID).String() != cmd.TargetID {
		return Write{}, "Throw only a creature " + t.Label + " is grappling."
	}
	for _, e := range r.st.fx.Active {
		if e.Target == held.ID && e.Source != nil && *e.Source == t.ID && e.Slug == "grappled" {
			w.ended = append(w.ended, e.ID)
		}
	}
	held.Q, held.R = to.Q, to.R
	w.Pushed = &held
	w.manuals = []domain.ManualPrompt{{ID: uuid.New(), Text: fmt.Sprintf("%s is thrown and takes %s bludgeoning damage.", held.Label, movement.ThrownDice)}}
	w.grounded = []grounding{{token: held.ID, effect: "prone"}}
	return w, ""
}

// throwObject throws a barrel or chest next to the thrower; a barrel bursts where it lands.
func (r *runtime) throwObject(m domain.Member, cmd Command, t domain.Token, to hex.Coord, w Write) (Write, string) {
	o, reason := r.reachObject(m, cmd, t, false)
	switch {
	case reason != "":
		return Write{}, reason
	case (o.Kind != string(objects.Barrel) && o.Kind != string(objects.Chest)) || o.Broken:
		return Write{}, "Only a whole barrel or chest can be thrown."
	}
	o.At = to
	if o.Kind == string(objects.Barrel) {
		o.HP, o.Broken = 0, true
		if o.Effect != "" {
			w.trigger = &trigger{effect: o.Effect, at: to, radiusFt: o.RadiusFt, user: nil}
		}
	}
	thrown := objectWrite(domain.ActionThrown, o)
	w.changedObjects, w.Objects = thrown.changedObjects, thrown.Objects
	w.manuals = []domain.ManualPrompt{{ID: uuid.New(), Text: fmt.Sprintf("%s throws the %s.", t.Label, strings.ToLower(o.Name))}}
	return w, ""
}

// applyThrow moves the thrown creature or object and spends the thrower's action.
func applyThrow(s *state, w *Write) {
	if w.Pushed != nil {
		s.tokens[w.Pushed.ID] = *w.Pushed
	}
	if w.changedObjects != nil {
		for id, o := range w.changedObjects {
			s.board.Objects[id] = o
		}
	}
	applyAction(s, w)
}
