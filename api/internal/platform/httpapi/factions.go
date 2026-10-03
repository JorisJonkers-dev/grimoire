package httpapi

import (
	"context"

	"github.com/google/uuid"

	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/standing"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// FactionService keeps a Campaign's Factions and how they regard the party.
type FactionService interface {
	List(ctx context.Context, c caller.Caller, id domain.CampaignID) ([]campaignapp.FactionView, error)
	Create(ctx context.Context, c caller.Caller, id domain.CampaignID, in campaignapp.FactionInput) (campaignapp.FactionView, error)
	Update(ctx context.Context, c caller.Caller, id domain.CampaignID, faction domain.FactionID, in campaignapp.FactionInput) error
	Delete(ctx context.Context, c caller.Caller, id domain.CampaignID, faction domain.FactionID) error
	Propose(ctx context.Context, c caller.Caller, id domain.CampaignID, faction domain.FactionID, g campaignapp.Suggestion) (domain.StandingChange, error)
	Decide(ctx context.Context, c caller.Caller, id domain.CampaignID, change domain.ChangeID, d campaignapp.Decision) error
}

//nolint:gosec // scores and deltas run from -100 to 100
func changeOut(ch campaignapp.Change) oas.StandingChange {
	out := oas.StandingChange{ID: oas.ID(ch.ID), Rose: ch.Rose, Reason: ch.Reason, Status: oas.StandingChangeStatus(ch.Status), CreatedAt: ch.CreatedAt.UTC()}
	if ch.CharacterID != nil {
		out.CharacterId = oas.NewOptID(oas.ID(*ch.CharacterID))
	}
	if ch.DM != nil {
		out.Dm = oas.NewOptStandingChangeSecrets(oas.StandingChangeSecrets{
			Delta: int32(ch.DM.Delta), ShareReason: ch.DM.ShareReason, Origin: oas.StandingChangeSecretsOrigin(ch.DM.Origin), Client: ch.DM.Client,
		})
	}
	return out
}

//nolint:gosec // scores run from -100 to 100
func factionOut(v campaignapp.FactionView) oas.Faction {
	out := oas.Faction{
		ID: oas.ID(v.Faction.ID), Name: v.Faction.Name, Archetype: v.Faction.Archetype, Tier: oas.StandingTier(v.Tier),
		Personal: make([]oas.PersonalStanding, 0, len(v.Personal)), Changes: make([]oas.StandingChange, 0, len(v.Changes)),
	}
	if v.DM {
		out.Dm = oas.NewOptFactionSecrets(oas.FactionSecrets{Goals: v.Faction.Goals, Territory: v.Faction.Territory, Notes: v.Faction.Notes, Score: int32(v.Faction.Score)})
	}
	for _, p := range v.Personal {
		row := oas.PersonalStanding{CharacterId: oas.ID(p.CharacterID), Character: p.Character, Tier: oas.StandingTier(p.Tier)}
		if p.Score != nil {
			row.Score = oas.NewOptInt32(int32(*p.Score))
		}
		out.Personal = append(out.Personal, row)
	}
	for _, ch := range v.Changes {
		out.Changes = append(out.Changes, changeOut(ch))
	}
	return out
}

func factionIn(req *oas.FactionInput) campaignapp.FactionInput {
	return campaignapp.FactionInput{Name: req.Name, Archetype: req.Archetype.Or(""), Goals: req.Goals.Or(""), Territory: req.Territory.Or(""), Notes: req.Notes.Or("")}
}

// ListFactionArchetypes lists the catalogue of generic Factions.
func (h *Handler) ListFactionArchetypes(ctx context.Context) (oas.ListFactionArchetypesRes, error) {
	if _, ok := uiCaller(ctx); !ok {
		return unauthorized(), nil
	}
	all := standing.Archetypes()
	out := make([]oas.FactionArchetype, 0, len(all))
	for _, a := range all {
		out = append(out, oas.FactionArchetype{Slug: oas.Slug(a.Slug), Name: a.Name, Description: a.Description, Goals: a.Goals})
	}
	return &oas.ListFactionArchetypesOKHeaders{Response: out}, nil
}

// ListFactions lists the Campaign's Factions as the caller may see them.
func (h *Handler) ListFactions(ctx context.Context, p oas.ListFactionsParams) (oas.ListFactionsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	list, err := h.Factions.List(ctx, c, domain.CampaignID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "list factions", err), nil
	}
	out := make([]oas.Faction, 0, len(list))
	for _, v := range list {
		out = append(out, factionOut(v))
	}
	return &oas.ListFactionsOKHeaders{Response: out}, nil
}

// CreateFaction adds a Faction.
func (h *Handler) CreateFaction(ctx context.Context, req *oas.FactionInput, p oas.CreateFactionParams) (oas.CreateFactionRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	v, err := h.Factions.Create(ctx, c, domain.CampaignID(p.CampaignId), factionIn(req))
	if err != nil {
		return h.campaignProblem(ctx, "create faction", err), nil
	}
	return &oas.FactionHeaders{Response: factionOut(v)}, nil
}

// UpdateFaction changes a Faction.
func (h *Handler) UpdateFaction(ctx context.Context, req *oas.FactionInput, p oas.UpdateFactionParams) (oas.UpdateFactionRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Factions.Update(ctx, c, domain.CampaignID(p.CampaignId), uuid.UUID(p.FactionId), factionIn(req)); err != nil {
		return h.campaignProblem(ctx, "update faction", err), nil
	}
	return &oas.UpdateFactionNoContent{}, nil
}

// DeleteFaction removes a Faction.
func (h *Handler) DeleteFaction(ctx context.Context, p oas.DeleteFactionParams) (oas.DeleteFactionRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Factions.Delete(ctx, c, domain.CampaignID(p.CampaignId), uuid.UUID(p.FactionId)); err != nil {
		return h.campaignProblem(ctx, "delete faction", err), nil
	}
	return &oas.DeleteFactionNoContent{}, nil
}

// ProposeStandingChange suggests a Standing Change, which then waits for the DM.
func (h *Handler) ProposeStandingChange(ctx context.Context, req *oas.StandingChangeInput, p oas.ProposeStandingChangeParams) (oas.ProposeStandingChangeRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	g := campaignapp.Suggestion{Character: nil, Delta: int(req.Delta), Reason: req.Reason, ShareReason: req.ShareReason.Or(false)}
	if id, ok := req.CharacterId.Get(); ok {
		character := domain.CharacterID(id)
		g.Character = &character
	}
	ch, err := h.Factions.Propose(ctx, c, domain.CampaignID(p.CampaignId), uuid.UUID(p.FactionId), g)
	if err != nil {
		return h.campaignProblem(ctx, "propose standing change", err), nil
	}
	return &oas.StandingChangeHeaders{Response: changeOut(campaignapp.Change{
		ID: ch.ID, CharacterID: ch.Character, Rose: ch.Delta > 0, Reason: ch.Reason, Status: ch.Status, CreatedAt: ch.CreatedAt,
		DM: &campaignapp.ChangeDetail{Delta: ch.Delta, ShareReason: ch.ShareReason, Origin: ch.Origin, Client: ch.Client},
	})}, nil
}

// DecideStandingChange confirms or dismisses a pending Standing Change.
func (h *Handler) DecideStandingChange(ctx context.Context, req *oas.StandingDecision, p oas.DecideStandingChangeParams) (oas.DecideStandingChangeRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	d := campaignapp.Decision{Confirm: req.Confirm, Delta: nil, Reason: nil, ShareReason: nil}
	if v, ok := req.Delta.Get(); ok {
		delta := int(v)
		d.Delta = &delta
	}
	if v, ok := req.Reason.Get(); ok {
		d.Reason = &v
	}
	if v, ok := req.ShareReason.Get(); ok {
		d.ShareReason = &v
	}
	if err := h.Factions.Decide(ctx, c, domain.CampaignID(p.CampaignId), uuid.UUID(p.ChangeId), d); err != nil {
		return h.campaignProblem(ctx, "decide standing change", err), nil
	}
	return &oas.DecideStandingChangeNoContent{}, nil
}
