package live

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// abilities a save can be made with.
func validAbility(a string) bool {
	switch a {
	case "strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma":
		return true
	}
	return false
}

func (r *runtime) planEffect(cmd Command) (Write, string) {
	switch cmd.Kind {
	case CmdEndEffect:
		i := slices.IndexFunc(r.st.fx.Active, func(e domain.Effect) bool { return uuid.UUID(e.ID).String() == cmd.EffectID })
		if i < 0 {
			return Write{}, "No such effect."
		}
		e := r.st.fx.Active[i]
		return Write{Kind: domain.ActionEffectEnded, Token: r.st.tokens[e.Target], ended: []domain.EffectID{e.ID}}, ""
	case CmdResolveManual:
		i := slices.IndexFunc(r.st.fx.Manual, func(m domain.ManualPrompt) bool { return m.ID.String() == cmd.ManualID })
		if i < 0 {
			return Write{}, "No such prompt."
		}
		return Write{Kind: domain.ActionManualResolved, resolved: r.st.fx.Manual[i].ID}, ""
	}
	return r.planApply(cmd)
}

// planApply puts an Effect on a token. A new concentration effect ends the source's previous ones, and
// every part the engine cannot compute becomes a Manual prompt for the DM.
func (r *runtime) planApply(cmd Command) (Write, string) {
	target, slug, source, reason := r.checkEffect(cmd)
	if reason != "" {
		return Write{}, reason
	}
	def, known := effects.Lookup(slug)
	name := def.Name
	if !known {
		name = strings.TrimSpace(cmd.EffectName)
		if name == "" || len([]rune(name)) > 80 {
			name = slug
		}
	}
	e := domain.Effect{
		ID: domain.EffectID(uuid.New()), Target: target.ID, Source: source, Slug: slug, Name: name, Concentration: def.Concentration && source != nil,
		RoundsLeft: cmd.Rounds, SaveAbility: cmd.SaveAbility, SaveDC: cmd.SaveDC,
	}
	w := Write{Kind: domain.ActionEffectApplied, Token: target, effect: &e}
	for _, old := range r.st.fx.Active {
		if e.Concentration && old.Concentration && old.Source != nil && *old.Source == *source {
			w.ended = append(w.ended, old.ID)
		}
	}
	for _, text := range effects.Instructions(slug, name) {
		w.manuals = append(w.manuals, domain.ManualPrompt{ID: uuid.New(), Text: target.Label + ": " + text})
	}
	return w, ""
}

// checkEffect validates an Effect's target, name, duration, ending save and source.
func (r *runtime) checkEffect(cmd Command) (domain.Token, string, *domain.TokenID, string) {
	tid, _ := uuid.Parse(cmd.TargetID)
	target, ok := r.st.tokens[domain.TokenID(tid)]
	slug := strings.ToLower(strings.TrimSpace(cmd.Effect))
	switch {
	case !ok:
		return target, slug, nil, "Choose a token for the effect."
	case slug == "" || len(slug) > 80:
		return target, slug, nil, "Name the effect."
	case cmd.Rounds < 0 || cmd.Rounds > 100:
		return target, slug, nil, "Effects last 0 to 100 rounds."
	case (cmd.SaveAbility == "") != (cmd.SaveDC == 0) || (cmd.SaveAbility != "" && !validAbility(cmd.SaveAbility)) || cmd.SaveDC < 0 || cmd.SaveDC > 40:
		return target, slug, nil, "An ending save needs an ability and a DC from 1 to 40."
	}
	if cmd.SourceID == "" {
		return target, slug, nil, ""
	}
	sid, _ := uuid.Parse(cmd.SourceID)
	s, ok := r.st.tokens[domain.TokenID(sid)]
	if !ok {
		return target, slug, nil, "No such source."
	}
	return target, slug, &s.ID, ""
}

// actives lists the Effects on a token for the rules engine.
func (s *state) actives(id domain.TokenID) []effects.Active {
	var out []effects.Active
	for _, e := range s.fx.Active {
		if e.Target == id {
			a := effects.Active{Slug: e.Slug, Source: ""}
			if e.Source != nil {
				a.Source = uuid.UUID(*e.Source).String()
			}
			out = append(out, a)
		}
	}
	return out
}

// saves opens the saving throws a token makes at the end of its turn to end its Effects.
func (r *runtime) saves(dm domain.Member, t domain.Token) ([]domain.Roll, []domain.PendingSave) {
	var rolls []domain.Roll
	var pending []domain.PendingSave
	extra := effects.SaveDice(r.st.actives(t.ID))
	for _, e := range r.st.fx.Active {
		if e.Target != t.ID || e.SaveAbility == "" || slices.ContainsFunc(r.st.fx.Saves, func(p domain.PendingSave) bool { return p.Effect == e.ID }) {
			continue
		}
		bonus := 0
		if t.Stats != nil {
			bonus = t.Stats.Saves[e.SaveAbility]
		}
		ability := strings.ToUpper(e.SaveAbility[:1]) + e.SaveAbility[1:]
		roll := r.request(dm, t, fmt.Sprintf("%s save to end %s (DC %d)", ability, e.Name, e.SaveDC), strings.Join(append([]string{"1d20"}, extra...), "+"),
			domain.Modifier{Label: ability + " save", Value: bonus})
		rolls = append(rolls, roll)
		pending = append(pending, domain.PendingSave{RollID: roll.ID, Effect: e.ID, DC: e.SaveDC})
	}
	return rolls, pending
}

// saveRolled ends an Effect whose saving throw met its DC.
func (r *runtime) saveRolled(p domain.PendingSave) {
	roll, err := r.store.Roll(context.Background(), r.st.session.CampaignID, p.RollID)
	if err != nil || roll.Status != domain.StatusResolved {
		return
	}
	i := slices.IndexFunc(r.st.fx.Active, func(e domain.Effect) bool { return e.ID == p.Effect })
	w := Write{Kind: domain.ActionSaveFailed, Token: r.st.tokens[r.st.fx.Active[i].Target], saved: p.RollID}
	if roll.Total >= p.DC {
		w.Kind, w.ended = domain.ActionSavePassed, []domain.EffectID{p.Effect}
	}
	r.commit(request{}, w, roll.Roller, caller.Caller{Subject: roll.Roller.Subject, Origin: caller.OriginSystem, Client: ""})
}

// applyEffects changes the Session's Effects for one write.
func applyEffects(s *state, w *Write) {
	fx := &s.fx
	if w.effect != nil {
		fx.Active = append(fx.Active, *w.effect)
	}
	fx.Manual = append(fx.Manual, w.manuals...)
	fx.Saves = append(fx.Saves, w.newSaves...)
	fx.Manual = slices.DeleteFunc(fx.Manual, func(m domain.ManualPrompt) bool { return m.ID == w.resolved })
	fx.Saves = slices.DeleteFunc(fx.Saves, func(p domain.PendingSave) bool { return p.RollID == w.saved })
	s.endEffects(w.ended)
}

// endEffects removes Effects and the saves waiting on them.
func (s *state) endEffects(ids []domain.EffectID) {
	fx := &s.fx
	fx.Active = slices.DeleteFunc(fx.Active, func(e domain.Effect) bool { return slices.Contains(ids, e.ID) })
	fx.Saves = slices.DeleteFunc(fx.Saves, func(p domain.PendingSave) bool { return slices.Contains(ids, p.Effect) })
}

// tick counts Effects down as their holders start a turn; an Effect expires when it reaches zero.
func (s *state) tick(started map[domain.TokenID]bool) bool {
	var expired []domain.EffectID
	changed := false
	for i, e := range s.fx.Active {
		if e.RoundsLeft == 0 || !started[e.Holder()] {
			continue
		}
		changed = true
		if s.fx.Active[i].RoundsLeft--; s.fx.Active[i].RoundsLeft == 0 {
			expired = append(expired, e.ID)
		}
	}
	s.endEffects(expired)
	return changed
}

// acting lists the tokens whose turn it is now.
func (s *state) acting() map[domain.TokenID]bool {
	out := map[domain.TokenID]bool{}
	if s.combat == nil {
		return out
	}
	for _, x := range s.combat.Combatants {
		if s.combat.Acting(x) {
			out[x.TokenID] = true
		}
	}
	return out
}

// concentrate handles damage to a concentrating creature: at 0 hit points its concentration ends,
// otherwise the DM gets the Constitution save to resolve.
func (s *state) concentrate(h HPChange) bool {
	var held []domain.Effect
	for _, e := range s.fx.Active {
		if e.Concentration && e.Source != nil && *e.Source == h.Token {
			held = append(held, e)
		}
	}
	if len(held) == 0 || h.After >= h.Before {
		return false
	}
	if h.After == 0 {
		ids := make([]domain.EffectID, 0, len(held))
		for _, e := range held {
			ids = append(ids, e.ID)
		}
		s.endEffects(ids)
		return true
	}
	taken := h.Before - h.After
	s.fx.Manual = append(s.fx.Manual, domain.ManualPrompt{ID: uuid.New(), Text: fmt.Sprintf(
		"%s took %d damage while concentrating on %s: Constitution save DC %d to keep it.", s.tokens[h.Token].Label, taken, held[0].Name, max(10, taken/2))})
	return true
}

// forget drops a removed token's Effects and the source it gave to others.
func (s *state) forget(id domain.TokenID) {
	var gone []domain.EffectID
	for i, e := range s.fx.Active {
		if e.Target == id {
			gone = append(gone, e.ID)
		}
		if e.Source != nil && *e.Source == id {
			s.fx.Active[i].Source = nil
		}
	}
	s.endEffects(gone)
}

// effectViews shows a token's Effects.
func (s *state) effectViews(id domain.TokenID) []EffectView {
	var out []EffectView
	for _, e := range s.fx.Active {
		if e.Target != id {
			continue
		}
		v := EffectView{ID: uuid.UUID(e.ID).String(), Slug: e.Slug, Name: e.Name, Concentration: e.Concentration, RoundsLeft: e.RoundsLeft}
		if e.Source != nil {
			v.SourceID = uuid.UUID(*e.Source).String()
		}
		out = append(out, v)
	}
	return out
}
