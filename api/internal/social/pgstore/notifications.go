package pgstore

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

// UpsertNotification stores a Notice, replacing an unread one with the same dedupe key.
func (s *Store) UpsertNotification(ctx context.Context, account domain.AccountID, id uuid.UUID, n domain.Notice, now time.Time) error {
	return s.q.UpsertNotification(ctx, queries.UpsertNotificationParams{
		ID: id, AccountID: account, Kind: n.Kind, Title: n.Title, Body: n.Body, ActionLabel: n.ActionLabel, ActionPath: n.ActionPath, DedupeKey: n.Dedupe, Now: now,
	})
}

// Notifications reads an Account's latest Notifications and its unread count.
func (s *Store) Notifications(ctx context.Context, account domain.AccountID) ([]domain.Notification, int, error) {
	rows, err := s.q.ListNotifications(ctx, account)
	if err != nil {
		return nil, 0, err
	}
	out := make([]domain.Notification, 0, len(rows))
	for _, r := range rows {
		n := domain.Notification{
			Notice: domain.Notice{Kind: r.Kind, Title: r.Title, Body: r.Body, ActionLabel: r.ActionLabel, ActionPath: r.ActionPath, Dedupe: ""},
			ID:     r.ID, At: r.CreatedAt, ReadAt: nil,
		}
		if r.ReadAt.Valid {
			read := r.ReadAt.Time
			n.ReadAt = &read
		}
		out = append(out, n)
	}
	unread, err := s.q.UnreadNotifications(ctx, account)
	return out, int(unread), err
}

// ReadNotification marks one Notification read; false when it was not an unread one of the Account's.
func (s *Store) ReadNotification(ctx context.Context, account domain.AccountID, id uuid.UUID, now time.Time) (bool, error) {
	n, err := s.q.ReadNotification(ctx, queries.ReadNotificationParams{Now: pgtype.Timestamptz{Time: now, Valid: true}, ID: id, AccountID: account})
	return n == 1, err
}

// ReadAllNotifications marks every Notification of an Account's read.
func (s *Store) ReadAllNotifications(ctx context.Context, account domain.AccountID, now time.Time) error {
	return s.q.ReadAllNotifications(ctx, queries.ReadAllNotificationsParams{Now: pgtype.Timestamptz{Time: now, Valid: true}, AccountID: account})
}

// Preferences reads the choices an Account made, keyed by kind and channel.
func (s *Store) Preferences(ctx context.Context, account domain.AccountID) (map[[2]string]bool, error) {
	rows, err := s.q.NotificationPreferences(ctx, account)
	if err != nil {
		return nil, err
	}
	out := make(map[[2]string]bool, len(rows))
	for _, r := range rows {
		out[[2]string{r.Kind, r.Channel}] = r.Enabled
	}
	return out, nil
}

// SetPreference stores whether a kind reaches an Account on a channel.
func (s *Store) SetPreference(ctx context.Context, account domain.AccountID, kind, channel string, enabled bool) error {
	return s.q.SetNotificationPreference(ctx, queries.SetNotificationPreferenceParams{AccountID: account, Kind: kind, Channel: channel, Enabled: enabled})
}

// Recipient reads where a live Account's Notifications go beyond the bell.
func (s *Store) Recipient(ctx context.Context, account domain.AccountID) (domain.Recipient, error) {
	r, err := s.q.SocialRecipient(ctx, account)
	return domain.Recipient{Subject: r.Subject, Email: r.Email, Nickname: r.Nickname}, notFound(err)
}

// QueueEmail keeps a Notice for the next Digest, under the email it becomes.
func (s *Store) QueueEmail(ctx context.Context, account domain.AccountID, id uuid.UUID, n domain.Notice, now time.Time) error {
	return s.q.QueueEmail(ctx, queries.QueueEmailParams{
		ID: id, AccountID: account, Kind: n.Mail, Title: n.Title, Body: n.Body, ActionLabel: n.ActionLabel, ActionPath: n.ActionPath, DedupeKey: n.Dedupe, Now: now,
	})
}

// DueDigests lists the Accounts with email waiting and no Digest since the cutoff.
func (s *Store) DueDigests(ctx context.Context, cutoff time.Time) ([]domain.AccountID, error) {
	return s.q.DueDigests(ctx, cutoff)
}

// TakeQueued removes and returns an Account's waiting email, oldest first.
func (s *Store) TakeQueued(ctx context.Context, account domain.AccountID) ([]domain.Queued, error) {
	rows, err := s.q.TakeQueuedEmail(ctx, account)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Queued, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Queued{Notice: domain.Notice{Kind: "", Title: r.Title, Body: r.Body, ActionLabel: r.ActionLabel, ActionPath: r.ActionPath, Dedupe: "", Mail: r.Kind}, At: r.CreatedAt})
	}
	slices.SortFunc(out, func(a, b domain.Queued) int { return a.At.Compare(b.At) })
	return out, nil
}

// MarkDigest records that an Account just got a Digest.
func (s *Store) MarkDigest(ctx context.Context, account domain.AccountID, now time.Time) error {
	return s.q.MarkDigest(ctx, queries.MarkDigestParams{AccountID: account, Now: now})
}
