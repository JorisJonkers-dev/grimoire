// Package auth turns the platform's forward-auth identity into a request identity.
package auth

import (
	"context"
	"errors"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
)

// ErrUnauthenticated is returned when a request carries no identity.
var ErrUnauthenticated = errors.New("auth: no identity")

// Identity is the authenticated account behind a request.
type Identity struct {
	Subject string
}

type ctxKey struct{}

// WithIdentity stores an identity in the context.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// FromContext returns the identity stored in the context, if any.
func FromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	return id, ok
}

// ForwardAuth implements the generated security handler for the forwardAuth scheme.
type ForwardAuth struct{}

// HandleForwardAuth accepts the subject injected by the platform's forward-auth.
func (ForwardAuth) HandleForwardAuth(ctx context.Context, _ oas.OperationName, t oas.ForwardAuth) (context.Context, error) {
	if t.APIKey == "" {
		return ctx, ErrUnauthenticated
	}
	return WithIdentity(ctx, Identity{Subject: t.APIKey}), nil
}
