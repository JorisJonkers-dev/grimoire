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
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/shops"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Settlements lists the Campaign's Settlements. DM only.
func (s *Service) Settlements(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.Settlement, error) {
	if _, err := s.dm(ctx, c, campaign); err != nil {
		return nil, err
	}
	return s.Repo.Settlements(ctx, campaign)
}

func cleanSettlement(ctx context.Context, r Repository, campaign uuid.UUID, x domain.Settlement) (domain.Settlement, error) {
	n, err := name(x.Name)
	if err != nil {
		return x, err
	}
	x.Name = n
	switch {
	case !slices.Contains(shops.Sizes(), x.Size):
		return x, apperr.Refuse("a settlement is a hamlet, village, town or city")
	case !slices.Contains(shops.Wealths(), x.Wealth):
		return x, apperr.Refuse("a settlement is poor, modest, comfortable or wealthy")
	}
	if x.LocationID != nil {
		err := r.Location(ctx, campaign, *x.LocationID)
		if errors.Is(err, apperr.ErrNotFound) {
			return x, apperr.Refuse("choose a location on one of the campaign's world maps")
		}
		return x, err
	}
	return x, nil
}

// SaveSettlement creates a Settlement, or replaces the one with the same id.
func (s *Service) SaveSettlement(ctx context.Context, c caller.Caller, campaign uuid.UUID, x domain.Settlement) (domain.Settlement, error) {
	err := s.write(ctx, c, campaign, func(r Repository, me playdomain.Member, now time.Time) error {
		all, err := r.Settlements(ctx, campaign)
		if err != nil {
			return err
		}
		action := campaigndomain.ActionCreate
		if x.ID != (domain.SettlementID{}) {
			if !slices.ContainsFunc(all, func(y domain.Settlement) bool { return y.ID == x.ID }) {
				return apperr.ErrNotFound
			}
			action = campaigndomain.ActionUpdate
		} else {
			x.ID = domain.SettlementID(uuid.New())
		}
		if x, err = cleanSettlement(ctx, r, campaign, x); err != nil {
			return err
		}
		x.UpdatedAt = now
		if err := r.SaveSettlement(ctx, campaign, x, now); err != nil {
			return err
		}
		return r.RecordSettlement(ctx, campaign, revision(action, me, now), c, x)
	})
	return x, err
}

// DeleteSettlement removes a Settlement that has no Shops left; its Revisions keep it restorable.
func (s *Service) DeleteSettlement(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.SettlementID) error {
	return s.write(ctx, c, campaign, func(r Repository, me playdomain.Member, now time.Time) error {
		all, err := r.Settlements(ctx, campaign)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(all, func(x domain.Settlement) bool { return x.ID == id })
		if i < 0 {
			return apperr.ErrNotFound
		}
		list, err := r.Shops(ctx, campaign)
		switch {
		case err != nil:
			return err
		case slices.ContainsFunc(list, func(x domain.Shop) bool { return x.SettlementID == id }):
			return apperr.Refuse("remove the settlement's shops first")
		}
		if _, err := r.DeleteSettlement(ctx, campaign, id); err != nil {
			return err
		}
		return r.RecordSettlement(ctx, campaign, revision(campaigndomain.ActionDelete, me, now), c, all[i])
	})
}

// SettlementRevisions lists a Settlement's Revisions, newest first.
func (s *Service) SettlementRevisions(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.SettlementID) ([]campaigndomain.Revision, error) {
	return s.revisions(ctx, c, campaign, domain.EntitySettlement, uuid.UUID(id))
}

func (s *Service) revisions(ctx context.Context, c caller.Caller, campaign uuid.UUID, entity string, id uuid.UUID) ([]campaigndomain.Revision, error) {
	if _, err := s.dm(ctx, c, campaign); err != nil {
		return nil, err
	}
	revs, err := s.Repo.Revisions(ctx, campaign, entity, id)
	if err == nil && len(revs) == 0 {
		return nil, apperr.ErrNotFound
	}
	return revs, err
}

// RestoreSettlement brings a Settlement back as it was at a Revision, deleted or not.
func (s *Service) RestoreSettlement(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.SettlementID, no int) (domain.Settlement, error) {
	var out domain.Settlement
	err := s.write(ctx, c, campaign, func(r Repository, me playdomain.Member, now time.Time) error {
		x, err := r.SettlementAt(ctx, campaign, id, no)
		if err != nil {
			return err
		}
		if x, err = cleanSettlement(ctx, r, campaign, x); err != nil {
			return err
		}
		x.UpdatedAt = now
		if err := r.SaveSettlement(ctx, campaign, x, now); err != nil {
			return err
		}
		rev := revision(campaigndomain.ActionRestore, me, now)
		rev.RestoredFrom, out = no, x
		return r.RecordSettlement(ctx, campaign, rev, c, x)
	})
	return out, err
}

// Shops lists the Campaign's Shops with their Stock. DM only.
func (s *Service) Shops(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.Shop, error) {
	if _, err := s.dm(ctx, c, campaign); err != nil {
		return nil, err
	}
	return s.Repo.Shops(ctx, campaign)
}

func cleanShop(ctx context.Context, r Repository, campaign uuid.UUID, x domain.Shop) (domain.Shop, error) {
	n, err := name(x.Name)
	if err != nil {
		return x, err
	}
	x.Name, x.Kind = n, strings.TrimSpace(x.Kind)
	switch {
	case x.Kind == "" || utf8.RuneCountInString(x.Kind) > 40:
		return x, apperr.Refuse("give the shop a trade of up to 40 characters")
	case x.MarkupPct < 0 || x.MarkupPct > 300:
		return x, apperr.Refuse("a markup runs from 0 to 300 percent")
	case x.HaggleDC < 5 || x.HaggleDC > 30 || x.HagglePct < 0 || x.HagglePct > 50:
		return x, apperr.Refuse("haggling has a DC from 5 to 30 and moves prices up to 50 percent")
	case x.Restock != domain.RestockNever && x.Restock != domain.RestockLongRest && x.Restock != domain.RestockDays:
		return x, apperr.Refuse("a shop restocks never, after a long rest, or every few days")
	case x.Restock == domain.RestockDays && (x.RestockDays < 1 || x.RestockDays > 365):
		return x, apperr.Refuse("a shop restocks every 1 to 365 days")
	}
	if x.Restock != domain.RestockDays {
		x.RestockDays = 0
	}
	return x, refs(ctx, r, campaign, x)
}

// refs checks the Settlement, owner and Loot Table a Shop names belong to the Campaign.
func refs(ctx context.Context, r Repository, campaign uuid.UUID, x domain.Shop) error {
	settlements, err := r.Settlements(ctx, campaign)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(settlements, func(y domain.Settlement) bool { return y.ID == x.SettlementID }) {
		return apperr.Refuse("choose one of the campaign's settlements")
	}
	if x.OwnerID != nil {
		ok, err := r.NpcExists(ctx, campaign, *x.OwnerID)
		if err != nil {
			return err
		}
		if !ok {
			return apperr.Refuse("choose one of the campaign's NPCs as the owner")
		}
	}
	if x.FactionID != nil {
		ok, err := r.FactionExists(ctx, campaign, *x.FactionID)
		if err != nil {
			return err
		}
		if !ok {
			return apperr.Refuse("choose one of the campaign's Factions")
		}
	}
	if x.LootTable == nil {
		return nil
	}
	tables, err := r.LootTables(ctx, campaign)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(tables, func(t domain.LootTable) bool { return t.ID == *x.LootTable }) {
		return apperr.Refuse("choose one of the campaign's loot tables for the stock")
	}
	return nil
}

// SaveShop creates a Shop, or replaces the one with the same id; its Stock stays as it was.
func (s *Service) SaveShop(ctx context.Context, c caller.Caller, campaign uuid.UUID, x domain.Shop) (domain.Shop, error) {
	err := s.write(ctx, c, campaign, func(r Repository, me playdomain.Member, now time.Time) error {
		all, err := r.Shops(ctx, campaign)
		if err != nil {
			return err
		}
		action := campaigndomain.ActionCreate
		x.Stock = []domain.StockItem{}
		if i := slices.IndexFunc(all, func(y domain.Shop) bool { return y.ID == x.ID }); i >= 0 {
			action, x.Stock, x.StockedDay = campaigndomain.ActionUpdate, all[i].Stock, all[i].StockedDay
		} else if x.ID != (domain.ShopID{}) {
			return apperr.ErrNotFound
		} else {
			x.ID = domain.ShopID(uuid.New())
		}
		if x, err = cleanShop(ctx, r, campaign, x); err != nil {
			return err
		}
		x.UpdatedAt = now
		if err := r.SaveShop(ctx, x, now); err != nil {
			return err
		}
		return r.RecordShop(ctx, campaign, revision(action, me, now), c, x)
	})
	return x, err
}

// DeleteShop removes a Shop; its Revisions keep it, Stock and all, restorable.
func (s *Service) DeleteShop(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.ShopID) error {
	return s.write(ctx, c, campaign, func(r Repository, me playdomain.Member, now time.Time) error {
		all, err := r.Shops(ctx, campaign)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(all, func(x domain.Shop) bool { return x.ID == id })
		if i < 0 {
			return apperr.ErrNotFound
		}
		if _, err := r.DeleteShop(ctx, campaign, id); err != nil {
			return err
		}
		return r.RecordShop(ctx, campaign, revision(campaigndomain.ActionDelete, me, now), c, all[i])
	})
}

// ShopRevisions lists a Shop's Revisions, newest first.
func (s *Service) ShopRevisions(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.ShopID) ([]campaigndomain.Revision, error) {
	return s.revisions(ctx, c, campaign, domain.EntityShop, uuid.UUID(id))
}

// RestoreShop brings a Shop and its Stock back as they were at a Revision, deleted or not.
func (s *Service) RestoreShop(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.ShopID, no int) (domain.Shop, error) {
	var out domain.Shop
	err := s.write(ctx, c, campaign, func(r Repository, me playdomain.Member, now time.Time) error {
		x, err := r.ShopAt(ctx, campaign, id, no)
		if err != nil {
			return err
		}
		if x, err = cleanShop(ctx, r, campaign, x); err != nil {
			return err
		}
		x.UpdatedAt = now
		if err := r.SaveShop(ctx, x, now); err != nil {
			return err
		}
		if err := r.SetStock(ctx, x.ID, x.Stock, x.StockedDay); err != nil {
			return err
		}
		rev := revision(campaigndomain.ActionRestore, me, now)
		rev.RestoredFrom, out = no, x
		return r.RecordShop(ctx, campaign, rev, c, x)
	})
	return out, err
}

// RerollStock generates a Shop's Stock afresh from its Loot Table, as a Revision.
func (s *Service) RerollStock(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.ShopID) (domain.Shop, error) {
	var out domain.Shop
	err := s.write(ctx, c, campaign, func(r Repository, me playdomain.Member, now time.Time) error {
		all, err := r.Shops(ctx, campaign)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(all, func(x domain.Shop) bool { return x.ID == id })
		if i < 0 {
			return apperr.ErrNotFound
		}
		x := all[i]
		if x.Stock, err = Stock(ctx, r, campaign, x, s.Source(s.Seed())); err != nil {
			return err
		}
		if x.StockedDay, err = r.GameDay(ctx, campaign); err != nil {
			return err
		}
		if err := r.SetStock(ctx, x.ID, x.Stock, x.StockedDay); err != nil {
			return err
		}
		out = x
		return r.RecordShop(ctx, campaign, revision(campaigndomain.ActionUpdate, me, now), c, x)
	})
	return out, err
}
