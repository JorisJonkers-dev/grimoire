package pgstore

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

// Tracks lists a Campaign's Tracks, oldest first, each with its thresholds in order.
//
//nolint:gosec // scores are bounded by the rules
func (s *Store) Tracks(ctx context.Context, id domain.CampaignID) ([]domain.Track, error) {
	rows, err := s.q.ListTracks(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	thresholds, err := s.q.ListTrackThresholds(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Track, 0, len(rows))
	for _, r := range rows {
		t := domain.Track{
			ID: r.ID, CampaignID: domain.CampaignID(r.CampaignID), Name: r.Name, Scope: r.Scope, Min: int(r.Lowest), Max: int(r.Highest), Start: int(r.Start),
			Thresholds: []domain.TrackThreshold{}, CreatedAt: r.CreatedAt,
		}
		for _, th := range thresholds {
			if th.TrackID != r.ID {
				continue
			}
			row := domain.TrackThreshold{At: int(th.At), Rising: th.Rising, Label: th.Label, Effect: th.Effect.String, RollTable: nil}
			if th.RollTable.Valid {
				table := uuid.UUID(th.RollTable.Bytes)
				row.RollTable = &table
			}
			t.Thresholds = append(t.Thresholds, row)
		}
		out = append(out, t)
	}
	return out, nil
}

// TrackValues reads where every Track of a Campaign stands that has moved from its start.
func (s *Store) TrackValues(ctx context.Context, id domain.CampaignID) ([]domain.TrackValue, error) {
	rows, err := s.q.ListTrackValues(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := make([]domain.TrackValue, 0, len(rows))
	for _, r := range rows {
		v := domain.TrackValue{Track: r.TrackID, Character: nil, Value: int(r.Value)}
		if r.CharacterID.Valid {
			ch := domain.CharacterID(r.CharacterID.Bytes)
			v.Character = &ch
		}
		out = append(out, v)
	}
	return out, nil
}

// TrackCharacters lists the Characters of a Campaign with who owns each, by name.
func (s *Store) TrackCharacters(ctx context.Context, id domain.CampaignID) ([]domain.TrackCharacter, error) {
	rows, err := s.q.TrackCharacters(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := make([]domain.TrackCharacter, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.TrackCharacter{ID: domain.CharacterID(r.ID), Name: r.Name, Owner: domain.MemberID(r.OwnerMemberID)})
	}
	return out, nil
}

// InsertTrack adds a Track with its thresholds, all or nothing.
//
//nolint:gosec // scores are bounded by the rules, and a Track has at most 20 thresholds
func (s *Store) InsertTrack(ctx context.Context, t domain.Track) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := queries.New(s.wrap(tx))
		if err := q.InsertTrack(ctx, queries.InsertTrackParams{
			ID: t.ID, CampaignID: uuid.UUID(t.CampaignID), Name: t.Name, Scope: t.Scope, Lowest: int32(t.Min), Highest: int32(t.Max), Start: int32(t.Start), Now: t.CreatedAt,
		}); err != nil {
			return err
		}
		for i, th := range t.Thresholds {
			p := queries.InsertTrackThresholdParams{TrackID: t.ID, Position: int32(i), At: int32(th.At), Rising: th.Rising, Label: th.Label}
			if th.Effect != "" {
				p.Effect = pgtype.Text{String: th.Effect, Valid: true}
			}
			if th.RollTable != nil {
				p.RollTable = pgtype.UUID{Bytes: *th.RollTable, Valid: true}
			}
			if err := q.InsertTrackThreshold(ctx, p); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteTrack removes a Track; ErrNotFound when the Campaign has no such one.
func (s *Store) DeleteTrack(ctx context.Context, id domain.CampaignID, track domain.TrackID) error {
	n, err := s.q.DeleteTrack(ctx, queries.DeleteTrackParams{CampaignID: uuid.UUID(id), ID: track})
	if err == nil && n == 0 {
		return domain.ErrNotFound
	}
	return err
}

// SetTrackValue keeps where a Track stands for a Character, or for the party.
//
//nolint:gosec // scores are bounded by the rules
func (s *Store) SetTrackValue(ctx context.Context, v domain.TrackValue) error {
	if v.Character == nil {
		return s.q.SetPartyTrackValue(ctx, queries.SetPartyTrackValueParams{TrackID: v.Track, Value: int32(v.Value)})
	}
	return s.q.SetCharacterTrackValue(ctx, queries.SetCharacterTrackValueParams{TrackID: v.Track, CharacterID: pgtype.UUID{Bytes: uuid.UUID(*v.Character), Valid: true}, Value: int32(v.Value)})
}
