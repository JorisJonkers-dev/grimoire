package live

import (
	"slices"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/actions"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/mastery"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// toppled is the pending Topple save; a failure leaves the target Prone.
const toppled actions.Option = "topple"

// nicks reports whether an attack's weapon has the Nick mastery, which folds the off-hand attack into
// the Attack action.
func nicks(a domain.Attack) bool {
	return mastery.FreeOffHand(mastery.Property(a.Mastery))
}

// anyNick reports whether any of a token's Light weapons has Nick.
func (s *state) anyNick(t domain.Token) bool {
	return t.Stats != nil && slices.ContainsFunc(t.Stats.Attacks, func(a domain.Attack) bool { return a.Light && nicks(a) })
}

// cleavable reports whether Cleave's second attack may strike a creature: a different one within 5 feet
// of the creature the Cleave hit struck.
func (s *state) cleavable(x domain.Combatant, t domain.Token) bool {
	if x.CleaveFrom == nil || *x.CleaveFrom == t.ID {
		return false
	}
	first, ok := s.tokens[*x.CleaveFrom]
	return ok && hex.Distance(hex.Coord{Q: first.Q, R: first.R}, hex.Coord{Q: t.Q, R: t.R}) <= 1
}

// graze deals a Graze weapon's ability modifier to the target of an attack that missed.
func (r *runtime) graze(a, t domain.Token, p domain.PendingAttack, actor domain.Member, c caller.Caller) {
	with := a.Stats.Attacks[p.AttackNo]
	if dmg := mastery.OnMiss(mastery.Property(with.Mastery), with.DamageMod); dmg > 0 {
		t = r.st.tokens[t.ID]
		r.commit(request{}, r.hurt(t, dmg, Write{Token: a, attack: &p}), actor, c)
	}
}

// masteryAfterHit does what the weapon's mastery does once its attack hits: Sap, Slow and Vex mark the
// target until the attacker's next turn (Slow and Vex only when the hit dealt damage), Push drives it
// 10 feet away, Topple forces a Constitution save against falling Prone, and Cleave opens a second
// attack against a creature next to it.
func (r *runtime) masteryAfterHit(a, t domain.Token, p domain.PendingAttack, dealt int, actor domain.Member, c caller.Caller) {
	with := a.Stats.Attacks[p.AttackNo]
	prop := mastery.Property(with.Mastery)
	h := mastery.OnHit(prop, with.ToHit)
	t = r.st.tokens[t.ID]
	w := Write{Kind: domain.ActionMasteryUsed, Token: a}
	switch {
	case h.Effect != "":
		if (prop == mastery.Slow || prop == mastery.Vex) && dealt == 0 {
			return
		}
		def, _ := r.st.catalog.Lookup(h.Effect)
		e := domain.Effect{ID: domain.EffectID(uuid.New()), Target: t.ID, Source: &a.ID, Slug: h.Effect, Name: def.Name, RoundsLeft: 1, Level: 1}
		w.effect = &e
	case h.PushFt > 0:
		w.Pushed = r.st.pushed(a, t, h.PushFt/hex.FeetPerHex)
		if w.Pushed == nil {
			return
		}
	case h.SaveAbility != "":
		roll, ok := r.saveRoll(actor, t, h.SaveAbility, "save against "+a.Label+"'s Topple"+r.dcNote(h.SaveDC))
		if !ok {
			r.st.unarmedOutcome(&w, a, t, string(toppled))
			break
		}
		target := t.ID
		w.Rolls, w.Pending = []domain.Roll{roll}, &domain.PendingAction{RollID: roll.ID, Actor: a.ID, Target: &target, Action: string(toppled), DC: h.SaveDC}
	case h.Cleave:
		x, ok := r.st.fighter(a.ID)
		if !ok || x.Cleaved || p.Cleave || p.Ranged {
			return
		}
		w.cleave, w.Combatant = &t.ID, x.ID
	default:
		return
	}
	r.commit(request{}, w, actor, c)
}

// pushed is where a push of some hexes straight away from the attacker leaves the target: as far as
// open hexes allow, or nil when it cannot move at all.
func (s *state) pushed(a, t domain.Token, hexes int) *domain.Token {
	dq, dr := t.Q-a.Q, t.R-a.R
	if hex.Distance(hex.Coord{Q: a.Q, R: a.R}, hex.Coord{Q: t.Q, R: t.R}) != 1 {
		return nil
	}
	moved := t
	for range hexes {
		next := hex.Coord{Q: moved.Q + dq, R: moved.R + dr}
		if !s.onBoard(next) || s.occupied(next) {
			break
		}
		moved.Q, moved.R = next.Q, next.R
	}
	if moved.Q == t.Q && moved.R == t.R {
		return nil
	}
	return &moved
}

// applyMastery keeps a pending Topple save, moves a pushed creature and opens Cleave's second attack.
func applyMastery(s *state, w *Write) {
	if w.Pending != nil {
		s.pending = append(s.pending, *w.Pending)
	}
	if w.Pushed != nil {
		s.tokens[w.Pushed.ID] = *w.Pushed
	}
	if w.cleave == nil || s.combat == nil {
		return
	}
	i := slices.IndexFunc(s.combat.Combatants, func(x domain.Combatant) bool { return x.ID == w.Combatant })
	s.combat.Combatants[i].CleaveFrom = w.cleave
	w.Combat = s.combat
}
