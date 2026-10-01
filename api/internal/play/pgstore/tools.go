package pgstore

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

// logTools records what undo needs: the creatures a spawn placed, the Effect an effect_applied put on,
// and the Action an undo reverted.
func (s *Store) logTools(ctx context.Context, actionID uuid.UUID, w live.Write) error {
	for _, t := range w.Spawned {
		if err := s.q.InsertSpawnEvent(ctx, queries.InsertSpawnEventParams{ActionID: actionID, TokenID: uuid.UUID(t.ID), Label: t.Label}); err != nil {
			return err
		}
	}
	if w.Kind == domain.ActionEffectApplied {
		if err := s.q.InsertEffectEvent(ctx, queries.InsertEffectEventParams{ActionID: actionID, EffectID: uuid.UUID(w.Applied())}); err != nil {
			return err
		}
	}
	if w.Undoes == uuid.Nil {
		return nil
	}
	return s.q.InsertUndo(ctx, queries.InsertUndoParams{ActionID: actionID, UndoesActionID: w.Undoes})
}

// Action reads one Action of the Session by its Action Log sequence, with what undoing it needs.
func (s *Store) Action(ctx context.Context, id domain.SessionID, seq int64) (live.ActionRecord, error) {
	row, err := s.q.SessionActionBySeq(ctx, queries.SessionActionBySeqParams{SessionID: pgtype.UUID{Bytes: id, Valid: true}, Seq: seq})
	if errors.Is(err, pgx.ErrNoRows) {
		return live.ActionRecord{}, apperr.ErrNotFound
	}
	if err != nil {
		return live.ActionRecord{}, err
	}
	a := live.ActionRecord{ID: row.ID, Kind: row.Kind, Undone: row.Undone}
	switch a.Kind {
	case domain.ActionTokenPlaced:
		err = s.tokenOf(ctx, &a)
	case domain.ActionEffectApplied:
		if err = s.tokenOf(ctx, &a); err == nil {
			var effect uuid.UUID
			effect, err = s.q.ActionEffectEvent(ctx, a.ID)
			a.Effect = domain.EffectID(effect)
		}
	case domain.ActionHexesRevealed, domain.ActionHexesConcealed:
		var rows []queries.ActionHexEventsRow
		rows, err = s.q.ActionHexEvents(ctx, a.ID)
		for _, r := range rows {
			a.Hexes = append(a.Hexes, hex.Coord{Q: int(r.Q), R: int(r.R)})
		}
	case domain.ActionDamageDealt, domain.ActionHPAdjusted:
		var hp queries.ActionHPEventRow
		hp, err = s.q.ActionHPEvent(ctx, a.ID)
		a.HP = live.HPChange{Token: domain.TokenID(hp.TokenID), Before: int(hp.HpBefore), After: int(hp.HpAfter)}
	case domain.ActionEncounterSpawned:
		var rows []queries.ActionSpawnEventsRow
		rows, err = s.q.ActionSpawnEvents(ctx, a.ID)
		for _, r := range rows {
			a.Spawned = append(a.Spawned, domain.TokenID(r.TokenID))
		}
	}
	return a, err
}

func (s *Store) tokenOf(ctx context.Context, a *live.ActionRecord) error {
	t, err := s.q.ActionTokenEvent(ctx, a.ID)
	a.Token = domain.TokenID(t.TokenID)
	return err
}

// SessionLog lists the Session's latest Actions, newest first.
func (s *Store) SessionLog(ctx context.Context, id domain.SessionID, limit int) ([]domain.LoggedAction, error) {
	rows, err := s.q.SessionLog(ctx, queries.SessionLogParams{SessionID: pgtype.UUID{Bytes: id, Valid: true}, Lim: int32(limit)}) //nolint:gosec // bounded by the API
	if err != nil {
		return nil, err
	}
	out := make([]domain.LoggedAction, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.LoggedAction{
			Seq: r.Seq, Kind: r.Kind, Actor: r.ActorName, Origin: r.Origin, Client: r.Client, Label: r.Label, Undone: r.Undone, At: r.CreatedAt,
		})
	}
	return out, nil
}
