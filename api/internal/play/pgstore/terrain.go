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
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
)

// saveTerrain writes what a change did to Surfaces, the waiting area spell and the Map's elevation.
//
//nolint:gosec // coordinates and heights are bounded by the rules
func (s *Store) saveTerrain(ctx context.Context, sid uuid.UUID, board *domain.MapState, w live.Write) error {
	if w.Kind == domain.ActionElevationSet {
		if err := s.saveElevation(ctx, board, w); err != nil {
			return err
		}
	}
	if w.SaveSurfaces {
		if err := s.saveSurfaces(ctx, sid, w.Surfaces); err != nil {
			return err
		}
	}
	if !w.SaveCast {
		return nil
	}
	if err := s.q.ClearCasts(ctx, sid); err != nil {
		return err
	}
	if w.Cast == nil {
		return nil
	}
	return s.insertCast(ctx, sid, w.Cast)
}

//nolint:gosec // coordinates and rounds are bounded by the rules
func (s *Store) saveSurfaces(ctx context.Context, sid uuid.UUID, ground map[hex.Coord]domain.Surface) error {
	if err := s.q.ClearSurfaces(ctx, sid); err != nil {
		return err
	}
	for c, sf := range ground {
		p := queries.InsertSurfaceParams{SessionID: sid, Q: int32(c.Q), R: int32(c.R), Kind: string(sf.Kind)}
		if sf.RoundsLeft > 0 {
			p.RoundsLeft = pgInt(sf.RoundsLeft)
		}
		if err := s.q.InsertSurface(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

//nolint:gosec // coordinates and DCs are bounded by the rules
func (s *Store) insertCast(ctx context.Context, sid uuid.UUID, c *domain.AreaCast) error {
	p := queries.InsertCastParams{ID: c.ID, SessionID: sid, CasterTokenID: uuid.UUID(c.Caster), Spell: c.Spell, Dc: int32(c.DC)}
	if c.DamageRoll != nil {
		p.DamageRollID = pgtype.UUID{Bytes: *c.DamageRoll, Valid: true}
	}
	if err := s.q.InsertCast(ctx, p); err != nil {
		return err
	}
	for _, h := range c.Hexes {
		if err := s.q.InsertCastHex(ctx, queries.InsertCastHexParams{CastID: c.ID, Q: int32(h.Q), R: int32(h.R)}); err != nil {
			return err
		}
	}
	for _, t := range c.Targets {
		tp := queries.InsertCastTargetParams{CastID: c.ID, TokenID: uuid.UUID(t.Token)}
		if t.SaveRoll != nil {
			tp.SaveRollID = pgtype.UUID{Bytes: *t.SaveRoll, Valid: true}
		}
		if err := s.q.InsertCastTarget(ctx, tp); err != nil {
			return err
		}
	}
	return nil
}

// LoadTerrain reads a Session's Surfaces and the area spell waiting on its rolls.
func (s *Store) LoadTerrain(ctx context.Context, id domain.SessionID) (map[hex.Coord]domain.Surface, *domain.AreaCast, error) {
	sid := uuid.UUID(id)
	rows, err := s.q.SessionSurfaces(ctx, sid)
	if err != nil {
		return nil, nil, err
	}
	ground := map[hex.Coord]domain.Surface{}
	for _, r := range rows {
		ground[hex.Coord{Q: int(r.Q), R: int(r.R)}] = domain.Surface{Kind: surface.Kind(r.Kind), RoundsLeft: int(r.RoundsLeft.Int32)}
	}
	c, err := s.q.SessionCast(ctx, sid)
	if errors.Is(err, pgx.ErrNoRows) {
		return ground, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	cast := &domain.AreaCast{ID: c.ID, Caster: domain.TokenID(c.CasterTokenID), Spell: c.Spell, DC: int(c.Dc)}
	if c.DamageRollID.Valid {
		roll := domain.RollID(c.DamageRollID.Bytes)
		cast.DamageRoll = &roll
	}
	hexes, err := s.q.CastHexes(ctx, c.ID)
	if err != nil {
		return nil, nil, err
	}
	for _, h := range hexes {
		cast.Hexes = append(cast.Hexes, hex.Coord{Q: int(h.Q), R: int(h.R)})
	}
	targets, err := s.q.CastTargets(ctx, c.ID)
	if err != nil {
		return nil, nil, err
	}
	for _, t := range targets {
		target := domain.AreaTarget{Token: domain.TokenID(t.TokenID)}
		if t.SaveRollID.Valid {
			roll := domain.RollID(t.SaveRollID.Bytes)
			target.SaveRoll = &roll
		}
		cast.Targets = append(cast.Targets, target)
	}
	return ground, cast, nil
}

// HighGround reads whether a Campaign uses the high-ground optional rule.
func (s *Store) HighGround(ctx context.Context, campaign uuid.UUID) (bool, error) {
	return s.q.CampaignHighGround(ctx, campaign)
}

//nolint:gosec // coordinates and heights are bounded by the rules
func (s *Store) saveElevation(ctx context.Context, board *domain.MapState, w live.Write) error {
	for _, c := range w.Hexes {
		p := queries.SetElevationParams{MapID: uuid.UUID(board.Map.ID), Q: int32(c.Q), R: int32(c.R), ElevationFt: int32(w.ElevationFt)}
		var err error
		if w.ElevationFt == 0 {
			err = s.q.ClearElevation(ctx, queries.ClearElevationParams{MapID: p.MapID, Q: p.Q, R: p.R})
		} else {
			err = s.q.SetElevation(ctx, p)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
