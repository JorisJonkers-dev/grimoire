package live

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dying"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// stabilising is the pending Wisdom (Medicine) check that stabilises a dying Character.
const stabilising = "stabilise"

// character reports whether a token is a Character, the only creatures that make death saves.
func character(t domain.Token) bool {
	return t.Stats != nil && strings.HasPrefix(t.Stats.Source, "character:")
}

// moment is when death and revival are measured against.
func (s *state) moment() dying.Now {
	if c := s.combat; c != nil && c.Status == domain.CombatActive {
		return dying.Now{Fight: uuid.UUID(c.ID).String(), Round: c.Round, Day: s.day}
	}
	return dying.Now{Fight: "", Round: 0, Day: s.day}
}

func (s *state) died() dying.Died {
	return dying.Died(s.moment())
}

// lifeAndDeath follows hit points across 0 for a Character: dropping to 0 lays it Unconscious and dying,
// or kills it outright on massive damage; damage while down costs death saves; healing wakes it.
func (r *runtime) lifeAndDeath(w Write, actor domain.Member, c caller.Caller) {
	h := w.HP
	if h == nil || w.Kind == domain.ActionDyingChanged || w.Kind == domain.ActionRevived {
		return
	}
	t := r.st.tokens[h.Token]
	if !character(t) {
		return
	}
	d, down := r.st.dying[t.ID]
	next := Write{Kind: domain.ActionDyingChanged, Token: t}
	switch {
	case h.Before > 0 && h.After == 0 && !down:
		nd := domain.Dying{Token: t.ID, State: dying.State{Successes: 0, Failures: 0, Stable: false, Dead: dying.MassiveDamage(h.Before, h.Raw, t.Stats.HPMax)}}
		if nd.State.Dead {
			nd.Died = r.st.died()
		} else {
			e := domain.Effect{ID: domain.EffectID(uuid.New()), Target: t.ID, Slug: "unconscious", Name: "Unconscious", Level: 1}
			next.effect, nd.Effect = &e, &e.ID
		}
		next.Kind, next.Dying = domain.ActionDowned, &nd
	case h.After == 0 && down && h.Raw > 0 && !d.State.Dead:
		d.State, d.RollID = d.State.Hurt(h.Raw, t.Stats.HPMax, h.Critical), nil
		if d.State.Dead {
			d.Died = r.st.died()
		}
		next.Dying = &d
	case h.After > 0 && down && !d.State.Dead:
		next.Undying = &t.ID
		if d.Effect != nil {
			next.ended = []domain.EffectID{*d.Effect}
		}
	default:
		return
	}
	r.commit(request{}, next, actor, c)
}

// deathSaves opens the death save of every dying Character whose turn it is and who has none out.
func (r *runtime) deathSaves(actor domain.Member, c caller.Caller) {
	f := r.st.combat
	if f == nil || f.Status != domain.CombatActive {
		return
	}
	for _, x := range f.Combatants {
		d, down := r.st.dying[x.TokenID]
		if !f.Acting(x) || !down || !d.State.Rolls() || d.RollID != nil {
			continue
		}
		t := r.st.tokens[x.TokenID]
		notation := strings.Join(append([]string{"1d20"}, r.st.catalog.SaveDice(r.st.actives(t.ID))...), "+")
		roll := r.request(actor, t, t.Label+": death saving throw", notation)
		d.RollID = &roll.ID
		r.commit(request{}, Write{Kind: domain.ActionDyingChanged, Token: t, Dying: &d, Rolls: []domain.Roll{roll}}, actor, c)
		return
	}
}

// pendingDeath finds the dying Character whose death save this roll is.
func (s *state) pendingDeath(id domain.RollID) (domain.Dying, bool) {
	for _, d := range s.dying {
		if d.RollID != nil && *d.RollID == id {
			return d, true
		}
	}
	return domain.Dying{}, false
}

// deathRolled applies a death save: a natural 20 wakes the Character with 1 hit point.
func (r *runtime) deathRolled(d domain.Dying, id domain.RollID) {
	roll, err := r.store.Roll(context.Background(), r.campaign, id)
	if err != nil || roll.Status != domain.StatusResolved {
		return
	}
	t := r.st.tokens[d.Token]
	state, woke := d.State.Save(natural(roll), roll.Total)
	w := Write{Kind: domain.ActionDyingChanged, Token: t}
	switch {
	case woke:
		w.Undying, w.HP = &t.ID, &HPChange{Token: t.ID, Before: t.Stats.HP, After: 1, Raw: -1}
		if d.Effect != nil {
			w.ended = []domain.EffectID{*d.Effect}
		}
	default:
		d.State, d.RollID = state, nil
		if state.Dead {
			d.Died = r.st.died()
		}
		w.Dying = &d
	}
	r.commit(request{}, w, roll.Roller, caller.Caller{Subject: roll.Roller.Subject, Origin: caller.OriginSystem, Client: ""})
}

// planStabilise stabilises a dying Character next to the helper: a DC 10 Wisdom (Medicine) check, or a
// spell such as Spare the Dying, which needs no roll. In a fight it takes the helper's action.
func (r *runtime) planStabilise(m domain.Member, cmd Command) (Write, string) {
	h, x, reason := r.actor(m, cmd.TokenID, false)
	if reason != "" {
		return Write{}, reason
	}
	t, ok := r.st.tokenByID(cmd.TargetID)
	d, down := r.st.dying[t.ID]
	switch {
	case !ok || !down || !d.State.Rolls():
		return Write{}, "Only a dying creature can be stabilised."
	case hex.Distance(hex.Coord{Q: h.Q, R: h.R}, hex.Coord{Q: t.Q, R: t.R}) > 1:
		return Write{}, t.Label + " is out of reach."
	case cmd.Option != "medicine" && cmd.Option != "spell":
		return Write{}, "Stabilise with a Medicine check or a spell."
	}
	w := Write{Kind: domain.ActionTaken, Token: h, taken: &takenAction{action: "", readied: nil}}
	if x != nil {
		w.Combatant = x.ID
	}
	if cmd.Option == "spell" {
		d.State = d.State.Stabilise()
		w.Dying = &d
		return w, ""
	}
	roll := r.request(m, h, "Wisdom (Medicine) check to stabilise "+t.Label+r.dcNote(dying.StabiliseDC), "1d20")
	target := t.ID
	w.Rolls, w.Pending = []domain.Roll{roll}, &domain.PendingAction{RollID: roll.ID, Actor: h.ID, Target: &target, Action: stabilising, DC: dying.StabiliseDC}
	return w, ""
}

// planRevive brings a dead Character back with 1 hit point, while the spell's window lasts.
func (r *runtime) planRevive(m domain.Member, cmd Command) (Write, string) {
	t, ok := r.st.tokenByID(cmd.TargetID)
	d, down := r.st.dying[t.ID]
	spell := dying.Spell(cmd.Option)
	switch {
	case !ok || !down || !d.State.Dead:
		return Write{}, "Only the dead can be revived."
	case spell != dying.Revivify && spell != dying.RaiseDead && spell != dying.Resurrection:
		return Write{}, "Revive with Revivify, Raise Dead or Resurrection."
	case !dying.Revives(spell, d.Died, r.st.moment()):
		return Write{}, "It is too late for " + strings.ReplaceAll(string(spell), "_", " ") + "."
	}
	if !m.DM {
		if _, _, reason := r.actor(m, cmd.TokenID, false); reason != "" {
			return Write{}, reason
		}
	}
	return Write{Kind: domain.ActionRevived, Token: t, Undying: &t.ID, HP: &HPChange{Token: t.ID, Before: 0, After: 1, Raw: -1}}, ""
}

// applyDying keeps a Character's death saves, or drops them when it wakes or is revived, and sets the
// hit points a death save or revival gives back.
func applyDying(s *state, w *Write) {
	if w.Dying != nil {
		s.dying[w.Dying.Token] = *w.Dying
	}
	if w.Undying != nil {
		delete(s.dying, *w.Undying)
	}
	if h := w.HP; h != nil && (w.Kind == domain.ActionDyingChanged || w.Kind == domain.ActionRevived) {
		t := s.tokens[h.Token]
		stats := *t.Stats
		stats.HP = h.After
		t.Stats = &stats
		s.tokens[t.ID] = t
	}
}

// DyingView is a Character at 0 hit points: its death saves, and whether it is stable or dead.
type DyingView struct {
	Successes int    `json:"successes"`
	Failures  int    `json:"failures"`
	Stable    bool   `json:"stable,omitempty"`
	Dead      bool   `json:"dead,omitempty"`
	RollID    string `json:"rollId,omitempty"`
}

func (s *state) dyingView(id domain.TokenID) *DyingView {
	d, ok := s.dying[id]
	if !ok {
		return nil
	}
	v := &DyingView{Successes: d.State.Successes, Failures: d.State.Failures, Stable: d.State.Stable, Dead: d.State.Dead}
	if d.RollID != nil {
		v.RollID = uuid.UUID(*d.RollID).String()
	}
	return v
}
