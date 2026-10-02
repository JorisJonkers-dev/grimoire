package pgstore

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

// InsertAccessToken stores an Access Token under its hash.
func (s *Store) InsertAccessToken(ctx context.Context, account domain.AccountID, t domain.AccessToken, tokenHash []byte) error {
	return s.q.InsertAccessToken(ctx, queries.InsertAccessTokenParams{
		ID: t.ID, AccountID: account, Name: t.Name, Scopes: t.Scopes, TokenHash: tokenHash, Now: t.CreatedAt, ExpiresAt: t.ExpiresAt,
	})
}

// AccessTokens lists an Account's live Access Tokens.
func (s *Store) AccessTokens(ctx context.Context, account domain.AccountID, now time.Time) ([]domain.AccessToken, error) {
	rows, err := s.q.ListAccessTokens(ctx, queries.ListAccessTokensParams{AccountID: account, Now: now})
	if err != nil {
		return nil, err
	}
	out := make([]domain.AccessToken, 0, len(rows))
	for _, r := range rows {
		t := domain.AccessToken{ID: r.ID, Name: r.Name, Scopes: r.Scopes, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, LastUsedAt: nil}
		if r.LastUsedAt.Valid {
			used := r.LastUsedAt.Time
			t.LastUsedAt = &used
		}
		out = append(out, t)
	}
	return out, nil
}

// RevokeAccessToken ends an Account's Access Token; false when it had no such live token.
func (s *Store) RevokeAccessToken(ctx context.Context, account domain.AccountID, id uuid.UUID, now time.Time) (bool, error) {
	n, err := s.q.RevokeAccessToken(ctx, queries.RevokeAccessTokenParams{Now: at(now), ID: id, AccountID: account})
	return n == 1, err
}

// AccessTokenSubject reads a live Access Token: its id, its Account's subject, its scopes, and whether
// the Account is disabled.
func (s *Store) AccessTokenSubject(ctx context.Context, tokenHash []byte, now time.Time) (uuid.UUID, string, []string, bool, error) {
	r, err := s.q.AccessTokenAccount(ctx, queries.AccessTokenAccountParams{TokenHash: tokenHash, Now: now})
	if err != nil {
		return uuid.Nil, "", nil, false, notFound(err)
	}
	return r.ID, r.Subject, r.Scopes, r.Disabled, nil
}

// TouchAccessToken records that a token was used, at most once a minute.
func (s *Store) TouchAccessToken(ctx context.Context, id uuid.UUID, now time.Time) error {
	return s.q.TouchAccessToken(ctx, queries.TouchAccessTokenParams{Now: at(now), ID: id, Cutoff: at(now.Add(-time.Minute))})
}
