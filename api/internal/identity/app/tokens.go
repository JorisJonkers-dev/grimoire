package app

import (
	"context"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
)

// AccessTokenRepository persists Access Tokens.
type AccessTokenRepository interface {
	InsertAccessToken(ctx context.Context, account domain.AccountID, t domain.AccessToken, tokenHash []byte) error
	AccessTokens(ctx context.Context, account domain.AccountID, now time.Time) ([]domain.AccessToken, error)
	RevokeAccessToken(ctx context.Context, account domain.AccountID, id uuid.UUID, now time.Time) (bool, error)
	AccessTokenSubject(ctx context.Context, tokenHash []byte, now time.Time) (uuid.UUID, string, []string, bool, error)
	TouchAccessToken(ctx context.Context, id uuid.UUID, now time.Time) error
}

// TokenPrefix marks a Grimoire Access Token, so a bearer meant for another service is left alone.
const TokenPrefix = "gmt_"

// MaxTokenDays is the longest an Access Token lives.
const MaxTokenDays = 365

// MintToken makes an Access Token for the signed-in Account: a name of up to 60 characters, at least
// one scope, and 1 to 365 days. It returns the token, shown only now.
func (s *Service) MintToken(ctx context.Context, subject, name string, scopes []string, days int) (string, domain.AccessToken, error) {
	name = strings.TrimSpace(name)
	scopes = cleanScopes(scopes)
	if name == "" || utf8.RuneCountInString(name) > 60 || len(scopes) == 0 || days < 1 || days > MaxTokenDays {
		return "", domain.AccessToken{}, domain.ErrInvalid
	}
	a, err := s.Repo.AccountBySubject(ctx, subject)
	if err != nil {
		return "", domain.AccessToken{}, err
	}
	raw, _, err := newToken()
	if err != nil {
		return "", domain.AccessToken{}, err
	}
	token := TokenPrefix + raw
	now := s.Now()
	t := domain.AccessToken{ID: uuid.New(), Name: name, Scopes: scopes, CreatedAt: now, ExpiresAt: now.AddDate(0, 0, days), LastUsedAt: nil}
	return token, t, s.Repo.InsertAccessToken(ctx, a.ID, t, HashToken(token))
}

// cleanScopes keeps the known scopes, once each, in a fixed order.
func cleanScopes(in []string) []string {
	out := make([]string, 0, 3)
	for _, s := range []string{domain.ScopeRead, domain.ScopeBuild, domain.ScopePlay} {
		if slices.Contains(in, s) {
			out = append(out, s)
		}
	}
	if len(out) != len(slices.Compact(slices.Sorted(slices.Values(in)))) {
		return nil
	}
	return out
}

// AccessTokens lists the signed-in Account's live Access Tokens, newest first.
func (s *Service) AccessTokens(ctx context.Context, subject string) ([]domain.AccessToken, error) {
	a, err := s.Repo.AccountBySubject(ctx, subject)
	if err != nil {
		return nil, err
	}
	return s.Repo.AccessTokens(ctx, a.ID, s.Now())
}

// RevokeToken ends one of the signed-in Account's Access Tokens at once.
func (s *Service) RevokeToken(ctx context.Context, subject string, id uuid.UUID) error {
	a, err := s.Repo.AccountBySubject(ctx, subject)
	if err != nil {
		return err
	}
	gone, err := s.Repo.RevokeAccessToken(ctx, a.ID, id, s.Now())
	if err != nil {
		return err
	}
	if !gone {
		return domain.ErrNotFound
	}
	return nil
}

// ResolveToken finds the subject and scopes an Access Token acts with, refusing one that is expired,
// revoked or whose Account is disabled.
func (s *Service) ResolveToken(ctx context.Context, token string) (string, []string, bool) {
	if !strings.HasPrefix(token, TokenPrefix) {
		return "", nil, false
	}
	now := s.Now()
	id, subject, scopes, disabled, err := s.Repo.AccessTokenSubject(ctx, HashToken(token), now)
	if err != nil || disabled {
		return "", nil, false
	}
	_ = s.Repo.TouchAccessToken(ctx, id, now)
	return subject, scopes, true
}
