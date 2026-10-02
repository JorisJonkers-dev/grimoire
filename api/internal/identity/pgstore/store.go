// Package pgstore is the Postgres adapter for Grimoire's own Accounts.
package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

// Store implements app.Repository.
type Store struct {
	pool *pgxpool.Pool
	q    *queries.Queries
}

var _ app.Repository = (*Store)(nil)

// New wraps a pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, q: queries.New(pool)}
}

// InTx runs fn against a Store bound to one transaction.
func (s *Store) InTx(ctx context.Context, fn func(app.Repository) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(&Store{pool: s.pool, q: queries.New(tx)})
	})
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func account(id uuid.UUID, subject, username, nickname, email string, admin, disabled bool, created time.Time) domain.Account {
	return domain.Account{ID: id, Subject: subject, Username: username, Nickname: nickname, Email: email, Admin: admin, Disabled: disabled, CreatedAt: created}
}

// InsertAccount stores a new Account; a taken Username, email or subject is ErrConflict.
func (s *Store) InsertAccount(ctx context.Context, a domain.Account, passwordHash string) (domain.Account, error) {
	r, err := s.q.InsertAccount(ctx, queries.InsertAccountParams{
		ID: a.ID, Subject: a.Subject, Username: a.Username, Nickname: a.Nickname, Email: a.Email,
		PasswordHash: pgtype.Text{String: passwordHash, Valid: passwordHash != ""}, Admin: a.Admin, Now: a.CreatedAt,
	})
	if err != nil {
		return domain.Account{}, conflict(err)
	}
	return account(r.ID, r.Subject, r.Username, r.Nickname, r.Email, r.Admin, r.Disabled, r.CreatedAt), nil
}

// AccountByUsername reads an Account and its password hash.
func (s *Store) AccountByUsername(ctx context.Context, username string) (domain.Account, string, error) {
	r, err := s.q.AccountByUsername(ctx, username)
	if err != nil {
		return domain.Account{}, "", notFound(err)
	}
	return account(r.ID, r.Subject, r.Username, r.Nickname, r.Email, r.Admin, r.Disabled, r.CreatedAt), r.PasswordHash.String, nil
}

// AccountByEmail reads an Account by its email, ignoring case.
func (s *Store) AccountByEmail(ctx context.Context, email string) (domain.Account, error) {
	r, err := s.q.AccountByEmail(ctx, email)
	if err != nil {
		return domain.Account{}, notFound(err)
	}
	return account(r.ID, r.Subject, r.Username, r.Nickname, r.Email, r.Admin, r.Disabled, r.CreatedAt), nil
}

// AccountBySubject reads the Account a subject signs in as.
func (s *Store) AccountBySubject(ctx context.Context, subject string) (domain.Account, error) {
	r, err := s.q.AccountBySubject(ctx, subject)
	if err != nil {
		return domain.Account{}, notFound(err)
	}
	return account(r.ID, r.Subject, r.Username, r.Nickname, r.Email, r.Admin, r.Disabled, r.CreatedAt), nil
}

// AccountByID reads an Account.
func (s *Store) AccountByID(ctx context.Context, id domain.AccountID) (domain.Account, error) {
	r, err := s.q.AccountByID(ctx, id)
	if err != nil {
		return domain.Account{}, notFound(err)
	}
	return account(r.ID, r.Subject, r.Username, r.Nickname, r.Email, r.Admin, r.Disabled, r.CreatedAt), nil
}

// SetPassword replaces an Account's password hash.
func (s *Store) SetPassword(ctx context.Context, id domain.AccountID, hash string) error {
	return s.q.SetAccountPassword(ctx, queries.SetAccountPasswordParams{ID: id, PasswordHash: pgtype.Text{String: hash, Valid: true}})
}

// InsertInvite stores an Account Invite under its token's hash.
func (s *Store) InsertInvite(ctx context.Context, inv domain.Invite, tokenHash []byte) error {
	return s.q.InsertInvite(ctx, queries.InsertInviteParams{ID: inv.ID, TokenHash: tokenHash, CreatedBy: inv.CreatedBy, Admin: inv.Admin, Now: inv.CreatedAt, ExpiresAt: inv.ExpiresAt})
}

// InviteByToken reads an Account Invite by its token's hash.
func (s *Store) InviteByToken(ctx context.Context, tokenHash []byte) (domain.Invite, error) {
	r, err := s.q.InviteByToken(ctx, tokenHash)
	if err != nil {
		return domain.Invite{}, notFound(err)
	}
	inv := domain.Invite{ID: r.ID, CreatedBy: r.CreatedBy, Admin: r.Admin, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, UsedAt: nil}
	if r.UsedAt.Valid {
		used := r.UsedAt.Time
		inv.UsedAt = &used
	}
	return inv, nil
}

// UseInvite closes an open Invite for the Account it set up; false when it was no longer open.
func (s *Store) UseInvite(ctx context.Context, id uuid.UUID, account domain.AccountID, now time.Time) (bool, error) {
	n, err := s.q.UseInvite(ctx, queries.UseInviteParams{ID: id, AccountID: pgtype.UUID{Bytes: account, Valid: true}, Now: pgtype.Timestamptz{Time: now, Valid: true}})
	return n == 1, err
}

// InsertSession stores a signed-in device under its token's hash.
func (s *Store) InsertSession(ctx context.Context, ses domain.Session, tokenHash []byte, now time.Time) error {
	return s.q.InsertAccountSession(ctx, queries.InsertAccountSessionParams{
		ID: ses.ID, AccountID: ses.Account, TokenHash: tokenHash, UserAgent: ses.UserAgent, Now: now, ExpiresAt: ses.ExpiresAt, Strong: ses.Strong,
	})
}

// SessionSubject reads the live session a token belongs to: its id, its Account's subject, whether
// that Account is disabled, and whether the session is strong.
func (s *Store) SessionSubject(ctx context.Context, tokenHash []byte, now time.Time) (domain.LiveSession, error) {
	r, err := s.q.SessionAccount(ctx, queries.SessionAccountParams{TokenHash: tokenHash, Now: now})
	if err != nil {
		return domain.LiveSession{}, notFound(err)
	}
	return domain.LiveSession{ID: r.ID, Subject: r.Subject, Disabled: r.Disabled, Strong: r.Strong}, nil
}

// TouchSession records that a session was used, at most once a minute.
func (s *Store) TouchSession(ctx context.Context, id uuid.UUID, now time.Time) error {
	return s.q.TouchAccountSession(ctx, queries.TouchAccountSessionParams{ID: id, Now: now, Cutoff: now.Add(-time.Minute)})
}

// RevokeSession ends the session a token belongs to.
func (s *Store) RevokeSession(ctx context.Context, tokenHash []byte, now time.Time) error {
	return s.q.RevokeAccountSession(ctx, queries.RevokeAccountSessionParams{TokenHash: tokenHash, Now: pgtype.Timestamptz{Time: now, Valid: true}})
}

// InsertSignInLink stores an emailed sign-in link under its token's hash.
func (s *Store) InsertSignInLink(ctx context.Context, tokenHash []byte, account domain.AccountID, now, expires time.Time) error {
	return s.q.InsertSignInLink(ctx, queries.InsertSignInLinkParams{TokenHash: tokenHash, AccountID: account, Now: now, ExpiresAt: expires})
}

// UseSignInLink closes an open sign-in link and returns its Account.
func (s *Store) UseSignInLink(ctx context.Context, tokenHash []byte, now time.Time) (domain.AccountID, error) {
	id, err := s.q.UseSignInLink(ctx, queries.UseSignInLinkParams{TokenHash: tokenHash, Now: pgtype.Timestamptz{Time: now, Valid: true}})
	return id, notFound(err)
}
