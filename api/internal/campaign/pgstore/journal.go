package pgstore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

// Quests lists a Campaign's Quests, oldest first, each with its steps in order.
func (s *Store) Quests(ctx context.Context, id domain.CampaignID) ([]domain.Quest, error) {
	rows, err := s.q.ListQuests(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	steps, err := s.q.ListQuestSteps(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Quest, 0, len(rows))
	for _, r := range rows {
		q := domain.Quest{ID: r.ID, CampaignID: domain.CampaignID(r.CampaignID), Name: r.Name, Summary: r.Summary, Status: r.Status, Steps: []domain.QuestStep{}, UpdatedAt: r.UpdatedAt}
		for _, step := range steps {
			if step.QuestID == r.ID {
				q.Steps = append(q.Steps, domain.QuestStep{Text: step.Body, Done: step.Done})
			}
		}
		out = append(out, q)
	}
	return out, nil
}

// saveSteps replaces a Quest's steps.
func saveSteps(ctx context.Context, q *queries.Queries, quest domain.Quest) error {
	if err := q.ClearQuestSteps(ctx, quest.ID); err != nil {
		return err
	}
	for i, step := range quest.Steps {
		if err := q.InsertQuestStep(ctx, queries.InsertQuestStepParams{QuestID: quest.ID, Position: int32(i), Body: step.Text, Done: step.Done}); err != nil { //nolint:gosec // at most 50 steps
			return err
		}
	}
	return nil
}

// InsertQuest adds a Quest with its steps, all or nothing.
func (s *Store) InsertQuest(ctx context.Context, quest domain.Quest, now time.Time) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := queries.New(s.wrap(tx))
		if err := q.InsertQuest(ctx, queries.InsertQuestParams{ID: quest.ID, CampaignID: uuid.UUID(quest.CampaignID), Name: quest.Name, Summary: quest.Summary, Status: quest.Status, Now: now}); err != nil {
			return err
		}
		return saveSteps(ctx, q, quest)
	})
}

// UpdateQuest changes a Quest of its Campaign and replaces its steps, all or nothing.
func (s *Store) UpdateQuest(ctx context.Context, quest domain.Quest, now time.Time) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := queries.New(s.wrap(tx))
		n, err := q.UpdateQuest(ctx, queries.UpdateQuestParams{CampaignID: uuid.UUID(quest.CampaignID), ID: quest.ID, Name: quest.Name, Summary: quest.Summary, Status: quest.Status, Now: now})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return saveSteps(ctx, q, quest)
	})
}

// DeleteQuest removes a Quest of its Campaign.
func (s *Store) DeleteQuest(ctx context.Context, id domain.CampaignID, quest domain.QuestID) error {
	n, err := s.q.DeleteQuest(ctx, queries.DeleteQuestParams{CampaignID: uuid.UUID(id), ID: quest})
	if err == nil && n == 0 {
		return domain.ErrNotFound
	}
	return err
}

func unlockedAt(l domain.Lore) pgtype.Timestamptz {
	if l.UnlockedAt == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *l.UnlockedAt, Valid: true}
}

// Lore lists a Campaign's Lore entries by title.
func (s *Store) Lore(ctx context.Context, id domain.CampaignID) ([]domain.Lore, error) {
	rows, err := s.q.ListLore(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Lore, 0, len(rows))
	for _, r := range rows {
		l := domain.Lore{ID: r.ID, CampaignID: domain.CampaignID(r.CampaignID), Title: r.Title, Body: r.Body, ItemSlug: r.ItemSlug, UnlockedAt: nil, UpdatedAt: r.UpdatedAt}
		if r.UnlockedAt.Valid {
			at := r.UnlockedAt.Time
			l.UnlockedAt = &at
		}
		out = append(out, l)
	}
	return out, nil
}

// InsertLore adds a Lore entry.
func (s *Store) InsertLore(ctx context.Context, l domain.Lore, now time.Time) error {
	return s.q.InsertLore(ctx, queries.InsertLoreParams{ID: l.ID, CampaignID: uuid.UUID(l.CampaignID), Title: l.Title, Body: l.Body, ItemSlug: l.ItemSlug, UnlockedAt: unlockedAt(l), Now: now})
}

// UpdateLore changes a Lore entry of its Campaign.
func (s *Store) UpdateLore(ctx context.Context, l domain.Lore, now time.Time) error {
	n, err := s.q.UpdateLore(ctx, queries.UpdateLoreParams{CampaignID: uuid.UUID(l.CampaignID), ID: l.ID, Title: l.Title, Body: l.Body, ItemSlug: l.ItemSlug, UnlockedAt: unlockedAt(l), Now: now})
	if err == nil && n == 0 {
		return domain.ErrNotFound
	}
	return err
}

// DeleteLore removes a Lore entry of its Campaign.
func (s *Store) DeleteLore(ctx context.Context, id domain.CampaignID, lore domain.LoreID) error {
	n, err := s.q.DeleteLore(ctx, queries.DeleteLoreParams{CampaignID: uuid.UUID(id), ID: lore})
	if err == nil && n == 0 {
		return domain.ErrNotFound
	}
	return err
}

// CarriedItems lists the items a Member can read in the Campaign.
func (s *Store) CarriedItems(ctx context.Context, id domain.CampaignID, member domain.MemberID) ([]string, error) {
	return s.q.CarriedItems(ctx, queries.CarriedItemsParams{CampaignID: uuid.UUID(id), MemberID: uuid.UUID(member)})
}

// UnlockLore unlocks every locked Lore entry of the Campaign an item holds.
func (s *Store) UnlockLore(ctx context.Context, id domain.CampaignID, itemSlug string, now time.Time) (int, error) {
	ids, err := s.q.UnlockLoreByItem(ctx, queries.UnlockLoreByItemParams{CampaignID: uuid.UUID(id), ItemSlug: itemSlug, Now: pgtype.Timestamptz{Time: now, Valid: true}})
	return len(ids), err
}
