package pgstore

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

// ListAccounts reads every Account with when it was last seen.
func (s *Store) ListAccounts(ctx context.Context) ([]domain.Listed, error) {
	rows, err := s.q.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Listed, 0, len(rows))
	for _, r := range rows {
		l := domain.Listed{Account: account(r.ID, r.Subject, r.Username, r.Nickname, r.Email, r.Admin, r.Disabled, r.CreatedAt), LastSeenAt: nil}
		if seen, ok := r.LastSeenAt.(time.Time); ok {
			l.LastSeenAt = &seen
		}
		out = append(out, l)
	}
	return out, nil
}

// UnusedInvites reads the Invites nobody has used, open or expired.
func (s *Store) UnusedInvites(ctx context.Context) ([]domain.Invite, error) {
	rows, err := s.q.ListUnusedInvites(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Invite, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Invite{ID: r.ID, CreatedBy: r.CreatedBy, Admin: r.Admin, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, UsedAt: nil})
	}
	return out, nil
}

// InsertEvent writes one line of an Account's history.
func (s *Store) InsertEvent(ctx context.Context, account domain.AccountID, e domain.Event) error {
	return s.q.InsertAccountEvent(ctx, queries.InsertAccountEventParams{ID: uuid.New(), AccountID: account, Actor: e.Actor, Action: e.Action, Detail: e.Detail, At: e.At})
}

// Events reads an Account's latest history, newest first.
func (s *Store) Events(ctx context.Context, account domain.AccountID) ([]domain.Event, error) {
	rows, err := s.q.ListAccountEvents(ctx, account)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Event, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Event{At: r.At, Actor: r.Actor, ActorName: r.ActorName, Action: r.Action, Detail: r.Detail})
	}
	return out, nil
}

// SetDisabled disables an Account or enables it.
func (s *Store) SetDisabled(ctx context.Context, account domain.AccountID, disabled bool) error {
	return s.q.SetAccountDisabled(ctx, queries.SetAccountDisabledParams{Disabled: disabled, ID: account})
}

// RevokeEverything ends every session and Access Token an Account has.
func (s *Store) RevokeEverything(ctx context.Context, account domain.AccountID, now time.Time) error {
	if err := s.q.RevokeAccountSessions(ctx, queries.RevokeAccountSessionsParams{Now: at(now), AccountID: account}); err != nil {
		return err
	}
	return s.q.RevokeAccountTokens(ctx, queries.RevokeAccountTokensParams{Now: at(now), AccountID: account})
}

// LiveCounts counts an Account's live sessions and Access Tokens.
func (s *Store) LiveCounts(ctx context.Context, account domain.AccountID, now time.Time) (int, int, error) {
	sessions, err := s.q.CountLiveSessions(ctx, queries.CountLiveSessionsParams{AccountID: account, Now: now})
	if err != nil {
		return 0, 0, err
	}
	tokens, err := s.q.CountLiveTokens(ctx, queries.CountLiveTokensParams{AccountID: account, Now: now})
	return int(sessions), int(tokens), err
}
