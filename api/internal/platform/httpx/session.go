package httpx

import (
	"context"
	"net/http"
	"strings"
)

// SessionResolver finds the subject a session token signs in and whether the session is strong, and the
// subject and scopes an Access Token acts with.
type SessionResolver interface {
	Resolve(ctx context.Context, token string) (string, bool, bool)
	ResolveToken(ctx context.Context, token string) (string, []string, bool)
}

// ScopesHeader carries the scopes of the Access Token a request came with; only Sessions sets it.
const ScopesHeader = "X-Grimoire-Scopes"

// tokenPrefix marks a Grimoire Access Token; any other bearer is left for the platform.
const tokenPrefix = "Bearer gmt_" //nolint:gosec // a prefix, not a credential

type (
	userAgentKey struct{}
	strongKey    struct{}
)

// Strong reports whether a request's identity is strong: a session that passed a second step or came
// from the external login, or an identity the platform's forward-auth set.
func Strong(ctx context.Context) bool {
	strong, _ := ctx.Value(strongKey{}).(bool)
	return strong
}

// UserAgent is the User-Agent of the request a context belongs to.
func UserAgent(ctx context.Context) string {
	ua, _ := ctx.Value(userAgentKey{}).(string)
	return ua
}

// Sessions turns a session cookie or an Access Token into the request's identity. Unless forward-auth
// is trusted, an identity a client sends itself is dropped first, so only a valid session, a token or
// the platform sets one. A token that is not valid is refused outright.
func Sessions(cookie string, resolver SessionResolver, trustForwardAuth bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del(ScopesHeader)
		if !trustForwardAuth {
			r.Header.Del(IdentityHeader)
		}
		if bearer := r.Header.Get("Authorization"); strings.HasPrefix(bearer, tokenPrefix) {
			subject, scopes, ok := resolver.ResolveToken(r.Context(), strings.TrimPrefix(bearer, "Bearer "))
			if !ok {
				WriteProblem(w, http.StatusUnauthorized, "Unauthorized", "This Access Token is not valid, has expired or was revoked.")
				return
			}
			r.Header.Set(IdentityHeader, subject)
			r.Header.Set(ScopesHeader, strings.Join(scopes, " "))
			ctx := context.WithValue(context.WithValue(r.Context(), userAgentKey{}, r.UserAgent()), strongKey{}, false)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		strong := r.Header.Get(IdentityHeader) != ""
		if c, err := r.Cookie(cookie); err == nil && c.Value != "" && !strong {
			if subject, s, ok := resolver.Resolve(r.Context(), c.Value); ok {
				r.Header.Set(IdentityHeader, subject)
				strong = s
			}
		}
		ctx := context.WithValue(context.WithValue(r.Context(), userAgentKey{}, r.UserAgent()), strongKey{}, strong)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
