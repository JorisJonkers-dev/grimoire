package app

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// NPCs runs the NPC use cases. NPCs are DM prep: every operation is DM only and every write is a Revision.
type NPCs struct {
	Repo Repository
	Now  func() time.Time
}

// NPCInput is the editable part of an NPC.
type NPCInput struct {
	Name        string
	Title       string
	Description string
	DMNotes     string
	Disposition string
}

func (in NPCInput) validate() (domain.NPC, error) {
	name, err := cleanText(in.Name, 80)
	if err != nil {
		return domain.NPC{}, err
	}
	switch {
	case utf8.RuneCountInString(in.Title) > 80, utf8.RuneCountInString(in.Description) > 8000, utf8.RuneCountInString(in.DMNotes) > 8000:
		return domain.NPC{}, domain.ErrInvalid
	case in.Disposition != "friendly" && in.Disposition != "neutral" && in.Disposition != "hostile":
		return domain.NPC{}, domain.ErrInvalid
	}
	return domain.NPC{Name: name, Title: in.Title, Description: in.Description, DMNotes: in.DMNotes, Disposition: in.Disposition}, nil
}

// write runs fn in a transaction as a DM and records the Revision it returns.
func (s *NPCs) write(ctx context.Context, c caller.Caller, id domain.CampaignID, fn func(r Repository, now time.Time) (domain.NPC, domain.Revision, error)) (domain.NPC, error) {
	var out domain.NPC
	err := s.Repo.InTx(ctx, func(r Repository) error {
		me, err := dm(ctx, r, c, id)
		if err != nil {
			return err
		}
		now := s.Now()
		n, rev, err := fn(r, now)
		if err != nil {
			return err
		}
		rev.Author, rev.CreatedAt = me.DisplayName, now
		if _, err := r.RecordNPCRevision(ctx, id, rev, c, n); err != nil {
			return err
		}
		out = n
		return nil
	})
	return out, err
}

// Create adds an NPC.
func (s *NPCs) Create(ctx context.Context, c caller.Caller, id domain.CampaignID, in NPCInput) (domain.NPC, error) {
	n, err := in.validate()
	if err != nil {
		return domain.NPC{}, err
	}
	return s.write(ctx, c, id, func(r Repository, now time.Time) (domain.NPC, domain.Revision, error) {
		nid, err := r.InsertNPC(ctx, id, nil, n, now)
		n.ID, n.UpdatedAt = nid, now
		return n, domain.Revision{Action: domain.ActionCreate}, err
	})
}

// Update replaces an NPC's fields.
func (s *NPCs) Update(ctx context.Context, c caller.Caller, id domain.CampaignID, npcID domain.NPCID, in NPCInput) (domain.NPC, error) {
	n, err := in.validate()
	if err != nil {
		return domain.NPC{}, err
	}
	n.ID = npcID
	return s.write(ctx, c, id, func(r Repository, now time.Time) (domain.NPC, domain.Revision, error) {
		n.UpdatedAt = now
		return n, domain.Revision{Action: domain.ActionUpdate}, found(r.UpdateNPC(ctx, id, n, now))
	})
}

// Delete removes an NPC; its Revisions keep it restorable.
func (s *NPCs) Delete(ctx context.Context, c caller.Caller, id domain.CampaignID, npcID domain.NPCID) error {
	_, err := s.write(ctx, c, id, func(r Repository, _ time.Time) (domain.NPC, domain.Revision, error) {
		n, err := r.NPC(ctx, id, npcID)
		if err != nil {
			return domain.NPC{}, domain.Revision{}, err
		}
		return n, domain.Revision{Action: domain.ActionDelete}, found(r.DeleteNPC(ctx, id, npcID))
	})
	return err
}

// Restore brings an NPC back to the state of one of its Revisions, recreating it if it was deleted.
func (s *NPCs) Restore(ctx context.Context, c caller.Caller, id domain.CampaignID, npcID domain.NPCID, no int) (domain.NPC, error) {
	return s.write(ctx, c, id, func(r Repository, now time.Time) (domain.NPC, domain.Revision, error) {
		n, err := r.NPCRevision(ctx, id, npcID, no)
		if err != nil {
			return domain.NPC{}, domain.Revision{}, err
		}
		n.ID, n.UpdatedAt = npcID, now
		rev := domain.Revision{Action: domain.ActionRestore, RestoredFrom: no}
		ok, err := r.UpdateNPC(ctx, id, n, now)
		if err != nil || ok {
			return n, rev, err
		}
		_, err = r.InsertNPC(ctx, id, &npcID, n, now)
		return n, rev, err
	})
}

func found(ok bool, err error) error {
	if err == nil && !ok {
		return domain.ErrNotFound
	}
	return err
}

// List returns the Campaign's NPCs.
func (s *NPCs) List(ctx context.Context, c caller.Caller, id domain.CampaignID) ([]domain.NPC, error) {
	if _, err := dm(ctx, s.Repo, c, id); err != nil {
		return nil, err
	}
	return s.Repo.NPCs(ctx, id)
}

// Deleted lists NPCs that were deleted and can be restored.
func (s *NPCs) Deleted(ctx context.Context, c caller.Caller, id domain.CampaignID) ([]domain.DeletedNPC, error) {
	if _, err := dm(ctx, s.Repo, c, id); err != nil {
		return nil, err
	}
	return s.Repo.DeletedNPCs(ctx, id)
}

// Get returns one NPC.
func (s *NPCs) Get(ctx context.Context, c caller.Caller, id domain.CampaignID, npcID domain.NPCID) (domain.NPC, error) {
	if _, err := dm(ctx, s.Repo, c, id); err != nil {
		return domain.NPC{}, err
	}
	return s.Repo.NPC(ctx, id, npcID)
}

// Revisions lists an NPC's Revisions, newest first; they remain after the NPC is deleted.
func (s *NPCs) Revisions(ctx context.Context, c caller.Caller, id domain.CampaignID, npcID domain.NPCID) ([]domain.Revision, error) {
	if _, err := dm(ctx, s.Repo, c, id); err != nil {
		return nil, err
	}
	revs, err := s.Repo.Revisions(ctx, id, domain.EntityNPC, uuid.UUID(npcID))
	if err == nil && len(revs) == 0 {
		return nil, domain.ErrNotFound
	}
	return revs, err
}

// Diff compares two Revisions of an NPC.
func (s *NPCs) Diff(ctx context.Context, c caller.Caller, id domain.CampaignID, npcID domain.NPCID, from, to int) ([]domain.Change, error) {
	if _, err := dm(ctx, s.Repo, c, id); err != nil {
		return nil, err
	}
	before, err := s.Repo.NPCRevision(ctx, id, npcID, from)
	if err != nil {
		return nil, err
	}
	after, err := s.Repo.NPCRevision(ctx, id, npcID, to)
	if err != nil {
		return nil, err
	}
	return domain.Diff(before.Fields(), after.Fields()), nil
}
