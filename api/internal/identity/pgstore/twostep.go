package pgstore

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

func at(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// StartTOTP stores a new authenticator secret; false when two-step is already on.
func (s *Store) StartTOTP(ctx context.Context, account domain.AccountID, secret string, now time.Time) (bool, error) {
	n, err := s.q.StartTOTP(ctx, queries.StartTOTPParams{AccountID: account, Secret: secret, Now: now})
	return n == 1, err
}

// TOTPFactor reads an Account's authenticator secret and whether it is confirmed.
func (s *Store) TOTPFactor(ctx context.Context, account domain.AccountID) (string, bool, error) {
	r, err := s.q.TOTPFactor(ctx, account)
	if err != nil {
		return "", false, notFound(err)
	}
	return r.Secret, r.ConfirmedAt.Valid, nil
}

// ConfirmTOTP turns two-step on at the step of its first code; false when it already was.
func (s *Store) ConfirmTOTP(ctx context.Context, account domain.AccountID, step int64, now time.Time) (bool, error) {
	n, err := s.q.ConfirmTOTP(ctx, queries.ConfirmTOTPParams{Now: at(now), Step: step, AccountID: account})
	return n == 1, err
}

// UseTOTPStep spends a time step; false when a code for it or a later one was used already.
func (s *Store) UseTOTPStep(ctx context.Context, account domain.AccountID, step int64) (bool, error) {
	n, err := s.q.UseTOTPStep(ctx, queries.UseTOTPStepParams{Step: step, AccountID: account})
	return n == 1, err
}

// DeleteTOTP turns two-step off and drops the recovery codes.
func (s *Store) DeleteTOTP(ctx context.Context, account domain.AccountID) error {
	if err := s.q.DeleteRecoveryCodes(ctx, account); err != nil {
		return err
	}
	return s.q.DeleteTOTP(ctx, account)
}

// ReplaceRecoveryCodes swaps an Account's recovery codes for new ones.
func (s *Store) ReplaceRecoveryCodes(ctx context.Context, account domain.AccountID, hashes [][]byte) error {
	if err := s.q.DeleteRecoveryCodes(ctx, account); err != nil {
		return err
	}
	for _, h := range hashes {
		if err := s.q.InsertRecoveryCode(ctx, queries.InsertRecoveryCodeParams{CodeHash: h, AccountID: account}); err != nil {
			return err
		}
	}
	return nil
}

// UseRecoveryCode spends a recovery code; false when it is not one of the Account's unused codes.
func (s *Store) UseRecoveryCode(ctx context.Context, account domain.AccountID, hash []byte, now time.Time) (bool, error) {
	n, err := s.q.UseRecoveryCode(ctx, queries.UseRecoveryCodeParams{Now: at(now), CodeHash: hash, AccountID: account})
	return n == 1, err
}

// RecoveryCodesLeft counts an Account's unused recovery codes.
func (s *Store) RecoveryCodesLeft(ctx context.Context, account domain.AccountID) (int, error) {
	n, err := s.q.RecoveryCodesLeft(ctx, account)
	return int(n), err
}

// InsertChallenge stores a sign-in waiting for its second step under its token's hash.
func (s *Store) InsertChallenge(ctx context.Context, tokenHash []byte, account domain.AccountID, now, expires time.Time) error {
	return s.q.InsertTwoStepChallenge(ctx, queries.InsertTwoStepChallengeParams{TokenHash: tokenHash, AccountID: account, Now: now, ExpiresAt: expires})
}

// TryChallenge counts an attempt at an open challenge and returns its Account.
func (s *Store) TryChallenge(ctx context.Context, tokenHash []byte, now time.Time, maxAttempts int) (domain.AccountID, error) {
	id, err := s.q.TryTwoStepChallenge(ctx, queries.TryTwoStepChallengeParams{TokenHash: tokenHash, Now: now, MaxAttempts: int32(maxAttempts)}) //nolint:gosec // a small constant
	return id, notFound(err)
}

// UseChallenge closes a challenge its second step answered.
func (s *Store) UseChallenge(ctx context.Context, tokenHash []byte, now time.Time) error {
	return s.q.UseTwoStepChallenge(ctx, queries.UseTwoStepChallengeParams{Now: at(now), TokenHash: tokenHash})
}

// StrengthenSession marks an Account's session strong.
func (s *Store) StrengthenSession(ctx context.Context, tokenHash []byte, account domain.AccountID) error {
	return s.q.StrengthenSession(ctx, queries.StrengthenSessionParams{TokenHash: tokenHash, AccountID: account})
}
