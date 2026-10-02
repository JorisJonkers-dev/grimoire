package app

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
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
	sheet, err := s.prepare(ctx, c, campaign, b, false)
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

// Draft reads the caller's Character draft in a Campaign.
func (s *Characters) Draft(ctx context.Context, c caller.Caller, id domain.CampaignID) (domain.Draft, error) {
	if _, err := member(ctx, s.Repo, c, id); err != nil {
		return domain.Draft{}, err
	}
	return s.Repo.Draft(ctx, id, c.Subject)
}

// SaveDraft keeps the wizard's step and choices for the caller.
func (s *Characters) SaveDraft(ctx context.Context, c caller.Caller, id domain.CampaignID, step int, build []byte) (domain.Draft, error) {
	if _, err := member(ctx, s.Repo, c, id); err != nil {
		return domain.Draft{}, err
	}
	if step < 0 || step > 8 || len(build) > 16_000 {
		return domain.Draft{}, domain.ErrInvalid
	}
	if err := s.Repo.SaveDraft(ctx, id, c.Subject, step, build, s.Now()); err != nil {
		return domain.Draft{}, err
	}
	return s.Repo.Draft(ctx, id, c.Subject)
}

// DiscardDraft starts the caller's wizard over, rolled scores and all.
func (s *Characters) DiscardDraft(ctx context.Context, c caller.Caller, id domain.CampaignID) error {
	if _, err := member(ctx, s.Repo, c, id); err != nil {
		return err
	}
	return s.Repo.DeleteDraft(ctx, id, c.Subject)
}

// RollScores rolls six ability scores for the caller's draft, once, when the Campaign allows rolling.
func (s *Characters) RollScores(ctx context.Context, c caller.Caller, id domain.CampaignID) (domain.Draft, error) {
	if _, err := member(ctx, s.Repo, c, id); err != nil {
		return domain.Draft{}, err
	}
	camp, err := s.Repo.GetCampaign(ctx, id)
	if err != nil {
		return domain.Draft{}, err
	}
	if s.Roll == nil || !slices.Contains(camp.CreationMethods, string(rules.Rolled)) {
		return domain.Draft{}, refuse("this Campaign does not roll ability scores")
	}
	rolled, err := s.Repo.RollDraft(ctx, id, c.Subject, s.Roll(), s.Now())
	if err != nil {
		return domain.Draft{}, err
	}
	if !rolled {
		return domain.Draft{}, domain.ErrConflict
	}
	return s.Repo.Draft(ctx, id, c.Subject)
}
