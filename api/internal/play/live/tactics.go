package live

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/tactics"
)

func (r *runtime) planTactics(cmd Command) (Write, string) {
	id, _ := uuid.Parse(cmd.TokenID)
	t, ok := r.st.tokens[domain.TokenID(id)]
	if !ok || t.Stats == nil || t.Kind == domain.TokenParty {
		return Write{}, "Only creatures the DM plays have tactics."
	}
	switch cmd.Tactics {
	case tactics.FromIntelligence, tactics.OverrideSimple, tactics.OverrideCunning, tactics.OverrideOff:
	default:
		return Write{}, "Tactics are auto, simple, cunning or off."
	}
	t.Tactics = cmd.Tactics
	return Write{Kind: domain.ActionTacticsSet, Token: t}, ""
}

// suggest is a creature's Suggested Action on its turn. It only uses what the creature can observe:
// which enemies it sees, how far they are, and the ranged damage it has watched them deal; never hit
// points, armour class or what other creatures intend.
func (s *state) suggest(x domain.Combatant, t domain.Token) *SuggestionView {
	if t.Stats == nil || t.Kind == domain.TokenParty || !s.combat.Acting(x) || !x.Economy.Action || s.combat.Attack != nil {
		return nil
	}
	style := tactics.For(t.Tactics, t.Stats.Intelligence)
	attacks := make([]tactics.Attack, 0, len(t.Stats.Attacks))
	for _, a := range t.Stats.Attacks {
		attacks = append(attacks, tactics.Attack{ReachFt: a.ReachFt, RangeFt: a.RangeFt, LongRangeFt: a.LongRangeFt})
	}
	g, from := s.sightGrid(), hex.Coord{Q: t.Q, R: t.R}
	var targets []tactics.Target
	labels := map[string]string{}
	for _, o := range s.tokens {
		to := hex.Coord{Q: o.Q, R: o.R}
		if !standing(o) || (o.Kind == domain.TokenParty) == (t.Kind == domain.TokenParty) || !hex.LineOfSight(g, from, to).Visible {
			continue
		}
		id := uuid.UUID(o.ID).String()
		labels[id] = o.Label
		targets = append(targets, tactics.Target{ID: id, DistanceFt: hex.Distance(from, to) * hex.FeetPerHex, RangedDamageSeen: s.observed[t.ID][o.ID]})
	}
	sug, ok := tactics.Suggest(style, attacks, targets)
	if !ok {
		return nil
	}
	who, name := labels[sug.Target.ID], map[tactics.Style]string{tactics.Simple: "Simple", tactics.Cunning: "Cunning"}[style]
	out := &SuggestionView{TargetID: sug.Target.ID}
	switch sug.Reason {
	case tactics.Sniper:
		out.Reason = fmt.Sprintf("%s: it saw %s deal %d damage from range.", name, who, sug.Target.RangedDamageSeen)
	case tactics.Approach:
		out.Reason = fmt.Sprintf("%s: nothing reaches yet; close in on %s, %d ft away.", name, who, sug.Target.DistanceFt)
		return out
	case tactics.Nearest:
		out.Reason = fmt.Sprintf("%s: %s is the nearest enemy, %d ft away.", name, who, sug.Target.DistanceFt)
	}
	n := sug.Attack
	out.AttackNo = &n
	return out
}
