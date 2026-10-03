package live

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/clock"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/features"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/inventory"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// rations is the item a Long Rest eats when the Campaign plays with camp supplies.
const rations = "rations"

// planRest proposes, agrees to, spends Hit Dice in, finishes or interrupts a rest.
func (r *runtime) planRest(m domain.Member, cmd Command) (Write, string) {
	rest := r.st.rest
	switch cmd.Kind {
	case CmdProposeRest:
		return r.proposeRest(m, cmd.Rest)
	case CmdAgreeRest:
		if rest == nil || rest.Status != domain.RestProposed {
			return Write{}, "There is nothing to agree to."
		}
		return r.agreeRest(m, rest)
	case CmdSpendHitDie:
		return r.spendHitDie(m, rest, cmd.TokenID)
	case CmdFinishRest:
		if rest == nil || rest.Status != domain.RestResting {
			return Write{}, "No rest is under way."
		}
		return r.finishRest(rest)
	default:
		if rest == nil {
			return Write{}, "No rest is under way."
		}
		return Write{Kind: domain.ActionRestInterrupted, RestOver: true}, ""
	}
}

func (r *runtime) proposeRest(m domain.Member, kind string) (Write, string) {
	switch {
	case kind != RestShort && kind != RestLong:
		return Write{}, "A rest is short or long."
	case r.st.combat != nil:
		return Write{}, "Nobody rests in the middle of a fight."
	case r.st.rest != nil:
		return Write{}, "A rest is already proposed."
	}
	resters, err := r.resters()
	if err != nil {
		r.log.Error("live: rest info", "error", err)
		return Write{}, "The party could not be read."
	}
	if len(resters) == 0 {
		return Write{}, "No Character is on the board to rest."
	}
	if kind == RestLong {
		if reason := r.suppliesFor(resters); reason != "" {
			return Write{}, reason
		}
	}
	rest := &domain.Rest{Kind: kind, Status: domain.RestProposed, ProposedBy: m.ID, Agreed: []uuid.UUID{m.ID}, Resters: resters}
	return r.agreed(domain.ActionRestProposed, rest, m), ""
}

func (r *runtime) agreeRest(m domain.Member, rest *domain.Rest) (Write, string) {
	if slices.Contains(rest.Agreed, m.ID) {
		return Write{}, "You already agreed."
	}
	if !m.DM && !slices.Contains(r.st.waitingOn(rest), m.ID) {
		return Write{}, "None of your Characters is resting."
	}
	next := rest.Clone()
	next.Agreed = append(next.Agreed, m.ID)
	return r.agreed(domain.ActionRestAgreed, next, m), ""
}

// agreed starts the rest once the DM and every Player resting a Character have agreed; a Long Rest
// with camp supplies eats its Rations then.
func (r *runtime) agreed(kind string, rest *domain.Rest, m domain.Member) Write {
	rest.DMAgreed = rest.DMAgreed || m.DM
	if !rest.DMAgreed || len(r.st.waitingOn(rest)) > 0 {
		return Write{Kind: kind, Resting: rest}
	}
	rest.Status = domain.RestResting
	w := Write{Kind: domain.ActionRestStarted, Resting: rest}
	if rest.Kind == RestLong && r.supplies() {
		w.Supplies = r.st.eat(rest.Resters)
	}
	return w
}

// waitingOn are the Players resting a Character who have not agreed yet.
func (s *state) waitingOn(rest *domain.Rest) []uuid.UUID {
	var out []uuid.UUID
	for _, x := range rest.Resters {
		t, ok := s.tokens[x.TokenID]
		if !ok || t.Controller == nil || slices.Contains(rest.Agreed, *t.Controller) || slices.Contains(out, *t.Controller) {
			continue
		}
		out = append(out, *t.Controller)
	}
	return out
}

// resters are the Characters on the board, with what the rest reads of them.
func (r *runtime) resters() ([]domain.Rester, error) {
	byCharacter := map[uuid.UUID]domain.TokenID{}
	var ids []uuid.UUID
	for _, t := range r.st.tokens {
		if t.Stats == nil || !strings.HasPrefix(t.Stats.Source, "character:") {
			continue
		}
		id, err := uuid.Parse(strings.TrimPrefix(t.Stats.Source, "character:"))
		if err != nil {
			continue
		}
		if _, seen := byCharacter[id]; !seen {
			ids = append(ids, id)
		}
		byCharacter[id] = t.ID
	}
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	out, err := r.store.RestInfo(context.Background(), r.campaign, ids)
	for i := range out {
		out[i].TokenID = byCharacter[out[i].CharacterID]
	}
	return out, err
}

func (r *runtime) supplies() bool {
	on, err := r.store.RestSupplies(context.Background(), r.campaign)
	if err != nil {
		r.log.Error("live: rest supplies", "error", err)
	}
	return on
}

// suppliesFor refuses a Long Rest the party cannot eat for, when the Campaign plays with camp supplies.
func (r *runtime) suppliesFor(resters []domain.Rester) string {
	if !r.supplies() {
		return ""
	}
	have := 0
	for _, c := range r.st.inventory.Containers {
		if c.Kind == domain.ContainerStash || r.st.carriedBy(c, resters) {
			have += c.Items[rations]
		}
	}
	if have >= len(resters) {
		return ""
	}
	var names []string
	for _, x := range resters {
		names = append(names, x.Name)
	}
	return "A Long Rest needs a day of Rations for each of " + strings.Join(names, ", ") + "."
}

func (s *state) carriedBy(c domain.Container, resters []domain.Rester) bool {
	return c.CharacterID != nil && slices.ContainsFunc(resters, func(x domain.Rester) bool { return x.CharacterID == *c.CharacterID })
}

// eat takes a day of Rations for each rester, from their own pack while it has some, then the Party Stash.
func (s *state) eat(resters []domain.Rester) []domain.Supply {
	left := map[domain.ContainerID]int{}
	stash := domain.ContainerID{}
	for _, c := range s.inventory.Containers {
		left[c.ID] = c.Items[rations]
		if c.Kind == domain.ContainerStash {
			stash = c.ID
		}
	}
	var order []domain.ContainerID
	for _, x := range resters {
		from := stash
		for _, c := range s.inventory.Containers {
			if c.CharacterID != nil && *c.CharacterID == x.CharacterID && left[c.ID] > 0 {
				from = c.ID
			}
		}
		left[from]--
		if !slices.Contains(order, from) {
			order = append(order, from)
		}
	}
	out := make([]domain.Supply, 0, len(order))
	for _, id := range order {
		out = append(out, domain.Supply{Container: id, Item: rations, Left: left[id]})
	}
	return out
}

// spendHitDie opens a Hit Die roll for a resting Character during a Short Rest.
func (r *runtime) spendHitDie(m domain.Member, rest *domain.Rest, tokenID string) (Write, string) {
	if rest == nil || rest.Status != domain.RestResting || rest.Kind != RestShort {
		return Write{}, "Hit Dice are spent once the rest has begun, in a Short Rest."
	}
	t, ok := r.st.tokenByID(tokenID)
	i := slices.IndexFunc(rest.Resters, func(x domain.Rester) bool { return x.TokenID == t.ID })
	switch {
	case !ok || i < 0:
		return Write{}, "That token is not resting."
	case !m.DM && (t.Controller == nil || *t.Controller != m.ID):
		return Write{}, "That Character is not yours."
	case rest.Resters[i].RollID != nil:
		return Write{}, "Their last Hit Die is still rolling."
	case rest.Resters[i].HitDiceLeft == 0:
		return Write{}, "No Hit Dice left."
	}
	x := rest.Resters[i]
	con := rules.Modifier(x.Abilities["constitution"])
	roll := r.request(m, t, x.Name+" spends a Hit Die", "1d"+strconv.Itoa(x.HitDie), domain.Modifier{Label: "Constitution", Value: con})
	next := rest.Clone()
	next.Resters[i].HitDiceLeft--
	next.Resters[i].RollID = &roll.ID
	return Write{Kind: domain.ActionHitDieSpent, Token: t, Rolls: []domain.Roll{roll}, Resting: next}, ""
}

// pendingHitDie finds the rester whose Hit Die roll this is.
func (s *state) pendingHitDie(id domain.RollID) (int, bool) {
	if s.rest == nil {
		return 0, false
	}
	i := slices.IndexFunc(s.rest.Resters, func(x domain.Rester) bool { return x.RollID != nil && *x.RollID == id })
	return i, i >= 0
}

// hitDieRolled heals the rester by their Hit Die roll, never below one point and never past their maximum.
func (r *runtime) hitDieRolled(i int, id domain.RollID) {
	roll, err := r.store.Roll(context.Background(), r.campaign, id)
	if err != nil || roll.Status != domain.StatusResolved {
		return
	}
	next := r.st.rest.Clone()
	x := &next.Resters[i]
	x.RollID = nil
	t := r.st.tokens[x.TokenID]
	after := min(t.Stats.HPMax, t.Stats.HP+max(1, roll.Total))
	w := Write{Kind: domain.ActionHitDieHealed, Token: t, HP: &HPChange{Token: t.ID, Before: t.Stats.HP, After: after}, Resting: next}
	r.commit(request{}, w, roll.Roller, caller.Caller{Subject: roll.Roller.Subject, Origin: caller.OriginSystem})
}

// finishRest ends a rest with its benefits: a Short Rest gives back what recharges on one; a Long Rest
// heals everyone, gives back every Hit Die and Resource and unlocks a level-up. Either moves the Game
// Clock on: an hour, or eight.
func (r *runtime) finishRest(rest *domain.Rest) (Write, string) {
	if slices.ContainsFunc(rest.Resters, func(x domain.Rester) bool { return x.RollID != nil }) {
		return Write{}, "Wait for the Hit Dice still rolling."
	}
	cat, err := r.store.Features(context.Background())
	if err != nil {
		r.log.Error("live: features", "error", err)
		return Write{}, "The rest could not be finished."
	}
	w := Write{Kind: domain.ActionRestTaken, Rest: rest.Kind, RestOver: true}
	event, took := features.ShortRest, clock.ShortRest
	if rest.Kind == RestLong {
		event, took = features.LongRest, clock.LongRest
	}
	r.pass(&w, r.st.gameTime().Add(took))
	for _, x := range rest.Resters {
		t := r.st.tokens[x.TokenID]
		res := domain.RestResult{CharacterID: x.CharacterID, HPCurrent: t.Stats.HP, HitDiceSpent: x.Level - x.HitDiceLeft, Used: map[string]int{}}
		if rest.Kind == RestLong {
			res.HPCurrent, res.HitDiceSpent, res.LevelUpReady = t.Stats.HPMax, 0, true
			w.Healed = append(w.Healed, HPChange{Token: t.ID, Before: t.Stats.HP, After: t.Stats.HPMax})
		}
		stats := features.Stats{Level: x.Level, AbilityMod: 0, Proficiency: rules.ProficiencyBonus(x.Level)}
		for slug, used := range x.Used {
			resource, known := cat.Resources[slug]
			if !known {
				res.Used[slug] = used
				continue
			}
			stats.AbilityMod = rules.Modifier(x.Abilities[resource.Ability])
			most := resource.Max(stats)
			res.Used[slug] = max(0, most-resource.Regain(event, stats, 0, most-used))
		}
		w.Results = append(w.Results, res)
		w.Recharged = append(w.Recharged, r.recharge(x.CharacterID, rest.Kind)...)
	}
	return w, ""
}

// recharge rolls what a rest gives back to the charged magic items a Character carries. What comes back
// at dawn waits for the Game Clock to pass one.
func (r *runtime) recharge(character uuid.UUID, kind string) []domain.Recharge {
	when := inventory.ShortRest
	if kind == RestLong {
		when = inventory.LongRest
	}
	var out []domain.Recharge
	for _, c := range r.st.inventory.Containers {
		if c.CharacterID == nil || *c.CharacterID != character {
			continue
		}
		for _, in := range c.Instances {
			if after, changed := r.regain(in, when); changed {
				out = append(out, domain.Recharge{Container: c.ID, Instance: in.ID, Charges: after})
			}
		}
	}
	return out
}

// regain is the charges an Item Instance holds after a rest, and whether that is news: it changed, or
// was never counted before.
func (r *runtime) regain(in domain.Instance, when inventory.Rest) (int, bool) {
	info := r.st.itemInfo(in.Slug)
	if info.MaxCharges == 0 {
		return 0, false
	}
	current := info.MaxCharges
	if in.Charges != nil {
		current = *in.Charges
	}
	src, rolled := r.source(r.seed()), 0
	for range info.RegainDice {
		rolled += dice.Face(src, info.RegainFaces)
	}
	schedule := inventory.Charges{Max: info.MaxCharges, Dice: info.RegainDice, Faces: info.RegainFaces, Bonus: info.RegainBonus, On: info.RechargeOn}
	after := schedule.Regain(when, current, rolled)
	return after, after != current || in.Charges == nil
}

// applyRest keeps the rest a write leaves, heals a Long Rest's resters, and eats its Rations.
func applyRest(s *state, w *Write) {
	switch {
	case w.RestOver:
		s.rest = nil
	case w.Resting != nil:
		s.rest = w.Resting
	}
	healed := w.Healed
	if w.Kind == domain.ActionHitDieHealed {
		healed = append(healed, *w.HP)
	}
	for _, h := range healed {
		t := s.tokens[h.Token]
		stats := *t.Stats
		stats.HP = h.After
		t.Stats = &stats
		s.tokens[t.ID] = t
	}
	for _, rc := range w.Recharged {
		s.setCharges(rc)
	}
	for _, sup := range w.Supplies {
		for i, c := range s.inventory.Containers {
			if c.ID != sup.Container {
				continue
			}
			s.inventory.Containers[i].Items[sup.Item] = sup.Left
			if sup.Left == 0 {
				delete(s.inventory.Containers[i].Items, sup.Item)
			}
		}
	}
}

// setCharges keeps the charges a rest gave back to an Item Instance.
func (s *state) setCharges(rc domain.Recharge) {
	for i, c := range s.inventory.Containers {
		if c.ID != rc.Container {
			continue
		}
		for j, in := range c.Instances {
			if in.ID == rc.Instance {
				n := rc.Charges
				s.inventory.Containers[i].Instances[j].Charges = &n
			}
		}
	}
}

// restView shows the rest to everyone at the table but the Table Display.
func (s *state) restView(a Audience) *RestView {
	rest := s.rest
	if rest == nil || a == AudienceTable {
		return nil
	}
	v := &RestView{
		Kind: rest.Kind, Status: rest.Status, ProposedBy: rest.ProposedBy.String(), Agreed: []string{}, Waiting: []string{},
		WaitingOnDM: !rest.DMAgreed, Resters: []ResterView{},
	}
	for _, id := range rest.Agreed {
		v.Agreed = append(v.Agreed, id.String())
	}
	for _, id := range s.waitingOn(rest) {
		v.Waiting = append(v.Waiting, id.String())
	}
	for _, x := range rest.Resters {
		rv := ResterView{CharacterID: x.CharacterID.String(), TokenID: uuid.UUID(x.TokenID).String(), Name: x.Name, HitDie: "d" + strconv.Itoa(x.HitDie), HitDiceLeft: x.HitDiceLeft}
		if x.RollID != nil {
			rv.RollID = uuid.UUID(*x.RollID).String()
		}
		v.Resters = append(v.Resters, rv)
	}
	return v
}
