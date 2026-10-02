package app

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Collections lists the caller's Collections.
func (s *Service) Collections(ctx context.Context, c caller.Caller) ([]domain.Collection, error) {
	return s.Repo.Collections(ctx, c.Subject, nil)
}

// CreateCollection starts an empty Collection in the caller's Library.
func (s *Service) CreateCollection(ctx context.Context, c caller.Caller, name, description string) (domain.Collection, error) {
	name, description, err := domain.CleanCollection(name, description)
	if err != nil {
		return domain.Collection{}, err
	}
	now := s.Now()
	col := domain.Collection{ID: uuid.New(), Owner: c.Subject, Name: name, Description: description, Entries: []uuid.UUID{}, CreatedAt: now, UpdatedAt: now}
	return col, s.Repo.InsertCollection(ctx, col)
}

// ownedCollection reads a Collection the caller owns; anyone else's looks like none at all.
func (s *Service) ownedCollection(ctx context.Context, c caller.Caller, id uuid.UUID) (domain.Collection, error) {
	col, err := s.Repo.Collection(ctx, id)
	if err == nil && col.Owner != c.Subject {
		return domain.Collection{}, apperr.ErrNotFound
	}
	return col, err
}

// UpdateCollection renames one of the caller's Collections and sets which of their entries it holds.
// Every Campaign it is switched on in sees the new set.
func (s *Service) UpdateCollection(ctx context.Context, c caller.Caller, id uuid.UUID, name, description string, entries []uuid.UUID) (domain.Collection, error) {
	col, err := s.ownedCollection(ctx, c, id)
	if err != nil {
		return col, err
	}
	if col.Name, col.Description, err = domain.CleanCollection(name, description); err != nil {
		return col, err
	}
	for _, e := range entries {
		_, err := s.owned(ctx, c, e)
		if errors.Is(err, apperr.ErrNotFound) {
			return col, apperr.Refuse("a Collection holds only entries from your own Library")
		}
		if err != nil {
			return col, err
		}
	}
	col.Entries, col.UpdatedAt = entries, s.Now()
	return col, s.Repo.InTx(ctx, func(r Repository) error { return r.UpdateCollection(ctx, col) })
}

// CampaignCollections lists the caller's Collections and any other switched on in a Campaign they run,
// each saying whether it is on there. DM only.
func (s *Service) CampaignCollections(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.Collection, error) {
	if err := s.dm(ctx, c, campaign); err != nil {
		return nil, err
	}
	return s.Repo.Collections(ctx, c.Subject, &campaign)
}

// Switch turns a Collection on or off in a Campaign the caller runs: on brings its entries in, off hides
// those not linked otherwise. Only its owner switches it on; any DM of the Campaign switches it off.
func (s *Service) Switch(ctx context.Context, c caller.Caller, campaign, collection uuid.UUID, on bool) ([]domain.Collection, error) {
	if err := s.dm(ctx, c, campaign); err != nil {
		return nil, err
	}
	get := s.Repo.Collection
	if on {
		get = func(ctx context.Context, id uuid.UUID) (domain.Collection, error) {
			return s.ownedCollection(ctx, c, id)
		}
	}
	if _, err := get(ctx, collection); err != nil {
		return nil, err
	}
	if err := s.Repo.Switch(ctx, campaign, collection, on, s.Now()); err != nil {
		return nil, err
	}
	return s.Repo.Collections(ctx, c.Subject, &campaign)
}
