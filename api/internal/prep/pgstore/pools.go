package pgstore

import (
	"context"
	"time"

	"github.com/google/uuid"

	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Pools lists the Campaign's Pools with their members.
func (s *Store) Pools(ctx context.Context, campaign uuid.UUID) ([]domain.Pool, error) {
	return LoadPools(ctx, s.q, campaign)
}

// LoadPools reads the Campaign's Pools with their members.
func LoadPools(ctx context.Context, q *queries.Queries, campaign uuid.UUID) ([]domain.Pool, error) {
	rows, err := q.ListPools(ctx, campaign)
	if err != nil {
		return nil, err
	}
	members, err := q.CampaignPoolMembers(ctx, campaign)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Pool, 0, len(rows))
	for _, r := range rows {
		p := domain.Pool{ID: domain.PoolID(r.ID), Name: r.Name, LevelMin: int(r.LevelMin), LevelMax: int(r.LevelMax), Difficulty: r.Difficulty, Members: []domain.PoolMember{}, UpdatedAt: r.UpdatedAt}
		for _, m := range members {
			if m.PoolID == r.ID {
				p.Members = append(p.Members, domain.PoolMember{Slug: m.MonsterSlug, Weight: int(m.Weight), Min: int(m.MinCount), Max: int(m.MaxCount)})
			}
		}
		out = append(out, p)
	}
	return out, nil
}

// SavePool writes a Pool and replaces its members.
//
//nolint:gosec // levels, weights and counts are bounded by the rules
func (s *Store) SavePool(ctx context.Context, campaign uuid.UUID, p domain.Pool, now time.Time) error {
	id := uuid.UUID(p.ID)
	if err := s.q.SavePool(ctx, queries.SavePoolParams{
		ID: id, CampaignID: campaign, Name: p.Name, LevelMin: int32(p.LevelMin), LevelMax: int32(p.LevelMax), Difficulty: p.Difficulty, Now: now,
	}); err != nil {
		return err
	}
	if err := s.q.ClearPoolMembers(ctx, id); err != nil {
		return err
	}
	for i, m := range p.Members {
		if err := s.q.InsertPoolMember(ctx, queries.InsertPoolMemberParams{
			PoolID: id, Ordering: int32(i), MonsterSlug: m.Slug, Weight: int32(m.Weight), MinCount: int32(m.Min), MaxCount: int32(m.Max),
		}); err != nil {
			return err
		}
	}
	return nil
}

// DeletePool removes a Pool.
func (s *Store) DeletePool(ctx context.Context, campaign uuid.UUID, id domain.PoolID) (bool, error) {
	n, err := s.q.DeletePool(ctx, queries.DeletePoolParams{CampaignID: campaign, ID: uuid.UUID(id)})
	return n > 0, err
}

// PoolInUse reports whether any Table draws from the Pool.
func (s *Store) PoolInUse(ctx context.Context, id domain.PoolID) (bool, error) {
	pid := uuid.UUID(id)
	n, err := s.q.PoolInUse(ctx, optUUID(&pid))
	return n > 0, err
}

// RecordPool stores a Revision with the Pool's snapshot.
//
//nolint:gosec // levels, weights and counts are bounded by the rules
func (s *Store) RecordPool(ctx context.Context, campaign uuid.UUID, rev campaigndomain.Revision, c caller.Caller, p domain.Pool) error {
	revID, err := s.record(ctx, campaign, domain.EntityPool, uuid.UUID(p.ID), rev, c)
	if err != nil {
		return err
	}
	if err := s.q.InsertPoolRevision(ctx, queries.InsertPoolRevisionParams{
		RevisionID: revID, Name: p.Name, LevelMin: int32(p.LevelMin), LevelMax: int32(p.LevelMax), Difficulty: p.Difficulty,
	}); err != nil {
		return err
	}
	for i, m := range p.Members {
		if err := s.q.InsertPoolRevisionMember(ctx, queries.InsertPoolRevisionMemberParams{
			RevisionID: revID, Ordering: int32(i), MonsterSlug: m.Slug, Weight: int32(m.Weight), MinCount: int32(m.Min), MaxCount: int32(m.Max),
		}); err != nil {
			return err
		}
	}
	return nil
}

// PoolAt reads a Pool as one of its Revisions recorded it.
func (s *Store) PoolAt(ctx context.Context, campaign uuid.UUID, id domain.PoolID, no int) (domain.Pool, error) {
	r, err := s.q.GetPoolRevision(ctx, queries.GetPoolRevisionParams{CampaignID: campaign, EntityID: uuid.UUID(id), RevisionNo: int32(no)}) //nolint:gosec // bounded by the API
	if err != nil {
		return domain.Pool{}, notFound(err)
	}
	rows, err := s.q.PoolRevisionMembers(ctx, r.ID)
	if err != nil {
		return domain.Pool{}, err
	}
	p := domain.Pool{ID: id, Name: r.Name, LevelMin: int(r.LevelMin), LevelMax: int(r.LevelMax), Difficulty: r.Difficulty, Members: []domain.PoolMember{}}
	for _, m := range rows {
		p.Members = append(p.Members, domain.PoolMember{Slug: m.MonsterSlug, Weight: int(m.Weight), Min: int(m.MinCount), Max: int(m.MaxCount)})
	}
	return p, nil
}
