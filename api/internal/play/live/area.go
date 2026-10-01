package live

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/combat"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// DefaultSpellDC is the save DC of a caster whose statblock gives none.
const DefaultSpellDC = 13

// areaPlan is an area spell worked out against the board before anything is rolled.
type areaPlan struct {
	caster  domain.Token
	slug    string
	spell   effects.AreaSpell
	hexes   []hex.Coord
	targets []domain.Token
}

// aimArea checks an area spell a member asks for: in combat, on the caster's turn with its action left,
// its point within range, and every creature standing in its hexes, friend or foe.
func (r *runtime) aimArea(m domain.Member, cmd Command) (areaPlan, string) {
	id, _ := uuid.Parse(cmd.TokenID)
	caster, ok := r.st.tokens[domain.TokenID(id)]
	spell, known := r.st.catalog.AreaOf(strings.ToLower(strings.TrimSpace(cmd.Effect)))
	switch {
	case !ok || caster.Stats == nil || (!m.DM && !r.st.shows(caster, r.st.vision())):
		return areaPlan{}, "No such caster."
	case !m.DM && (caster.Controller == nil || *caster.Controller != m.ID):
		return areaPlan{}, "That token is not yours to play."
	case !known:
		return areaPlan{}, "That is not an area spell the rules know."
	case r.st.cast != nil:
		return areaPlan{}, "An area spell is still waiting on its rolls."
	case r.st.catalog.Incapacitated(r.st.actives(caster.ID)):
		return areaPlan{}, caster.Label + " can't act while Incapacitated."
	}
	if c := r.st.combat; c == nil || c.Status != domain.CombatActive {
		return areaPlan{}, "Spells happen in combat, once initiative is rolled."
	}
	x, acting := r.st.combatantOf(caster.ID)
	point, from := hex.Coord{Q: cmd.Q, R: cmd.R}, hex.Coord{Q: caster.Q, R: caster.R}
	switch {
	case !acting:
		return areaPlan{}, "It is not " + caster.Label + "'s turn."
	case !x.Economy.Action:
		return areaPlan{}, caster.Label + " has already used their action."
	case !r.st.onBoard(point):
		return areaPlan{}, "That hex is off the map."
	case spell.Area.RangeFt > 0 && hex.Distance(from, point)*hex.FeetPerHex > spell.Area.RangeFt:
		return areaPlan{}, "The point is out of range."
	case spell.Area.RangeFt == 0 && point == from && spell.Area.Shape != hex.EmanationArea:
		return areaPlan{}, "Aim away from the caster."
	}
	origin, aim := point, point
	switch {
	case spell.Area.RangeFt == 0:
		origin = from
	case spell.Area.Shape == hex.WallArea:
		aim = hex.Coord{Q: 2*point.Q - from.Q, R: 2*point.R - from.R}
	}
	p := areaPlan{caster: caster, slug: strings.ToLower(strings.TrimSpace(cmd.Effect)), spell: spell}
	p.hexes = slices.DeleteFunc(hex.Area(spell.Area.Shape, origin, aim, spell.Area.SizeFt), func(c hex.Coord) bool { return !r.st.onBoard(c) })
	for _, t := range r.st.tokens {
		if standing(t) && slices.Contains(p.hexes, hex.Coord{Q: t.Q, R: t.R}) {
			p.targets = append(p.targets, t)
		}
	}
	slices.SortFunc(p.targets, func(a, b domain.Token) int { return strings.Compare(a.Label, b.Label) })
	return p, ""
}

// upcast scales an area spell's damage to the slot it is cast with: no lower than the spell's own
// level, no higher than 9.
func (p *areaPlan) upcast(cat effects.Catalog, slot int) string {
	def, _ := cat.Lookup(p.slug)
	sc := def.Scaling
	if slot == 0 || sc == nil || sc.Axis != effects.SlotLevel {
		return ""
	}
	if slot < sc.Base || slot > 9 {
		return fmt.Sprintf("Cast %s with a slot of level %d to 9.", p.spell.Name, sc.Base)
	}
	p.spell.Damage.Dice = sc.Apply(p.spell.Damage.Dice, effects.Level{Slot: slot})
	return ""
}

func spellDC(t domain.Token) int {
	if t.Stats.SpellDC > 0 {
		return t.Stats.SpellDC
	}
	return DefaultSpellDC
}

// aimCast aims an area spell and scales it to the slot it is cast with.
func (r *runtime) aimCast(m domain.Member, cmd Command) (areaPlan, string) {
	p, reason := r.aimArea(m, cmd)
	if reason == "" {
		reason = p.upcast(r.st.catalog, cmd.Slot)
	}
	return p, reason
}

func (r *runtime) previewArea(req request) {
	p, reason := r.aimCast(req.from.Member, req.cmd)
	if reason != "" {
		r.reject(req, reason)
		return
	}
	out := &AreaPreview{TokenID: req.cmd.TokenID, Effect: p.slug, Name: p.spell.Name, DC: spellDC(p.caster), Hexes: wireHexes(p.hexes), Targets: []AreaTarget{}}
	if def, _ := r.st.catalog.Lookup(p.slug); def.Concentration {
		for _, e := range r.st.held(p.caster.ID) {
			out.Ends = append(out.Ends, e.Name)
		}
	}
	seen := r.st.vision()
	for _, t := range p.targets {
		if req.from.Member.DM || r.st.shows(t, seen) {
			ally := (t.Kind == domain.TokenParty) == (p.caster.Kind == domain.TokenParty)
			out.Targets = append(out.Targets, AreaTarget{TokenID: uuid.UUID(t.ID).String(), Ally: ally})
			if ally {
				out.Allies++
			}
		}
	}
	r.send(req.from, Update{Kind: UpdAreaPreview, Seq: r.st.session.Seq, Nonce: req.cmd.Nonce, Area: out})
}

// planCast spends the caster's action and opens the damage roll and every target's saving throw.
func (r *runtime) planCast(m domain.Member, cmd Command) (Write, string) {
	p, reason := r.aimCast(m, cmd)
	if reason != "" {
		return Write{}, reason
	}
	x, _ := r.st.combatantOf(p.caster.ID)
	cast := &domain.AreaCast{ID: uuid.New(), Caster: p.caster.ID, Spell: p.slug, DC: spellDC(p.caster), Hexes: p.hexes}
	w := Write{Kind: domain.ActionAreaCast, Token: p.caster, Combatant: x.ID, cast: cast}
	if dmg := p.spell.Damage; dmg.Dice != "" {
		roll := r.request(m, p.caster, p.spell.Name+" damage", dmg.Dice)
		cast.DamageRoll = &roll.ID
		w.Rolls = append(w.Rolls, roll)
	}
	ability := p.spell.Save
	for _, t := range p.targets {
		target := domain.AreaTarget{Token: t.ID}
		if roll, ok := r.saveRoll(m, t, ability, fmt.Sprintf("save against %s (DC %d)", p.spell.Name, cast.DC)); ok {
			target.SaveRoll = &roll.ID
			w.Rolls = append(w.Rolls, roll)
		}
		cast.Targets = append(cast.Targets, target)
	}
	return w, ""
}

// castRolls lists every roll an area spell waits on.
func castRolls(c *domain.AreaCast) []domain.RollID {
	var out []domain.RollID
	if c.DamageRoll != nil {
		out = append(out, *c.DamageRoll)
	}
	for _, t := range c.Targets {
		if t.SaveRoll != nil {
			out = append(out, *t.SaveRoll)
		}
	}
	return out
}

// areaRolled resolves an area spell once its last roll is in and no Counterspell is pending: the ground
// reacts to the damage and takes any new Surface, every target takes full or half damage, those who
// failed are pushed and get its condition, and a concentration spell stays on its caster.
func (r *runtime) areaRolled() {
	if r.st.cast == nil || r.st.awaitingCounter() {
		return
	}
	c := *r.st.cast
	totals, actor, ok := r.castTotals(&c)
	if !ok {
		return
	}
	spell, _ := r.st.catalog.AreaOf(c.Spell)
	sys := caller.Caller{Subject: actor.Subject, Origin: caller.OriginSystem, Client: ""}
	caster := r.st.tokens[c.Caster]
	failed, hits := r.st.outcomes(&c, spell, totals)
	extra, branchNotes := r.st.branched(&c, totals)
	w := Write{Kind: domain.ActionAreaResolved, Token: caster, terrain: true, Hexes: c.Hexes, damageType: spell.Damage.Type, created: spell.Surface, manuals: append(notes(spell, failed), branchNotes...)}
	r.commit(request{}, w, actor, sys)
	for _, h := range hits {
		r.commit(request{}, Write{Kind: domain.ActionDamageDealt, Token: caster, HP: &h}, actor, sys)
	}
	for _, t := range failed {
		if moved := r.st.forced(caster, r.st.tokens[t.ID], spell.Push); spell.Push.Ft > 0 && moved != nil {
			r.commit(request{}, Write{Kind: domain.ActionTokenMoved, Token: *moved}, actor, sys)
		}
	}
	if w := r.st.sustained(r.st.tokens[caster.ID], c.Spell); w != nil {
		r.commit(request{}, *w, actor, sys)
	}
	for _, e := range extra {
		r.commit(request{}, Write{Kind: domain.ActionEffectApplied, Token: r.st.tokens[e.Target], effect: &e}, actor, sys)
	}
	if spell.Condition == "" {
		return
	}
	def, _ := r.st.catalog.Lookup(spell.Condition)
	for _, t := range failed {
		e := domain.Effect{ID: domain.EffectID(uuid.New()), Target: t.ID, Slug: spell.Condition, Name: def.Name}
		r.commit(request{}, Write{Kind: domain.ActionEffectApplied, Token: t, effect: &e}, actor, sys)
	}
}

// castTotals reads every roll of an area spell; ok is false while any is still open. The damage roll's
// roller signs the result.
func (r *runtime) castTotals(c *domain.AreaCast) (map[domain.RollID]int, domain.Member, bool) {
	totals := map[domain.RollID]int{}
	var actor domain.Member
	for _, id := range castRolls(c) {
		roll, err := r.store.Roll(context.Background(), r.st.session.CampaignID, id)
		if err != nil || roll.Status != domain.StatusResolved {
			return nil, actor, false
		}
		totals[id] = roll.Total
		if c.DamageRoll != nil && id == *c.DamageRoll || actor.Name == "" {
			actor = roll.Roller
		}
	}
	return totals, actor, true
}

// outcomes works out who failed their save and how much damage each target takes: all of it, half on a
// save when the spell halves, or none.
func (s *state) outcomes(c *domain.AreaCast, spell effects.AreaSpell, totals map[domain.RollID]int) ([]domain.Token, []HPChange) {
	var failed []domain.Token
	var hits []HPChange
	for _, target := range c.Targets {
		t, ok := s.tokens[target.Token]
		if !ok {
			continue
		}
		saved := target.SaveRoll != nil && totals[*target.SaveRoll] >= c.DC
		if !saved {
			failed = append(failed, t)
		}
		amount := 0
		if c.DamageRoll != nil {
			amount = totals[*c.DamageRoll]
		}
		if saved {
			amount = map[bool]int{true: amount / 2, false: 0}[spell.Damage.Half]
		}
		if amount > 0 {
			hits = append(hits, damage(t, amount, false))
		}
	}
	return failed, hits
}

// branched works out what an area spell's branches add for each target as the save came out and the
// target's hit points stood before the damage: conditions to put on, and parts the DM resolves.
func (s *state) branched(c *domain.AreaCast, totals map[domain.RollID]int) ([]domain.Effect, []domain.ManualPrompt) {
	def, _ := s.catalog.Lookup(c.Spell)
	var fx []domain.Effect
	var out []domain.ManualPrompt
	for _, target := range c.Targets {
		t, ok := s.tokens[target.Token]
		if !ok || t.Stats == nil {
			continue
		}
		at := effects.Situation{Saved: true, Margin: 0, HP: t.Stats.HP, First: true, Type: ""}
		if target.SaveRoll != nil {
			total := totals[*target.SaveRoll]
			at.Saved, at.Margin = total >= c.DC, c.DC-total
		}
		for _, part := range def.Branches("", at) {
			switch part := part.(type) {
			case effects.SaveCondition:
				cond, _ := s.catalog.Lookup(part.Slug)
				fx = append(fx, domain.Effect{ID: domain.EffectID(uuid.New()), Target: t.ID, Slug: part.Slug, Name: cond.Name, Level: 1})
			case effects.Manual:
				out = append(out, domain.ManualPrompt{ID: uuid.New(), Text: t.Label + ": " + part.Instruction})
			default:
			}
		}
	}
	return fx, out
}

// notes hands the DM a spell's unmodelled parts, naming who failed the save.
func notes(spell effects.AreaSpell, failed []domain.Token) []domain.ManualPrompt {
	if len(failed) == 0 {
		return nil
	}
	names := make([]string, 0, len(failed))
	for _, t := range failed {
		names = append(names, t.Label)
	}
	out := make([]domain.ManualPrompt, 0, len(spell.Instructions))
	for _, text := range spell.Instructions {
		out = append(out, domain.ManualPrompt{ID: uuid.New(), Text: text + " (" + strings.Join(names, ", ") + ")"})
	}
	return out
}

func (r *runtime) planTerrain(cmd Command) (Write, string) {
	hs, reason := r.boardHexes(cmd.Hexes)
	if reason != "" {
		return Write{}, reason
	}
	if cmd.Kind == CmdSetElevation {
		switch {
		case r.st.board == nil:
			return Write{}, "Choose a map first."
		case cmd.ElevationFt < -100 || cmd.ElevationFt > 100:
			return Write{}, "Elevation runs from -100 to 100 feet."
		}
		return Write{Kind: domain.ActionElevationSet, Hexes: hs, elevation: cmd.ElevationFt}, ""
	}
	k := surface.Kind(cmd.Surface)
	switch {
	case k != surface.None && !surface.Valid(k):
		return Write{}, "Surfaces are fire, grease, water, ice, web or electrified."
	case cmd.Rounds < 0 || cmd.Rounds > 100:
		return Write{}, "Surfaces last 0 to 100 rounds."
	}
	return Write{Kind: domain.ActionSurfacesSet, Hexes: hs, terrain: true, created: effects.CreateSurface{Kind: k, Rounds: cmd.Rounds}}, ""
}

// applyTerrain changes Surfaces and height: painted by the DM, or left by an area spell's damage.
func applyTerrain(s *state, w *Write) {
	switch w.Kind {
	case domain.ActionAreaCast:
		s.cast = w.cast
		i := slices.IndexFunc(s.combat.Combatants, func(x domain.Combatant) bool { return x.ID == w.Combatant })
		s.combat.Combatants[i].Economy, _ = s.combat.Combatants[i].Economy.Spend(combat.Action)
		w.Combat = s.combat
		return
	case domain.ActionElevationSet:
		for _, c := range w.Hexes {
			s.board.Elevation[c] = w.elevation
			if w.elevation == 0 {
				delete(s.board.Elevation, c)
			}
		}
		return
	case domain.ActionAreaResolved:
		s.cast = nil
	}
	if s.surfaces == nil {
		s.surfaces = map[hex.Coord]domain.Surface{}
	}
	for _, c := range w.Hexes {
		now := s.surfaces[c]
		now.Kind = surface.React(now.Kind, w.damageType)
		if w.Kind == domain.ActionSurfacesSet || w.created.Kind != surface.None {
			now = domain.Surface{Kind: w.created.Kind, RoundsLeft: w.created.Rounds}
		}
		s.surfaces[c] = now
		if now.Kind == surface.None {
			delete(s.surfaces, c)
		}
	}
}

// weather counts Surfaces down when a new round starts; one that reaches zero is gone.
func (s *state) weather() bool {
	changed := false
	for c, sf := range s.surfaces {
		if sf.RoundsLeft == 0 {
			continue
		}
		changed = true
		if sf.RoundsLeft--; sf.RoundsLeft == 0 {
			delete(s.surfaces, c)
			continue
		}
		s.surfaces[c] = sf
	}
	return changed
}

// hazards hands the DM the damage of creatures that start their turn in, or walk into, a harmful Surface.
func (s *state) hazards(started map[domain.TokenID]bool, w *Write) bool {
	var texts []string
	for id, now := range started {
		t := s.tokens[id]
		if dice, kind, ok := surface.Hazard(s.surfaces[hex.Coord{Q: t.Q, R: t.R}].Kind); now && ok {
			texts = append(texts, fmt.Sprintf("%s starts its turn in %s: %s %s damage.", t.Label, s.surfaces[hex.Coord{Q: t.Q, R: t.R}].Kind, dice, kind))
		}
	}
	if w.Kind == domain.ActionTokenWalked {
		for _, c := range w.Path[min(1, len(w.Path)):] {
			if dice, kind, ok := surface.Hazard(s.surfaces[c].Kind); ok {
				texts = append(texts, fmt.Sprintf("%s walks into %s: %s %s damage.", w.Token.Label, s.surfaces[c].Kind, dice, kind))
				break
			}
		}
	}
	slices.Sort(texts)
	for _, t := range texts {
		s.fx.Manual = append(s.fx.Manual, domain.ManualPrompt{ID: uuid.New(), Text: t})
	}
	return len(texts) > 0
}

// terrainViews shows Surfaces and height where the audience knows the ground, and the area spell on
// the table to anyone who sees its caster.
func (s *state) terrainViews(v *View, a Audience, seen map[hex.Coord]bool) {
	knows := func(c hex.Coord) bool {
		return a == AudienceDM || s.board == nil || seen[c] || s.board.Reveals[c]
	}
	for c, sf := range s.surfaces {
		if knows(c) {
			v.Surfaces = append(v.Surfaces, SurfaceView{Q: c.Q, R: c.R, Kind: string(sf.Kind), RoundsLeft: sf.RoundsLeft})
		}
	}
	slices.SortFunc(v.Surfaces, func(a, b SurfaceView) int { return compareHex(Hex{a.Q, a.R}, Hex{b.Q, b.R}) })
	if s.board != nil {
		for c, e := range s.board.Elevation {
			if knows(c) {
				v.Elevation = append(v.Elevation, ElevationView{Q: c.Q, R: c.R, ElevationFt: e})
			}
		}
		slices.SortFunc(v.Elevation, func(a, b ElevationView) int { return compareHex(Hex{a.Q, a.R}, Hex{b.Q, b.R}) })
	}
	if c := s.cast; c != nil && (a == AudienceDM || s.shows(s.tokens[c.Caster], seen)) {
		v.Area = s.areaView(c, a, seen)
	}
}

func (s *state) areaView(c *domain.AreaCast, a Audience, seen map[hex.Coord]bool) *AreaView {
	spell, _ := s.catalog.AreaOf(c.Spell)
	av := &AreaView{CasterID: uuid.UUID(c.Caster).String(), Name: spell.Name, Hexes: wireHexes(c.Hexes), Saves: []AreaSave{}}
	if c.DamageRoll != nil {
		av.DamageRollID = uuid.UUID(*c.DamageRoll).String()
	}
	for _, t := range c.Targets {
		if a != AudienceDM && !s.shows(s.tokens[t.Token], seen) {
			continue
		}
		sv := AreaSave{TokenID: uuid.UUID(t.Token).String()}
		if t.SaveRoll != nil {
			sv.RollID = uuid.UUID(*t.SaveRoll).String()
		}
		av.Saves = append(av.Saves, sv)
	}
	return av
}

func compareHex(a, b Hex) int {
	if a.Q != b.Q {
		return a.Q - b.Q
	}
	return a.R - b.R
}
