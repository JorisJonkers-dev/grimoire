package app

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// designed reads an entry a builder opens: one of the caller's, or a Shared Library copy, of its kind.
func (s *Service) designed(ctx context.Context, c caller.Caller, id uuid.UUID, kind, builder string) (domain.Entry, error) {
	e, err := s.linkable(ctx, c, id)
	if err == nil && e.Kind != kind {
		err = apperr.Refuse("the " + builder + " opens only entries of kind " + kind)
	}
	return e, err
}

// designOf reads an entry's stored design over where a new one starts.
func designOf[D any](e domain.Entry, first D) D {
	if e.Design != nil {
		_ = json.Unmarshal(e.Design, &first) // stored designs were checked when saved
	}
	return first
}

// ownDesigned is one of the caller's own entries of a builder's kind, to save a design on.
func (s *Service) ownDesigned(ctx context.Context, c caller.Caller, id uuid.UUID, kind, builder string) (domain.Entry, error) {
	e, err := s.owned(ctx, c, id)
	if err == nil && e.Kind != kind {
		err = apperr.Refuse("the " + builder + " opens only entries of kind " + kind)
	}
	return e, err
}

// saveDesign stores a checked design on an entry as its next Revision.
func (s *Service) saveDesign(ctx context.Context, c caller.Caller, e domain.Entry, design any) error {
	raw, _ := json.Marshal(design) //nolint:errchkjson // a design is plain data
	now := s.Now()
	return s.Repo.InTx(ctx, func(r Repository) error {
		no, err := r.UpdateEntry(ctx, e.ID, e.Name, e.Fields, raw, now)
		if err != nil {
			return err
		}
		return r.InsertRevision(ctx, e.ID, by(c, domain.Revision{No: no, Name: e.Name, Fields: e.Fields, Design: raw, At: now}))
	})
}
