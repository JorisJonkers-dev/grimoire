package app

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/vehicles"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// MaxVehicles is how many vehicles a Campaign keeps.
const MaxVehicles = 50

// VehicleStore keeps a Campaign's vehicles.
type VehicleStore interface {
	Vehicles(ctx context.Context, campaign uuid.UUID) ([]domain.Vehicle, error)
	// WriteVehicles runs vehicle changes in one transaction.
	WriteVehicles(ctx context.Context, fn func(VehicleWriter) error) error
}

// VehicleWriter reads and changes vehicles inside a transaction. What a change is worked out from is
// read here, after Lock, so that two blows at once both land.
type VehicleWriter interface {
	// Lock holds the Campaign's vehicles until the transaction ends; a change that comes second waits,
	// and then reads what the first left.
	Lock(ctx context.Context, campaign uuid.UUID) error
	Vehicles(ctx context.Context, campaign uuid.UUID) ([]domain.Vehicle, error)
	InsertVehicle(ctx context.Context, campaign uuid.UUID, v domain.Vehicle, now time.Time) error
	// DeleteVehicle reports false for a vehicle the Campaign does not have.
	DeleteVehicle(ctx context.Context, campaign, vehicle uuid.UUID) (bool, error)
	SetVehicleHull(ctx context.Context, campaign, vehicle uuid.UUID, hp int) error
	SetVehiclePartHP(ctx context.Context, vehicle, part uuid.UUID, hp int) error
	SetVehiclePosted(ctx context.Context, vehicle, station uuid.UUID, posted int) error
}

// Vehicles runs the vehicle use cases: the DM builds vehicles and ships, damages and repairs their
// hulls and components and posts crew to their stations; every Member sees them.
type Vehicles struct {
	Store   VehicleStore
	Members Members
	Now     func() time.Time
}

// VehiclesView is a Campaign's vehicles as the caller may see them.
type VehiclesView struct {
	DM       bool
	Vehicles []domain.Vehicle
}

// List shows a Member the Campaign's vehicles.
func (s *Vehicles) List(ctx context.Context, c caller.Caller, campaign uuid.UUID) (VehiclesView, error) {
	me, err := s.Members.Membership(ctx, campaign, c.Subject)
	if err != nil {
		return VehiclesView{}, err
	}
	list, err := s.Store.Vehicles(ctx, campaign)
	return VehiclesView{DM: me.DM, Vehicles: list}, err
}

func (s *Vehicles) dm(ctx context.Context, c caller.Caller, campaign uuid.UUID) error {
	me, err := s.Members.Membership(ctx, campaign, c.Subject)
	if err == nil && !me.DM {
		err = apperr.ErrForbidden
	}
	return err
}

// Add builds a vehicle, whole and with nobody at its stations. DM only.
func (s *Vehicles) Add(ctx context.Context, c caller.Caller, campaign uuid.UUID, d vehicles.Design) (domain.Vehicle, error) {
	if err := s.dm(ctx, c, campaign); err != nil {
		return domain.Vehicle{}, err
	}
	d.Name = strings.TrimSpace(d.Name)
	if err := d.Check(); err != nil {
		return domain.Vehicle{}, apperr.Refuse(err.Error())
	}
	made := domain.Vehicle{
		ID: uuid.New(), Name: d.Name, Kind: d.Kind, Hull: d.HullMax, HullMax: d.HullMax, Threshold: d.Threshold, MilesPerDay: d.MilesPerDay,
		Parts: make([]domain.VehiclePart, 0, len(d.Components)), Posts: make([]domain.VehiclePost, 0, len(d.Stations)),
	}
	for _, p := range d.Components {
		made.Parts = append(made.Parts, domain.VehiclePart{ID: uuid.New(), Name: p.Name, HP: p.HPMax, HPMax: p.HPMax, Drives: p.Drives})
	}
	for _, p := range d.Stations {
		made.Posts = append(made.Posts, domain.VehiclePost{ID: uuid.New(), Name: p.Name, Crew: p.Crew, Posted: 0})
	}
	return made, s.Store.WriteVehicles(ctx, func(w VehicleWriter) error {
		if err := w.Lock(ctx, campaign); err != nil {
			return err
		}
		list, err := w.Vehicles(ctx, campaign)
		if err != nil {
			return err
		}
		if len(list) >= MaxVehicles {
			return apperr.Refuse("a Campaign keeps up to 50 vehicles")
		}
		return w.InsertVehicle(ctx, campaign, made, s.Now())
	})
}

// Remove takes a vehicle out of the Campaign. DM only.
func (s *Vehicles) Remove(ctx context.Context, c caller.Caller, campaign, vehicle uuid.UUID) error {
	if err := s.dm(ctx, c, campaign); err != nil {
		return err
	}
	return s.Store.WriteVehicles(ctx, func(w VehicleWriter) error {
		found, err := w.DeleteVehicle(ctx, campaign, vehicle)
		if err == nil && !found {
			err = apperr.ErrNotFound
		}
		return err
	})
}

// VehicleBlow is damage to a vehicle's hull, or to one of its components, or the repair of it.
type VehicleBlow struct {
	Part   *uuid.UUID
	Amount int
	Repair bool
}

// change runs one change to a vehicle as it stands, alone with the Campaign's vehicles, and returns
// the vehicle as the change leaves it.
func (s *Vehicles) change(ctx context.Context, c caller.Caller, campaign, vehicle uuid.UUID, fn func(VehicleWriter, *domain.Vehicle) error) (domain.Vehicle, error) {
	if err := s.dm(ctx, c, campaign); err != nil {
		return domain.Vehicle{}, err
	}
	var out domain.Vehicle
	return out, s.Store.WriteVehicles(ctx, func(w VehicleWriter) error {
		if err := w.Lock(ctx, campaign); err != nil {
			return err
		}
		list, err := w.Vehicles(ctx, campaign)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(list, func(v domain.Vehicle) bool { return v.ID == vehicle })
		if i < 0 {
			return apperr.ErrNotFound
		}
		out = list[i]
		return fn(w, &out)
	})
}

// Strike damages a vehicle's hull or one of its components, or repairs it. A blow under the vehicle's
// damage threshold does nothing; a repair stops at what the part can have. DM only.
func (s *Vehicles) Strike(ctx context.Context, c caller.Caller, campaign, vehicle uuid.UUID, b VehicleBlow) (domain.Vehicle, error) {
	if b.Amount < 1 || b.Amount > vehicles.MaxHP {
		return domain.Vehicle{}, apperr.Refuse("damage and repairs run from 1 to 10000 hit points")
	}
	after := func(hp, most, threshold int) int {
		if b.Repair {
			return vehicles.Mend(hp, most, b.Amount)
		}
		return vehicles.Hit(hp, threshold, b.Amount)
	}
	return s.change(ctx, c, campaign, vehicle, func(w VehicleWriter, v *domain.Vehicle) error {
		if b.Part == nil {
			v.Hull = after(v.Hull, v.HullMax, v.Threshold)
			return w.SetVehicleHull(ctx, campaign, v.ID, v.Hull)
		}
		i := slices.IndexFunc(v.Parts, func(p domain.VehiclePart) bool { return p.ID == *b.Part })
		if i < 0 {
			return apperr.ErrNotFound
		}
		v.Parts[i].HP = after(v.Parts[i].HP, v.Parts[i].HPMax, v.Threshold)
		return w.SetVehiclePartHP(ctx, v.ID, v.Parts[i].ID, v.Parts[i].HP)
	})
}

// Post puts crew at a station, up to what it takes. DM only.
func (s *Vehicles) Post(ctx context.Context, c caller.Caller, campaign, vehicle, station uuid.UUID, posted int) (domain.Vehicle, error) {
	return s.change(ctx, c, campaign, vehicle, func(w VehicleWriter, v *domain.Vehicle) error {
		i := slices.IndexFunc(v.Posts, func(p domain.VehiclePost) bool { return p.ID == station })
		if i < 0 {
			return apperr.ErrNotFound
		}
		if posted < 0 || posted > v.Posts[i].Crew {
			return apperr.Refuse("post no more crew than the station takes")
		}
		v.Posts[i].Posted = posted
		return w.SetVehiclePosted(ctx, v.ID, station, posted)
	})
}
