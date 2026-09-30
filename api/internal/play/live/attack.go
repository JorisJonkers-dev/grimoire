package live

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/attack"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// HPChange is a token's hit points before and after a write; Undoes names the damage it reverts.
type HPChange struct {
	Token  domain.TokenID
	Before int
	After  int
	Undoes uuid.UUID
}

// aim is an attack worked out against the rules, before anything is rolled.
type aim struct {
	attacker, target domain.Token
	no               int
	with             domain.Attack
	mode             attack.Mode
	cover            int
	reasons          []string
}

// aimAt checks an attack a member asks for: in combat, on the attacker's turn with its action left,
// against a target the member can see, in range and in sight. It collects every reason that shapes it.
func (r *runtime) aimAt(m domain.Member, cmd Command) (aim, string) {
	c := r.st.combat
	if c == nil || c.Status != domain.CombatActive {
		return aim{}, "Attacks happen in combat, once initiative is rolled."
	}
	if c.Attack != nil {
		return aim{}, "Finish the attack on the table first."
	}
	seen := r.st.vision()
	find := func(id string) (domain.Token, bool) {
		u, _ := uuid.Parse(id)
		t, ok := r.st.tokens[domain.TokenID(u)]
		return t, ok && t.Stats != nil && (m.DM || r.st.shows(t, seen))
	}
	a, ok := find(cmd.TokenID)
	switch {
	case !ok || cmd.AttackNo < 0 || cmd.AttackNo >= len(a.Stats.Attacks):
		return aim{}, "No such attack."
	case !m.DM && (a.Controller == nil || *a.Controller != m.ID):
		return aim{}, "That token is not yours to play."
	}
	x, acting := r.st.combatantOf(a.ID)
	switch {
	case !acting:
		return aim{}, "It is not " + a.Label + "'s turn."
	case !x.Economy.Action:
		return aim{}, a.Label + " has already used their action."
	}
	t, ok := find(cmd.TargetID)
	if !ok || t.ID == a.ID {
		return aim{}, "Choose a creature to attack."
	}
	return r.st.shape(aim{attacker: a, target: t, no: cmd.AttackNo, with: a.Stats.Attacks[cmd.AttackNo]})
}

// combatantOf finds a token's Combatant and whether it is acting now.
func (s *state) combatantOf(id domain.TokenID) (domain.Combatant, bool) {
	for _, x := range s.combat.Combatants {
		if x.TokenID == id {
			return x, s.combat.Acting(x)
		}
	}
	return domain.Combatant{}, false
}

// shape applies range, sight, cover and the advantage sources to an attack.
func (s *state) shape(p aim) (aim, string) {
	from, to := hex.Coord{Q: p.attacker.Q, R: p.attacker.R}, hex.Coord{Q: p.target.Q, R: p.target.R}
	band := attack.BandOf(hex.Distance(from, to)*hex.FeetPerHex, p.with.ReachFt, p.with.RangeFt, p.with.LongRangeFt)
	if band == attack.OutOfRange {
		return aim{}, "The target is out of range."
	}
	g := hex.Grid{Cells: map[hex.Coord]hex.Cell{}, Occupants: map[hex.Coord]hex.Occupant{}}
	for _, c := range s.ground() {
		wall := s.board != nil && s.board.Walls[c]
		g.Cells[c] = hex.Cell{Blocked: wall, BlocksSight: wall}
	}
	for _, t := range s.tokens {
		if t.ID != p.attacker.ID && t.ID != p.target.ID && standing(t) {
			g.Occupants[hex.Coord{Q: t.Q, R: t.R}] = hex.Enemy
		}
	}
	sight := hex.LineOfSight(g, from, to)
	if !sight.Visible {
		return aim{}, "There is no clear line to the target."
	}
	p.reasons = append(p.reasons, fmt.Sprintf("%s: %+d to hit", p.with.Name, p.with.ToHit))
	if p.cover = sight.Cover.ACBonus(); p.cover > 0 {
		p.reasons = append(p.reasons, fmt.Sprintf("Cover: +%d to the target's AC", p.cover))
	}
	disadvantages := 0
	if band == attack.LongRange {
		disadvantages++
		p.reasons = append(p.reasons, "Disadvantage: long range")
	}
	if band != attack.InReach && s.hostileNextTo(p.attacker) {
		disadvantages++
		p.reasons = append(p.reasons, "Disadvantage: a hostile creature is next to the attacker")
	}
	p.mode = attack.ModeOf(0, disadvantages)
	return p, ""
}

// standing is a visible creature still on its feet; only those give cover or threaten a ranged attacker.
func standing(t domain.Token) bool {
	return !t.Hidden && t.Stats != nil && t.Stats.HP > 0
}

// hostileNextTo reports a standing creature of the other side within 5 feet.
func (s *state) hostileNextTo(a domain.Token) bool {
	for _, t := range s.tokens {
		near := hex.Distance(hex.Coord{Q: a.Q, R: a.R}, hex.Coord{Q: t.Q, R: t.R}) == 1
		if near && standing(t) && (t.Kind == domain.TokenParty) != (a.Kind == domain.TokenParty) {
			return true
		}
	}
	return false
}

func (r *runtime) previewAttack(req request) {
	p, reason := r.aimAt(req.from.Member, req.cmd)
	if reason != "" {
		r.reject(req, reason)
		return
	}
	spec, _ := dice.Parse(p.with.Damage)
	least, most := attack.DamageRange(spec, p.with.DamageBonus)
	_, critMost := attack.DamageRange(attack.CriticalDice(spec), p.with.DamageBonus)
	r.send(req.from, Update{Kind: UpdAttackPreview, Seq: r.st.session.Seq, Nonce: req.cmd.Nonce, Preview: &AttackPreview{
		TokenID: req.cmd.TokenID, TargetID: req.cmd.TargetID, AttackNo: p.no, Name: p.with.Name,
		HitChance: attack.HitChance(p.with.ToHit, p.target.Stats.AC+p.cover, p.mode), Mode: p.mode.String(),
		DamageMin: least, DamageMax: most, CritMax: critMost, Reasons: p.reasons,
	}})
}

// planAttack spends the attacker's action and opens the attack Roll Request.
func (r *runtime) planAttack(m domain.Member, cmd Command) (Write, string) {
	p, reason := r.aimAt(m, cmd)
	if reason != "" {
		return Write{}, reason
	}
	roll := r.request(m, p.attacker, p.with.Name+" attack against "+p.target.Label, attack.D20(p.mode), domain.Modifier{Label: p.with.Name, Value: p.with.ToHit})
	x, _ := r.st.combatantOf(p.attacker.ID)
	pending := &domain.PendingAttack{
		ID: uuid.New(), Attacker: p.attacker.ID, Target: p.target.ID, AttackNo: p.no, Mode: p.mode, CoverBonus: p.cover,
		Stage: domain.StageToHit, RollID: roll.ID,
	}
	return Write{Kind: domain.ActionAttackDeclared, Token: p.attacker, Combatant: x.ID, Rolls: []domain.Roll{roll}, attack: pending}, ""
}

// request opens a Roll Request for a token: its Controller rolls it, or the DM.
func (r *runtime) request(dm domain.Member, t domain.Token, purpose, notation string, mods ...domain.Modifier) domain.Roll {
	roller := dm
	if t.Controller != nil {
		if m, err := r.members.Member(context.Background(), r.st.session.CampaignID, *t.Controller); err == nil {
			roller = m
		}
	}
	spec, _ := dice.Parse(notation)
	roll := domain.Roll{
		ID: domain.RollID(uuid.New()), CampaignID: r.st.session.CampaignID, Purpose: purpose, Notation: notation,
		RequestedBy: dm.Name, Roller: roller, Status: domain.StatusPending,
	}
	for g, group := range spec.Groups {
		for range group.Count {
			roll.Dice = append(roll.Dice, domain.Die{No: len(roll.Dice), Group: g, Faces: group.Faces})
		}
	}
	for _, m := range mods {
		if m.Value != 0 {
			roll.Modifiers = append(roll.Modifiers, m)
		}
	}
	return roll
}

// attackRolled moves the attack on once its roll resolves: a miss ends it, a hit opens the damage
// roll (every die doubled on a critical), and the damage roll takes hit points off the target.
func (r *runtime) attackRolled(roll domain.Roll) {
	p := r.st.combat.Attack
	a, t := r.st.tokens[p.Attacker], r.st.tokens[p.Target]
	with := a.Stats.Attacks[p.AttackNo]
	sys := caller.Caller{Subject: roll.Roller.Subject, Origin: caller.OriginSystem}
	next := *p
	w := Write{Token: a, attack: &next}
	if p.Stage == domain.StageDamage {
		r.commit(request{}, r.hurt(t, roll.Total, w), roll.Roller, sys)
		return
	}
	result := attack.Outcome(natural(roll), with.ToHit, t.Stats.AC+p.CoverBonus)
	spec, _ := dice.Parse(with.Damage)
	switch {
	case result == attack.Miss:
		w.Kind, w.attack = domain.ActionAttackMissed, nil
	case len(spec.Groups) == 0:
		w = r.hurt(t, with.DamageBonus, w)
	default:
		if result == attack.Critical {
			spec, next.Critical = attack.CriticalDice(spec), true
		}
		purpose := with.Name + " damage to " + t.Label
		if next.Critical {
			purpose += " (critical)"
		}
		dmg := r.request(roll.Roller, a, purpose, spec.String(), domain.Modifier{Label: with.Name, Value: with.DamageBonus})
		dmg.Roller, dmg.RequestedBy = roll.Roller, roll.RequestedBy
		next.Stage, next.RollID = domain.StageDamage, dmg.ID
		w.Kind, w.Rolls = domain.ActionAttackHit, []domain.Roll{dmg}
	}
	r.commit(request{}, w, roll.Roller, sys)
}

// hurt takes damage off a target's hit points and ends the attack.
func (r *runtime) hurt(t domain.Token, amount int, w Write) Write {
	before := t.Stats.HP
	w.Kind, w.attack = domain.ActionDamageDealt, nil
	w.HP = &HPChange{Token: t.ID, Before: before, After: max(before-max(amount, 0), 0)}
	return w
}

// natural is the d20 face that counts: the higher with advantage, the lower with disadvantage.
func natural(roll domain.Roll) int {
	spec, _ := dice.Parse(roll.Notation)
	faces := make([]int, 0, len(roll.Dice))
	for _, d := range roll.Dice {
		faces = append(faces, d.Value)
	}
	return faces[slices.Index(spec.Groups[0].Kept(faces), true)]
}

// planUndo reverts the Session's most recent damage that is not undone yet.
func (r *runtime) planUndo() (Write, string) {
	last, ok, err := r.store.LastDamage(context.Background(), r.st.session.ID)
	t, exists := r.st.tokens[last.Token]
	if err != nil || !ok || !exists || t.Stats == nil {
		return Write{}, "There is no damage to undo."
	}
	healed := min(t.Stats.HP+last.Before-last.After, t.Stats.HPMax)
	return Write{Kind: domain.ActionDamageUndone, Token: t, HP: &HPChange{Token: t.ID, Before: t.Stats.HP, After: healed, Undoes: last.Undoes}}, ""
}
