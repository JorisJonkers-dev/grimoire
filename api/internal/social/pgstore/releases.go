package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

func when(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func release(id uuid.UUID, version, title, body string, created, updated time.Time, publish, announced pgtype.Timestamptz) domain.ReleaseNote {
	return domain.ReleaseNote{ID: id, Version: version, Title: title, Body: body, CreatedAt: created, UpdatedAt: updated, PublishAt: when(publish), AnnouncedAt: when(announced)}
}

// InsertRelease stores a new draft; a version that already has one is ErrConflict.
func (s *Store) InsertRelease(ctx context.Context, n domain.ReleaseNote, by string) error {
	err := s.q.InsertReleaseNote(ctx, queries.InsertReleaseNoteParams{ID: n.ID, Version: n.Version, Title: n.Title, Body: n.Body, CreatedBy: by, Now: n.CreatedAt})
	return conflictOf(err)
}

// Releases lists Release Notes, newest first.
func (s *Store) Releases(ctx context.Context) ([]domain.ReleaseNote, error) {
	rows, err := s.q.ListReleaseNotes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ReleaseNote, 0, len(rows))
	for _, r := range rows {
		out = append(out, release(r.ID, r.Version, r.Title, r.Body, r.CreatedAt, r.UpdatedAt, r.PublishAt, r.AnnouncedAt))
	}
	return out, nil
}

// Release reads one Release Note.
func (s *Store) Release(ctx context.Context, id uuid.UUID) (domain.ReleaseNote, error) {
	r, err := s.q.GetReleaseNote(ctx, id)
	if err != nil {
		return domain.ReleaseNote{}, notFound(err)
	}
	return release(r.ID, r.Version, r.Title, r.Body, r.CreatedAt, r.UpdatedAt, r.PublishAt, r.AnnouncedAt), nil
}

// EditRelease changes an unannounced Release Note's words; false once announced.
func (s *Store) EditRelease(ctx context.Context, id uuid.UUID, title, body string, now time.Time) (bool, error) {
	n, err := s.q.UpdateReleaseNote(ctx, queries.UpdateReleaseNoteParams{Title: title, Body: body, Now: now, ID: id})
	return n == 1, err
}

// ScheduleRelease sets when an unannounced Release Note goes live; false once announced.
func (s *Store) ScheduleRelease(ctx context.Context, id uuid.UUID, at *time.Time, now time.Time) (bool, error) {
	p := queries.ScheduleReleaseNoteParams{PublishAt: pgtype.Timestamptz{}, Now: now, ID: id}
	if at != nil {
		p.PublishAt = pgtype.Timestamptz{Time: *at, Valid: true}
	}
	n, err := s.q.ScheduleReleaseNote(ctx, p)
	return n == 1, err
}

// DueReleases lists live Release Notes not announced yet.
func (s *Store) DueReleases(ctx context.Context, now time.Time) ([]domain.ReleaseNote, error) {
	rows, err := s.q.DueReleaseNotes(ctx, pgtype.Timestamptz{Time: now, Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]domain.ReleaseNote, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.ReleaseNote{ID: r.ID, Version: r.Version, Title: r.Title})
	}
	return out, nil
}

// AnnounceRelease records that a Release Note rang every bell.
func (s *Store) AnnounceRelease(ctx context.Context, id uuid.UUID, now time.Time) error {
	return s.q.AnnounceReleaseNote(ctx, queries.AnnounceReleaseNoteParams{Now: pgtype.Timestamptz{Time: now, Valid: true}, ID: id})
}

// ActiveAccounts lists every Account not disabled.
func (s *Store) ActiveAccounts(ctx context.Context) ([]domain.AccountID, error) {
	return s.q.ActiveAccounts(ctx)
}

// UnseenRelease finds the newest live Release Note an Account has not seen.
func (s *Store) UnseenRelease(ctx context.Context, account domain.AccountID, now time.Time) (domain.ReleaseNote, error) {
	r, err := s.q.UnseenReleaseNote(ctx, queries.UnseenReleaseNoteParams{Now: pgtype.Timestamptz{Time: now, Valid: true}, AccountID: account})
	if err != nil {
		return domain.ReleaseNote{}, notFound(err)
	}
	return release(r.ID, r.Version, r.Title, r.Body, r.CreatedAt, r.UpdatedAt, r.PublishAt, r.AnnouncedAt), nil
}

// SeeRelease records that an Account saw a live Release Note; false when it is not live.
func (s *Store) SeeRelease(ctx context.Context, account domain.AccountID, id uuid.UUID, now time.Time) (bool, error) {
	n, err := s.q.SeeReleaseNote(ctx, queries.SeeReleaseNoteParams{AccountID: account, Now: now, ID: id})
	return n == 1, err
}

// conflictOf reports a unique constraint as ErrConflict.
func conflictOf(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return domain.ErrConflict
	}
	return err
}
