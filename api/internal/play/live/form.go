package live

import (
	"context"
	"maps"
	"slices"
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// shape gives an Effect's landing the form it lays over the target: the creature it names, or the one
// whoever applies it chooses, with the form's hit points as Temporary Hit Points.
func (r *runtime) shape(w *Write, f effects.Form, cmd Command, target domain.Token, effect domain.EffectID) string {
	slug := f.Monster
	if slug == "" {
		slug = strings.ToLower(strings.TrimSpace(cmd.MonsterSlug))
	}
	switch {
	case target.Stats == nil:
		return "Only a creature can take a form."
	case slug == "":
		return "Choose a creature to become."
	}
	name, stats, err := r.stats.Monster(context.Background(), r.st.session.CampaignID, slug)
	if err != nil {
		return "No such creature: " + slug + "."
	}
	own := *target.Stats
	if target.Form != nil {
		own = target.Form.Own
	}
	temp := f.TempHP
	switch {
	case cmd.TempHP > 0:
		temp = min(cmd.TempHP, 999)
	case temp == 0:
		temp = stats.HPMax
	}
	shaped := overlay(own, stats)
	shaped.HP, shaped.TempHP = target.Stats.HP, temp
	formed := target
	formed.Stats, formed.Form = &shaped, &domain.Form{Effect: effect, Name: name, Own: own}
	w.Formed, w.Token = &formed, formed
	w.HP = &HPChange{Token: target.ID, Before: target.Stats.HP, After: target.Stats.HP, Temp: &temp}
	return ""
}

// overlay lays a creature's statistics over a token's own: its Armor Class, attacks, speed, skills and
// physical saves. The token keeps its hit points, its mind and its spellcasting.
func overlay(own, form domain.Stats) domain.Stats {
	out := own
	out.AC, out.Attacks, out.SpeedFt, out.Stealth, out.Perception = form.AC, slices.Clone(form.Attacks), form.SpeedFt, form.Stealth, form.Perception
	out.Initiative, out.UnarmedDC, out.AttacksPerAction, out.Shield = form.Initiative, form.UnarmedDC, max(1, form.AttacksPerAction), false
	out.Saves = maps.Clone(own.Saves)
	if out.Saves == nil {
		out.Saves = map[string]int{}
	}
	for _, ability := range []string{"strength", "dexterity", "constitution"} {
		out.Saves[ability] = form.Saves[ability]
	}
	return out
}

// formBroken ends a form whose hit points are gone, or whose bearer dropped to 0 hit points.
func (s *state) formBroken(w *Write) {
	if w.HP == nil {
		return
	}
	t, ok := s.tokens[w.HP.Token]
	if !ok || t.Form == nil || t.Stats == nil {
		return
	}
	if (w.HP.Temp != nil && *w.HP.Temp == 0) || t.Stats.HP == 0 {
		w.ended = append(w.ended, t.Form.Effect)
	}
}

// revert returns every token whose form's Effect has ended to its own statistics, keeping the hit
// points it has now; it reports whether any reverted.
func (s *state) revert(w *Write) bool {
	reverted := false
	for id, t := range s.tokens {
		if t.Form == nil || slices.ContainsFunc(s.fx.Active, func(e domain.Effect) bool { return e.ID == t.Form.Effect }) {
			continue
		}
		own := t.Form.Own
		own.HP, own.TempHP = t.Stats.HP, 0
		t.Stats, t.Form = &own, nil
		s.tokens[id] = t
		w.Reverted = append(w.Reverted, id)
		reverted = true
	}
	return reverted
}
