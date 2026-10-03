package live

import (
	"context"
	"slices"

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

// swayed shapes the check of an Influence action by how the target's Faction regards whoever tries:
// how the d20 is rolled, and the line the Roll Card carries. A creature's Faction is open to every
// screen that can see the creature, so the line tells nobody anything new.
func (r *runtime) swayed(m domain.Member, actor domain.Token, targetID string) (string, *domain.Modifier, string) {
	if targetID == "" {
		return attack.D20(attack.Normal), nil, ""
	}
	// A creature the party cannot see is not there to be swayed, and is refused as one that does not
	// exist: the roll's card would otherwise say whose it is.
	target, ok := r.st.tokenByID(targetID)
	if !ok || (!m.DM && !r.st.shows(target, r.st.vision())) {
		return "", nil, "No such creature."
	}
	if target.Faction == nil {
		return attack.D20(attack.Normal), nil, ""
	}
	r.readStandings()
	st, known := r.st.factionOf(*target.Faction)
	if !known {
		return attack.D20(attack.Normal), nil, ""
	}
	tier := tierFor(st, actor)
	effect := regard.SocialCheck(tier)
	mode := map[regard.Mode]attack.Mode{regard.Advantage: attack.Advantage, regard.Disadvantage: attack.Disadvantage}[effect.Mode]
	return attack.D20(mode), &domain.Modifier{Label: regard.Line(tier, st.Name), Value: effect.Bonus}, ""
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
