// Package pgstore is the Postgres adapter for random-encounter prep.
package pgstore

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/prep/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Store implements app.Repository.
type Store struct {
	pool *pgxpool.Pool
	q    *queries.Queries
	wrap func(queries.DBTX) queries.DBTX
}

var _ app.Repository = (*Store)(nil)

// New wraps a pool.
func New(pool *pgxpool.Pool) *Store {
	return newWrapped(pool, func(db queries.DBTX) queries.DBTX { return db })
}

func newWrapped(pool *pgxpool.Pool, wrap func(queries.DBTX) queries.DBTX) *Store {
	return &Store{pool: pool, q: queries.New(wrap(pool)), wrap: wrap}
}

// InTx runs fn against a Store bound to one transaction.
func (s *Store) InTx(ctx context.Context, fn func(app.Repository) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(&Store{pool: s.pool, q: queries.New(s.wrap(tx)), wrap: s.wrap})
	})
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.ErrNotFound
	}
	return err
}

func optUUID(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

func fromUUID(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	u := uuid.UUID(id.Bytes)
	return &u
}

// MonsterXP is a compendium monster's XP, preferring the Campaign's ruleset.
func (s *Store) MonsterXP(ctx context.Context, campaign uuid.UUID, slug string) (int, error) {
	r, err := s.q.MonsterXP(ctx, queries.MonsterXPParams{Slug: slug, CampaignID: campaign})
	return int(r.Xp), notFound(err)
}

// Location checks that a place lies on one of the Campaign's maps.
func (s *Store) Location(ctx context.Context, campaign, id uuid.UUID) error {
	_, err := s.q.CampaignNode(ctx, queries.CampaignNodeParams{CampaignID: campaign, ID: id})
	return notFound(err)
}

// Locations lists the places on the Campaign's world maps.
func (s *Store) Locations(ctx context.Context, campaign uuid.UUID) ([]domain.Location, error) {
	rows, err := s.q.CampaignLocations(ctx, campaign)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Location, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Location{ID: r.ID, Name: r.Name, MapName: r.MapName})
	}
	return out, nil
}

// record numbers and stores a Revision, returning its id for the snapshot. Call it inside a transaction.
func (s *Store) record(ctx context.Context, campaign uuid.UUID, entity string, id uuid.UUID, r campaigndomain.Revision, c caller.Caller) (uuid.UUID, error) {
	if err := s.q.LockEntity(ctx, entity+":"+id.String()); err != nil {
		return uuid.UUID{}, err
	}
	no, err := s.q.NextRevisionNo(ctx, queries.NextRevisionNoParams{EntityType: entity, EntityID: id})
	if err != nil {
		return uuid.UUID{}, err
	}
	p := queries.InsertRevisionParams{
		CampaignID: campaign, EntityType: entity, EntityID: id, RevisionNo: no, Action: string(r.Action),
		CallerSubject: c.Subject, CallerName: r.Author, Origin: string(c.Origin), Client: c.Client, Now: r.CreatedAt,
	}
	if r.RestoredFrom > 0 {
		p.RestoredFrom = pgtype.Int4{Int32: int32(r.RestoredFrom), Valid: true} //nolint:gosec // revision numbers are small
	}
	return s.q.InsertRevision(ctx, p)
}

// Revisions lists an entity's Revisions, newest first.
func (s *Store) Revisions(ctx context.Context, campaign uuid.UUID, entity string, id uuid.UUID) ([]campaigndomain.Revision, error) {
	rows, err := s.q.ListRevisions(ctx, queries.ListRevisionsParams{CampaignID: campaign, EntityType: entity, EntityID: id})
	if err != nil {
		return nil, err
	}
	out := make([]campaigndomain.Revision, 0, len(rows))
	for _, r := range rows {
		out = append(out, campaigndomain.Revision{
			No: int(r.RevisionNo), Action: campaigndomain.RevisionAction(r.Action), Author: r.CallerName, Origin: r.Origin, Client: r.Client,
			RestoredFrom: int(r.RestoredFrom.Int32), CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}
