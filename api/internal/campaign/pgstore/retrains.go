package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// insertSnapshot stores a Character's rebuildable choices.
//
//nolint:gosec // scores and levels are bounded by the rules
func (s *Store) insertSnapshot(ctx context.Context, ch domain.CharacterID, sn domain.Snapshot, now time.Time) error {
	p := queries.InsertSnapshotParams{ID: sn.ID, CharacterID: uuid.UUID(ch), SpeciesSlug: sn.Species, BackgroundSlug: sn.Background, AbilityMethod: sn.Method, Now: now}
	if err := s.q.InsertSnapshot(ctx, p); err != nil {
		return err
	}
	for ability, base := range sn.Base {
		a := queries.InsertSnapshotAbilityParams{SnapshotID: sn.ID, Ability: ability, Base: int32(base), Bonus: int32(sn.Bonus[ability]), Increase: int32(sn.Increase[ability])}
		if err := s.q.InsertSnapshotAbility(ctx, a); err != nil {
			return err
		}
	}
	for _, skill := range sn.Skills {
		if err := s.q.InsertSnapshotSkill(ctx, queries.InsertSnapshotSkillParams{SnapshotID: sn.ID, Skill: skill}); err != nil {
			return err
		}
	}
	for _, pk := range sn.Picks {
		if err := s.q.InsertSnapshotPick(ctx, queries.InsertSnapshotPickParams{SnapshotID: sn.ID, Level: int32(pk.Level), Choice: pk.Choice, Value: pk.Value}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) snapshot(ctx context.Context, id uuid.UUID) (domain.Snapshot, error) {
	r, err := s.q.GetSnapshot(ctx, id)
	if err != nil {
		return domain.Snapshot{}, err
	}
	out := domain.Snapshot{
		ID: r.ID, Species: r.SpeciesSlug, Background: r.BackgroundSlug, Method: r.AbilityMethod, CreatedAt: r.CreatedAt,
		Base: map[string]int{}, Bonus: map[string]int{}, Increase: map[string]int{}, Skills: []string{}, Picks: []domain.Pick{},
	}
	abilities, err := s.q.SnapshotAbilities(ctx, id)
	if err != nil {
		return domain.Snapshot{}, err
	}
	for _, a := range abilities {
		out.Base[a.Ability] = int(a.Base)
		if a.Bonus > 0 {
			out.Bonus[a.Ability] = int(a.Bonus)
		}
		if a.Increase > 0 {
			out.Increase[a.Ability] = int(a.Increase)
		}
	}
	if out.Skills, err = s.q.SnapshotSkills(ctx, id); err != nil {
		return domain.Snapshot{}, err
	}
	picks, err := s.q.SnapshotPicks(ctx, id)
	for _, p := range picks {
		out.Picks = append(out.Picks, domain.Pick{Level: int(p.Level), Choice: p.Choice, Value: p.Value})
	}
	return out, err
}

// InsertRetrain stores a pending retrain with the build it proposes; ErrConflict when one is already
// pending for the Character.
func (s *Store) InsertRetrain(ctx context.Context, id domain.CampaignID, r domain.Retrain, now time.Time) error {
	if err := s.insertSnapshot(ctx, r.CharacterID, r.Proposed, now); err != nil {
		return err
	}
	err := s.q.InsertRetrain(ctx, queries.InsertRetrainParams{
		ID: r.ID, CampaignID: uuid.UUID(id), CharacterID: uuid.UUID(r.CharacterID), ProposedID: r.Proposed.ID,
		Reason: r.Reason, RequestedBy: r.RequestedBy, Now: now,
	})
	return conflict(err)
}

type retrainRow struct {
	id, characterID, proposedID uuid.UUID
	status, reason, by, decider string
	created                     time.Time
	decided                     pgtype.Timestamptz
}

func (s *Store) retrain(ctx context.Context, r retrainRow) (domain.Retrain, error) {
	out := domain.Retrain{
		ID: r.id, CharacterID: domain.CharacterID(r.characterID), Status: r.status, Reason: r.reason, RequestedBy: r.by,
		DecidedBy: r.decider, CreatedAt: r.created, DecidedAt: nil,
	}
	if r.decided.Valid {
		out.DecidedAt = &r.decided.Time
	}
	var err error
	out.Proposed, err = s.snapshot(ctx, r.proposedID)
	return out, err
}

// Retrains lists a Character's retrains, newest first.
func (s *Store) Retrains(ctx context.Context, id domain.CampaignID, ch domain.CharacterID) ([]domain.Retrain, error) {
	rows, err := s.q.ListRetrains(ctx, queries.ListRetrainsParams{CampaignID: uuid.UUID(id), CharacterID: uuid.UUID(ch)})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Retrain, 0, len(rows))
	for _, r := range rows {
		rt, err := s.retrain(ctx, retrainRow{r.ID, r.CharacterID, r.ProposedID, r.Status, r.Reason, r.RequestedBy, r.DecidedBy, r.CreatedAt, r.DecidedAt})
		if err != nil {
			return nil, err
		}
		out = append(out, rt)
	}
	return out, nil
}

// Retrain reads one retrain.
func (s *Store) Retrain(ctx context.Context, id domain.CampaignID, retrain uuid.UUID) (domain.Retrain, error) {
	r, err := s.q.GetRetrain(ctx, queries.GetRetrainParams{CampaignID: uuid.UUID(id), ID: retrain})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Retrain{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Retrain{}, err
	}
	return s.retrain(ctx, retrainRow{r.ID, r.CharacterID, r.ProposedID, r.Status, r.Reason, r.RequestedBy, r.DecidedBy, r.CreatedAt, r.DecidedAt})
}

// DecideRetrain approves or declines a pending retrain; ErrConflict when it was already decided.
func (s *Store) DecideRetrain(ctx context.Context, retrain uuid.UUID, status, by string, now time.Time) error {
	n, err := s.q.DecideRetrain(ctx, queries.DecideRetrainParams{ID: retrain, Status: status, DecidedBy: by, Now: pgtype.Timestamptz{Time: now, Valid: true}})
	if err != nil || n == 0 {
		return firstErr(err, domain.ErrConflict)
	}
	return nil
}

// ApplyRetrain keeps the Character's current build as a Revision and rebuilds it as next. Call it inside
// a transaction.
//
//nolint:gosec // scores, levels and hit points are bounded by the rules
func (s *Store) ApplyRetrain(ctx context.Context, id domain.CampaignID, previous domain.Snapshot, next domain.Character, retrain uuid.UUID, c caller.Caller, author string, now time.Time) error {
	entity := uuid.UUID(next.ID)
	if err := s.insertSnapshot(ctx, next.ID, previous, now); err != nil {
		return err
	}
	if err := s.q.LockEntity(ctx, string(domain.EntityCharacter)+":"+entity.String()); err != nil {
		return err
	}
	no, err := s.q.NextRevisionNo(ctx, queries.NextRevisionNoParams{EntityType: string(domain.EntityCharacter), EntityID: entity})
	if err != nil {
		return err
	}
	revID, err := s.q.InsertRevision(ctx, queries.InsertRevisionParams{
		CampaignID: uuid.UUID(id), EntityType: string(domain.EntityCharacter), EntityID: entity, RevisionNo: no, Action: string(domain.ActionUpdate),
		CallerSubject: c.Subject, CallerName: author, Origin: string(c.Origin), Client: c.Client, Now: now,
	})
	if err != nil {
		return err
	}
	if err := s.q.InsertCharacterRevision(ctx, queries.InsertCharacterRevisionParams{RevisionID: revID, SnapshotID: previous.ID, RetrainID: pgtype.UUID{Bytes: retrain, Valid: true}}); err != nil {
		return err
	}
	if err := s.q.RetrainCharacter(ctx, queries.RetrainCharacterParams{
		CampaignID: uuid.UUID(id), ID: entity, SpeciesSlug: next.Species, BackgroundSlug: next.Background, AbilityMethod: next.Method, HpMax: int32(next.HPMax), Now: now,
	}); err != nil {
		return err
	}
	return s.rebuild(ctx, entity, next)
}

//nolint:gosec // scores and levels are bounded by the rules
func (s *Store) rebuild(ctx context.Context, id uuid.UUID, next domain.Character) error {
	for _, clear := range []func(context.Context, uuid.UUID) error{s.q.ClearCharacterAbilities, s.q.ClearCharacterSkills, s.q.ClearCharacterPicks} {
		if err := clear(ctx, id); err != nil {
			return err
		}
	}
	for ability, base := range next.Base {
		if err := s.q.SetCharacterAbility(ctx, queries.SetCharacterAbilityParams{CharacterID: id, Ability: ability, Base: int32(base), Bonus: int32(next.Bonus[ability])}); err != nil {
			return err
		}
		if err := s.q.SetAbilityIncrease(ctx, queries.SetAbilityIncreaseParams{CharacterID: id, Ability: ability, Increase: int32(next.Increase[ability])}); err != nil {
			return err
		}
	}
	if err := s.rebuildSkills(ctx, id, next); err != nil {
		return err
	}
	for _, p := range next.Picks {
		if err := s.q.InsertCharacterPick(ctx, queries.InsertCharacterPickParams{CharacterID: id, Level: int32(p.Level), Choice: p.Choice, Value: p.Value}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) rebuildSkills(ctx context.Context, id uuid.UUID, next domain.Character) error {
	for source, skills := range map[string][]string{"class": next.Skills, "background": next.BackgroundSkills} {
		for _, skill := range skills {
			if err := s.q.AddCharacterSkill(ctx, queries.AddCharacterSkillParams{CharacterID: id, Skill: skill, Source: source}); err != nil {
				return err
			}
		}
	}
	return nil
}

// CharacterRevisions lists the builds a Character's retrains replaced, newest first.
func (s *Store) CharacterRevisions(ctx context.Context, id domain.CampaignID, ch domain.CharacterID) ([]domain.CharacterRevision, error) {
	rows, err := s.q.CharacterRevisions(ctx, queries.CharacterRevisionsParams{CampaignID: uuid.UUID(id), EntityID: uuid.UUID(ch)})
	if err != nil {
		return nil, err
	}
	out := make([]domain.CharacterRevision, 0, len(rows))
	for _, r := range rows {
		rev := domain.CharacterRevision{No: int(r.RevisionNo), Author: r.CallerName, CreatedAt: r.CreatedAt, RetrainID: nil}
		if r.RetrainID.Valid {
			retrain := uuid.UUID(r.RetrainID.Bytes)
			rev.RetrainID = &retrain
		}
		if rev.Build, err = s.snapshot(ctx, r.SnapshotID); err != nil {
			return nil, err
		}
		out = append(out, rev)
	}
	return out, nil
}
