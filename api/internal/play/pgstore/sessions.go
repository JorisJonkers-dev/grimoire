package pgstore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

var (
	_ app.SessionRepository = (*Store)(nil)
	_ live.Store            = (*Store)(nil)
)

func session(s queries.PlaySession) domain.Session {
	return domain.Session{
		ID: domain.SessionID(s.ID), CampaignID: s.CampaignID, Number: int(s.Number), Status: s.Status, Seq: s.Seq,
		GridRadius: int(s.GridRadius), StartedAt: s.StartedAt, EndedAt: s.EndedAt.Time,
	}
}

// sessionAction appends a Session Action with the next campaign sequence; call it inside a transaction.
func (s *Store) sessionAction(ctx context.Context, campaign uuid.UUID, sid domain.SessionID, kind string, actor domain.Member, c caller.Caller, now time.Time) (uuid.UUID, error) {
	if err := s.q.LockCampaignLog(ctx, "log:"+campaign.String()); err != nil {
		return uuid.UUID{}, err
	}
	seq, err := s.q.NextActionSeq(ctx, campaign)
	if err != nil {
		return uuid.UUID{}, err
	}
	return s.q.InsertSessionAction(ctx, queries.InsertSessionActionParams{
		CampaignID: campaign, SessionID: pgtype.UUID{Bytes: sid, Valid: true}, Seq: int64(seq), Kind: kind,
		ActorMemberID: actor.ID, ActorName: actor.Name, Origin: string(c.Origin), Client: c.Client, Now: now,
	})
}

// CreateSession opens the Campaign's next Session.
func (s *Store) CreateSession(ctx context.Context, campaign uuid.UUID, actor domain.Member, c caller.Caller, now time.Time) (domain.Session, error) {
	var out domain.Session
	err := s.InTx(ctx, func(r app.Repository) error {
		tx := r.(*Store) //nolint:forcetypeassert // InTx always hands back a *Store
		if err := tx.q.LockCampaignLog(ctx, "sessions:"+campaign.String()); err != nil {
			return err
		}
		n, err := tx.q.NextSessionNumber(ctx, campaign)
		if err != nil {
			return err
		}
		row, err := tx.q.InsertSession(ctx, queries.InsertSessionParams{CampaignID: campaign, Number: n, Now: now})
		if err != nil {
			return err
		}
		out = session(row)
		_, err = tx.sessionAction(ctx, campaign, out.ID, domain.ActionSessionStarted, actor, c, now)
		return err
	})
	return out, err
}

// Session reads one Session of a Campaign.
func (s *Store) Session(ctx context.Context, campaign uuid.UUID, id domain.SessionID) (domain.Session, error) {
	row, err := s.q.GetSession(ctx, queries.GetSessionParams{CampaignID: campaign, ID: uuid.UUID(id)})
	if err != nil {
		return domain.Session{}, notFound(err)
	}
	return session(row), nil
}

// Sessions lists a Campaign's Sessions, newest first.
func (s *Store) Sessions(ctx context.Context, campaign uuid.UUID) ([]domain.Session, error) {
	rows, err := s.q.ListSessions(ctx, campaign)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Session, 0, len(rows))
	for _, r := range rows {
		out = append(out, session(r))
	}
	return out, nil
}

// EndSession ends a live Session.
func (s *Store) EndSession(ctx context.Context, campaign uuid.UUID, id domain.SessionID, actor domain.Member, c caller.Caller, now time.Time) error {
	return s.InTx(ctx, func(r app.Repository) error {
		tx := r.(*Store) //nolint:forcetypeassert // InTx always hands back a *Store
		n, err := tx.q.EndSession(ctx, queries.EndSessionParams{CampaignID: campaign, ID: uuid.UUID(id), Now: pgtypeTime(now)})
		if err != nil {
			return err
		}
		if n == 0 {
			return apperr.ErrNotFound
		}
		_, err = tx.sessionAction(ctx, campaign, id, domain.ActionSessionEnded, actor, c, now)
		return err
	})
}

// Load reads a Session and its Tokens for the live runtime.
func (s *Store) Load(ctx context.Context, id domain.SessionID) (domain.Session, []domain.Token, error) {
	row, err := s.q.SessionByID(ctx, uuid.UUID(id))
	if err != nil {
		return domain.Session{}, nil, notFound(err)
	}
	rows, err := s.q.SessionTokens(ctx, row.ID)
	if err != nil {
		return domain.Session{}, nil, err
	}
	tokens := make([]domain.Token, 0, len(rows))
	for _, t := range rows {
		tokens = append(tokens, domain.Token{ID: domain.TokenID(t.ID), Label: t.Label, Kind: t.Kind, Q: int(t.Q), R: int(t.R), Hidden: t.Hidden})
	}
	return session(row), tokens, nil
}

// Apply writes one token change, bumps the Session sequence and logs the Action, all in one transaction.
func (s *Store) Apply(ctx context.Context, sess domain.Session, ch live.Change, actor domain.Member, c caller.Caller, now time.Time) (int64, domain.Token, error) {
	var seq int64
	t := ch.Token
	err := s.InTx(ctx, func(r app.Repository) error {
		tx := r.(*Store) //nolint:forcetypeassert // InTx always hands back a *Store
		sid := uuid.UUID(sess.ID)
		var err error
		if seq, err = tx.q.BumpSessionSeq(ctx, sid); err != nil {
			return err
		}
		if err := tx.mutate(ctx, sid, ch.Kind, &t); err != nil {
			return err
		}
		actionID, err := tx.sessionAction(ctx, sess.CampaignID, sess.ID, ch.Kind, actor, c, now)
		if err != nil {
			return err
		}
		return tx.q.InsertTokenEvent(ctx, queries.InsertTokenEventParams{
			ActionID: actionID, TokenID: uuid.UUID(t.ID), Label: t.Label, Q: int32(t.Q), R: int32(t.R), Hidden: t.Hidden, //nolint:gosec // bounded by the grid
		})
	})
	return seq, t, err
}

//nolint:gosec // coordinates are bounded by the grid radius
func (s *Store) mutate(ctx context.Context, sid uuid.UUID, kind string, t *domain.Token) error {
	switch kind {
	case domain.ActionTokenPlaced:
		id, err := s.q.InsertToken(ctx, queries.InsertTokenParams{SessionID: sid, Label: t.Label, Kind: t.Kind, Q: int32(t.Q), R: int32(t.R), Hidden: t.Hidden})
		t.ID = domain.TokenID(id)
		return err
	case domain.ActionTokenRemoved:
		return s.q.DeleteToken(ctx, queries.DeleteTokenParams{SessionID: sid, ID: uuid.UUID(t.ID)})
	default:
		return s.q.UpdateToken(ctx, queries.UpdateTokenParams{SessionID: sid, ID: uuid.UUID(t.ID), Q: int32(t.Q), R: int32(t.R), Hidden: t.Hidden})
	}
}

// Owner holds a Postgres advisory lock per live Session, so two processes can never run the same Session.
type Owner struct {
	Pool *pgxpool.Pool
}

// Acquire takes the lock on a dedicated connection and returns its release.
func (o Owner) Acquire(ctx context.Context, id domain.SessionID) (func(), error) {
	conn, err := o.Pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	q := queries.New(conn)
	key := "session:" + uuid.UUID(id).String()
	ok, err := q.LockSessionOwner(ctx, key)
	if err != nil || !ok {
		conn.Release()
		if err == nil {
			err = apperr.ErrConflict
		}
		return nil, err
	}
	return func() {
		_, _ = q.UnlockSessionOwner(context.Background(), key)
		conn.Release()
	}, nil
}
