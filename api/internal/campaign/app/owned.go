package app

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Mine lists the caller's Characters with their progress in each Campaign.
func (s *Characters) Mine(ctx context.Context, c caller.Caller) ([]domain.OwnedCharacter, error) {
	return s.Repo.OwnedCharacters(ctx, c.Subject)
}

// Owned reads one of the caller's Characters; anyone else's is not found.
func (s *Characters) Owned(ctx context.Context, c caller.Caller, id domain.OwnedID) (domain.OwnedCharacter, error) {
	o, err := s.Repo.OwnedCharacter(ctx, id)
	if err != nil {
		return domain.OwnedCharacter{}, err
	}
	if o.OwnerSubject != c.Subject {
		return domain.OwnedCharacter{}, domain.ErrNotFound
	}
	return o, nil
}

// UpdateOwned changes a Character's name and Backstory; the name flows to every Campaign it plays in.
func (s *Characters) UpdateOwned(ctx context.Context, c caller.Caller, id domain.OwnedID, name, backstory string) (domain.OwnedCharacter, error) {
	if _, err := s.Owned(ctx, c, id); err != nil {
		return domain.OwnedCharacter{}, err
	}
	name, err := cleanText(name, 60)
	if err != nil {
		return domain.OwnedCharacter{}, err
	}
	backstory = strings.TrimSpace(backstory)
	if utf8.RuneCountInString(backstory) > 4000 {
		return domain.OwnedCharacter{}, domain.ErrInvalid
	}
	if err := s.Repo.UpdateOwnedCharacter(ctx, id, name, backstory, s.Now()); err != nil {
		return domain.OwnedCharacter{}, err
	}
	return s.Owned(ctx, c, id)
}

// Join brings one of the caller's Characters into another Campaign the caller is a Member of. The new
// Campaign Character takes the Character's build, checked against that Campaign's rules, and starts
// its own progress at first level; a Character plays once per Campaign.
func (s *Characters) Join(ctx context.Context, c caller.Caller, id domain.OwnedID, campaign domain.CampaignID) (Sheet, error) {
	o, err := s.Owned(ctx, c, id)
	if err != nil {
		return Sheet{}, err
	}
	if len(o.Campaigns) == 0 {
		return Sheet{}, refuse("this Character has no build to bring yet; create it in a Campaign first")
	}
	for _, e := range o.Campaigns {
		if e.CampaignID == campaign {
			return Sheet{}, domain.ErrConflict
		}
	}
	latest := o.Campaigns[0]
	source, err := s.Repo.Character(ctx, latest.CampaignID, latest.CharacterID)
	if err != nil {
		return Sheet{}, err
	}
	b := source.Build
	b.Name = o.Name
	sheet, err := s.prepare(ctx, c, campaign, b)
	if err != nil {
		return Sheet{}, err
	}
	sheet.Owned = id
	sheet.Portrait, sheet.Token = source.Portrait, source.Token
	cid, err := s.Repo.InsertCharacter(ctx, sheet.Character, s.Now())
	if err != nil {
		return Sheet{}, err
	}
	sheet.ID = cid
	return sheet, nil
}
