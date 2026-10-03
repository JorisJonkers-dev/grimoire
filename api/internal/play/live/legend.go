package live

import (
	"slices"
	"strconv"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
)

// Legendary creatures' commands: a legendary action, a lair action, and a Legendary Resistance.
const (
	CmdLegendary = "legendary_action"
	CmdLair      = "lair_action"
	CmdResist    = "legendary_resistance"
)

// LegendChange is a token's Legend as a change leaves it, and its hit point maximum when a mythic phase
// gave it fresh hit points.
type LegendChange struct {
	Token  domain.TokenID
	Legend domain.Legend
	HPMax  *int
}

// planLegend takes a legendary action, a lair action or a Legendary Resistance for the DM.
func (r *runtime) planLegend(cmd Command) (Write, string) {
	id, _ := uuid.Parse(cmd.TokenID)
	t, ok := r.st.tokens[domain.TokenID(id)]
	switch {
	case !ok || t.Stats == nil || t.Stats.Legend == nil:
		return Write{}, "That creature has no legendary or lair actions."
	case r.st.combat == nil || r.st.combat.Status != domain.CombatActive:
		return Write{}, "Legendary and lair actions wait for a fight."
	}
	l := *t.Stats.Legend
	switch cmd.Kind {
	case CmdLegendary:
		return r.legendary(t, l, cmd.Legend)
	case CmdLair:
		return r.lair(t, l, cmd.Legend)
	}
	if l.ResistLeft < 1 {
		return Write{}, t.Label + " has no Legendary Resistance left."
	}
	l.ResistLeft--
	return r.legendWrite(domain.ActionLegendaryResistance, t, l, t.Label+" turns a failed save into a success with Legendary Resistance ("+strconv.Itoa(l.ResistLeft)+" left)."), ""
}

func (r *runtime) legendary(t domain.Token, l domain.Legend, name string) (Write, string) {
	i := slices.IndexFunc(l.Actions, func(a domain.LegendAction) bool { return a.Name == name })
	acting := slices.ContainsFunc(r.st.combat.Combatants, func(x domain.Combatant) bool { return x.TokenID == t.ID && r.st.combat.Acting(x) })
	switch {
	case i < 0:
		return Write{}, t.Label + " has no legendary action called " + name + "."
	case acting || !l.Ready:
		return Write{}, t.Label + " takes a legendary action only right after another creature's turn."
	case l.Left < l.Actions[i].Cost:
		return Write{}, t.Label + " has " + strconv.Itoa(l.Left) + " legendary actions left this round."
	}
	a := l.Actions[i]
	l.Left, l.Ready = l.Left-a.Cost, false
	return r.legendWrite(domain.ActionLegendaryAction, t, l, t.Label+" uses "+a.Name+": "+a.Text), ""
}

func (r *runtime) lair(t domain.Token, l domain.Legend, name string) (Write, string) {
	i := slices.IndexFunc(l.Lair, func(a domain.LegendAction) bool { return a.Name == name })
	c := r.st.combat
	switch {
	case i < 0:
		return Write{}, t.Label + "'s lair has no action called " + name + "."
	case c.Turn > 20:
		return Write{}, "The lair acts on initiative count 20."
	case l.LairRound == c.Round:
		return Write{}, "The lair has acted this round."
	}
	l.LairRound = c.Round
	a := l.Lair[i]
	return r.legendWrite(domain.ActionLairAction, t, l, t.Label+"'s lair: "+a.Name+": "+a.Text), ""
}

// legendWrite is a change to a legendary creature's Legend, with what it does for the DM to resolve.
func (r *runtime) legendWrite(kind string, t domain.Token, l domain.Legend, text string) Write {
	return Write{
		Kind: kind, Token: t, Legends: []LegendChange{{Token: t.ID, Legend: l, HPMax: nil}},
		manuals: []domain.ManualPrompt{{ID: uuid.New(), Text: text}},
	}
}

// rise starts a mythic creature's next phase when damage drops it to 0 hit points: fresh hit points, a
// new maximum, and what the phase brings for the DM to resolve.
func rise(w *Write, h *HPChange, t domain.Token) {
	l := t.Stats.Legend
	if h.After > 0 || l == nil || l.Phase >= len(l.Phases) {
		return
	}
	p := l.Phases[l.Phase]
	next := *l
	next.Phase++
	h.After = p.HP
	most := p.HP
	w.Legends = append(w.Legends, LegendChange{Token: t.ID, Legend: next, HPMax: &most})
	w.manuals = append(w.manuals, domain.ManualPrompt{ID: uuid.New(), Text: t.Label + " enters its next phase: " + p.Name + ". " + p.Text})
}

// legends keeps legendary creatures' rounds: another creature's ended turn opens a window for each of
// them, and its own turn gives a creature its legendary actions back.
func (s *state) legends(w *Write, started map[domain.TokenID]bool) {
	if s.combat == nil {
		return
	}
	for _, x := range s.combat.Combatants {
		t := s.tokens[x.TokenID]
		if t.Stats == nil || t.Stats.Legend == nil {
			continue
		}
		next := *t.Stats.Legend
		switch {
		case started[t.ID]:
			next.Left, next.Ready = next.Uses, false
		case w.Kind == domain.ActionTurnEnded:
			next.Ready = t.ID != w.Token.ID
		default:
			continue
		}
		c := LegendChange{Token: t.ID, Legend: next, HPMax: nil}
		s.setLegend(c)
		w.Legends = append(w.Legends, c)
	}
}

// setLegend puts a changed Legend on its token.
func (s *state) setLegend(c LegendChange) {
	t, ok := s.tokens[c.Token]
	if !ok || t.Stats == nil {
		return
	}
	stats := *t.Stats
	l := c.Legend
	stats.Legend = &l
	if c.HPMax != nil {
		stats.HPMax = *c.HPMax
	}
	t.Stats = &stats
	s.tokens[t.ID] = t
}

// legendView is a legendary creature as the DM sees it; nil for any other.
func (s *state) legendView(t domain.Token) *LegendView {
	if t.Stats == nil || t.Stats.Legend == nil {
		return nil
	}
	l := t.Stats.Legend
	v := &LegendView{
		Uses: l.Uses, Left: l.Left, Ready: l.Ready, Actions: actionViews(l.Actions), Lair: actionViews(l.Lair), ResistLeft: l.ResistLeft,
		Phase: l.Phase, Phases: len(l.Phases), Threshold: l.Threshold,
	}
	if c := s.combat; c != nil && c.Status == domain.CombatActive && len(l.Lair) > 0 {
		v.LairReady = c.Turn <= 20 && l.LairRound != c.Round
	}
	return v
}

func actionViews(as []domain.LegendAction) []LegendActionView {
	out := make([]LegendActionView, 0, len(as))
	for _, a := range as {
		out = append(out, LegendActionView{Name: a.Name, Cost: a.Cost, Text: a.Text})
	}
	return out
}
