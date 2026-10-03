package live

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/attack"
	regard "github.com/JorisJonkers-dev/grimoire/api/internal/rules/standing"
)

// readStandings reads how the Campaign's Factions regard the party now. The DM changes Standing
// outside the Session, so it is read whenever it is about to matter; what was read last stands when
// it cannot be read.
func (r *runtime) readStandings() {
	list, err := r.store.Standings(context.Background(), r.campaign)
	if err != nil {
		r.log.Error("live: standings", "error", err)
		return
	}
	r.st.standings = list
}

// factionOf finds a Faction of the Campaign among the Standings read last.
func (s *state) factionOf(id uuid.UUID) (domain.Standing, bool) {
	i := slices.IndexFunc(s.standings, func(x domain.Standing) bool { return x.Faction == id })
	if i < 0 {
		return domain.Standing{}, false
	}
	return s.standings[i], true
}

// tierFor is how a Faction regards one creature: by the Character's Personal Standing when the
// creature is a Character that has one, else by the party's.
func tierFor(st domain.Standing, t domain.Token) regard.Tier {
	if id, ok := characterOf(t); ok {
		if own, has := st.Personal[id]; has {
			return regard.TierOf(own)
		}
	}
	return regard.TierOf(st.Score)
}

// firstReactions tells the DM how each creature of a Faction first takes to the party.
func (s *state) firstReactions(tokens []TokenView, a Audience) {
	if a != AudienceDM {
		return
	}
	for i, v := range tokens {
		id, err := uuid.Parse(v.FactionID)
		if st, ok := s.factionOf(id); err == nil && ok {
			tokens[i].FirstReaction = string(regard.FirstReaction(regard.TierOf(st.Score)))
		}
	}
}

// sway is how an Influence check is rolled: its d20, the lines its Roll Card carries, and, when it is
// aimed at a creature by a Character, the DC it is rolled against and whether the card shows it.
type sway struct {
	notation string
	lines    []domain.Modifier
	target   *domain.TokenID
	dc       int
	shownDC  string
}

// attitudeOf is how a creature takes to a Character: as an Influence check left it, else as its
// Faction regards that Character, else indifferent.
func (s *state) attitudeOf(target domain.Token, actor domain.Token, character uuid.UUID) regard.Attitude {
	if moved, ok := s.movedAttitude(target.ID, character); ok {
		return moved
	}
	if target.Faction != nil {
		if st, ok := s.factionOf(*target.Faction); ok {
			return regard.FirstReaction(tierFor(st, actor))
		}
	}
	return regard.AttitudeIndifferent
}

// movedAttitude is the attitude an Influence check has left a creature with towards a Character, if one has.
func (s *state) movedAttitude(token domain.TokenID, character uuid.UUID) (regard.Attitude, bool) {
	for _, a := range s.attitudes {
		if a.Token == token && a.Character == character {
			return regard.Attitude(a.Value), true
		}
	}
	return "", false
}

// swayed shapes the check of an Influence action. The target's attitude towards whoever tries and how
// its Faction regards them are each a source of Advantage or Disadvantage, and the Roll Card carries a
// line for each. A creature's Faction is open to every screen that can see the creature, so the lines
// tell nobody anything new.
func (r *runtime) swayed(m domain.Member, actor domain.Token, targetID string) (sway, string) {
	plain := sway{notation: attack.D20(attack.Normal)}
	if targetID == "" {
		return plain, ""
	}
	// A creature the party cannot see is not there to be swayed, and is refused as one that does not
	// exist: the roll's card would otherwise say whose it is.
	target, ok := r.st.tokenByID(targetID)
	if !ok || (!m.DM && !r.st.shows(target, r.st.vision())) {
		return plain, "No such creature."
	}
	r.readStandings()
	out := plain
	sources := []regard.Mode{r.aimed(&out, target, actor), r.regarded(&out, target, actor)}
	advantages, disadvantages := 0, 0
	for _, mode := range sources {
		switch mode {
		case regard.Advantage:
			advantages++
		case regard.Disadvantage:
			disadvantages++
		case regard.Straight:
		}
	}
	out.notation = attack.D20(attack.ModeOf(advantages, disadvantages))
	return out, ""
}

// aimed makes a Character's Influence check one against the creature itself: it has a DC, and the
// attitude an earlier check left is a source of its own. Until one has, the creature takes to the
// Character as its Faction does, which the Standing already counts.
func (r *runtime) aimed(out *sway, target, actor domain.Token) regard.Mode {
	character, isCharacter := characterOf(actor)
	if !isCharacter {
		return regard.Straight
	}
	out.target, out.dc = &target.ID, influenceDC(target)
	if r.st.showDCs {
		out.shownDC = " (DC " + strconv.Itoa(out.dc) + ")"
	}
	attitude, moved := r.st.movedAttitude(target.ID, character)
	if !moved {
		return regard.Straight
	}
	if line := attitudeLine(attitude, actor.Label); line != "" {
		out.lines = append(out.lines, domain.Modifier{Label: line, Value: 0})
	}
	return regard.AttitudeMode(attitude)
}

// regarded is what the target's Faction, if it has one, does to the check by how it regards whoever tries.
func (r *runtime) regarded(out *sway, target, actor domain.Token) regard.Mode {
	if target.Faction == nil {
		return regard.Straight
	}
	st, known := r.st.factionOf(*target.Faction)
	if !known {
		return regard.Straight
	}
	tier := tierFor(st, actor)
	effect := regard.SocialCheck(tier)
	out.lines = append(out.lines, domain.Modifier{Label: regard.Line(tier, st.Name), Value: effect.Bonus})
	return effect.Mode
}

// influenceDC is what swaying a creature is rolled against.
func influenceDC(target domain.Token) int {
	if target.Stats == nil {
		return regard.InfluenceDC(0)
	}
	return regard.InfluenceDC(target.Stats.Intelligence)
}

// attitudeLine is what a Roll Card says of an attitude that gives Advantage or Disadvantage.
func attitudeLine(a regard.Attitude, towards string) string {
	switch regard.AttitudeMode(a) {
	case regard.Advantage:
		return "Friendly towards " + towards + ": Advantage"
	case regard.Disadvantage:
		return "Hostile towards " + towards + ": Disadvantage"
	case regard.Straight:
	}
	return ""
}

// swayedTo is the attitude an Influence check leaves a creature with towards the Character who tried.
func (s *state) swayedTo(actor domain.Token, targetID domain.TokenID, total, dc int) *domain.Attitude {
	character, ok := characterOf(actor)
	target, there := s.tokens[targetID]
	if !ok || !there {
		return nil
	}
	after := regard.Sway(s.attitudeOf(target, actor, character), total, dc)
	return &domain.Attitude{Token: targetID, Character: character, Value: string(after)}
}

// shopFaction is the Faction the open Shop belongs to, as its Standing was last read.
func (s *state) shopFaction() (domain.Standing, bool) {
	if s.shop == nil || s.shop.Shop.FactionID == nil {
		return domain.Standing{}, false
	}
	return s.factionOf(*s.shop.Shop.FactionID)
}

// shopPct is what the open Shop adds to its prices for the Character a Container belongs to, by how
// the Shop's Faction regards that Character: its Personal Standing if it has one, else the party's.
func (r *runtime) shopPct(c domain.Container) int {
	r.readStandings()
	st, ok := r.st.shopFaction()
	if !ok {
		return 0
	}
	score := st.Score
	if c.CharacterID != nil {
		if own, has := st.Personal[*c.CharacterID]; has {
			score = own
		}
	}
	return regard.PricePct(regard.TierOf(score))
}

// standingsNow reads how the Campaign's Factions regard the party and hands back what was read.
func (r *runtime) standingsNow() []domain.Standing {
	r.readStandings()
	return r.st.standings
}

// factionNamed is the Faction of the Campaign a command names, or nil when it names none.
func (r *runtime) factionNamed(raw string) (*uuid.UUID, string) {
	if raw == "" {
		return nil, ""
	}
	r.readStandings()
	id, err := uuid.Parse(raw)
	if _, known := r.st.factionOf(id); err != nil || !known {
		return nil, "No such Faction."
	}
	return &id, ""
}

// attitudeViews tells each screen how the creatures it sees take to the Characters.
func (s *state) attitudeViews(tokens []TokenView) {
	for i, v := range tokens {
		for _, a := range s.attitudes {
			if uuid.UUID(a.Token).String() == v.ID {
				tokens[i].Attitudes = append(tokens[i].Attitudes, AttitudeView{CharacterID: a.Character.String(), Attitude: a.Value})
			}
		}
		slices.SortFunc(tokens[i].Attitudes, func(x, y AttitudeView) int { return strings.Compare(x.CharacterID, y.CharacterID) })
	}
}
