package httpx

import (
	"context"
	"net/http"
)

// SessionResolver finds the subject a session token signs in, and whether the session is strong.
type SessionResolver interface {
	Resolve(ctx context.Context, token string) (string, bool, bool)
}

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

// Sessions turns a session cookie into the request's identity. Unless forward-auth is trusted, an
// identity a client sends itself is dropped first, so only a valid session or the platform sets one.
func Sessions(cookie string, resolver SessionResolver, trustForwardAuth bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !trustForwardAuth {
			r.Header.Del(IdentityHeader)
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
