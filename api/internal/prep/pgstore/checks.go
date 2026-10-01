package pgstore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

func tableRef(id pgtype.UUID) *domain.TableID {
	if !id.Valid {
		return nil
	}
	t := domain.TableID(id.Bytes)
	return &t
}

func check(r queries.CampaignChecksRow) domain.Check {
	return domain.Check{
		ID: domain.CheckID(r.ID), SessionID: fromUUID(r.SessionID), TableID: tableRef(r.TableID), TableName: r.TableName, Trigger: r.Trigger,
		Mode: r.Mode, Visibility: r.Visibility, Seed: r.Seed, ChancePct: int(r.ChancePct), ChanceRoll: int(r.ChanceRoll.Int32),
		RollID: fromUUID(r.RollID), Status: r.Status, Outcome: r.Outcome.String, EntryLabel: r.EntryLabel, Monsters: []domain.EntryMonster{}, CreatedAt: r.CreatedAt,
	}
}

// withMonsters attaches each check's monsters.
func withMonsters(ctx context.Context, q *queries.Queries, campaign uuid.UUID, out []domain.Check) ([]domain.Check, error) {
	rows, err := q.CheckMonsters(ctx, campaign)
	if err != nil {
		return nil, err
	}
	for i := range out {
		for _, m := range rows {
			if m.CheckID == uuid.UUID(out[i].ID) {
				out[i].Monsters = append(out[i].Monsters, domain.EntryMonster{Slug: m.MonsterSlug, Count: int(m.Count)})
			}
		}
	}
	return out, nil
}

// Checks lists the Campaign's latest Encounter Checks, newest first.
func (s *Store) Checks(ctx context.Context, campaign uuid.UUID) ([]domain.Check, error) {
	rows, err := s.q.CampaignChecks(ctx, campaign)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Check, 0, len(rows))
	for _, r := range rows {
		out = append(out, check(r))
	}
	return withMonsters(ctx, s.q, campaign, out)
}

// SessionChecks lists a Session's Encounter Checks, oldest first.
func SessionChecks(ctx context.Context, q *queries.Queries, campaign, sid uuid.UUID) ([]domain.Check, error) {
	rows, err := q.SessionChecks(ctx, pgtype.UUID{Bytes: sid, Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Check, 0, len(rows))
	for _, r := range rows {
		out = append(out, check(queries.CampaignChecksRow(r)))
	}
	return withMonsters(ctx, q, campaign, out)
}

// WriteCheck stores an Encounter Check, and its monsters once it resolves.
//
//nolint:gosec // chances, rolls and counts are bounded by the rules
func WriteCheck(ctx context.Context, q *queries.Queries, campaign uuid.UUID, c domain.Check, now time.Time) error {
	p := queries.SaveEncounterCheckParams{
		ID: uuid.UUID(c.ID), CampaignID: campaign, SessionID: optUUID(c.SessionID), TableName: c.TableName, Trigger: c.Trigger, Mode: c.Mode,
		Visibility: c.Visibility, Seed: c.Seed, ChancePct: int32(c.ChancePct), RollID: optUUID(c.RollID), Status: c.Status, EntryLabel: c.EntryLabel, Now: now,
	}
	if c.TableID != nil {
		p.TableID = pgtype.UUID{Bytes: *c.TableID, Valid: true}
	}
	if c.ChanceRoll > 0 {
		p.ChanceRoll = pgtype.Int4{Int32: int32(c.ChanceRoll), Valid: true}
	}
	if c.Outcome != "" {
		p.Outcome = pgtype.Text{String: c.Outcome, Valid: true}
	}
	if err := q.SaveEncounterCheck(ctx, p); err != nil {
		return err
	}
	if c.Status != domain.CheckResolved {
		return nil
	}
	for i, m := range c.Monsters {
		if err := q.InsertCheckMonster(ctx, queries.InsertCheckMonsterParams{CheckID: uuid.UUID(c.ID), Position: int32(i), MonsterSlug: m.Slug, Count: int32(m.Count)}); err != nil {
			return err
		}
	}
	return nil
}

// LoadPrep reads everything an Encounter Check draws on.
func LoadPrep(ctx context.Context, q *queries.Queries, campaign uuid.UUID) (domain.Prep, error) {
	out := domain.Prep{XP: map[string]int{}, Scheduled: []domain.Scheduled{}}
	var err error
	if out.Tables, err = LoadTables(ctx, q, campaign); err != nil {
		return out, err
	}
	if out.Pools, err = LoadPools(ctx, q, campaign); err != nil {
		return out, err
	}
	for _, p := range out.Pools {
		for _, m := range p.Members {
			r, err := q.MonsterXP(ctx, queries.MonsterXPParams{Slug: m.Slug, CampaignID: campaign})
			if err != nil {
				return out, err
			}
			out.XP[m.Slug] = int(r.Xp)
		}
	}
	levels, err := q.PartyLevels(ctx, campaign)
	if err != nil {
		return out, err
	}
	for _, l := range levels {
		out.Levels = append(out.Levels, int(l))
	}
	due, err := q.ScheduledChecks(ctx, campaign)
	if err != nil {
		return out, err
	}
	for _, d := range due {
		out.Scheduled = append(out.Scheduled, domain.Scheduled{ID: d.ID, TableID: domain.TableID(d.TableID), Due: d.Due})
	}
	return out, nil
}

// RecordCheck records an Encounter Check's Revision: its creation, or the roll that resolved it.
func RecordCheck(ctx context.Context, q *queries.Queries, campaign uuid.UUID, id domain.CheckID, action, author string, c caller.Caller, now time.Time) error {
	no, err := q.NextRevisionNo(ctx, queries.NextRevisionNoParams{EntityType: domain.EntityCheck, EntityID: uuid.UUID(id)})
	if err != nil {
		return err
	}
	_, err = q.InsertRevision(ctx, queries.InsertRevisionParams{
		CampaignID: campaign, EntityType: domain.EntityCheck, EntityID: uuid.UUID(id), RevisionNo: no, Action: action,
		CallerSubject: c.Subject, CallerName: author, Origin: string(c.Origin), Client: c.Client, Now: now,
	})
	return err
}
