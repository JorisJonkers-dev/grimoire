package pgstore

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
)

// saveTable writes the Table Display after a change to it.
//
//nolint:gosec // coordinates and zoom are bounded by the API
func (s *Store) saveTable(ctx context.Context, sid uuid.UUID, t *domain.TableDisplay) error {
	if t == nil {
		return nil
	}
	p := queries.SaveTableParams{
		SessionID: sid, Camera: t.Camera, Q: int32(t.Q), R: int32(t.R), ZoomPct: int32(t.ZoomPct), Scene: t.Scene, Title: t.Title, Body: t.Body,
		Blackout: t.Blackout,
	}
	if t.MapID != nil {
		p.MapID = pgtype.UUID{Bytes: *t.MapID, Valid: true}
	}
	return s.q.SaveTable(ctx, p)
}

// LoadTable reads a Session's Table Display, or the default when the DM never touched it.
func (s *Store) LoadTable(ctx context.Context, id domain.SessionID) (domain.TableDisplay, error) {
	r, err := s.q.SessionTable(ctx, uuid.UUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DefaultTable(), nil
	}
	if err != nil {
		return domain.TableDisplay{}, err
	}
	t := domain.TableDisplay{
		Camera: r.Camera, Q: int(r.Q), R: int(r.R), ZoomPct: int(r.ZoomPct), Scene: r.Scene, Title: r.Title, Body: r.Body, Blackout: r.Blackout,
	}
	if r.MapID.Valid {
		id := domain.MapID(r.MapID.Bytes)
		t.MapID = &id
	}
	return t, nil
}
