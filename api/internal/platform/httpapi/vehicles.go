package httpapi

import (
	"context"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	playapp "github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/vehicles"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// VehicleService keeps a Campaign's vehicles and ships.
type VehicleService interface {
	List(ctx context.Context, c caller.Caller, campaign uuid.UUID) (playapp.VehiclesView, error)
	Add(ctx context.Context, c caller.Caller, campaign uuid.UUID, d vehicles.Design) (playdomain.Vehicle, error)
	Remove(ctx context.Context, c caller.Caller, campaign, vehicle uuid.UUID) error
	Strike(ctx context.Context, c caller.Caller, campaign, vehicle uuid.UUID, b playapp.VehicleBlow) (playdomain.Vehicle, error)
	Post(ctx context.Context, c caller.Caller, campaign, vehicle, station uuid.UUID, posted int) (playdomain.Vehicle, error)
}

//nolint:gosec // a vehicle is bounded by the rules
func vehicleOut(v playdomain.Vehicle) oas.Vehicle {
	out := oas.Vehicle{
		ID: oas.ID(v.ID), Name: v.Name, Kind: oas.VehicleKind(v.Kind), Hull: int32(v.Hull), HullMax: int32(v.HullMax), Threshold: int32(v.Threshold),
		MilesPerDay: int32(v.MilesPerDay), Speed: int32(v.Speed()), ShortHanded: v.State().ShortHanded,
		Components: make([]oas.VehicleComponent, 0, len(v.Parts)), Stations: make([]oas.VehicleStation, 0, len(v.Posts)),
	}
	for _, p := range v.Parts {
		out.Components = append(out.Components, oas.VehicleComponent{ID: oas.ID(p.ID), Name: p.Name, Hp: int32(p.HP), HpMax: int32(p.HPMax), Drives: p.Drives})
	}
	for _, p := range v.Posts {
		out.Stations = append(out.Stations, oas.VehicleStation{ID: oas.ID(p.ID), Name: p.Name, Crew: int32(p.Crew), Posted: int32(p.Posted)})
	}
	return out
}

// ListVehicles shows the Campaign's vehicles.
func (h *Handler) ListVehicles(ctx context.Context, p oas.ListVehiclesParams) (oas.ListVehiclesRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	v, err := h.Vehicles.List(ctx, c, uuid.UUID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "list vehicles", err), nil
	}
	out := oas.Vehicles{Dm: v.DM, Vehicles: make([]oas.Vehicle, 0, len(v.Vehicles))}
	for _, one := range v.Vehicles {
		out.Vehicles = append(out.Vehicles, vehicleOut(one))
	}
	return &oas.VehiclesHeaders{Response: out}, nil
}

// CreateVehicle builds a vehicle.
func (h *Handler) CreateVehicle(ctx context.Context, req *oas.VehicleInput, p oas.CreateVehicleParams) (oas.CreateVehicleRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	d := vehicles.Design{
		Name: req.Name, Kind: string(req.Kind), HullMax: int(req.HullMax), Threshold: int(req.Threshold.Or(0)), MilesPerDay: int(req.MilesPerDay),
		Components: make([]vehicles.Component, 0, len(req.Components)), Stations: make([]vehicles.Station, 0, len(req.Stations)),
	}
	for _, part := range req.Components {
		d.Components = append(d.Components, vehicles.Component{Name: part.Name, HPMax: int(part.HpMax), Drives: part.Drives.Or(false)})
	}
	for _, post := range req.Stations {
		d.Stations = append(d.Stations, vehicles.Station{Name: post.Name, Crew: int(post.Crew)})
	}
	made, err := h.Vehicles.Add(ctx, c, uuid.UUID(p.CampaignId), d)
	if err != nil {
		return h.campaignProblem(ctx, "create vehicle", err), nil
	}
	return &oas.VehicleHeaders{Response: vehicleOut(made)}, nil
}

// DeleteVehicle removes a vehicle.
func (h *Handler) DeleteVehicle(ctx context.Context, p oas.DeleteVehicleParams) (oas.DeleteVehicleRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Vehicles.Remove(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(p.VehicleId)); err != nil {
		return h.campaignProblem(ctx, "delete vehicle", err), nil
	}
	return &oas.DeleteVehicleNoContent{}, nil
}

// DamageVehicle damages or repairs a vehicle's hull or one of its components.
func (h *Handler) DamageVehicle(ctx context.Context, req *oas.VehicleBlow, p oas.DamageVehicleParams) (oas.DamageVehicleRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	b := playapp.VehicleBlow{Part: nil, Amount: int(req.Amount), Repair: req.Repair.Or(false)}
	if id, set := req.ComponentId.Get(); set {
		part := uuid.UUID(id)
		b.Part = &part
	}
	v, err := h.Vehicles.Strike(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(p.VehicleId), b)
	if err != nil {
		return h.campaignProblem(ctx, "damage vehicle", err), nil
	}
	return &oas.VehicleHeaders{Response: vehicleOut(v)}, nil
}

// PostVehicleCrew posts crew to a station.
func (h *Handler) PostVehicleCrew(ctx context.Context, req *oas.VehicleCrew, p oas.PostVehicleCrewParams) (oas.PostVehicleCrewRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	v, err := h.Vehicles.Post(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(p.VehicleId), uuid.UUID(p.StationId), int(req.Posted))
	if err != nil {
		return h.campaignProblem(ctx, "post vehicle crew", err), nil
	}
	return &oas.VehicleHeaders{Response: vehicleOut(v)}, nil
}
