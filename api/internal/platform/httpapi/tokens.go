package httpapi

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/auth"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpx"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
)

// scopeOf is the Access Token scope each operation group needs; the Account group takes a session.
var scopeOf = map[string]string{"Read": domain.ScopeRead, "Build": domain.ScopeBuild, "Play": domain.ScopePlay} //nolint:gochecknoglobals // a fixed table

// allows reports whether an Access Token's scopes cover an operation group.
func allows(granted, group string) bool {
	need, ok := scopeOf[group]
	return ok && slices.Contains(strings.Fields(granted), need)
}

func refuseScope(w http.ResponseWriter) {
	httpx.WriteProblem(w, http.StatusForbidden, "Forbidden", "This Access Token's scopes do not allow that.")
}

// scoped holds a request that came with an Access Token to the scope its operation needs.
func scoped(srv *oas.Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if granted := r.Header.Get(httpx.ScopesHeader); granted != "" {
			if route, ok := srv.FindPath(r.Method, r.URL); ok && !allows(granted, route.OperationGroup()) {
				refuseScope(w)
				return
			}
		}
		srv.ServeHTTP(w, r)
	})
}

// playing holds the live socket to Access Tokens that may play.
func playing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if granted := r.Header.Get(httpx.ScopesHeader); granted != "" && !allows(granted, "Play") {
			refuseScope(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func accessTokenOut(t domain.AccessToken) oas.AccessToken {
	scopes := make([]oas.AccessTokenScope, 0, len(t.Scopes))
	for _, s := range t.Scopes {
		scopes = append(scopes, oas.AccessTokenScope(s))
	}
	out := oas.AccessToken{ID: oas.ID(t.ID), Name: t.Name, Scopes: scopes, CreatedAt: t.CreatedAt.UTC(), ExpiresAt: t.ExpiresAt.UTC(), LastUsedAt: oas.OptDateTime{}}
	if t.LastUsedAt != nil {
		out.LastUsedAt = oas.NewOptDateTime(t.LastUsedAt.UTC())
	}
	return out
}

// ListAccessTokens lists the signed-in Account's Access Tokens.
func (h *Handler) ListAccessTokens(ctx context.Context) (oas.ListAccessTokensRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	tokens, err := h.Accounts.AccessTokens(ctx, id.Subject)
	if err != nil {
		return h.identityError(ctx, "list access tokens", err), nil
	}
	items := make([]oas.AccessToken, 0, len(tokens))
	for _, t := range tokens {
		items = append(items, accessTokenOut(t))
	}
	return &oas.AccessTokenListHeaders{Response: oas.AccessTokenList{Items: items}}, nil
}

// CreateAccessToken mints an Access Token for the signed-in Account.
func (h *Handler) CreateAccessToken(ctx context.Context, req *oas.AccessTokenRequest) (oas.CreateAccessTokenRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	scopes := make([]string, 0, len(req.Scopes))
	for _, s := range req.Scopes {
		scopes = append(scopes, string(s))
	}
	token, t, err := h.Accounts.MintToken(ctx, id.Subject, req.Name, scopes, int(req.Days))
	if errors.Is(err, domain.ErrInvalid) {
		return problem(http.StatusUnprocessableEntity, "Invalid", "Give the token a name, at least one scope and 1 to 365 days."), nil
	}
	if err != nil {
		return h.identityError(ctx, "create access token", err), nil
	}
	return &oas.AccessTokenCreatedHeaders{Response: oas.AccessTokenCreated{Token: token, AccessToken: accessTokenOut(t)}}, nil
}

// RevokeAccessToken ends one of the signed-in Account's Access Tokens.
func (h *Handler) RevokeAccessToken(ctx context.Context, params oas.RevokeAccessTokenParams) (oas.RevokeAccessTokenRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Accounts.RevokeToken(ctx, id.Subject, uuid.UUID(params.AccessId)); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return problem(http.StatusNotFound, "Not found", "You have no such Access Token."), nil
		}
		return h.identityError(ctx, "revoke access token", err), nil
	}
	return &oas.RevokeAccessTokenNoContent{}, nil
}
