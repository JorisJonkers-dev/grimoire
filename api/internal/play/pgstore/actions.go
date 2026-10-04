package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dying"
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
		if r.ObjectID.Valid {
			o := domain.ObjectID(r.ObjectID.Bytes)
			p.Object = &o
		}
		if r.RollTable.Valid {
			table := uuid.UUID(r.RollTable.Bytes)
			p.Table, p.Hook = &table, r.HookName.String
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
	opens := w.Kind == domain.ActionTaken || w.Kind == domain.ActionUnarmed || w.Kind == domain.ActionMasteryUsed || w.Kind == domain.ActionConcentrationChecked ||
		w.Kind == domain.ActionDyingChanged || w.Kind == domain.ActionSneakStarted || w.Kind == domain.ActionHookFired || w.Kind == domain.ActionSeatChecked
	if opens && w.Combat == nil {
		if err := s.openRolls(ctx, sess, w.Rolls, actor, c, now); err != nil {
			return err
		}
	}
	if err := s.savePending(ctx, sid, w); err != nil {
		return err
	}

	if err := s.saveSneak(ctx, sid, w); err != nil {
		return err
	}
	if err := s.saveExploration(ctx, sid, w); err != nil {
		return err
	}
	if err := s.saveDying(ctx, sid, w); err != nil {
		return err
	}
	return s.saveCarried(ctx, sid, w)
}

// saveCarried writes where a change left the creatures it moved besides its own: one shoved, one
// dragged, and riders taken along by their mounts or off them.
//
//nolint:gosec // coordinates are bounded by the map
func (s *Store) saveCarried(ctx context.Context, sid uuid.UUID, w live.Write) error {
	for _, t := range []*domain.Token{w.Pushed, w.Dragged} {
		if t == nil {
			continue
		}
		if err := s.q.UpdateToken(ctx, queries.UpdateTokenParams{SessionID: sid, ID: uuid.UUID(t.ID), Q: int32(t.Q), R: int32(t.R), Hidden: t.Hidden}); err != nil {
			return err
		}
	}
	for _, t := range w.Riders {
		if err := s.saveSeat(ctx, sid, t); err != nil {
			return err
		}
	}
	return nil
}

// saveSeat writes where a rider is and what it rides.
//
//nolint:gosec // coordinates are bounded by the map
func (s *Store) saveSeat(ctx context.Context, sid uuid.UUID, t domain.Token) error {
	p := queries.SetTokenSeatParams{SessionID: sid, ID: uuid.UUID(t.ID), Q: int32(t.Q), R: int32(t.R), MountControlled: t.Steers}
	if t.Mount != nil {
		p.MountTokenID = pgtype.UUID{Bytes: *t.Mount, Valid: true}
	}
	return s.q.SetTokenSeat(ctx, p)
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
		if p.Object != nil {
			row.ObjectID = pgtype.UUID{Bytes: *p.Object, Valid: true}
		}
		if p.Table != nil {
			row.RollTable, row.HookName = pgtype.UUID{Bytes: *p.Table, Valid: true}, pgtype.Text{String: p.Hook, Valid: true}
		}
		if err := s.q.InsertPendingAction(ctx, row); err != nil {
			return err
		}
	}
	if w.Settled == (domain.RollID{}) {
		return nil
	}
	if err := s.q.DeletePendingAction(ctx, uuid.UUID(w.Settled)); err != nil {
		return err
	}
	// What the settled check left behind: the attitude an Influence check moved.
	a := w.Attitude
	if a == nil {
		return nil
	}
	return s.q.SetAttitude(ctx, queries.SetAttitudeParams{SessionID: sid, TokenID: uuid.UUID(a.Token), CharacterID: a.Character, Attitude: a.Value})
}

// LoadDying reads the Characters of a Session at 0 hit points.
func (s *Store) LoadDying(ctx context.Context, id domain.SessionID) (map[domain.TokenID]domain.Dying, error) {
	rows, err := s.q.SessionDying(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := map[domain.TokenID]domain.Dying{}
	for _, r := range rows {
		d := domain.Dying{
			Token: domain.TokenID(r.TokenID),
			State: dying.State{Successes: int(r.Successes), Failures: int(r.Failures), Stable: r.Stable, Dead: r.Dead},
			Died:  dying.Died{Fight: r.DiedFight.String, Round: int(r.DiedRound), Day: int(r.DiedDay)},
		}
		if r.EffectID.Valid {
			e := domain.EffectID(r.EffectID.Bytes)
			d.Effect = &e
		}
		if r.RollID.Valid {
			roll := domain.RollID(r.RollID.Bytes)
			d.RollID = &roll
		}
		out[d.Token] = d
	}
	return out, nil
}

// saveDying writes a Character's death saves, or drops them once it is back on its feet.
//
//nolint:gosec // counts and rounds are bounded by the rules
func (s *Store) saveDying(ctx context.Context, sid uuid.UUID, w live.Write) error {
	if id := w.Undying; id != nil {
		return s.q.DeleteDying(ctx, uuid.UUID(*id))
	}
	d := w.Dying
	if d == nil {
		return nil
	}
	p := queries.SaveDyingParams{
		TokenID: uuid.UUID(d.Token), SessionID: sid, Successes: int32(d.State.Successes), Failures: int32(d.State.Failures), Stable: d.State.Stable,
		Dead: d.State.Dead, DiedFight: pgtype.Text{String: d.Died.Fight, Valid: d.Died.Fight != ""}, DiedRound: int32(d.Died.Round), DiedDay: int32(d.Died.Day),
	}
	if d.Effect != nil {
		p.EffectID = pgtype.UUID{Bytes: *d.Effect, Valid: true}
	}
	if d.RollID != nil {
		p.RollID = pgtype.UUID{Bytes: *d.RollID, Valid: true}
	}
	return s.q.SaveDying(ctx, p)
}

// saveSneak writes whether the party sneaks and its Stealth rolls.
//
//nolint:gosec // totals are bounded by the dice
func (s *Store) saveSneak(ctx context.Context, sid uuid.UUID, w live.Write) error {
	if !w.SaveSneak {
		return nil
	}
	if err := s.q.SetSessionSneaking(ctx, queries.SetSessionSneakingParams{ID: sid, Sneaking: w.Sneak != nil}); err != nil {
		return err
	}
	if err := s.q.ClearSneakRolls(ctx, sid); err != nil || w.Sneak == nil {
		return err
	}
	for _, r := range w.Sneak.Rolls {
		p := queries.AddSneakRollParams{SessionID: sid, TokenID: uuid.UUID(r.Token), RollID: uuid.UUID(r.RollID)}
		if r.Total != nil {
			p.Total = pgtype.Int4{Int32: int32(*r.Total), Valid: true}
		}
		if err := s.q.AddSneakRoll(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

// LoadSneak reads the party's sneaking, nil when it is not.
func (s *Store) LoadSneak(ctx context.Context, id domain.SessionID) (*domain.Sneak, error) {
	sneaking, err := s.q.SessionSneaking(ctx, uuid.UUID(id))
	if err != nil || !sneaking {
		return nil, err
	}
	rows, err := s.q.SessionSneakRolls(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := &domain.Sneak{Rolls: nil}
	for _, r := range rows {
		sr := domain.SneakRoll{Token: domain.TokenID(r.TokenID), RollID: domain.RollID(r.RollID), Total: nil}
		if r.Total.Valid {
			total := int(r.Total.Int32)
			sr.Total = &total
		}
		out.Rolls = append(out.Rolls, sr)
	}
	return out, nil
}

// CampaignInitiative reads how a Campaign's fights roll initiative.
func (s *Store) CampaignInitiative(ctx context.Context, campaign uuid.UUID) (string, bool, error) {
	r, err := s.q.CampaignInitiative(ctx, campaign)
	return r.InitiativeMode, r.ShareInitiative, err
}

// saveExploration writes Exploration's turns, or clears them.
//
//nolint:gosec // turns and feet are bounded by the rules
func (s *Store) saveExploration(ctx context.Context, sid uuid.UUID, w live.Write) error {
	if !w.SaveExplore {
		return nil
	}
	e := w.Explore
	if e == nil {
		return s.q.ClearExploration(ctx, sid)
	}
	order := make([]uuid.UUID, 0, len(e.Order))
	for _, id := range e.Order {
		order = append(order, uuid.UUID(id))
	}
	return s.q.SaveExploration(ctx, queries.SaveExplorationParams{SessionID: sid, TurnOrder: order, Turn: int32(e.Turn), MovedFt: int32(e.MovedFt)})
}

// LoadExploration reads Exploration's turns, nil when it has none.
func (s *Store) LoadExploration(ctx context.Context, id domain.SessionID) (*domain.Exploration, error) {
	r, err := s.q.SessionExploration(ctx, uuid.UUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // no turns is not an error
	}
	if err != nil {
		return nil, err
	}
	out := &domain.Exploration{Order: nil, Turn: int(r.Turn), MovedFt: int(r.MovedFt)}
	for _, t := range r.TurnOrder {
		out.Order = append(out.Order, domain.TokenID(t))
	}
	return out, nil
}

// LoadAttitudes reads how the creatures of a Session take to the Campaign's Characters.
func (s *Store) LoadAttitudes(ctx context.Context, id domain.SessionID) ([]domain.Attitude, error) {
	rows, err := s.q.SessionAttitudes(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Attitude, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Attitude{Token: domain.TokenID(r.TokenID), Character: r.CharacterID, Value: r.Attitude})
	}
	return out, nil
}

// Difficulty reads the Campaign's difficulty preset.
func (s *Store) Difficulty(ctx context.Context, campaign uuid.UUID) (string, error) {
	return s.q.CampaignDifficulty(ctx, campaign)
}

// ShowDCs reads whether the Campaign shows the DC of a check on its Roll Card.
func (s *Store) ShowDCs(ctx context.Context, campaign uuid.UUID) (bool, error) {
	return s.q.CampaignShowDCs(ctx, campaign)
}
