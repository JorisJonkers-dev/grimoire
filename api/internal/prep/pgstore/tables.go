package pgstore

import (
	"context"
	"time"

	"github.com/google/uuid"

	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

func poolRef(id *uuid.UUID) *domain.PoolID {
	if id == nil {
		return nil
	}
	p := domain.PoolID(*id)
	return &p
}

func poolUUID(id *domain.PoolID) *uuid.UUID {
	if id == nil {
		return nil
	}
	u := uuid.UUID(*id)
	return &u
}

// Tables lists the Campaign's Tables with their entries.
func (s *Store) Tables(ctx context.Context, campaign uuid.UUID) ([]domain.Table, error) {
	return LoadTables(ctx, s.q, campaign)
}

// LoadTables reads the Campaign's Tables with their entries.
func LoadTables(ctx context.Context, q *queries.Queries, campaign uuid.UUID) ([]domain.Table, error) {
	rows, err := q.ListTables(ctx, campaign)
	if err != nil {
		return nil, err
	}
	entries, err := q.CampaignTableEntries(ctx, campaign)
	if err != nil {
		return nil, err
	}
	monsters, err := q.CampaignEntryMonsters(ctx, campaign)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Table, 0, len(rows))
	for _, r := range rows {
		t := domain.Table{
			ID: domain.TableID(r.ID), Name: r.Name, RegionID: fromUUID(r.RegionNodeID), ChancePct: int(r.ChancePct), Visibility: r.Visibility,
			Entries: []domain.Entry{}, UpdatedAt: r.UpdatedAt,
		}
		t.Entries = entriesOf(r.ID, entries, monsters)
		out = append(out, t)
	}
	return out, nil
}

// entriesOf picks one Table's entries and their monsters out of the Campaign's.
func entriesOf(id uuid.UUID, entries []queries.PrepTableEntry, monsters []queries.PrepEntryMonster) []domain.Entry {
	out := []domain.Entry{}
	for _, e := range entries {
		if e.TableID != id {
			continue
		}
		entry := domain.Entry{Weight: int(e.Weight), Kind: e.Kind, Label: e.Label, PoolID: poolRef(fromUUID(e.PoolID)), FactionID: fromUUID(e.FactionID)}
		for _, m := range monsters {
			if m.TableID == id && m.Ordering == e.Ordering {
				entry.Monsters = append(entry.Monsters, domain.EntryMonster{Slug: m.MonsterSlug, Count: int(m.Count)})
			}
		}
		out = append(out, entry)
	}
	return out
}

// SaveTable writes a Table and replaces its entries.
//
//nolint:gosec // chances, weights and counts are bounded by the rules
func (s *Store) SaveTable(ctx context.Context, campaign uuid.UUID, t domain.Table, now time.Time) error {
	id := uuid.UUID(t.ID)
	if err := s.q.SaveEncounterTable(ctx, queries.SaveEncounterTableParams{
		ID: id, CampaignID: campaign, Name: t.Name, RegionNodeID: optUUID(t.RegionID), ChancePct: int32(t.ChancePct), Visibility: t.Visibility, Now: now,
	}); err != nil {
		return err
	}
	if err := s.q.ClearTableEntries(ctx, id); err != nil {
		return err
	}
	for i, e := range t.Entries {
		if err := s.q.InsertTableEntry(ctx, queries.InsertTableEntryParams{
			TableID: id, Ordering: int32(i), Weight: int32(e.Weight), Kind: e.Kind, Label: e.Label, PoolID: optUUID(poolUUID(e.PoolID)), FactionID: optUUID(e.FactionID),
		}); err != nil {
			return err
		}
		for j, m := range e.Monsters {
			if err := s.q.InsertEntryMonster(ctx, queries.InsertEntryMonsterParams{
				TableID: id, Ordering: int32(i), Position: int32(j), MonsterSlug: m.Slug, Count: int32(m.Count),
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// DeleteTable removes a Table.
func (s *Store) DeleteTable(ctx context.Context, campaign uuid.UUID, id domain.TableID) (bool, error) {
	n, err := s.q.DeleteEncounterTable(ctx, queries.DeleteEncounterTableParams{CampaignID: campaign, ID: uuid.UUID(id)})
	return n > 0, err
}

// RecordTable stores a Revision with the Table's snapshot.
//
//nolint:gosec // chances, weights and counts are bounded by the rules
func (s *Store) RecordTable(ctx context.Context, campaign uuid.UUID, rev campaigndomain.Revision, c caller.Caller, t domain.Table) error {
	revID, err := s.record(ctx, campaign, domain.EntityTable, uuid.UUID(t.ID), rev, c)
	if err != nil {
		return err
	}
	if err := s.q.InsertTableRevision(ctx, queries.InsertTableRevisionParams{
		RevisionID: revID, Name: t.Name, RegionNodeID: optUUID(t.RegionID), ChancePct: int32(t.ChancePct), Visibility: t.Visibility,
	}); err != nil {
		return err
	}
	for i, e := range t.Entries {
		if err := s.q.InsertTableRevisionEntry(ctx, queries.InsertTableRevisionEntryParams{
			RevisionID: revID, Ordering: int32(i), Weight: int32(e.Weight), Kind: e.Kind, Label: e.Label, PoolID: optUUID(poolUUID(e.PoolID)), FactionID: optUUID(e.FactionID),
		}); err != nil {
			return err
		}
		for j, m := range e.Monsters {
			if err := s.q.InsertTableRevisionMonster(ctx, queries.InsertTableRevisionMonsterParams{
				RevisionID: revID, Ordering: int32(i), Position: int32(j), MonsterSlug: m.Slug, Count: int32(m.Count),
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// TableAt reads a Table as one of its Revisions recorded it.
func (s *Store) TableAt(ctx context.Context, campaign uuid.UUID, id domain.TableID, no int) (domain.Table, error) {
	r, err := s.q.GetTableRevision(ctx, queries.GetTableRevisionParams{CampaignID: campaign, EntityID: uuid.UUID(id), RevisionNo: int32(no)}) //nolint:gosec // bounded by the API
	if err != nil {
		return domain.Table{}, notFound(err)
	}
	entries, err := s.q.TableRevisionEntries(ctx, r.ID)
	if err != nil {
		return domain.Table{}, err
	}
	monsters, err := s.q.TableRevisionMonsters(ctx, r.ID)
	if err != nil {
		return domain.Table{}, err
	}
	t := domain.Table{ID: id, Name: r.Name, RegionID: fromUUID(r.RegionNodeID), ChancePct: int(r.ChancePct), Visibility: r.Visibility, Entries: []domain.Entry{}}
	for _, e := range entries {
		entry := domain.Entry{Weight: int(e.Weight), Kind: e.Kind, Label: e.Label, PoolID: poolRef(fromUUID(e.PoolID)), FactionID: fromUUID(e.FactionID)}
		for _, m := range monsters {
			if m.Ordering == e.Ordering {
				entry.Monsters = append(entry.Monsters, domain.EntryMonster{Slug: m.MonsterSlug, Count: int(m.Count)})
			}
		}
		t.Entries = append(t.Entries, entry)
	}
	return t, nil
}
