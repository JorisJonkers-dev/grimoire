// Package app runs Grimoire's own Accounts: Admins invite people, invitees set up an Account, and
// Account holders sign in with a password or an emailed link.
package app

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
)

// Repository persists Accounts, Invites, sessions and sign-in links.
type Repository interface {
	InsertAccount(ctx context.Context, a domain.Account, passwordHash string) (domain.Account, error)
	AccountByUsername(ctx context.Context, username string) (domain.Account, string, error)
	AccountByEmail(ctx context.Context, email string) (domain.Account, error)
	AccountBySubject(ctx context.Context, subject string) (domain.Account, error)
	AccountByID(ctx context.Context, id domain.AccountID) (domain.Account, error)
	SetPassword(ctx context.Context, id domain.AccountID, hash string) error
	InsertInvite(ctx context.Context, inv domain.Invite, tokenHash []byte) error
	InviteByToken(ctx context.Context, tokenHash []byte) (domain.Invite, error)
	UseInvite(ctx context.Context, id uuid.UUID, account domain.AccountID, now time.Time) (bool, error)
	InsertSession(ctx context.Context, s domain.Session, tokenHash []byte, now time.Time) error
	SessionSubject(ctx context.Context, tokenHash []byte, now time.Time) (domain.LiveSession, error)
	TouchSession(ctx context.Context, id uuid.UUID, now time.Time) error
	RevokeSession(ctx context.Context, tokenHash []byte, now time.Time) error
	InsertSignInLink(ctx context.Context, tokenHash []byte, account domain.AccountID, now, expires time.Time) error
	UseSignInLink(ctx context.Context, tokenHash []byte, now time.Time) (domain.AccountID, error)
	HasPassword(ctx context.Context, id domain.AccountID) (bool, error)
	UpdateProfile(ctx context.Context, id domain.AccountID, p domain.ProfileChange) error
	SetAdmin(ctx context.Context, id domain.AccountID, admin bool) error
	OIDCRepository
	TwoStepRepository
	AccessTokenRepository
	AdminRepository
	InTx(ctx context.Context, fn func(Repository) error) error
}

// Mailer sends an email.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// How long things last.
const (
	SessionTTL  = 30 * 24 * time.Hour
	SignInLink  = 30 * time.Minute
	MaxInviteHr = 30 * 24
)

// Service is the identity use cases.
type Service struct {
	Repo      Repository
	Mailer    Mailer
	Passwords Passwords
	Now       func() time.Time
	// Admins are subjects that act as Admins without an Account flag, to bootstrap the first invite.
	Admins map[string]bool
	// BaseURL is where links in emails point, such as https://grimoire.example.
	BaseURL string
	// OIDC is the external sign-in, if one is set up: Grant is the role a login needs to sign in at
	// all, AdminRole the role that makes its Account an Admin.
	OIDC      Provider
	Grant     string
	AdminRole string
	// Strong reports whether a request's session holds Admin powers: it passed a second step, came from
	// the external login, or the platform vouched for it.
	Strong func(context.Context) bool
	// Alerts reach an Account's holder when its sign-in changes; nil reaches nobody.
	Alerts Alerts
}

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{2,31}$`)

// IsAdmin reports whether a subject may use Admin powers in this request: an Admin Account needs a
// strong session, so two-step sign-in guards a password.
func (s *Service) IsAdmin(ctx context.Context, subject string) bool {
	if s.Admins[subject] {
		return true
	}
	a, err := s.Repo.AccountBySubject(ctx, subject)
	return err == nil && a.Admin && !a.Disabled && s.Strong != nil && s.Strong(ctx)
}

// CreateInvite makes an Account Invite that expires after so many hours, for an Admin; only an Admin's
// invite can set up another Admin. It returns the token the invite link carries.
func (s *Service) CreateInvite(ctx context.Context, by string, hours int, admin bool) (string, domain.Invite, error) {
	if !s.IsAdmin(ctx, by) {
		return "", domain.Invite{}, domain.ErrForbidden
	}
	if hours < 1 || hours > MaxInviteHr {
		return "", domain.Invite{}, domain.ErrInvalid
	}
	token, hash, err := newToken()
	if err != nil {
		return "", domain.Invite{}, err
	}
	now := s.Now()
	inv := domain.Invite{ID: uuid.New(), CreatedBy: by, Admin: admin, CreatedAt: now, ExpiresAt: now.Add(time.Duration(hours) * time.Hour), UsedAt: nil}
	return token, inv, s.Repo.InsertInvite(ctx, inv, hash)
}

// Invite reads an Account Invite by its token: ErrExpired once used or past its time.
func (s *Service) Invite(ctx context.Context, token string) (domain.Invite, error) {
	inv, err := s.Repo.InviteByToken(ctx, HashToken(token))
	if err != nil {
		return domain.Invite{}, err
	}
	if !inv.Open(s.Now()) {
		return inv, domain.ErrExpired
	}
	return inv, nil
}

// Accept sets up an Account from an open Invite and signs it in, returning the session token.
func (s *Service) Accept(ctx context.Context, token string, in domain.Setup, userAgent string) (domain.Account, string, error) {
	in, err := clean(in)
	if err != nil {
		return domain.Account{}, "", err
	}
	hash, err := s.Passwords.Hash(in.Password)
	if err != nil {
		return domain.Account{}, "", err
	}
	var out domain.Account
	var session string
	err = s.Repo.InTx(ctx, func(r Repository) error {
		inv, err := r.InviteByToken(ctx, HashToken(token))
		if err != nil {
			return err
		}
		now := s.Now()
		if !inv.Open(now) {
			return domain.ErrExpired
		}
		id := uuid.New()
		a := domain.Account{ID: id, Subject: domain.SubjectFor(id), Username: in.Username, Nickname: in.Nickname, Email: in.Email, Admin: inv.Admin, Disabled: false, CreatedAt: now}
		if out, err = r.InsertAccount(ctx, a, hash); err != nil {
			return err
		}
		if used, err := r.UseInvite(ctx, inv.ID, id, now); err != nil || !used {
			return firstErr(err, domain.ErrExpired)
		}
		if err := s.record(ctx, r, id, out.Subject, domain.EventCreated, "from an invite"); err != nil {
			return err
		}
		session, err = startSession(ctx, r, id, userAgent, now, false)
		return err
	})
	return out, session, err
}

func firstErr(err, otherwise error) error {
	if err != nil {
		return err
	}
	return otherwise
}

// clean checks and tidies what an invitee chose: a profile with an email and a password of 10 to 200
// characters.
func clean(in domain.Setup) (domain.Setup, error) {
	if strings.TrimSpace(in.Email) == "" {
		return in, domain.ErrInvalid
	}
	p, err := cleanProfile(domain.ProfileChange{Username: in.Username, Nickname: in.Nickname, Email: in.Email})
	in.Username, in.Nickname, in.Email = p.Username, p.Nickname, p.Email
	if err != nil {
		return in, err
	}
	if n := utf8.RuneCountInString(in.Password); n < 10 || n > 200 {
		return in, domain.ErrInvalid
	}
	return in, nil
}

// cleanProfile checks and tidies a profile: a lowercase Username of 3 to 32 letters, digits, dots,
// dashes or underscores; a Nickname of up to 40 characters; an email, or none.
func cleanProfile(in domain.ProfileChange) (domain.ProfileChange, error) {
	in.Username = strings.ToLower(strings.TrimSpace(in.Username))
	in.Nickname = strings.TrimSpace(in.Nickname)
	in.Email = strings.TrimSpace(in.Email)
	switch {
	case !usernamePattern.MatchString(in.Username):
		return in, domain.ErrInvalid
	case in.Nickname == "" || utf8.RuneCountInString(in.Nickname) > 40:
		return in, domain.ErrInvalid
	case in.Email != "" && (len(in.Email) > 254 || !strings.Contains(in.Email, "@") || strings.HasPrefix(in.Email, "@") || strings.HasSuffix(in.Email, "@")):
		return in, domain.ErrInvalid
	}
	return in, nil
}

// dummyHash keeps a sign-in with an unknown Username as slow as one with a wrong password.
const dummyHash = "$argon2id$v=19$m=19456,t=2,p=1$c29tZXNhbHRzb21lc2FsdA$4m0gQ8Rk1dG8F0bD1yV3c3lnQ1hQ3m5u1n2dQ3lB9w0"

// SignIn checks a Username and password. It signs the Account in, or asks for the second step when
// two-step is on. Every failure looks the same.
func (s *Service) SignIn(ctx context.Context, username, password, userAgent string) (domain.SignedIn, error) {
	a, hash, err := s.Repo.AccountByUsername(ctx, strings.ToLower(strings.TrimSpace(username)))
	if err != nil || hash == "" {
		s.Passwords.Verify(password, dummyHash)
		return domain.SignedIn{}, domain.ErrUnauthenticated
	}
	if !s.Passwords.Verify(password, hash) || a.Disabled {
		return domain.SignedIn{}, domain.ErrUnauthenticated
	}
	return s.firstStep(ctx, a, userAgent)
}

func startSession(ctx context.Context, r Repository, account domain.AccountID, userAgent string, now time.Time, strong bool) (string, error) {
	token, hash, err := newToken()
	if err != nil {
		return "", err
	}
	ua := userAgent
	if len(ua) > 300 {
		ua = ua[:300]
	}
	s := domain.Session{ID: uuid.New(), Account: account, UserAgent: ua, ExpiresAt: now.Add(SessionTTL), Strong: strong}
	return token, r.InsertSession(ctx, s, hash, now)
}

// SignOut ends the session a token belongs to.
func (s *Service) SignOut(ctx context.Context, token string) error {
	return s.Repo.RevokeSession(ctx, HashToken(token), s.Now())
}

// Resolve finds the subject a session token signs in and whether the session is strong, refusing
// disabled Accounts.
func (s *Service) Resolve(ctx context.Context, token string) (string, bool, bool) {
	now := s.Now()
	live, err := s.Repo.SessionSubject(ctx, HashToken(token), now)
	if err != nil || live.Disabled {
		return "", false, false
	}
	_ = s.Repo.TouchSession(ctx, live.ID, now)
	return live.Subject, live.Strong, true
}

// RequestLink emails an Account's holder a sign-in link, if an Account has that email. It never says
// whether one does.
func (s *Service) RequestLink(ctx context.Context, email string) error {
	a, err := s.Repo.AccountByEmail(ctx, strings.TrimSpace(email))
	if err != nil || a.Disabled {
		return nil //nolint:nilerr // the answer is the same either way
	}
	return s.mailLink(ctx, s.Repo, a)
}

// mailLink emails an Account's holder a sign-in link that works once, within 30 minutes; an Account
// without an email cannot get one.
func (s *Service) mailLink(ctx context.Context, r Repository, a domain.Account) error {
	if a.Email == "" {
		return domain.ErrConflict
	}
	token, hash, err := newToken()
	if err != nil {
		return err
	}
	now := s.Now()
	if err := r.InsertSignInLink(ctx, hash, a.ID, now, now.Add(SignInLink)); err != nil {
		return err
	}
	body := "Hello " + a.Nickname + ",\n\nSign in to Grimoire with this link; it works once, within 30 minutes:\n\n" +
		strings.TrimRight(s.BaseURL, "/") + "/sign-in-link#" + token + "\n\nIf you did not ask for it, ignore this email.\n"
	return s.Mailer.Send(ctx, a.Email, "Your Grimoire sign-in link", body)
}

// UseLink signs in with an emailed link, once, or asks for the second step when two-step is on.
func (s *Service) UseLink(ctx context.Context, token, userAgent string) (domain.SignedIn, error) {
	id, err := s.Repo.UseSignInLink(ctx, HashToken(token), s.Now())
	if err != nil {
		return domain.SignedIn{}, domain.ErrExpired
	}
	a, err := s.Repo.AccountByID(ctx, id)
	if err != nil || a.Disabled {
		return domain.SignedIn{}, domain.ErrExpired
	}
	return s.firstStep(ctx, a, userAgent)
}

// Me reads the Account a subject signs in as, with how it signs in.
func (s *Service) Me(ctx context.Context, subject string) (domain.Profile, error) {
	a, err := s.Repo.AccountBySubject(ctx, subject)
	if err != nil {
		return domain.Profile{}, err
	}
	return s.profile(ctx, a)
}

func (s *Service) profile(ctx context.Context, a domain.Account) (domain.Profile, error) {
	p := domain.Profile{Account: a, HasPassword: false, Link: nil, TwoStep: false, RecoveryCodesLeft: 0, AdminPowers: s.IsAdmin(ctx, a.Subject)}
	var err error
	if p.HasPassword, err = s.Repo.HasPassword(ctx, a.ID); err != nil {
		return p, err
	}
	if p.TwoStep, p.RecoveryCodesLeft, err = s.twoStepOf(ctx, a.ID); err != nil {
		return p, err
	}
	link, err := s.Repo.LinkOf(ctx, a.ID)
	switch {
	case err == nil:
		p.Link = &link
	case !errors.Is(err, domain.ErrNotFound):
		return p, err
	}
	return p, nil
}

// UpdateProfile changes the signed-in Account's Username, Nickname and email. A linked login's own
// claims stay as the provider sent them.
func (s *Service) UpdateProfile(ctx context.Context, subject string, in domain.ProfileChange) (domain.Profile, error) {
	in, err := cleanProfile(in)
	if err != nil {
		return domain.Profile{}, err
	}
	a, err := s.Repo.AccountBySubject(ctx, subject)
	if err != nil {
		return domain.Profile{}, err
	}
	if err := s.Repo.UpdateProfile(ctx, a.ID, in); err != nil {
		return domain.Profile{}, err
	}
	a.Username, a.Nickname, a.Email = in.Username, in.Nickname, in.Email
	return s.profile(ctx, a)
}

// SetPassword sets a new password for the signed-in Account.
func (s *Service) SetPassword(ctx context.Context, subject, password string) error {
	if n := utf8.RuneCountInString(password); n < 10 || n > 200 {
		return domain.ErrInvalid
	}
	a, err := s.Repo.AccountBySubject(ctx, subject)
	if err != nil {
		return err
	}
	hash, err := s.Passwords.Hash(password)
	if err != nil {
		return err
	}
	if err := s.Repo.SetPassword(ctx, a.ID, hash); err != nil {
		return err
	}
	return s.record(ctx, s.Repo, a.ID, subject, domain.EventPasswordSet, "")
}
