package pgstore

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

// conflict reports a unique constraint as ErrConflict.
func conflict(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return domain.ErrConflict
	}
	return err
}

// OwnedCharacters lists the Characters an Account owns, with their Campaign Characters.
func (s *Store) OwnedCharacters(ctx context.Context, subject string) ([]domain.OwnedCharacter, error) {
	rows, err := s.q.ListAccountCharacters(ctx, subject)
	if err != nil {
		return nil, err
	}
	out := make([]domain.OwnedCharacter, 0, len(rows))
	ids := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.OwnedCharacter{
			ID: domain.OwnedID(r.ID), OwnerSubject: subject, Name: r.Name, Ruleset: r.Ruleset, Species: r.SpeciesSlug, Class: r.ClassSlug,
			Background: r.BackgroundSlug, Backstory: r.Backstory, HasPortrait: r.PortraitKey.Valid, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			Campaigns: []domain.CampaignEntry{},
		})
		ids = append(ids, r.ID)
	}
	return out, s.campaignsOf(ctx, out, ids)
}

// OwnedCharacter reads one Character with its Campaign Characters.
func (s *Store) OwnedCharacter(ctx context.Context, id domain.OwnedID) (domain.OwnedCharacter, error) {
	r, err := s.q.GetAccountCharacter(ctx, uuid.UUID(id))
	if err != nil {
		return domain.OwnedCharacter{}, notFound(err)
	}
	out := []domain.OwnedCharacter{{
		ID: id, OwnerSubject: r.OwnerSubject, Name: r.Name, Ruleset: r.Ruleset, Species: r.SpeciesSlug, Class: r.ClassSlug,
		Background: r.BackgroundSlug, Backstory: r.Backstory, HasPortrait: r.PortraitKey.Valid, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		Campaigns: []domain.CampaignEntry{},
	}}
	return out[0], s.campaignsOf(ctx, out, []uuid.UUID{r.ID})
}

// campaignsOf fills in each Character's Campaign Characters, most recently played first.
func (s *Store) campaignsOf(ctx context.Context, owned []domain.OwnedCharacter, ids []uuid.UUID) error {
	rows, err := s.q.AccountCharacterCampaigns(ctx, ids)
	if err != nil {
		return err
	}
	for _, r := range rows {
		for i := range owned {
			if uuid.UUID(owned[i].ID) == uuid.UUID(r.CharacterID.Bytes) {
				owned[i].Campaigns = append(owned[i].Campaigns, domain.CampaignEntry{
					CampaignID: domain.CampaignID(r.CampaignID), CampaignName: r.CampaignName, CharacterID: domain.CharacterID(r.ID),
					Level: int(r.Level), HPCurrent: int(r.HpCurrent), HPMax: int(r.HpMax),
				})
			}
		}
	}
	return nil
}

// UpdateOwnedCharacter sets a Character's name and Backstory; the name flows to every Campaign.
func (s *Store) UpdateOwnedCharacter(ctx context.Context, id domain.OwnedID, name, backstory string, now time.Time) error {
	return s.InTx(ctx, func(r app.Repository) error {
		q := r.(*Store).q
		if err := q.UpdateAccountCharacter(ctx, queries.UpdateAccountCharacterParams{Name: name, Backstory: backstory, Now: now, ID: uuid.UUID(id)}); err != nil {
			return err
		}
		return q.RenameCampaignCharacters(ctx, queries.RenameCampaignCharactersParams{Name: name, CharacterID: pgtype.UUID{Bytes: id, Valid: true}})
	})
}

// ActionBars reads a Character's action bar layout; one nobody arranged is empty.
func (s *Store) ActionBars(ctx context.Context, id domain.OwnedID) (domain.ActionBars, error) {
	raw, err := s.q.AccountCharacterActionBars(ctx, uuid.UUID(id))
	if err != nil {
		return domain.ActionBars{}, notFound(err)
	}
	out := domain.ActionBars{Bars: [][]string{}, Quick: []string{}, Stowed: []string{}, Arranged: raw != nil}
	if raw != nil {
		_ = json.Unmarshal(raw, &out) // written by SetActionBars
	}
	return out, nil
}

// SetActionBars saves a Character's action bar layout.
func (s *Store) SetActionBars(ctx context.Context, id domain.OwnedID, bars domain.ActionBars, now time.Time) error {
	raw, _ := json.Marshal(bars) //nolint:errchkjson // a layout is plain strings
	return s.q.SetAccountCharacterActionBars(ctx, queries.SetAccountCharacterActionBarsParams{ActionBars: raw, Now: now, ID: uuid.UUID(id)})
}
