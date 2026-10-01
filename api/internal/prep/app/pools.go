package app

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Pools lists the Campaign's Encounter Pools. DM only.
func (s *Service) Pools(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.Pool, error) {
	if _, err := s.dm(ctx, c, campaign); err != nil {
		return nil, err
	}
	return s.Repo.Pools(ctx, campaign)
}

// SavePool creates a Pool, or replaces the one with the same id.
func (s *Service) SavePool(ctx context.Context, c caller.Caller, campaign uuid.UUID, p domain.Pool) (domain.Pool, error) {
	err := s.write(ctx, c, campaign, func(r Repository, me playdomain.Member, now time.Time) error {
		pools, err := r.Pools(ctx, campaign)
		if err != nil {
			return err
		}
		action := campaigndomain.ActionCreate
		if p.ID != (domain.PoolID{}) {
			if !slices.ContainsFunc(pools, func(x domain.Pool) bool { return x.ID == p.ID }) {
				return apperr.ErrNotFound
			}
			action = campaigndomain.ActionUpdate
		} else {
			p.ID = domain.PoolID(uuid.New())
		}
		if p, err = s.cleanPool(ctx, r, campaign, p); err != nil {
			return err
		}
		p.UpdatedAt = now
		if err := r.SavePool(ctx, campaign, p, now); err != nil {
			return err
		}
		return r.RecordPool(ctx, campaign, revision(action, me, now), c, p)
	})
	return p, err
}

// DeletePool removes a Pool that no Table draws from; its Revisions keep it restorable.
func (s *Service) DeletePool(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.PoolID) error {
	return s.write(ctx, c, campaign, func(r Repository, me playdomain.Member, now time.Time) error {
		pools, err := r.Pools(ctx, campaign)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(pools, func(p domain.Pool) bool { return p.ID == id })
		if i < 0 {
			return apperr.ErrNotFound
		}
		used, err := r.PoolInUse(ctx, id)
		switch {
		case err != nil:
			return err
		case used:
			return apperr.Refuse("a table still draws from this pool")
		}
		if _, err := r.DeletePool(ctx, campaign, id); err != nil {
			return err
		}
		return r.RecordPool(ctx, campaign, revision(campaigndomain.ActionDelete, me, now), c, pools[i])
	})
}

// PoolRevisions lists a Pool's Revisions, newest first.
func (s *Service) PoolRevisions(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.PoolID) ([]campaigndomain.Revision, error) {
	if _, err := s.dm(ctx, c, campaign); err != nil {
		return nil, err
	}
	revs, err := s.Repo.Revisions(ctx, campaign, domain.EntityPool, uuid.UUID(id))
	if err == nil && len(revs) == 0 {
		return nil, apperr.ErrNotFound
	}
	return revs, err
}

// RestorePool brings a Pool back as it was at a Revision, deleted or not.
func (s *Service) RestorePool(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.PoolID, no int) (domain.Pool, error) {
	var out domain.Pool
	err := s.write(ctx, c, campaign, func(r Repository, me playdomain.Member, now time.Time) error {
		p, err := r.PoolAt(ctx, campaign, id, no)
		if err != nil {
			return err
		}
		if p, err = s.cleanPool(ctx, r, campaign, p); err != nil {
			return err
		}
		p.UpdatedAt = now
		if err := r.SavePool(ctx, campaign, p, now); err != nil {
			return err
		}
		rev := revision(campaigndomain.ActionRestore, me, now)
		rev.RestoredFrom = no
		out = p
		return r.RecordPool(ctx, campaign, rev, c, p)
	})
	return out, err
}
