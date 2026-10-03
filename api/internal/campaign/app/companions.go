package app

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// CompanionRepository keeps a Campaign's Companions.
type CompanionRepository interface {
	Membership(ctx context.Context, id domain.CampaignID, subject string) (domain.Member, error)
	Member(ctx context.Context, id domain.CampaignID, m domain.MemberID) (domain.Member, error)
	Companions(ctx context.Context, id domain.CampaignID) ([]domain.Companion, error)
	InsertCompanion(ctx context.Context, c domain.Companion, now time.Time) error
	// UpdateCompanion and DeleteCompanion report ErrNotFound for a Companion the Campaign does not have.
	UpdateCompanion(ctx context.Context, c domain.Companion, now time.Time) error
	DeleteCompanion(ctx context.Context, id domain.CampaignID, companion domain.CompanionID) error
}

// Creatures tells whether a Campaign has a creature to make a Companion of.
type Creatures interface {
	Creature(ctx context.Context, campaign domain.CampaignID, slug string) error
}

// Companions runs the Companion use cases: every Member sees the party's Companions, the DM keeps them.
type Companions struct {
	Repo      CompanionRepository
	Creatures Creatures
	Now       func() time.Time
}

// CompanionInput is the editable part of a Companion.
type CompanionInput struct {
	Name        string
	Kind        string
	MonsterSlug string
	Controller  *domain.MemberID
	SharesXP    bool
	Notes       string
}

// List shows the party's Companions to any Member; the DM's notes stay the DM's.
func (s *Companions) List(ctx context.Context, c caller.Caller, id domain.CampaignID) ([]domain.Companion, error) {
	me, err := s.Repo.Membership(ctx, id, c.Subject)
	if err != nil {
		return nil, err
	}
	list, err := s.Repo.Companions(ctx, id)
	if err != nil {
		return nil, err
	}
	if me.Role != domain.RoleDM {
		for i := range list {
			list[i].Notes = ""
		}
	}
	return list, nil
}

// checked is the Companion an input describes, when the DM asks and all of it holds.
func (s *Companions) checked(ctx context.Context, c caller.Caller, id domain.CampaignID, in CompanionInput) (domain.Companion, error) {
	me, err := s.Repo.Membership(ctx, id, c.Subject)
	if err != nil {
		return domain.Companion{}, err
	}
	if me.Role != domain.RoleDM {
		return domain.Companion{}, domain.ErrForbidden
	}
	name, slug := strings.TrimSpace(in.Name), strings.TrimSpace(in.MonsterSlug)
	switch {
	case name == "" || utf8.RuneCountInString(name) > 40 || slug == "" || len(slug) > 120 || utf8.RuneCountInString(in.Notes) > 2000:
		return domain.Companion{}, domain.ErrInvalid
	case in.Kind != domain.KindCompanion && in.Kind != domain.KindHireling:
		return domain.Companion{}, domain.ErrInvalid
	}
	// Only what is not there is refused; what could not be read is a fault, and said to be one.
	if err := s.Creatures.Creature(ctx, id, slug); errors.Is(err, domain.ErrNotFound) {
		return domain.Companion{}, refuse("there is no such creature in this Campaign")
	} else if err != nil {
		return domain.Companion{}, err
	}
	if in.Controller != nil {
		if _, err := s.Repo.Member(ctx, id, *in.Controller); errors.Is(err, domain.ErrNotFound) {
			return domain.Companion{}, refuse("whoever runs a Companion is a Member of the Campaign")
		} else if err != nil {
			return domain.Companion{}, err
		}
	}
	return domain.Companion{
		ID: uuid.Nil, CampaignID: id, Name: name, Kind: in.Kind, MonsterSlug: slug, Controller: in.Controller, SharesXP: in.SharesXP, HP: nil,
		Notes: in.Notes, UpdatedAt: s.Now(),
	}, nil
}

// Create adds a Companion. DM only.
func (s *Companions) Create(ctx context.Context, c caller.Caller, id domain.CampaignID, in CompanionInput) (domain.Companion, error) {
	out, err := s.checked(ctx, c, id, in)
	if err != nil {
		return domain.Companion{}, err
	}
	out.ID = uuid.New()
	return out, s.Repo.InsertCompanion(ctx, out, out.UpdatedAt)
}

// Update changes a Companion. DM only.
func (s *Companions) Update(ctx context.Context, c caller.Caller, id domain.CampaignID, companion domain.CompanionID, in CompanionInput) (domain.Companion, error) {
	out, err := s.checked(ctx, c, id, in)
	if err != nil {
		return domain.Companion{}, err
	}
	out.ID = companion
	return out, s.Repo.UpdateCompanion(ctx, out, out.UpdatedAt)
}

// Delete lets a Companion go. DM only.
func (s *Companions) Delete(ctx context.Context, c caller.Caller, id domain.CampaignID, companion domain.CompanionID) error {
	me, err := s.Repo.Membership(ctx, id, c.Subject)
	if err != nil {
		return err
	}
	if me.Role != domain.RoleDM {
		return domain.ErrForbidden
	}
	return s.Repo.DeleteCompanion(ctx, id, companion)
}
