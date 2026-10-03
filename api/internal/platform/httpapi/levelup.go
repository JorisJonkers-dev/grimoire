package httpapi

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
)

// PlanLevelUp shows what the next level offers in a class.
func (h *Handler) PlanLevelUp(ctx context.Context, p oas.PlanLevelUpParams) (oas.PlanLevelUpRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	plan, err := h.Characters.PlanLevelUp(ctx, c, domain.CampaignID(p.CampaignId), domain.CharacterID(p.CharacterId), string(p.Class.Or("")), string(p.Subclass.Or("")))
	if err != nil {
		return h.campaignProblem(ctx, "plan level up", err), nil
	}
	return &oas.LevelUpPlanHeaders{Response: planOut(plan)}, nil
}

// LevelUp takes the next level with the choices made for it.
func (h *Handler) LevelUp(ctx context.Context, req *oas.LevelUpRequest, p oas.LevelUpParams) (oas.LevelUpRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	in := app.LevelUpRequest{
		Class: string(req.Class), Roll: req.HitPoints.Or(oas.LevelUpRequestHitPointsAverage) == oas.LevelUpRequestHitPointsRoll,
		Picks: map[string][]string{}, Increase: bonusMap(req.Increase.Or(oas.AbilityBonus{})), Spells: slugList(req.Spells),
	}
	for _, pick := range req.Picks {
		in.Picks[string(pick.Choice)] = slugList(pick.Values)
	}
	s, err := h.Characters.LevelUp(ctx, c, domain.CampaignID(p.CampaignId), domain.CharacterID(p.CharacterId), in)
	if err != nil {
		return h.campaignProblem(ctx, "level up", err), nil
	}
	return &oas.CharacterSheetHeaders{Response: sheetOut(s)}, nil
}

//nolint:gosec // levels, dice and counts are bounded by the rules
func planOut(p app.LevelUpPlan) oas.LevelUpPlan {
	out := oas.LevelUpPlan{
		Ready: p.Ready, Held: p.Held, Level: int32(p.Level), Class: oas.Slug(p.Class), ClassLevel: int32(p.ClassLevel),
		HitDie: int32(p.HitDie), Average: int32(p.Average), Cantrips: int32(p.Cantrips), Spells: int32(p.Spells),
		Classes: make([]oas.LevelUpClass, 0, len(p.Classes)), Choices: make([]oas.LevelUpChoice, 0, len(p.Choices)),
		SpellList: make([]oas.SpellPick, 0, len(p.SpellList)),
	}
	for _, c := range p.Classes {
		out.Classes = append(out.Classes, oas.LevelUpClass{Slug: oas.Slug(c.Slug), Name: c.Name, HitDie: int32(c.HitDie), Level: int32(c.Level), Unmet: nonNil(c.Unmet)})
	}
	for _, ch := range p.Choices {
		line := oas.LevelUpChoice{Slug: oas.Slug(ch.Slug), Name: ch.Name, Pool: oas.LevelUpChoicePool(ch.Pool), Count: int32(ch.Count), Options: make([]oas.LevelUpOption, 0, len(ch.Options))}
		for _, o := range ch.Options {
			line.Options = append(line.Options, oas.LevelUpOption{Slug: oas.Slug(o.Slug), Name: o.Name, Unmet: nonNil(o.Unmet)})
		}
		out.Choices = append(out.Choices, line)
	}
	for _, sp := range p.SpellList {
		out.SpellList = append(out.SpellList, oas.SpellPick{Slug: oas.Slug(sp.Slug), Name: sp.Name, Level: int32(sp.Level)})
	}
	return out
}
