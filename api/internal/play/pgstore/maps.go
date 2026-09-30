package pgstore

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

var _ app.MapRepository = (*Store)(nil)

// InsertMap stores a new Map.
func (s *Store) InsertMap(ctx context.Context, m domain.Map, now time.Time) (domain.Map, error) {
	row, err := s.q.InsertMap(ctx, queries.InsertMapParams{
		CampaignID: m.CampaignID, Name: m.Name, ImageKey: m.ImageKey, ImageType: m.ImageType, WidthPx: int32(m.Width), HeightPx: int32(m.Height), //nolint:gosec // capped pixels
		HexSizePx: m.HexSize, OriginX: m.OriginX, OriginY: m.OriginY, Now: now,
	})
	if err != nil {
		return domain.Map{}, err
	}
	return mapRow(row), nil
}

// GetMap reads one Map.
func (s *Store) GetMap(ctx context.Context, campaign uuid.UUID, id domain.MapID) (domain.Map, error) {
	row, err := s.q.GetMap(ctx, queries.GetMapParams{CampaignID: campaign, ID: uuid.UUID(id)})
	if err != nil {
		return domain.Map{}, notFound(err)
	}
	return mapRow(row), nil
}

// Maps lists a Campaign's Maps.
func (s *Store) Maps(ctx context.Context, campaign uuid.UUID) ([]domain.Map, error) {
	rows, err := s.q.ListMaps(ctx, campaign)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Map, 0, len(rows))
	for _, r := range rows {
		out = append(out, mapRow(r))
	}
	return out, nil
}

// UpdateMap stores a Map's name, calibration and ambient light.
func (s *Store) UpdateMap(ctx context.Context, m domain.Map, now time.Time) error {
	n, err := s.q.UpdateMap(ctx, queries.UpdateMapParams{
		CampaignID: m.CampaignID, ID: uuid.UUID(m.ID), Name: m.Name, HexSizePx: m.HexSize, OriginX: m.OriginX, OriginY: m.OriginY,
		Ambient: m.Ambient, Now: now,
	})
	if err == nil && n == 0 {
		return apperr.ErrNotFound
	}
	return err
}
