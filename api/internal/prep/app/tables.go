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

// Tables lists the Campaign's Encounter Tables. DM only.
func (s *Service) Tables(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.Table, error) {
	if _, err := s.dm(ctx, c, campaign); err != nil {
		return nil, err
	}
	return s.Repo.Tables(ctx, campaign)
}

// Locations lists the places on the Campaign's world maps a Table can belong to. DM only.
func (s *Service) Locations(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.Location, error) {
	if _, err := s.dm(ctx, c, campaign); err != nil {
		return nil, err
	}
	return s.Repo.Locations(ctx, campaign)
}

// Checks lists the Campaign's latest Encounter Checks with their seeds. DM only.
func (s *Service) Checks(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.Check, error) {
	if _, err := s.dm(ctx, c, campaign); err != nil {
		return nil, err
	}
	return s.Repo.Checks(ctx, campaign)
}

// SaveTable creates a Table, or replaces the one with the same id.
func (s *Service) SaveTable(ctx context.Context, c caller.Caller, campaign uuid.UUID, t domain.Table) (domain.Table, error) {
	err := s.write(ctx, c, campaign, func(r Repository, me playdomain.Member, now time.Time) error {
		tables, err := r.Tables(ctx, campaign)
		if err != nil {
			return err
		}
		action := campaigndomain.ActionCreate
		if t.ID != (domain.TableID{}) {
			if !slices.ContainsFunc(tables, func(x domain.Table) bool { return x.ID == t.ID }) {
				return apperr.ErrNotFound
			}
			action = campaigndomain.ActionUpdate
		} else {
			t.ID = domain.TableID(uuid.New())
		}
		if t, err = s.cleanTable(ctx, r, campaign, t); err != nil {
			return err
		}
		t.UpdatedAt = now
		if err := r.SaveTable(ctx, campaign, t, now); err != nil {
			return err
		}
		return r.RecordTable(ctx, campaign, revision(action, me, now), c, t)
	})
	return t, err
}

// DeleteTable removes a Table; its Revisions keep it restorable.
func (s *Service) DeleteTable(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.TableID) error {
	return s.write(ctx, c, campaign, func(r Repository, me playdomain.Member, now time.Time) error {
		tables, err := r.Tables(ctx, campaign)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(tables, func(t domain.Table) bool { return t.ID == id })
		if i < 0 {
			return apperr.ErrNotFound
		}
		if _, err := r.DeleteTable(ctx, campaign, id); err != nil {
			return err
		}
		return r.RecordTable(ctx, campaign, revision(campaigndomain.ActionDelete, me, now), c, tables[i])
	})
}

// TableRevisions lists a Table's Revisions, newest first.
func (s *Service) TableRevisions(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.TableID) ([]campaigndomain.Revision, error) {
	if _, err := s.dm(ctx, c, campaign); err != nil {
		return nil, err
	}
	revs, err := s.Repo.Revisions(ctx, campaign, domain.EntityTable, uuid.UUID(id))
	if err == nil && len(revs) == 0 {
		return nil, apperr.ErrNotFound
	}
	return revs, err
}

// RestoreTable brings a Table back as it was at a Revision, deleted or not. Its Pools and Region must still exist.
func (s *Service) RestoreTable(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.TableID, no int) (domain.Table, error) {
	var out domain.Table
	err := s.write(ctx, c, campaign, func(r Repository, me playdomain.Member, now time.Time) error {
		t, err := r.TableAt(ctx, campaign, id, no)
		if err != nil {
			return err
		}
		if t, err = s.cleanTable(ctx, r, campaign, t); err != nil {
			return err
		}
		t.UpdatedAt = now
		if err := r.SaveTable(ctx, campaign, t, now); err != nil {
			return err
		}
		rev := revision(campaigndomain.ActionRestore, me, now)
		rev.RestoredFrom = no
		out = t
		return r.RecordTable(ctx, campaign, rev, c, t)
	})
	return out, err
}
