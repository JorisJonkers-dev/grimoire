package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

func conflict(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return domain.ErrConflict
	}
	return err
}

// HasPassword reports whether an Account can sign in with a password.
func (s *Store) HasPassword(ctx context.Context, id domain.AccountID) (bool, error) {
	has, err := s.q.AccountHasPassword(ctx, id)
	return has, notFound(err)
}

// UpdateProfile sets an Account's Username, Nickname and email; a taken one is ErrConflict.
func (s *Store) UpdateProfile(ctx context.Context, id domain.AccountID, p domain.ProfileChange) error {
	return conflict(s.q.UpdateAccountProfile(ctx, queries.UpdateAccountProfileParams{Username: p.Username, Nickname: p.Nickname, Email: optional(p.Email), ID: id}))
}

// SetAdmin makes an Account an Admin or not.
func (s *Store) SetAdmin(ctx context.Context, id domain.AccountID, admin bool) error {
	return s.q.SetAccountAdmin(ctx, queries.SetAccountAdminParams{Admin: admin, ID: id})
}

// InsertOIDCRequest stores an OIDC sign-in sent to the provider under its state's hash.
func (s *Store) InsertOIDCRequest(ctx context.Context, stateHash []byte, nonce, verifier string, account *domain.AccountID, now, expires time.Time) error {
	p := queries.InsertOIDCRequestParams{StateHash: stateHash, Nonce: nonce, Verifier: verifier, AccountID: pgtype.UUID{}, Now: now, ExpiresAt: expires}
	if account != nil {
		p.AccountID = pgtype.UUID{Bytes: *account, Valid: true}
	}
	return s.q.InsertOIDCRequest(ctx, p)
}

// UseOIDCRequest closes an open OIDC sign-in and returns its nonce, verifier and the Account it links.
func (s *Store) UseOIDCRequest(ctx context.Context, stateHash []byte, now time.Time) (string, string, *domain.AccountID, error) {
	r, err := s.q.UseOIDCRequest(ctx, queries.UseOIDCRequestParams{Now: pgtype.Timestamptz{Time: now, Valid: true}, StateHash: stateHash})
	if err != nil {
		return "", "", nil, notFound(err)
	}
	if !r.AccountID.Valid {
		return r.Nonce, r.Verifier, nil, nil
	}
	id := domain.AccountID(r.AccountID.Bytes)
	return r.Nonce, r.Verifier, &id, nil
}

// LinkOf reads the login linked to an Account.
func (s *Store) LinkOf(ctx context.Context, account domain.AccountID) (domain.Link, error) {
	r, err := s.q.OIDCLinkByAccount(ctx, account)
	if err != nil {
		return domain.Link{}, notFound(err)
	}
	return domain.Link{Issuer: r.Issuer, Subject: r.Subject, Email: r.Email, Username: r.Username, Name: r.Name, LinkedAt: r.LinkedAt}, nil
}

// LinkedAccount finds the Account a login is linked to.
func (s *Store) LinkedAccount(ctx context.Context, issuer, subject string) (domain.AccountID, error) {
	r, err := s.q.OIDCLinkBySubject(ctx, queries.OIDCLinkBySubjectParams{Issuer: issuer, Subject: subject})
	if err != nil {
		return domain.AccountID{}, notFound(err)
	}
	return r.AccountID, nil
}

// InsertLink links a login to an Account; an Account already linked, or a login linked elsewhere, is
// ErrConflict.
func (s *Store) InsertLink(ctx context.Context, account domain.AccountID, l domain.Link) error {
	return conflict(s.q.InsertOIDCLink(ctx, queries.InsertOIDCLinkParams{
		AccountID: account, Issuer: l.Issuer, Subject: l.Subject, Email: l.Email, Username: l.Username, Name: l.Name, Now: l.LinkedAt,
	}))
}

// UpdateLink stores the claims a linked login last sent.
func (s *Store) UpdateLink(ctx context.Context, account domain.AccountID, l domain.Link) error {
	return s.q.UpdateOIDCLink(ctx, queries.UpdateOIDCLinkParams{Email: l.Email, Username: l.Username, Name: l.Name, AccountID: account})
}

// DeleteLink unlinks an Account's login; false when it had none.
func (s *Store) DeleteLink(ctx context.Context, account domain.AccountID) (bool, error) {
	n, err := s.q.DeleteOIDCLink(ctx, account)
	return n == 1, err
}

// InsertPending stores a login waiting for an Account under its token's hash.
func (s *Store) InsertPending(ctx context.Context, tokenHash []byte, c domain.Claims, admin bool, now, expires time.Time) error {
	return s.q.InsertOIDCPending(ctx, queries.InsertOIDCPendingParams{
		TokenHash: tokenHash, Issuer: c.Issuer, Subject: c.Subject, Email: c.Email, Username: c.Username, Name: c.Name, Admin: admin, Now: now, ExpiresAt: expires,
	})
}

// UsePending closes a waiting login and returns its claims and whether it makes an Admin.
func (s *Store) UsePending(ctx context.Context, tokenHash []byte, now time.Time) (domain.Claims, bool, error) {
	r, err := s.q.UseOIDCPending(ctx, queries.UseOIDCPendingParams{Now: pgtype.Timestamptz{Time: now, Valid: true}, TokenHash: tokenHash})
	if err != nil {
		return domain.Claims{}, false, notFound(err)
	}
	return domain.Claims{Issuer: r.Issuer, Subject: r.Subject, Email: r.Email, Username: r.Username, Name: r.Name, Roles: nil, Nonce: ""}, r.Admin, nil
}
