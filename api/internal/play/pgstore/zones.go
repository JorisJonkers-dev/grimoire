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
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// LoadZones reads a Session's Encounter Zones with who each held and who noticed it.
func (s *Store) LoadZones(ctx context.Context, id domain.SessionID) ([]domain.Zone, error) {
	sid := uuid.UUID(id)
	rows, err := s.q.SessionZones(ctx, sid)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Zone, 0, len(rows))
	for _, z := range rows {
		out = append(out, domain.Zone{
			ID: domain.ZoneID(z.ID), Name: z.Name, At: hex.Coord{Q: int(z.Q), R: int(z.R)}, RadiusHexes: int(z.RadiusHexes), DMOnly: z.DmOnly,
			Held: z.Held, Status: z.Status, DC: int(z.Dc), Checks: []domain.ZoneCheck{},
		})
	}
	find := func(id uuid.UUID) *domain.Zone {
		return &out[slices.IndexFunc(out, func(z domain.Zone) bool { return uuid.UUID(z.ID) == id })]
	}
	creatures, err := s.q.SessionZoneCreatures(ctx, sid)
	if err != nil {
		return nil, err
	}
	for _, c := range creatures {
		z := find(c.ZoneID)
		z.Creatures = append(z.Creatures, domain.TokenID(c.TokenID))
	}
	checks, err := s.q.SessionZoneChecks(ctx, sid)
	if err != nil {
		return nil, err
	}
	for _, c := range checks {
		check := domain.ZoneCheck{Token: domain.TokenID(c.TokenID)}
		if c.RollID.Valid {
			id := domain.RollID(c.RollID.Bytes)
			check.RollID = &id
		}
		if c.Noticed.Valid {
			yes := c.Noticed.Bool
			check.Noticed = &yes
		}
		z := find(c.ZoneID)
		z.Checks = append(z.Checks, check)
	}
	return out, nil
}

// saveZone writes the zone a change touched, the Perception rolls it opened and the creatures it revealed.
func (s *Store) saveZone(ctx context.Context, sess domain.Session, w live.Write, actor domain.Member, c caller.Caller, now time.Time) error {
	if err := s.reveal(ctx, sess.ID, w.Revealed); err != nil {
		return err
	}
	z := w.Zone
	switch {
	case z == nil:
		return nil
	case w.Kind == domain.ActionZoneRemoved:
		return s.q.DeleteZone(ctx, queries.DeleteZoneParams{SessionID: uuid.UUID(sess.ID), ID: uuid.UUID(z.ID)})
	case w.Combat == nil:
		if err := s.openRolls(ctx, sess, w.Rolls, actor, c, now); err != nil {
			return err
		}
	}
	return s.writeZone(ctx, sess.ID, z)
}

//nolint:gosec // coordinates are bounded by the map
func (s *Store) reveal(ctx context.Context, sid domain.SessionID, ts []domain.Token) error {
	for _, t := range ts {
		if err := s.q.UpdateToken(ctx, queries.UpdateTokenParams{SessionID: uuid.UUID(sid), ID: uuid.UUID(t.ID), Q: int32(t.Q), R: int32(t.R), Hidden: t.Hidden}); err != nil {
			return err
		}
	}
	return nil
}

// openRolls opens Roll Requests outside a Combat.
func (s *Store) openRolls(ctx context.Context, sess domain.Session, rolls []domain.Roll, actor domain.Member, c caller.Caller, now time.Time) error {
	for _, r := range rolls {
		if _, err := s.InsertRoll(ctx, r, now); err != nil {
			return err
		}
		if err := s.Append(ctx, sess.CampaignID, app.LogEntry{Kind: domain.ActionRollRequested, Actor: actor, Caller: c, RollID: r.ID, At: now}); err != nil {
			return err
		}
	}
	return nil
}

//nolint:gosec // coordinates, radius and DC are bounded by the rules
func (s *Store) writeZone(ctx context.Context, sid domain.SessionID, z *domain.Zone) error {
	if err := s.q.SaveZone(ctx, queries.SaveZoneParams{
		ID: uuid.UUID(z.ID), SessionID: uuid.UUID(sid), Name: z.Name, Q: int32(z.At.Q), R: int32(z.At.R), RadiusHexes: int32(z.RadiusHexes),
		DmOnly: z.DMOnly, Held: z.Held, Status: z.Status, Dc: int32(z.DC),
	}); err != nil {
		return err
	}
	for _, t := range z.Creatures {
		if err := s.q.AddZoneCreature(ctx, queries.AddZoneCreatureParams{ZoneID: uuid.UUID(z.ID), TokenID: uuid.UUID(t)}); err != nil {
			return err
		}
	}
	for _, ch := range z.Checks {
		p := queries.SaveZoneCheckParams{ZoneID: uuid.UUID(z.ID), TokenID: uuid.UUID(ch.Token)}
		if ch.RollID != nil {
			p.RollID = pgtype.UUID{Bytes: *ch.RollID, Valid: true}
		}
		if ch.Noticed != nil {
			p.Noticed = pgtype.Bool{Bool: *ch.Noticed, Valid: true}
		}
		if err := s.q.SaveZoneCheck(ctx, p); err != nil {
			return err
		}
	}
	return nil
}
