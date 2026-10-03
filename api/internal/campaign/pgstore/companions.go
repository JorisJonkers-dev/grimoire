package pgstore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

func companion(r queries.CampaignCompanion) domain.Companion {
	c := domain.Companion{
		ID: r.ID, CampaignID: domain.CampaignID(r.CampaignID), Name: r.Name, Kind: r.Kind, MonsterSlug: r.MonsterSlug, Controller: nil,
		SharesXP: r.SharesXp, HP: nil, Notes: r.Notes, UpdatedAt: r.UpdatedAt,
	}
	if r.ControllerMemberID.Valid {
		id := domain.MemberID(r.ControllerMemberID.Bytes)
		c.Controller = &id
	}
	if r.HpCurrent.Valid {
		hp := int(r.HpCurrent.Int32)
		c.HP = &hp
	}
	return c
}

func controller(id *domain.MemberID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

// Companions lists a Campaign's Companions by name.
func (s *Store) Companions(ctx context.Context, id domain.CampaignID) ([]domain.Companion, error) {
	rows, err := s.q.ListCompanions(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Companion, 0, len(rows))
	for _, r := range rows {
		out = append(out, companion(r))
	}
	return out, nil
}

// InsertCompanion stores a new Companion.
func (s *Store) InsertCompanion(ctx context.Context, c domain.Companion, now time.Time) error {
	return s.q.InsertCompanion(ctx, queries.InsertCompanionParams{
		ID: c.ID, CampaignID: uuid.UUID(c.CampaignID), Name: c.Name, Kind: c.Kind, MonsterSlug: c.MonsterSlug, ControllerMemberID: controller(c.Controller),
		SharesXp: c.SharesXP, Notes: c.Notes, Now: now,
	})
}

// UpdateCompanion changes a Companion of the Campaign.
func (s *Store) UpdateCompanion(ctx context.Context, c domain.Companion, now time.Time) error {
	n, err := s.q.UpdateCompanion(ctx, queries.UpdateCompanionParams{
		Name: c.Name, Kind: c.Kind, MonsterSlug: c.MonsterSlug, ControllerMemberID: controller(c.Controller), SharesXp: c.SharesXP, Notes: c.Notes,
		Now: now, CampaignID: uuid.UUID(c.CampaignID), ID: c.ID,
	})
	if err == nil && n == 0 {
		return domain.ErrNotFound
	}
	return err
}

// DeleteCompanion lets a Companion of the Campaign go.
func (s *Store) DeleteCompanion(ctx context.Context, id domain.CampaignID, companion domain.CompanionID) error {
	n, err := s.q.DeleteCompanion(ctx, queries.DeleteCompanionParams{CampaignID: uuid.UUID(id), ID: companion})
	if err == nil && n == 0 {
		return domain.ErrNotFound
	}
	return err
}
