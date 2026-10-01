package app

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/loot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// LootTables lists the Campaign's Loot Tables. DM only.
func (s *Service) LootTables(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.LootTable, error) {
	if _, err := s.dm(ctx, c, campaign); err != nil {
		return nil, err
	}
	return s.Repo.LootTables(ctx, campaign)
}

func (s *Service) cleanLoot(ctx context.Context, r Repository, campaign uuid.UUID, t domain.LootTable, all []domain.LootTable) (domain.LootTable, error) {
	n, err := name(t.Name)
	if err != nil {
		return t, err
	}
	t.Name = n
	switch {
	case t.Rolls < 1 || t.Rolls > 10:
		return t, apperr.Refuse("a loot table is rolled 1 to 10 times")
	case len(t.Entries) == 0 || len(t.Entries) > 50:
		return t, apperr.Refuse("a loot table holds 1 to 50 entries")
	}
	for i, e := range t.Entries {
		if t.Entries[i], err = cleanLootEntry(ctx, r, campaign, e, all); err != nil {
			return t, err
		}
	}
	if nests(t, all, t.ID, 0) {
		return t, apperr.Refuse("a loot table cannot roll on itself, however deep")
	}
	return t, nil
}

func cleanLootEntry(ctx context.Context, r Repository, campaign uuid.UUID, e domain.LootEntry, all []domain.LootTable) (domain.LootEntry, error) {
	if e.Weight < 1 || e.Weight > 100 {
		return e, apperr.Refuse("weights run from 1 to 100")
	}
	out := domain.LootEntry{Weight: e.Weight, Kind: e.Kind}
	if e.Kind == loot.Item || e.Kind == loot.Currency {
		if _, err := loot.ParseAmount(e.Amount); err != nil {
			return e, apperr.Refuse("write amounts as 3, 2d6, 1d4+1 or 4d6x10")
		}
		out.Amount = e.Amount
	}
	switch e.Kind {
	case loot.Item:
		if err := item(ctx, r, campaign, e.Item); err != nil {
			return e, err
		}
		out.Item = e.Item
	case loot.Currency:
		if !slices.Contains(loot.Coins(), e.Coin) {
			return e, apperr.Refuse("coins are cp, sp, ep, gp or pp")
		}
		out.Coin = e.Coin
	case loot.Nested:
		if e.Table == nil || !slices.ContainsFunc(all, func(t domain.LootTable) bool { return t.ID == *e.Table }) {
			return e, apperr.Refuse("choose one of the campaign's loot tables")
		}
		out.Table = e.Table
	case loot.Nothing:
	default:
		return e, apperr.Refuse("an entry is an item, coins, another table or nothing")
	}
	return out, nil
}

func item(ctx context.Context, r Repository, campaign uuid.UUID, slug string) error {
	ok, err := r.ItemExists(ctx, campaign, slug)
	if err == nil && !ok {
		return apperr.Refuse("there is no item " + slug)
	}
	return err
}

// nests reports whether table t reaches the table id through its nested entries.
func nests(t domain.LootTable, all []domain.LootTable, id domain.LootTableID, depth int) bool {
	if depth > len(all) {
		return false
	}
	for _, e := range t.Entries {
		if e.Table == nil {
			continue
		}
		if *e.Table == id {
			return true
		}
		i := slices.IndexFunc(all, func(x domain.LootTable) bool { return x.ID == *e.Table })
		if nests(all[i], all, id, depth+1) {
			return true
		}
	}
	return false
}

// SaveLootTable creates a Loot Table, or replaces the one with the same id.
func (s *Service) SaveLootTable(ctx context.Context, c caller.Caller, campaign uuid.UUID, t domain.LootTable) (domain.LootTable, error) {
	err := s.write(ctx, c, campaign, func(r Repository, me playdomain.Member, now time.Time) error {
		all, err := r.LootTables(ctx, campaign)
		if err != nil {
			return err
		}
		action := campaigndomain.ActionCreate
		if t.ID != (domain.LootTableID{}) {
			if !slices.ContainsFunc(all, func(x domain.LootTable) bool { return x.ID == t.ID }) {
				return apperr.ErrNotFound
			}
			action = campaigndomain.ActionUpdate
		} else {
			t.ID = domain.LootTableID(uuid.New())
		}
		if t, err = s.cleanLoot(ctx, r, campaign, t, all); err != nil {
			return err
		}
		t.UpdatedAt = now
		if err := r.SaveLootTable(ctx, campaign, t, now); err != nil {
			return err
		}
		return r.RecordLootTable(ctx, campaign, revision(action, me, now), c, t)
	})
	return t, err
}

// DeleteLootTable removes a Loot Table no other table rolls on; its Revisions keep it restorable.
func (s *Service) DeleteLootTable(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.LootTableID) error {
	return s.write(ctx, c, campaign, func(r Repository, me playdomain.Member, now time.Time) error {
		all, err := r.LootTables(ctx, campaign)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(all, func(t domain.LootTable) bool { return t.ID == id })
		if i < 0 {
			return apperr.ErrNotFound
		}
		used, err := r.LootTableInUse(ctx, id)
		switch {
		case err != nil:
			return err
		case used:
			return apperr.Refuse("another loot table still rolls on this one")
		}
		if _, err := r.DeleteLootTable(ctx, campaign, id); err != nil {
			return err
		}
		return r.RecordLootTable(ctx, campaign, revision(campaigndomain.ActionDelete, me, now), c, all[i])
	})
}

// LootTableRevisions lists a Loot Table's Revisions, newest first.
func (s *Service) LootTableRevisions(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.LootTableID) ([]campaigndomain.Revision, error) {
	if _, err := s.dm(ctx, c, campaign); err != nil {
		return nil, err
	}
	revs, err := s.Repo.Revisions(ctx, campaign, domain.EntityLoot, uuid.UUID(id))
	if err == nil && len(revs) == 0 {
		return nil, apperr.ErrNotFound
	}
	return revs, err
}

// RestoreLootTable brings a Loot Table back as it was at a Revision, deleted or not.
func (s *Service) RestoreLootTable(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.LootTableID, no int) (domain.LootTable, error) {
	var out domain.LootTable
	err := s.write(ctx, c, campaign, func(r Repository, me playdomain.Member, now time.Time) error {
		t, err := r.LootTableAt(ctx, campaign, id, no)
		if err != nil {
			return err
		}
		all, err := r.LootTables(ctx, campaign)
		if err != nil {
			return err
		}
		if t, err = s.cleanLoot(ctx, r, campaign, t, all); err != nil {
			return err
		}
		t.UpdatedAt = now
		if err := r.SaveLootTable(ctx, campaign, t, now); err != nil {
			return err
		}
		rev := revision(campaigndomain.ActionRestore, me, now)
		rev.RestoredFrom = no
		out = t
		return r.RecordLootTable(ctx, campaign, rev, c, t)
	})
	return out, err
}
