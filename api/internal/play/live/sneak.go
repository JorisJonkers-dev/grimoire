package live

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/objects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/vision"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// planSneak starts the party sneaking, outside a fight, with a Stealth roll for every member on the
// board; or stops it.
func (r *runtime) planSneak(m domain.Member, cmd Command) (Write, string) {
	if !cmd.On {
		if r.st.sneak == nil {
			return Write{}, "The party is not sneaking."
		}
		return Write{Kind: domain.ActionSneakEnded, sneak: nil}, ""
	}
	switch {
	case r.st.combat != nil:
		return Write{}, "Sneaking is for exploration, not a fight."
	case r.st.sneak != nil:
		return Write{}, "The party is already sneaking."
	}
	sneak := &domain.Sneak{Rolls: nil}
	w := Write{Kind: domain.ActionSneakStarted, sneak: sneak}
	for _, t := range r.st.partyMembers() {
		roll := r.request(m, t, "Stealth while sneaking", "1d20", domain.Modifier{Label: "Dexterity (Stealth)", Value: t.Stats.Stealth})
		w.Rolls = append(w.Rolls, roll)
		sneak.Rolls = append(sneak.Rolls, domain.SneakRoll{Token: t.ID, RollID: roll.ID, Total: nil})
	}
	if len(sneak.Rolls) == 0 {
		return Write{}, "No party member is on the board to sneak."
	}
	return w, ""
}

// partyMembers are the standing party creatures, by label.
func (s *state) partyMembers() []domain.Token {
	var out []domain.Token
	for _, t := range s.tokens {
		if t.Kind == domain.TokenParty && standing(t) {
			out = append(out, t)
		}
	}
	slices.SortFunc(out, func(a, b domain.Token) int { return strings.Compare(a.Label, b.Label) })
	return out
}

// sneakRoll finds the member a Stealth roll for sneaking belongs to.
func (s *state) sneakRoll(id domain.RollID) (domain.TokenID, bool) {
	if s.sneak == nil {
		return domain.TokenID{}, false
	}
	for _, sr := range s.sneak.Rolls {
		if sr.RollID == id && sr.Total == nil {
			return sr.Token, true
		}
	}
	return domain.TokenID{}, false
}

// stealthRolled records a member's Stealth total.
func (r *runtime) stealthRolled(t domain.TokenID, id domain.RollID) {
	roll, err := r.store.Roll(context.Background(), r.campaign, id)
	if err != nil || roll.Status != domain.StatusResolved {
		return
	}
	next := &domain.Sneak{Rolls: slices.Clone(r.st.sneak.Rolls)}
	for i := range next.Rolls {
		if next.Rolls[i].Token == t {
			total := roll.Total
			next.Rolls[i].Total = &total
		}
	}
	r.commit(request{}, Write{Kind: domain.ActionStealthRolled, Token: r.st.tokens[t], sneak: next}, roll.Roller, caller.Caller{Subject: roll.Roller.Subject, Origin: caller.OriginSystem, Client: ""})
}

// totals are the group's Stealth totals, false while any is still to roll.
func (s *state) stealthTotals() ([]int, bool) {
	if s.sneak == nil {
		return nil, false
	}
	var out []int
	for _, sr := range s.sneak.Rolls {
		if sr.Total == nil {
			return nil, false
		}
		out = append(out, *sr.Total)
	}
	return out, true
}

// sneakPast checks a sneaking party member who moved against every creature whose reach it is now in:
// the group passes a creature when at least half its Stealth totals meet the creature's passive
// Perception, and the first creature it fails against notices the party, which ends the sneaking.
func (r *runtime) sneakPast(w Write, actor domain.Member, c caller.Caller) {
	switch w.Kind {
	case domain.ActionTokenWalked, domain.ActionTokenMoved, domain.ActionTeleported, domain.ActionJumped:
	default:
		return
	}
	totals, rolled := r.st.stealthTotals()
	mover, ok := r.st.tokens[w.Token.ID]
	if !rolled || !ok || mover.Kind != domain.TokenParty {
		return
	}
	at := hex.Coord{Q: mover.Q, R: mover.R}
	for _, t := range r.st.watchers() {
		if slices.Contains(r.st.reachOf(t), at) && !rules.GroupCheck(totals, rules.PassivePerception(t.Stats.Perception)) {
			sys := caller.Caller{Subject: c.Subject, Origin: caller.OriginSystem, Client: ""}
			w := Write{Kind: domain.ActionPartyNoticed, Token: t, sneak: nil}
			w.manuals = []domain.ManualPrompt{{ID: uuid.New(), Text: t.Label + " notices the party."}}
			r.commit(request{}, w, actor, sys)
			return
		}
	}
}

// watchers are the standing creatures not of the party that could notice it, by label.
func (s *state) watchers() []domain.Token {
	var out []domain.Token
	for _, t := range s.tokens {
		if t.Kind != domain.TokenParty && standing(t) && !s.catalog.Incapacitated(s.actives(t.ID)) {
			out = append(out, t)
		}
	}
	slices.SortFunc(out, func(a, b domain.Token) int { return strings.Compare(a.Label, b.Label) })
	return out
}

// reachOf is where a creature's passive Perception reaches: the hexes it sees within 30 feet.
func (s *state) reachOf(t domain.Token) []hex.Coord {
	at := hex.Coord{Q: t.Q, R: t.R}
	near := hex.Disk(at, objects.ApproachFt/hex.FeetPerHex)
	if s.board == nil {
		return slices.DeleteFunc(near, func(c hex.Coord) bool { return !s.onBoard(c) })
	}
	g := hex.Grid{Cells: map[hex.Coord]hex.Cell{}, Occupants: nil}
	for c := range s.cells {
		g.Cells[c] = s.cell(c)
	}
	scene := vision.Scene{Grid: g, Ambient: ambient(s.board.Map.Ambient)}
	for _, l := range s.board.Lights {
		scene.Lights = append(scene.Lights, vision.Light{At: l.At, BrightFt: l.BrightFt, DimFt: l.DimFt})
	}
	seen := scene.Visible([]vision.Viewer{{At: at, DarkvisionFt: t.DarkvisionFt}})
	return slices.DeleteFunc(near, func(c hex.Coord) bool { return !seen[c] })
}

// SneakView is the party's sneaking: whether Stealth rolls are still out, and the hexes the creatures
// the audience sees can notice it in. The DM sees every creature's reach and each member's total.
type SneakView struct {
	Waiting bool           `json:"waiting"`
	Reach   []Hex          `json:"reach"`
	Totals  map[string]int `json:"totals,omitempty"`
}

// sneakView shows the sneaking to an audience.
func (s *state) sneakView(a Audience, seen map[hex.Coord]bool) *SneakView {
	if s.sneak == nil {
		return nil
	}
	_, rolled := s.stealthTotals()
	v := &SneakView{Waiting: !rolled, Reach: []Hex{}, Totals: nil}
	in := map[hex.Coord]bool{}
	for _, t := range s.watchers() {
		if a != AudienceDM && !s.shows(t, seen) {
			continue
		}
		for _, c := range s.reachOf(t) {
			in[c] = true
		}
	}
	keys := make([]hex.Coord, 0, len(in))
	for c := range in {
		keys = append(keys, c)
	}
	v.Reach = wireHexes(keys)
	slices.SortFunc(v.Reach, compareHex)
	if a == AudienceDM {
		v.Totals = map[string]int{}
		for _, sr := range s.sneak.Rolls {
			if sr.Total != nil {
				v.Totals[uuid.UUID(sr.Token).String()] = *sr.Total
			}
		}
	}
	return v
}
