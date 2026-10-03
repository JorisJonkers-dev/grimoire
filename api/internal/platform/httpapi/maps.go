package httpapi

import (
	"bytes"
	"context"
	"io"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	playapp "github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// MapService is what the Map operations need.
type MapService interface {
	Upload(ctx context.Context, c caller.Caller, campaign uuid.UUID, name, kind string, data []byte) (playdomain.Map, error)
	List(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]playdomain.Map, error)
	Get(ctx context.Context, c caller.Caller, campaign uuid.UUID, id playdomain.MapID) (playdomain.Map, error)
	Update(ctx context.Context, c caller.Caller, campaign uuid.UUID, id playdomain.MapID, e playapp.MapEdit) (playdomain.Map, error)
	Calibrate(ctx context.Context, c caller.Caller, campaign uuid.UUID, id playdomain.MapID, k playapp.Calibration) (playdomain.Map, error)
	UseDefaultWorld(ctx context.Context, c caller.Caller, campaign uuid.UUID) (playdomain.Map, error)
	Image(ctx context.Context, c caller.Caller, campaign uuid.UUID, id playdomain.MapID) (string, []byte, error)
}

func mapOut(m playdomain.Map) oas.LocalMap {
	return oas.LocalMap{
		ID: oas.ID(m.ID), Name: m.Name, Kind: oas.MapKind(m.Kind), Width: int32(m.Width), Height: int32(m.Height), HexSizePx: m.HexSize, //nolint:gosec // capped pixels
		OriginX: m.OriginX, OriginY: m.OriginY, Ambient: oas.AmbientLight(m.Ambient),
		GridKind: oas.GridKind(m.Grid), GridStrength: oas.GridStrength(m.GridStrength), ScaleMiles: oas.ScaleMiles(m.ScaleMiles), Found: m.Found, //nolint:gosec // 0 to 100
		ImageUrl: oas.AssetUrl("/api/v1/campaigns/" + m.CampaignID.String() + "/maps/" + uuid.UUID(m.ID).String() + "/image"),
	}
}

// ListMaps lists the Campaign's Maps.
func (h *Handler) ListMaps(ctx context.Context, p oas.ListMapsParams) (oas.ListMapsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	list, err := h.Maps.List(ctx, c, uuid.UUID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "list maps", err), nil
	}
	out := make([]oas.LocalMap, 0, len(list))
	for _, m := range list {
		out = append(out, mapOut(m))
	}
	return &oas.ListMapsOKHeaders{Response: out}, nil
}

// UploadMap stores a new Map picture.
func (h *Handler) UploadMap(ctx context.Context, req oas.UploadMapReq, p oas.UploadMapParams) (oas.UploadMapRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	data, _ := io.ReadAll(io.LimitReader(req.Data, playapp.MaxMapBytes+1)) // a truncated body fails as an unreadable picture
	m, err := h.Maps.Upload(ctx, c, uuid.UUID(p.CampaignId), p.Name, string(p.Kind.Or(oas.MapKindLocal)), data)
	if err != nil {
		return h.campaignProblem(ctx, "upload map", err), nil
	}
	return &oas.LocalMapHeaders{Response: mapOut(m)}, nil
}

// GetMap returns one Map.
func (h *Handler) GetMap(ctx context.Context, p oas.GetMapParams) (oas.GetMapRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	m, err := h.Maps.Get(ctx, c, uuid.UUID(p.CampaignId), playdomain.MapID(p.MapId))
	if err != nil {
		return h.campaignProblem(ctx, "get map", err), nil
	}
	return &oas.LocalMapHeaders{Response: mapOut(m)}, nil
}

// UpdateMap calibrates a Map.
func (h *Handler) UpdateMap(ctx context.Context, req *oas.MapEdit, p oas.UpdateMapParams) (oas.UpdateMapRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	edit := playapp.MapEdit{
		Name: req.Name, HexSize: req.HexSizePx, OriginX: req.OriginX, OriginY: req.OriginY, Ambient: string(req.Ambient), Grid: string(req.GridKind.Value),
	}
	if v, ok := req.GridStrength.Get(); ok {
		strength := int(v)
		edit.GridStrength = &strength
	}
	if v, ok := req.ScaleMiles.Get(); ok {
		miles := float64(v)
		edit.ScaleMiles = &miles
	}
	m, err := h.Maps.Update(ctx, c, uuid.UUID(p.CampaignId), playdomain.MapID(p.MapId), edit)
	if err != nil {
		return h.campaignProblem(ctx, "update map", err), nil
	}
	return &oas.LocalMapHeaders{Response: mapOut(m)}, nil
}

// CalibrateMap sizes a Map's grid from two points a known distance apart.
func (h *Handler) CalibrateMap(ctx context.Context, req *oas.MapCalibration, p oas.CalibrateMapParams) (oas.CalibrateMapRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	m, err := h.Maps.Calibrate(ctx, c, uuid.UUID(p.CampaignId), playdomain.MapID(p.MapId), playapp.Calibration{
		A: hex.Point{X: req.Ax, Y: req.Ay}, B: hex.Point{X: req.Bx, Y: req.By}, Distance: req.Distance,
	})
	if err != nil {
		return h.campaignProblem(ctx, "calibrate map", err), nil
	}
	return &oas.LocalMapHeaders{Response: mapOut(m)}, nil
}

// UseDefaultWorld gives the Campaign the painted Default World.
func (h *Handler) UseDefaultWorld(ctx context.Context, p oas.UseDefaultWorldParams) (oas.UseDefaultWorldRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	m, err := h.Maps.UseDefaultWorld(ctx, c, uuid.UUID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "use the default world", err), nil
	}
	return &oas.LocalMapHeaders{Response: mapOut(m)}, nil
}

// GetMapImage serves the picture, masked for anyone but the DM.
func (h *Handler) GetMapImage(ctx context.Context, p oas.GetMapImageParams) (oas.GetMapImageRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	contentType, data, err := h.Maps.Image(ctx, c, uuid.UUID(p.CampaignId), playdomain.MapID(p.MapId))
	if err != nil {
		return h.campaignProblem(ctx, "map image", err), nil
	}
	cache, body := oas.NewOptString("private, max-age=86400"), bytes.NewReader(data)
	switch contentType {
	case "image/jpeg":
		return &oas.GetMapImageOKImageJpegHeaders{CacheControl: cache, Response: oas.GetMapImageOKImageJpeg{Data: body}}, nil
	case "image/webp":
		return &oas.GetMapImageOKImageWEBPHeaders{CacheControl: cache, Response: oas.GetMapImageOKImageWEBP{Data: body}}, nil
	default:
		return &oas.GetMapImageOKImagePNGHeaders{CacheControl: cache, Response: oas.GetMapImageOKImagePNG{Data: body}}, nil
	}
}
