package pgstore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

func optText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func lootRef(id pgtype.UUID) *domain.LootTableID {
	if !id.Valid {
		return nil
	}
	t := domain.LootTableID(id.Bytes)
	return &t
}

func lootUUID(id *domain.LootTableID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

// ItemExists reports whether the compendium has an item.
func (s *Store) ItemExists(ctx context.Context, campaign uuid.UUID, slug string) (bool, error) {
	rows, err := s.q.ItemsBySlug(ctx, queries.ItemsBySlugParams{Slugs: []string{slug}, CampaignID: campaign})
	return len(rows) > 0, err
}

// LootTables lists the Campaign's Loot Tables with their entries.
func (s *Store) LootTables(ctx context.Context, campaign uuid.UUID) ([]domain.LootTable, error) {
	return LoadLootTables(ctx, s.q, campaign)
}

// LoadLootTables reads the Campaign's Loot Tables with their entries.
func LoadLootTables(ctx context.Context, q *queries.Queries, campaign uuid.UUID) ([]domain.LootTable, error) {
	rows, err := q.ListLootTables(ctx, campaign)
	if err != nil {
		return nil, err
	}
	entries, err := q.CampaignLootEntries(ctx, campaign)
	if err != nil {
		return nil, err
	}
	out := make([]domain.LootTable, 0, len(rows))
	for _, r := range rows {
		t := domain.LootTable{ID: domain.LootTableID(r.ID), Name: r.Name, Rolls: int(r.Rolls), Entries: []domain.LootEntry{}, UpdatedAt: r.UpdatedAt}
		for _, e := range entries {
			if e.TableID == r.ID {
				t.Entries = append(t.Entries, domain.LootEntry{
					Weight: int(e.Weight), Kind: e.Kind, Item: e.ItemSlug.String, Coin: e.Coin.String, Amount: e.Amount, Table: lootRef(e.NestedTableID),
				})
			}
		}
		out = append(out, t)
	}
	return out, nil
}

// SaveLootTable writes a Loot Table and replaces its entries.
//
//nolint:gosec // rolls and weights are bounded by the rules
func (s *Store) SaveLootTable(ctx context.Context, campaign uuid.UUID, t domain.LootTable, now time.Time) error {
	id := uuid.UUID(t.ID)
	if err := s.q.SaveLootTable(ctx, queries.SaveLootTableParams{ID: id, CampaignID: campaign, Name: t.Name, Rolls: int32(t.Rolls), Now: now}); err != nil {
		return err
	}
	if err := s.q.ClearLootEntries(ctx, id); err != nil {
		return err
	}
	for i, e := range t.Entries {
		if err := s.q.InsertLootEntry(ctx, queries.InsertLootEntryParams{
			TableID: id, Ordering: int32(i), Weight: int32(e.Weight), Kind: e.Kind, ItemSlug: optText(e.Item), Coin: optText(e.Coin), Amount: e.Amount,
			NestedTableID: lootUUID(e.Table),
		}); err != nil {
			return err
		}
	}
	return nil
}

// DeleteLootTable removes a Loot Table.
func (s *Store) DeleteLootTable(ctx context.Context, campaign uuid.UUID, id domain.LootTableID) (bool, error) {
	n, err := s.q.DeleteLootTable(ctx, queries.DeleteLootTableParams{CampaignID: campaign, ID: uuid.UUID(id)})
	return n > 0, err
}

// LootTableInUse reports whether another Loot Table rolls on this one.
func (s *Store) LootTableInUse(ctx context.Context, id domain.LootTableID) (bool, error) {
	n, err := s.q.LootTableInUse(ctx, lootUUID(&id))
	return n > 0, err
}

// RecordLootTable stores a Revision with the Loot Table's snapshot.
//
//nolint:gosec // rolls and weights are bounded by the rules
func (s *Store) RecordLootTable(ctx context.Context, campaign uuid.UUID, rev campaigndomain.Revision, c caller.Caller, t domain.LootTable) error {
	revID, err := s.record(ctx, campaign, domain.EntityLoot, uuid.UUID(t.ID), rev, c)
	if err != nil {
		return err
	}
	if err := s.q.InsertLootTableRevision(ctx, queries.InsertLootTableRevisionParams{RevisionID: revID, Name: t.Name, Rolls: int32(t.Rolls)}); err != nil {
		return err
	}
	for i, e := range t.Entries {
		if err := s.q.InsertLootRevisionEntry(ctx, queries.InsertLootRevisionEntryParams{
			RevisionID: revID, Ordering: int32(i), Weight: int32(e.Weight), Kind: e.Kind, ItemSlug: optText(e.Item), Coin: optText(e.Coin), Amount: e.Amount,
			NestedTableID: lootUUID(e.Table),
		}); err != nil {
			return err
		}
	}
	return nil
}

// LootTableAt reads a Loot Table as one of its Revisions recorded it.
func (s *Store) LootTableAt(ctx context.Context, campaign uuid.UUID, id domain.LootTableID, no int) (domain.LootTable, error) {
	r, err := s.q.GetLootTableRevision(ctx, queries.GetLootTableRevisionParams{CampaignID: campaign, EntityID: uuid.UUID(id), RevisionNo: int32(no)}) //nolint:gosec // bounded by the API
	if err != nil {
		return domain.LootTable{}, notFound(err)
	}
	rows, err := s.q.LootRevisionEntries(ctx, r.ID)
	if err != nil {
		return domain.LootTable{}, err
	}
	t := domain.LootTable{ID: id, Name: r.Name, Rolls: int(r.Rolls), Entries: []domain.LootEntry{}}
	for _, e := range rows {
		t.Entries = append(t.Entries, domain.LootEntry{Weight: int(e.Weight), Kind: e.Kind, Item: e.ItemSlug.String, Coin: e.Coin.String, Amount: e.Amount, Table: lootRef(e.NestedTableID)})
	}
	return t, nil
}
