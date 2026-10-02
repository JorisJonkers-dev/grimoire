package app

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/social/changelog"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

// ReleaseRepository persists Release Notes and who has seen them.
type ReleaseRepository interface {
	InsertRelease(ctx context.Context, n domain.ReleaseNote, by string) error
	Releases(ctx context.Context) ([]domain.ReleaseNote, error)
	Release(ctx context.Context, id uuid.UUID) (domain.ReleaseNote, error)
	EditRelease(ctx context.Context, id uuid.UUID, title, body string, now time.Time) (bool, error)
	ScheduleRelease(ctx context.Context, id uuid.UUID, at *time.Time, now time.Time) (bool, error)
	DueReleases(ctx context.Context, now time.Time) ([]domain.ReleaseNote, error)
	AnnounceRelease(ctx context.Context, id uuid.UUID, now time.Time) error
	ActiveAccounts(ctx context.Context) ([]domain.AccountID, error)
	UnseenRelease(ctx context.Context, account domain.AccountID, now time.Time) (domain.ReleaseNote, error)
	SeeRelease(ctx context.Context, account domain.AccountID, id uuid.UUID, now time.Time) (bool, error)
}

// fullRelease is a version with no pre-release or build part; only those get a Release Note.
var fullRelease = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// DraftRelease starts the Release Note for a full release, listing the features the changelog says it
// added; each release gets one.
func (s *Service) DraftRelease(ctx context.Context, by, version string) (domain.ReleaseNote, error) {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if !fullRelease.MatchString(version) {
		return domain.ReleaseNote{}, domain.ErrInvalid
	}
	var lines []string
	if s.Changelog != nil {
		for _, f := range changelog.Features(s.Changelog(), version) {
			lines = append(lines, "- "+f)
		}
	}
	now := s.Now()
	n := domain.ReleaseNote{ID: uuid.New(), Version: version, Title: "What is new in Grimoire " + version, Body: strings.Join(lines, "\n"), CreatedAt: now, UpdatedAt: now, PublishAt: nil, AnnouncedAt: nil}
	return n, s.Repo.InsertRelease(ctx, n, by)
}

// Releases lists every Release Note, newest first.
func (s *Service) Releases(ctx context.Context) ([]domain.ReleaseNote, error) {
	return s.Repo.Releases(ctx)
}

// EditRelease changes a Release Note's words until it has been announced.
func (s *Service) EditRelease(ctx context.Context, id uuid.UUID, title, body string) (domain.ReleaseNote, error) {
	title, body = strings.TrimSpace(title), strings.TrimSpace(body)
	if title == "" || utf8.RuneCountInString(title) > 120 || utf8.RuneCountInString(body) > 8000 {
		return domain.ReleaseNote{}, domain.ErrInvalid
	}
	return s.changeRelease(ctx, id, func(now time.Time) (bool, error) { return s.Repo.EditRelease(ctx, id, title, body, now) })
}

// PublishRelease puts a Release Note live now, or at a later moment; one already announced stays as it
// is. It is announced in every Account's bell once live.
func (s *Service) PublishRelease(ctx context.Context, id uuid.UUID, at *time.Time) (domain.ReleaseNote, error) {
	var out domain.ReleaseNote
	err := s.Repo.InTx(ctx, func(r Repository) error {
		tx := *s
		tx.Repo = r
		if _, err := tx.changeRelease(ctx, id, func(now time.Time) (bool, error) {
			when := now
			if at != nil && at.After(now) {
				when = *at
			}
			return r.ScheduleRelease(ctx, id, &when, now)
		}); err != nil {
			return err
		}
		if err := tx.AnnounceReleases(ctx); err != nil {
			return err
		}
		var err error
		out, err = r.Release(ctx, id)
		return err
	})
	return out, err
}

func (s *Service) changeRelease(ctx context.Context, id uuid.UUID, change func(now time.Time) (bool, error)) (domain.ReleaseNote, error) {
	if _, err := s.Repo.Release(ctx, id); err != nil {
		return domain.ReleaseNote{}, err
	}
	changed, err := change(s.Now())
	if err != nil {
		return domain.ReleaseNote{}, err
	}
	if !changed {
		return domain.ReleaseNote{}, domain.ErrConflict
	}
	return s.Repo.Release(ctx, id)
}

// AnnounceReleases rings every Account's bell for each Release Note that went live and was not
// announced yet.
func (s *Service) AnnounceReleases(ctx context.Context) error {
	now := s.Now()
	due, err := s.Repo.DueReleases(ctx, now)
	if err != nil || len(due) == 0 {
		return err
	}
	accounts, err := s.Repo.ActiveAccounts(ctx)
	if err != nil {
		return err
	}
	for _, n := range due {
		for _, a := range accounts {
			if err := s.Notify(ctx, a, domain.Notice{Kind: domain.KindReleaseNote, Title: n.Title, Body: "", ActionLabel: "Open", ActionPath: "/", Dedupe: "release:" + n.ID.String(), Mail: ""}); err != nil {
				return err
			}
		}
		if err := s.Repo.AnnounceRelease(ctx, n.ID, now); err != nil {
			return err
		}
	}
	return nil
}

// UnseenRelease is the newest live Release Note the caller has not seen yet, if any.
func (s *Service) UnseenRelease(ctx context.Context, subject string) (*domain.ReleaseNote, error) {
	me, err := s.me(ctx, subject)
	if err != nil {
		return nil, err
	}
	n, err := s.Repo.UnseenRelease(ctx, me.ID, s.Now())
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil //nolint:nilnil // nothing new is not an error
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// SeeRelease records that the caller saw a live Release Note, so it does not show again.
func (s *Service) SeeRelease(ctx context.Context, subject string, id uuid.UUID) error {
	me, err := s.me(ctx, subject)
	if err != nil {
		return err
	}
	if _, err := s.Repo.Release(ctx, id); err != nil {
		return err
	}
	if _, err := s.Repo.SeeRelease(ctx, me.ID, id, s.Now()); err != nil {
		return err
	}
	return nil
}
