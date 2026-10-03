// Package pgstore is the Postgres adapter for the campaign context.
package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
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
		return domain.ErrNotFound
	}
	return err
}

// settings are a Campaign's optional rules as stored.
type settings struct {
	highGround, restSupplies, shareInitiative bool
	initiativeMode                            string
	creationMethods                           []string
	startingLevel                             int32
	holdLevelUps                              bool
	exhaustionVariant                         string
}

func campaign(id uuid.UUID, name, ruleset string, timeout int32, o settings, created time.Time) domain.Campaign {
	return domain.Campaign{
		ID: domain.CampaignID(id), Name: name, Ruleset: ruleset, ReactionTimeoutS: int(timeout), HighGround: o.highGround, RestSupplies: o.restSupplies,
		InitiativeMode: o.initiativeMode, ShareInitiative: o.shareInitiative, CreationMethods: o.creationMethods, StartingLevel: int(o.startingLevel), HoldLevelUps: o.holdLevelUps, ExhaustionVariant: o.exhaustionVariant, CreatedAt: created,
	}
}

func member(m queries.CampaignMember) domain.Member {
	return domain.Member{
		ID: domain.MemberID(m.ID), CampaignID: domain.CampaignID(m.CampaignID), Subject: m.AuthSubject,
		DisplayName: m.DisplayName, Role: domain.Role(m.Role), JoinedAt: m.JoinedAt,
	}
}

func optText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// CreateCampaign inserts a Campaign.
func (s *Store) CreateCampaign(ctx context.Context, name, ruleset, subject string, now time.Time) (domain.Campaign, error) {
	if ruleset == "" {
		ruleset = "srd-2024"
	}
	r, err := s.q.CreateCampaign(ctx, queries.CreateCampaignParams{Name: name, RulesetPref: ruleset, CreatedBy: subject, Now: now})
	if err != nil {
		return domain.Campaign{}, err
	}
	return campaign(r.ID, r.Name, r.RulesetPref, r.ReactionTimeoutS, settings{highGround: r.HighGround, restSupplies: r.RestSupplies, shareInitiative: r.ShareInitiative, initiativeMode: r.InitiativeMode, creationMethods: r.CreationMethods, startingLevel: r.StartingLevel, holdLevelUps: r.HoldLevelUps, exhaustionVariant: r.ExhaustionVariant}, r.CreatedAt), nil
}

// UpdateCampaign changes the given fields.
func (s *Store) UpdateCampaign(ctx context.Context, id domain.CampaignID, change domain.SettingsChange, now time.Time) (domain.Campaign, error) {
	p := queries.UpdateCampaignParams{ID: uuid.UUID(id), Name: optText(change.Name), RulesetPref: optText(change.Ruleset), Now: now}
	if v := change.HighGround; v != nil {
		p.HighGround = pgtype.Bool{Bool: *v, Valid: true}
	}
	if v := change.RestSupplies; v != nil {
		p.RestSupplies = pgtype.Bool{Bool: *v, Valid: true}
	}
	if v := change.ShareInitiative; v != nil {
		p.ShareInitiative = pgtype.Bool{Bool: *v, Valid: true}
	}
	p.InitiativeMode = optText(change.InitiativeMode)
	p.CreationMethods = change.CreationMethods
	if v := change.StartingLevel; v != nil {
		p.StartingLevel = pgtype.Int4{Int32: int32(*v), Valid: true} //nolint:gosec // 1 to 20
	}
	if v := change.HoldLevelUps; v != nil {
		p.HoldLevelUps = pgtype.Bool{Bool: *v, Valid: true}
	}
	p.ExhaustionVariant = optText(change.ExhaustionVariant)
	if v := change.ReactionTimeoutS; v != nil {
		p.ReactionTimeoutS = pgtype.Int4{Int32: int32(*v), Valid: true} //nolint:gosec // 3 to 120 seconds
	}
	r, err := s.q.UpdateCampaign(ctx, p)
	if err != nil {
		return domain.Campaign{}, notFound(err)
	}
	return campaign(r.ID, r.Name, r.RulesetPref, r.ReactionTimeoutS, settings{highGround: r.HighGround, restSupplies: r.RestSupplies, shareInitiative: r.ShareInitiative, initiativeMode: r.InitiativeMode, creationMethods: r.CreationMethods, startingLevel: r.StartingLevel, holdLevelUps: r.HoldLevelUps, exhaustionVariant: r.ExhaustionVariant}, r.CreatedAt), nil
}

// GetCampaign reads one Campaign.
func (s *Store) GetCampaign(ctx context.Context, id domain.CampaignID) (domain.Campaign, error) {
	r, err := s.q.GetCampaign(ctx, uuid.UUID(id))
	if err != nil {
		return domain.Campaign{}, notFound(err)
	}
	return campaign(r.ID, r.Name, r.RulesetPref, r.ReactionTimeoutS, settings{highGround: r.HighGround, restSupplies: r.RestSupplies, shareInitiative: r.ShareInitiative, initiativeMode: r.InitiativeMode, creationMethods: r.CreationMethods, startingLevel: r.StartingLevel, holdLevelUps: r.HoldLevelUps, exhaustionVariant: r.ExhaustionVariant}, r.CreatedAt), nil
}

// ListCampaigns returns a subject's Campaigns, newest first.
func (s *Store) ListCampaigns(ctx context.Context, subject string, after *domain.ListCursor, pageSize int) ([]domain.Summary, error) {
	p := queries.ListCampaignsForSubjectParams{Subject: subject, PageSize: int32(pageSize)} //nolint:gosec // capped by the API
	if after != nil {
		p.AfterCreated = pgtype.Timestamptz{Time: after.CreatedAt, Valid: true}
		p.AfterID = pgtype.UUID{Bytes: after.ID, Valid: true}
	}
	rows, err := s.q.ListCampaignsForSubject(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Summary, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Summary{
			Campaign: campaign(r.ID, r.Name, r.RulesetPref, r.ReactionTimeoutS, settings{highGround: r.HighGround, restSupplies: r.RestSupplies, shareInitiative: r.ShareInitiative, initiativeMode: r.InitiativeMode, creationMethods: r.CreationMethods, startingLevel: r.StartingLevel, holdLevelUps: r.HoldLevelUps, exhaustionVariant: r.ExhaustionVariant}, r.CreatedAt), MyRole: domain.Role(r.Role), MemberCount: int(r.MemberCount),
		})
	}
	return out, nil
}

// LockCampaign serialises membership changes within one Campaign.
func (s *Store) LockCampaign(ctx context.Context, id domain.CampaignID) error {
	return notFound(s.q.LockCampaign(ctx, uuid.UUID(id)))
}

// AddMember inserts a Member, or returns the existing one unchanged.
func (s *Store) AddMember(ctx context.Context, m domain.Member) (domain.Member, error) {
	r, err := s.q.AddMember(ctx, queries.AddMemberParams{
		CampaignID: uuid.UUID(m.CampaignID), AuthSubject: m.Subject, DisplayName: m.DisplayName, Role: string(m.Role), Now: m.JoinedAt,
	})
	if err != nil {
		return domain.Member{}, err
	}
	return member(queries.CampaignMember(r)), nil
}

// Membership finds a subject's Member row in a Campaign.
func (s *Store) Membership(ctx context.Context, id domain.CampaignID, subject string) (domain.Member, error) {
	r, err := s.q.GetMembership(ctx, queries.GetMembershipParams{CampaignID: uuid.UUID(id), AuthSubject: subject})
	if err != nil {
		return domain.Member{}, notFound(err)
	}
	return member(queries.CampaignMember(r)), nil
}

// Member reads one Member of a Campaign.
func (s *Store) Member(ctx context.Context, id domain.CampaignID, m domain.MemberID) (domain.Member, error) {
	r, err := s.q.GetMember(ctx, queries.GetMemberParams{CampaignID: uuid.UUID(id), ID: uuid.UUID(m)})
	if err != nil {
		return domain.Member{}, notFound(err)
	}
	return member(queries.CampaignMember(r)), nil
}

// Members lists a Campaign's Members, DMs first.
func (s *Store) Members(ctx context.Context, id domain.CampaignID) ([]domain.Member, error) {
	rows, err := s.q.ListMembers(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Member, 0, len(rows))
	for _, r := range rows {
		out = append(out, member(queries.CampaignMember(r)))
	}
	return out, nil
}

// SetRole changes a Member's role.
func (s *Store) SetRole(ctx context.Context, id domain.CampaignID, m domain.MemberID, role domain.Role) error {
	return s.q.SetMemberRole(ctx, queries.SetMemberRoleParams{CampaignID: uuid.UUID(id), ID: uuid.UUID(m), Role: string(role)})
}

// RemoveMember deletes a Member.
func (s *Store) RemoveMember(ctx context.Context, id domain.CampaignID, m domain.MemberID) error {
	return s.q.RemoveMember(ctx, queries.RemoveMemberParams{CampaignID: uuid.UUID(id), ID: uuid.UUID(m)})
}

// CountDMs counts a Campaign's DMs.
func (s *Store) CountDMs(ctx context.Context, id domain.CampaignID) (int, error) {
	n, err := s.q.CountDMs(ctx, uuid.UUID(id))
	return int(n), err
}

// CreateInvite stores an invite by its token hash.
func (s *Store) CreateInvite(ctx context.Context, id domain.CampaignID, hash []byte, by domain.MemberID, now, expires time.Time) (domain.Invite, error) {
	r, err := s.q.CreateInvite(ctx, queries.CreateInviteParams{CampaignID: uuid.UUID(id), TokenHash: hash, CreatedBy: uuid.UUID(by), Now: now, ExpiresAt: expires})
	if err != nil {
		return domain.Invite{}, err
	}
	return domain.Invite{ID: domain.InviteID(r.ID), CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt}, nil
}

// Invites lists open invites.
func (s *Store) Invites(ctx context.Context, id domain.CampaignID, now time.Time) ([]domain.Invite, error) {
	rows, err := s.q.ListInvites(ctx, queries.ListInvitesParams{CampaignID: uuid.UUID(id), Now: now})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Invite, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Invite{ID: domain.InviteID(r.ID), CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, CreatedByName: r.CreatedByName})
	}
	return out, nil
}

// RevokeInvite closes an open invite and reports whether one was closed.
func (s *Store) RevokeInvite(ctx context.Context, id domain.CampaignID, invite domain.InviteID, now time.Time) (bool, error) {
	n, err := s.q.RevokeInvite(ctx, queries.RevokeInviteParams{CampaignID: uuid.UUID(id), ID: uuid.UUID(invite), Now: now})
	return n == 1, err
}

// FindInvite resolves an open invite by its token hash.
func (s *Store) FindInvite(ctx context.Context, hash []byte, now time.Time) (domain.InvitePreview, error) {
	r, err := s.q.FindInvite(ctx, queries.FindInviteParams{TokenHash: hash, Now: now})
	if err != nil {
		return domain.InvitePreview{}, notFound(err)
	}
	return domain.InvitePreview{CampaignID: domain.CampaignID(r.CampaignID), CampaignName: r.CampaignName, InvitedBy: r.CreatedByName}, nil
}
