package pgstore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

func faction(r queries.CampaignFaction) domain.Faction {
	return domain.Faction{
		ID: r.ID, CampaignID: domain.CampaignID(r.CampaignID), Name: r.Name, Archetype: r.Archetype, Goals: r.Goals, Territory: r.Territory,
		Notes: r.Notes, Score: int(r.Score), UpdatedAt: r.UpdatedAt,
	}
}

func standingChange(r queries.CampaignStandingChange) domain.StandingChange {
	c := domain.StandingChange{
		ID: r.ID, FactionID: r.FactionID, Character: nil, Delta: int(r.Delta), Reason: r.Reason, ShareReason: r.ShareReason, Status: r.Status,
		Origin: r.Origin, Client: r.Client, ProposedBy: r.ProposedBy, CreatedAt: r.CreatedAt, DecidedAt: nil,
	}
	if r.CharacterID.Valid {
		id := domain.CharacterID(r.CharacterID.Bytes)
		c.Character = &id
	}
	if r.DecidedAt.Valid {
		at := r.DecidedAt.Time
		c.DecidedAt = &at
	}
	return c
}

// Factions lists a Campaign's Factions by name.
func (s *Store) Factions(ctx context.Context, id domain.CampaignID) ([]domain.Faction, error) {
	rows, err := s.q.ListFactions(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Faction, 0, len(rows))
	for _, r := range rows {
		out = append(out, faction(r))
	}
	return out, nil
}

// Faction reads one Faction of a Campaign.
func (s *Store) Faction(ctx context.Context, id domain.CampaignID, f domain.FactionID) (domain.Faction, error) {
	r, err := s.q.GetFaction(ctx, queries.GetFactionParams{CampaignID: uuid.UUID(id), ID: f})
	if err != nil {
		return domain.Faction{}, notFound(err)
	}
	return faction(r), nil
}

// InsertFaction adds a Faction, Neutral towards the party.
func (s *Store) InsertFaction(ctx context.Context, f domain.Faction, now time.Time) error {
	return s.q.InsertFaction(ctx, queries.InsertFactionParams{
		ID: f.ID, CampaignID: uuid.UUID(f.CampaignID), Name: f.Name, Archetype: f.Archetype, Goals: f.Goals, Territory: f.Territory, Notes: f.Notes, Now: now,
	})
}

// UpdateFaction changes a Faction of its Campaign.
func (s *Store) UpdateFaction(ctx context.Context, f domain.Faction, now time.Time) error {
	n, err := s.q.UpdateFaction(ctx, queries.UpdateFactionParams{
		CampaignID: uuid.UUID(f.CampaignID), ID: f.ID, Name: f.Name, Archetype: f.Archetype, Goals: f.Goals, Territory: f.Territory, Notes: f.Notes, Now: now,
	})
	if err == nil && n == 0 {
		return domain.ErrNotFound
	}
	return err
}

// DeleteFaction removes a Faction of its Campaign.
func (s *Store) DeleteFaction(ctx context.Context, id domain.CampaignID, f domain.FactionID) error {
	n, err := s.q.DeleteFaction(ctx, queries.DeleteFactionParams{CampaignID: uuid.UUID(id), ID: f})
	if err == nil && n == 0 {
		return domain.ErrNotFound
	}
	return err
}

// PersonalStandings lists every Personal Standing with a Faction of the Campaign.
func (s *Store) PersonalStandings(ctx context.Context, id domain.CampaignID) ([]domain.PersonalStanding, error) {
	rows, err := s.q.ListPersonalStandings(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := make([]domain.PersonalStanding, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.PersonalStanding{
			FactionID: r.FactionID, CharacterID: domain.CharacterID(r.CharacterID), Character: r.Name, Owner: domain.MemberID(r.OwnerMemberID), Score: int(r.Score),
		})
	}
	return out, nil
}

// CharacterOwner is the Member a Character of the Campaign belongs to.
func (s *Store) CharacterOwner(ctx context.Context, id domain.CampaignID, character domain.CharacterID) (domain.MemberID, error) {
	owner, err := s.q.CampaignCharacterOwner(ctx, queries.CampaignCharacterOwnerParams{CampaignID: uuid.UUID(id), ID: uuid.UUID(character)})
	return domain.MemberID(owner), notFound(err)
}

// StandingChanges lists the Standing Changes of a Campaign's Factions, newest first.
func (s *Store) StandingChanges(ctx context.Context, id domain.CampaignID) ([]domain.StandingChange, error) {
	rows, err := s.q.ListStandingChanges(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := make([]domain.StandingChange, 0, len(rows))
	for _, r := range rows {
		out = append(out, standingChange(r))
	}
	return out, nil
}

// InsertStandingChange keeps a suggested Standing Change, pending.
func (s *Store) InsertStandingChange(ctx context.Context, c domain.StandingChange) error {
	p := queries.InsertStandingChangeParams{
		ID: c.ID, FactionID: c.FactionID, CharacterID: pgtype.UUID{}, Delta: int32(c.Delta), Reason: c.Reason, ShareReason: c.ShareReason, //nolint:gosec // -100 to 100
		Origin: c.Origin, Client: c.Client, ProposedBy: c.ProposedBy, Now: c.CreatedAt,
	}
	if c.Character != nil {
		p.CharacterID = pgtype.UUID{Bytes: *c.Character, Valid: true}
	}
	return s.q.InsertStandingChange(ctx, p)
}

// standingNow is the score a Standing Change would move: the party's, or the Character's Personal
// Standing, which starts from the party's.
func standingNow(ctx context.Context, q *queries.Queries, id domain.CampaignID, ch domain.StandingChange) (int, error) {
	f, err := q.GetFaction(ctx, queries.GetFactionParams{CampaignID: uuid.UUID(id), ID: ch.FactionID})
	if err != nil || ch.Character == nil {
		return int(f.Score), err
	}
	own, err := q.PersonalStanding(ctx, queries.PersonalStandingParams{FactionID: ch.FactionID, CharacterID: uuid.UUID(*ch.Character)})
	score := int(f.Score)
	for _, v := range own {
		score = int(v)
	}
	return score, err
}

// DecideStandingChange holds a pending change and its Faction while the DM's decision is worked out,
// then keeps the change as decided and the score it leaves, all or nothing.
//
//nolint:gosec // scores and deltas run from -100 to 100
func (s *Store) DecideStandingChange(ctx context.Context, id domain.CampaignID, change domain.ChangeID, now time.Time, decide func(domain.StandingChange, int) (domain.StandingChange, *int)) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := queries.New(s.wrap(tx))
		row, err := q.LockStandingChange(ctx, queries.LockStandingChangeParams{CampaignID: uuid.UUID(id), ID: change})
		if err != nil {
			return notFound(err)
		}
		ch := standingChange(row)
		if ch.Status != domain.ChangePending {
			return domain.ErrConflict
		}
		score, err := standingNow(ctx, q, id, ch)
		if err != nil {
			return err
		}
		decided, after := decide(ch, score)
		if err := q.DecideStandingChange(ctx, queries.DecideStandingChangeParams{
			ID: change, Status: decided.Status, Delta: int32(decided.Delta), Reason: decided.Reason, ShareReason: decided.ShareReason, Now: pgtype.Timestamptz{Time: now, Valid: true},
		}); err != nil || after == nil {
			return err
		}
		if ch.Character != nil {
			return q.SetPersonalStanding(ctx, queries.SetPersonalStandingParams{CampaignID: uuid.UUID(id), FactionID: ch.FactionID, CharacterID: uuid.UUID(*ch.Character), Score: int32(*after)})
		}
		return q.SetFactionScore(ctx, queries.SetFactionScoreParams{CampaignID: uuid.UUID(id), ID: ch.FactionID, Score: int32(*after), Now: now})
	})
}
