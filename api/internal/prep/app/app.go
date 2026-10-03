// Package app runs the random-encounter prep use cases. Pools and Tables are DM prep: every operation
// is DM only and every write is a Revision the DM can restore.
package app

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/encounters"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Repository is the persistence port for prep.
type Repository interface {
	InTx(ctx context.Context, fn func(r Repository) error) error
	// MonsterXP is a compendium monster's XP in the Campaign's ruleset; apperr.ErrNotFound when there is none.
	MonsterXP(ctx context.Context, campaign uuid.UUID, slug string) (int, error)
	Location(ctx context.Context, campaign, id uuid.UUID) error
	Locations(ctx context.Context, campaign uuid.UUID) ([]domain.Location, error)
	Pools(ctx context.Context, campaign uuid.UUID) ([]domain.Pool, error)
	SavePool(ctx context.Context, campaign uuid.UUID, p domain.Pool, now time.Time) error
	DeletePool(ctx context.Context, campaign uuid.UUID, id domain.PoolID) (bool, error)
	PoolInUse(ctx context.Context, id domain.PoolID) (bool, error)
	Tables(ctx context.Context, campaign uuid.UUID) ([]domain.Table, error)
	SaveTable(ctx context.Context, campaign uuid.UUID, t domain.Table, now time.Time) error
	DeleteTable(ctx context.Context, campaign uuid.UUID, id domain.TableID) (bool, error)
	RecordPool(ctx context.Context, campaign uuid.UUID, rev campaigndomain.Revision, c caller.Caller, p domain.Pool) error
	RecordTable(ctx context.Context, campaign uuid.UUID, rev campaigndomain.Revision, c caller.Caller, t domain.Table) error
	Revisions(ctx context.Context, campaign uuid.UUID, entity string, id uuid.UUID) ([]campaigndomain.Revision, error)
	PoolAt(ctx context.Context, campaign uuid.UUID, id domain.PoolID, no int) (domain.Pool, error)
	TableAt(ctx context.Context, campaign uuid.UUID, id domain.TableID, no int) (domain.Table, error)
	// ItemExists reports whether the compendium has an item.
	ItemExists(ctx context.Context, campaign uuid.UUID, slug string) (bool, error)
	LootTables(ctx context.Context, campaign uuid.UUID) ([]domain.LootTable, error)
	SaveLootTable(ctx context.Context, campaign uuid.UUID, t domain.LootTable, now time.Time) error
	DeleteLootTable(ctx context.Context, campaign uuid.UUID, id domain.LootTableID) (bool, error)
	LootTableInUse(ctx context.Context, id domain.LootTableID) (bool, error)
	RecordLootTable(ctx context.Context, campaign uuid.UUID, rev campaigndomain.Revision, c caller.Caller, t domain.LootTable) error
	LootTableAt(ctx context.Context, campaign uuid.UUID, id domain.LootTableID, no int) (domain.LootTable, error)
	Settlements(ctx context.Context, campaign uuid.UUID) ([]domain.Settlement, error)
	SaveSettlement(ctx context.Context, campaign uuid.UUID, s domain.Settlement, now time.Time) error
	DeleteSettlement(ctx context.Context, campaign uuid.UUID, id domain.SettlementID) (bool, error)
	RecordSettlement(ctx context.Context, campaign uuid.UUID, rev campaigndomain.Revision, c caller.Caller, s domain.Settlement) error
	SettlementAt(ctx context.Context, campaign uuid.UUID, id domain.SettlementID, no int) (domain.Settlement, error)
	Shops(ctx context.Context, campaign uuid.UUID) ([]domain.Shop, error)
	// SaveShop writes a Shop's fields; SetStock replaces its Stock and the day it was stocked.
	SaveShop(ctx context.Context, s domain.Shop, now time.Time) error
	SetStock(ctx context.Context, id domain.ShopID, stock []domain.StockItem, day int) error
	DeleteShop(ctx context.Context, campaign uuid.UUID, id domain.ShopID) (bool, error)
	RecordShop(ctx context.Context, campaign uuid.UUID, rev campaigndomain.Revision, c caller.Caller, s domain.Shop) error
	ShopAt(ctx context.Context, campaign uuid.UUID, id domain.ShopID, no int) (domain.Shop, error)
	NpcExists(ctx context.Context, campaign, id uuid.UUID) (bool, error)
	FactionExists(ctx context.Context, campaign, id uuid.UUID) (bool, error)
	ItemPrices(ctx context.Context, campaign uuid.UUID, slugs []string) (map[string]domain.ItemPrice, error)
	GameDay(ctx context.Context, campaign uuid.UUID) (int, error)
	// Checks lists the Campaign's latest Encounter Checks, newest first.
	Checks(ctx context.Context, campaign uuid.UUID) ([]domain.Check, error)
}

// Members finds who a caller is in a Campaign.
type Members interface {
	Membership(ctx context.Context, campaign uuid.UUID, subject string) (playdomain.Member, error)
}

// Service runs the prep use cases.
type Service struct {
	Repo    Repository
	Members Members
	Now     func() time.Time
	// Seed and Source drive Stock rolls.
	Seed   func() uint64
	Source func(seed uint64) dice.Source
}

func (s *Service) dm(ctx context.Context, c caller.Caller, campaign uuid.UUID) (playdomain.Member, error) {
	me, err := s.Members.Membership(ctx, campaign, c.Subject)
	if err != nil {
		return playdomain.Member{}, err
	}
	if !me.DM {
		return playdomain.Member{}, apperr.ErrForbidden
	}
	return me, nil
}

func name(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || utf8.RuneCountInString(s) > 80 {
		return "", apperr.Refuse("give it a name of up to 80 characters")
	}
	return s, nil
}

// monster checks that a creature exists in the compendium.
func monster(ctx context.Context, r Repository, campaign uuid.UUID, slug string) error {
	if _, err := r.MonsterXP(ctx, campaign, slug); err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			return apperr.Refuse("there is no monster " + slug)
		}
		return err
	}
	return nil
}

func (s *Service) cleanPool(ctx context.Context, r Repository, campaign uuid.UUID, p domain.Pool) (domain.Pool, error) {
	n, err := name(p.Name)
	if err != nil {
		return p, err
	}
	p.Name = n
	_, known := encounters.ParseDifficulty(p.Difficulty)
	switch {
	case p.LevelMin < 1 || p.LevelMax > 20 || p.LevelMin > p.LevelMax:
		return p, apperr.Refuse("levels run from 1 to 20, lowest first")
	case !known:
		return p, apperr.Refuse("difficulty is low, moderate or high")
	case len(p.Members) == 0 || len(p.Members) > 20:
		return p, apperr.Refuse("a pool holds 1 to 20 creatures")
	}
	for _, m := range p.Members {
		if m.Weight < 1 || m.Weight > 100 || m.Min < 0 || m.Max < 1 || m.Max > 20 || m.Min > m.Max {
			return p, apperr.Refuse("weights run from 1 to 100, counts from 0 to 20 with the fewest first")
		}
		if err := monster(ctx, r, campaign, m.Slug); err != nil {
			return p, err
		}
	}
	return p, nil
}

func (s *Service) cleanTable(ctx context.Context, r Repository, campaign uuid.UUID, t domain.Table) (domain.Table, error) {
	n, err := name(t.Name)
	if err != nil {
		return t, err
	}
	t.Name = n
	switch {
	case t.ChancePct < 0 || t.ChancePct > 100:
		return t, apperr.Refuse("the chance runs from 0 to 100 percent")
	case t.Visibility != domain.Secret && t.Visibility != domain.Open:
		return t, apperr.Refuse("checks are secret or open")
	case len(t.Entries) == 0 || len(t.Entries) > 50:
		return t, apperr.Refuse("a table holds 1 to 50 entries")
	}
	if t.RegionID != nil {
		err := r.Location(ctx, campaign, *t.RegionID)
		if errors.Is(err, apperr.ErrNotFound) {
			return t, apperr.Refuse("choose a location on one of the campaign's world maps")
		}
		if err != nil {
			return t, err
		}
	}
	pools, err := r.Pools(ctx, campaign)
	if err != nil {
		return t, err
	}
	for i, e := range t.Entries {
		if t.Entries[i], err = cleanEntry(ctx, r, campaign, e, pools); err != nil {
			return t, err
		}
	}
	return t, nil
}

func cleanEntry(ctx context.Context, r Repository, campaign uuid.UUID, e domain.Entry, pools []domain.Pool) (domain.Entry, error) {
	e.Label = strings.TrimSpace(e.Label)
	if e.Weight < 1 || e.Weight > 100 || utf8.RuneCountInString(e.Label) > 80 {
		return e, apperr.Refuse("weights run from 1 to 100 and labels to 80 characters")
	}
	if e.FactionID != nil {
		if ok, err := r.FactionExists(ctx, campaign, *e.FactionID); err != nil {
			return e, err
		} else if !ok {
			return e, apperr.Refuse("choose one of the campaign's Factions")
		}
	}
	switch e.Kind {
	case domain.EntryNothing:
		e.PoolID, e.Monsters = nil, nil
	case domain.EntryPool:
		if e.PoolID == nil || !slices.ContainsFunc(pools, func(p domain.Pool) bool { return p.ID == *e.PoolID }) {
			return e, apperr.Refuse("choose one of the campaign's pools")
		}
		e.Monsters = nil
	case domain.EntryEncounter:
		e.PoolID = nil
		return e, cleanEncounter(ctx, r, campaign, e)
	default:
		return e, apperr.Refuse("an entry is an encounter, a pool draw or nothing")
	}
	return e, nil
}

func cleanEncounter(ctx context.Context, r Repository, campaign uuid.UUID, e domain.Entry) error {
	if e.Label == "" || len(e.Monsters) == 0 || len(e.Monsters) > 10 {
		return apperr.Refuse("a prepared encounter has a name and 1 to 10 kinds of creature")
	}
	for _, m := range e.Monsters {
		if m.Count < 1 || m.Count > 20 {
			return apperr.Refuse("an encounter holds 1 to 20 of each creature")
		}
		if err := monster(ctx, r, campaign, m.Slug); err != nil {
			return err
		}
	}
	return nil
}

// write runs fn in a transaction as the DM and records its Revision.
func (s *Service) write(ctx context.Context, c caller.Caller, campaign uuid.UUID, fn func(r Repository, me playdomain.Member, now time.Time) error) error {
	return s.Repo.InTx(ctx, func(r Repository) error {
		me, err := s.dm(ctx, c, campaign)
		if err != nil {
			return err
		}
		return fn(r, me, s.Now())
	})
}

func revision(action campaigndomain.RevisionAction, me playdomain.Member, now time.Time) campaigndomain.Revision {
	return campaigndomain.Revision{Action: action, Author: me.Name, CreatedAt: now}
}
