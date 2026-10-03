package pgstore

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
)

// Companion reads what a Companion brings to the map.
func (s Statblocks) Companion(ctx context.Context, campaign, id uuid.UUID) (domain.CompanionRef, error) {
	r, err := s.Store.q.GetCompanion(ctx, queries.GetCompanionParams{CampaignID: campaign, ID: id})
	if err != nil {
		return domain.CompanionRef{}, notFound(err)
	}
	ref := domain.CompanionRef{Name: r.Name, Slug: r.MonsterSlug, Controller: nil, HP: nil}
	if r.ControllerMemberID.Valid {
		c := uuid.UUID(r.ControllerMemberID.Bytes)
		ref.Controller = &c
	}
	if r.HpCurrent.Valid {
		hp := int(r.HpCurrent.Int32)
		ref.HP = &hp
	}
	return ref, nil
}

// Creature reports whether the Campaign has a creature of that name to make a Companion of.
func (s Statblocks) Creature(ctx context.Context, campaign campaigndomain.CampaignID, slug string) error {
	_, _, err := s.Monster(ctx, uuid.UUID(campaign), slug)
	return err
}

// assignControl hands a Companion's token, and the Companion with it, to whoever runs it now.
func (s *Store) assignControl(ctx context.Context, sid uuid.UUID, t domain.Token) error {
	who := pgtype.UUID{}
	if t.Controller != nil {
		who = pgtype.UUID{Bytes: *t.Controller, Valid: true}
	}
	if err := s.q.SetTokenController(ctx, queries.SetTokenControllerParams{ControllerMemberID: who, SessionID: sid, ID: uuid.UUID(t.ID)}); err != nil {
		return err
	}
	if t.Companion == nil {
		return nil
	}
	return s.q.SetCompanionController(ctx, queries.SetCompanionControllerParams{ControllerMemberID: who, ID: *t.Companion, SessionID: sid})
}

// awardXP gives each Character its XP and records the award with the Action.
func (s *Store) awardXP(ctx context.Context, actionID uuid.UUID, campaign uuid.UUID, awards []domain.XPAward) error {
	for _, a := range awards {
		//nolint:gosec // XP for one fight is small
		if err := s.q.AwardXP(ctx, queries.AwardXPParams{Amount: int32(a.Amount), CampaignID: campaign, ID: a.Character}); err != nil {
			return err
		}
		//nolint:gosec // XP for one fight is small
		if err := s.q.InsertXPAward(ctx, queries.InsertXPAwardParams{ActionID: actionID, Amount: int32(a.Amount), CharacterID: a.Character, CampaignID: campaign}); err != nil {
			return err
		}
	}
	return nil
}

// CreatureXP is the XP a creature of the Campaign is worth, 0 for one the Compendium does not know.
func (s *Store) CreatureXP(ctx context.Context, campaign uuid.UUID, slug string) (int, error) {
	r, err := s.q.MonsterXP(ctx, queries.MonsterXPParams{Slug: slug, CampaignID: campaign})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return int(r.Xp), err
}

// SharingCompanions counts the Companions among these that take a share of the XP.
func (s *Store) SharingCompanions(ctx context.Context, campaign uuid.UUID, ids []uuid.UUID) (int, error) {
	n, err := s.q.SharingCompanions(ctx, queries.SharingCompanionsParams{CampaignID: campaign, Ids: ids})
	return int(n), err
}
