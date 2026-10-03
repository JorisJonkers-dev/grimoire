package app

import (
	"context"

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
	e, err := s.designed(ctx, c, id, "item", "item builder")
	if err != nil {
		return ItemBuild{}, err
	}
	return itemBuild(e, e.Name, designOf(e, firstItem())), nil
}

// SaveItem saves a design for one of the caller's items as its next Revision.
func (s *Service) SaveItem(ctx context.Context, c caller.Caller, id uuid.UUID, d itembuild.Design) (ItemBuild, error) {
	e, err := s.ownDesigned(ctx, c, id, "item", "item builder")
	if err != nil {
		return ItemBuild{}, err
	}
	if err := itembuild.Check(d); err != nil {
		return ItemBuild{}, apperr.Refuse(err.Error())
	}
	if err := s.saveDesign(ctx, c, e, d); err != nil {
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
