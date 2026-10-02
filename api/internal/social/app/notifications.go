package app

import (
	"context"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

// NotificationRepository persists Notifications and the channels each kind reaches.
type NotificationRepository interface {
	UpsertNotification(ctx context.Context, account domain.AccountID, id uuid.UUID, n domain.Notice, now time.Time) error
	Notifications(ctx context.Context, account domain.AccountID) ([]domain.Notification, int, error)
	ReadNotification(ctx context.Context, account domain.AccountID, id uuid.UUID, now time.Time) (bool, error)
	ReadAllNotifications(ctx context.Context, account domain.AccountID, now time.Time) error
	Preferences(ctx context.Context, account domain.AccountID) (map[[2]string]bool, error)
	SetPreference(ctx context.Context, account domain.AccountID, kind, channel string, enabled bool) error
}

// Enabled reports whether a kind reaches an Account on a channel, by its choice or the default.
func (s *Service) Enabled(ctx context.Context, account domain.AccountID, kind, channel string) (bool, error) {
	return enabled(ctx, s.Repo, account, kind, channel)
}

func enabled(ctx context.Context, r Repository, account domain.AccountID, kind, channel string) (bool, error) {
	prefs, err := r.Preferences(ctx, account)
	if err != nil {
		return false, err
	}
	if on, chosen := prefs[[2]string{kind, channel}]; chosen {
		return on, nil
	}
	return domain.Default(kind, channel), nil
}

// Notify puts a Notice in an Account's bell, unless it turned that kind off in app.
func (s *Service) Notify(ctx context.Context, account domain.AccountID, n domain.Notice) error {
	return s.notify(ctx, s.Repo, account, n)
}

func (s *Service) notify(ctx context.Context, r Repository, account domain.AccountID, n domain.Notice) error {
	on, err := enabled(ctx, r, account, n.Kind, domain.ChannelInApp)
	if err != nil || !on {
		return err
	}
	n.Title, n.Body = cut(n.Title, 120), cut(n.Body, 200)
	return r.UpsertNotification(ctx, account, uuid.New(), n, s.Now())
}

// cut keeps the first runes of a text that fit.
func cut(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	return string([]rune(s)[:limit-1]) + "…"
}

// Notifications reads the caller's latest Notifications and how many are unread.
func (s *Service) Notifications(ctx context.Context, subject string) ([]domain.Notification, int, error) {
	me, err := s.me(ctx, subject)
	if err != nil {
		return nil, 0, err
	}
	return s.Repo.Notifications(ctx, me.ID)
}

// ReadNotification marks one of the caller's Notifications read.
func (s *Service) ReadNotification(ctx context.Context, subject string, id uuid.UUID) error {
	me, err := s.me(ctx, subject)
	if err != nil {
		return err
	}
	done, err := s.Repo.ReadNotification(ctx, me.ID, id, s.Now())
	if err != nil {
		return err
	}
	if !done {
		return domain.ErrNotFound
	}
	return nil
}

// ReadAll marks every Notification of the caller's read.
func (s *Service) ReadAll(ctx context.Context, subject string) error {
	me, err := s.me(ctx, subject)
	if err != nil {
		return err
	}
	return s.Repo.ReadAllNotifications(ctx, me.ID, s.Now())
}

// Preferences reads which kinds reach the caller on which channel, defaults filled in.
func (s *Service) Preferences(ctx context.Context, subject string) ([]domain.Preference, error) {
	me, err := s.me(ctx, subject)
	if err != nil {
		return nil, err
	}
	prefs, err := s.Repo.Preferences(ctx, me.ID)
	if err != nil {
		return nil, err
	}
	pick := func(kind, channel string) bool {
		if on, chosen := prefs[[2]string{kind, channel}]; chosen {
			return on
		}
		return domain.Default(kind, channel)
	}
	out := make([]domain.Preference, 0, len(domain.Kinds()))
	for _, k := range domain.Kinds() {
		out = append(out, domain.Preference{Kind: k, InApp: pick(k, domain.ChannelInApp), Push: pick(k, domain.ChannelPush), Email: pick(k, domain.ChannelEmail)})
	}
	return out, nil
}

// SetPreferences stores the caller's choice for each kind given; security always shows in app.
func (s *Service) SetPreferences(ctx context.Context, subject string, in []domain.Preference) ([]domain.Preference, error) {
	me, err := s.me(ctx, subject)
	if err != nil {
		return nil, err
	}
	for _, p := range in {
		if !slices.Contains(domain.Kinds(), p.Kind) || (p.Kind == domain.KindSecurity && !p.InApp) {
			return nil, domain.ErrInvalid
		}
	}
	err = s.Repo.InTx(ctx, func(r Repository) error {
		for _, p := range in {
			if err := setPreference(ctx, r, me.ID, p); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Preferences(ctx, subject)
}

// Alert tells an Account's holder about a change to how it signs in.
func (s *Service) Alert(ctx context.Context, account domain.AccountID, title string) error {
	return s.Notify(ctx, account, domain.Notice{Kind: domain.KindSecurity, Title: title, Body: "If this was not you, change your password and tell an Admin.", ActionLabel: "Open", ActionPath: "/account", Dedupe: ""})
}

func setPreference(ctx context.Context, r Repository, account domain.AccountID, p domain.Preference) error {
	for channel, on := range map[string]bool{domain.ChannelInApp: p.InApp, domain.ChannelPush: p.Push, domain.ChannelEmail: p.Email} {
		if err := r.SetPreference(ctx, account, p.Kind, channel, on); err != nil {
			return err
		}
	}
	return nil
}
