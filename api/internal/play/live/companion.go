package live

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
)

// companionToken is the token a Companion is on the map as, or nil.
func (s *state) companionToken(id uuid.UUID) *domain.Token {
	for _, t := range s.tokens {
		if t.Companion != nil && *t.Companion == id {
			return &t
		}
	}
	return nil
}

// companionStats is what a Companion comes onto the map with: its own name, its creature's statblock
// with the hit points it kept, the party's side, and whoever it is given to.
func (r *runtime) companionStats(ctx context.Context, campaign uuid.UUID, cmd *Command) (string, domain.Stats, string) {
	id, err := uuid.Parse(cmd.CompanionID)
	var ref domain.CompanionRef
	var stats domain.Stats
	if err == nil {
		ref, err = r.stats.Companion(ctx, campaign, id)
	}
	if err == nil {
		_, stats, err = r.stats.Monster(ctx, campaign, ref.Slug)
	}
	if err != nil {
		return "", domain.Stats{}, "No such Companion."
	}
	cmd.companion, cmd.TokenKind, cmd.ControllerID = &id, domain.TokenParty, ""
	if ref.Controller != nil {
		cmd.ControllerID = ref.Controller.String()
	}
	if ref.HP != nil {
		stats.HP = min(*ref.HP, stats.HPMax)
	}
	return ref.Name, stats, ""
}

// planAssign hands a Companion to a Player, or back to the DM when no one is named. The Companion keeps
// its new hand between Sessions.
func (r *runtime) planAssign(cmd Command) (Write, string) {
	t, ok := r.st.tokenByID(cmd.TokenID)
	switch {
	case !ok:
		return Write{}, "No such token."
	case t.Companion == nil:
		return Write{}, "Only a Companion changes hands."
	}
	t.Controller = nil
	if cmd.ControllerID != "" {
		id, err := uuid.Parse(cmd.ControllerID)
		if err != nil {
			return Write{}, "No such member."
		}
		if _, err := r.members.Member(context.Background(), r.st.session.CampaignID, id); err != nil {
			return Write{}, "No such member."
		}
		t.Controller = &id
	}
	return Write{Kind: domain.ActionControlAssigned, Token: t}, ""
}

// fought is what a fight was worth and who shares it: the XP of its defeated enemies, the Characters
// of the party who fought, and the Companions beside them.
func (r *runtime) fought(c *domain.Combat) (int, []uuid.UUID, []uuid.UUID, error) {
	total := 0
	var heroes, companions []uuid.UUID
	for _, x := range c.Combatants {
		t, ok := r.st.tokens[x.TokenID]
		if !ok || t.Stats == nil {
			continue
		}
		id, hero := characterOf(t)
		switch {
		case t.Kind == domain.TokenEnemy && t.Stats.HP == 0 && strings.HasPrefix(t.Stats.Source, "monster:"):
			xp, err := r.store.CreatureXP(context.Background(), r.st.session.CampaignID, strings.TrimPrefix(t.Stats.Source, "monster:"))
			if err != nil {
				return 0, nil, nil, err
			}
			total += xp
		case t.Kind == domain.TokenParty && hero:
			heroes = append(heroes, id)
		case t.Kind == domain.TokenParty && t.Companion != nil:
			companions = append(companions, *t.Companion)
		}
	}
	return total, heroes, companions, nil
}

// awards is the XP a fight that ends gives: what its defeated enemies were worth, split evenly among
// the party's Characters who fought and the Companions who take a share. A Companion's share goes to
// nobody: that is its price. A fight with no Character in it awards nothing, and neither does one whose
// worth cannot be reckoned.
func (r *runtime) awards(c *domain.Combat) []domain.XPAward {
	total, heroes, companions, err := r.fought(c)
	sharing := 0
	if err == nil && total > 0 && len(heroes) > 0 {
		sharing, err = r.store.SharingCompanions(context.Background(), r.st.session.CampaignID, companions)
	}
	if err != nil {
		r.log.Error("live: reckon xp", "error", err)
		return nil
	}
	if total == 0 || len(heroes) == 0 || total/(len(heroes)+sharing) == 0 {
		return nil
	}
	out := make([]domain.XPAward, 0, len(heroes))
	for _, id := range heroes {
		out = append(out, domain.XPAward{Character: id, Amount: total / (len(heroes) + sharing)})
	}
	return out
}
