package live

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/actions"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/attack"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/combat"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// dodging is the Effect the Dodge action puts on its taker until their next turn.
const dodging = "dodging"

// actor finds the token a member takes an action with, and its Combatant when a fight is on; in a
// fight it must be the token's turn with its action unspent, and it must be able to act at all.
func (r *runtime) actor(m domain.Member, tokenID string, fightOnly bool) (domain.Token, *domain.Combatant, string) {
	return r.taker(m, tokenID, fightOnly, owns)
}

// taker is actor for whoever may is satisfied by: its Controller or the DM, or for the actions a
// controlled mount takes, its rider's Player too.
func (r *runtime) taker(m domain.Member, tokenID string, fightOnly bool, may func(domain.Member, domain.Token) bool) (domain.Token, *domain.Combatant, string) {
	t, ok := r.st.tokenByID(tokenID)
	switch {
	case !ok || t.Stats == nil:
		return t, nil, "No such creature."
	case !may(m, t):
		return t, nil, "That token is not yours to play."
	case r.st.catalog.Incapacitated(r.st.actives(t.ID)):
		return t, nil, t.Label + " can't act while Incapacitated."
	}
	c := r.st.combat
	if c == nil || c.Status != domain.CombatActive {
		if fightOnly {
			return t, nil, "That action only matters in combat."
		}
		return t, nil, ""
	}
	x, acting := r.st.combatantOf(t.ID)
	switch {
	case !acting:
		return t, nil, "It is not " + t.Label + "'s turn."
	case !x.Economy.Action:
		return t, nil, t.Label + " has already used their action."
	}
	return t, &x, ""
}

// planAction takes one of the 2024 actions: Dash, Disengage, Dodge and Ready change the turn; Hide,
// Search, Study and Influence open a check; Help, Magic and Utilize hand the DM what was done.
func (r *runtime) planAction(m domain.Member, cmd Command) (Write, string) {
	info, known := actions.Find(actions.Action(cmd.Action))
	if !known {
		return Write{}, "Choose one of the actions."
	}
	inFight := info.Action == actions.Dash || info.Action == actions.Disengage || info.Action == actions.Dodge || info.Action == actions.Ready
	// A rider's Player may ask a controlled mount for an action; barred leaves it the three it takes.
	t, x, reason := r.taker(m, cmd.TokenID, inFight, r.st.steers)
	if reason == "" {
		reason = r.st.barred(t, info.Action)
	}
	if reason != "" {
		return Write{}, reason
	}
	w := Write{Kind: domain.ActionTaken, Token: t, taken: &takenAction{action: info.Action}}
	if x != nil {
		w.Combatant = x.ID
	}
	switch info.Action {
	case actions.Dodge:
		e := domain.Effect{ID: domain.EffectID(uuid.New()), Target: t.ID, Slug: dodging, Name: "Dodging", RoundsLeft: 1, Level: 1}
		w.effect = &e
	case actions.Ready:
		return r.ready(t, cmd, w)
	case actions.Influence:
		plan, reason := r.swayed(m, t, cmd.TargetID)
		if reason != "" {
			return Write{}, reason
		}
		roll := r.abilityCheck(m, t, info, plan.notation)
		roll.Purpose += plan.shownDC
		roll.Modifiers = append(roll.Modifiers, plan.lines...)
		w.Rolls = []domain.Roll{roll}
		if plan.target != nil {
			w.Pending = &domain.PendingAction{RollID: roll.ID, Actor: t.ID, Target: plan.target, Action: string(actions.Influence), DC: plan.dc}
		}
	case actions.Hide, actions.Search, actions.Study:
		w.Rolls = []domain.Roll{r.abilityCheck(m, t, info, attack.D20(attack.Normal))}
		if info.Action == actions.Hide {
			w.Pending = &domain.PendingAction{RollID: w.Rolls[0].ID, Actor: t.ID, Action: string(actions.Hide), DC: actions.HideDC}
		}
	case actions.Help, actions.Magic, actions.Utilize:
		text := t.Label + " takes the " + info.Name + " action"
		if d := strings.TrimSpace(cmd.Detail); d != "" {
			text += ": " + d
		}
		w.manuals = []domain.ManualPrompt{{ID: uuid.New(), Text: text + "."}}
	case actions.Dash, actions.Disengage:
	}
	return w, ""
}

// takenAction is the action a write takes, for the turn it changes.
type takenAction struct {
	action  actions.Action
	readied *domain.Readied
}

// abilityCheck opens an action's ability check, with the skill bonus the token's statblock has for it.
func (r *runtime) abilityCheck(m domain.Member, t domain.Token, info actions.Info, notation string) domain.Roll {
	bonus := map[string]int{"stealth": t.Stats.Stealth, "perception": t.Stats.Perception}[info.Skill]
	label := strings.ToUpper(info.Check[:1]) + info.Check[1:]
	if info.Skill != "" {
		label += " (" + strings.ToUpper(info.Skill[:1]) + info.Skill[1:] + ")"
	}
	return r.request(m, t, info.Name+": "+label+" check", notation, domain.Modifier{Label: label, Value: bonus})
}

// ready sets an attack to fire when its trigger happens, before the token's next turn.
func (r *runtime) ready(t domain.Token, cmd Command, w Write) (Write, string) {
	kind := actions.TriggerKind(cmd.Trigger)
	switch {
	case kind != actions.EntersReach:
		return Write{}, "Choose what sets the readied attack off."
	case cmd.AttackNo < 0 || cmd.AttackNo >= len(t.Stats.Attacks) || t.Stats.Attacks[cmd.AttackNo].ReachFt == 0:
		return Write{}, "Ready an attack the token can make in reach."
	}
	who := ""
	if cmd.TargetID != "" {
		target, ok := r.st.tokenByID(cmd.TargetID)
		if !ok {
			return Write{}, "No such creature to watch for."
		}
		who = uuid.UUID(target.ID).String()
	}
	w.taken.readied = &domain.Readied{Trigger: actions.Trigger{Kind: kind, Who: who}, AttackNo: cmd.AttackNo}
	return w, ""
}

// applyAction spends the action and changes the turn: Dash adds a Speed of movement, Disengage keeps
// the mover safe from opportunity attacks, Ready keeps the attack waiting.
func applyAction(s *state, w *Write) {
	if w.Pending != nil {
		s.pending = append(s.pending, *w.Pending)
	}
	if s.combat == nil || w.taken == nil {
		return
	}
	i := slices.IndexFunc(s.combat.Combatants, func(x domain.Combatant) bool { return x.ID == w.Combatant })
	if i < 0 {
		return
	}
	x := &s.combat.Combatants[i]
	x.Economy, _ = x.Economy.Spend(combat.Action)
	switch w.taken.action {
	case actions.Dash:
		x.Economy.MovementFt += s.speedOf(*x)
	case actions.Disengage:
		x.Disengaged = true
	case actions.Ready:
		x.Readied = w.taken.readied
	case actions.Dodge, actions.Help, actions.Hide, actions.Influence, actions.Magic, actions.Search, actions.Study, actions.Utilize:
	}
	w.Combat = s.combat
}

// planUnarmed makes an Unarmed Strike that grapples or shoves: the target, within 5 feet, saves with
// Strength or Dexterity, whichever it is better at, against the attacker's Unarmed Strike DC.
func (r *runtime) planUnarmed(m domain.Member, cmd Command) (Write, string) {
	option := actions.Option(cmd.Option)
	if option != actions.Grapple && option != actions.ShovePush && option != actions.ShoveDown {
		return Write{}, "Grapple, or shove away or down."
	}
	a, x, reason := r.actor(m, cmd.TokenID, true)
	if reason == "" {
		reason = r.st.reined(a, "")
	}
	if reason != "" {
		return Write{}, reason
	}
	t, ok := r.st.tokenByID(cmd.TargetID)
	switch {
	case !ok || t.Stats == nil || t.ID == a.ID:
		return Write{}, "Choose a creature to grapple or shove."
	case hex.Distance(hex.Coord{Q: a.Q, R: a.R}, hex.Coord{Q: t.Q, R: t.R}) > 1:
		return Write{}, t.Label + " is out of reach."
	}
	dc := max(1, a.Stats.UnarmedDC)
	ability, bonus := actions.Resist(t.Stats.Saves)
	w := Write{Kind: domain.ActionUnarmed, Token: a, Combatant: x.ID, taken: &takenAction{action: "", readied: nil}}
	p := r.st.catalog.ForSave(r.st.actives(t.ID), ability)
	if p.Fails {
		r.st.unarmedOutcome(&w, a, t, string(option))
		return w, ""
	}
	name := strings.ToUpper(ability[:1]) + ability[1:]
	notation := strings.Join(append([]string{attack.D20(attack.ModeOf(len(p.Advantages), len(p.Disadvantages)))}, p.Dice...), "+")
	roll := r.request(m, t, fmt.Sprintf("%s save against %s's %s (DC %d)", name, a.Label, strings.ReplaceAll(string(option), "_", " "), dc), notation,
		domain.Modifier{Label: name + " save", Value: bonus}, domain.Modifier{Label: "Exhaustion", Value: -p.Penalty})
	target := t.ID
	w.Rolls, w.Pending = []domain.Roll{roll}, &domain.PendingAction{RollID: roll.ID, Actor: a.ID, Target: &target, Action: string(option), DC: dc}
	return w, ""
}

// pendingAction finds the Hide, Grapple or Shove waiting on a roll.
func (s *state) pendingAction(id domain.RollID) (domain.PendingAction, bool) {
	i := slices.IndexFunc(s.pending, func(p domain.PendingAction) bool { return p.RollID == id })
	if i < 0 {
		return domain.PendingAction{}, false
	}
	return s.pending[i], true
}

// actionRolled settles a Hide (Invisible on a 15 or more) or a Grapple or Shove (it lands when the save fails).
func (r *runtime) actionRolled(p domain.PendingAction) {
	roll, err := r.store.Roll(context.Background(), r.campaign, p.RollID)
	if err != nil || roll.Status != domain.StatusResolved {
		return
	}
	a := r.st.tokens[p.Actor]
	w := Write{Kind: domain.ActionResolved, Token: a, Settled: p.RollID}
	if p.Object != nil {
		sys := caller.Caller{Subject: roll.Roller.Subject, Origin: caller.OriginSystem, Client: ""}
		r.commit(request{}, w, roll.Roller, sys)
		r.objectChecked(p, roll.Total, roll.Roller, sys)
		return
	}
	switch {
	case p.Action == tableRoll:
		r.tableRolled(w, p, roll)
		return
	case p.Action == keepSeat:
		r.seatRolled(w, p, roll)
		return
	case p.Action == stabilising:
		if d, down := r.st.dying[*p.Target]; down && roll.Total >= p.DC && d.State.Rolls() {
			d.State = d.State.Stabilise()
			w.Dying = &d
		}
	case p.Action == concentrating:
		if roll.Total < p.DC {
			for _, e := range r.st.held(a.ID) {
				w.ended = append(w.ended, e.ID)
			}
		}
	case p.Action == string(actions.Influence):
		w.Attitude = r.st.swayedTo(a, *p.Target, roll.Total, p.DC)
	case p.Action == string(actions.Hide) && roll.Total >= p.DC:
		e := domain.Effect{ID: domain.EffectID(uuid.New()), Target: a.ID, Slug: "invisible", Name: "Invisible", Level: 1}
		w.effect = &e
	case p.Target != nil && roll.Total < p.DC:
		r.st.unarmedOutcome(&w, a, r.st.tokens[*p.Target], p.Action)
	}
	r.commit(request{}, w, roll.Roller, caller.Caller{Subject: roll.Roller.Subject, Origin: caller.OriginSystem, Client: ""})
}

// unarmedOutcome lands a Grapple or Shove: the target is Grappled by the attacker, knocked Prone, or
// pushed 5 feet straight away when that hex is open.
func (s *state) unarmedOutcome(w *Write, a, t domain.Token, option string) {
	switch actions.Option(option) {
	case actions.Grapple, actions.ShoveDown, toppled:
		e := domain.Effect{ID: domain.EffectID(uuid.New()), Target: t.ID, Slug: "prone", Name: "Prone", Level: 1}
		if actions.Option(option) == actions.Grapple {
			e.Slug, e.Name, e.Source = "grappled", "Grappled", &a.ID
		}
		w.effect = &e
	case actions.ShovePush:
		to := hex.Coord{Q: 2*t.Q - a.Q, R: 2*t.R - a.R}
		if s.onBoard(to) && !s.occupied(to) {
			moved := t
			moved.Q, moved.R = to.Q, to.R
			w.Pushed = &moved
		}
	}
}

// occupied reports whether a creature stands on a hex.
func (s *state) occupied(c hex.Coord) bool {
	for _, t := range s.tokens {
		if t.Q == c.Q && t.R == c.R && t.Stats != nil {
			return true
		}
	}
	return false
}

// applyResolved settles a pending action and moves a pushed creature.
func applyResolved(s *state, w *Write) {
	applyDying(s, w)
	s.pending = slices.DeleteFunc(s.pending, func(p domain.PendingAction) bool { return p.RollID == w.Settled })
	if a := w.Attitude; a != nil {
		s.attitudes = append(slices.DeleteFunc(s.attitudes, func(x domain.Attitude) bool { return x.Token == a.Token && x.Character == a.Character }), *a)
	}
	if w.Pushed != nil {
		s.tokens[w.Pushed.ID] = *w.Pushed
	}
}

// dragged is the creature a token holds Grappled, if any; dragging it doubles what moving costs.
func (s *state) dragged(t domain.TokenID) (domain.Token, bool) {
	for _, e := range s.fx.Active {
		if e.Source != nil && *e.Source == t && s.catalog.Immobile([]effects.Active{{Slug: e.Slug, Source: "", Level: e.Level}}) {
			held, ok := s.tokens[e.Target]
			return held, ok
		}
	}
	return domain.Token{}, false
}

// readied finds the first step of a path that sets off a readied attack: the mover stops on the hex
// that sets it off.
func (s *state) readied(mover domain.Token, path []hex.Coord) (int, domain.Token, int, bool) {
	if s.combat == nil || s.combat.Status != domain.CombatActive {
		return 0, domain.Token{}, 0, false
	}
	for k := 1; k < len(path); k++ {
		for _, x := range s.combat.Combatants {
			h, ok := s.tokens[x.TokenID]
			if x.Readied == nil || !ok || h.ID == mover.ID || !x.Economy.Reaction || !standing(h) || s.catalog.Incapacitated(s.actives(h.ID)) ||
				!s.offers(h, domain.PromptReadied) {
				continue
			}
			at := hex.Coord{Q: h.Q, R: h.R}
			step := actions.Step{Mover: uuid.UUID(mover.ID).String(), BeforeFt: hex.Distance(at, path[k-1]) * hex.FeetPerHex, AfterFt: hex.Distance(at, path[k]) * hex.FeetPerHex}
			if x.Readied.Trigger.Fires(step, h.Stats.Attacks[x.Readied.AttackNo].ReachFt) {
				return k, h, x.Readied.AttackNo, true
			}
		}
	}
	return 0, domain.Token{}, 0, false
}

// planInteract uses the turn's one free object interaction: drawing a weapon, opening a door.
func (r *runtime) planInteract(m domain.Member, cmd Command) (Write, string) {
	t, ok := r.st.tokenByID(cmd.TokenID)
	what := strings.TrimSpace(cmd.Detail)
	switch {
	case !ok || t.Stats == nil:
		return Write{}, "No such creature."
	case !m.DM && (t.Controller == nil || *t.Controller != m.ID):
		return Write{}, "That token is not yours to play."
	case what == "" || len([]rune(what)) > 200:
		return Write{}, "Say what the object interaction does."
	}
	w := Write{Kind: domain.ActionObjectUsed, Token: t, manuals: []domain.ManualPrompt{{ID: uuid.New(), Text: t.Label + " " + what + "."}}}
	if c := r.st.combat; c != nil && c.Status == domain.CombatActive {
		x, acting := r.st.combatantOf(t.ID)
		switch {
		case !acting:
			return Write{}, "It is not " + t.Label + "'s turn."
		case !x.Economy.Interaction:
			return Write{}, t.Label + " has used their free object interaction; another takes the Utilize action."
		}
		w.Combatant = x.ID
	}
	return w, ""
}

// applyInteraction uses up the free object interaction of a Combatant's turn.
func applyInteraction(s *state, w *Write) {
	if w.Swap != nil {
		applySwap(s, w)
		return
	}
	if s.combat == nil {
		return
	}
	i := slices.IndexFunc(s.combat.Combatants, func(x domain.Combatant) bool { return x.ID == w.Combatant })
	if i < 0 {
		return
	}
	s.combat.Combatants[i].Economy, _ = s.combat.Combatants[i].Economy.Interact()
	w.Combat = s.combat
}
