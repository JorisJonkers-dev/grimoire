package httpx

import (
	"context"
	"net/http"
)

// SessionResolver finds the subject a session token signs in.
type SessionResolver interface {
	Resolve(ctx context.Context, token string) (string, bool)
}

type userAgentKey struct{}

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
		if c, err := r.Cookie(cookie); err == nil && c.Value != "" && r.Header.Get(IdentityHeader) == "" {
			if subject, ok := resolver.Resolve(r.Context(), c.Value); ok {
				r.Header.Set(IdentityHeader, subject)
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userAgentKey{}, r.UserAgent())))
	})
}
