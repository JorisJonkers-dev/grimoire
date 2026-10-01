package live

import (
	"context"
	"slices"
	"sort"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/combat"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// MaxCombatants caps one Combat.
const MaxCombatants = 50

func (r *runtime) planCombat(m domain.Member, cmd Command) (Write, string) {
	if cmd.Kind == CmdStartCombat {
		return r.planStart(m, cmd)
	}
	c := r.st.combat
	if c == nil {
		return Write{}, "There is no fight on."
	}
	if cmd.Kind == CmdEndCombat {
		ended := *c
		ended.Status, ended.EndedAt, ended.Attack, ended.Prompt, ended.Resume = domain.CombatEnded, r.now(), nil, nil, nil
		return Write{Kind: domain.ActionCombatEnded, Combat: &ended, lootTable: cmd.LootTableID}, ""
	}
	x, t, reason := r.combatant(m, cmd.CombatantID)
	if reason != "" {
		return Write{}, reason
	}
	if cmd.Kind == CmdEndTurn {
		if !c.Acting(x) {
			return Write{}, "It is not " + t.Label + "'s turn."
		}
		rolls, saves := r.saves(m, t)
		return Write{Kind: domain.ActionTurnEnded, Token: t, Combatant: x.ID, Rolls: rolls, newSaves: saves}, ""
	}
	res, ok := map[string]combat.Resource{ResourceAction: combat.Action, ResourceBonusAction: combat.BonusAction, ResourceReaction: combat.Reaction}[cmd.Resource]
	switch {
	case !ok:
		return Write{}, "Spend an action, a bonus action or a reaction."
	case res != combat.Reaction && !c.Acting(x):
		return Write{}, "It is not " + t.Label + "'s turn."
	case c.Status != domain.CombatActive:
		return Write{}, "Roll initiative first."
	}
	if _, ok := x.Economy.Spend(res); !ok {
		return Write{}, "That is already spent this turn."
	}
	return Write{Kind: domain.ActionResourceSpent, Token: t, Combatant: x.ID, resource: res}, ""
}

// combatant finds a Combatant that the member may act for: the DM, or its token's Controller.
func (r *runtime) combatant(m domain.Member, id string) (domain.Combatant, domain.Token, string) {
	cid, _ := uuid.Parse(id)
	for _, x := range r.st.combat.Combatants {
		if x.ID != domain.CombatantID(cid) {
			continue
		}
		t := r.st.tokens[x.TokenID]
		if !m.DM && (t.Controller == nil || *t.Controller != m.ID) {
			return x, t, "That combatant is not yours to play."
		}
		return x, t, ""
	}
	return domain.Combatant{}, domain.Token{}, "No such combatant."
}

func (r *runtime) planStart(dm domain.Member, cmd Command) (Write, string) {
	if r.st.combat != nil {
		return Write{}, "A fight is already on."
	}
	if len(cmd.Combatants) == 0 || len(cmd.Combatants) > MaxCombatants {
		return Write{}, "Choose between 1 and 50 combatants."
	}
	c := &domain.Combat{ID: domain.CombatID(uuid.New()), Status: domain.CombatRolling, StartedAt: r.now()}
	var rolls []domain.Roll
	seen := map[domain.TokenID]bool{}
	for _, in := range cmd.Combatants {
		id, _ := uuid.Parse(in.TokenID)
		t, ok := r.st.tokens[domain.TokenID(id)]
		switch {
		case !ok || seen[t.ID]:
			return Write{}, "Choose each token once."
		case in.InitiativeBonus < -10 || in.InitiativeBonus > 20:
			return Write{}, "Initiative bonuses run from -10 to +20."
		case in.SpeedFt < 0 || in.SpeedFt > 120:
			return Write{}, "Speed runs from 0 to 120 feet."
		}
		seen[t.ID] = true
		roll := r.initiativeRoll(dm, t, in.InitiativeBonus)
		rolls = append(rolls, roll)
		c.Combatants = append(c.Combatants, domain.Combatant{
			ID: domain.CombatantID(uuid.New()), TokenID: t.ID, RollID: roll.ID, InitiativeBonus: in.InitiativeBonus, SpeedFt: in.SpeedFt,
		})
	}
	return Write{Kind: domain.ActionCombatStarted, Combat: c, Rolls: rolls}, ""
}

// initiativeRoll opens the Roll Request a Combatant's Controller fills in on a Roll Card; the DM rolls
// for everyone else, and for a Controller who has left the Campaign.
func (r *runtime) initiativeRoll(dm domain.Member, t domain.Token, bonus int) domain.Roll {
	roller := dm
	if t.Controller != nil {
		if m, err := r.members.Member(context.Background(), r.st.session.CampaignID, *t.Controller); err == nil {
			roller = m
		}
	}
	roll := domain.Roll{
		ID: domain.RollID(uuid.New()), CampaignID: r.st.session.CampaignID, Purpose: "Initiative for " + t.Label, Notation: "1d20",
		RequestedBy: dm.Name, Roller: roller, Status: domain.StatusPending, Dice: []domain.Die{{No: 0, Group: 0, Faces: 20}},
	}
	if bonus != 0 {
		roll.Modifiers = []domain.Modifier{{Label: "Initiative", Value: bonus}}
	}
	return roll
}

// rolled takes a resolved Roll Request into the Combat it belongs to: an initiative, or the attack on
// the table.
func (r *runtime) rolled(req request) {
	if r.outOfCombatRoll(req.cmd.rollID) {
		return
	}
	c := r.st.combat
	if c == nil {
		return
	}
	if c.Attack != nil && c.Attack.RollID == req.cmd.rollID {
		roll, err := r.store.Roll(context.Background(), r.st.session.CampaignID, c.Attack.RollID)
		if err == nil && roll.Status == domain.StatusResolved {
			r.attackRolled(roll)
		}
		return
	}
	for _, x := range c.Combatants {
		if x.RollID != req.cmd.rollID || x.Initiative != nil {
			continue
		}
		roll, err := r.store.Roll(context.Background(), r.st.session.CampaignID, x.RollID)
		if err != nil || roll.Status != domain.StatusResolved {
			return
		}
		w := Write{Kind: domain.ActionInitiativeRolled, Token: r.st.tokens[x.TokenID], Combatant: x.ID, total: roll.Total}
		r.commit(request{cmd: req.cmd}, w, roll.Roller, caller.Caller{Subject: roll.Roller.Subject, Origin: caller.OriginSystem})
		return
	}
}

// outOfCombatRoll takes a resolved roll that belongs to an Effect's save or an area spell.
func (r *runtime) outOfCombatRoll(id domain.RollID) bool {
	if i := slices.IndexFunc(r.st.fx.Saves, func(p domain.PendingSave) bool { return p.RollID == id }); i >= 0 {
		r.saveRolled(r.st.fx.Saves[i])
		return true
	}
	if r.st.cast != nil && slices.Contains(castRolls(r.st.cast), id) {
		r.areaRolled()
		return true
	}
	if z, ok := r.st.zoneRoll(id); ok {
		r.perceived(z, id)
		return true
	}
	if c, ok := r.st.pendingCheck(id); ok {
		r.checkRolled(c)
		return true
	}
	if character, ok := r.st.pendingHaggle(id); ok {
		r.haggled(character, id)
		return true
	}
	return false
}

// waitingRolls are the Perception and Encounter Check rolls still open outside Combat.
func (s *state) waitingRolls() []domain.RollID {
	var out []domain.RollID
	for _, z := range s.zones {
		for _, c := range z.Checks {
			if c.RollID != nil && c.Noticed == nil {
				out = append(out, *c.RollID)
			}
		}
	}
	for _, c := range s.checks {
		if c.Status == prep.CheckPending {
			out = append(out, domain.RollID(*c.RollID))
		}
	}
	return append(out, s.haggleRolls()...)
}

// haggleRolls are the haggling rolls still out at the open Shop.
func (s *state) haggleRolls() []domain.RollID {
	if s.shop == nil {
		return nil
	}
	var out []domain.RollID
	for _, h := range s.shop.Haggles {
		if h.RollID != nil && h.Adjust == nil {
			out = append(out, *h.RollID)
		}
	}
	return out
}

// catchUp takes in rolls resolved while the runtime was not running.
func (r *runtime) catchUp() {
	if r.st.cast != nil {
		r.areaRolled()
	}
	for _, id := range r.st.waitingRolls() {
		r.rolled(request{cmd: Command{rollID: id}})
	}
	if r.st.combat == nil {
		return
	}
	for _, x := range r.st.combat.Combatants {
		if x.Initiative == nil {
			r.rolled(request{cmd: Command{rollID: x.RollID}})
		}
	}
	if a := r.st.combat.Attack; a != nil {
		r.rolled(request{cmd: Command{rollID: a.RollID}})
	}
}

// applyCombat changes the Combat for one write and hands the result to the store.
func applyCombat(s *state, w *Write) {
	switch w.Kind {
	case domain.ActionCombatStarted:
		s.combat = w.Combat
		return
	case domain.ActionCombatEnded:
		s.combat = nil
		return
	}
	c := s.combat
	i := slices.IndexFunc(c.Combatants, func(x domain.Combatant) bool { return x.ID == w.Combatant })
	x := &c.Combatants[i]
	switch w.Kind {
	case domain.ActionInitiativeRolled:
		total := w.total
		x.Initiative = &total
	case domain.ActionTurnEnded:
		x.Done = true
	default:
		x.Economy, _ = x.Economy.Spend(w.resource)
	}
	settle(c)
	w.Combat = c
}

// settle starts the fight once everyone has rolled, and moves to the next initiative count once
// everyone on the current one has ended their turn.
func settle(c *domain.Combat) {
	totals := c.Totals()
	if c.Status == domain.CombatRolling {
		if len(totals) < len(c.Combatants) {
			return
		}
		c.Status, c.Round, c.Turn = domain.CombatActive, 1, combat.Counts(totals)[0]
		for i := range c.Combatants {
			c.Combatants[i].Economy.Reaction = true
		}
		startTurn(c)
		return
	}
	for range len(totals) + 1 {
		if slices.ContainsFunc(c.Combatants, c.Acting) {
			return
		}
		next, newRound := combat.Next(totals, c.Turn)
		if newRound {
			c.Round++
			for i := range c.Combatants {
				c.Combatants[i].Done = false
			}
		}
		c.Turn = next
		startTurn(c)
	}
}

func startTurn(c *domain.Combat) {
	for i, x := range c.Combatants {
		if c.Acting(x) {
			c.Combatants[i].Economy, c.Combatants[i].Shielded = combat.Fresh(x.SpeedFt), false
		}
	}
}

// dropCombatant takes a removed token out of the Combat.
func dropCombatant(s *state, w *Write) {
	if s.combat == nil {
		return
	}
	s.combat.Combatants = slices.DeleteFunc(s.combat.Combatants, func(x domain.Combatant) bool { return x.TokenID == w.Token.ID })
	if a := s.combat.Attack; a != nil && (a.Attacker == w.Token.ID || a.Target == w.Token.ID) {
		s.combat.Attack = nil
	}
	if p := s.combat.Prompt; p != nil && (p.Reactor == w.Token.ID || p.Trigger == w.Token.ID) {
		s.combat.Prompt = nil
	}
	if r := s.combat.Resume; r != nil && r.Token == w.Token.ID {
		s.combat.Resume = nil
	}
	settle(s.combat)
	w.Combat = s.combat
}

// projectCombat builds the initiative rail. The party and the Table only get Combatants they see.
func (s *state) projectCombat(v *View, a Audience, seen map[hex.Coord]bool) {
	c := s.combat
	if c == nil {
		return
	}
	totals := c.Totals()
	v.Combat = &CombatView{Status: c.Status, Round: c.Round, Combatants: []CombatantView{}}
	for _, x := range c.Combatants {
		t := s.tokens[x.TokenID]
		if a != AudienceDM && !s.shows(t, seen) {
			continue
		}
		cv := s.combatantView(x, t, totals, a)
		v.Combat.Combatants = append(v.Combat.Combatants, cv)
	}
	v.Combat.Attack, v.Combat.Prompt = s.pendingView(a, seen), s.promptView(a, seen, s.now())
	sort.SliceStable(v.Combat.Combatants, func(i, j int) bool {
		a, b := v.Combat.Combatants[i], v.Combat.Combatants[j]
		if (a.Rank == 0) != (b.Rank == 0) {
			return b.Rank == 0
		}
		if a.Rank != b.Rank {
			return a.Rank < b.Rank
		}
		return a.Label < b.Label
	})
}

func (s *state) combatantView(x domain.Combatant, t domain.Token, totals []int, a Audience) CombatantView {
	c := s.combat
	cv := CombatantView{
		ID: uuid.UUID(x.ID).String(), TokenID: uuid.UUID(t.ID).String(), Label: t.Label, Kind: t.Kind, RollID: uuid.UUID(x.RollID).String(),
		Initiative: x.Initiative, Acting: c.Acting(x), Done: x.Done, Action: x.Economy.Action, BonusAction: x.Economy.BonusAction,
		Reaction: x.Economy.Reaction, MovementFt: x.Economy.MovementFt, SpeedFt: x.SpeedFt, Surprised: x.Surprised,
	}
	if x.Initiative != nil {
		cv.Rank = combat.Rank(totals, *x.Initiative)
	}
	if t.Controller != nil {
		cv.ControllerID = t.Controller.String()
	}
	if a == AudienceDM {
		cv.Suggestion = s.suggest(x, t)
		if t.Stats != nil && t.Kind != domain.TokenParty {
			cv.Tactics = t.Tactics
		}
	}
	return cv
}

// applyAttack moves the attack on the table along, changes hit points and records who saw it.
func applyAttack(s *state, w *Write) {
	for _, o := range w.Observers {
		if s.observed[o] == nil {
			s.observed[o] = map[domain.TokenID]int{}
		}
		s.observed[o][w.Token.ID] += w.HP.Before - w.HP.After
	}
	if h := w.HP; h != nil {
		t := s.tokens[h.Token]
		stats := *t.Stats
		stats.HP = h.After
		t.Stats = &stats
		s.tokens[t.ID] = t
	}
	if s.combat == nil || w.Kind == domain.ActionDamageUndone {
		return
	}
	s.combat.Attack = w.attack
	if w.Kind == domain.ActionAttackDeclared {
		i := slices.IndexFunc(s.combat.Combatants, func(x domain.Combatant) bool { return x.ID == w.Combatant })
		s.combat.Combatants[i].Economy, _ = s.combat.Combatants[i].Economy.Spend(combat.Action)
	}
	w.Combat = s.combat
}

// pendingView shows the attack on the table to an audience that sees both creatures in it.
func (s *state) pendingView(a Audience, seen map[hex.Coord]bool) *PendingAttackView {
	p := s.combat.Attack
	if p == nil {
		return nil
	}
	from, to := s.tokens[p.Attacker], s.tokens[p.Target]
	if a != AudienceDM && (!s.shows(from, seen) || !s.shows(to, seen)) {
		return nil
	}
	return &PendingAttackView{
		AttackerID: uuid.UUID(p.Attacker).String(), TargetID: uuid.UUID(p.Target).String(), Name: from.Stats.Attacks[p.AttackNo].Name,
		Stage: p.Stage, RollID: uuid.UUID(p.RollID).String(), Critical: p.Critical,
	}
}
