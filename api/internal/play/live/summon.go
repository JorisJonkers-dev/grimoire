package live

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/combat"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/tactics"
)

// planSummon casts an Effect that summons. The creatures appear on the free hexes nearest the point, on
// the caster's side and under its Controller, kept here by an Effect on the caster. In a fight it takes
// the caster's action, and each creature joins the roster: on the caster's Initiative, or rolling its own.
func (r *runtime) planSummon(m domain.Member, cmd Command) (Write, string) {
	caster, x, reason := r.actor(m, cmd.TokenID, false)
	if reason != "" {
		return Write{}, reason
	}
	slug := strings.ToLower(strings.TrimSpace(cmd.Effect))
	sm, ok := r.st.catalog.SummonOf(slug)
	at := hex.Coord{Q: cmd.Q, R: cmd.R}
	switch {
	case !ok:
		return Write{}, "That is not a summoning the rules know."
	case !r.st.onBoard(at):
		return Write{}, "That hex is off the map."
	}
	free := r.st.freeHexes(at, sm.Count)
	if len(free) < sm.Count {
		return Write{}, "There is no room for them there."
	}
	name, stats, err := r.stats.Monster(context.Background(), r.st.session.CampaignID, sm.Monster)
	if err != nil {
		return Write{}, "No such creature: " + sm.Monster + "."
	}
	def, _ := r.st.catalog.Lookup(slug)
	e := domain.Effect{ID: domain.EffectID(uuid.New()), Target: caster.ID, Source: &caster.ID, Slug: slug, Name: def.Name, Concentration: def.Concentration, Level: 1}
	r.st.lasting(&e, def.Duration)
	w := Write{Kind: domain.ActionSummoned, Token: caster, effect: &e}
	if def.Concentration {
		for _, old := range r.st.held(caster.ID) {
			w.ended = append(w.ended, old.ID)
		}
	}
	for i := range sm.Count {
		label := name
		if sm.Count > 1 {
			label = name + " " + strconv.Itoa(i+1)
		}
		st := stats
		w.Spawned = append(w.Spawned, domain.Token{
			ID: domain.TokenID(uuid.New()), Label: string([]rune(label)[:min(len([]rune(label)), 40)]), Kind: caster.Kind, Q: free[i].Q, R: free[i].R,
			Controller: caster.Controller, Stats: &st, Tactics: tactics.FromIntelligence, CanShield: st.Shield, Summon: &e.ID,
		})
	}
	if x != nil {
		w.Combatant = x.ID
		w.summons = r.joinRoster(m, *x, sm.Shares, &w)
	}
	return w, ""
}

// joinRoster makes each summoned creature a Combatant its summoner owns. One that shares the summoner's
// turn takes its Initiative; one that does not rolls its own.
func (r *runtime) joinRoster(m domain.Member, owner domain.Combatant, shares bool, w *Write) []domain.Combatant {
	var out []domain.Combatant
	for _, t := range w.Spawned {
		x := domain.Combatant{
			ID: domain.CombatantID(uuid.New()), TokenID: t.ID, RollID: owner.RollID, InitiativeBonus: max(-10, min(20, t.Stats.Initiative)),
			SpeedFt: max(0, min(120, t.Stats.SpeedFt)), Owner: &owner.ID, Economy: combat.Fresh(t.Stats.SpeedFt),
		}
		if shares && owner.Initiative != nil {
			count := *owner.Initiative
			x.Initiative = &count
		} else {
			roll := r.initiativeRoll(m, t, x.InitiativeBonus)
			x.RollID = roll.ID
			w.Rolls = append(w.Rolls, roll)
		}
		out = append(out, x)
	}
	return out
}

// planCommand spends the summoner's Bonus Action so a summoned creature does more than Dodge this round.
func (r *runtime) planCommand(m domain.Member, cmd Command) (Write, string) {
	owner, ok := r.st.tokenByID(cmd.TokenID)
	switch {
	case !ok || owner.Stats == nil:
		return Write{}, "No such creature."
	case !m.DM && (owner.Controller == nil || *owner.Controller != m.ID):
		return Write{}, "That token is not yours to play."
	case r.st.combat == nil || r.st.combat.Status != domain.CombatActive:
		return Write{}, "Commands matter only in combat."
	case r.st.catalog.Incapacitated(r.st.actives(owner.ID)):
		return Write{}, owner.Label + " can't act while Incapacitated."
	}
	x, acting := r.st.combatantOf(owner.ID)
	summon, found := r.st.tokenByID(cmd.TargetID)
	sx, _ := r.st.fighter(summon.ID)
	switch {
	case !acting:
		return Write{}, "It is not " + owner.Label + "'s turn."
	case !found || sx.Owner == nil || *sx.Owner != x.ID:
		return Write{}, "Command a creature " + owner.Label + " summoned."
	case !x.Economy.BonusAction:
		return Write{}, owner.Label + " has already used their Bonus Action."
	case sx.Commanded:
		return Write{}, summon.Label + " already has its orders this round."
	}
	return Write{Kind: domain.ActionCommanded, Token: summon, Combatant: x.ID, commanded: sx.ID}, ""
}

// applySummon puts the summoned creatures on the board and in the roster and spends the summoner's
// action, or spends its Bonus Action on a command.
func applySummon(s *state, w *Write) {
	for _, t := range w.Spawned {
		s.tokens[t.ID] = t
	}
	if s.combat == nil {
		return
	}
	c := s.combat
	i := slices.IndexFunc(c.Combatants, func(x domain.Combatant) bool { return x.ID == w.Combatant })
	if w.Kind == domain.ActionCommanded {
		c.Combatants[i].Economy, _ = c.Combatants[i].Economy.Spend(combat.BonusAction)
		j := slices.IndexFunc(c.Combatants, func(x domain.Combatant) bool { return x.ID == w.commanded })
		c.Combatants[j].Commanded = true
		w.Combat = c
		return
	}
	if i >= 0 {
		c.Combatants[i].Economy, _ = c.Combatants[i].Economy.Spend(combat.Action)
	}
	c.Combatants = append(c.Combatants, w.summons...)
	w.Combat = c
}

// waiting reports why a summoned creature that needs its summoner's command can only Dodge.
func (s *state) uncommanded(t domain.Token) string {
	if t.Summon == nil || s.combat == nil {
		return ""
	}
	x, _ := s.combatantOf(t.ID)
	sm, _ := s.catalog.SummonOf(s.summonSlug(*t.Summon))
	if !sm.NeedsCommand || x.Commanded {
		return ""
	}
	return t.Label + " takes the Dodge action until it is commanded."
}

// summonSlug is the slug of the Effect that keeps a summoned creature here.
func (s *state) summonSlug(id domain.EffectID) string {
	for _, e := range s.fx.Active {
		if e.ID == id {
			return e.Slug
		}
	}
	return ""
}

// dismiss sends away summoned creatures whose Effect has ended, with their Combatants and Effects;
// it reports whether any left.
func (s *state) dismiss(w *Write) bool {
	gone := false
	for {
		var leaving []domain.TokenID
		for id, t := range s.tokens {
			if t.Summon != nil && !slices.ContainsFunc(s.fx.Active, func(e domain.Effect) bool { return e.ID == *t.Summon }) {
				leaving = append(leaving, id)
			}
		}
		if len(leaving) == 0 {
			return gone
		}
		for _, id := range leaving {
			t := s.tokens[id]
			delete(s.tokens, id)
			dropCombatant(s, &Write{Token: t})
			if s.combat != nil {
				w.Combat = s.combat
			}
			s.forget(id)
			w.Dismissed = append(w.Dismissed, id)
		}
		gone = true
	}
}
