package pgstore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	preppg "github.com/JorisJonkers-dev/grimoire/api/internal/prep/pgstore"
)

// LoadLoot reads the Campaign's Loot Tables.
func (s *Store) LoadLoot(ctx context.Context, campaign uuid.UUID) ([]prep.LootTable, error) {
	return preppg.LoadLootTables(ctx, s.q, campaign)
}

// Items reads what items are called and weigh.
func (s *Store) Items(ctx context.Context, campaign uuid.UUID, slugs []string) (map[string]domain.ItemInfo, error) {
	out := map[string]domain.ItemInfo{}
	if len(slugs) == 0 {
		return out, nil
	}
	rows, err := s.q.ItemsBySlug(ctx, queries.ItemsBySlugParams{Slugs: slugs, CampaignID: campaign})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.Slug] = domain.ItemInfo{
			Name: r.Name, WeightLb: r.WeightLb, Category: r.Category, RequiresAttunement: r.RequiresAttunement, AttunementDetail: r.AttunementDetail,
			MaxCharges: int(r.MaxCharges), RegainDice: int(r.RegainDice), RegainFaces: int(r.RegainFaces), RegainBonus: int(r.RegainBonus), RechargeOn: r.RechargeOn,
		}
	}
	return out, nil
}

// LoadInventory reads every Container of the Campaign, first making the Party Stash and an Inventory
// for each Character that has none yet.
func (s *Store) LoadInventory(ctx context.Context, campaign uuid.UUID) (domain.Inventory, error) {
	inv := domain.Inventory{Containers: []domain.Container{}, Bearers: []domain.Bearer{}}
	now := time.Now()
	if err := s.q.InsertContainer(ctx, queries.InsertContainerParams{ID: uuid.New(), CampaignID: campaign, Kind: domain.ContainerStash, Label: "Party Stash", Now: now}); err != nil {
		return inv, err
	}
	chars, err := s.q.InventoryCharacters(ctx, campaign)
	if err != nil {
		return inv, err
	}
	for _, c := range chars {
		inv.Bearers = append(inv.Bearers, domain.Bearer{CharacterID: c.ID, Name: c.Name, Owner: c.OwnerMemberID, Strength: int(c.Strength), Classes: c.Classes, WeaponSet: c.WeaponSet})
		p := queries.InsertContainerParams{ID: uuid.New(), CampaignID: campaign, Kind: domain.ContainerCharacter, CharacterID: pgtype.UUID{Bytes: c.ID, Valid: true}, Label: c.Name, Now: now}
		if err := s.q.InsertContainer(ctx, p); err != nil {
			return inv, err
		}
	}
	return s.readContainers(ctx, campaign, inv)
}

func (s *Store) readContainers(ctx context.Context, campaign uuid.UUID, inv domain.Inventory) (domain.Inventory, error) {
	rows, err := s.q.CampaignContainers(ctx, campaign)
	if err != nil {
		return inv, err
	}
	instances, err := s.q.CampaignItemInstances(ctx, campaign)
	if err != nil {
		return inv, err
	}
	coins, err := s.q.CampaignContainerCoins(ctx, campaign)
	if err != nil {
		return inv, err
	}
	var slugs []string
	for _, r := range rows {
		c := domain.Container{ID: domain.ContainerID(r.ID), Kind: r.Kind, Label: r.Label, Items: map[string]int{}, Coins: map[string]int{}, CreatedAt: r.CreatedAt}
		if r.CharacterID.Valid {
			id := uuid.UUID(r.CharacterID.Bytes)
			c.CharacterID = &id
		}
		if r.ParentID.Valid {
			id := domain.ContainerID(r.ParentID.Bytes)
			c.ParentID = &id
		}
		inv.Containers = append(inv.Containers, c)
	}
	byID := map[uuid.UUID]*domain.Container{}
	for i := range inv.Containers {
		byID[uuid.UUID(inv.Containers[i].ID)] = &inv.Containers[i]
	}
	for _, r := range instances {
		c, in := byID[r.ContainerID], instanceOf(r)
		if in.Plain() {
			c.Items[in.Slug] = in.Quantity
		} else {
			c.Instances = append(c.Instances, in)
		}
		slugs = append(slugs, r.ItemSlug)
	}
	for _, k := range coins {
		byID[k.ContainerID].Coins[k.Coin] = int(k.Amount)
	}
	inv.Items, err = s.Items(ctx, campaign, slugs)
	return inv, err
}

func instanceOf(r queries.CampaignItemInstancesRow) domain.Instance {
	in := domain.Instance{
		ID: domain.InstanceID(r.ID), Slug: r.ItemSlug, CustomName: r.CustomName.String, Quantity: int(r.Quantity),
		Identified: r.Identified, Attuned: r.Attuned, Slot: r.EquippedSlot.String,
	}
	if r.Charges.Valid {
		n := int(r.Charges.Int32)
		in.Charges = &n
	}
	return in
}

// saveInventory writes a drop of loot, or the two Containers a transfer changed, and clears an emptied drop.
func (s *Store) saveInventory(ctx context.Context, sess domain.Session, w live.Write, now time.Time) error {
	if d := w.Drop; d != nil {
		if err := s.q.InsertContainer(ctx, queries.InsertContainerParams{ID: uuid.UUID(d.ID), CampaignID: sess.CampaignID, Kind: d.Kind, Label: d.Label, Now: now}); err != nil {
			return err
		}
		for slug, n := range d.Items {
			if err := s.setCount(ctx, d.ID, slug, "", n); err != nil {
				return err
			}
		}
		for coin, n := range d.Coins {
			if err := s.setCount(ctx, d.ID, "", coin, n); err != nil {
				return err
			}
		}
		return nil
	}
	if w.Move == nil {
		return nil
	}
	if w.Gone != nil {
		return s.moveCount(ctx, *w.Move, true)
	}
	return s.moveCount(ctx, *w.Move, false)
}

// moveCount writes a transfer's new counts on both sides; an emptied drop is deleted instead.
func (s *Store) moveCount(ctx context.Context, mv domain.Move, gone bool) error {
	if mv.Instance != nil {
		if err := s.q.MoveInstance(ctx, queries.MoveInstanceParams{ID: uuid.UUID(*mv.Instance), ContainerID: uuid.UUID(mv.To)}); err != nil {
			return err
		}
	}
	if gone {
		if err := s.q.DeleteContainer(ctx, uuid.UUID(mv.From)); err != nil {
			return err
		}
	} else if mv.Instance == nil {
		if err := s.setCount(ctx, mv.From, mv.Item, mv.Coin, mv.Left); err != nil {
			return err
		}
	}
	if mv.Instance != nil {
		return nil
	}
	return s.setCount(ctx, mv.To, mv.Item, mv.Coin, mv.Now)
}

//nolint:gosec // counts are bounded by the rules
func (s *Store) setCount(ctx context.Context, id domain.ContainerID, item, coin string, n int) error {
	cid := uuid.UUID(id)
	switch {
	case coin != "" && n > 0:
		return s.q.SetContainerCoins(ctx, queries.SetContainerCoinsParams{ContainerID: cid, Coin: coin, Amount: int32(n)})
	case coin != "":
		return s.q.DeleteContainerCoins(ctx, queries.DeleteContainerCoinsParams{ContainerID: cid, Coin: coin})
	case n > 0:
		return s.q.SetStack(ctx, queries.SetStackParams{ID: uuid.New(), ContainerID: cid, ItemSlug: item, Quantity: int32(n), Now: time.Now()})
	default:
		return s.q.DeleteStack(ctx, queries.DeleteStackParams{ContainerID: cid, ItemSlug: item})
	}
}

// logItems records what a drop or transfer moved against its Action.
//
//nolint:gosec // counts are bounded by the rules
func (s *Store) logItems(ctx context.Context, actionID uuid.UUID, w live.Write) error {
	type line struct {
		from, to, item, coin string
		n                    int
	}
	var lines []line
	if d := w.Drop; d != nil {
		for slug, n := range d.Items {
			lines = append(lines, line{from: "Loot table", to: d.Label, item: slug, n: n})
		}
		for coin, n := range d.Coins {
			lines = append(lines, line{from: "Loot table", to: d.Label, coin: coin, n: n})
		}
	}
	if mv := w.Move; mv != nil {
		lines = append(lines, line{from: mv.FromLabel, to: mv.ToLabel, item: mv.Item, coin: mv.Coin, n: mv.Count})
	}
	if t := w.Trade; t != nil && w.Kind == domain.ActionItemBought {
		lines = append(lines, line{from: t.Shop, to: t.Label, item: t.Item, n: t.Count}, line{from: t.Label, to: t.Shop, coin: "cp", n: t.PriceCP})
	}
	if t := w.Trade; t != nil && w.Kind == domain.ActionItemSold {
		lines = append(lines, line{from: t.Label, to: t.Shop, item: t.Item, n: t.Count})
		if t.PriceCP > 0 {
			lines = append(lines, line{from: t.Shop, to: t.Label, coin: "cp", n: t.PriceCP})
		}
	}
	for i, l := range lines {
		p := queries.InsertItemEventParams{ActionID: actionID, Position: int32(i), FromLabel: l.from, ToLabel: l.to, Count: int32(l.n)}
		p.ItemSlug = pgtype.Text{String: l.item, Valid: l.item != ""}
		p.Coin = pgtype.Text{String: l.coin, Valid: l.coin != ""}
		if err := s.q.InsertItemEvent(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

// LiveSession reports whether the Campaign has a Session running.
func (s *Store) LiveSession(ctx context.Context, campaign uuid.UUID) (bool, error) {
	return s.q.CampaignHasLiveSession(ctx, campaign)
}

// WriteInventory runs Inventory changes in one transaction.
func (s *Store) WriteInventory(ctx context.Context, fn func(app.InventoryWriter) error) error {
	return s.InTx(ctx, func(r app.Repository) error { return fn(r.(*Store)) })
}

// SetSlot puts an Item Instance in an equipment slot, or none.
func (s *Store) SetSlot(ctx context.Context, id domain.InstanceID, slot string) error {
	return s.q.SetInstanceSlot(ctx, queries.SetInstanceSlotParams{ID: uuid.UUID(id), EquippedSlot: pgtype.Text{String: slot, Valid: slot != ""}})
}

// AddInstance adds one item as its own Instance, in a slot or none, attuned or not.
func (s *Store) AddInstance(ctx context.Context, id domain.InstanceID, to domain.ContainerID, slug, slot string, attuned bool) error {
	return s.q.InsertInstance(ctx, queries.InsertInstanceParams{
		ID: uuid.UUID(id), ContainerID: uuid.UUID(to), ItemSlug: slug, EquippedSlot: pgtype.Text{String: slot, Valid: slot != ""}, Attuned: attuned, Now: time.Now(),
	})
}

// SetAttuned attunes or unattunes an Item Instance.
func (s *Store) SetAttuned(ctx context.Context, id domain.InstanceID, attuned bool) error {
	return s.q.SetInstanceAttuned(ctx, queries.SetInstanceAttunedParams{ID: uuid.UUID(id), Attuned: attuned})
}

// Identify reveals what an Item Instance is.
func (s *Store) Identify(ctx context.Context, id domain.InstanceID) error {
	return s.q.IdentifyInstance(ctx, uuid.UUID(id))
}

// SetCharges sets the charges an Item Instance holds.
func (s *Store) SetCharges(ctx context.Context, id domain.InstanceID, n int) error {
	return s.q.SetInstanceCharges(ctx, queries.SetInstanceChargesParams{ID: uuid.UUID(id), Charges: pgtype.Int4{Int32: int32(n), Valid: true}}) //nolint:gosec // 0 to 100
}

// SetWeaponSet changes which weapon set a Character holds.
func (s *Store) SetWeaponSet(ctx context.Context, character uuid.UUID, set string) error {
	return s.q.SetWeaponSet(ctx, queries.SetWeaponSetParams{ID: character, WeaponSet: set})
}

// RemoveInstance takes an Item Instance out of the Campaign: drunk, thrown or merged into a stack.
func (s *Store) RemoveInstance(ctx context.Context, id domain.InstanceID) error {
	return s.q.DeleteInstance(ctx, uuid.UUID(id))
}

// MoveInstance moves an Item Instance to another Container, out of any slot.
func (s *Store) MoveInstance(ctx context.Context, id domain.InstanceID, to domain.ContainerID) error {
	return s.q.MoveInstance(ctx, queries.MoveInstanceParams{ID: uuid.UUID(id), ContainerID: uuid.UUID(to)})
}

// SetStack sets how many of a plain item a Container holds.
func (s *Store) SetStack(ctx context.Context, in domain.ContainerID, slug string, n int) error {
	return s.setCount(ctx, in, slug, "", n)
}

// Heal restores a Character's hit points, up to its maximum.
func (s *Store) Heal(ctx context.Context, character uuid.UUID, hp int) error {
	return s.q.HealCharacter(ctx, queries.HealCharacterParams{ID: character, Amount: int32(hp)}) //nolint:gosec // potion dice are small
}

// SyncEquipment sets the armor, shield and weapons a Character's sheet is built with.
func (s *Store) SyncEquipment(ctx context.Context, character uuid.UUID, armor string, shield bool, weapons []string) error {
	if err := s.q.SetCharacterArmor(ctx, queries.SetCharacterArmorParams{ID: character, ArmorSlug: pgtype.Text{String: armor, Valid: armor != ""}, Shield: shield}); err != nil {
		return err
	}
	if err := s.q.ClearCharacterWeapons(ctx, character); err != nil {
		return err
	}
	for i, w := range weapons {
		if err := s.q.AddCharacterWeapon(ctx, queries.AddCharacterWeaponParams{CharacterID: character, WeaponSlug: w, Ordering: int32(i)}); err != nil { //nolint:gosec // a handful
			return err
		}
	}
	return nil
}
