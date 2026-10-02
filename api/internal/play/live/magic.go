package live

import (
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/combat"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// planTeleport moves a creature by magic to a free hex within the spell's range. It does not walk, so
// it provokes no opportunity attacks; in a fight it takes the Bonus Action of the creature's turn.
func (r *runtime) planTeleport(m domain.Member, cmd Command) (Write, string) {
	t, ok := r.st.tokenByID(cmd.TokenID)
	rangeFt, known := r.st.catalog.TeleportOf(strings.ToLower(strings.TrimSpace(cmd.Effect)))
	to := hex.Coord{Q: cmd.Q, R: cmd.R}
	switch {
	case !ok || t.Stats == nil:
		return Write{}, "No such creature."
	case !m.DM && (t.Controller == nil || *t.Controller != m.ID):
		return Write{}, "That token is not yours to play."
	case !known:
		return Write{}, "That is not a teleport the rules know."
	case r.st.catalog.Incapacitated(r.st.actives(t.ID)):
		return Write{}, t.Label + " can't act while Incapacitated."
	case !r.st.onBoard(to) || r.st.occupied(to):
		return Write{}, "Teleport to a free hex on the map."
	case hex.Distance(hex.Coord{Q: t.Q, R: t.R}, to)*hex.FeetPerHex > rangeFt:
		return Write{}, "That hex is out of range."
	}
	moved := t
	moved.Q, moved.R = to.Q, to.R
	w := Write{Kind: domain.ActionTeleported, Token: moved}
	if c := r.st.combat; c != nil && c.Status == domain.CombatActive {
		x, acting := r.st.combatantOf(t.ID)
		switch {
		case !acting:
			return Write{}, "It is not " + t.Label + "'s turn."
		case !x.Economy.BonusAction:
			return Write{}, t.Label + " has already used their Bonus Action."
		}
		w.Combatant = x.ID
	}
	return w, ""
}

// applyTeleport puts the creature where it went and spends the Bonus Action of its turn.
func applyTeleport(s *state, w *Write) {
	s.tokens[w.Token.ID] = w.Token
	if s.combat == nil {
		return
	}
	i := slices.IndexFunc(s.combat.Combatants, func(x domain.Combatant) bool { return x.ID == w.Combatant })
	if i < 0 {
		return
	}
	s.combat.Combatants[i].Economy, _ = s.combat.Combatants[i].Economy.Spend(combat.BonusAction)
	w.Combat = s.combat
}

// offerCounter asks the first creature on the other side able to counter a spell, and close enough to
// the caster, whether it does (Counterspell). The spell waits on the answer.
func (r *runtime) offerCounter(caster domain.Token, actor domain.Member, c caller.Caller) {
	f := r.st.combat
	if f == nil || f.Prompt != nil {
		return
	}
	ids := make([]domain.TokenID, 0, len(r.st.tokens))
	for id := range r.st.tokens {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b domain.TokenID) int { return strings.Compare(r.st.tokens[a].Label, r.st.tokens[b].Label) })
	for _, id := range ids {
		t := r.st.tokens[id]
		rangeFt, name, ok := r.st.catalog.CounterOf(r.st.actives(t.ID))
		x, fighting := r.st.fighter(t.ID)
		near := hex.Distance(hex.Coord{Q: t.Q, R: t.R}, hex.Coord{Q: caster.Q, R: caster.R})*hex.FeetPerHex <= rangeFt
		if !ok || !fighting || !near || !x.Economy.Reaction || !standing(t) || (t.Kind == domain.TokenParty) == (caster.Kind == domain.TokenParty) ||
			r.st.catalog.Incapacitated(r.st.actives(t.ID)) || !r.st.offers(t, domain.PromptEffect) {
			continue
		}
		spell, _ := r.st.catalog.Lookup(r.st.cast.Spell)
		pr := r.prompt(domain.PromptEffect, t, caster, 0, name+": counter "+caster.Label+"'s "+spell.Name+".")
		r.commit(request{}, Write{Kind: domain.ActionReactionOffered, Token: t, prompt: pr}, actor, c)
		return
	}
}

// countering reports whether a reaction prompt offers to counter the area spell being cast.
func (s *state) countering(p *domain.ReactionPrompt) bool {
	if p == nil || p.Kind != domain.PromptEffect || s.cast == nil || s.cast.Caster != p.Trigger {
		return false
	}
	_, _, ok := s.catalog.CounterOf(s.actives(p.Reactor))
	return ok
}

// awaitingCounter reports whether the area spell being cast waits on a Counterspell answer.
func (s *state) awaitingCounter() bool {
	return s.combat != nil && s.countering(s.combat.Prompt)
}

// forced moves a creature that failed its save the spell's distance straight away from, or toward, the
// caster, stopping short of the map's edge and of other creatures.
func (s *state) forced(from domain.Token, t domain.Token, push effects.ForcedMove) *domain.Token {
	origin, moved := hex.Coord{Q: from.Q, R: from.R}, t
	for range push.Ft / hex.FeetPerHex {
		next := hex.Push(origin, hex.Coord{Q: moved.Q, R: moved.R}, push.Toward)
		if next == origin || !s.onBoard(next) || s.occupied(next) {
			break
		}
		moved.Q, moved.R = next.Q, next.R
	}
	if moved.Q == t.Q && moved.R == t.R {
		return nil
	}
	return &moved
}

// sustained puts a concentration area spell on its caster, ending what the caster concentrated on
// before; an emanation then moves with the caster.
func (s *state) sustained(caster domain.Token, slug string) *Write {
	def, _ := s.catalog.Lookup(slug)
	if !def.Concentration {
		return nil
	}
	e := domain.Effect{ID: domain.EffectID(uuid.New()), Target: caster.ID, Source: &caster.ID, Slug: slug, Name: def.Name, Concentration: true, Level: 1}
	w := Write{Kind: domain.ActionEffectApplied, Token: caster, effect: &e}
	for _, old := range s.held(caster.ID) {
		w.ended = append(w.ended, old.ID)
	}
	return &w
}

// emanation is the hexes an emanation Effect covers around its bearer where the bearer stands now.
func (s *state) emanation(e domain.Effect) []Hex {
	spell, ok := s.catalog.AreaOf(e.Slug)
	t, here := s.tokens[e.Target]
	if !ok || !here || spell.Area.Shape != hex.EmanationArea {
		return nil
	}
	at := hex.Coord{Q: t.Q, R: t.R}
	return wireHexes(slices.DeleteFunc(hex.Area(hex.EmanationArea, at, at, spell.Area.SizeFt), func(c hex.Coord) bool { return !s.onBoard(c) }))
}
