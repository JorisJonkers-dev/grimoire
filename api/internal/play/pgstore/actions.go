package pgstore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// LoadPendingActions reads the Hides, Grapples and Shoves of a Session waiting on rolls.
func (s *Store) LoadPendingActions(ctx context.Context, id domain.SessionID) ([]domain.PendingAction, error) {
	rows, err := s.q.SessionPendingActions(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := make([]domain.PendingAction, 0, len(rows))
	for _, r := range rows {
		p := domain.PendingAction{RollID: domain.RollID(r.RollID), Actor: domain.TokenID(r.ActorTokenID), Action: r.Action, DC: int(r.Dc)}
		if r.TargetTokenID.Valid {
			t := domain.TokenID(r.TargetTokenID.Bytes)
			p.Target = &t
		}
		out = append(out, p)
	}
	return out, nil
}

// saveActions writes what a 2024 action leaves: its rolls outside a fight, a pending Hide, Grapple or
// Shove and its settling, and creatures shoved or dragged.
//
//nolint:gosec // coordinates are bounded by the map
func (s *Store) saveActions(ctx context.Context, sess domain.Session, w live.Write, actor domain.Member, c caller.Caller, now time.Time) error {
	sid := uuid.UUID(sess.ID)
	if (w.Kind == domain.ActionTaken || w.Kind == domain.ActionUnarmed) && w.Combat == nil {
		if err := s.openRolls(ctx, sess, w.Rolls, actor, c, now); err != nil {
			return err
		}
	}
	if err := s.savePending(ctx, sid, w); err != nil {
		return err
	}
	for _, t := range []*domain.Token{w.Pushed, w.Dragged} {
		if t == nil {
			continue
		}
		if err := s.q.UpdateToken(ctx, queries.UpdateTokenParams{SessionID: sid, ID: uuid.UUID(t.ID), Q: int32(t.Q), R: int32(t.R), Hidden: t.Hidden}); err != nil {
			return err
		}
	}
	return nil
}

// savePending writes a Hide, Grapple or Shove that waits on its roll, or clears one that settled.
//
//nolint:gosec // DCs are bounded by the rules
func (s *Store) savePending(ctx context.Context, sid uuid.UUID, w live.Write) error {
	if p := w.Pending; p != nil {
		row := queries.InsertPendingActionParams{RollID: uuid.UUID(p.RollID), SessionID: sid, ActorTokenID: uuid.UUID(p.Actor), Action: p.Action, Dc: int32(p.DC)}
		if p.Target != nil {
			row.TargetTokenID = pgtype.UUID{Bytes: *p.Target, Valid: true}
		}
		if err := s.q.InsertPendingAction(ctx, row); err != nil {
			return err
		}
	}
	if w.Settled == (domain.RollID{}) {
		return nil
	}
	return s.q.DeletePendingAction(ctx, uuid.UUID(w.Settled))
}
