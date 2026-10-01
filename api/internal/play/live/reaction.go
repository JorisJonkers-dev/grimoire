package live

import (
	"context"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/combat"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// DefaultReactionTimeout is how long a Reaction Prompt waits when the Campaign's setting cannot be read.
const DefaultReactionTimeout = 10 * time.Second

// systemActor signs what nobody did: a prompt that ran out of time.
var systemActor = domain.Member{ID: uuid.Nil, Subject: "", Name: "Grimoire", DM: false} //nolint:gochecknoglobals // a fixed signature

// prompt opens a Reaction Prompt that declines on its own once the Campaign's reaction timeout passes.
func (r *runtime) prompt(kind string, reactor, trigger domain.Token, attackNo int, effect string) *domain.ReactionPrompt {
	wait := DefaultReactionTimeout
	if s, err := r.store.ReactionTimeout(context.Background(), r.st.session.CampaignID); err == nil {
		wait = time.Duration(s) * time.Second
	}
	return &domain.ReactionPrompt{
		ID: uuid.New(), Kind: kind, Reactor: reactor.ID, Trigger: trigger.ID, AttackNo: attackNo, Effect: effect, Deadline: r.now().Add(wait),
	}
}

// walkWrite walks a token along a path, stopping where it leaves an enemy's reach if that enemy can
// still make an opportunity attack; the rest of the walk waits for the answer. A resumed walk skips the
// step whose opportunity was already offered.
func (r *runtime) walkWrite(t domain.Token, path []hex.Coord, resumed bool) Write {
	costs := pathCosts(r.st.walkGrid(true, t, nil), path)
	for i := range costs {
		costs[i] *= effects.MoveMultiplier(r.st.actives(t.ID))
	}
	w := Write{Kind: domain.ActionTokenWalked, Token: t, Path: path, CostFt: costs[len(costs)-1]}
	first := 0
	if resumed {
		first = 1
	}
	k, reactor, no, ok := r.st.opportunity(t, path, first)
	if !ok {
		return w
	}
	with := reactor.Stats.Attacks[no]
	w.prompt = r.prompt(domain.PromptOpportunity, reactor, t, no,
		fmt.Sprintf("%s leaves %s's reach: %s attack (%+d) before they go on.", t.Label, reactor.Label, with.Name, with.ToHit))
	w.resume = &domain.Resume{Token: t.ID, Path: path[k:], CostFt: costs[len(costs)-1] - costs[k]}
	if k == 0 {
		return Write{Kind: domain.ActionReactionOffered, Token: reactor, prompt: w.prompt, resume: w.resume}
	}
	w.Path, w.CostFt = path[:k+1], costs[k]
	return w
}

// pathCosts is the movement spent reaching each hex of a path; a step that can no longer be taken ends it.
func pathCosts(g hex.Grid, path []hex.Coord) []int {
	costs := []int{0}
	for i := 1; i < len(path); i++ {
		step, ok := hex.StepCost(g, path[i-1], path[i], false)
		if !ok {
			break
		}
		costs = append(costs, costs[i-1]+step)
	}
	return costs
}

// opportunity finds the first step of a path that leaves the reach of a standing enemy Combatant that
// sees the mover and still has its reaction.
func (s *state) opportunity(mover domain.Token, path []hex.Coord, first int) (int, domain.Token, int, bool) {
	if s.combat == nil || s.combat.Status != domain.CombatActive {
		return 0, domain.Token{}, 0, false
	}
	ids := make([]string, 0, len(s.tokens))
	for id := range s.tokens {
		ids = append(ids, uuid.UUID(id).String())
	}
	slices.Sort(ids)
	g := s.sightGrid()
	for k := first; k+1 < len(path); k++ {
		for _, id := range ids {
			h := s.tokens[domain.TokenID(uuid.MustParse(id))]
			x, fighting := s.fighter(h.ID)
			no := slices.IndexFunc(attacksOf(h), func(a domain.Attack) bool { return a.ReachFt > 0 })
			if !fighting || !x.Economy.Reaction || !standing(h) || no < 0 || (h.Kind == domain.TokenParty) == (mover.Kind == domain.TokenParty) {
				continue
			}
			at := hex.Coord{Q: h.Q, R: h.R}
			reach := h.Stats.Attacks[no].ReachFt
			if hex.Distance(at, path[k])*hex.FeetPerHex <= reach && hex.Distance(at, path[k+1])*hex.FeetPerHex > reach &&
				hex.LineOfSight(g, at, path[k]).Visible {
				return k, h, no, true
			}
		}
	}
	return 0, domain.Token{}, 0, false
}

func attacksOf(t domain.Token) []domain.Attack {
	if t.Stats == nil {
		return nil
	}
	return t.Stats.Attacks
}

// fighter finds a token's Combatant.
func (s *state) fighter(id domain.TokenID) (domain.Combatant, bool) {
	for _, x := range s.combat.Combatants {
		if x.TokenID == id {
			return x, true
		}
	}
	return domain.Combatant{}, false
}

// armor is a token's AC now, with Shield if it is up.
func (s *state) armor(t domain.Token) int {
	if x, ok := s.fighter(t.ID); ok && x.Shielded {
		return t.Stats.AC + 5
	}
	return t.Stats.AC
}

// shieldPrompt offers Shield to a hit target that can cast it and whose raised AC would turn the hit
// into a miss.
func (r *runtime) shieldPrompt(a, t domain.Token, p domain.PendingAttack, total int) *domain.ReactionPrompt {
	x, ok := r.st.fighter(t.ID)
	ac := r.st.armor(t) + p.CoverBonus
	if !t.CanShield || !ok || !x.Economy.Reaction || x.Shielded || total >= ac+5 {
		return nil
	}
	return r.prompt(domain.PromptShield, t, a, p.AttackNo, fmt.Sprintf("Shield: AC %d → %d, so the attack (%d) would miss.", ac, ac+5, total))
}

func (r *runtime) planReact(m domain.Member, cmd Command) (Write, string) {
	var p *domain.ReactionPrompt
	if r.st.combat != nil {
		p = r.st.combat.Prompt
	}
	if p == nil {
		return Write{}, "Nothing is waiting on a reaction."
	}
	reactor := r.st.tokens[p.Reactor]
	if !m.DM && (reactor.Controller == nil || *reactor.Controller != m.ID) {
		return Write{}, "That reaction is not yours to take."
	}
	return r.answer(p, cmd.Use, m), ""
}

// answer turns a reaction into a write: an opportunity attack opens its attack roll, Shield raises AC.
func (r *runtime) answer(p *domain.ReactionPrompt, use bool, m domain.Member) Write {
	reactor := r.st.tokens[p.Reactor]
	x, _ := r.st.fighter(reactor.ID)
	w := Write{Kind: domain.ActionReactionDeclined, Token: reactor, Combatant: x.ID, answered: p}
	if !use {
		return w
	}
	w.Kind = domain.ActionReactionUsed
	if p.Kind == domain.PromptShield {
		return w
	}
	trigger, with := r.st.tokens[p.Trigger], reactor.Stats.Attacks[p.AttackNo]
	roll := r.request(m, reactor, "Opportunity attack against "+trigger.Label+" with "+with.Name, "1d20", domain.Modifier{Label: with.Name, Value: with.ToHit})
	w.Rolls = []domain.Roll{roll}
	w.attack = &domain.PendingAttack{
		ID: uuid.New(), Attacker: reactor.ID, Target: trigger.ID, AttackNo: p.AttackNo, Stage: domain.StageToHit, RollID: roll.ID, Opportunity: true,
	}
	return w
}

// applyReaction records a prompt, its answer, and what the answer changes.
func applyReaction(s *state, w *Write) {
	c := s.combat
	if w.Kind == domain.ActionReactionOffered {
		c.Prompt = w.prompt
		if w.resume != nil {
			c.Resume = w.resume
		}
		if w.attack != nil {
			c.Attack = w.attack
		}
		w.Combat = c
		return
	}
	c.Prompt = nil
	if w.Kind == domain.ActionReactionUsed {
		i := slices.IndexFunc(c.Combatants, func(x domain.Combatant) bool { return x.ID == w.Combatant })
		c.Combatants[i].Economy, _ = c.Combatants[i].Economy.Spend(combat.Reaction)
		c.Combatants[i].Shielded = w.answered.Kind == domain.PromptShield
		if w.attack != nil {
			c.Attack = w.attack
		}
	}
	w.Combat = c
}

// follow carries the fight on after a write that leaves something to do: a declined or spent reaction
// lets the interrupted attack or walk go on, and a finished opportunity attack lets the walk resume.
func (r *runtime) follow(w Write, actor domain.Member, c caller.Caller) {
	if actor.DM {
		r.dm = &actor
	}
	switch {
	case w.Kind == domain.ActionRestTaken && w.Rest == RestLong:
		r.encounterChecks(prep.TriggerLongRest, prep.DueNextRest, actor, c)
	case w.Kind == domain.ActionRestTaken:
		r.encounterChecks(prep.TriggerShortRest, prep.DueNextRest, actor, c)
	case w.Kind == domain.ActionTravelLeg:
		r.encounterChecks(prep.TriggerTravelLeg, prep.DueNextTravel, actor, c)
	}
	f := r.st.combat
	if f == nil {
		r.ambush(c)
		return
	}
	switch {
	case w.answered != nil && w.answered.Kind == domain.PromptShield && f.Attack != nil:
		r.shieldAnswered(w.Kind == domain.ActionReactionUsed, actor, c)
	case w.answered != nil && w.Kind == domain.ActionReactionDeclined, f.Attack == nil && f.Prompt == nil && f.Resume != nil:
		r.resumeWalk(actor, c)
	}
}

// shieldAnswered finishes the attack Shield was offered against: a miss if it went up, otherwise the hit.
func (r *runtime) shieldAnswered(raised bool, actor domain.Member, c caller.Caller) {
	p := *r.st.combat.Attack
	a, t := r.st.tokens[p.Attacker], r.st.tokens[p.Target]
	if raised {
		r.commit(request{}, Write{Kind: domain.ActionAttackMissed, Token: a}, actor, c)
		return
	}
	roll, err := r.store.Roll(context.Background(), r.st.session.CampaignID, p.RollID)
	if err != nil {
		r.log.Error("live: attack roll", "error", err)
		return
	}
	r.commit(request{}, r.hit(a, t, p, false, roll), actor, c)
}

// resumeWalk walks the rest of an interrupted walk, unless its walker fell or left.
func (r *runtime) resumeWalk(actor domain.Member, c caller.Caller) {
	rest := r.st.combat.Resume
	t, ok := r.st.tokens[rest.Token]
	if !ok || !standing(t) || len(rest.Path) < 2 {
		r.commit(request{}, Write{Kind: domain.ActionTokenWalked, Token: t, Path: []hex.Coord{{Q: t.Q, R: t.R}}}, actor, c)
		return
	}
	costs := pathCosts(r.st.walkGrid(true, t, nil), rest.Path)
	r.commit(request{}, r.walkWrite(t, rest.Path[:len(costs)], true), actor, c)
}

// arm starts the countdown of a new Reaction Prompt; when it runs out the prompt declines.
func (r *runtime) arm() {
	if r.st.combat == nil || r.st.combat.Prompt == nil || r.st.combat.Prompt.ID == r.armed {
		return
	}
	p := r.st.combat.Prompt
	r.armed = p.ID
	time.AfterFunc(max(p.Deadline.Sub(r.now()), 0), func() {
		select {
		case r.cmds <- request{cmd: Command{Kind: cmdPromptTimeout, promptID: p.ID}}:
		case <-r.done:
		}
	})
}

// timedOut declines a prompt nobody answered.
func (r *runtime) timedOut(id uuid.UUID) {
	if r.st.combat == nil || r.st.combat.Prompt == nil || r.st.combat.Prompt.ID != id {
		return
	}
	r.commit(request{}, r.answer(r.st.combat.Prompt, false, systemActor), systemActor, caller.Caller{Subject: "", Origin: caller.OriginSystem, Client: ""})
}

// promptView shows a Reaction Prompt to the DM, and to anyone who can see both creatures in it.
func (s *state) promptView(a Audience, seen map[hex.Coord]bool, now time.Time) *PromptView {
	p := s.combat.Prompt
	if p == nil {
		return nil
	}
	if a != AudienceDM && (!s.shows(s.tokens[p.Reactor], seen) || !s.shows(s.tokens[p.Trigger], seen)) {
		return nil
	}
	return &PromptView{
		ID: p.ID.String(), Kind: p.Kind, ReactorID: uuid.UUID(p.Reactor).String(), TriggerID: uuid.UUID(p.Trigger).String(), Effect: p.Effect,
		SecondsLeft: int(math.Ceil(max(p.Deadline.Sub(now), 0).Seconds())),
	}
}
