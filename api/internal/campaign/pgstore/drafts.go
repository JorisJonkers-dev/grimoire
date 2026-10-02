package pgstore

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

// Draft reads a player's Character draft in a Campaign.
func (s *Store) Draft(ctx context.Context, campaign domain.CampaignID, subject string) (domain.Draft, error) {
	r, err := s.q.GetCharacterDraft(ctx, queries.GetCharacterDraftParams{CampaignID: uuid.UUID(campaign), OwnerSubject: subject})
	if err != nil {
		return domain.Draft{}, notFound(err)
	}
	d := domain.Draft{Step: int(r.Step), Build: r.Build, Rolled: nil, UpdatedAt: r.UpdatedAt}
	for _, v := range r.Rolled {
		d.Rolled = append(d.Rolled, int(v))
	}
	return d, nil
}

// SaveDraft keeps a draft's step and choices, leaving its rolled scores alone.
func (s *Store) SaveDraft(ctx context.Context, campaign domain.CampaignID, subject string, step int, build []byte, now time.Time) error {
	return s.q.SaveCharacterDraft(ctx, queries.SaveCharacterDraftParams{CampaignID: uuid.UUID(campaign), OwnerSubject: subject, Step: int32(step), Build: build, Now: now}) //nolint:gosec // 0 to 8
}

// RollDraft stores the scores rolled for a draft; false when it already has some.
func (s *Store) RollDraft(ctx context.Context, campaign domain.CampaignID, subject string, rolled []int, now time.Time) (bool, error) {
	scores := make([]int32, 0, len(rolled))
	for _, v := range rolled {
		scores = append(scores, int32(v)) //nolint:gosec // 3 to 18
	}
	n, err := s.q.RollCharacterDraft(ctx, queries.RollCharacterDraftParams{CampaignID: uuid.UUID(campaign), OwnerSubject: subject, Rolled: scores, Now: now})
	return n == 1, err
}

// DeleteDraft drops a player's draft once the Character is made or started over.
func (s *Store) DeleteDraft(ctx context.Context, campaign domain.CampaignID, subject string) error {
	return s.q.DeleteCharacterDraft(ctx, queries.DeleteCharacterDraftParams{CampaignID: uuid.UUID(campaign), OwnerSubject: subject})
}
