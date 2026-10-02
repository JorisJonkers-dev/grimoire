package httpapi

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"net/url"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/auth"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpx"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
)

// OIDCCookie binds an external sign-in to the browser that started it, so nobody can finish one in
// someone else's browser.
const OIDCCookie = "grimoire_oidc"

func oidcCookie(state string) string {
	return OIDCCookie + "=" + state + "; Path=/api/v1/oidc; Max-Age=600; HttpOnly; Secure; SameSite=Lax"
}

func (h *Handler) oidcError(ctx context.Context, op string, err error) *oas.ProblemStatusCodeWithHeaders {
	switch {
	case errors.Is(err, domain.ErrDisabled):
		return problem(http.StatusNotFound, "Not set up", "Signing in with an external login is not set up here.")
	case errors.Is(err, domain.ErrForbidden):
		return problem(http.StatusForbidden, "Forbidden", "Your login does not have access to Grimoire.")
	case errors.Is(err, domain.ErrUnauthenticated):
		return problem(http.StatusUnauthorized, "Unauthorized", "The sign-in could not be verified; try again.")
	case errors.Is(err, domain.ErrConflict):
		return problem(http.StatusConflict, "Already linked", "This Account already has a linked login, or that login belongs to another Account.")
	case errors.Is(err, domain.ErrManaged):
		return problem(http.StatusConflict, "Password needed", "Set a password before unlinking, or this Account could sign in only by email.")
	}
	return h.identityError(ctx, op, err)
}

// GetSignInMethods names the external sign-in, if one is set up.
func (h *Handler) GetSignInMethods(_ context.Context) (oas.GetSignInMethodsRes, error) {
	out := oas.SignInMethods{Oidc: oas.OptString{}}
	if h.Accounts.OIDCEnabled() {
		out.Oidc = oas.NewOptString(h.OIDCName)
	}
	return &oas.SignInMethodsHeaders{Response: out}, nil
}

func (h *Handler) startOIDC(ctx context.Context, linkFor string) (*oas.OidcRedirectHeaders, *oas.ProblemStatusCodeWithHeaders) {
	to, state, err := h.Accounts.StartOIDC(ctx, linkFor)
	if err != nil {
		return nil, h.oidcError(ctx, "start oidc", err)
	}
	u, err := url.Parse(to)
	if err != nil {
		return nil, h.oidcError(ctx, "start oidc", err)
	}
	return &oas.OidcRedirectHeaders{SetCookie: oas.NewOptString(oidcCookie(state)), Response: oas.OidcRedirect{URL: *u}}, nil
}

// StartOidcSignIn starts signing in with the external login.
func (h *Handler) StartOidcSignIn(ctx context.Context) (oas.StartOidcSignInRes, error) {
	out, bad := h.startOIDC(ctx, "")
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// StartOidcLink starts linking the external login to the signed-in Account.
func (h *Handler) StartOidcLink(ctx context.Context) (oas.StartOidcLinkRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	out, bad := h.startOIDC(ctx, id.Subject)
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// FinishOidc takes the browser back from the provider, in the browser that started the sign-in.
func (h *Handler) FinishOidc(ctx context.Context, req *oas.OidcCallback, params oas.FinishOidcParams) (oas.FinishOidcRes, error) {
	bound, _ := params.GrimoireOidc.Get()
	if subtle.ConstantTimeCompare([]byte(bound), []byte(req.State)) != 1 {
		return problem(http.StatusGone, "Gone", "This sign-in was started in another browser or has expired; start again."), nil
	}
	got, err := h.Accounts.FinishOIDC(ctx, req.Code, req.State, httpx.UserAgent(ctx))
	if err != nil {
		return h.oidcError(ctx, "finish oidc", err), nil
	}
	out := &oas.OidcOutcomeHeaders{Response: oas.OidcOutcome{Status: oas.OidcOutcomeStatusChoose, Account: oas.OptAccount{}, Pending: oas.OptOidcPending{}}}
	switch {
	case got.Pending != nil:
		p := got.Pending
		out.Response.Pending = oas.NewOptOidcPending(oas.OidcPending{Token: p.Token, Email: p.Email, Username: p.Username, Name: p.Name})
	case got.Linked:
		out.Response.Status = oas.OidcOutcomeStatusLinked
		out.Response.Account = oas.NewOptAccount(accountOut(got.Account))
	default:
		out.Response.Status = oas.OidcOutcomeStatusSignedIn
		out.Response.Account = oas.NewOptAccount(accountOut(got.Account))
		out.SetCookie = oas.NewOptString(sessionCookie(got.Session))
	}
	return out, nil
}

// CreateOidcAccount sets up an Account for a waiting login and signs it in.
func (h *Handler) CreateOidcAccount(ctx context.Context, req *oas.OidcAccountSetup) (oas.CreateOidcAccountRes, error) {
	a, token, err := h.Accounts.CreateFromOIDC(ctx, req.Token, req.Username, req.Nickname, httpx.UserAgent(ctx))
	if err != nil {
		return h.identityError(ctx, "create oidc account", err), nil
	}
	return h.signedInProfile(ctx, a, token), nil
}

// LinkOidcAccount links a waiting login to an existing Account and signs it in.
func (h *Handler) LinkOidcAccount(ctx context.Context, req *oas.OidcAccountLink) (oas.LinkOidcAccountRes, error) {
	a, token, err := h.Accounts.LinkFromOIDC(ctx, req.Token, req.Username, req.Password, httpx.UserAgent(ctx))
	if err != nil {
		return h.oidcError(ctx, "link oidc account", err), nil
	}
	return h.signedInProfile(ctx, a, token), nil
}

// signedInProfile answers a sign-in with the whole Account, link included.
func (h *Handler) signedInProfile(ctx context.Context, a domain.Account, token string) *oas.AcceptAccountInviteCreatedHeaders {
	out := signedIn(a, token)
	if p, err := h.Accounts.Me(ctx, a.Subject); err == nil {
		out.Response = profileOut(p)
	}
	return out
}

// UnlinkOidc removes the signed-in Account's linked login.
func (h *Handler) UnlinkOidc(ctx context.Context) (oas.UnlinkOidcRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Accounts.Unlink(ctx, id.Subject); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return problem(http.StatusNotFound, "Not linked", "This Account has no linked login."), nil
		}
		return h.oidcError(ctx, "unlink oidc", err), nil
	}
	return &oas.UnlinkOidcNoContent{}, nil
}

// UpdateAccount changes the signed-in Account's Username, Nickname and email.
func (h *Handler) UpdateAccount(ctx context.Context, req *oas.AccountChange) (oas.UpdateAccountRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	p, err := h.Accounts.UpdateProfile(ctx, id.Subject, domain.ProfileChange{Username: req.Username, Nickname: req.Nickname, Email: req.Email})
	if err != nil {
		return h.identityError(ctx, "update account", err), nil
	}
	return &oas.AccountHeaders{Response: profileOut(p)}, nil
}

func profileOut(p domain.Profile) oas.Account {
	out := accountOut(p.Account)
	out.HasPassword, out.TwoStep, out.AdminPowers = p.HasPassword, p.TwoStep, p.AdminPowers
	out.RecoveryCodesLeft = int32(p.RecoveryCodesLeft) //nolint:gosec // at most ten
	if l := p.Link; l != nil {
		out.Oidc = oas.NewOptOidcLink(oas.OidcLink{Email: l.Email, Username: l.Username, Name: l.Name, LinkedAt: l.LinkedAt.UTC()})
	}
	return out
}
