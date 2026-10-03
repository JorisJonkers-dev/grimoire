package httpapi

import (
	"context"

	"github.com/google/uuid"

	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// TrackService keeps a Campaign's Tracks and moves their scores.
type TrackService interface {
	List(ctx context.Context, c caller.Caller, id domain.CampaignID) (campaignapp.TracksView, error)
	Create(ctx context.Context, c caller.Caller, id domain.CampaignID, in campaignapp.TrackInput) (domain.Track, error)
	Delete(ctx context.Context, c caller.Caller, id domain.CampaignID, track domain.TrackID) error
	Adjust(ctx context.Context, c caller.Caller, id domain.CampaignID, track domain.TrackID, character *domain.CharacterID, delta int) (campaignapp.TrackAdjusted, error)
}

//nolint:gosec // scores are bounded by the rules
func thresholdsOut(list []domain.TrackThreshold) []oas.TrackThreshold {
	out := make([]oas.TrackThreshold, 0, len(list))
	for _, th := range list {
		row := oas.TrackThreshold{At: int32(th.At), Rising: th.Rising, Label: th.Label}
		if th.Effect != "" {
			row.Effect = oas.NewOptString(th.Effect)
		}
		if th.RollTable != nil {
			row.RollTableId = oas.NewOptID(oas.ID(*th.RollTable))
		}
		out = append(out, row)
	}
	return out
}

// trackOut shapes a Track; its thresholds go to the DM alone.
//
//nolint:gosec // scores are bounded by the rules
func trackOut(t domain.Track, standings []campaignapp.TrackStanding, dm bool) oas.Track {
	out := oas.Track{
		ID: oas.ID(t.ID), Name: t.Name, Scope: oas.TrackScope(t.Scope), Min: int32(t.Min), Max: int32(t.Max), Start: int32(t.Start),
		Standings: make([]oas.TrackStanding, 0, len(standings)),
	}
	if dm {
		out.Thresholds = thresholdsOut(t.Thresholds)
	}
	for _, st := range standings {
		row := oas.TrackStanding{Name: st.Name, Value: int32(st.Value)}
		if st.Character != nil {
			row.CharacterId = oas.NewOptID(oas.ID(*st.Character))
		}
		out.Standings = append(out.Standings, row)
	}
	return out
}

// ListTracks lists the Campaign's Tracks as the caller may see them.
func (h *Handler) ListTracks(ctx context.Context, p oas.ListTracksParams) (oas.ListTracksRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	v, err := h.Tracks.List(ctx, c, domain.CampaignID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "list tracks", err), nil
	}
	out := oas.Tracks{Dm: v.DM, Tracks: make([]oas.Track, 0, len(v.Tracks))}
	for _, row := range v.Tracks {
		out.Tracks = append(out.Tracks, trackOut(row.Track, row.Standings, v.DM))
	}
	return &oas.TracksHeaders{Response: out}, nil
}

// CreateTrack adds a Track.
func (h *Handler) CreateTrack(ctx context.Context, req *oas.TrackInput, p oas.CreateTrackParams) (oas.CreateTrackRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	in := campaignapp.TrackInput{Name: req.Name, Scope: string(req.Scope), Min: int(req.Min), Max: int(req.Max), Start: int(req.Start), Thresholds: make([]domain.TrackThreshold, 0, len(req.Thresholds))}
	for _, th := range req.Thresholds {
		row := domain.TrackThreshold{At: int(th.At), Rising: th.Rising, Label: th.Label, Effect: th.Effect.Or(""), RollTable: nil}
		if id, set := th.RollTableId.Get(); set {
			table := uuid.UUID(id)
			row.RollTable = &table
		}
		in.Thresholds = append(in.Thresholds, row)
	}
	t, err := h.Tracks.Create(ctx, c, domain.CampaignID(p.CampaignId), in)
	if err != nil {
		return h.campaignProblem(ctx, "create track", err), nil
	}
	return &oas.TrackHeaders{Response: trackOut(t, nil, true)}, nil
}

// DeleteTrack removes a Track.
func (h *Handler) DeleteTrack(ctx context.Context, p oas.DeleteTrackParams) (oas.DeleteTrackRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Tracks.Delete(ctx, c, domain.CampaignID(p.CampaignId), uuid.UUID(p.TrackId)); err != nil {
		return h.campaignProblem(ctx, "delete track", err), nil
	}
	return &oas.DeleteTrackNoContent{}, nil
}

// AdjustTrack moves a Track's score.
func (h *Handler) AdjustTrack(ctx context.Context, req *oas.TrackAdjustment, p oas.AdjustTrackParams) (oas.AdjustTrackRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	var character *domain.CharacterID
	if id, set := req.CharacterId.Get(); set {
		ch := domain.CharacterID(id)
		character = &ch
	}
	moved, err := h.Tracks.Adjust(ctx, c, domain.CampaignID(p.CampaignId), uuid.UUID(p.TrackId), character, int(req.Delta))
	if err != nil {
		return h.campaignProblem(ctx, "adjust track", err), nil
	}
	return &oas.TrackAdjustedHeaders{Response: oas.TrackAdjusted{Value: int32(moved.Value), Crossed: thresholdsOut(moved.Crossed)}}, nil //nolint:gosec // scores are bounded by the rules
}
