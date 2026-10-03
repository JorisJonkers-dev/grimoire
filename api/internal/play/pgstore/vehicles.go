package pgstore

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
)

// Vehicles reads a Campaign's vehicles with their components and crew stations, oldest first.
func (s *Store) Vehicles(ctx context.Context, campaign uuid.UUID) ([]domain.Vehicle, error) {
	rows, err := s.q.ListVehicles(ctx, campaign)
	if err != nil {
		return nil, err
	}
	parts, err := s.q.ListVehicleComponents(ctx, campaign)
	if err != nil {
		return nil, err
	}
	posts, err := s.q.ListVehicleStations(ctx, campaign)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Vehicle, 0, len(rows))
	for _, r := range rows {
		v := domain.Vehicle{
			ID: r.ID, Name: r.Name, Kind: r.Kind, Hull: int(r.HullHp), HullMax: int(r.HullMax), Threshold: int(r.Threshold), MilesPerDay: int(r.MilesPerDay),
			Parts: []domain.VehiclePart{}, Posts: []domain.VehiclePost{},
		}
		for _, p := range parts {
			if p.VehicleID == r.ID {
				v.Parts = append(v.Parts, domain.VehiclePart{ID: p.ID, Name: p.Name, HP: int(p.Hp), HPMax: int(p.HpMax), Drives: p.Drives})
			}
		}
		for _, p := range posts {
			if p.VehicleID == r.ID {
				v.Posts = append(v.Posts, domain.VehiclePost{ID: p.ID, Name: p.Name, Crew: int(p.Crew), Posted: int(p.Posted)})
			}
		}
		out = append(out, v)
	}
	return out, nil
}

// WriteVehicles runs vehicle changes in one transaction.
func (s *Store) WriteVehicles(ctx context.Context, fn func(app.VehicleWriter) error) error {
	return s.InTx(ctx, func(r app.Repository) error { return fn(r.(*Store)) })
}

// InsertVehicle adds a vehicle with its components and crew stations.
//
//nolint:gosec // a vehicle is bounded by the rules
func (s *Store) InsertVehicle(ctx context.Context, campaign uuid.UUID, v domain.Vehicle, now time.Time) error {
	err := s.q.InsertVehicle(ctx, queries.InsertVehicleParams{
		ID: v.ID, CampaignID: campaign, Name: v.Name, Kind: v.Kind, HullMax: int32(v.HullMax), Threshold: int32(v.Threshold), MilesPerDay: int32(v.MilesPerDay), Now: now,
	})
	if err != nil {
		return err
	}
	for i, p := range v.Parts {
		if err := s.q.InsertVehicleComponent(ctx, queries.InsertVehicleComponentParams{ID: p.ID, VehicleID: v.ID, Position: int32(i), Name: p.Name, HpMax: int32(p.HPMax), Drives: p.Drives}); err != nil {
			return err
		}
	}
	for i, p := range v.Posts {
		if err := s.q.InsertVehicleStation(ctx, queries.InsertVehicleStationParams{ID: p.ID, VehicleID: v.ID, Position: int32(i), Name: p.Name, Crew: int32(p.Crew)}); err != nil {
			return err
		}
	}
	return nil
}

// DeleteVehicle removes a vehicle; it reports false for one the Campaign does not have.
func (s *Store) DeleteVehicle(ctx context.Context, campaign, vehicle uuid.UUID) (bool, error) {
	n, err := s.q.DeleteVehicle(ctx, queries.DeleteVehicleParams{CampaignID: campaign, ID: vehicle})
	return n > 0, err
}

// SetVehicleHull keeps the hit points a vehicle's hull has left.
//
//nolint:gosec // hit points are bounded by the rules
func (s *Store) SetVehicleHull(ctx context.Context, campaign, vehicle uuid.UUID, hp int) error {
	return s.q.SetVehicleHull(ctx, queries.SetVehicleHullParams{CampaignID: campaign, ID: vehicle, HullHp: int32(hp)})
}

// SetVehiclePartHP keeps the hit points a component has left.
//
//nolint:gosec // hit points are bounded by the rules
func (s *Store) SetVehiclePartHP(ctx context.Context, vehicle, part uuid.UUID, hp int) error {
	return s.q.SetVehicleComponentHP(ctx, queries.SetVehicleComponentHPParams{VehicleID: vehicle, ID: part, Hp: int32(hp)})
}

// SetVehiclePosted keeps how many crew are posted at a station.
//
//nolint:gosec // crew is bounded by the rules
func (s *Store) SetVehiclePosted(ctx context.Context, vehicle, station uuid.UUID, posted int) error {
	return s.q.SetVehicleStationPosted(ctx, queries.SetVehicleStationPostedParams{VehicleID: vehicle, ID: station, Posted: int32(posted)})
}
