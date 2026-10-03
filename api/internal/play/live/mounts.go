package live

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/actions"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/combat"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/mounted"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// keepSeat is the pending save a rider makes to stay on its mount.
const keepSeat = "keep_seat"

// rider is the creature riding a token, if any.
func (s *state) rider(mount domain.TokenID) (domain.Token, bool) {
	for _, t := range s.tokens {
		if t.Mount != nil && *t.Mount == mount {
			return t, true
		}
	}
	return domain.Token{}, false
}

// plays reports whether a member may play a token: the DM, its Controller, or the Player whose own
// creature rides it and controls it.
func (s *state) plays(m domain.Member, t domain.Token) bool {
	if m.DM || (t.Controller != nil && *t.Controller == m.ID) {
		return true
	}
	r, ridden := s.rider(t.ID)
	return ridden && r.Steers && r.Controller != nil && *r.Controller == m.ID
}

// reined says why a mount cannot do something: one its rider controls only Dashes, Disengages or Dodges.
func (s *state) reined(t domain.Token, a actions.Action) string {
	if r, ridden := s.rider(t.ID); !ridden || mounted.Allows(r.Steers, a) {
		return ""
	}
	return t.Label + " is ridden: it only Dashes, Disengages or Dodges."
}

// steed is the mount a creature rides and controls, if any.
func (s *state) steed(rider domain.TokenID) (domain.TokenID, bool) {
	t := s.tokens[rider]
	if t.Mount == nil || !t.Steers {
		return domain.TokenID{}, false
	}
	return *t.Mount, true
}

// saddles names each mount's rider among the tokens a screen is shown, and hides a mount it is not shown.
func saddles(tokens []TokenView) {
	at := make(map[string]int, len(tokens))
	for i, t := range tokens {
		at[t.ID] = i
	}
	for i, t := range tokens {
		j, shown := at[t.MountID]
		if !shown {
			tokens[i].MountID, tokens[i].MountControlled = "", false
			continue
		}
		tokens[j].RiderID = t.ID
	}
}

// barred says why a creature whose turn it is cannot take an action: a summon still waiting on its
// command does nothing but Dodge, and a mount its rider controls only Dashes, Disengages or Dodges.
func (s *state) barred(t domain.Token, a actions.Action) string {
	if reason := s.uncommanded(t); reason != "" && a != actions.Dodge {
		return reason
	}
	return s.reined(t, a)
}

// saddler finds the creature a member has get on or off a mount: one of theirs, and in a fight on its turn.
func (r *runtime) saddler(m domain.Member, tokenID string) (domain.Token, *domain.Combatant, string) {
	t, ok := r.st.tokenByID(tokenID)
	switch {
	case !ok || t.Stats == nil:
		return t, nil, "No such creature."
	case !m.DM && (t.Controller == nil || *t.Controller != m.ID):
		return t, nil, "That token is not yours to play."
	}
	// Outside a fight, and for a creature that is not in it, getting on and off is free.
	var fighting []domain.Combatant
	if r.st.combat != nil {
		fighting = r.st.combat.Combatants
	}
	for _, x := range fighting {
		if x.TokenID == t.ID && !r.st.combat.Acting(x) {
			return t, nil, "It is not " + t.Label + "'s turn."
		}
		if x.TokenID == t.ID {
			return t, &x, ""
		}
	}
	return t, nil, ""
}

// seatCost is the movement getting on or off a mount takes: half the creature's Speed in a fight.
func (s *state) seatCost(t domain.Token, x *domain.Combatant) (int, string) {
	if s.catalog.Immobile(s.actives(t.ID)) {
		return 0, t.Label + " can't move."
	}
	if x == nil {
		return 0, ""
	}
	cost := mounted.Cost(s.speedOf(*x))
	if cost > x.Economy.MovementFt {
		return 0, t.Label + " has " + strconv.Itoa(x.Economy.MovementFt) + " ft of movement left."
	}
	return cost, ""
}

// planMount puts a creature on a willing creature beside it: one of its own side, carrying nobody and
// riding nothing. The rider says whether it controls the mount or lets it act for itself.
func (r *runtime) planMount(m domain.Member, cmd Command) (Write, string) {
	t, x, reason := r.saddler(m, cmd.TokenID)
	if reason != "" {
		return Write{}, reason
	}
	mount, ok := r.st.tokenByID(cmd.TargetID)
	_, carrying := r.st.rider(t.ID)
	_, ridden := r.st.rider(mount.ID)
	switch {
	case !ok || mount.Stats == nil || mount.ID == t.ID:
		return Write{}, "Choose a creature to ride."
	case t.Mount != nil:
		return Write{}, t.Label + " is riding already."
	case carrying:
		return Write{}, t.Label + " is carrying a rider."
	case ridden:
		return Write{}, mount.Label + " already carries a rider."
	case mount.Mount != nil:
		return Write{}, mount.Label + " is riding, and carries no one."
	case (mount.Kind == domain.TokenParty) != (t.Kind == domain.TokenParty):
		return Write{}, mount.Label + " will not carry " + t.Label + "."
	case hex.Distance(hex.Coord{Q: t.Q, R: t.R}, hex.Coord{Q: mount.Q, R: mount.R}) > 1:
		return Write{}, mount.Label + " is out of reach."
	}
	cost, reason := r.st.seatCost(t, x)
	if reason != "" {
		return Write{}, reason
	}
	t.Q, t.R, t.Mount, t.Steers = mount.Q, mount.R, &mount.ID, cmd.Controlled
	return Write{Kind: domain.ActionMounted, Token: t, CostFt: cost, Note: mount.Label}, ""
}

// planDismount takes a rider off its mount onto a free hex next to it.
func (r *runtime) planDismount(m domain.Member, cmd Command) (Write, string) {
	t, x, reason := r.saddler(m, cmd.TokenID)
	if reason != "" {
		return Write{}, reason
	}
	if t.Mount == nil {
		return Write{}, t.Label + " is not riding."
	}
	mount, to := r.st.tokens[*t.Mount], hex.Coord{Q: cmd.Q, R: cmd.R}
	switch {
	case !r.st.onBoard(to):
		return Write{}, "That hex is off the map."
	case hex.Distance(hex.Coord{Q: mount.Q, R: mount.R}, to) != 1:
		return Write{}, "Dismount next to " + mount.Label + "."
	case r.st.occupied(to):
		return Write{}, "That hex is taken."
	}
	cost, reason := r.st.seatCost(t, x)
	if reason != "" {
		return Write{}, reason
	}
	t.Q, t.R, t.Mount, t.Steers = to.Q, to.R, nil, false
	return Write{Kind: domain.ActionDismounted, Token: t, CostFt: cost, Note: mount.Label}, ""
}

// applySeat puts a rider where getting on, getting off or falling leaves it, and spends what it cost.
func applySeat(s *state, w *Write) {
	s.tokens[w.Token.ID] = w.Token
	if w.Pending != nil {
		s.pending = append(s.pending, *w.Pending)
	}
	if s.combat == nil {
		return
	}
	for i, x := range s.combat.Combatants {
		if x.TokenID == w.Token.ID {
			s.combat.Combatants[i].Economy, _ = x.Economy.Move(w.CostFt)
		}
	}
	w.Combat = s.combat
}

// seating is a rider and its mount as they were before a change, and which of them lay Prone.
type seating struct {
	rider, mount           domain.Token
	riderProne, mountProne bool
}

// prone reports whether a creature lies Prone.
func (s *state) prone(id domain.TokenID) bool {
	return slices.ContainsFunc(s.fx.Active, func(e domain.Effect) bool { return e.Target == id && e.Slug == "prone" })
}

// seats is every rider with its mount, as they are now.
func (s *state) seats() map[domain.TokenID]seating {
	out := map[domain.TokenID]seating{}
	for id, t := range s.tokens {
		if t.Mount != nil {
			out[id] = seating{rider: t, mount: s.tokens[*t.Mount], riderProne: s.prone(id), mountProne: s.prone(*t.Mount)}
		}
	}
	return out
}

// carry takes a rider's own step with its mount: every frame of a walk shows them together.
func (s *state) carry(mount domain.Token) {
	if r, ridden := s.rider(mount.ID); ridden {
		r.Q, r.R = mount.Q, mount.R
		s.tokens[r.ID] = r
	}
}

// ride keeps riders and mounts together after a change, and a controlled mount on its rider's turn. A rider goes where its mount went; one whose
// mount is gone, or who was moved off it, rides no more. A rider whose mount was moved against its
// will, or who was knocked Prone, must save to stay on; one whose mount was knocked Prone is thrown.
func (s *state) ride(w *Write, before map[domain.TokenID]seating) {
	for _, t := range s.ordered() {
		id := t.ID
		was, rode := before[id]
		if !rode || t.Mount == nil {
			continue
		}
		m, kept := s.tokens[*t.Mount]
		moved := t.Q != was.rider.Q || t.R != was.rider.R
		apart := t.Q != m.Q || t.R != m.R
		switch {
		case !kept || (moved && apart):
			t.Mount, t.Steers = nil, false
		case apart:
			t.Q, t.R = m.Q, m.R
			s.shaken(w, was, t, w.shoves(m.ID))
		default:
			s.shaken(w, was, t, false)
		}
		if t.Q != was.rider.Q || t.R != was.rider.R || t.Mount == nil {
			s.tokens[id] = t
			w.Riders = append(w.Riders, t)
		}
	}
	s.rein()
}

// shoves reports whether a write moved a creature against its will.
func (w *Write) shoves(id domain.TokenID) bool {
	return (w.Pushed != nil && w.Pushed.ID == id) || (w.forced && w.Kind == domain.ActionTokenMoved && w.Token.ID == id)
}

// shaken marks a rider thrown when its mount was knocked Prone, and one whose mount was shoved from
// under it, or who was knocked Prone itself, to save.
func (s *state) shaken(w *Write, was seating, t domain.Token, shoved bool) {
	switch {
	case !was.mountProne && s.prone(*t.Mount):
		w.thrown = append(w.thrown, t.ID)
	case shoved || (!was.riderProne && s.prone(t.ID)):
		w.unseated = append(w.unseated, t.ID)
	}
}

// rein gives every controlled mount in a fight its rider's initiative, and a turn of its own on it:
// it acts on its rider's turn, at once when that turn is now. The changes that can bring this about,
// getting on and the last initiative roll, write the Combat themselves.
func (s *state) rein() {
	c := s.combat
	if c == nil || c.Status != domain.CombatActive {
		return
	}
	for i := range c.Combatants {
		t := s.tokens[c.Combatants[i].TokenID]
		if !t.Steers {
			continue
		}
		j := slices.IndexFunc(c.Combatants, func(x domain.Combatant) bool { return x.TokenID == *t.Mount })
		if j < 0 || *c.Combatants[j].Initiative == *c.Combatants[i].Initiative {
			continue
		}
		count := *c.Combatants[i].Initiative
		c.Combatants[j].Initiative, c.Combatants[j].Done = &count, false
		c.Combatants[j].Economy = combat.Fresh(s.speedOf(c.Combatants[j]))
	}
}

// unseat follows a change that shook riders: the thrown fall, and the others save to stay on.
func (r *runtime) unseat(w Write, actor domain.Member, c caller.Caller) {
	for _, id := range w.thrown {
		r.fall(id, actor, c)
	}
	for _, id := range w.unseated {
		t := r.st.tokens[id]
		roll, rolled := r.saveRoll(actor, t, "dexterity", fmt.Sprintf("save to stay on %s (DC %d)", r.st.tokens[*t.Mount].Label, mounted.FallDC))
		if !rolled {
			r.fall(id, actor, c)
			continue
		}
		next := Write{Kind: domain.ActionSeatChecked, Token: t, Rolls: []domain.Roll{roll}}
		next.Pending = &domain.PendingAction{RollID: roll.ID, Actor: id, Target: &id, Action: keepSeat, DC: mounted.FallDC}
		r.commit(request{}, next, actor, c)
	}
}

// seatRolled settles the save to stay on a mount: under the DC, the rider falls.
func (r *runtime) seatRolled(w Write, p domain.PendingAction, roll domain.Roll) {
	sys := caller.Caller{Subject: roll.Roller.Subject, Origin: caller.OriginSystem, Client: ""}
	r.commit(request{}, w, roll.Roller, sys)
	if mounted.Falls(roll.Total) {
		r.fall(p.Actor, roll.Roller, sys)
	}
}

// fall takes a rider off its mount onto a free hex beside it, the mount's own when there is none, and
// leaves it Prone.
func (r *runtime) fall(id domain.TokenID, actor domain.Member, c caller.Caller) {
	t := r.st.tokens[id]
	if t.Mount == nil {
		return
	}
	mount := r.st.tokens[*t.Mount]
	for _, n := range (hex.Coord{Q: mount.Q, R: mount.R}).Neighbors() {
		if r.st.onBoard(n) && !r.st.occupied(n) {
			t.Q, t.R = n.Q, n.R
			break
		}
	}
	t.Mount, t.Steers = nil, false
	sys := caller.Caller{Subject: c.Subject, Origin: caller.OriginSystem, Client: ""}
	r.commit(request{}, Write{Kind: domain.ActionUnseated, Token: t, Note: mount.Label}, actor, sys)
	if r.st.prone(id) {
		return
	}
	if next, reason := r.planApply(Command{Kind: CmdApplyEffect, TargetID: uuid.UUID(id).String(), Effect: "prone"}); reason == "" {
		r.commit(request{}, next, actor, sys)
	}
}
