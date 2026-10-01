package pgstore

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

// Edits lists a Campaign's prep changes, newest first.
func (s *Store) Edits(ctx context.Context, id domain.CampaignID, f domain.EditFilter) ([]domain.Edit, error) {
	rows, err := s.q.Edits(ctx, queries.EditsParams{
		CampaignID: uuid.UUID(id),
		Origin:     pgtype.Text{String: f.Origin, Valid: f.Origin != ""},
		ID:         pgtype.UUID{Bytes: f.RevisionID, Valid: f.RevisionID != uuid.Nil},
		EntityType: pgtype.Text{String: string(f.EntityType), Valid: f.EntityType != ""},
		EntityID:   pgtype.UUID{Bytes: f.EntityID, Valid: f.EntityID != uuid.Nil},
		Lim:        int32(f.Limit), //nolint:gosec // bounded by the callers
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Edit, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Edit{
			Revision: domain.Revision{
				No: int(r.RevisionNo), Action: domain.RevisionAction(r.Action), Author: r.CallerName, Origin: r.Origin, Client: r.Client,
				RestoredFrom: int(r.RestoredFrom.Int32), CreatedAt: r.CreatedAt,
			},
			RevisionID: r.ID, EntityType: domain.EntityType(r.EntityType), EntityID: r.EntityID, Name: r.Name, Latest: r.Latest,
		})
	}
	return out, nil
}
