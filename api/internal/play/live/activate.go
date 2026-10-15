package live

import (
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/vision"
)

// planActivate puts an Effect on the targets its Targeting allows, from the caster as its source: at the
// slot it is cast with, for the caster's action in combat, ending the caster's other concentration. The
// DM may force targets the Targeting refuses; the Action Log records that apart.
func (r *runtime) planActivate(m domain.Member, cmd Command) (Write, string) {
	caster, def, reason := r.activation(m, cmd)
	if reason != "" {
		return Write{}, reason
	}
	w := Write{Kind: domain.ActionActivated, Token: caster}
	if reason := r.st.payAction(&w, caster); reason != "" {
		return Write{}, reason
	}
	targets, picks, reason := r.picks(m, caster, cmd.Targets)
	if reason != "" {
		return Write{}, reason
	}
	if refused := def.Targeting.Refusals(hex.Coord{Q: caster.Q, R: caster.R}, picks); len(refused) > 0 {
		if !m.DM || !cmd.Force {
			return Write{}, r.refusal(m, refused[0], targets)
		}
		w.Kind = domain.ActionActivationForced
	}
	if def.Targeting.Kind == effects.TargetSelf && len(targets) == 0 {
		targets = []domain.Token{caster}
	}
	r.st.putOn(&w, def, caster, targets, cmd.EffectMode)
	return w, ""
}

// putOn puts the Effect on every target from the caster, with its duration and the parts the DM resolves,
// and ends the caster's other concentration when it needs concentration.
func (s *state) putOn(w *Write, def effects.Definition, caster domain.Token, targets []domain.Token, mode string) {
	for _, t := range targets {
		e := domain.Effect{
			ID: domain.EffectID(uuid.New()), Target: t.ID, Source: &caster.ID, Slug: def.Slug, Name: def.Name, Concentration: def.Concentration,
			Level: 1, Mode: mode,
		}
		s.lasting(&e, def.Duration)
		w.activated = append(w.activated, e)
		for _, text := range s.catalog.Instructions(def.Slug, def.Name, e.Mode) {
			w.manuals = append(w.manuals, domain.ManualPrompt{ID: uuid.New(), Text: t.Label + ": " + text})
		}
	}
	for _, old := range s.fx.Active {
		if def.Concentration && old.Concentration && old.Source != nil && *old.Source == caster.ID {
			w.ended = append(w.ended, old.ID)
		}
	}
}

// activation is the caster and the Effect at the slot it is cast with, or why it cannot be activated.
func (r *runtime) activation(m domain.Member, cmd Command) (domain.Token, effects.Definition, string) {
	id, _ := uuid.Parse(cmd.TokenID)
	caster, ok := r.st.tokens[domain.TokenID(id)]
	slug := strings.ToLower(strings.TrimSpace(cmd.Effect))
	def, known := r.st.catalog.Lookup(slug)
	_, area := r.st.catalog.AreaAt(slug, effects.Level{})
	switch {
	case !ok || caster.Stats == nil || (!m.DM && !r.st.shows(caster, r.st.vision())):
		return caster, def, "No such caster."
	case !m.DM && (caster.Controller == nil || *caster.Controller != m.ID):
		return caster, def, "That token is not yours to play."
	case !known:
		return caster, def, "That is not an Effect the rules know."
	case area || def.Targeting.Kind == effects.TargetArea:
		return caster, def, "Cast " + def.Name + " as an area spell."
	case def.Targeting.Kind == "":
		return caster, def, def.Name + " states no Targeting; the DM applies it by hand."
	case r.st.catalog.Incapacitated(r.st.actives(caster.ID)):
		return caster, def, caster.Label + " can't act while Incapacitated."
	case !landsWhole(r.st.catalog.LandingOf(def.Slug, cmd.EffectMode)):
		return caster, def, def.Name + " does more as it lands than an Activation can yet; the DM applies it by hand."
	}
	if modes := def.Modes(); len(modes) > 0 && !slices.Contains(modes, cmd.EffectMode) {
		return caster, def, "Choose " + strings.Join(modes, " or ") + "."
	}
	slot, refused := r.st.slotOf(cmd)
	return caster, def.At(effects.Level{Slot: slot}), refused
}

// landsWhole reports whether an Effect does nothing the moment it lands beyond being put on.
func landsWhole(l effects.Landing) bool {
	return l.TempHP == 0 && !l.Dispels && len(l.Grants)+len(l.Resources)+len(l.Reveals) == 0 && l.Form == nil
}

// payAction takes the caster's action in combat, on its turn; out of combat an Activation costs nothing.
func (s *state) payAction(w *Write, caster domain.Token) string {
	if c := s.combat; c == nil || c.Status != domain.CombatActive {
		return ""
	}
	x, acting := s.combatantOf(caster.ID)
	switch {
	case !acting:
		return "It is not " + caster.Label + "'s turn."
	case !x.Economy.Action:
		return caster.Label + " has already used their action."
	}
	w.Combatant = x.ID
	return ""
}

// picks are the tokens an Activation names, as its Targeting sees them; a token the member cannot see is
// no pick at all.
func (r *runtime) picks(m domain.Member, caster domain.Token, ids []string) ([]domain.Token, []effects.Candidate, string) {
	seen := r.st.vision()
	var targets []domain.Token
	var out []effects.Candidate
	for _, raw := range ids {
		id, _ := uuid.Parse(raw)
		t, ok := r.st.tokens[domain.TokenID(id)]
		if !ok || (!m.DM && !r.st.shows(t, seen)) {
			return nil, nil, "Choose targets on the map."
		}
		targets = append(targets, t)
		out = append(out, effects.Candidate{
			ID: raw, At: hex.Coord{Q: t.Q, R: t.R}, Self: t.ID == caster.ID, Ally: (t.Kind == domain.TokenParty) == (caster.Kind == domain.TokenParty),
			Object: t.Kind == domain.TokenObject || t.Stats == nil, Type: creatureType(t), Seen: r.st.sees(caster, t),
		})
	}
	return targets, out, ""
}

func creatureType(t domain.Token) string {
	if t.Stats == nil {
		return ""
	}
	return t.Stats.CreatureType
}

// sees reports whether a caster sees a token with its own Senses, not the party's.
func (s *state) sees(caster, t domain.Token) bool {
	if t.ID == caster.ID {
		return true
	}
	if t.Hidden || (s.board != nil && !s.sight([]vision.Viewer{viewer(caster)})[hex.Coord{Q: t.Q, R: t.R}]) {
		return false
	}
	qualities, beaten := s.qualities(t)
	if len(qualities) == 0 {
		return true
	}
	at, from := hex.Coord{Q: t.Q, R: t.R}, hex.Coord{Q: caster.Q, R: caster.R}
	target := vision.Target{Qualities: qualities, Beaten: beaten, DistanceFt: hex.Distance(at, from) * hex.FeetPerHex, OnGround: true}
	return vision.Perceive(perceivers(caster), target).Seen
}

// refusal is why the picks are refused, naming the pick as the member sees it.
func (r *runtime) refusal(m domain.Member, refused effects.Refusal, targets []domain.Token) string {
	if refused.Pick < 0 {
		return refused.Reason
	}
	t := targets[refused.Pick]
	if !m.DM {
		t = r.st.masked(t, AudienceParty)
	}
	return t.Label + ": " + refused.Reason
}
