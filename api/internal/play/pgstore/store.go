// Package pgstore is the Postgres adapter for the play context.
package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
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

// InsertRoll stores a Roll Request with its labels, modifiers and empty dice.
func (s *Store) InsertRoll(ctx context.Context, r domain.Roll, now time.Time) (domain.RollID, error) {
	id, err := s.q.InsertRoll(ctx, queries.InsertRollParams{
		ID: pgtype.UUID{Bytes: r.ID, Valid: r.ID != domain.RollID{}}, CampaignID: r.CampaignID, Purpose: r.Purpose, Notation: r.Notation, RequestedByName: r.RequestedBy,
		RollerMemberID: r.Roller.ID, RollerSubject: r.Roller.Subject, RollerName: r.Roller.Name, Now: now,
	})
	if err != nil {
		return domain.RollID{}, err
	}
	for g, label := range r.Labels {
		if err := s.q.InsertRollLabel(ctx, queries.InsertRollLabelParams{RollID: id, GroupNo: int32(g), Label: label}); err != nil { //nolint:gosec // bounded by the spec
			return domain.RollID{}, err
		}
	}
	for i, m := range r.Modifiers {
		if err := s.q.InsertRollModifier(ctx, queries.InsertRollModifierParams{RollID: id, Ordering: int32(i), Label: m.Label, Value: int32(m.Value)}); err != nil { //nolint:gosec // bounded by the API
			return domain.RollID{}, err
		}
	}
	for _, d := range r.Dice {
		if err := s.q.InsertRollDie(ctx, queries.InsertRollDieParams{RollID: id, DieNo: int32(d.No), GroupNo: int32(d.Group), Faces: int32(d.Faces)}); err != nil { //nolint:gosec // bounded by the spec
			return domain.RollID{}, err
		}
	}
	return domain.RollID(id), nil
}

// LockRoll locks a Roll Request for the rest of the transaction and returns its status.
func (s *Store) LockRoll(ctx context.Context, campaign uuid.UUID, id domain.RollID) (string, error) {
	status, err := s.q.LockRoll(ctx, queries.LockRollParams{CampaignID: campaign, ID: uuid.UUID(id)})
	return status, notFound(err)
}

// Roll reads a Roll Request with its dice.
func (s *Store) Roll(ctx context.Context, campaign uuid.UUID, id domain.RollID) (domain.Roll, error) {
	r, err := s.q.GetRoll(ctx, queries.GetRollParams{CampaignID: campaign, ID: uuid.UUID(id)})
	if err != nil {
		return domain.Roll{}, notFound(err)
	}
	out := domain.Roll{
		ID: domain.RollID(r.ID), CampaignID: r.CampaignID, Purpose: r.Purpose, Notation: r.Notation, Labels: map[int]string{},
		Modifiers: []domain.Modifier{}, RequestedBy: r.RequestedByName, Status: r.Status, Total: int(r.Total.Int32),
		Roller: domain.Member{ID: r.RollerMemberID, Subject: r.RollerSubject, Name: r.RollerName}, CreatedAt: r.CreatedAt,
		ResolvedAt: r.ResolvedAt.Time,
	}
	labels, err := s.q.RollLabels(ctx, r.ID)
	if err != nil {
		return domain.Roll{}, err
	}
	for _, l := range labels {
		out.Labels[int(l.GroupNo)] = l.Label
	}
	mods, err := s.q.RollModifiers(ctx, r.ID)
	if err != nil {
		return domain.Roll{}, err
	}
	for _, m := range mods {
		out.Modifiers = append(out.Modifiers, domain.Modifier{Label: m.Label, Value: int(m.Value)})
	}
	dice, err := s.q.RollDice(ctx, r.ID)
	if err != nil {
		return domain.Roll{}, err
	}
	for _, d := range dice {
		out.Dice = append(out.Dice, domain.Die{No: int(d.DieNo), Group: int(d.GroupNo), Faces: int(d.Faces), Value: int(d.Value.Int32), Mode: d.Mode.String})
	}
	return out, nil
}

// Rolls lists recent Roll Requests, newest first.
func (s *Store) Rolls(ctx context.Context, campaign uuid.UUID, limit int) ([]domain.Roll, error) {
	ids, err := s.q.ListRolls(ctx, queries.ListRollsParams{CampaignID: campaign, PageSize: int32(limit)}) //nolint:gosec // capped by the API
	if err != nil {
		return nil, err
	}
	out := make([]domain.Roll, 0, len(ids))
	for _, id := range ids {
		r, err := s.Roll(ctx, campaign, domain.RollID(id))
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// SetDie records a face for an empty die and reports whether it was empty.
func (s *Store) SetDie(ctx context.Context, id domain.RollID, no, value int, mode string) (bool, error) {
	n, err := s.q.SetRollDie(ctx, queries.SetRollDieParams{
		RollID: uuid.UUID(id), DieNo: int32(no), Value: pgtype.Int4{Int32: int32(value), Valid: true}, //nolint:gosec // die faces
		Mode: pgtype.Text{String: mode, Valid: true},
	})
	return n == 1, err
}

// ResolveRoll records the total.
func (s *Store) ResolveRoll(ctx context.Context, id domain.RollID, total int, now time.Time) error {
	return s.q.ResolveRoll(ctx, queries.ResolveRollParams{ID: uuid.UUID(id), Total: pgtype.Int4{Int32: int32(total), Valid: true}, Now: pgtype.Timestamptz{Time: now, Valid: true}}) //nolint:gosec // totals are small
}

// Append adds an Action to the Campaign's log with the next sequence number. Call it inside a transaction.
func (s *Store) Append(ctx context.Context, campaign uuid.UUID, e app.LogEntry) error {
	if err := s.q.LockCampaignLog(ctx, "log:"+campaign.String()); err != nil {
		return err
	}
	seq, err := s.q.NextActionSeq(ctx, campaign)
	if err != nil {
		return err
	}
	p := queries.InsertActionParams{
		CampaignID: campaign, Seq: int64(seq), Kind: e.Kind, ActorMemberID: e.Actor.ID, ActorName: e.Actor.Name,
		Origin: string(e.Caller.Origin), Client: e.Caller.Client, Now: e.At,
	}
	if e.Seed != nil {
		p.Seed = pgtype.Int8{Int64: int64(*e.Seed), Valid: true} //nolint:gosec // the seed's bits are stored as they are
	}
	actionID, err := s.q.InsertAction(ctx, p)
	if err != nil {
		return err
	}
	ev := queries.InsertRollEventParams{ActionID: actionID, RollID: uuid.UUID(e.RollID), Value: int32(e.Value)} //nolint:gosec // die faces and totals
	if e.DieNo != nil {
		ev.DieNo = pgtype.Int4{Int32: int32(*e.DieNo), Valid: true} //nolint:gosec // bounded by the spec
	}
	return s.q.InsertRollEvent(ctx, ev)
}

// ActionLog reads the Campaign's recent Actions, newest first.
func (s *Store) ActionLog(ctx context.Context, campaign uuid.UUID, limit int) ([]domain.Action, error) {
	rows, err := s.q.ActionLog(ctx, queries.ActionLogParams{CampaignID: campaign, PageSize: int32(limit)}) //nolint:gosec // capped by the API
	if err != nil {
		return nil, err
	}
	out := make([]domain.Action, 0, len(rows))
	for _, r := range rows {
		a := domain.Action{
			Seq: r.Seq, Kind: r.Kind, Actor: r.ActorName, Origin: r.Origin, Client: r.Client, Value: int(r.Value.Int32), CreatedAt: r.CreatedAt,
		}
		if r.Seed.Valid {
			seed := uint64(r.Seed.Int64) //nolint:gosec // the seed's bits are stored as they are
			a.Seed = &seed
		}
		if r.RollID.Valid {
			id := domain.RollID(r.RollID.Bytes)
			a.RollID = &id
		}
		if r.DieNo.Valid {
			no := int(r.DieNo.Int32)
			a.DieNo = &no
		}
		out = append(out, a)
	}
	return out, nil
}

// CampaignMembers answers membership questions from the campaign context.
type CampaignMembers struct {
	Store *campaignpg.Store
}

func member(m campaigndomain.Member) domain.Member {
	return domain.Member{ID: uuid.UUID(m.ID), Subject: m.Subject, Name: m.DisplayName, DM: m.Role == campaigndomain.RoleDM}
}

// Membership finds the caller's membership.
func (c CampaignMembers) Membership(ctx context.Context, campaign uuid.UUID, subject string) (domain.Member, error) {
	m, err := c.Store.Membership(ctx, campaigndomain.CampaignID(campaign), subject)
	return member(m), err
}

// Member finds a member by id.
func (c CampaignMembers) Member(ctx context.Context, campaign, id uuid.UUID) (domain.Member, error) {
	m, err := c.Store.Member(ctx, campaigndomain.CampaignID(campaign), campaigndomain.MemberID(id))
	return member(m), err
}

func pgtypeTime(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
