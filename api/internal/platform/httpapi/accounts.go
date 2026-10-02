package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/auth"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpx"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
)

// AccountService is Grimoire's own Accounts.
type AccountService interface {
	CreateInvite(ctx context.Context, by string, hours int, admin bool) (string, domain.Invite, error)
	Invite(ctx context.Context, token string) (domain.Invite, error)
	Accept(ctx context.Context, token string, in domain.Setup, userAgent string) (domain.Account, string, error)
	SignIn(ctx context.Context, username, password, userAgent string) (domain.SignedIn, error)
	SignOut(ctx context.Context, token string) error
	RequestLink(ctx context.Context, email string) error
	UseLink(ctx context.Context, token, userAgent string) (domain.SignedIn, error)
	Me(ctx context.Context, subject string) (domain.Profile, error)
	SetPassword(ctx context.Context, subject, password string) error
	UpdateProfile(ctx context.Context, subject string, in domain.ProfileChange) (domain.Profile, error)
	OIDCEnabled() bool
	StartOIDC(ctx context.Context, linkFor string) (string, string, error)
	FinishOIDC(ctx context.Context, code, state, userAgent string) (app.OIDCOutcome, error)
	CreateFromOIDC(ctx context.Context, token, username, nickname, userAgent string) (domain.Account, string, error)
	LinkFromOIDC(ctx context.Context, token, username, password, userAgent string) (domain.Account, string, error)
	Unlink(ctx context.Context, subject string) error
	PassTwoStep(ctx context.Context, challenge, code, userAgent string) (domain.Account, string, error)
	BeginTwoStep(ctx context.Context, subject string) (app.TwoStepSetup, error)
	ConfirmTwoStep(ctx context.Context, subject, code, session string) ([]string, error)
	DisableTwoStep(ctx context.Context, subject, code string) error
	ResetRecoveryCodes(ctx context.Context, subject, code string) ([]string, error)
	MintToken(ctx context.Context, subject, name string, scopes []string, days int) (string, domain.AccessToken, error)
	AccessTokens(ctx context.Context, subject string) ([]domain.AccessToken, error)
	RevokeToken(ctx context.Context, subject string, id uuid.UUID) error
	AdminAccounts(ctx context.Context, actor string) ([]domain.Listed, []domain.ListedInvite, error)
	AdminAccount(ctx context.Context, actor string, id domain.AccountID) (domain.Detail, error)
	SendSignInLink(ctx context.Context, actor string, id domain.AccountID) error
	SetAdmin(ctx context.Context, actor string, id domain.AccountID, admin bool) error
	SetDisabled(ctx context.Context, actor string, id domain.AccountID, disabled bool) error
	ResetTwoStep(ctx context.Context, actor string, id domain.AccountID) error
	History(ctx context.Context, subject string) ([]domain.Event, error)
	IsAdmin(ctx context.Context, subject string) bool
}

var _ AccountService = (*app.Service)(nil)

// SessionCookie is the name of the cookie a signed-in device carries.
const SessionCookie = "grimoire_session"

func sessionCookie(token string) string {
	return SessionCookie + "=" + token + "; Path=/; Max-Age=" + strconv.Itoa(int(app.SessionTTL.Seconds())) + "; HttpOnly; Secure; SameSite=Lax"
}

func accountOut(a domain.Account) oas.Account {
	return oas.Account{ID: oas.ID(a.ID), Username: oas.Username(a.Username), Nickname: a.Nickname, Email: a.Email, Admin: a.Admin}
}

func signedIn(a domain.Account, token string) *oas.AcceptAccountInviteCreatedHeaders {
	return &oas.AcceptAccountInviteCreatedHeaders{SetCookie: oas.NewOptString(sessionCookie(token)), Response: accountOut(a)}
}

func (h *Handler) identityError(ctx context.Context, op string, err error) *oas.ProblemStatusCodeWithHeaders {
	switch {
	case errors.Is(err, domain.ErrInvalid):
		return problem(http.StatusUnprocessableEntity, "Invalid", "Choose a Username of 3 to 32 lowercase letters, digits, dots, dashes or underscores, a Nickname, an email and a password of at least 10 characters.")
	case errors.Is(err, domain.ErrConflict):
		return problem(http.StatusConflict, "Taken", "That Username or email already has an Account.")
	case errors.Is(err, domain.ErrExpired), errors.Is(err, domain.ErrNotFound):
		return problem(http.StatusGone, "Gone", "This link has been used or has expired.")
	case errors.Is(err, domain.ErrUnauthenticated):
		return problem(http.StatusUnauthorized, "Unauthorized", "That Username and password do not match.")
	case errors.Is(err, domain.ErrForbidden):
		return problem(http.StatusForbidden, "Forbidden", "Only an Admin can do that.")
	}
	h.Log.ErrorContext(ctx, op, "error", err)
	return unavailable()
}

// CreateAccountInvite makes an Account Invite for an Admin.
func (h *Handler) CreateAccountInvite(ctx context.Context, req *oas.AccountInviteRequest) (oas.CreateAccountInviteRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	token, inv, err := h.Accounts.CreateInvite(ctx, id.Subject, int(req.Hours), req.Admin.Or(false))
	if err != nil {
		return h.identityError(ctx, "create account invite", err), nil
	}
	return &oas.AccountInviteCreatedHeaders{Response: oas.AccountInviteCreated{Token: token, ExpiresAt: inv.ExpiresAt.UTC()}}, nil
}

// PreviewAccountInvite checks an invite link.
func (h *Handler) PreviewAccountInvite(ctx context.Context, req *oas.LinkToken) (oas.PreviewAccountInviteRes, error) {
	inv, err := h.Accounts.Invite(ctx, req.Token)
	if err != nil {
		return h.identityError(ctx, "preview account invite", err), nil
	}
	return &oas.AccountInviteHeaders{Response: oas.AccountInvite{ExpiresAt: inv.ExpiresAt.UTC(), Admin: inv.Admin}}, nil
}

// AcceptAccountInvite sets up an Account from an invite and signs it in.
func (h *Handler) AcceptAccountInvite(ctx context.Context, req *oas.AccountSetup) (oas.AcceptAccountInviteRes, error) {
	in := domain.Setup{Username: req.Username, Nickname: req.Nickname, Email: req.Email, Password: req.Password}
	a, token, err := h.Accounts.Accept(ctx, req.Token, in, httpx.UserAgent(ctx))
	if err != nil {
		return h.identityError(ctx, "accept account invite", err), nil
	}
	return h.signedInProfile(ctx, a, token), nil
}

// SignIn signs in with a Username and password.
func (h *Handler) SignIn(ctx context.Context, req *oas.SignInRequest) (oas.SignInRes, error) {
	out, err := h.Accounts.SignIn(ctx, req.Username, req.Password, httpx.UserAgent(ctx))
	if err != nil {
		return h.identityError(ctx, "sign in", err), nil
	}
	if out.Challenge != "" {
		return challenged(out), nil
	}
	return h.signedInProfile(ctx, out.Account, out.Session), nil
}

// SignOut ends this device's session.
func (h *Handler) SignOut(ctx context.Context, params oas.SignOutParams) (oas.SignOutRes, error) {
	if token, ok := params.GrimoireSession.Get(); ok {
		if err := h.Accounts.SignOut(ctx, token); err != nil {
			return h.identityError(ctx, "sign out", err), nil
		}
	}
	gone := SessionCookie + "=; Path=/; Max-Age=0; HttpOnly; Secure; SameSite=Lax"
	return &oas.SignOutNoContent{SetCookie: oas.NewOptString(gone)}, nil
}

// RequestSignInLink emails a sign-in link, if an Account has the email.
func (h *Handler) RequestSignInLink(ctx context.Context, req *oas.SignInLinkRequest) (oas.RequestSignInLinkRes, error) {
	if err := h.Accounts.RequestLink(ctx, req.Email); err != nil {
		return h.identityError(ctx, "request sign-in link", err), nil
	}
	return &oas.RequestSignInLinkAccepted{}, nil
}

// UseSignInLink signs in with an emailed link.
func (h *Handler) UseSignInLink(ctx context.Context, req *oas.LinkToken) (oas.UseSignInLinkRes, error) {
	out, err := h.Accounts.UseLink(ctx, req.Token, httpx.UserAgent(ctx))
	if err != nil {
		return h.identityError(ctx, "use sign-in link", err), nil
	}
	if out.Challenge != "" {
		return challenged(out), nil
	}
	return h.signedInProfile(ctx, out.Account, out.Session), nil
}

// GetAccount reads the signed-in Account.
func (h *Handler) GetAccount(ctx context.Context) (oas.GetAccountRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	p, err := h.Accounts.Me(ctx, id.Subject)
	if errors.Is(err, domain.ErrNotFound) {
		return problem(http.StatusNotFound, "Not found", "You are signed in without a Grimoire Account."), nil
	}
	if err != nil {
		return h.identityError(ctx, "get account", err), nil
	}
	return &oas.AccountHeaders{Response: profileOut(p)}, nil
}

// SetAccountPassword sets the signed-in Account's password.
func (h *Handler) SetAccountPassword(ctx context.Context, req *oas.PasswordChange) (oas.SetAccountPasswordRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Accounts.SetPassword(ctx, id.Subject, req.Password); err != nil {
		return h.identityError(ctx, "set password", err), nil
	}
	return &oas.SetAccountPasswordNoContent{}, nil
}
