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
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

var (
	_ app.SessionRepository = (*Store)(nil)
	_ live.Store            = (*Store)(nil)
)

func session(s queries.PlaySession) domain.Session {
	out := domain.Session{
		ID: domain.SessionID(s.ID), CampaignID: s.CampaignID, Number: int(s.Number), Status: s.Status, Seq: s.Seq,
		GridRadius: int(s.GridRadius), StartedAt: s.StartedAt, EndedAt: s.EndedAt.Time,
	}
	if s.MapID.Valid {
		id := domain.MapID(s.MapID.Bytes)
		out.MapID = &id
	}
	return out
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

// Load reads a Session, its Tokens and its active Map for the live runtime.
func (s *Store) Load(ctx context.Context, id domain.SessionID) (domain.Session, []domain.Token, *domain.MapState, error) {
	row, err := s.q.SessionByID(ctx, uuid.UUID(id))
	if err != nil {
		return domain.Session{}, nil, nil, notFound(err)
	}
	rows, err := s.q.SessionTokens(ctx, row.ID)
	if err != nil {
		return domain.Session{}, nil, nil, err
	}
	tokens := make([]domain.Token, 0, len(rows))
	for _, t := range rows {
		tokens = append(tokens, domain.Token{
			ID: domain.TokenID(t.ID), Label: t.Label, Kind: t.Kind, Q: int(t.Q), R: int(t.R), Hidden: t.Hidden, DarkvisionFt: int(t.DarkvisionFt),
		})
	}
	sess := session(row)
	if sess.MapID == nil {
		return sess, tokens, nil, nil
	}
	board, err := s.LoadMap(ctx, sess.CampaignID, *sess.MapID)
	return sess, tokens, board, err
}

// Commit writes one change, the hexes it reveals, the next Session sequence and its Action in one transaction.
func (s *Store) Commit(ctx context.Context, sess domain.Session, board *domain.MapState, w live.Write, actor domain.Member, c caller.Caller, now time.Time) (int64, error) {
	var seq int64
	err := s.InTx(ctx, func(r app.Repository) error {
		tx := r.(*Store) //nolint:forcetypeassert // InTx always hands back a *Store
		sid := uuid.UUID(sess.ID)
		var err error
		if seq, err = tx.q.BumpSessionSeq(ctx, sid); err != nil {
			return err
		}
		if err := tx.write(ctx, sid, board, w, now); err != nil {
			return err
		}
		if board != nil {
			if err := tx.addReveals(ctx, board.Map.ID, w.AutoReveal); err != nil {
				return err
			}
		}
		actionID, err := tx.sessionAction(ctx, sess.CampaignID, sess.ID, w.Kind, actor, c, now)
		if err != nil {
			return err
		}
		return tx.logWrite(ctx, actionID, w)
	})
	return seq, err
}

//nolint:gosec // coordinates and ranges are bounded by the map
func (s *Store) write(ctx context.Context, sid uuid.UUID, board *domain.MapState, w live.Write, now time.Time) error {
	t := w.Token
	switch w.Kind {
	case domain.ActionTokenPlaced:
		return s.q.InsertToken(ctx, queries.InsertTokenParams{
			ID: uuid.UUID(t.ID), SessionID: sid, Label: t.Label, Kind: t.Kind, Q: int32(t.Q), R: int32(t.R), Hidden: t.Hidden, DarkvisionFt: int32(t.DarkvisionFt),
		})
	case domain.ActionTokenRemoved:
		return s.q.DeleteToken(ctx, queries.DeleteTokenParams{SessionID: sid, ID: uuid.UUID(t.ID)})
	case domain.ActionTokenMoved, domain.ActionTokenHidden, domain.ActionTokenRevealed:
		return s.q.UpdateToken(ctx, queries.UpdateTokenParams{SessionID: sid, ID: uuid.UUID(t.ID), Q: int32(t.Q), R: int32(t.R), Hidden: t.Hidden})
	case domain.ActionMapSet:
		p := queries.SetSessionMapParams{ID: sid}
		if w.MapID != nil {
			p.MapID = pgtype.UUID{Bytes: *w.MapID, Valid: true}
		}
		return s.q.SetSessionMap(ctx, p)
	default:
		return s.writeBoard(ctx, board, w, now)
	}
}

//nolint:gosec // coordinates and ranges are bounded by the map
func (s *Store) writeBoard(ctx context.Context, board *domain.MapState, w live.Write, now time.Time) error {
	switch w.Kind {
	case domain.ActionHexesRevealed:
		return s.addReveals(ctx, board.Map.ID, w.Hexes)
	case domain.ActionHexesConcealed:
		for _, c := range w.Hexes {
			if err := s.q.RemoveReveal(ctx, queries.RemoveRevealParams{MapID: uuid.UUID(board.Map.ID), Q: int32(c.Q), R: int32(c.R)}); err != nil {
				return err
			}
		}
		return nil
	case domain.ActionWallsSet, domain.ActionWallsCleared:
		for _, c := range w.Hexes {
			p := queries.AddWallParams{MapID: uuid.UUID(board.Map.ID), Q: int32(c.Q), R: int32(c.R)}
			err := s.q.AddWall(ctx, p)
			if w.Kind == domain.ActionWallsCleared {
				err = s.q.RemoveWall(ctx, queries.RemoveWallParams(p))
			}
			if err != nil {
				return err
			}
		}
		return nil
	case domain.ActionLightPlaced:
		l := w.Light
		return s.q.InsertLight(ctx, queries.InsertLightParams{
			ID: uuid.UUID(l.ID), MapID: uuid.UUID(board.Map.ID), Q: int32(l.At.Q), R: int32(l.At.R), BrightFt: int32(l.BrightFt), DimFt: int32(l.DimFt),
		})
	case domain.ActionLightRemoved:
		return s.q.DeleteLight(ctx, queries.DeleteLightParams{MapID: uuid.UUID(board.Map.ID), ID: uuid.UUID(w.Light.ID)})
	default:
		return s.q.SetMapAmbient(ctx, queries.SetMapAmbientParams{ID: uuid.UUID(board.Map.ID), Ambient: w.Ambient, Now: now})
	}
}

func (s *Store) addReveals(ctx context.Context, id domain.MapID, hs []hex.Coord) error {
	for _, c := range hs {
		if err := s.q.AddReveal(ctx, queries.AddRevealParams{MapID: uuid.UUID(id), Q: int32(c.Q), R: int32(c.R)}); err != nil { //nolint:gosec // map coordinates
			return err
		}
	}
	return nil
}

//nolint:gosec // coordinates are bounded by the map
func (s *Store) logWrite(ctx context.Context, actionID uuid.UUID, w live.Write) error {
	switch w.Kind {
	case domain.ActionTokenPlaced, domain.ActionTokenMoved, domain.ActionTokenHidden, domain.ActionTokenRevealed, domain.ActionTokenRemoved:
		t := w.Token
		return s.q.InsertTokenEvent(ctx, queries.InsertTokenEventParams{
			ActionID: actionID, TokenID: uuid.UUID(t.ID), Label: t.Label, Q: int32(t.Q), R: int32(t.R), Hidden: t.Hidden,
		})
	}
	hs := append(append([]hex.Coord{}, w.Hexes...), w.AutoReveal...)
	if w.Kind == domain.ActionLightPlaced || w.Kind == domain.ActionLightRemoved {
		hs = append(hs, w.Light.At)
	}
	for _, c := range hs {
		if err := s.q.InsertHexEvent(ctx, queries.InsertHexEventParams{ActionID: actionID, Q: int32(c.Q), R: int32(c.R)}); err != nil {
			return err
		}
	}
	return nil
}

// LoadMap reads a Map with its walls, lights and remembered hexes.
func (s *Store) LoadMap(ctx context.Context, campaign uuid.UUID, id domain.MapID) (*domain.MapState, error) {
	m, err := s.q.GetMap(ctx, queries.GetMapParams{CampaignID: campaign, ID: uuid.UUID(id)})
	if err != nil {
		return nil, notFound(err)
	}
	out := &domain.MapState{Map: mapRow(m), Walls: map[hex.Coord]bool{}, Lights: []domain.MapLight{}, Reveals: map[hex.Coord]bool{}}
	walls, err := s.q.MapWalls(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	for _, w := range walls {
		out.Walls[hex.Coord{Q: int(w.Q), R: int(w.R)}] = true
	}
	lights, err := s.q.MapLights(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	for _, l := range lights {
		out.Lights = append(out.Lights, domain.MapLight{ID: domain.LightID(l.ID), At: hex.Coord{Q: int(l.Q), R: int(l.R)}, BrightFt: int(l.BrightFt), DimFt: int(l.DimFt)})
	}
	reveals, err := s.q.MapReveals(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	for _, rv := range reveals {
		out.Reveals[hex.Coord{Q: int(rv.Q), R: int(rv.R)}] = true
	}
	return out, nil
}

func mapRow(m queries.CampaignMap) domain.Map {
	return domain.Map{
		ID: domain.MapID(m.ID), CampaignID: m.CampaignID, Name: m.Name, ImageKey: m.ImageKey, ImageType: m.ImageType,
		Width: int(m.WidthPx), Height: int(m.HeightPx), HexSize: m.HexSizePx, OriginX: m.OriginX, OriginY: m.OriginY,
		Ambient: m.Ambient, UpdatedAt: m.UpdatedAt,
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
