package live

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/attack"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// HPChange is a token's hit points before and after a write; Undoes names the damage it reverts.
type HPChange struct {
	Token  domain.TokenID
	Before int
	After  int
	Undoes uuid.UUID
	// Raw is the damage dealt before hit points stopped it at 0, negative for healing; Critical marks
	// damage from a Critical Hit.
	Raw      int
	Critical bool
}

// aim is an attack worked out against the rules, before anything is rolled.
type aim struct {
	attacker, target domain.Token
	no               int
	with             domain.Attack
	mode             attack.Mode
	cover            int
	ranged           bool
	reasons          []string
	prof             effects.AttackProfile
	// bonus is added to the attack roll, from high ground.
	bonus int
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
	if reason := r.st.attackBlocked(a, cmd); reason != "" {
		return aim{}, reason
	}
	t, ok := find(cmd.TargetID)
	if !ok || t.ID == a.ID {
		return aim{}, "Choose a creature to attack."
	}
	if x, _ := r.st.combatantOf(a.ID); cmd.Cleave && !r.st.cleavable(x, t) {
		return aim{}, "Cleave strikes a second creature within 5 feet of the first."
	}
	p, reason := r.st.shape(aim{attacker: a, target: t, no: cmd.AttackNo, with: a.Stats.Attacks[cmd.AttackNo]})
	if reason == "" {
		r.highGround(&p)
	}
	return p, reason
}

// highGround gives +2 to hit from higher ground when the Campaign uses that optional rule.
func (r *runtime) highGround(p *aim) {
	if r.st.height(p.attacker) <= r.st.height(p.target) {
		return
	}
	if on, err := r.store.HighGround(context.Background(), r.st.session.CampaignID); err == nil && on {
		p.bonus, p.reasons = 2, append(p.reasons, "High ground: +2 to hit")
	}
}

// height is the elevation of the hex a token stands on.
func (s *state) height(t domain.Token) int {
	if s.board == nil {
		return 0
	}
	return s.board.Elevation[hex.Coord{Q: t.Q, R: t.R}]
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

// attackBlocked says why a token cannot make this attack now: not its turn, Incapacitated, or no attack
// of the Attack action (or off-hand attack) left.
func (s *state) attackBlocked(a domain.Token, cmd Command) string {
	x, acting := s.combatantOf(a.ID)
	switch {
	case !acting:
		return "It is not " + a.Label + "'s turn."
	case s.catalog.Incapacitated(s.actives(a.ID)):
		return a.Label + " can't act while Incapacitated."
	case cmd.Cleave && (x.CleaveFrom == nil || x.Cleaved):
		return a.Label + " has no Cleave attack open: it follows a Cleave hit, once a turn."
	case cmd.Cleave:
		return ""
	case cmd.OffHand && !a.Stats.Attacks[cmd.AttackNo].Light:
		return "Only a Light weapon makes the off-hand attack."
	case cmd.OffHand && !x.Economy.CanOffHand(nicks(a.Stats.Attacks[cmd.AttackNo])):
		return a.Label + " has no off-hand attack left: it follows an attack with a Light weapon, for a Bonus Action."
	case !cmd.OffHand && !x.Economy.CanAttack():
		return a.Label + " has no attacks left this turn."
	}
	return ""
}

// shape applies range, sight, cover and the advantage sources to an attack.
func (s *state) shape(p aim) (aim, string) {
	from, to := hex.Coord{Q: p.attacker.Q, R: p.attacker.R}, hex.Coord{Q: p.target.Q, R: p.target.R}
	band := attack.BandOf(hex.Distance(from, to)*hex.FeetPerHex, p.with.ReachFt, p.with.RangeFt, p.with.LongRangeFt)
	if band == attack.OutOfRange {
		return aim{}, "The target is out of range."
	}
	p.ranged = band != attack.InReach
	g := hex.Grid{Cells: map[hex.Coord]hex.Cell{}, Occupants: map[hex.Coord]hex.Occupant{}}
	for _, c := range s.ground() {
		g.Cells[c] = s.cell(c)
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
	if s.armor(p.target) > p.target.Stats.AC {
		p.reasons = append(p.reasons, "Shield: +5 to the target's AC")
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
	p.prof = s.catalog.ForAttack(s.actives(p.attacker.ID), s.actives(p.target.ID), uuid.UUID(p.attacker.ID).String(), hex.Distance(from, to) <= 1)
	p.reasons = append(append(append(p.reasons, p.prof.Advantages...), p.prof.Disadvantages...), p.prof.Notes...)
	p.mode = attack.ModeOf(len(p.prof.Advantages), disadvantages+len(p.prof.Disadvantages))
	return p, ""
}

// standing is a visible creature still on its feet; only those give cover or threaten a ranged attacker.
func standing(t domain.Token) bool {
	return !t.Hidden && t.Stats != nil && t.Stats.HP > 0
}

// hostileNextTo reports a standing creature of the other side within 5 feet that is not Incapacitated.
func (s *state) hostileNextTo(a domain.Token) bool {
	for _, t := range s.tokens {
		near := hex.Distance(hex.Coord{Q: a.Q, R: a.R}, hex.Coord{Q: t.Q, R: t.R}) == 1
		if near && standing(t) && (t.Kind == domain.TokenParty) != (a.Kind == domain.TokenParty) && !s.catalog.Incapacitated(s.actives(t.ID)) {
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
	spec := joinDice(p.with.Damage, p.prof.DamageDice)
	least, most := attack.DamageRange(spec, p.with.DamageBonus)
	_, critMost := attack.DamageRange(attack.CriticalDice(spec), p.with.DamageBonus)
	r.send(req.from, Update{Kind: UpdAttackPreview, Seq: r.st.session.Seq, Nonce: req.cmd.Nonce, Preview: &AttackPreview{
		TokenID: req.cmd.TokenID, TargetID: req.cmd.TargetID, AttackNo: p.no, Name: p.with.Name,
		HitChance: attack.HitChanceDice(p.with.ToHit+p.bonus-p.prof.Penalty, joinDice("", p.prof.AttackDice), r.st.armor(p.target)+p.cover, p.mode), Mode: p.mode.String(),
		DamageMin: least, DamageMax: most, CritMax: critMost, Reasons: p.reasons,
	}})
}

// planAttack spends the attacker's action and opens the attack Roll Request.
func (r *runtime) planAttack(m domain.Member, cmd Command) (Write, string) {
	p, reason := r.aimAt(m, cmd)
	if reason != "" {
		return Write{}, reason
	}
	notation := strings.Join(append([]string{attack.D20(p.mode)}, p.prof.AttackDice...), "+")
	roll := r.request(m, p.attacker, p.with.Name+" attack against "+p.target.Label, notation, domain.Modifier{Label: p.with.Name, Value: p.with.ToHit},
		domain.Modifier{Label: "High ground", Value: p.bonus}, domain.Modifier{Label: "Exhaustion", Value: -p.prof.Penalty})
	x, _ := r.st.combatantOf(p.attacker.ID)
	pending := &domain.PendingAttack{
		ID: uuid.New(), Attacker: p.attacker.ID, Target: p.target.ID, AttackNo: p.no, Mode: p.mode, CoverBonus: p.cover,
		Stage: domain.StageToHit, RollID: roll.ID, Ranged: p.ranged, OffHand: cmd.OffHand, Cleave: cmd.Cleave,
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
	p := *r.st.combat.Attack
	a, t := r.st.tokens[p.Attacker], r.st.tokens[p.Target]
	sys := caller.Caller{Subject: roll.Roller.Subject, Origin: caller.OriginSystem, Client: ""}
	if p.Stage == domain.StageDamage {
		w := r.hurt(t, roll.Total, Write{Token: a, attack: &p})
		r.commit(request{}, w, roll.Roller, sys)
		r.masteryAfterHit(a, t, p, w.HP.Before-w.HP.After, roll.Roller, sys)
		r.reactToDamage(t, a, w.HP.Before-w.HP.After, roll.Roller, sys)
		return
	}
	result := attack.Outcome(natural(roll), roll.Total-natural(roll), r.st.armor(t)+p.CoverBonus)
	if result == attack.Miss {
		r.commit(request{}, Write{Kind: domain.ActionAttackMissed, Token: a}, roll.Roller, sys)
		r.graze(a, t, p, roll.Roller, sys)
		return
	}
	if result == attack.Hit && r.st.closeCrit(a, t) {
		result = attack.Critical
	}
	if pr := r.shieldPrompt(a, t, p, roll.Total); pr != nil && result == attack.Hit {
		p.Stage, p.Total = domain.StageReaction, roll.Total
		r.commit(request{}, Write{Kind: domain.ActionReactionOffered, Token: t, attack: &p, prompt: pr}, roll.Roller, sys)
		return
	}
	w := r.hit(a, t, p, result == attack.Critical, roll)
	r.commit(request{}, w, roll.Roller, sys)
	if w.Kind == domain.ActionDamageDealt {
		r.masteryAfterHit(a, t, p, w.HP.Before-w.HP.After, roll.Roller, sys)
		r.reactToDamage(t, a, w.HP.Before-w.HP.After, roll.Roller, sys)
	}
}

// closeCrit reports whether the target's effects turn a hit from where the attacker stands into a
// Critical Hit (paralysed or unconscious, struck from within 5 feet).
func (s *state) closeCrit(a, t domain.Token) bool {
	near := hex.Distance(hex.Coord{Q: a.Q, R: a.R}, hex.Coord{Q: t.Q, R: t.R}) <= 1
	return s.catalog.ForAttack(nil, s.actives(t.ID), "", near).Crit
}

// hit opens the damage roll of an attack that hit, every die doubled on a critical; flat damage lands at once.
func (r *runtime) hit(a, t domain.Token, p domain.PendingAttack, critical bool, roll domain.Roll) Write {
	with := a.Stats.Attacks[p.AttackNo]
	spec := joinDice(with.Damage, r.st.catalog.ForAttack(nil, r.st.actives(t.ID), uuid.UUID(a.ID).String(), false).DamageDice)
	bonus := with.DamageBonus
	if (p.OffHand || p.Cleave) && with.DamageMod > 0 {
		bonus -= with.DamageMod
	}
	if len(spec.Groups) == 0 {
		return r.hurt(t, bonus, Write{Token: a, attack: &p})
	}
	purpose := with.Name + " damage to " + t.Label
	if critical {
		spec, purpose = attack.CriticalDice(spec), purpose+" (critical)"
	}
	dmg := r.request(roll.Roller, a, purpose, spec.String(), domain.Modifier{Label: with.Name, Value: bonus})
	dmg.Roller, dmg.RequestedBy = roll.Roller, roll.RequestedBy
	p.Stage, p.RollID, p.Critical = domain.StageDamage, dmg.ID, critical
	return Write{Kind: domain.ActionAttackHit, Token: a, attack: &p, Rolls: []domain.Roll{dmg}}
}

// hurt takes damage off a target's hit points and ends the attack. Creatures that see a ranged attacker
// deal damage remember it.
func (r *runtime) hurt(t domain.Token, amount int, w Write) Write {
	before := t.Stats.HP
	ranged, critical := w.attack.Ranged, w.attack.Critical
	w.Kind, w.attack = domain.ActionDamageDealt, nil
	w.HP = &HPChange{Token: t.ID, Before: before, After: max(before-max(amount, 0), 0), Raw: max(amount, 0), Critical: critical}
	if ranged && w.HP.After < before {
		w.Observers = r.st.witnesses(w.Token)
	}
	return w
}

// witnesses are the standing creatures on the other side from a token that have a clear line to it.
func (s *state) witnesses(t domain.Token) []domain.TokenID {
	g := s.sightGrid()
	var out []domain.TokenID
	for _, o := range s.tokens {
		if standing(o) && (o.Kind == domain.TokenParty) != (t.Kind == domain.TokenParty) &&
			hex.LineOfSight(g, hex.Coord{Q: o.Q, R: o.R}, hex.Coord{Q: t.Q, R: t.R}).Visible {
			out = append(out, o.ID)
		}
	}
	return out
}

// sightGrid is the board with its walls, for creatures looking at each other.
func (s *state) sightGrid() hex.Grid {
	g := hex.Grid{Cells: map[hex.Coord]hex.Cell{}, Occupants: nil}
	for _, c := range s.ground() {
		g.Cells[c] = s.cell(c)
	}
	return g
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

// joinDice adds extra dice to a notation; with nothing at all it is no dice.
func joinDice(base string, extra []string) dice.Spec {
	parts := slices.DeleteFunc(append([]string{base}, extra...), func(s string) bool { return s == "" })
	spec, _ := dice.Parse(strings.Join(parts, "+"))
	return spec
}
