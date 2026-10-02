package httpapi

import (
	"context"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
)

// RequestRetrain proposes a rebuilt build for the DM to approve.
func (h *Handler) RequestRetrain(ctx context.Context, req *oas.RetrainRequest, p oas.RequestRetrainParams) (oas.RequestRetrainRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	b := req.Build
	in := app.RetrainInput{
		Species: string(b.Species), Background: string(b.Background), Method: string(b.Method), Base: baseMap(b.Base), Bonus: bonusMap(b.Bonus),
		Skills: slugList(b.Skills), Picks: make([]domain.Pick, 0, len(b.Picks)), Increase: increaseMap(b.Increase), Reason: req.Reason,
	}
	for _, pk := range b.Picks {
		in.Picks = append(in.Picks, domain.Pick{Level: int(pk.Level), Choice: string(pk.Choice), Value: string(pk.Value)})
	}
	r, err := h.Characters.RequestRetrain(ctx, c, domain.CampaignID(p.CampaignId), domain.CharacterID(p.CharacterId), in)
	if err != nil {
		return h.campaignProblem(ctx, "request retrain", err), nil
	}
	return &oas.RetrainHeaders{Response: retrainOut(r)}, nil
}

// ListRetrains lists a Character's retrains.
func (h *Handler) ListRetrains(ctx context.Context, p oas.ListRetrainsParams) (oas.ListRetrainsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	rs, err := h.Characters.Retrains(ctx, c, domain.CampaignID(p.CampaignId), domain.CharacterID(p.CharacterId))
	if err != nil {
		return h.campaignProblem(ctx, "list retrains", err), nil
	}
	out := make([]oas.Retrain, 0, len(rs))
	for _, r := range rs {
		out = append(out, retrainOut(r))
	}
	return &oas.ListRetrainsOKHeaders{Response: out}, nil
}

// ListRetrainChoices lists the picks a retrain can change, with their options.
func (h *Handler) ListRetrainChoices(ctx context.Context, p oas.ListRetrainChoicesParams) (oas.ListRetrainChoicesRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	choices, err := h.Characters.RetrainChoices(ctx, c, domain.CampaignID(p.CampaignId), domain.CharacterID(p.CharacterId))
	if err != nil {
		return h.campaignProblem(ctx, "retrain choices", err), nil
	}
	out := make([]oas.RetrainChoice, 0, len(choices))
	for _, ch := range choices {
		line := oas.RetrainChoice{Level: int32(ch.Level), Choice: oas.Slug(ch.Choice), Name: ch.Name, Value: oas.Slug(ch.Value), Options: make([]oas.LevelUpOption, 0, len(ch.Options))} //nolint:gosec // 1 to 20
		for _, o := range ch.Options {
			line.Options = append(line.Options, oas.LevelUpOption{Slug: oas.Slug(o.Slug), Name: o.Name, Unmet: nonNil(o.Unmet)})
		}
		out = append(out, line)
	}
	return &oas.ListRetrainChoicesOKHeaders{Response: out}, nil
}

// ListCharacterRevisions lists the builds a Character's retrains replaced.
func (h *Handler) ListCharacterRevisions(ctx context.Context, p oas.ListCharacterRevisionsParams) (oas.ListCharacterRevisionsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	revs, err := h.Characters.CharacterRevisions(ctx, c, domain.CampaignID(p.CampaignId), domain.CharacterID(p.CharacterId))
	if err != nil {
		return h.campaignProblem(ctx, "character revisions", err), nil
	}
	out := make([]oas.CharacterRevisionLine, 0, len(revs))
	for _, r := range revs {
		line := oas.CharacterRevisionLine{No: int32(r.No), Author: oas.DisplayName(r.Author), CreatedAt: r.CreatedAt.UTC(), Build: snapshotOut(r.Build)} //nolint:gosec // revision numbers are small
		if r.RetrainID != nil {
			line.RetrainId = oas.NewOptID(oas.ID(*r.RetrainID))
		}
		out = append(out, line)
	}
	return &oas.ListCharacterRevisionsOKHeaders{Response: out}, nil
}

// ApproveRetrain applies a pending retrain.
func (h *Handler) ApproveRetrain(ctx context.Context, p oas.ApproveRetrainParams) (oas.ApproveRetrainRes, error) {
	r, problem := h.decideRetrain(ctx, uuid.UUID(p.CampaignId), uuid.UUID(p.RetrainId), true)
	if problem != nil {
		return problem, nil
	}
	return &oas.RetrainHeaders{Response: r}, nil
}

// DeclineRetrain declines a pending retrain.
func (h *Handler) DeclineRetrain(ctx context.Context, p oas.DeclineRetrainParams) (oas.DeclineRetrainRes, error) {
	r, problem := h.decideRetrain(ctx, uuid.UUID(p.CampaignId), uuid.UUID(p.RetrainId), false)
	if problem != nil {
		return problem, nil
	}
	return &oas.RetrainHeaders{Response: r}, nil
}

func (h *Handler) decideRetrain(ctx context.Context, campaign, retrain uuid.UUID, approve bool) (oas.Retrain, *oas.ProblemStatusCodeWithHeaders) {
	c, ok := uiCaller(ctx)
	if !ok {
		return oas.Retrain{}, unauthorized()
	}
	r, err := h.Characters.DecideRetrain(ctx, c, domain.CampaignID(campaign), retrain, approve)
	if err != nil {
		return oas.Retrain{}, h.campaignProblem(ctx, "decide retrain", err)
	}
	return retrainOut(r), nil
}

func retrainOut(r domain.Retrain) oas.Retrain {
	out := oas.Retrain{
		ID: oas.ID(r.ID), CharacterId: oas.ID(r.CharacterID), Status: oas.RetrainStatus(r.Status), Reason: r.Reason,
		RequestedBy: oas.DisplayName(r.RequestedBy), CreatedAt: r.CreatedAt.UTC(), Proposed: snapshotOut(r.Proposed),
	}
	if r.DecidedBy != "" {
		out.DecidedBy = oas.NewOptDisplayName(oas.DisplayName(r.DecidedBy))
	}
	if r.DecidedAt != nil {
		out.DecidedAt = oas.NewOptDateTime(r.DecidedAt.UTC())
	}
	return out
}

//nolint:gosec // scores and levels are bounded by the rules
func snapshotOut(s domain.Snapshot) oas.BuildSnapshot {
	out := oas.BuildSnapshot{
		Species: oas.Slug(s.Species), Background: oas.Slug(s.Background), Method: oas.BuildSnapshotMethod(s.Method),
		Base: oas.AbilityBase{
			Strength: int32(s.Base["strength"]), Dexterity: int32(s.Base["dexterity"]), Constitution: int32(s.Base["constitution"]),
			Intelligence: int32(s.Base["intelligence"]), Wisdom: int32(s.Base["wisdom"]), Charisma: int32(s.Base["charisma"]),
		},
		Bonus: bonusOut(s.Bonus), Increase: increaseOut(s.Increase), Skills: slugsOf(s.Skills), Picks: make([]oas.PickLine, 0, len(s.Picks)),
	}
	for _, p := range s.Picks {
		out.Picks = append(out.Picks, oas.PickLine{Level: int32(p.Level), Choice: oas.Slug(p.Choice), Value: oas.Slug(p.Value)})
	}
	return out
}

func increaseFields(b *oas.AbilityIncrease) map[string]*oas.OptInt32 {
	return map[string]*oas.OptInt32{
		"strength": &b.Strength, "dexterity": &b.Dexterity, "constitution": &b.Constitution,
		"intelligence": &b.Intelligence, "wisdom": &b.Wisdom, "charisma": &b.Charisma,
	}
}

func increaseMap(b oas.AbilityIncrease) map[string]int {
	out := map[string]int{}
	for name, v := range increaseFields(&b) {
		if n, set := v.Get(); set && n > 0 {
			out[name] = int(n)
		}
	}
	return out
}

func increaseOut(m map[string]int) oas.AbilityIncrease {
	var b oas.AbilityIncrease
	for name, field := range increaseFields(&b) {
		if v, ok := m[name]; ok {
			*field = oas.NewOptInt32(int32(v)) //nolint:gosec // 0..20
		}
	}
	return b
}
