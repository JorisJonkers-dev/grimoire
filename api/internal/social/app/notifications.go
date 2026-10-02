package app

import (
	"context"
	"errors"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/mail/letters"
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
	Recipient(ctx context.Context, account domain.AccountID) (domain.Recipient, error)
	QueueEmail(ctx context.Context, account domain.AccountID, id uuid.UUID, n domain.Notice, now time.Time) error
	DueDigests(ctx context.Context, cutoff time.Time) ([]domain.AccountID, error)
	TakeQueued(ctx context.Context, account domain.AccountID) ([]domain.Queued, error)
	MarkDigest(ctx context.Context, account domain.AccountID, now time.Time) error
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

// NotifySubject rings the bell of the Account a subject signs in as; a subject without an Account has
// no bell, so nothing rings.
func (s *Service) NotifySubject(ctx context.Context, subject string, n domain.Notice) error {
	p, err := s.Repo.AccountBySubject(ctx, subject)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.Notify(ctx, p.ID, n)
}

func (s *Service) notify(ctx context.Context, r Repository, account domain.AccountID, n domain.Notice) error {
	prefs, err := r.Preferences(ctx, account)
	if err != nil {
		return err
	}
	on := func(channel string) bool {
		if v, chosen := prefs[[2]string{n.Kind, channel}]; chosen {
			return v
		}
		return domain.Default(n.Kind, channel)
	}
	n.Title, n.Body = cut(n.Title, 120), cut(n.Body, 200)
	now := s.Now()
	if on(domain.ChannelInApp) {
		if err := r.UpsertNotification(ctx, account, uuid.New(), n, now); err != nil {
			return err
		}
	}
	return s.beyondTheBell(ctx, r, account, n, on(domain.ChannelPush) && s.Devices != nil, on(domain.ChannelEmail) && s.Mailer != nil, now)
}

// beyondTheBell pushes a Notice to the Account's devices and mails it: security at once, the rest
// into the next Digest.
func (s *Service) beyondTheBell(ctx context.Context, r Repository, account domain.AccountID, n domain.Notice, push, email bool, now time.Time) error {
	if !push && !email {
		return nil
	}
	to, err := r.Recipient(ctx, account)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if push {
		s.Devices.Push(to.Subject, n.Title, n.Body, s.link(n.ActionPath))
	}
	if !email || to.Email == "" {
		return nil
	}
	if n.Kind == domain.KindSecurity {
		return s.mail(ctx, to, letter(to, n, s.link(n.ActionPath)))
	}
	if n.Mail == "" {
		n.Mail = string(mailKinds[n.Kind])
	}
	return r.QueueEmail(ctx, account, uuid.New(), n, now)
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

// Alert tells an Account's holder about a change to how it signs in; detail says from where, for a
// new sign-in.
func (s *Service) Alert(ctx context.Context, account domain.AccountID, event, title, detail string) error {
	body := "If this was not you, change your password and tell an Admin."
	if detail != "" {
		body = "From " + detail + ". " + body
	}
	mail := map[string]string{"two_step_reset": string(letters.TwoStepReset), "disabled": string(letters.AccountDisabled)}[event]
	if mail == "" {
		mail = string(letters.NewSignIn)
	}
	return s.Notify(ctx, account, domain.Notice{Kind: domain.KindSecurity, Title: title, Body: body, ActionLabel: "Open", ActionPath: "/account", Dedupe: "", Mail: mail})
}

func setPreference(ctx context.Context, r Repository, account domain.AccountID, p domain.Preference) error {
	for channel, on := range map[string]bool{domain.ChannelInApp: p.InApp, domain.ChannelPush: p.Push, domain.ChannelEmail: p.Email} {
		if err := r.SetPreference(ctx, account, p.Kind, channel, on); err != nil {
			return err
		}
	}
	return nil
}
