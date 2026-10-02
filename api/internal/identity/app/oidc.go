package app

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
)

// Provider is an OIDC provider: where to send someone to sign in, and what it vouches for once they
// come back with a code.
type Provider interface {
	AuthURL(ctx context.Context, state, nonce, verifier string) (string, error)
	Exchange(ctx context.Context, code, verifier string) (domain.Claims, error)
}

// OIDCRepository persists OIDC sign-ins under way, logins waiting for an Account, and links.
type OIDCRepository interface {
	InsertOIDCRequest(ctx context.Context, stateHash []byte, nonce, verifier string, account *domain.AccountID, now, expires time.Time) error
	UseOIDCRequest(ctx context.Context, stateHash []byte, now time.Time) (string, string, *domain.AccountID, error)
	LinkOf(ctx context.Context, account domain.AccountID) (domain.Link, error)
	LinkedAccount(ctx context.Context, issuer, subject string) (domain.AccountID, error)
	InsertLink(ctx context.Context, account domain.AccountID, l domain.Link) error
	UpdateLink(ctx context.Context, account domain.AccountID, l domain.Link) error
	DeleteLink(ctx context.Context, account domain.AccountID) (bool, error)
	InsertPending(ctx context.Context, tokenHash []byte, c domain.Claims, admin bool, now, expires time.Time) error
	UsePending(ctx context.Context, tokenHash []byte, now time.Time) (domain.Claims, bool, error)
}

// How long an OIDC sign-in stays open, at the provider and then waiting for an Account.
const (
	OIDCRequestTTL = 10 * time.Minute
	OIDCPendingTTL = 15 * time.Minute
)

// OIDCOutcome is how an OIDC sign-in ended: signed in with a session, linked to the Account that
// started it, or waiting for its holder to create or link an Account.
type OIDCOutcome struct {
	Account domain.Account
	Session string
	Linked  bool
	Pending *domain.Pending
}

// StartOIDC opens an OIDC sign-in and returns where to send the browser, and the state it comes back
// with. With a subject it links the login to that subject's Account instead of signing in.
func (s *Service) StartOIDC(ctx context.Context, linkFor string) (string, string, error) {
	if s.OIDC == nil {
		return "", "", domain.ErrDisabled
	}
	var account *domain.AccountID
	if linkFor != "" {
		a, err := s.Repo.AccountBySubject(ctx, linkFor)
		if err != nil {
			return "", "", err
		}
		account = &a.ID
	}
	state, hash, err := newToken()
	if err != nil {
		return "", "", err
	}
	nonce, _, err := newToken()
	if err != nil {
		return "", "", err
	}
	verifier, _, err := newToken()
	if err != nil {
		return "", "", err
	}
	now := s.Now()
	if err := s.Repo.InsertOIDCRequest(ctx, hash, nonce, verifier, account, now, now.Add(OIDCRequestTTL)); err != nil {
		return "", "", err
	}
	url, err := s.OIDC.AuthURL(ctx, state, nonce, verifier)
	return url, state, err
}

// FinishOIDC takes the browser back from the provider. Only a login holding the Grant role gets
// further; losing it stops this path alone, never a password.
func (s *Service) FinishOIDC(ctx context.Context, code, state, userAgent string) (OIDCOutcome, error) {
	var out OIDCOutcome
	if s.OIDC == nil {
		return out, domain.ErrDisabled
	}
	now := s.Now()
	nonce, verifier, account, err := s.Repo.UseOIDCRequest(ctx, HashToken(state), now)
	if err != nil {
		return out, domain.ErrExpired
	}
	c, err := s.OIDC.Exchange(ctx, code, verifier)
	if err != nil || c.Nonce != nonce {
		return out, domain.ErrUnauthenticated
	}
	if !s.granted(c) {
		return out, domain.ErrForbidden
	}
	if account != nil {
		return s.linkTo(ctx, *account, c, now)
	}
	id, err := s.Repo.LinkedAccount(ctx, c.Issuer, c.Subject)
	if errors.Is(err, domain.ErrNotFound) {
		out.Pending, err = s.pend(ctx, c, now)
		return out, err
	}
	if err != nil {
		return out, err
	}
	return s.signInLinked(ctx, id, c, userAgent, now)
}

func (s *Service) granted(c domain.Claims) bool {
	return (s.Grant != "" && slices.Contains(c.Roles, s.Grant)) || s.admin(c)
}

func (s *Service) admin(c domain.Claims) bool {
	return s.AdminRole != "" && slices.Contains(c.Roles, s.AdminRole)
}

func linkOf(c domain.Claims, now time.Time) domain.Link {
	return domain.Link{Issuer: c.Issuer, Subject: c.Subject, Email: c.Email, Username: c.Username, Name: c.Name, LinkedAt: now}
}

func (s *Service) linkTo(ctx context.Context, id domain.AccountID, c domain.Claims, now time.Time) (OIDCOutcome, error) {
	out := OIDCOutcome{Account: domain.Account{}, Session: "", Linked: true, Pending: nil}
	a, err := s.Repo.AccountByID(ctx, id)
	if err != nil {
		return out, err
	}
	if err := s.Repo.InsertLink(ctx, id, linkOf(c, now)); err != nil {
		return out, err
	}
	if err := s.record(ctx, s.Repo, id, a.Subject, domain.EventLinked, "from the Account page"); err != nil {
		return out, err
	}
	out.Account, err = promote(ctx, s.Repo, a, s.admin(c))
	return out, err
}

// promote makes an Account an Admin when its login carries the AdminRole. It never demotes: another
// Admin may have promoted it.
func promote(ctx context.Context, r Repository, a domain.Account, admin bool) (domain.Account, error) {
	if a.Admin || !admin {
		return a, nil
	}
	a.Admin = true
	return a, r.SetAdmin(ctx, a.ID, true)
}

func (s *Service) signInLinked(ctx context.Context, id domain.AccountID, c domain.Claims, userAgent string, now time.Time) (OIDCOutcome, error) {
	out := OIDCOutcome{Account: domain.Account{}, Session: "", Linked: false, Pending: nil}
	a, err := s.Repo.AccountByID(ctx, id)
	if err != nil {
		return out, err
	}
	if a.Disabled {
		return out, domain.ErrUnauthenticated
	}
	if err := s.Repo.UpdateLink(ctx, id, linkOf(c, now)); err != nil {
		return out, err
	}
	if out.Account, err = promote(ctx, s.Repo, a, s.admin(c)); err != nil {
		return out, err
	}
	out.Session, err = startSession(ctx, s.Repo, id, userAgent, now, true)
	return out, err
}

func (s *Service) pend(ctx context.Context, c domain.Claims, now time.Time) (*domain.Pending, error) {
	token, hash, err := newToken()
	if err != nil {
		return nil, err
	}
	if err := s.Repo.InsertPending(ctx, hash, c, s.admin(c), now, now.Add(OIDCPendingTTL)); err != nil {
		return nil, err
	}
	return &domain.Pending{Token: token, Email: c.Email, Username: c.Username, Name: c.Name}, nil
}

// CreateFromOIDC sets up an Account for a login waiting for one and signs it in. The Account keys on
// the login's own subject, so whatever that subject already joined stays its own.
func (s *Service) CreateFromOIDC(ctx context.Context, token, username, nickname, userAgent string) (domain.Account, string, error) {
	var out domain.Account
	var session string
	err := s.Repo.InTx(ctx, func(r Repository) error {
		now := s.Now()
		c, admin, err := r.UsePending(ctx, HashToken(token), now)
		if err != nil {
			return domain.ErrExpired
		}
		in, err := cleanProfile(domain.ProfileChange{Username: username, Nickname: nickname, Email: c.Email})
		if err != nil {
			return err
		}
		a := domain.Account{ID: uuid.New(), Subject: c.Subject, Username: in.Username, Nickname: in.Nickname, Email: in.Email, Admin: admin, Disabled: false, CreatedAt: now}
		if out, err = r.InsertAccount(ctx, a, ""); err != nil {
			return err
		}
		if err := r.InsertLink(ctx, out.ID, linkOf(c, now)); err != nil {
			return err
		}
		if err := s.record(ctx, r, out.ID, out.Subject, domain.EventCreated, "from the external login"); err != nil {
			return err
		}
		session, err = startSession(ctx, r, out.ID, userAgent, now, true)
		return err
	})
	return out, session, err
}

// LinkFromOIDC links a login waiting for an Account to the Account a Username and password sign in,
// and signs it in. A wrong password leaves the login waiting.
func (s *Service) LinkFromOIDC(ctx context.Context, token, username, password, userAgent string) (domain.Account, string, error) {
	a, hash, err := s.Repo.AccountByUsername(ctx, strings.ToLower(strings.TrimSpace(username)))
	if err != nil || hash == "" {
		s.Passwords.Verify(password, dummyHash)
		return domain.Account{}, "", domain.ErrUnauthenticated
	}
	if !s.Passwords.Verify(password, hash) || a.Disabled {
		return domain.Account{}, "", domain.ErrUnauthenticated
	}
	var session string
	err = s.Repo.InTx(ctx, func(r Repository) error {
		now := s.Now()
		c, admin, err := r.UsePending(ctx, HashToken(token), now)
		if err != nil {
			return domain.ErrExpired
		}
		if err := r.InsertLink(ctx, a.ID, linkOf(c, now)); err != nil {
			return err
		}
		if err := s.record(ctx, r, a.ID, a.Subject, domain.EventLinked, "from the sign-in page"); err != nil {
			return err
		}
		if a, err = promote(ctx, r, a, admin); err != nil {
			return err
		}
		session, err = startSession(ctx, r, a.ID, userAgent, now, true)
		return err
	})
	return a, session, err
}

// Unlink removes the signed-in Account's linked login; the Account stays. An Account without a
// password keeps its login, or it could no longer sign in but by email.
func (s *Service) Unlink(ctx context.Context, subject string) error {
	a, err := s.Repo.AccountBySubject(ctx, subject)
	if err != nil {
		return err
	}
	has, err := s.Repo.HasPassword(ctx, a.ID)
	if err != nil {
		return err
	}
	if !has {
		return domain.ErrManaged
	}
	gone, err := s.Repo.DeleteLink(ctx, a.ID)
	if err != nil {
		return err
	}
	if !gone {
		return domain.ErrNotFound
	}
	return s.record(ctx, s.Repo, a.ID, subject, domain.EventUnlinked, "")
}

// OIDCEnabled reports whether an OIDC sign-in is set up.
func (s *Service) OIDCEnabled() bool {
	return s.OIDC != nil
}
