package pgstore

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Groups lists the live Sessions a party is split over, the one it split from first, with the party
// tokens in each. A party that is together is one group.
func (s *Store) Groups(ctx context.Context, sess domain.Session) ([]domain.PartyGroup, error) {
	root := uuid.UUID(sess.Root())
	rows, err := s.q.PartyGroups(ctx, root)
	if err != nil {
		return nil, err
	}
	tokens, err := s.q.PartyGroupTokens(ctx, root)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PartyGroup, 0, len(rows))
	for _, r := range rows {
		g := domain.PartyGroup{Session: domain.SessionID(r.ID), Number: int(r.Number), Name: r.GroupName, Home: r.Home, Table: r.Shown, Tokens: []domain.GroupToken{}}
		for _, t := range tokens {
			if t.SessionID != r.ID {
				continue
			}
			gt := domain.GroupToken{ID: domain.TokenID(t.ID), Label: t.Label, Controller: nil}
			if t.ControllerMemberID.Valid {
				id := uuid.UUID(t.ControllerMemberID.Bytes)
				gt.Controller = &id
			}
			g.Tokens = append(g.Tokens, gt)
		}
		out = append(out, g)
	}
	return out, nil
}

// Place is the Session of a party's groups a member belongs in. The DM belongs wherever they look. The
// Table Display belongs with the group the DM has it follow. A Player belongs with a group their own
// party token is in, and with the Session the party split from when they play none: each group's
// Party Vision is its own. Everyone belongs in that Session once a group's own Session has ended.
func (s *Store) Place(ctx context.Context, id domain.SessionID, member uuid.UUID, dm, table bool) (domain.SessionID, error) {
	row, err := s.q.SessionByID(ctx, uuid.UUID(id))
	if err != nil {
		return id, notFound(err)
	}
	sess := session(row)
	root := sess.Root()
	switch {
	case sess.Status != domain.SessionLive:
		return root, nil
	case dm:
		return id, nil
	case table:
		at, err := s.q.SessionPlace(ctx, uuid.UUID(root))
		return domain.SessionID(at), err
	}
	mine, err := s.q.MemberGroups(ctx, queries.MemberGroupsParams{Root: uuid.UUID(root), Member: pgtype.UUID{Bytes: member, Valid: true}})
	switch {
	case err != nil:
		return id, err
	case slices.Contains(mine, uuid.UUID(id)):
		return id, nil
	case len(mine) > 0:
		return domain.SessionID(mine[0]), nil
	}
	return root, nil
}

// moveTokens takes tokens from one Session to another, with the Effects on them, the saves those
// Effects wait on, and their death saves.
//
//nolint:gosec // coordinates are bounded by the map
func (s *Store) moveTokens(ctx context.Context, from, to uuid.UUID, places []domain.TokenPlace) error {
	ids := make([]uuid.UUID, 0, len(places))
	for _, p := range places {
		n, err := s.q.MoveToken(ctx, queries.MoveTokenParams{ToSession: to, Q: int32(p.Q), R: int32(p.R), FromSession: from, ID: uuid.UUID(p.ID)})
		if err != nil {
			return err
		}
		if n == 0 {
			return apperr.ErrNotFound
		}
		ids = append(ids, uuid.UUID(p.ID))
	}
	if err := s.q.MoveTokenEffects(ctx, queries.MoveTokenEffectsParams{ToSession: to, FromSession: from, Ids: ids}); err != nil {
		return err
	}
	if err := s.q.MoveEffectSaves(ctx, queries.MoveEffectSavesParams{ToSession: to, FromSession: from, Ids: ids}); err != nil {
		return err
	}
	return s.q.MoveTokenDying(ctx, queries.MoveTokenDyingParams{ToSession: to, FromSession: from, Ids: ids})
}

// openGroup opens the Campaign's next Session as a group's own, on the map the group goes to.
func (s *Store) openGroup(ctx context.Context, sess domain.Session, name string, to domain.MapID, now time.Time) (domain.Session, error) {
	if err := s.q.LockCampaignLog(ctx, "sessions:"+sess.CampaignID.String()); err != nil {
		return domain.Session{}, err
	}
	n, err := s.q.NextSessionNumber(ctx, sess.CampaignID)
	if err != nil {
		return domain.Session{}, err
	}
	row, err := s.q.InsertGroupSession(ctx, queries.InsertGroupSessionParams{
		CampaignID: sess.CampaignID, Number: n, Now: now, ParentSessionID: pgtype.UUID{Bytes: sess.Root(), Valid: true}, GroupName: name,
		MapID: pgtype.UUID{Bytes: to, Valid: true},
	})
	if err != nil {
		return domain.Session{}, err
	}
	return session(row), nil
}

// closeGroup ends a group's own Session, takes the Table Display back to the party when it followed
// that group, and lets the party's Checkpoints go: they keep tokens that have since changed Sessions.
func (s *Store) closeGroup(ctx context.Context, sess domain.Session, group domain.SessionID, actor domain.Member, cl caller.Caller, now time.Time) error {
	n, err := s.q.EndSession(ctx, queries.EndSessionParams{CampaignID: sess.CampaignID, ID: uuid.UUID(group), Now: pgtypeTime(now)})
	if err != nil {
		return err
	}
	if n == 0 {
		return apperr.ErrNotFound
	}
	if sess.Table != nil && *sess.Table == group {
		if err := s.q.SetTableSession(ctx, queries.SetTableSessionParams{ID: uuid.UUID(sess.ID), TableSessionID: pgtype.UUID{}}); err != nil {
			return err
		}
	}
	if err := s.q.DropGroupCheckpoints(ctx, uuid.UUID(sess.ID)); err != nil {
		return err
	}
	_, err = s.sessionAction(ctx, sess.CampaignID, group, domain.ActionSessionEnded, actor, cl, now)
	return err
}

// SplitParty opens a Session of its own for a group that leaves the party, on the map it goes to, and
// takes the group's tokens there. read is handed the store of the same transaction; when it fails,
// nothing of the split stays. Checkpoints of the Session go: they keep tokens that are no longer its.
func (s *Store) SplitParty(ctx context.Context, sess domain.Session, name string, to domain.MapID, places []domain.TokenPlace, actor domain.Member, cl caller.Caller, now time.Time, read func(live.Store) error) (domain.Session, live.Committed, error) {
	var side domain.Session
	var done live.Committed
	err := s.InTx(ctx, func(r app.Repository) error {
		tx := r.(*Store) //nolint:forcetypeassert // InTx always hands back a *Store
		var err error
		if done.Seq, err = tx.q.BumpSessionSeq(ctx, uuid.UUID(sess.ID)); err != nil {
			return err
		}
		if side, err = tx.openGroup(ctx, sess, name, to, now); err != nil {
			return err
		}
		if err := tx.moveTokens(ctx, uuid.UUID(sess.ID), uuid.UUID(side.ID), places); err != nil {
			return err
		}
		if err := tx.q.DropGroupCheckpoints(ctx, uuid.UUID(sess.Root())); err != nil {
			return err
		}
		if _, err := tx.sessionAction(ctx, sess.CampaignID, side.ID, domain.ActionSessionStarted, actor, cl, now); err != nil {
			return err
		}
		if _, done.Action, err = tx.loggedAction(ctx, sess.CampaignID, sess.ID, domain.ActionPartySplit, actor, cl, now); err != nil {
			return err
		}
		return read(tx)
	})
	return side, done, err
}

// RejoinParty brings a group's party tokens back to the Session the party split from and ends the
// group's own Session. The Table Display goes back with them when it followed that group.
func (s *Store) RejoinParty(ctx context.Context, sess domain.Session, group domain.SessionID, places []domain.TokenPlace, actor domain.Member, cl caller.Caller, now time.Time, read func(live.Store) error) (live.Committed, error) {
	var done live.Committed
	err := s.InTx(ctx, func(r app.Repository) error {
		tx := r.(*Store) //nolint:forcetypeassert // InTx always hands back a *Store
		var err error
		if done.Seq, err = tx.q.BumpSessionSeq(ctx, uuid.UUID(sess.ID)); err != nil {
			return err
		}
		if err := tx.moveTokens(ctx, uuid.UUID(group), uuid.UUID(sess.ID), places); err != nil {
			return err
		}
		if err := tx.closeGroup(ctx, sess, group, actor, cl, now); err != nil {
			return err
		}
		if _, done.Action, err = tx.loggedAction(ctx, sess.CampaignID, sess.ID, domain.ActionPartyRejoined, actor, cl, now); err != nil {
			return err
		}
		return read(tx)
	})
	return done, err
}

// FollowTable has the Table Display follow one of the party's groups; nil follows the Session the
// party split from.
func (s *Store) FollowTable(ctx context.Context, sess domain.Session, group *domain.SessionID, actor domain.Member, cl caller.Caller, now time.Time) (live.Committed, error) {
	var done live.Committed
	err := s.InTx(ctx, func(r app.Repository) error {
		tx := r.(*Store) //nolint:forcetypeassert // InTx always hands back a *Store
		var err error
		if done.Seq, err = tx.q.BumpSessionSeq(ctx, uuid.UUID(sess.ID)); err != nil {
			return err
		}
		to := pgtype.UUID{}
		if group != nil {
			to = pgtype.UUID{Bytes: *group, Valid: true}
		}
		if err := tx.q.SetTableSession(ctx, queries.SetTableSessionParams{ID: uuid.UUID(sess.ID), TableSessionID: to}); err != nil {
			return err
		}
		_, done.Action, err = tx.loggedAction(ctx, sess.CampaignID, sess.ID, domain.ActionTableSet, actor, cl, now)
		return err
	})
	return done, err
}
