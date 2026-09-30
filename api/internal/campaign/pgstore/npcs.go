package pgstore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

func npc(r queries.GetNPCRow) domain.NPC {
	return domain.NPC{
		ID: domain.NPCID(r.ID), Name: r.Name, Title: r.Title, Description: r.Description, DMNotes: r.DmNotes,
		Disposition: r.Disposition, UpdatedAt: r.UpdatedAt,
	}
}

// InsertNPC stores an NPC, under the given id when restoring a deleted one.
func (s *Store) InsertNPC(ctx context.Context, id domain.CampaignID, npcID *domain.NPCID, n domain.NPC, now time.Time) (domain.NPCID, error) {
	p := queries.InsertNPCParams{
		CampaignID: uuid.UUID(id), Name: n.Name, Title: n.Title, Description: n.Description, DmNotes: n.DMNotes,
		Disposition: n.Disposition, Now: now,
	}
	if npcID != nil {
		p.ID = pgtype.UUID{Bytes: *npcID, Valid: true}
	}
	nid, err := s.q.InsertNPC(ctx, p)
	return domain.NPCID(nid), err
}

// UpdateNPC replaces an NPC's fields and reports whether it existed.
func (s *Store) UpdateNPC(ctx context.Context, id domain.CampaignID, n domain.NPC, now time.Time) (bool, error) {
	rows, err := s.q.UpdateNPC(ctx, queries.UpdateNPCParams{
		CampaignID: uuid.UUID(id), ID: uuid.UUID(n.ID), Name: n.Name, Title: n.Title, Description: n.Description,
		DmNotes: n.DMNotes, Disposition: n.Disposition, Now: now,
	})
	return rows == 1, err
}

// DeleteNPC removes an NPC and reports whether it existed.
func (s *Store) DeleteNPC(ctx context.Context, id domain.CampaignID, npcID domain.NPCID) (bool, error) {
	rows, err := s.q.DeleteNPC(ctx, queries.DeleteNPCParams{CampaignID: uuid.UUID(id), ID: uuid.UUID(npcID)})
	return rows == 1, err
}

// NPC reads one NPC.
func (s *Store) NPC(ctx context.Context, id domain.CampaignID, npcID domain.NPCID) (domain.NPC, error) {
	r, err := s.q.GetNPC(ctx, queries.GetNPCParams{CampaignID: uuid.UUID(id), ID: uuid.UUID(npcID)})
	if err != nil {
		return domain.NPC{}, notFound(err)
	}
	return npc(r), nil
}

// NPCs lists a Campaign's NPCs by name.
func (s *Store) NPCs(ctx context.Context, id domain.CampaignID) ([]domain.NPC, error) {
	rows, err := s.q.ListNPCs(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := make([]domain.NPC, 0, len(rows))
	for _, r := range rows {
		out = append(out, npc(queries.GetNPCRow(r)))
	}
	return out, nil
}

// DeletedNPCs lists NPCs known only from their Revisions.
func (s *Store) DeletedNPCs(ctx context.Context, id domain.CampaignID) ([]domain.DeletedNPC, error) {
	rows, err := s.q.DeletedNPCs(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := make([]domain.DeletedNPC, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.DeletedNPC{ID: domain.NPCID(r.EntityID), Name: r.Name, DeletedAt: r.CreatedAt})
	}
	return out, nil
}

// RecordNPCRevision numbers and stores a Revision with the NPC's snapshot. Call it inside a transaction.
func (s *Store) RecordNPCRevision(ctx context.Context, id domain.CampaignID, r domain.Revision, c caller.Caller, n domain.NPC) (int, error) {
	entity := uuid.UUID(n.ID)
	if err := s.q.LockEntity(ctx, string(domain.EntityNPC)+":"+entity.String()); err != nil {
		return 0, err
	}
	no, err := s.q.NextRevisionNo(ctx, queries.NextRevisionNoParams{EntityType: string(domain.EntityNPC), EntityID: entity})
	if err != nil {
		return 0, err
	}
	p := queries.InsertRevisionParams{
		CampaignID: uuid.UUID(id), EntityType: string(domain.EntityNPC), EntityID: entity, RevisionNo: no, Action: string(r.Action),
		CallerSubject: c.Subject, CallerName: r.Author, Origin: string(c.Origin), Client: c.Client, Now: r.CreatedAt,
	}
	if r.RestoredFrom > 0 {
		p.RestoredFrom = pgtype.Int4{Int32: int32(r.RestoredFrom), Valid: true} //nolint:gosec // revision numbers are small
	}
	revID, err := s.q.InsertRevision(ctx, p)
	if err != nil {
		return 0, err
	}
	err = s.q.InsertNPCRevision(ctx, queries.InsertNPCRevisionParams{
		RevisionID: revID, Name: n.Name, Title: n.Title, Description: n.Description, DmNotes: n.DMNotes, Disposition: n.Disposition,
	})
	return int(no), err
}

// Revisions lists an entity's Revisions, newest first.
func (s *Store) Revisions(ctx context.Context, id domain.CampaignID, t domain.EntityType, entity uuid.UUID) ([]domain.Revision, error) {
	rows, err := s.q.ListRevisions(ctx, queries.ListRevisionsParams{CampaignID: uuid.UUID(id), EntityType: string(t), EntityID: entity})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Revision, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Revision{
			No: int(r.RevisionNo), Action: domain.RevisionAction(r.Action), Author: r.CallerName, Origin: r.Origin, Client: r.Client,
			RestoredFrom: int(r.RestoredFrom.Int32), CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

// NPCRevision reads the NPC as one Revision recorded it.
func (s *Store) NPCRevision(ctx context.Context, id domain.CampaignID, npcID domain.NPCID, no int) (domain.NPC, error) {
	r, err := s.q.GetNPCRevision(ctx, queries.GetNPCRevisionParams{CampaignID: uuid.UUID(id), EntityID: uuid.UUID(npcID), RevisionNo: int32(no)}) //nolint:gosec // bounded by the API
	if err != nil {
		return domain.NPC{}, notFound(err)
	}
	return domain.NPC{ID: npcID, Name: r.Name, Title: r.Title, Description: r.Description, DMNotes: r.DmNotes, Disposition: r.Disposition}, nil
}
