package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// activityLimit is how many changes the activity feed shows.
const activityLimit = 50

// Activity lists the latest prep changes made through MCP, newest first. DM only.
func (s *Service) Activity(ctx context.Context, c caller.Caller, id domain.CampaignID) ([]domain.Edit, error) {
	if _, err := dm(ctx, s.Repo, c, id); err != nil {
		return nil, err
	}
	return s.Repo.Edits(ctx, id, domain.EditFilter{Origin: string(caller.OriginMCP), Limit: activityLimit})
}

// LatestEdit is an entity's newest change. DM only.
func (s *Service) LatestEdit(ctx context.Context, c caller.Caller, id domain.CampaignID, t domain.EntityType, entity uuid.UUID) (domain.Edit, error) {
	if _, err := dm(ctx, s.Repo, c, id); err != nil {
		return domain.Edit{}, err
	}
	return s.one(ctx, id, domain.EditFilter{EntityType: t, EntityID: entity, Limit: 1})
}

// UndoPlan says how to undo a change: restore the Revision it returns, or delete the entity when that
// is 0. Only an entity's latest change can be undone. DM only.
func (s *Service) UndoPlan(ctx context.Context, c caller.Caller, id domain.CampaignID, revision uuid.UUID) (domain.Edit, int, error) {
	if _, err := dm(ctx, s.Repo, c, id); err != nil {
		return domain.Edit{}, 0, err
	}
	e, err := s.one(ctx, id, domain.EditFilter{RevisionID: revision, Limit: 1})
	switch {
	case err != nil:
		return domain.Edit{}, 0, err
	case !e.Latest:
		return domain.Edit{}, 0, apperr.Refuse("Undo the later changes to " + e.Name + " first.")
	case e.No == 1:
		return e, 0, nil
	}
	both, err := s.Repo.Edits(ctx, id, domain.EditFilter{EntityType: e.EntityType, EntityID: e.EntityID, Limit: 2})
	if err != nil {
		return domain.Edit{}, 0, err
	}
	if both[1].Action == domain.ActionDelete {
		return e, 0, nil
	}
	return e, e.No - 1, nil
}

func (s *Service) one(ctx context.Context, id domain.CampaignID, f domain.EditFilter) (domain.Edit, error) {
	edits, err := s.Repo.Edits(ctx, id, f)
	if err != nil {
		return domain.Edit{}, err
	}
	if len(edits) == 0 {
		return domain.Edit{}, domain.ErrNotFound
	}
	return edits[0], nil
}
