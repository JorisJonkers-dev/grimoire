package app

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/itembuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// ItemBuild is a homebrew item in the item builder: its entry, its design, the slug it is known by in
// play, its card as players read it once it is known, and its Price Check.
type ItemBuild struct {
	Entry  domain.Entry
	Design itembuild.Design
	Slug   string
	Card   []string
	Price  itembuild.PriceCheck
}

func firstItem() itembuild.Design {
	return itembuild.Design{Kind: "trinket", Rarity: "common", ValueGP: 100, Properties: []itembuild.Property{}}
}

func itemBuild(e domain.Entry, name string, d itembuild.Design) ItemBuild {
	return ItemBuild{Entry: e, Design: d, Slug: spellbuild.Slug(e.ID.String()), Card: itembuild.Card(name, d, true), Price: itembuild.Price(d)}
}

// Item reads a homebrew item in the builder: one of the caller's, or a Shared Library copy.
func (s *Service) Item(ctx context.Context, c caller.Caller, id uuid.UUID) (ItemBuild, error) {
	e, err := s.linkable(ctx, c, id)
	if err == nil && e.Kind != "item" {
		err = apperr.Refuse("only an item is built in the item builder")
	}
	if err != nil {
		return ItemBuild{}, err
	}
	d := firstItem()
	if e.Design != nil {
		_ = json.Unmarshal(e.Design, &d) // stored designs were checked when saved
	}
	return itemBuild(e, e.Name, d), nil
}

// SaveItem saves a design for one of the caller's items as its next Revision.
func (s *Service) SaveItem(ctx context.Context, c caller.Caller, id uuid.UUID, d itembuild.Design) (ItemBuild, error) {
	e, err := s.owned(ctx, c, id)
	if err == nil && e.Kind != "item" {
		err = apperr.Refuse("only an item is built in the item builder")
	}
	if err != nil {
		return ItemBuild{}, err
	}
	if err := itembuild.Check(d); err != nil {
		return ItemBuild{}, apperr.Refuse(err.Error())
	}
	raw, _ := json.Marshal(d) //nolint:errchkjson // a design is plain data
	now := s.Now()
	err = s.Repo.InTx(ctx, func(r Repository) error {
		no, err := r.UpdateEntry(ctx, id, e.Name, e.Fields, raw, now)
		if err != nil {
			return err
		}
		return r.InsertRevision(ctx, id, domain.Revision{No: no, Name: e.Name, Fields: e.Fields, Design: raw, Author: c.Subject, At: now})
	})
	if err != nil {
		return ItemBuild{}, err
	}
	return s.Item(ctx, c, id)
}

// PreviewItem checks a design without saving it: its card and its Price Check.
func (s *Service) PreviewItem(name string, d itembuild.Design) (ItemBuild, error) {
	if err := itembuild.Check(d); err != nil {
		return ItemBuild{}, apperr.Refuse(err.Error())
	}
	return ItemBuild{Design: d, Card: itembuild.Card(name, d, true), Price: itembuild.Price(d)}, nil
}
