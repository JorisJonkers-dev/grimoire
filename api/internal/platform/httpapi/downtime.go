package httpapi

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	playapp "github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/downtime"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// DowntimeService gives and spends downtime days and keeps a Campaign's Recipes.
type DowntimeService interface {
	View(ctx context.Context, c caller.Caller, campaign uuid.UUID) (playapp.DowntimeView, error)
	Grant(ctx context.Context, c caller.Caller, campaign uuid.UUID, character *uuid.UUID, days int) (playapp.DowntimeView, error)
	AddRecipe(ctx context.Context, c caller.Caller, campaign uuid.UUID, r downtime.Recipe) (playdomain.CampaignRecipe, error)
	RemoveRecipe(ctx context.Context, c caller.Caller, campaign, recipe uuid.UUID) error
	Spend(ctx context.Context, c caller.Caller, campaign, character uuid.UUID, a playapp.DowntimeActivity) (playapp.DowntimeView, error)
}

//nolint:gosec // a Recipe is bounded by the rules
func recipeOut(r playdomain.CampaignRecipe) oas.Recipe {
	out := oas.Recipe{
		ID: oas.ID(r.ID), Name: r.Recipe.Name, Makes: r.Recipe.Makes, Quantity: int32(r.Recipe.Quantity), Days: int32(r.Recipe.Days), CostCp: int32(r.Recipe.CostCP),
		Ingredients: make([]oas.RecipeIngredient, 0, len(r.Recipe.Ingredients)),
	}
	if r.Recipe.Tool != "" {
		out.Tool = oas.NewOptString(r.Recipe.Tool)
	}
	for _, in := range r.Recipe.Ingredients {
		out.Ingredients = append(out.Ingredients, oas.RecipeIngredient{Item: in.Item, Count: int32(in.Count)})
	}
	return out
}

//nolint:gosec // days and the clock are bounded by the rules
func downtimeOut(v playapp.DowntimeView) *oas.DowntimeHeaders {
	d := v.Downtime
	out := oas.Downtime{
		Dm: v.DM, GameDay: int32(d.Clock.Day), GameMinute: int32(d.Clock.Minute),
		Characters: make([]oas.DowntimeCharacter, 0, len(d.Characters)), Recipes: make([]oas.Recipe, 0, len(d.Recipes)), Log: make([]oas.DowntimeEntry, 0, len(d.Log)),
	}
	for _, ch := range d.Characters {
		out.Characters = append(out.Characters, oas.DowntimeCharacter{ID: oas.ID(ch.ID), Name: ch.Name, Days: int32(ch.Days), Mine: slices.Contains(v.Mine, ch.ID)})
	}
	for _, r := range d.Recipes {
		out.Recipes = append(out.Recipes, recipeOut(r))
	}
	for _, e := range d.Log {
		out.Log = append(out.Log, oas.DowntimeEntry{ID: oas.ID(e.ID), Character: e.Name, Activity: oas.DowntimeKind(e.Activity), Detail: e.Detail, Days: int32(e.Days), At: e.At.UTC()})
	}
	return &oas.DowntimeHeaders{Response: out}
}

// GetDowntime shows the Campaign's downtime.
func (h *Handler) GetDowntime(ctx context.Context, p oas.GetDowntimeParams) (oas.GetDowntimeRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	v, err := h.Downtime.View(ctx, c, uuid.UUID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "downtime", err), nil
	}
	return downtimeOut(v), nil
}

// GrantDowntime gives downtime days.
func (h *Handler) GrantDowntime(ctx context.Context, req *oas.DowntimeGrant, p oas.GrantDowntimeParams) (oas.GrantDowntimeRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	var character *uuid.UUID
	if id, set := req.CharacterId.Get(); set {
		ch := uuid.UUID(id)
		character = &ch
	}
	v, err := h.Downtime.Grant(ctx, c, uuid.UUID(p.CampaignId), character, int(req.Days))
	if err != nil {
		return h.campaignProblem(ctx, "grant downtime", err), nil
	}
	return downtimeOut(v), nil
}

// SpendDowntime has a Character spend downtime days.
func (h *Handler) SpendDowntime(ctx context.Context, req *oas.DowntimeActivity, p oas.SpendDowntimeParams) (oas.SpendDowntimeRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	a := playapp.DowntimeActivity{Kind: string(req.Activity), Days: int(req.Days.Or(0)), Recipe: uuid.UUID(req.RecipeId.Or(oas.ID{})), Subject: req.Subject.Or("")}
	v, err := h.Downtime.Spend(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(p.CharacterId), a)
	if err != nil {
		return h.campaignProblem(ctx, "spend downtime", err), nil
	}
	return downtimeOut(v), nil
}

// CreateRecipe adds a Recipe.
func (h *Handler) CreateRecipe(ctx context.Context, req *oas.RecipeInput, p oas.CreateRecipeParams) (oas.CreateRecipeRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	r := downtime.Recipe{Name: req.Name, Makes: req.Makes, Quantity: int(req.Quantity), Ingredients: make([]downtime.Ingredient, 0, len(req.Ingredients)), Tool: req.Tool.Or(""), Days: int(req.Days), CostCP: int(req.CostCp.Or(0))}
	for _, in := range req.Ingredients {
		r.Ingredients = append(r.Ingredients, downtime.Ingredient{Item: in.Item, Count: int(in.Count)})
	}
	made, err := h.Downtime.AddRecipe(ctx, c, uuid.UUID(p.CampaignId), r)
	if err != nil {
		return h.campaignProblem(ctx, "create recipe", err), nil
	}
	return &oas.RecipeHeaders{Response: recipeOut(made)}, nil
}

// DeleteRecipe removes a Recipe.
func (h *Handler) DeleteRecipe(ctx context.Context, p oas.DeleteRecipeParams) (oas.DeleteRecipeRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Downtime.RemoveRecipe(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(p.RecipeId)); err != nil {
		return h.campaignProblem(ctx, "delete recipe", err), nil
	}
	return &oas.DeleteRecipeNoContent{}, nil
}
