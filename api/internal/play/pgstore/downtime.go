package pgstore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/clock"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/downtime"
)

// Downtime reads a Campaign's downtime: its Game Clock, its Characters' days, its Recipes and the last
// fifty things done.
func (s *Store) Downtime(ctx context.Context, campaign uuid.UUID) (domain.Downtime, error) {
	var out domain.Downtime
	c, err := s.q.CampaignDowntime(ctx, campaign)
	if err != nil {
		return out, err
	}
	out.Clock, out.Advanced = clock.Time{Day: int(c.GameDay), Minute: int(c.GameMinute)}, int(c.DowntimeAdvanced)
	characters, err := s.q.DowntimeCharacters(ctx, campaign)
	if err != nil {
		return out, err
	}
	out.Characters = make([]domain.DowntimeCharacter, 0, len(characters))
	for _, ch := range characters {
		out.Characters = append(out.Characters, domain.DowntimeCharacter{ID: ch.ID, Name: ch.Name, Owner: ch.OwnerMemberID, Days: int(ch.DowntimeDays), Spent: int(ch.DowntimeSpent)})
	}
	if out.Recipes, err = s.recipes(ctx, campaign); err != nil {
		return out, err
	}
	log, err := s.q.ListDowntimeLog(ctx, campaign)
	if err != nil {
		return out, err
	}
	out.Log = make([]domain.DowntimeEntry, 0, len(log))
	for _, l := range log {
		out.Log = append(out.Log, domain.DowntimeEntry{ID: l.ID, Campaign: campaign, Character: l.CharacterID, Name: l.CharacterName, Activity: l.Activity, Detail: l.Detail, Days: int(l.Days), At: l.CreatedAt})
	}
	return out, nil
}

func (s *Store) recipes(ctx context.Context, campaign uuid.UUID) ([]domain.CampaignRecipe, error) {
	rows, err := s.q.ListRecipes(ctx, campaign)
	if err != nil {
		return nil, err
	}
	parts, err := s.q.ListRecipeIngredients(ctx, campaign)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CampaignRecipe, 0, len(rows))
	for _, r := range rows {
		recipe := downtime.Recipe{Name: r.Name, Makes: r.ItemSlug, Quantity: int(r.Quantity), Ingredients: []downtime.Ingredient{}, Tool: r.ToolSlug.String, Days: int(r.Days), CostCP: int(r.CostCp)}
		for _, p := range parts {
			if p.RecipeID == r.ID {
				recipe.Ingredients = append(recipe.Ingredients, downtime.Ingredient{Item: p.ItemSlug, Count: int(p.Count)})
			}
		}
		out = append(out, domain.CampaignRecipe{ID: r.ID, Recipe: recipe})
	}
	return out, nil
}

// TrainingDays is how many days a Character has spent training in one thing.
func (s *Store) TrainingDays(ctx context.Context, character uuid.UUID, subject string) (int, error) {
	n, err := s.q.TrainingDays(ctx, queries.TrainingDaysParams{CharacterID: character, Detail: subject})
	return int(n), err
}

// Lock holds the Campaign's downtime until the transaction ends.
func (s *Store) Lock(ctx context.Context, campaign uuid.UUID) error {
	_, err := s.q.LockCampaignDowntime(ctx, campaign)
	return err
}

// WriteDowntime runs downtime changes in one transaction.
func (s *Store) WriteDowntime(ctx context.Context, fn func(app.DowntimeWriter) error) error {
	return s.InTx(ctx, func(r app.Repository) error { return fn(r.(*Store)) })
}

// SetPurse makes a Container's coins what they are after a change: each coin it now holds, and none
// of a coin it no longer does.
func (s *Store) SetPurse(ctx context.Context, in domain.ContainerID, before, after map[string]int) error {
	for coin := range before {
		if after[coin] == 0 {
			if err := s.setCount(ctx, in, "", coin, 0); err != nil {
				return err
			}
		}
	}
	for coin, n := range after {
		if err := s.setCount(ctx, in, "", coin, n); err != nil {
			return err
		}
	}
	return nil
}

// SetDowntimeDays keeps the downtime days a Character has left and has lived through.
//
//nolint:gosec // days are bounded by the rules
func (s *Store) SetDowntimeDays(ctx context.Context, character uuid.UUID, days, spent int) error {
	return s.q.SetCharacterDowntime(ctx, queries.SetCharacterDowntimeParams{ID: character, Days: int32(days), Spent: int32(spent)})
}

// GrantDowntime gives days to one Character, or to every Character of the Campaign, which starts a new
// downtime the Game Clock has not yet moved through.
//
//nolint:gosec // days are bounded by the rules
func (s *Store) GrantDowntime(ctx context.Context, campaign uuid.UUID, character *uuid.UUID, days int) (bool, error) {
	if character != nil {
		n, err := s.q.GrantDowntimeToOne(ctx, queries.GrantDowntimeToOneParams{CampaignID: campaign, ID: *character, Days: int32(days)})
		return n > 0, err
	}
	if err := s.q.GrantDowntimeToAll(ctx, queries.GrantDowntimeToAllParams{CampaignID: campaign, Days: int32(days)}); err != nil {
		return false, err
	}
	c, err := s.q.CampaignDowntime(ctx, campaign)
	if err != nil {
		return false, err
	}
	return true, s.q.SetDowntimeClock(ctx, queries.SetDowntimeClockParams{ID: campaign, GameDay: c.GameDay, GameMinute: c.GameMinute, Advanced: 0})
}

// MoveDowntimeClock moves the Game Clock on through downtime, and keeps how far this downtime has moved it.
//
//nolint:gosec // days and minutes are bounded by the rules
func (s *Store) MoveDowntimeClock(ctx context.Context, campaign uuid.UUID, to clock.Time, advanced int) error {
	return s.q.SetDowntimeClock(ctx, queries.SetDowntimeClockParams{ID: campaign, GameDay: int32(to.Day), GameMinute: int32(to.Minute), Advanced: int32(advanced)})
}

// LogDowntime records what a Character did with its downtime.
//
//nolint:gosec // days are bounded by the rules
func (s *Store) LogDowntime(ctx context.Context, e domain.DowntimeEntry) error {
	return s.q.InsertDowntimeLog(ctx, queries.InsertDowntimeLogParams{ID: e.ID, CampaignID: e.Campaign, CharacterID: e.Character, Activity: e.Activity, Detail: e.Detail, Days: int32(e.Days), Now: e.At})
}

// InsertRecipe adds a Recipe with its ingredients.
//
//nolint:gosec // a Recipe is bounded by the rules
func (s *Store) InsertRecipe(ctx context.Context, campaign uuid.UUID, r domain.CampaignRecipe, now time.Time) error {
	if err := s.q.InsertRecipe(ctx, queries.InsertRecipeParams{
		ID: r.ID, CampaignID: campaign, Name: r.Recipe.Name, ItemSlug: r.Recipe.Makes, Quantity: int32(r.Recipe.Quantity),
		ToolSlug: pgtype.Text{String: r.Recipe.Tool, Valid: r.Recipe.Tool != ""}, Days: int32(r.Recipe.Days), CostCp: int32(r.Recipe.CostCP), Now: now,
	}); err != nil {
		return err
	}
	for i, in := range r.Recipe.Ingredients {
		if err := s.q.InsertRecipeIngredient(ctx, queries.InsertRecipeIngredientParams{RecipeID: r.ID, Position: int32(i), ItemSlug: in.Item, Count: int32(in.Count)}); err != nil {
			return err
		}
	}
	return nil
}

// DeleteRecipe removes a Recipe; false when the Campaign has no such one.
func (s *Store) DeleteRecipe(ctx context.Context, campaign, recipe uuid.UUID) (bool, error) {
	n, err := s.q.DeleteRecipe(ctx, queries.DeleteRecipeParams{CampaignID: campaign, ID: recipe})
	return n > 0, err
}
